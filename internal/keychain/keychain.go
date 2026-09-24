// Package keychain stores generic passwords in the default (login) Keychain with
// an access list that trusts only the running binary. That Keychain needs no
// keychain-access-groups entitlement, which ad-hoc signed binaries cannot carry.
// Its APIs are all deprecated, but it is the only one usable without it.
package keychain

import (
	"bytes"
	"errors"
	"fmt"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

const (
	errSecSuccess         = 0
	errSecItemNotFound    = -25300
	kCFStringEncodingUTF8 = 0x08000100
)

type Item struct {
	Service string
	Account string
}

var (
	cfStringCreateWithCString         func(alloc uintptr, s string, encoding uint32) uintptr
	cfStringGetLength                 func(s uintptr) int
	cfStringGetMaximumSizeForEncoding func(length int, encoding uint32) int
	cfStringGetCString                func(s uintptr, buf *byte, size int, encoding uint32) bool
	cfDictionaryCreateMutable         func(alloc uintptr, capacity int, keyCallBacks, valueCallBacks uintptr) uintptr
	cfDictionarySetValue              func(dict, key, value uintptr)
	cfArrayCreate                     func(alloc uintptr, values *uintptr, count int, callBacks uintptr) uintptr
	cfDataCreate                      func(alloc uintptr, bytes *byte, length int) uintptr
	cfDataGetLength                   func(data uintptr) int
	cfDataGetBytePtr                  func(data uintptr) *byte
	cfRelease                         func(ref uintptr)

	secKeychainCopyDefault              func(keychain *uintptr) int32
	secItemCopyMatching                 func(query uintptr, result *uintptr) int32
	secItemAdd                          func(attributes uintptr, result *uintptr) int32
	secItemUpdate                       func(query, attributes uintptr) int32
	secItemDelete                       func(query uintptr) int32
	secTrustedApplicationCreateFromPath func(path *byte, app *uintptr) int32
	secAccessCreate                     func(descriptor, trustedList uintptr, access *uintptr) int32
	secCopyErrorMessageString           func(status int32, reserved uintptr) uintptr

	kCFTypeDictionaryKeyCallBacks, kCFTypeDictionaryValueCallBacks, kCFTypeArrayCallBacks uintptr

	kCFBooleanTrue, kSecClass, kSecClassGenericPassword, kSecAttrService, kSecAttrAccount,
	kSecUseKeychain, kSecReturnData, kSecMatchLimit, kSecMatchLimitOne, kSecValueData,
	kSecAttrAccess uintptr
)

var load = sync.OnceValue(func() error {
	cf, err := purego.Dlopen("/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation", purego.RTLD_GLOBAL)
	if err != nil {
		return err
	}
	sec, err := purego.Dlopen("/System/Library/Frameworks/Security.framework/Security", purego.RTLD_GLOBAL)
	if err != nil {
		return err
	}
	for name, fn := range map[string]any{
		"CFStringCreateWithCString":         &cfStringCreateWithCString,
		"CFStringGetLength":                 &cfStringGetLength,
		"CFStringGetMaximumSizeForEncoding": &cfStringGetMaximumSizeForEncoding,
		"CFStringGetCString":                &cfStringGetCString,
		"CFDictionaryCreateMutable":         &cfDictionaryCreateMutable,
		"CFDictionarySetValue":              &cfDictionarySetValue,
		"CFArrayCreate":                     &cfArrayCreate,
		"CFDataCreate":                      &cfDataCreate,
		"CFDataGetLength":                   &cfDataGetLength,
		"CFDataGetBytePtr":                  &cfDataGetBytePtr,
		"CFRelease":                         &cfRelease,
	} {
		purego.RegisterLibFunc(fn, cf, name)
	}
	for name, fn := range map[string]any{
		"SecKeychainCopyDefault":              &secKeychainCopyDefault,
		"SecItemCopyMatching":                 &secItemCopyMatching,
		"SecItemAdd":                          &secItemAdd,
		"SecItemUpdate":                       &secItemUpdate,
		"SecItemDelete":                       &secItemDelete,
		"SecTrustedApplicationCreateFromPath": &secTrustedApplicationCreateFromPath,
		"SecAccessCreate":                     &secAccessCreate,
		"SecCopyErrorMessageString":           &secCopyErrorMessageString,
	} {
		purego.RegisterLibFunc(fn, sec, name)
	}

	for name, address := range map[string]*uintptr{
		"kCFTypeDictionaryKeyCallBacks":   &kCFTypeDictionaryKeyCallBacks,
		"kCFTypeDictionaryValueCallBacks": &kCFTypeDictionaryValueCallBacks,
		"kCFTypeArrayCallBacks":           &kCFTypeArrayCallBacks,
	} {
		if *address, err = purego.Dlsym(cf, name); err != nil {
			return err
		}
	}
	for lib, constants := range map[uintptr]map[string]*uintptr{
		cf: {"kCFBooleanTrue": &kCFBooleanTrue},
		sec: {
			"kSecClass":                &kSecClass,
			"kSecClassGenericPassword": &kSecClassGenericPassword,
			"kSecAttrService":          &kSecAttrService,
			"kSecAttrAccount":          &kSecAttrAccount,
			"kSecUseKeychain":          &kSecUseKeychain,
			"kSecReturnData":           &kSecReturnData,
			"kSecMatchLimit":           &kSecMatchLimit,
			"kSecMatchLimitOne":        &kSecMatchLimitOne,
			"kSecValueData":            &kSecValueData,
			"kSecAttrAccess":           &kSecAttrAccess,
		},
	} {
		for name, value := range constants {
			address, err := purego.Dlsym(lib, name)
			if err != nil {
				return err
			}
			*value = **(**uintptr)(unsafe.Pointer(&address))
		}
	}
	return nil
})

func cfString(s string) uintptr {
	return cfStringCreateWithCString(0, s, kCFStringEncodingUTF8)
}

func newDictionary() uintptr {
	return cfDictionaryCreateMutable(0, 0, kCFTypeDictionaryKeyCallBacks, kCFTypeDictionaryValueCallBacks)
}

func statusError(operation string, status int32) error {
	message := secCopyErrorMessageString(status, 0)
	if message == 0 {
		return fmt.Errorf("%s: OSStatus %d", operation, status)
	}
	defer cfRelease(message)
	buf := make([]byte, cfStringGetMaximumSizeForEncoding(cfStringGetLength(message), kCFStringEncodingUTF8)+1)
	if !cfStringGetCString(message, &buf[0], len(buf), kCFStringEncodingUTF8) {
		return fmt.Errorf("%s: OSStatus %d", operation, status)
	}
	return fmt.Errorf("%s: %s", operation, buf[:bytes.IndexByte(buf, 0)])
}

// query returns a new dictionary matching the item in the default Keychain.
func query(item Item) (uintptr, int32) {
	var keychain uintptr
	if status := secKeychainCopyDefault(&keychain); status != errSecSuccess {
		return 0, status
	}
	defer cfRelease(keychain)
	service := cfString(item.Service)
	defer cfRelease(service)
	account := cfString(item.Account)
	defer cfRelease(account)

	q := newDictionary()
	cfDictionarySetValue(q, kSecClass, kSecClassGenericPassword)
	cfDictionarySetValue(q, kSecAttrService, service)
	cfDictionarySetValue(q, kSecAttrAccount, account)
	cfDictionarySetValue(q, kSecUseKeychain, keychain)
	return q, errSecSuccess
}

// trustedSelf returns an access list that trusts only the running binary.
func trustedSelf() (uintptr, int32) {
	var application uintptr
	if status := secTrustedApplicationCreateFromPath(nil, &application); status != errSecSuccess {
		return 0, status
	}
	defer cfRelease(application)
	applications := cfArrayCreate(0, &application, 1, kCFTypeArrayCallBacks)
	defer cfRelease(applications)
	descriptor := cfString("ghtkn agent passphrase")
	defer cfRelease(descriptor)
	var access uintptr
	status := secAccessCreate(descriptor, applications, &access)
	return access, status
}

// Read returns nil without an error when the item does not exist.
func Read(item Item) ([]byte, error) {
	if err := load(); err != nil {
		return nil, err
	}
	operation := "read " + item.Service + " from Keychain"
	q, status := query(item)
	if status != errSecSuccess {
		return nil, statusError(operation, status)
	}
	defer cfRelease(q)
	cfDictionarySetValue(q, kSecReturnData, kCFBooleanTrue)
	cfDictionarySetValue(q, kSecMatchLimit, kSecMatchLimitOne)

	var data uintptr
	status = secItemCopyMatching(q, &data)
	if status == errSecItemNotFound {
		return nil, nil
	}
	if status != errSecSuccess {
		return nil, statusError(operation, status)
	}
	defer cfRelease(data)
	length := cfDataGetLength(data)
	if length == 0 {
		return nil, errors.New("the Keychain item " + item.Service + " is empty")
	}
	return append([]byte(nil), unsafe.Slice(cfDataGetBytePtr(data), length)...), nil
}

// Upsert writes the value and resets the access list to the running binary.
func Upsert(item Item, value []byte) error {
	if err := load(); err != nil {
		return err
	}
	operation := "store " + item.Service + " in Keychain"
	access, status := trustedSelf()
	if status != errSecSuccess {
		return statusError(operation, status)
	}
	defer cfRelease(access)
	data := cfDataCreate(0, unsafe.SliceData(value), len(value))
	defer cfRelease(data)
	q, status := query(item)
	if status != errSecSuccess {
		return statusError(operation, status)
	}
	defer cfRelease(q)

	attributes := newDictionary()
	defer cfRelease(attributes)
	cfDictionarySetValue(attributes, kSecValueData, data)
	cfDictionarySetValue(attributes, kSecAttrAccess, access)
	status = secItemUpdate(q, attributes)
	if status == errSecItemNotFound {
		cfDictionarySetValue(q, kSecValueData, data)
		cfDictionarySetValue(q, kSecAttrAccess, access)
		status = secItemAdd(q, nil)
	}
	if status != errSecSuccess {
		return statusError(operation, status)
	}
	return nil
}

// Delete succeeds when the item does not exist.
func Delete(item Item) error {
	if err := load(); err != nil {
		return err
	}
	operation := "remove " + item.Service + " from Keychain"
	q, status := query(item)
	if status != errSecSuccess {
		return statusError(operation, status)
	}
	defer cfRelease(q)
	status = secItemDelete(q)
	if status != errSecSuccess && status != errSecItemNotFound {
		return statusError(operation, status)
	}
	return nil
}

// Rename moves the item to another service under the same account.
func Rename(item Item, service string) error {
	if err := load(); err != nil {
		return err
	}
	operation := "rename " + item.Service + " to " + service + " in Keychain"
	q, status := query(item)
	if status != errSecSuccess {
		return statusError(operation, status)
	}
	defer cfRelease(q)
	to := cfString(service)
	defer cfRelease(to)
	attributes := newDictionary()
	defer cfRelease(attributes)
	cfDictionarySetValue(attributes, kSecAttrService, to)
	if status := secItemUpdate(q, attributes); status != errSecSuccess {
		return statusError(operation, status)
	}
	return nil
}
