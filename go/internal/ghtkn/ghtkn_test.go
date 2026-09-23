package ghtkn

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeExecutable(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("#!/bin/bash\n"+body+"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
}

func TestGhtknLookup(t *testing.T) {
	dir := t.TempDir()
	executable := filepath.Join(dir, "ghtkn")
	writeExecutable(t, executable, "exit 0")

	t.Setenv("PATH", ".:"+dir)
	if resolved, err := Executable(); err != nil || resolved != executable {
		t.Fatalf("resolved=%q err=%v", resolved, err)
	}
	t.Setenv("PATH", ".:")
	if _, err := Executable(); err == nil || !strings.Contains(err.Error(), "absolute PATH") {
		t.Fatalf("got %v", err)
	}
}

func TestPTY(t *testing.T) {
	dir := t.TempDir()
	secret := []byte("pty-only-secret")

	success := filepath.Join(dir, "success.sh")
	writeExecutable(t, success, `test "$#" -eq 2 || exit 10
test "$1" = agent && test "$2" = reset || exit 10
printf 'confirmation text deliberately differs: '
IFS= read -r answer
test "$answer" = y || exit 11
stty -echo
printf 'first secret: '
IFS= read -r first
printf 'second secret: '
IFS= read -r second
stty echo
test "$first" = "$second"`)
	if status, err := ResetAgent(success, secret); err != nil || status != 0 {
		t.Fatalf("status=%d err=%v", status, err)
	}

	echo := filepath.Join(dir, "echo.sh")
	received := filepath.Join(dir, "received")
	writeExecutable(t, echo, `printf 'confirmation: '
IFS= read -r answer
printf 'echo remains enabled: '
IFS= read -r value
printf '%s' "$value" > '`+received+`'`)
	if _, err := ResetAgent(echo, secret); err == nil || !strings.Contains(err.Error(), "echo remained enabled") {
		t.Fatalf("got %v", err)
	}
	if _, err := os.Stat(received); err == nil {
		t.Fatal("passphrase sent with echo enabled")
	}

	abnormal := filepath.Join(dir, "abnormal.sh")
	writeExecutable(t, abnormal, `printf 'confirmation: '
IFS= read -r answer
kill -TERM $$`)
	if status, err := ResetAgent(abnormal, secret); err != nil || status != 143 {
		t.Fatalf("status=%d err=%v", status, err)
	}
}
