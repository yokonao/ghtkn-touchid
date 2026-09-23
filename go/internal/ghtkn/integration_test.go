//go:build integration

package ghtkn_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/yokonao/ghtkn-touchid/go/internal/agent"
	"github.com/yokonao/ghtkn-touchid/go/internal/ghtkn"
)

func TestGhtknIntegration(t *testing.T) {
	path, err := ghtkn.Executable()
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
	if status, err := ghtkn.ResetAgent(path, passphrase); err != nil || status != 0 {
		t.Fatalf("ghtkn agent reset: status=%d err=%v", status, err)
	}

	agentCmd := exec.Command(path, "agent", "start")
	if err := agentCmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		agentCmd.Process.Kill()
		agentCmd.Wait()
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

	status, err := agent.Send(agent.StatusRequest())
	if err != nil || !status.OK || status.Locked == nil || !*status.Locked {
		t.Fatalf("a freshly reset ghtkn agent was not locked: %+v %v", status, err)
	}
	_, err = agent.Unlock(
		func() ([][]byte, error) { return [][]byte{append([]byte(nil), passphrase...)}, nil },
		agent.Send)
	if err != nil {
		t.Fatal(err)
	}
	status, err = agent.Send(agent.StatusRequest())
	if err != nil || !status.OK || (status.Locked != nil && *status.Locked) {
		t.Fatalf("the ghtkn agent was still locked after unlock: %+v %v", status, err)
	}
}
