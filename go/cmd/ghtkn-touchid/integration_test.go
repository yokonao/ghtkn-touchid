//go:build integration

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestGhtknIntegration(t *testing.T) {
	ghtkn, err := ghtknExecutable()
	if err != nil {
		t.Fatal(err)
	}
	// AF_UNIX socket paths are capped at ~104 bytes, which t.TempDir() under
	// /var/folders already eats most of.
	root, err := os.MkdirTemp("/tmp", "ghtkn-touchid-it.")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)
	socket := filepath.Join(root, "agent.sock")
	t.Setenv("GHTKN_AGENT_SOCKET", socket)
	t.Setenv("GHTKN_AGENT_KEY", filepath.Join(root, "key"))
	t.Setenv("GHTKN_AGENT_TOKEN_DIR", filepath.Join(root, "tokens"))

	passphrase := []byte("ghtkn-touchid-integration")
	if status, err := runGhtknReset(ghtkn, passphrase); err != nil || status != 0 {
		t.Fatalf("ghtkn agent reset: status=%d err=%v", status, err)
	}

	agent := exec.Command(ghtkn, "agent", "start")
	if err := agent.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		agent.Process.Kill()
		agent.Wait()
	}()
	for i := 0; ; i++ {
		if _, err := os.Stat(socket); err == nil {
			break
		}
		if i == 100 {
			t.Fatal("the ghtkn agent socket never appeared")
		}
		time.Sleep(50 * time.Millisecond)
	}

	status, err := send(statusRequest())
	if err != nil || !status.OK || status.Locked == nil || !*status.Locked {
		t.Fatalf("a freshly reset ghtkn agent was not locked: %+v %v", status, err)
	}
	err = unlock(
		func() ([]storedPassphrase, error) {
			return []storedPassphrase{{"integration", append([]byte(nil), passphrase...)}}, nil
		},
		send)
	if err != nil {
		t.Fatal(err)
	}
	status, err = send(statusRequest())
	if err != nil || !status.OK || (status.Locked != nil && *status.Locked) {
		t.Fatalf("the ghtkn agent was still locked after unlock: %+v %v", status, err)
	}
}
