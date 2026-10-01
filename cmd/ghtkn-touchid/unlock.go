package main

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/yokonao/ghtkn-touchid/internal/agent"
	"github.com/yokonao/ghtkn-touchid/internal/passphrase"
)

func newUnlockCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "unlock",
		Short: "Unlock the ghtkn agent",
		Args:  cobra.NoArgs,
		RunE:  func(*cobra.Command, []string) error { return unlock(os.Stderr) },
	}
}

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
