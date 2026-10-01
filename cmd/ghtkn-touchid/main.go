package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// version is set at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	root := &cobra.Command{
		Use:           "ghtkn-touchid",
		Short:         "Unlock a local ghtkn agent with a Touch ID-protected passphrase",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.CompletionOptions.DisableDefaultCmd = true
	root.AddCommand(newUnlockCmd(), newResetCmd())
	if err := root.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "ghtkn-touchid: %v\n", err)
		os.Exit(1)
	}
}
