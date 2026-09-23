package main

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/creack/pty"
	"golang.org/x/sys/unix"
)

func reset() error {
	if _, err := unix.IoctlGetTermios(int(os.Stdin.Fd()), unix.TIOCGETA); err != nil {
		return errors.New("reset requires a terminal")
	}
	fmt.Fprint(os.Stderr, "This deletes the ghtkn agent key and all cached tokens. Type RESET to continue: ")
	answer, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	if strings.TrimSuffix(answer, "\n") != "RESET" {
		fmt.Fprintln(os.Stderr, "Canceled.")
		return nil
	}

	ghtkn, err := ghtknExecutable()
	if err != nil {
		return err
	}
	passphrase, err := randomPassphrase()
	if err != nil {
		return err
	}
	defer clear(passphrase)
	if err := stagePassphrase(passphrase); err != nil {
		return err
	}
	err = performReset(
		func() (int, error) { return runGhtknReset(ghtkn, passphrase) },
		commitPending)
	if err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "Reset the ghtkn agent with a generated passphrase. Start and unlock the agent.")
	return nil
}

func randomPassphrase() ([]byte, error) {
	random := make([]byte, 32)
	defer clear(random)
	if _, err := rand.Read(random); err != nil {
		return nil, errors.New("generate a random passphrase")
	}
	passphrase := make([]byte, base64.StdEncoding.EncodedLen(len(random)))
	base64.StdEncoding.Encode(passphrase, random)
	return passphrase, nil
}

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

func ghtknExecutable() (string, error) {
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if !filepath.IsAbs(dir) {
			continue
		}
		candidate := filepath.Join(dir, "ghtkn")
		if info, err := os.Stat(candidate); err == nil && info.Mode().IsRegular() && unix.Access(candidate, unix.X_OK) == nil {
			return candidate, nil
		}
	}
	return "", errors.New("ghtkn was not found in an absolute PATH entry")
}

func runGhtknReset(ghtkn string, passphrase []byte) (int, error) {
	cmd := exec.Command(ghtkn, "agent", "reset")
	master, err := pty.Start(cmd)
	if err != nil {
		return 0, fmt.Errorf("start ghtkn: %w", err)
	}
	defer master.Close()
	fd := int(master.Fd())
	pid := cmd.Process.Pid

	status, err := feedPassphrase(fd, pid, passphrase)
	if err != nil {
		unix.Kill(pid, unix.SIGTERM)
		// A session leader blocks in its exit path until the terminal output queue is
		// drained, so read the rest of it before waiting for the child.
		drainPTY(fd)
		waitForChild(pid)
		return 0, err
	}
	if status != nil {
		return exitCode(*status), nil
	}
	ws, err := waitForChild(pid)
	if err != nil {
		return 0, err
	}
	return exitCode(ws), nil
}

// feedPassphrase returns the child's status when it exits before asking for the passphrase.
func feedPassphrase(fd, pid int, passphrase []byte) (*unix.WaitStatus, error) {
	if err := writePTY(fd, []byte("y\n")); err != nil {
		return nil, err
	}
	if status, err := waitForEchoDisabled(fd, pid); status != nil || err != nil {
		return status, err
	}

	input := make([]byte, 0, len(passphrase)*2+2)
	input = append(append(input, passphrase...), '\n')
	input = append(append(input, passphrase...), '\n')
	defer clear(input)
	if err := writePTY(fd, input); err != nil {
		return nil, err
	}
	return nil, drainPTY(fd)
}

func exitCode(ws unix.WaitStatus) int {
	if ws.Signaled() {
		return 128 + int(ws.Signal())
	}
	return ws.ExitStatus()
}

func waitForEchoDisabled(fd, pid int) (*unix.WaitStatus, error) {
	for range 200 {
		// The prompts written so far are discarded rather than left in the terminal
		// output queue, which would otherwise stall the child once it fills up.
		if err := drainReadable(fd); err != nil {
			return nil, err
		}
		if t, err := unix.IoctlGetTermios(fd, unix.TIOCGETA); err == nil && t.Lflag&unix.ECHO == 0 {
			return nil, nil
		}
		var ws unix.WaitStatus
		wpid, err := unix.Wait4(pid, &ws, unix.WNOHANG, nil)
		if wpid == pid {
			return &ws, nil
		}
		if err != nil && err != unix.EINTR {
			return nil, fmt.Errorf("wait for ghtkn: %w", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	return nil, errors.New("ghtkn terminal echo remained enabled; refusing to send the passphrase")
}

func writePTY(fd int, data []byte) error {
	for len(data) > 0 {
		n, err := unix.Write(fd, data)
		if err == unix.EINTR {
			continue
		}
		if err != nil {
			return fmt.Errorf("write to ghtkn: %w", err)
		}
		if n == 0 {
			return errors.New("write to ghtkn: wrote zero bytes")
		}
		data = data[n:]
	}
	return nil
}

// drainReadable reads and discards whatever the child has already written, without blocking.
func drainReadable(fd int) error {
	buf := make([]byte, 4096)
	defer clear(buf)
	for {
		fds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
		n, err := unix.Poll(fds, 0)
		if err == unix.EINTR {
			continue
		}
		if err != nil {
			return fmt.Errorf("poll ghtkn output: %w", err)
		}
		if n == 0 {
			return nil
		}
		if done, err := readPTY(fd, buf); done || err != nil {
			return err
		}
	}
}

func drainPTY(fd int) error {
	buf := make([]byte, 4096)
	defer clear(buf)
	for {
		if done, err := readPTY(fd, buf); done || err != nil {
			return err
		}
	}
}

func readPTY(fd int, buf []byte) (bool, error) {
	for {
		n, err := unix.Read(fd, buf)
		if err == unix.EINTR {
			continue
		}
		if n == 0 || err == unix.EIO {
			return true, nil
		}
		if err != nil {
			return false, fmt.Errorf("read ghtkn output: %w", err)
		}
		return false, nil
	}
}

func waitForChild(pid int) (unix.WaitStatus, error) {
	var ws unix.WaitStatus
	for {
		_, err := unix.Wait4(pid, &ws, 0, nil)
		if err == unix.EINTR {
			continue
		}
		if err != nil {
			return ws, fmt.Errorf("wait for ghtkn: %w", err)
		}
		return ws, nil
	}
}
