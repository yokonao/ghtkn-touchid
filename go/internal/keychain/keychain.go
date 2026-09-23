// Package keychain stores generic passwords in the default (login) Keychain with
// an access list that trusts only the running binary. That Keychain needs no
// keychain-access-groups entitlement, which ad-hoc signed binaries cannot carry.
package keychain

/*
#cgo LDFLAGS: -framework CoreFoundation -framework Security
#include <stdlib.h>
#include "keychain.h"
*/
import "C"

import (
	"errors"
	"fmt"
	"unsafe"
)

type Item struct {
	Service string
	Account string
}

type cItem struct {
	service, account *C.char
}

func (i Item) c() cItem {
	return cItem{C.CString(i.Service), C.CString(i.Account)}
}

func (c cItem) free() {
	C.free(unsafe.Pointer(c.service))
	C.free(unsafe.Pointer(c.account))
}

func statusError(operation string, status C.OSStatus) error {
	message := C.kc_error(status)
	if message == nil {
		return fmt.Errorf("%s: OSStatus %d", operation, int32(status))
	}
	defer C.free(unsafe.Pointer(message))
	return fmt.Errorf("%s: %s", operation, C.GoString(message))
}

// Read returns nil without an error when the item does not exist.
func Read(item Item) ([]byte, error) {
	c := item.c()
	defer c.free()

	var data unsafe.Pointer
	var length C.size_t
	status := C.kc_read(c.service, c.account, &data, &length)
	if status == C.errSecItemNotFound {
		return nil, nil
	}
	if status != C.errSecSuccess {
		return nil, statusError("read "+item.Service+" from Keychain", status)
	}
	raw := unsafe.Slice((*byte)(data), length)
	result := make([]byte, length)
	copy(result, raw)
	clear(raw)
	C.free(data)
	if len(result) == 0 {
		return nil, errors.New("the Keychain item " + item.Service + " is empty")
	}
	return result, nil
}

// Upsert writes the value and resets the access list to the running binary.
func Upsert(item Item, value []byte) error {
	c := item.c()
	defer c.free()

	status := C.kc_upsert(c.service, c.account, unsafe.Pointer(unsafe.SliceData(value)), C.size_t(len(value)))
	if status != C.errSecSuccess {
		return statusError("store "+item.Service+" in Keychain", status)
	}
	return nil
}

// Delete succeeds when the item does not exist.
func Delete(item Item) error {
	c := item.c()
	defer c.free()

	status := C.kc_delete(c.service, c.account)
	if status != C.errSecSuccess && status != C.errSecItemNotFound {
		return statusError("remove "+item.Service+" from Keychain", status)
	}
	return nil
}

// Rename moves the item to another service under the same account.
func Rename(item Item, service string) error {
	c := item.c()
	defer c.free()
	to := C.CString(service)
	defer C.free(unsafe.Pointer(to))

	if status := C.kc_rename(c.service, to, c.account); status != C.errSecSuccess {
		return statusError("rename "+item.Service+" to "+service+" in Keychain", status)
	}
	return nil
}
