package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
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
	// Session (di) flags
	diMode := flag.Bool("di", false, "start in detachable TUI session")
	attachMode := flag.Bool("attach", false, "attach to existing session (fzf)")
	listMode := flag.Bool("list", false, "list active sessions")
	detachName := flag.String("detach", "", "detach a session by name")
	tuiDirect := flag.Bool("tui-direct", false, "internal: run TUI directly (used by --di server)")
	flag.Parse()

	// Handle session commands first (before any other logic)
	if *diMode {
		if err := runDiMode(); err != nil {
			fmt.Fprintf(os.Stderr, "di error: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if *attachMode {
		if err := session.PickAndAttach(); err != nil {
			fmt.Fprintf(os.Stderr, "attach error: %v\n", err)
			os.Exit(1)
		}
		return
	}
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
		sock := filepath.Join(dir, *detachName+".sock")
		if err := session.DetachSession(sock); err != nil {
			fmt.Fprintf(os.Stderr, "detach error: %v\n", err)
			os.Exit(1)
		}
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

	logLevel := slog.LevelInfo
	if *verbose {
		logLevel = slog.LevelDebug
	}

	// Daemon mode: log to stderr + file. TUI mode: file only (stderr corrupts display).
	var logWriter io.Writer
	if *useTUI {
		lf, _ := os.OpenFile("/tmp/audio-talk-ai.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if lf != nil {
			logWriter = lf
		} else {
			logWriter = io.Discard
		}
	} else {
		lf, _ := os.OpenFile("/tmp/audio-talk-ai.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
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
		runTUI(eng, cfg, *debug)
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

func runTUI(eng *engine.Engine, cfg *config.Config, debug bool) {
	voice.SetupTUILog()
	voice.SetOutput(io.Discard)
	model := tui.New(cfg)
	model.SetDebug(debug)
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

	// Find our own executable path
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
		Command: []string{exe, "--tui-direct"},
	}
	if err := session.WriteMeta(sock, meta); err != nil {
		return err
	}

	fmt.Printf("starting detachable session: %s\n", filepath.Base(sock))
	fmt.Printf("  socket: %s\n", sock)
	fmt.Printf("  detach: Ctrl-]\n")
	fmt.Printf("  reattach: audio-talk-ai --attach\n")
	fmt.Println()

	return session.RunServer(sock, []string{exe, "--tui-direct"}, slog.Default())
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
	for _, s := range sessions {
		name := s.Meta.Name
		if name == "" {
			name = filepath.Base(s.Sock)
		}
		fmt.Printf("%-32s  %s\n", name, s.Meta.PWD)
	}
	return nil
}

func printTroubleshooting(err error) {
	fmt.Fprintf(os.Stderr, "Error: %v\n\nTroubleshooting:\n", err)
	fmt.Fprintf(os.Stderr, "  X11:      Ensure $DISPLAY is set\n")
	fmt.Fprintf(os.Stderr, "  Wayland:  Add user to 'input' group\n")
	fmt.Fprintf(os.Stderr, "  macOS:    Grant Accessibility permission\n")
}

func installSelf() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("find home directory: %w", err)
	}
	targetDir := filepath.Join(home, ".local", "bin")
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
	target := filepath.Join(targetDir, "audio-talk-ai")
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
	if err := os.Rename(tmpName, target); err != nil {
		return fmt.Errorf("install to %s: %w", target, err)
	}
	ok = true

	fmt.Fprintf(os.Stdout, "Installed audio-talk-ai to %s\n", target)
	printInstallPathNote(targetDir)
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
	if xdg := os.Getenv("XDG_STATE_HOME"); xdg != "" {
		return xdg
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "state")
}
