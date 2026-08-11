//go:build windows

package session

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

// SessionMeta contains metadata about a detachable session.
type SessionMeta struct {
	Name    string
	Command []string
	PWD     string
}

// SessionInfo represents a discovered session.
type SessionInfo struct {
	Sock string
	Meta SessionMeta
}

// DisplayLine returns a human-readable description of the session.
func (s SessionInfo) DisplayLine() string {
	return fmt.Sprintf("%s (%s)", s.Meta.Name, s.Meta.PWD)
}

// SessionDir returns the directory where session sockets are stored.
func SessionDir() (string, error) {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		return "", fmt.Errorf("session mode is not supported on Windows")
	}
	dir := filepath.Join(base, "audio-talk-ai", "sessions")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	return dir, nil
}

// AllSessions returns a list of active sessions.
func AllSessions() ([]SessionInfo, error) {
	return nil, nil
}

// UniqueSocketPath generates a unique socket path in the given directory.
func UniqueSocketPath(dir, base string) string {
	return filepath.Join(dir, base+".sock")
}

// MetaPath returns the metadata file path for a session socket.
func MetaPath(sock string) string {
	return sock + ".meta"
}

// WriteMeta writes session metadata to disk.
func WriteMeta(sock string, meta SessionMeta) error {
	return fmt.Errorf("session mode is not supported on Windows")
}

// ReadMeta reads session metadata from disk.
func ReadMeta(sock string) SessionMeta {
	return SessionMeta{}
}

// IsSocket checks if a path is a socket.
func IsSocket(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// SessionReachable checks if a session socket is reachable.
func SessionReachable(sock string) bool {
	return IsSocket(sock)
}

// RemoveSessionFiles removes session socket and metadata files.
func RemoveSessionFiles(sock string) {
	os.Remove(sock)
	os.Remove(MetaPath(sock))
}

// Attach connects to a session.
func Attach(sock string) error {
	return fmt.Errorf("session mode is not supported on Windows")
}

// PickAndAttach allows the user to pick a session to attach to.
func PickAndAttach() error {
	return fmt.Errorf("session mode is not supported on Windows")
}

// DetachSession detaches a session.
func DetachSession(sock string) error {
	return fmt.Errorf("session mode is not supported on Windows")
}

// RunServer starts a PTY server session.
func RunServer(sock string, cmdArgs []string, logger *slog.Logger) error {
	return fmt.Errorf("session mode is not supported on Windows")
}

// RunServerWithRestart starts a PTY server session with auto-restart.
func RunServerWithRestart(sock string, cmdArgs []string, logger *slog.Logger) error {
	return fmt.Errorf("session mode is not supported on Windows")
}