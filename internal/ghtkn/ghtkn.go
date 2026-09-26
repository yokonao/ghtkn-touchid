// Package ghtkn drives the ghtkn CLI.
package ghtkn

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/creack/pty"
	"golang.org/x/sys/unix"
)

// Executable resolves ghtkn from absolute PATH entries only.
func Executable() (string, error) {
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

// ResetAgent runs `ghtkn agent reset` in a PTY, confirms it, and types the
// passphrase twice only once the terminal echo is off. It returns the exit code.
func ResetAgent(ghtkn string, passphrase []byte) (int, error) {
	cmd := exec.Command(ghtkn, "agent", "reset")
	master, err := pty.Start(cmd)
	if err != nil {
		return 0, fmt.Errorf("start ghtkn: %w", err)
	}
	defer func() { _ = master.Close() }()
	fd := int(master.Fd())
	pid := cmd.Process.Pid

	status, err := feedPassphrase(fd, pid, passphrase)
	if err != nil {
		_ = unix.Kill(pid, unix.SIGTERM)
		// A session leader blocks in its exit path until the terminal output queue is
		// drained, so read the rest of it before waiting for the child.
		_ = drainPTY(fd)
		_, _ = waitForChild(pid)
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
