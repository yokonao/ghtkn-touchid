package main

import (
	"errors"
	"strings"
	"testing"
)

func TestResetRecoveryBoundary(t *testing.T) {
	committed := false
	commit := func() error { committed = true; return nil }
	if err := performReset(func() (int, error) { return 23, nil }, commit); err == nil ||
		!strings.Contains(err.Error(), "kept the pending passphrase") {
		t.Fatalf("got %v", err)
	}
	if err := performReset(func() (int, error) { return 0, errors.New("child setup") }, commit); err == nil ||
		!strings.Contains(err.Error(), "kept the pending passphrase") {
		t.Fatalf("got %v", err)
	}
	if committed {
		t.Fatal("failed reset committed")
	}
	if err := performReset(func() (int, error) { return 0, nil }, commit); err != nil || !committed {
		t.Fatalf("err=%v committed=%t", err, committed)
	}
}
