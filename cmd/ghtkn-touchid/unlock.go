package main

import (
	"fmt"
	"io"

	"github.com/yokonao/ghtkn-touchid/internal/agent"
	"github.com/yokonao/ghtkn-touchid/internal/passphrase"
)

func unlock(stderr io.Writer) error {
	result, err := agent.Unlock(passphrase.Load, agent.Send)
	if err != nil {
		return err
	}
	if result.AlreadyUnlocked {
		_, _ = fmt.Fprintf(stderr, "ghtkn agent is already unlocked; refresh_token_enabled=%t\n", result.RefreshTokenEnabled)
	} else {
		_, _ = fmt.Fprintln(stderr, "ghtkn agent unlocked; refresh_token_enabled=true")
	}
	return nil
}
