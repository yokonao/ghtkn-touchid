package main

/*
#cgo LDFLAGS: -framework Foundation -framework LocalAuthentication
#include <stdlib.h>
#include "touchid.h"
*/
import "C"

import (
	"errors"
	"unsafe"
)

func authenticate(reason string) error {
	r := C.CString(reason)
	defer C.free(unsafe.Pointer(r))
	var message *C.char
	if C.touchid_authenticate(r, &message) != 0 {
		return nil
	}
	defer C.free(unsafe.Pointer(message))
	return errors.New(C.GoString(message))
}
