package main

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/yokonao/ghtkn-touchid/go/internal/ghtkn"
	"github.com/yokonao/ghtkn-touchid/go/internal/passphrase"
)

func reset(stdin *os.File, stderr io.Writer) error {
	if _, err := unix.IoctlGetTermios(int(stdin.Fd()), unix.TIOCGETA); err != nil {
		return errors.New("reset requires a terminal")
	}
	fmt.Fprint(stderr, "This deletes the ghtkn agent key and all cached tokens. Type RESET to continue: ")
	answer, _ := bufio.NewReader(stdin).ReadString('\n')
	if strings.TrimSuffix(answer, "\n") != "RESET" {
		fmt.Fprintln(stderr, "Canceled.")
		return nil
	}

	path, err := ghtkn.Executable()
	if err != nil {
		return err
	}
	secret, err := randomPassphrase()
	if err != nil {
		return err
	}
	defer clear(secret)
	if err := passphrase.Stage(secret); err != nil {
		return err
	}
	err = performReset(func() (int, error) { return ghtkn.ResetAgent(path, secret) }, passphrase.Commit)
	if errors.Is(err, passphrase.ErrKeptCommitted) {
		fmt.Fprintf(stderr, "warning: %v\n", err)
	} else if err != nil {
		return err
	}
	fmt.Fprintln(stderr, "Reset the ghtkn agent with a generated passphrase. Start and unlock the agent.")
	return nil
}

func randomPassphrase() ([]byte, error) {
	random := make([]byte, 32)
	defer clear(random)
	if _, err := rand.Read(random); err != nil {
		return nil, errors.New("generate a random passphrase")
	}
	encoded := make([]byte, base64.StdEncoding.EncodedLen(len(random)))
	base64.StdEncoding.Encode(encoded, random)
	return encoded, nil
}

// performReset commits the staged passphrase only after ghtkn accepted it, so a
// failed reset leaves the pending one for recovery.
func performReset(run func() (int, error), commit func() error) error {
	status, err := run()
	if err != nil {
		return errors.New("ghtkn agent reset failed; kept the pending passphrase for recovery")
	}
	if status != 0 {
		return fmt.Errorf("ghtkn agent reset exited %d; kept the pending passphrase for recovery", status)
	}
	return commit()
}
