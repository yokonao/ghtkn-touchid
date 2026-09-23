package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/yokonao/ghtkn-touchid/go/internal/app"
)

func main() {
	unlock := func(*cobra.Command, []string) error { return app.Unlock(os.Stderr) }
	root := &cobra.Command{
		Use:           "ghtkn-touchid",
		Short:         "Unlock a local ghtkn agent with a Touch ID-protected passphrase",
		Args:          cobra.NoArgs,
		RunE:          unlock,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.CompletionOptions.DisableDefaultCmd = true
	root.AddCommand(
		&cobra.Command{
			Use:   "unlock",
			Short: "Unlock the ghtkn agent (default)",
			Args:  cobra.NoArgs,
			RunE:  unlock,
		},
		&cobra.Command{
			Use:   "reset",
			Short: "Reset the ghtkn agent with a new passphrase stored in Keychain",
			Args:  cobra.NoArgs,
			RunE:  func(*cobra.Command, []string) error { return app.Reset(os.Stdin, os.Stderr) },
		},
	)
	if err := root.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "ghtkn-touchid: %v\n", err)
		os.Exit(1)
	}
}
