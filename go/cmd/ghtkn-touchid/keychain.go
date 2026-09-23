package main

/*
#cgo LDFLAGS: -framework CoreFoundation -framework Security
#include <stdlib.h>
#include "keychain.h"
*/
import "C"

import (
	"errors"
	"fmt"
	"os/user"
	"unsafe"
)

const (
	activeService    = "ghtkn-touchid.agent-passphrase"
	pendingService   = activeService + ".pending"
	committedService = activeService + ".committed"
)

func keychainError(operation string, status C.OSStatus) error {
	message := C.kc_error(status)
	if message == nil {
		return fmt.Errorf("%s: OSStatus %d", operation, int32(status))
	}
	defer C.free(unsafe.Pointer(message))
	return fmt.Errorf("%s: %s", operation, C.GoString(message))
}

func account() (*C.char, error) {
	u, err := user.Current()
	if err != nil {
		return nil, err
	}
	return C.CString(u.Username), nil
}

func loadCandidates() ([]storedPassphrase, error) {
	if err := authenticate("Unlock the ghtkn agent"); err != nil {
		return nil, err
	}
	var candidates []storedPassphrase
	var firstErr error
	for _, service := range []string{committedService, activeService, pendingService} {
		data, err := readPassphrase(service)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if data != nil {
			candidates = append(candidates, storedPassphrase{service, data})
		}
	}
	if len(candidates) == 0 {
		if firstErr != nil {
			return nil, firstErr
		}
		return nil, errors.New("the ghtkn passphrase is not stored in Keychain")
	}
	return candidates, nil
}

func stagePassphrase(passphrase []byte) error {
	if err := authenticate("Reset the ghtkn agent passphrase"); err != nil {
		return err
	}
	if err := deleteItem(pendingService); err != nil {
		return err
	}
	return upsert(pendingService, passphrase)
}

func commitPending() error {
	if err := deleteItem(committedService); err != nil {
		return err
	}
	if err := rename(pendingService, committedService); err != nil {
		return err
	}
	err := deleteItem(activeService)
	if err == nil {
		err = rename(committedService, activeService)
	}
	if err != nil {
		printErr("kept the committed passphrase for recovery: %v", err)
	}
	return nil
}

func readPassphrase(service string) ([]byte, error) {
	acc, err := account()
	if err != nil {
		return nil, err
	}
	defer C.free(unsafe.Pointer(acc))
	svc := C.CString(service)
	defer C.free(unsafe.Pointer(svc))

	var data unsafe.Pointer
	var length C.size_t
	status := C.kc_read(svc, acc, &data, &length)
	if status == C.errSecItemNotFound {
		return nil, nil
	}
	if status != C.errSecSuccess {
		return nil, keychainError("read the passphrase from Keychain", status)
	}
	raw := unsafe.Slice((*byte)(data), length)
	result := make([]byte, length)
	copy(result, raw)
	clear(raw)
	C.free(data)
	if len(result) == 0 {
		return nil, errors.New("the Keychain item contains no passphrase")
	}
	return result, nil
}

func upsert(service string, passphrase []byte) error {
	acc, err := account()
	if err != nil {
		return err
	}
	defer C.free(unsafe.Pointer(acc))
	svc := C.CString(service)
	defer C.free(unsafe.Pointer(svc))

	status := C.kc_upsert(svc, acc, unsafe.Pointer(unsafe.SliceData(passphrase)), C.size_t(len(passphrase)))
	if status != C.errSecSuccess {
		return keychainError("store the passphrase in Keychain", status)
	}
	return nil
}

func deleteItem(service string) error {
	acc, err := account()
	if err != nil {
		return err
	}
	defer C.free(unsafe.Pointer(acc))
	svc := C.CString(service)
	defer C.free(unsafe.Pointer(svc))

	status := C.kc_delete(svc, acc)
	if status != C.errSecSuccess && status != C.errSecItemNotFound {
		return keychainError("remove "+service+" from Keychain", status)
	}
	return nil
}

func rename(from, to string) error {
	acc, err := account()
	if err != nil {
		return err
	}
	defer C.free(unsafe.Pointer(acc))
	f, t := C.CString(from), C.CString(to)
	defer C.free(unsafe.Pointer(f))
	defer C.free(unsafe.Pointer(t))

	if status := C.kc_rename(f, t, acc); status != C.errSecSuccess {
		return keychainError("commit the passphrase in Keychain", status)
	}
	return nil
}
