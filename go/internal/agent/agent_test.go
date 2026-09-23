package agent

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func response(ok bool, locked *bool, version, minimum int) Response {
	return Response{OK: ok, Locked: locked, ProtocolVersion: &version, MinProtocolVersion: &minimum}
}

func ptr[T any](v T) *T { return &v }

func TestUnlockRequestEncoding(t *testing.T) {
	var object map[string]any
	if err := json.Unmarshal(unlockRequest([]byte("pass\"\\\nphrase\x01")), &object); err != nil {
		t.Fatal(err)
	}
	if object["command"] != "UNLOCK" || object["passphrase"] != "pass\"\\\nphrase\x01" ||
		object["protocol_version"] != 1.0 || object["refresh_token_ttl"] != 604800000000000.0 {
		t.Fatalf("unexpected request: %v", object)
	}
}

func TestAlreadyUnlockedSkipsKeychain(t *testing.T) {
	_, err := Unlock(
		func() ([][]byte, error) { t.Fatal("loaded Keychain"); return nil, nil },
		func(request []byte) (Response, error) {
			if !bytes.Equal(request, StatusRequest()) {
				t.Fatal("status must come first")
			}
			return response(true, ptr(false), 1, 0), nil
		})
	if err != nil {
		t.Fatal(err)
	}
}

func TestProtocolMismatchSkipsKeychain(t *testing.T) {
	_, err := Unlock(
		func() ([][]byte, error) { t.Fatal("loaded Keychain"); return nil, nil },
		func([]byte) (Response, error) { return response(true, ptr(true), 2, 2), nil })
	if err == nil || err.Error() != "unsupported ghtkn agent protocol" {
		t.Fatalf("got %v", err)
	}
}

func TestCandidateFallback(t *testing.T) {
	var attempted []string
	calls := 0
	_, err := Unlock(
		func() ([][]byte, error) { return [][]byte{[]byte("first"), []byte("second")}, nil },
		func(request []byte) (Response, error) {
			calls++
			if calls == 1 {
				return response(true, ptr(true), 1, 0), nil
			}
			var object map[string]any
			json.Unmarshal(request, &object)
			attempted = append(attempted, object["passphrase"].(string))
			return Response{OK: calls == 3}, nil
		})
	if err != nil || strings.Join(attempted, ",") != "first,second" {
		t.Fatalf("err=%v attempted=%v", err, attempted)
	}
}

func TestRejectedPassphraseIsRedacted(t *testing.T) {
	secret := "do-not-print-this-passphrase"
	calls := 0
	_, err := Unlock(
		func() ([][]byte, error) { return [][]byte{[]byte(secret)}, nil },
		func([]byte) (Response, error) {
			calls++
			if calls == 1 {
				return response(true, ptr(true), 1, 0), nil
			}
			return Response{}, nil
		})
	if err == nil || strings.Contains(err.Error(), secret) || !strings.Contains(err.Error(), "agent rejected") {
		t.Fatalf("got %v", err)
	}
	if calls != 2 {
		t.Fatalf("calls=%d", calls)
	}
}

func TestSocketBounds(t *testing.T) {
	t.Setenv("GHTKN_AGENT_SOCKET", strings.Repeat("a", 1024))
	if _, err := Send(StatusRequest()); err == nil || !strings.Contains(err.Error(), "path is too long") {
		t.Fatalf("got %v", err)
	}
	if _, err := readLine(bytes.NewReader(bytes.Repeat([]byte("a"), 1<<20+1))); err == nil ||
		!strings.Contains(err.Error(), "response is too large") {
		t.Fatalf("got %v", err)
	}
}
