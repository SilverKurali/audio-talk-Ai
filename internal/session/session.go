//go:build !windows

package session

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// SessionDir returns the session directory for storing sockets and metadata.
func SessionDir() (string, error) {
	if xdg := os.Getenv("XDG_RUNTIME_DIR"); xdg != "" {
		return filepath.Join(xdg, "audio-talk-ai"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "state", "audio-talk-ai"), nil
}

// SessionMeta stores metadata for a session.
type SessionMeta struct {
	Name      string   `json:"name"`
	Command   []string `json:"command"`
	PWD       string   `json:"pwd"`
	StartedAt string   `json:"started_at"`
}

// SessionInfo combines socket path with metadata.
type SessionInfo struct {
	Sock string
	Meta SessionMeta
}

// DisplayLine returns a formatted line for listing.
func (s SessionInfo) DisplayLine() string {
	name := s.Meta.Name
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(s.Sock), ".sock")
	}
	pwd := s.Meta.PWD
	if pwd == "" {
		pwd = "-"
	}
	cmd := strings.Join(s.Meta.Command, " ")
	if cmd == "" {
		cmd = name
	}
	return fmt.Sprintf("%s\t%-56s\t%s", s.Sock, pwd, cmd)
}

// AllSessions returns all active sessions, cleaning up unreachable ones.
func AllSessions() ([]SessionInfo, error) {
	dir, err := SessionDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var sessions []SessionInfo
	for _, e := range entries {
		path := filepath.Join(dir, e.Name())
		if strings.HasSuffix(e.Name(), ".sock") && IsSocket(path) {
			if !SessionReachable(path) {
				RemoveSessionFiles(path)
				continue
			}
			sessions = append(sessions, SessionInfo{Sock: path, Meta: ReadMeta(path)})
		}
	}
	return sessions, nil
}

// UniqueSocketPath generates a unique socket path.
func UniqueSocketPath(dir, base string) string {
	if base == "" {
		base = "session"
	}
	re := regexp.MustCompile(`[^A-Za-z0-9._+-]+`)
	base = re.ReplaceAllString(base, "-")
	base = strings.Trim(base, "-")
	if base == "" {
		base = "session"
	}
	for i := 0; ; i++ {
		name := fmt.Sprintf("%s-%d-%d", base, time.Now().UnixNano(), os.Getpid())
		if i > 0 {
			name = fmt.Sprintf("%s-%d-%d-%d", base, time.Now().UnixNano(), os.Getpid(), i)
		}
		path := filepath.Join(dir, name+".sock")
		if !IsSocket(path) {
			return path
		}
	}
}

// MetaPath returns the metadata file path for a socket.
func MetaPath(sock string) string {
	return strings.TrimSuffix(sock, ".sock") + ".json"
}

// WriteMeta writes session metadata to disk.
func WriteMeta(sock string, meta SessionMeta) error {
	pwd, _ := os.Getwd()
	meta.PWD = pwd
	meta.StartedAt = time.Now().Format(time.RFC3339)
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(MetaPath(sock), data, 0o600)
}

// ReadMeta reads session metadata from disk.
func ReadMeta(sock string) SessionMeta {
	var meta SessionMeta
	data, err := os.ReadFile(MetaPath(sock))
	if err != nil {
		meta.Name = strings.TrimSuffix(filepath.Base(sock), ".sock")
		return meta
	}
	if err := json.Unmarshal(data, &meta); err != nil {
		meta.Name = strings.TrimSuffix(filepath.Base(sock), ".sock")
	}
	return meta
}

// IsSocket checks if a path is a Unix socket.
func IsSocket(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeSocket != 0
}

// SessionReachable checks if a session's socket is accepting connections.
func SessionReachable(sock string) bool {
	var dialer net.Dialer
	dialer.Timeout = 200 * time.Millisecond
	conn, err := dialer.Dial("unix", sock)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// RemoveSessionFiles removes socket and metadata files.
func RemoveSessionFiles(sock string) {
	_ = os.Remove(sock)
	_ = os.Remove(MetaPath(sock))
}
