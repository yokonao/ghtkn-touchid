package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	unlockCmd := func(*cobra.Command, []string) error { return unlock(loadCandidates, send) }
	root := &cobra.Command{
		Use:           "ghtkn-touchid",
		Short:         "Unlock a local ghtkn agent with a Touch ID-protected passphrase",
		Args:          cobra.NoArgs,
		RunE:          unlockCmd,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.CompletionOptions.DisableDefaultCmd = true
	root.AddCommand(
		&cobra.Command{
			Use:   "unlock",
			Short: "Unlock the ghtkn agent (default)",
			Args:  cobra.NoArgs,
			RunE:  unlockCmd,
		},
		&cobra.Command{
			Use:   "reset",
			Short: "Reset the ghtkn agent with a new passphrase stored in Keychain",
			Args:  cobra.NoArgs,
			RunE:  func(*cobra.Command, []string) error { return reset() },
		},
	)
	if err := root.Execute(); err != nil {
		printErr("%v", err)
		os.Exit(1)
	}
}

func printErr(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "ghtkn-touchid: "+format+"\n", args...)
}
