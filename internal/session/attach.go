//go:build !windows

package session

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"golang.org/x/term"
)

const defaultDetachKey = "^]"

// Attach connects to a session socket and enters raw terminal mode.
func Attach(sock string) error {
	var conn net.Conn
	var err error
	for i := 0; i < 10; i++ {
		conn, err = dialSession(sock)
		if err == nil {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if err != nil {
		RemoveSessionFiles(sock)
		return fmt.Errorf("connect to session: %w", err)
	}
	defer conn.Close()

	if !term.IsTerminal(0) {
		return fmt.Errorf("stdin is not a terminal")
	}
	oldTerm, err := term.MakeRaw(0)
	if err != nil {
		return err
	}
	defer restoreTerm(oldTerm)

	_ = sendWindowSize(conn)
	go watchWindowSize(conn)

	done := make(chan struct{})
	firstData := make(chan struct{})
	go func() {
		buf := make([]byte, 4096)
		first := true
		for {
			n, err := conn.Read(buf)
			if n > 0 {
				if first {
					close(firstData)
					first = false
				}
				os.Stdout.Write(buf[:n])
			}
			if err != nil {
				close(done)
				return
			}
		}
	}()

	inputErr := make(chan error, 1)
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := os.Stdin.Read(buf)
			if n > 0 {
				chunk := buf[:n]
				if isDetachInput(chunk) {
					inputErr <- nil
					return
				}
				if isMouseWheelInput(chunk) {
					continue
				}
				if werr := writeFrame(conn, frameInput, chunk); werr != nil {
					inputErr <- werr
					return
				}
			}
			if err != nil {
				inputErr <- err
				return
			}
		}
	}()

	// Wait for first data with timeout (prevents hanging on dead sessions)
	select {
	case <-firstData:
		// Got data, continue normal flow
	case <-time.After(5 * time.Second):
		return fmt.Errorf("连接会话超时，可能 TUI 还没启动")
	case <-done:
		return nil
	}

	select {
	case <-done:
		return nil
	case err := <-inputErr:
		if err == nil {
			fmt.Print("\x1b[H\x1b[2J") // clear screen
		}
		return err
	}
}

// PickAndAttach uses fzf to select a session and attach to it.
// If only one session exists, attaches directly without fzf.
func PickAndAttach() error {
	sessions, err := AllSessions()
	if err != nil {
		return err
	}
	if len(sessions) == 0 {
		return errors.New("no active sessions found")
	}
	// Single session: attach directly
	if len(sessions) == 1 {
		return Attach(sessions[0].Sock)
	}
	// Multiple sessions: use fzf
	if _, err := exec.LookPath("fzf"); err != nil {
		// No fzf, attach to first session
		return Attach(sessions[0].Sock)
	}
	lines := make([]string, 0, len(sessions))
	for _, s := range sessions {
		lines = append(lines, s.DisplayLine())
	}

	cmd := exec.Command("fzf", "--prompt=audio-talk-ai> ", "--height=40%", "--reverse", "--delimiter=\t", "--with-nth=2..")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	go func() {
		defer stdin.Close()
		for _, line := range lines {
			fmt.Fprintln(stdin, line)
		}
	}()
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 130 {
			return nil // user cancelled
		}
		return err
	}
	selected := strings.TrimSpace(string(out))
	if selected == "" {
		return nil
	}
	fields := strings.Split(selected, "\t")
	if len(fields) == 0 || fields[0] == "" {
		return nil
	}
	return Attach(fields[0])
}

// DetachSession sends a detach-all frame to a session.
func DetachSession(sock string) error {
	conn, err := dialSession(sock)
	if err != nil {
		RemoveSessionFiles(sock)
		return err
	}
	defer conn.Close()
	return writeFrame(conn, frameDetachAll, nil)
}

func dialSession(sock string) (net.Conn, error) {
	var dialer net.Dialer
	dialer.Timeout = 200 * time.Millisecond
	return dialer.Dial("unix", sock)
}

func restoreTerm(state *term.State) {
	_ = term.Restore(0, state)
	fmt.Print("\x1b[?25h\x1b[0m")
}

func sendWindowSize(w io.Writer) error {
	width, height, err := term.GetSize(0)
	if err != nil {
		return nil
	}
	payload := make([]byte, 8)
	binary.BigEndian.PutUint32(payload[:4], uint32(height))
	binary.BigEndian.PutUint32(payload[4:], uint32(width))
	return writeFrame(w, frameResize, payload)
}

func watchWindowSize(w io.Writer) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH)
	for range ch {
		_ = sendWindowSize(w)
	}
}

func isDetachInput(buf []byte) bool {
	key := detachKeyByte()
	if len(buf) == 1 && buf[0] == key {
		return true
	}
	// In --d mode, 'b' also triggers detach (same as Ctrl+])
	if os.Getenv("AUDIO_TALK_DETACH") == "1" && len(buf) == 1 && buf[0] == 'b' {
		return true
	}
	return enhancedDetachInput(buf, key)
}

func detachKeyByte() byte {
	key := os.Getenv("D_DETACH")
	if key == "" {
		key = defaultDetachKey
	}
	if len(key) >= 2 && key[0] == '^' {
		if key[1] == '?' {
			return 0x7f
		}
		return key[1] & 0x1f
	}
	return key[0]
}

func enhancedDetachInput(buf []byte, ctrl byte) bool {
	if len(buf) < 6 || buf[0] != 0x1b || buf[1] != '[' {
		return false
	}
	key := ctrlToKeyCode(ctrl)
	s := string(buf[2:])
	var code, mod int
	if strings.HasSuffix(s, "u") {
		s = strings.TrimSuffix(s, "u")
		parts := strings.Split(s, ";")
		switch len(parts) {
		case 2:
			if _, err := fmt.Sscanf(parts[0]+";"+parts[1], "%d;%d", &code, &mod); err == nil {
				return ctrlModifier(mod) && code == key
			}
		case 3:
			var base int
			if _, err := fmt.Sscanf(parts[0]+";"+parts[1]+";"+parts[2], "%d;%d;%d", &base, &mod, &code); err == nil {
				return ctrlModifier(mod) && code == key
			}
		}
	}
	if strings.HasSuffix(s, "~") {
		s = strings.TrimSuffix(s, "~")
		parts := strings.Split(s, ";")
		switch len(parts) {
		case 2:
			if _, err := fmt.Sscanf(parts[0]+";"+parts[1], "%d;%d", &code, &mod); err == nil {
				return ctrlModifier(mod) && code == key
			}
		case 3:
			var base int
			if _, err := fmt.Sscanf(parts[0]+";"+parts[1]+";"+parts[2], "%d;%d;%d", &base, &mod, &code); err == nil {
				return ctrlModifier(mod) && code == key
			}
		}
	}
	return false
}

func isMouseWheelInput(buf []byte) bool {
	if len(buf) < 6 || buf[0] != 0x1b || buf[1] != '[' {
		return false
	}
	last := buf[len(buf)-1]
	if last == 'M' || last == 'm' {
		if len(buf) >= 9 && buf[2] == '<' {
			return true // SGR mouse
		}
	}
	return false
}

func ctrlToKeyCode(ctrl byte) int {
	if ctrl >= 1 && ctrl <= 26 {
		return int('a' + ctrl - 1)
	}
	switch ctrl {
	case 28:
		return '\\'
	case 29:
		return ']'
	case 30:
		return '^'
	case 31:
		return '_'
	default:
		return int(ctrl)
	}
}

func ctrlModifier(mod int) bool {
	return mod > 1 && ((mod-1)&4) != 0
}
