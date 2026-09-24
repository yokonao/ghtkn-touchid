//go:build integration

package ghtkn_test

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/yokonao/ghtkn-touchid/internal/agent"
	"github.com/yokonao/ghtkn-touchid/internal/ghtkn"
)

// download fetches a ghtkn release into the user cache and returns its directory.
func download(version string) (string, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(cache, "ghtkn-touchid", "ghtkn-"+version)
	if _, err := os.Stat(filepath.Join(dir, "ghtkn")); err == nil {
		return dir, nil
	}
	url := fmt.Sprintf("https://github.com/suzuki-shunsuke/ghtkn/releases/download/%s/ghtkn_darwin_%s.tar.gz", version, runtime.GOARCH)
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return "", err
	}
	archive := tar.NewReader(gz)
	for {
		header, err := archive.Next()
		if err == io.EOF {
			return "", errors.New("ghtkn is missing from " + url)
		}
		if err != nil {
			return "", err
		}
		if header.Name != "ghtkn" {
			continue
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", err
		}
		tmp, err := os.CreateTemp(dir, "ghtkn.")
		if err != nil {
			return "", err
		}
		defer os.Remove(tmp.Name())
		if _, err := io.Copy(tmp, archive); err != nil {
			tmp.Close()
			return "", err
		}
		if err := tmp.Close(); err != nil {
			return "", err
		}
		if err := os.Chmod(tmp.Name(), 0o755); err != nil {
			return "", err
		}
		return dir, os.Rename(tmp.Name(), filepath.Join(dir, "ghtkn"))
	}
}

func TestGhtknIntegration(t *testing.T) {
	if version := os.Getenv("GHTKN_VERSION"); version != "" {
		dir, err := download(version)
		if err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	}
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
