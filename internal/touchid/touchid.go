// Package touchid gates an operation behind a Touch ID prompt.
package touchid

import (
	"errors"
	"sync"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

const deviceOwnerAuthenticationWithBiometrics = 1

var load = sync.OnceValue(func() error {
	_, err := purego.Dlopen("/System/Library/Frameworks/LocalAuthentication.framework/LocalAuthentication", purego.RTLD_GLOBAL)
	return err
})

func nsString(s string) objc.ID {
	return objc.ID(objc.GetClass("NSString")).Send(objc.RegisterName("stringWithUTF8String:"), s)
}

func describe(nserror objc.ID, fallback string) error {
	if nserror != 0 {
		description := nserror.Send(objc.RegisterName("localizedDescription"))
		if message := objc.Send[string](description, objc.RegisterName("UTF8String")); message != "" {
			return errors.New(message)
		}
	}
	return errors.New(fallback)
}

func Authenticate(reason string) error {
	if err := load(); err != nil {
		return err
	}
	pool := objc.ID(objc.GetClass("NSAutoreleasePool")).Send(objc.RegisterName("new"))
	defer pool.Send(objc.RegisterName("drain"))

	context := objc.ID(objc.GetClass("LAContext")).Send(objc.RegisterName("new"))
	defer context.Send(objc.RegisterName("release"))
	context.Send(objc.RegisterName("setLocalizedCancelTitle:"), nsString("Cancel"))
	context.Send(objc.RegisterName("setLocalizedFallbackTitle:"), nsString(""))
	context.Send(objc.RegisterName("setTouchIDAuthenticationAllowableReuseDuration:"), 0.0)

	var availability objc.ID
	if !objc.Send[bool](context, objc.RegisterName("canEvaluatePolicy:error:"), deviceOwnerAuthenticationWithBiometrics, &availability) {
		return describe(availability, "Touch ID is unavailable")
	}

	result := make(chan error, 1)
	reply := objc.NewBlock(func(_ objc.Block, ok bool, nserror objc.ID) {
		if ok {
			result <- nil
		} else {
			result <- describe(nserror, "Touch ID authentication failed")
		}
	})
	defer reply.Release()
	context.Send(objc.RegisterName("evaluatePolicy:localizedReason:reply:"),
		deviceOwnerAuthenticationWithBiometrics, nsString(reason), reply)
	return <-result
}
