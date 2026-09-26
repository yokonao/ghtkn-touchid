module github.com/yokonao/ghtkn-touchid

go 1.27.1

require (
	github.com/creack/pty v1.1.24
	github.com/ebitengine/purego v0.11.1
	github.com/spf13/cobra v1.10.2
	github.com/yokonao/appleframeworks v0.0.0-20260926004439-59d729c40644
	golang.org/x/sys v0.48.0
)

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
)

// Shipped the old Swift build output, which made the module ~141MB.
retract [v0.3.0, v0.3.1]
