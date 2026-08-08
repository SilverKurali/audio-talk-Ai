# AGENTS.md

This file gives coding agents concise guidance for working in this repository.

## Project

Audio Talk AI is a desktop voice input tool. It records with a global hotkey, sends audio to streaming ASR, then copies recognized text to the clipboard or submits it into the focused input field.

The current supported desktop targets are Linux and macOS. Windows is not implemented.

## Build And Test

This project uses native platform APIs and requires cgo for supported desktop builds.

```bash
make build              # Build for the current platform
make run                # Run on the current platform
make test               # Run all tests (uses -v)
go test ./...           # Faster default test command
go test ./... -tags no_x11  # Skip X11 cgo deps (useful on headless or Wayland-only systems)
go test ./asr/...                 # Run tests for a single package
go test ./internal/doctor/... -run TestASR  # Run a single test by name
CGO_ENABLED=1 go build -o build/audio-talk-ai ./cmd/audio-talk-ai
```

Build tags: `no_x11` disables X11 cgo code (hotkeys, overlay, autotype). macOS files use `darwin && cgo`. Windows provider exists but is not a supported target.

Do not add or preserve non-cgo macOS fallback builds. A build that compiles but cannot provide native hotkeys, recording, clipboard, auto-submit, or overlay is worse than an explicit build failure.

Useful runtime commands:

```bash
audio-talk-ai               # TUI mode, default
audio-talk-ai --no-tui      # daemon mode
audio-talk-ai --doctor      # startup environment check
audio-talk-ai --d           # detachable TUI session
audio-talk-ai --di          # reattach to existing session
audio-talk-ai --list        # list active sessions
audio-talk-ai --backend x11
audio-talk-ai --backend wayland
```

`JUST_TALK_BACKEND` or `--backend` can force `x11`, `wayland`, or `darwin`. `mock` exists for internal provider testing but is not part of the normal user path.

## Platform Dependencies

Linux:

- Wayland hotkeys use evdev and require readable `/dev/input/event*`, usually via the `input` group.
- Wayland clipboard and auto-submit use `wl-clipboard` and `wtype`.
- X11 hotkeys and overlay use native X11 through cgo.
- X11 auto-submit uses XTest and clipboard tools.

macOS:

- Global hotkeys use CGEventTap through `ApplicationServices`.
- Recording uses CoreAudio / AudioQueue.
- Clipboard uses NSPasteboard.
- Auto-submit posts native keyboard events.
- Overlay uses an AppKit `NSPanel` helper process.
- Users grant Accessibility and Microphone permissions to the terminal app that launches Audio Talk AI, not to a separate `.app` bundle.
- Full Xcode is not required, but Apple Command Line Tools must provide `clang` and the macOS SDK.

## Architecture

```text
cmd/audio-talk-ai/main.go
  -> config.Load
  -> doctor.Run
  -> hotkey.NewProvider
  -> engine.New
  -> load voice + overlay plugins
  -> TUI or daemon mode
```

Core packages:

- `config/`: loads/saves TOML config, parses hotkey combos, manages ASR provider entries. Config lives at `~/.config/audio-talk-ai/config.toml`; `config.toml.example` documents every provider field.
- `asr/`: provider-agnostic streaming ASR abstraction. `Client` is a session (Connect/SendAudio/Results/Done/Final). `registry.go` registers factories by name (database/sql-style lookup), so `asr.NewClient` is the only entry point — add a provider by calling `asr.Register` from its package init, not by editing callers. Current providers (each in its own subdirectory): `doubao`, `mimoasr`, `openairealtime`, `openaiwhisper`, `xfyuniat`, `xfyunlfasr`, `xfyunrtasr`, `xfyunrtasrstd`, `xfyunspark`.
- `hotkey/`: platform global hotkey providers plus shared combo/event types.
- `engine/`: plugin lifecycle, hotkey registry ownership, and config hot-reload orchestration.
- `plugins/voice/`: recorder, ASR streaming, hotkey behavior, clipboard/auto-submit dispatch, stats.
- `plugins/overlay/`: recording status capsule for Linux and macOS.
- `internal/autotype/`: platform paste/auto-submit implementation.
- `internal/clipboard/`: platform clipboard implementation.
- `internal/doctor/`: startup environment checks.
- `internal/tui/`: Bubble Tea configuration UI.
- `internal/webui/`: HTTP server (port 8391) with `go:embed` static SPA; manages config CRUD, provider CRUD, and transcription history (JSON file in XDG state dir).
- `internal/session/`: detachable TUI sessions (di mode) over a Unix PTY/socket daemon.

## Big Picture

`main.go` parses flags, loads config, runs the doctor, creates the hotkey `Provider`, then builds the `Engine` and loads `voice` + `overlay` plugins (plus `debug` in daemon mode). The Engine owns the `hotkey.Registry`; each plugin registers its hotkeys in `Init()` (never `Start()`) and receives lifecycle via `Start(ctx)`/`Stop()`.

The voice path is the heart of the app: a hotkey event (toggle/hold) flips recording state quickly in the handler, while the actual recorder stop, final audio send, ASR final wait, clipboard write, and auto-submit run as background finish work to keep the hotkey event loop responsive. Recognized text is dispatched exactly once per user-stopped session (guard both `Final()` and `Done()` paths). Transcriptions are also fanned out to the WebUI `HistoryStore` via `voice.SetTranscriptionCallback`.

Configuration changes are hot-reloaded: `Engine.WatchConfig` watches the config directory with fsnotify and, on write, reloads and calls `OnConfigReload` on any plugin implementing the `Reloader` interface, then notifies `OnConfigChange` subscribers (the TUI and WebUI). ASR providers are selected by name from config; the `asr` registry resolves the factory, so switching providers needs no code change in the voice plugin.

Module import path is `gitee.com/AY77-OP/audio-talk-ai` (the repo is mirrored there, not on GitHub's go module path). Go version: `go 1.25.0` in `go.mod`.

Key documentation files under `docs/`:
- `docs/asr-providers-guide.md` — comprehensive ASR provider configuration reference.
- `docs/xfyun-guide.md` — iFlytek service setup guide.
- `docs/knowledge/zh/` — Chinese-language design notes on config encryption, logging, WebUI theme, provider metadata, error handling, Go/Node dual-stack, and build/install. Read these before making changes to sensitive areas.

## Hotkey Notes

`Combo` is `{Mods Modifier, Key KeyCode}`. Modifier-only hotkeys use `KeyNone`, for example `Option+Command` on macOS maps to `ModAlt|ModSuper` with `KeyNone`.

Providers should emit key down/up events promptly. Fast repeated toggle presses and hold-mode release handling are user-visible and have historically been fragile, so avoid changes that add blocking work to provider event loops or hotkey handlers.

Plugin hotkey registration must happen in `Plugin.Init()`, not in `Plugin.Start()`. The registry starts dispatch goroutines after plugin loading; registering late can drop events.

## Voice Pipeline Notes

Stopping a recording is intentionally split from hotkey handling:

- Hotkey handlers update state quickly.
- Recorder stop, final audio send, ASR final wait, clipboard writes, and auto-submit happen in background finish work.
- Debug logs should identify whether a stop is waiting on recorder, final audio send, ASR final, clipboard, or ASR client close.

Avoid double-output bugs: recognized final text should be dispatched once per user-stopped session. If changing ASR result handling, check both `Final()` and `Done()` paths.

## UI And Logging

TUI mode must not write normal logs to stdout/stderr because it corrupts the Bubble Tea layout. TUI logs go through the in-app log/status area; debug event details should only be visible when `--debug` is enabled.

`voice.SetOutput(io.Discard)` is intentional in TUI mode.

## Repository Rules

- Prefer existing package boundaries and platform-specific files with build tags.
- Keep user-facing doctor output short and action-oriented. Do not list implementation details as checks unless the user can act on them.
- README is bilingual: update both `README.md` and `README.en.md`.
- `CHANGELOG.md` should be updated for user-visible behavior changes.
- The project does not accept pull requests; issues are welcome.
- Licensed under GPL v3.0 with a no-commercial-use clause. Avoid adding dependencies or code that would conflict with GPL v3 or the non-commercial restriction.
