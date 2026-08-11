package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"gitee.com/AY77-OP/audio-talk-ai/config"
	"gitee.com/AY77-OP/audio-talk-ai/engine"
	"gitee.com/AY77-OP/audio-talk-ai/hotkey"
	"gitee.com/AY77-OP/audio-talk-ai/internal/doctor"
	"gitee.com/AY77-OP/audio-talk-ai/internal/session"
	"gitee.com/AY77-OP/audio-talk-ai/internal/tui"
	"gitee.com/AY77-OP/audio-talk-ai/internal/webui"
	"gitee.com/AY77-OP/audio-talk-ai/plugins"
	"gitee.com/AY77-OP/audio-talk-ai/plugins/overlay"
	"gitee.com/AY77-OP/audio-talk-ai/plugins/voice"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	backend := flag.String("backend", "", "force backend")
	cfgPath := flag.String("config", "", "path to config file")
	debug := flag.Bool("debug", false, "enable debug plugin")
	verbose := flag.Bool("verbose", false, "verbose logging")
	useTUI := flag.Bool("tui", true, "run with terminal UI")
	noTUI := flag.Bool("no-tui", false, "run without terminal UI")
	doctorOnly := flag.Bool("doctor", false, "run startup doctor and exit")
	installOnly := flag.Bool("install", false, "install audio-talk-ai to ~/.local/bin")
	overlayHelper := flag.Bool("overlay-helper", false, "run macOS overlay helper")
	overlayPosition := flag.String("overlay-position", "top-right", "overlay helper position")
	overlayScale := flag.Float64("overlay-scale", 1.0, "overlay helper scale")
	// Session flags
	dStart := flag.Bool("d", false, "start new detachable TUI session")
	dReattach := flag.Bool("di", false, "reattach to existing session")
	listMode := flag.Bool("list", false, "list active sessions")
	detachName := flag.String("detach", "", "detach a session by name")
	tuiDirect := flag.Bool("tui-direct", false, "internal: run TUI directly (used by --d server)")
	detachMode := flag.Bool("detach-mode", false, "internal: enable 'b' detach key (used by --d server)")
	serverSock := flag.String("server", "", "internal: run as PTY server daemon")
	flag.Parse()

	// Handle --server (daemon mode for PTY server, restart TUI on exit)
	if *serverSock != "" {
		// Write PID file for --detach
		pidFile := *serverSock + ".pid"
		os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", os.Getpid())), 0644)
		// Clean up on SIGTERM
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGTERM)
		go func() {
			<-sigCh
			os.Remove(*serverSock)
			os.Remove(pidFile)
			os.Remove(session.MetaPath(*serverSock))
			os.Exit(0)
		}()

		if err := session.RunServer(*serverSock, []string{os.Args[0], "--tui-direct", "--detach-mode"}, slog.Default()); err != nil {
			os.Exit(1)
		}
		// Child exited (user pressed 'q'), clean up and stop
		os.Remove(*serverSock)
		os.Remove(pidFile)
		os.Remove(session.MetaPath(*serverSock))
		return
	}

	// Handle session commands first (before any other logic)
	if *listMode {
		if err := runListSessions(); err != nil {
			fmt.Fprintf(os.Stderr, "list error: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if *detachName != "" {
		dir, err := session.SessionDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "session dir error: %v\n", err)
			os.Exit(1)
		}
		// Support index numbers (1, 2, ...) from --list
		sockName := *detachName
		var n int
		if _, err := fmt.Sscanf(*detachName, "%d", &n); err == nil && n > 0 {
			sessions, _ := session.AllSessions()
			if n >= 1 && n <= len(sessions) {
				sockName = filepath.Base(sessions[n-1].Sock)
				sockName = strings.TrimSuffix(sockName, ".sock")
			}
		}
		sock := filepath.Join(dir, sockName+".sock")
		session.DetachSession(sock) // best effort
		// Kill daemon via PID file
		pidData, err := os.ReadFile(sock + ".pid")
		if err == nil {
			var pid int
			fmt.Sscanf(string(pidData), "%d", &pid)
			if proc, err := os.FindProcess(pid); err == nil {
				proc.Signal(syscall.SIGTERM)
			}
		}
		os.Remove(sock)
		os.Remove(sock + ".pid")
		os.Remove(session.MetaPath(sock))
		fmt.Println("detached")
		return
	}

	if *installOnly {
		if err := installSelf(); err != nil {
			fmt.Fprintf(os.Stderr, "install failed: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if *overlayHelper {
		if err := overlay.RunHelper(*overlayPosition, *overlayScale, os.Stdin); err != nil {
			fmt.Fprintf(os.Stderr, "overlay helper error: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if *noTUI {
		*useTUI = false
	}

	// --tui-direct: internal flag used by di server, force TUI mode
	if *tuiDirect {
		*useTUI = true
	}

	// --d/--di: detachable session
	if *dStart {
		// Check if a daemon is already running
		dir, _ := session.SessionDir()
		if sessions, _ := session.AllSessions(); len(sessions) > 0 {
			for _, s := range sessions {
				pidFile := s.Sock + ".pid"
				if pidData, err := os.ReadFile(pidFile); err == nil {
					var pid int
					fmt.Sscanf(string(pidData), "%d", &pid)
					if proc, err := os.FindProcess(pid); err == nil && proc.Signal(syscall.Signal(0)) == nil {
						fmt.Fprintf(os.Stderr, "会话已在运行中，使用 audio-talk-ai --di 恢复\n")
						os.Exit(1)
					}
				}
			}
			// Stale sessions found, clean up
			for _, s := range sessions {
				os.Remove(s.Sock)
				os.Remove(s.Sock + ".pid")
				os.Remove(session.MetaPath(s.Sock))
			}
			_ = dir
		}
		if err := runDiMode(); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if *dReattach {
		if err := session.PickAndAttach(); err != nil {
			fmt.Fprintf(os.Stderr, "attach error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Prevent duplicate instances for normal TUI/daemon mode
	lockFile := acquireLock()
	if lockFile == nil {
		fmt.Fprintln(os.Stderr, "audio-talk-ai is already running.")
		os.Exit(1)
	}
	defer releaseLock(lockFile)

	logLevel := slog.LevelInfo
	if *verbose {
		logLevel = slog.LevelDebug
	}

	// Daemon mode: log to stderr + file. TUI mode: file only (stderr corrupts display).
	logPath := filepath.Join(os.TempDir(), "audio-talk-ai.log")
	var logWriter io.Writer
	if *useTUI {
		lf, _ := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if lf != nil {
			logWriter = lf
		} else {
			logWriter = io.Discard
		}
	} else {
		lf, _ := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if lf != nil {
			logWriter = io.MultiWriter(os.Stderr, lf)
		} else {
			logWriter = os.Stderr
		}
	}
	logger := slog.New(slog.NewTextHandler(logWriter, &slog.HandlerOptions{Level: logLevel}))
	slog.SetDefault(logger)

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Migrate any still-plaintext secrets to encrypted-at-rest on first run.
	// The migration is safe-by-construction: it never overwrites the live
	// config unless the encrypted bytes round-trip back to the same plaintext
	// and a backup of the original file exists.
	if config.HasPlaintextSecrets(cfg) {
		if err := config.MigratePlaintextSecrets(*cfgPath, logger); err != nil {
			logger.Error("failed to encrypt config secrets (config left unchanged)", "error", err)
		}
	}

	if *backend == "" {
		*backend = os.Getenv("JUST_TALK_BACKEND")
	}
	if *backend != "" {
		os.Setenv("JUST_TALK_BACKEND", *backend)
	}

	report := doctor.Run(cfg, *backend)
	if *doctorOnly || !report.Healthy() {
		report.Print(os.Stderr)
		if report.Healthy() {
			return
		}
		os.Exit(1)
	}

	provider, err := createProvider(*backend)
	if err != nil {
		logger.Error("failed to create provider", "error", err)
		printTroubleshooting(err)
		os.Exit(1)
	}
	logger.Info("provider created", "platform", provider.Info().Platform, "backend", provider.Info().Backend)

	eng := engine.New(provider, cfg, logger)

	if *debug && cfg.Debug.Enabled && !*useTUI {
		eng.LoadPlugin(plugins.NewDebugPlugin())
	}
	eng.LoadPlugin(voice.NewVoicePlugin())
	eng.LoadPlugin(overlay.NewOverlayPlugin())
	if p := config.FindConfig(); p != "" {
		eng.WatchConfig(p)
	}

	// WebUI and history
	stateDir := filepath.Join(stateDir(), "audio-talk-ai")
	os.MkdirAll(stateDir, 0755)
	historyStore := webui.NewHistoryStore(stateDir)
	voice.SetTranscriptionCallback(func(text, provider string, duration time.Duration) {
		historyStore.Add(webui.HistoryEntry{Text: text, Provider: provider, Duration: duration.Seconds()})
	})
	if cfg.Web.Enabled && cfg.Web.Port > 0 {
		webSrv := webui.NewServer(cfg, eng, historyStore, cfg.Web.Port, logger)
		webSrv.Start()
	}

	if *useTUI {
		runTUI(eng, cfg, *debug, *detachMode)
	} else {
		runDaemon(eng)
	}
}

func runDaemon(eng *engine.Engine) {
	slog.Info("audio-talk-ai started — press hotkeys, Ctrl+C to quit")
	if err := eng.Start(true); err != nil && err != context.Canceled {
		slog.Error("engine exited with error", "error", err)
		os.Exit(1)
	}
}

func runTUI(eng *engine.Engine, cfg *config.Config, debug bool, detach bool) {
	voice.SetupTUILog()
	voice.SetOutput(io.Discard)
	model := tui.New(cfg)
	model.SetDebug(debug)
	model.SetDetach(detach)
	model.OnSave = func(c *config.Config) error { return eng.ReloadConfig(c) }
	go func() {
		if err := eng.Start(false); err != nil && err != context.Canceled {
			slog.Error("engine error", "error", err)
		}
	}()
	go func() { model.Update(tui.SetProviderInfo(eng.Provider().Info())) }()
	p := tea.NewProgram(model, tea.WithAltScreen())
	// Subscribe to config changes from WebUI
	cfgCh := eng.OnConfigChange()
	go func() {
		for newCfg := range cfgCh {
			p.Send(tui.ConfigReloadMsg(newCfg))
		}
	}()
	finalModel, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
		os.Exit(1)
	}

	// Check if user requested background mode
	if m, ok := finalModel.(*tui.Model); ok {
		if m.WantsBackground() {
			fmt.Println("已切换到后台模式，热键继续工作。Ctrl+C 退出。")
			// Wait for engine to finish (Ctrl+C)
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, os.Interrupt)
			<-sigCh
			fmt.Println("\n正在退出...")
		}
	}
	eng.Stop()
}

func createProvider(backend string) (hotkey.Provider, error) {
	if backend == "mock" {
		return hotkey.NewMockProvider(), nil
	}
	return hotkey.NewProvider()
}

func runDiMode() error {
	dir, err := session.SessionDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}

	sock := session.UniqueSocketPath(dir, "audio-talk-ai")
	meta := session.SessionMeta{
		Name:    filepath.Base(sock),
		Command: []string{exe, "--tui-direct", "--detach-mode"},
	}
	if err := session.WriteMeta(sock, meta); err != nil {
		return err
	}

	// Fork a daemon to run the PTY server (detached from terminal)
	devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer devNull.Close()
	cmd := exec.Command(exe, "--server", sock)
	cmd.Stdin = devNull
	cmd.Stdout = devNull
	cmd.Stderr = devNull
	if runtime.GOOS != "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start daemon: %w", err)
	}
	cmd.Process.Release()

	// Wait for socket to be ready
	for i := 0; i < 30; i++ {
		time.Sleep(100 * time.Millisecond)
		if session.IsSocket(sock) {
			break
		}
	}
	if !session.IsSocket(sock) {
		return fmt.Errorf("server did not start")
	}

	fmt.Println("  detach: Ctrl+] or b")
	fmt.Println("  reattach: audio-talk-ai --di")
	// Set env so Attach layer intercepts 'b' as detach
	os.Setenv("AUDIO_TALK_DETACH", "1")
	return session.Attach(sock)
}

func runListSessions() error {
	sessions, err := session.AllSessions()
	if err != nil {
		return err
	}
	if len(sessions) == 0 {
		fmt.Println("no active sessions")
		return nil
	}
	for i, s := range sessions {
		name := s.Meta.Name
		if name == "" {
			name = filepath.Base(s.Sock)
		}
		fmt.Printf("  [%d] %-32s  %s\n", i+1, name, s.Meta.PWD)
	}
	return nil
}

func printTroubleshooting(err error) {
	fmt.Fprintf(os.Stderr, "Error: %v\n\nTroubleshooting:\n", err)
	fmt.Fprintf(os.Stderr, "  X11:      Ensure $DISPLAY is set\n")
	fmt.Fprintf(os.Stderr, "  Wayland:  Add user to 'input' group\n")
	fmt.Fprintf(os.Stderr, "  macOS:    Grant Accessibility permission\n")
	fmt.Fprintf(os.Stderr, "  Windows:  Check microphone privacy settings\n")
}

func installSelf() error {
	targetDir, err := installDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("create %s: %w", targetDir, err)
	}

	src, err := os.Executable()
	if err != nil {
		return fmt.Errorf("find current executable: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(src); err == nil {
		src = resolved
	}
	name := "audio-talk-ai"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	target := filepath.Join(targetDir, name)
	if samePath(src, target) {
		fmt.Fprintf(os.Stdout, "audio-talk-ai is already installed at %s\n", target)
		printInstallPathNote(targetDir)
		return nil
	}

	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open current executable %s: %w", src, err)
	}
	defer in.Close()

	tmp, err := os.CreateTemp(targetDir, ".audio-talk-ai-*")
	if err != nil {
		return fmt.Errorf("create temporary installer file: %w", err)
	}
	tmpName := tmp.Name()
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := io.Copy(tmp, in); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("copy executable: %w", err)
	}
	if err := tmp.Chmod(0755); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("set executable mode: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temporary installer file: %w", err)
	}
	if err := replaceInstalledFile(tmpName, target); err != nil {
		return fmt.Errorf("install to %s: %w", target, err)
	}
	ok = true

	fmt.Fprintf(os.Stdout, "Installed audio-talk-ai to %s\n", target)
	printInstallPathNote(targetDir)
	return nil
}

func installDir() (string, error) {
	if runtime.GOOS == "windows" {
		base := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))
		if base == "" {
			var err error
			base, err = os.UserCacheDir()
			if err != nil {
				return "", fmt.Errorf("find local application data directory: %w", err)
			}
		}
		return filepath.Join(base, "Programs", "audio-talk-ai"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find home directory: %w", err)
	}
	return filepath.Join(home, ".local", "bin"), nil
}

func replaceInstalledFile(source, target string) error {
	if runtime.GOOS != "windows" {
		return os.Rename(source, target)
	}
	// Windows: rename-rename pattern to avoid "file in use" errors
	backup := target + ".old"
	_ = os.Remove(backup)
	hadTarget := false
	if _, err := os.Stat(target); err == nil {
		if err := os.Rename(target, backup); err != nil {
			return err
		}
		hadTarget = true
	}
	if err := os.Rename(source, target); err != nil {
		if hadTarget {
			_ = os.Rename(backup, target)
		}
		return err
	}
	if hadTarget {
		_ = os.Remove(backup)
	}
	return nil
}

func samePath(a, b string) bool {
	absA, errA := filepath.Abs(a)
	absB, errB := filepath.Abs(b)
	if errA == nil {
		a = absA
	}
	if errB == nil {
		b = absB
	}
	if resolved, err := filepath.EvalSymlinks(b); err == nil {
		b = resolved
	}
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func printInstallPathNote(dir string) {
	if !pathContains(dir) {
		fmt.Fprintf(os.Stdout, "Note: %s is not in PATH. Add it to your shell profile to run audio-talk-ai directly.\n", dir)
	}
}

func pathContains(dir string) bool {
	for _, p := range filepath.SplitList(os.Getenv("PATH")) {
		if p == dir {
			return true
		}
		if rel, err := filepath.Rel(p, dir); err == nil && rel == "." {
			return true
		}
		if strings.TrimRight(p, string(os.PathSeparator)) == strings.TrimRight(dir, string(os.PathSeparator)) {
			return true
		}
	}
	return false
}

func stateDir() string {
	if runtime.GOOS == "windows" {
		if dir, err := os.UserCacheDir(); err == nil {
			return dir
		}
		// Fallback to LocalAppData
		if dir := os.Getenv("LOCALAPPDATA"); dir != "" {
			return dir
		}
	}
	if xdg := os.Getenv("XDG_STATE_HOME"); xdg != "" {
		return xdg
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "state")
}

func lockPath() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(os.TempDir(), "audio-talk-ai.lock")
	}
	runtime := os.Getenv("XDG_RUNTIME_DIR")
	if runtime == "" {
		runtime = "/tmp"
	}
	return filepath.Join(runtime, "audio-talk-ai.lock")
}

func acquireLock() *os.File {
	path := lockPath()
	// On Unix, use flock for robust locking. On Windows, use O_EXCL.
	if runtime.GOOS != "windows" {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
		if err != nil {
			return nil
		}
		if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
			f.Close()
			return nil
		}
		return f
	}
	// Windows: O_EXCL provides atomic creation
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if err != nil {
		return nil
	}
	return f
}

func releaseLock(f *os.File) {
	if f != nil {
		path := f.Name()
		if runtime.GOOS != "windows" {
			syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		}
		f.Close()
		os.Remove(path)
	}
}

func processAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}
