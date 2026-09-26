// Package touchid gates an operation behind a Touch ID prompt.
package touchid

import (
	"errors"

	la "github.com/yokonao/appleframeworks/localauthentication"
)

func Authenticate(reason string) error {
	c, err := la.NewContext()
	if err != nil {
		return err
	}
	c.SetLocalizedCancelTitle("Cancel")
	c.SetLocalizedFallbackTitle("")
	c.SetTouchIDAuthenticationAllowableReuseDuration(0)
	if err := c.CanEvaluatePolicy(la.PolicyDeviceOwnerAuthenticationWithBiometrics); err != nil {
		return describe(err)
	}
	return describe(c.EvaluatePolicy(la.PolicyDeviceOwnerAuthenticationWithBiometrics, reason))
}

// describe drops the domain and code, which mean nothing to the user.
func describe(err error) error {
	if e, ok := errors.AsType[*la.Error](err); ok {
		return errors.New(e.Description)
	}
	return err
}
