package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func response(ok bool, locked *bool, version, minimum int) agentResponse {
	return agentResponse{OK: ok, Locked: locked, ProtocolVersion: &version, MinProtocolVersion: &minimum}
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
	err := unlock(
		func() ([]storedPassphrase, error) { t.Fatal("loaded Keychain"); return nil, nil },
		func(request []byte) (agentResponse, error) {
			if !bytes.Equal(request, statusRequest()) {
				t.Fatal("status must come first")
			}
			return response(true, ptr(false), 1, 0), nil
		})
	if err != nil {
		t.Fatal(err)
	}
}

func TestProtocolMismatchSkipsKeychain(t *testing.T) {
	err := unlock(
		func() ([]storedPassphrase, error) { t.Fatal("loaded Keychain"); return nil, nil },
		func([]byte) (agentResponse, error) { return response(true, ptr(true), 2, 2), nil })
	if err == nil || err.Error() != "unsupported ghtkn agent protocol" {
		t.Fatalf("got %v", err)
	}
}

func TestCandidateFallback(t *testing.T) {
	var attempted []string
	calls := 0
	err := unlock(
		func() ([]storedPassphrase, error) {
			return []storedPassphrase{{"committed", []byte("first")}, {"active", []byte("second")}}, nil
		},
		func(request []byte) (agentResponse, error) {
			calls++
			if calls == 1 {
				return response(true, ptr(true), 1, 0), nil
			}
			var object map[string]any
			json.Unmarshal(request, &object)
			attempted = append(attempted, object["passphrase"].(string))
			return agentResponse{OK: calls == 3}, nil
		})
	if err != nil || strings.Join(attempted, ",") != "first,second" {
		t.Fatalf("err=%v attempted=%v", err, attempted)
	}
}

func TestRejectedPassphraseIsRedacted(t *testing.T) {
	secret := "do-not-print-this-passphrase"
	calls := 0
	err := unlock(
		func() ([]storedPassphrase, error) { return []storedPassphrase{{"active", []byte(secret)}}, nil },
		func([]byte) (agentResponse, error) {
			calls++
			if calls == 1 {
				return response(true, ptr(true), 1, 0), nil
			}
			return agentResponse{}, nil
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
	if _, err := send(statusRequest()); err == nil || !strings.Contains(err.Error(), "path is too long") {
		t.Fatalf("got %v", err)
	}
	if _, err := readLine(bytes.NewReader(bytes.Repeat([]byte("a"), 1<<20+1))); err == nil ||
		!strings.Contains(err.Error(), "response is too large") {
		t.Fatalf("got %v", err)
	}
}
