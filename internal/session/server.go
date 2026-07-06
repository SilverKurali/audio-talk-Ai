package session

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
)

const (
	frameInput     = 'i'
	frameResize    = 'w'
	frameDetachAll = 'D'
)

// Server manages a PTY session with multiple attached clients.
type Server struct {
	master  *os.File
	clients map[net.Conn]struct{}
	history []byte
	mu      sync.Mutex
}

func (s *Server) add(conn net.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clients[conn] = struct{}{}
	if len(s.history) > 0 {
		_, _ = conn.Write(s.history)
	}
}

func (s *Server) remove(conn net.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.clients, conn)
	_ = conn.Close()
}

func (s *Server) closeClients() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for conn := range s.clients {
		_ = conn.Close()
		delete(s.clients, conn)
	}
}

func (s *Server) broadcastPTY() {
	buf := make([]byte, 4096)
	for {
		n, err := s.master.Read(buf)
		if n > 0 {
			s.mu.Lock()
			s.appendHistory(buf[:n])
			for conn := range s.clients {
				if _, werr := conn.Write(buf[:n]); werr != nil {
					_ = conn.Close()
					delete(s.clients, conn)
				}
			}
			s.mu.Unlock()
		}
		if err != nil {
			s.closeClients()
			return
		}
	}
}

func (s *Server) appendHistory(p []byte) {
	const maxHistory = 64 << 10 // 64KB
	s.history = append(s.history, p...)
	if len(s.history) > maxHistory {
		s.history = append([]byte(nil), s.history[len(s.history)-maxHistory:]...)
	}
}

func (s *Server) handle(conn net.Conn) {
	defer s.remove(conn)
	for {
		typ, payload, err := readFrame(conn)
		if err != nil {
			return
		}
		switch typ {
		case frameInput:
			_, _ = s.master.Write(payload)
		case frameResize:
			if len(payload) == 8 {
				rows := binary.BigEndian.Uint32(payload[:4])
				cols := binary.BigEndian.Uint32(payload[4:])
				_ = pty.Setsize(s.master, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
			}
		case frameDetachAll:
			s.closeClients()
			return
		}
	}
}

// RunServer starts a PTY server that forks the given command.
// The command runs in a PTY; clients connect via Unix socket to interact with it.
func RunServer(sock string, cmdArgs []string, logger *slog.Logger) error {
	return runServerOnce(sock, cmdArgs, logger)
}

// RunServerWithRestart keeps the listener alive and restarts the child process when it exits.
// The listener is only closed when the socket file is removed externally (e.g. user pressed 'q').
func RunServerWithRestart(sock string, cmdArgs []string, logger *slog.Logger) error {
	if len(cmdArgs) == 0 {
		return errors.New("session: no command specified")
	}

	if err := os.MkdirAll(filepath.Dir(sock), 0o700); err != nil {
		return err
	}
	_ = os.Remove(sock)

	ln, err := net.Listen("unix", sock)
	if err != nil {
		return err
	}
	defer ln.Close()

	exe, err := exec.LookPath(cmdArgs[0])
	if err != nil {
		exe = cmdArgs[0]
	}

	for {
		// Check if socket was removed externally (user pressed 'q')
		if !IsSocket(sock) {
			logger.Info("socket removed, stopping daemon")
			return nil
		}

		logger.Info("starting child process", "command", cmdArgs)
		cmd := exec.Command(exe, cmdArgs[1:]...)
		cmd.Env = append(os.Environ(), "AUDIO_TALK_SOCK="+sock)
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}

		ptmx, err := pty.Start(cmd)
		if err != nil {
			logger.Error("start PTY failed", "error", err)
			return fmt.Errorf("session: start PTY: %w", err)
		}

		server := &Server{master: ptmx, clients: map[net.Conn]struct{}{}}
		go server.broadcastPTY()

		// Accept clients in background
		acceptDone := make(chan struct{})
		go func() {
			for {
				conn, err := ln.Accept()
				if err != nil {
					close(acceptDone)
					return
				}
				server.add(conn)
				go server.handle(conn)
			}
		}()

		// Wait for child to exit (don't close listener!)
		cmd.Wait()
		ptmx.Close()
		server.closeClients()
		logger.Info("child process exited, restarting...")

		// Brief pause before restart
		time.Sleep(200 * time.Millisecond)
	}
}

func runServerOnce(sock string, cmdArgs []string, logger *slog.Logger) error {
	if len(cmdArgs) == 0 {
		return errors.New("session: no command specified")
	}

	if err := os.MkdirAll(filepath.Dir(sock), 0o700); err != nil {
		return err
	}
	_ = os.Remove(sock)

	ln, err := net.Listen("unix", sock)
	if err != nil {
		return err
	}
	defer ln.Close()

	// Find the executable
	exe, err := exec.LookPath(cmdArgs[0])
	if err != nil {
		// Try as absolute path
		exe = cmdArgs[0]
	}

	cmd := exec.Command(exe, cmdArgs[1:]...)
	cmd.Env = append(os.Environ(), "AUDIO_TALK_SOCK="+sock)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return fmt.Errorf("session: start PTY: %w", err)
	}
	defer ptmx.Close()

	server := &Server{master: ptmx, clients: map[net.Conn]struct{}{}}
	go server.broadcastPTY()
	go func() {
		_ = cmd.Wait()
		_ = ln.Close()
		_ = ptmx.Close()
	}()

	logger.Info("session server started", "socket", sock, "command", cmdArgs)

	for {
		conn, err := ln.Accept()
		if err != nil {
			return nil // listener closed, session ended
		}
		server.add(conn)
		go server.handle(conn)
	}
}

// readFrame reads a framed message from the connection.
func readFrame(r io.Reader) (byte, []byte, error) {
	var header [5]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return 0, nil, err
	}
	n := binary.BigEndian.Uint32(header[1:])
	if n > 1<<20 { // 1MB max
		return 0, nil, errors.New("frame too large")
	}
	payload := make([]byte, n)
	if _, err := io.ReadFull(r, payload); err != nil {
		return 0, nil, err
	}
	return header[0], payload, nil
}

// writeFrame writes a framed message to the connection.
func writeFrame(w io.Writer, typ byte, payload []byte) error {
	var header [5]byte
	header[0] = typ
	binary.BigEndian.PutUint32(header[1:], uint32(len(payload)))
	if _, err := w.Write(header[:]); err != nil {
		return err
	}
	_, err := w.Write(payload)
	return err
}
