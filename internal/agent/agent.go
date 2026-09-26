// Package agent speaks protocol v1 of ghtkn's newline-delimited JSON agent protocol.
package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/unix"
)

const protocolVersion = 1

type Response struct {
	OK                  bool  `json:"ok"`
	Locked              *bool `json:"locked"`
	RefreshTokenEnabled *bool `json:"refresh_token_enabled"`
	ProtocolVersion     *int  `json:"protocol_version"`
	MinProtocolVersion  *int  `json:"min_protocol_version"`
}

type Result struct {
	AlreadyUnlocked     bool
	RefreshTokenEnabled bool
}

// Unlock loads candidate passphrases only when the agent is locked, tries them in
// order, and zeroes them afterwards. Agent errors are not relayed because an
// untrusted socket could reflect a passphrase.
func Unlock(load func() ([][]byte, error), send func([]byte) (Response, error)) (Result, error) {
	status, err := send(StatusRequest())
	if err != nil {
		return Result{}, err
	}
	if !status.OK {
		return Result{}, errors.New("query the ghtkn agent: request rejected")
	}
	if err := checkProtocol(status); err != nil {
		return Result{}, err
	}
	if status.Locked == nil || !*status.Locked {
		enabled := status.RefreshTokenEnabled != nil && *status.RefreshTokenEnabled
		return Result{AlreadyUnlocked: true, RefreshTokenEnabled: enabled}, nil
	}

	candidates, err := load()
	if err != nil {
		return Result{}, err
	}
	defer func() {
		for _, c := range candidates {
			clear(c)
		}
	}()

	for _, c := range candidates {
		request := unlockRequest(c)
		response, err := send(request)
		clear(request)
		if err != nil {
			return Result{}, err
		}
		if response.OK {
			return Result{RefreshTokenEnabled: true}, nil
		}
	}
	return Result{}, errors.New("unlock the ghtkn agent: agent rejected the passphrase")
}

func checkProtocol(r Response) error {
	minimum := 0
	if r.MinProtocolVersion != nil {
		minimum = *r.MinProtocolVersion
	}
	if r.ProtocolVersion == nil || minimum > protocolVersion || protocolVersion > *r.ProtocolVersion {
		return errors.New("unsupported ghtkn agent protocol")
	}
	return nil
}

func StatusRequest() []byte {
	return []byte(`{"command":"STATUS","protocol_version":1}` + "\n")
}

func unlockRequest(passphrase []byte) []byte {
	data := make([]byte, 0, 128+len(passphrase)*6)
	data = append(data, `{"command":"UNLOCK","protocol_version":1,"passphrase":`...)
	data = appendJSONString(data, passphrase)
	return append(data, `,"enable_refresh_token":true,"refresh_token_ttl":604800000000000}`+"\n"...)
}

func appendJSONString(data, value []byte) []byte {
	const hex = "0123456789abcdef"
	data = append(data, '"')
	for _, b := range value {
		switch b {
		case '"', '\\':
			data = append(data, '\\', b)
		case '\b':
			data = append(data, '\\', 'b')
		case '\f':
			data = append(data, '\\', 'f')
		case '\n':
			data = append(data, '\\', 'n')
		case '\r':
			data = append(data, '\\', 'r')
		case '\t':
			data = append(data, '\\', 't')
		default:
			if b < 0x20 {
				data = append(data, '\\', 'u', '0', '0', hex[b>>4], hex[b&0x0f])
			} else {
				data = append(data, b)
			}
		}
	}
	return append(data, '"')
}

// SocketPath follows ghtkn's lookup order.
func SocketPath() string {
	if path := os.Getenv("GHTKN_AGENT_SOCKET"); path != "" {
		return path
	}
	if path := os.Getenv("XDG_RUNTIME_DIR"); path != "" {
		return filepath.Join(path, "ghtkn/agent.sock")
	}
	if path := os.Getenv("XDG_CACHE_HOME"); path != "" {
		return filepath.Join(path, "ghtkn/agent.sock")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cache/ghtkn/agent.sock")
}

func Send(request []byte) (Response, error) {
	var response Response
	path := SocketPath()
	if len(path) >= len(unix.RawSockaddrUnix{}.Path) {
		return response, fmt.Errorf("ghtkn agent socket path is too long: %s", path)
	}
	conn, err := net.DialTimeout("unix", path, 10*time.Second)
	if err != nil {
		return response, fmt.Errorf("connect to the ghtkn agent: %w", errors.Unwrap(err))
	}
	defer func() { _ = conn.Close() }()
	if err := conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return response, fmt.Errorf("configure the ghtkn agent socket: %w", err)
	}
	if err := checkPeer(conn.(*net.UnixConn)); err != nil {
		return response, err
	}
	if _, err := conn.Write(request); err != nil {
		return response, fmt.Errorf("write to the ghtkn agent: %w", errors.Unwrap(err))
	}
	line, err := readLine(conn)
	defer clear(line)
	if err != nil {
		return response, err
	}
	if err := json.Unmarshal(line, &response); err != nil {
		return response, fmt.Errorf("decode the ghtkn agent response: %w", err)
	}
	return response, nil
}

func checkPeer(conn *net.UnixConn) error {
	raw, err := conn.SyscallConn()
	if err != nil {
		return err
	}
	var cred *unix.Xucred
	var credErr error
	if err := raw.Control(func(fd uintptr) {
		cred, credErr = unix.GetsockoptXucred(int(fd), unix.SOL_LOCAL, unix.LOCAL_PEERCRED)
	}); err != nil {
		return err
	}
	if credErr != nil || int(cred.Uid) != os.Geteuid() {
		return errors.New("the ghtkn agent socket is not owned by the current user")
	}
	return nil
}

func readLine(r io.Reader) ([]byte, error) {
	var line []byte
	b := make([]byte, 1)
	for len(line) < 1<<20 {
		n, err := r.Read(b)
		if n == 1 {
			if b[0] == '\n' {
				return line, nil
			}
			line = append(line, b[0])
			continue
		}
		if err == io.EOF {
			return line, nil
		}
		if err != nil {
			clear(line)
			return nil, fmt.Errorf("read from the ghtkn agent: %w", err)
		}
	}
	clear(line)
	return nil, errors.New("the ghtkn agent response is too large")
}
