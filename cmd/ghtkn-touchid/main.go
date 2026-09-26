package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:           "ghtkn-touchid",
		Short:         "Unlock a local ghtkn agent with a Touch ID-protected passphrase",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.CompletionOptions.DisableDefaultCmd = true
	root.AddCommand(
		&cobra.Command{
			Use:   "unlock",
			Short: "Unlock the ghtkn agent",
			Args:  cobra.NoArgs,
			RunE:  func(*cobra.Command, []string) error { return unlock(os.Stderr) },
		},
		&cobra.Command{
			Use:   "reset",
			Short: "Reset the ghtkn agent with a new passphrase stored in Keychain",
			Args:  cobra.NoArgs,
			RunE:  func(*cobra.Command, []string) error { return reset(os.Stdin, os.Stderr) },
		},
	)
	if err := root.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "ghtkn-touchid: %v\n", err)
		os.Exit(1)
	}
}
