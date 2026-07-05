# Audio Talk AI

[中文](README.md)

Type less, speak more.

Audio Talk AI is a desktop voice input tool. It records audio with a global hotkey, sends it to a streaming ASR service, and copies the recognized text to the clipboard or submits it directly into the focused input field. Ideal for coding, chatting, note-taking, and long text input.

Supports multiple ASR providers with one-click switching in the TUI.

## Features

- Global hotkey recording with `toggle` (press to start, press again to stop) and `hold` (press and speak) modes.
- 5 ASR providers with dynamic switching in TUI:
  - **Doubao** (ByteDance Volcengine) — streaming ASR, real-time partial results
  - **OpenAI Realtime** — streaming ASR via WebSocket
  - **OpenAI Whisper** — batch transcription, compatible with Ollama and other OpenAI-format services
  - **iFlytek Spark** — streaming ASR with dynamic correction and 202 dialects
  - **Xiaomi MiMo** — batch transcription, Chinese/English and dialect support
- Automatic clipboard copy and auto-submit into focused input field.
- Always-on-top recording status overlay for Wayland, X11, and macOS.
- TUI configuration for hotkeys, mode, ASR provider, auto-submit, stop delay, hotwords, and more.
- ASR hotwords for project names, people names, English terms, and domain-specific vocabulary.
- Usage statistics: total sessions, total characters, average speed, and recent speed.

## Platform Status

| Platform | Status | Notes |
| --- | --- | --- |
| Linux Wayland | Supported | Hotkeys via evdev (requires input group); clipboard via wl-clipboard, auto-submit via wtype or uinput |
| Linux X11 | Supported | Native X11 global hotkeys and XTest auto-submit |
| macOS | Supported | Hotkeys via CGEventTap, recording via CoreAudio, clipboard via NSPasteboard |
| Windows | Not implemented | Not supported yet |

## Build

Audio Talk AI uses native platform APIs and requires cgo.

```bash
# Linux (Debian / Ubuntu)
sudo apt install golang-go build-essential libx11-dev libxtst-dev libxext-dev libxinerama-dev libwayland-dev

# Linux (Arch)
sudo pacman -S --needed go gcc libx11 libxtst libxext wayland

# macOS
xcode-select --install
```

```bash
git clone https://gitee.com/AY77-OP/audio-talk-ai.git
cd audio-talk-ai
make build          # or go build -o build/audio-talk-ai ./cmd/audio-talk-ai
make install        # installs to ~/.local/bin/
```

## Usage

```bash
audio-talk-ai              # TUI mode (default)
audio-talk-ai --no-tui     # daemon mode
audio-talk-ai --doctor     # environment check
audio-talk-ai --backend wayland   # force Wayland
audio-talk-ai --backend x11      # force X11
```

### Detachable Session (di mode)

TUI mode supports detach/reattach. Press `Ctrl-]` to detach — Audio Talk AI keeps running in the background (hotkeys still work). Reattach later to see the TUI again. Requires `fzf` for session selection.

```bash
audio-talk-ai --di         # start a detachable TUI session
audio-talk-ai --attach     # reattach (fzf picker)
audio-talk-ai --list       # list active sessions
audio-talk-ai --detach <name>  # detach a session from another terminal
```

## Configuration

Config file: `~/.config/audio-talk-ai/config.toml`

### Basic Config

```toml
[voice]
mode = "toggle"
push_to_talk = "F9"              # default F9; Alt+Super and other combos also work
language = "zh-CN"
auto_submit = true               # auto-submit; false = clipboard only
# hotwords = ["project-name", "person-name", "term"]
```

### ASR Provider Config

You can configure multiple providers and switch in TUI. Without `[[asr_providers]]`, you can put `app_key` / `access_key` directly in `[voice]` (backward compatible).

```toml
# Doubao ASR (streaming, recommended)
[[asr_providers]]
name = "doubao"
type = "doubao"
default = true
app_key = "your_app_key"
access_key = "your_access_key"

# OpenAI Realtime (streaming)
# [[asr_providers]]
# name = "openai"
# type = "openai-realtime"
# api_key = "sk-..."
# model = "gpt-4o-mini-transcribe"

# OpenAI Whisper (batch, compatible with Ollama etc.)
# [[asr_providers]]
# name = "whisper"
# type = "openai-whisper"
# api_key = "sk-..."
# model = "whisper-1"
# endpoint = "http://localhost:11434/v1/audio/transcriptions"

# iFlytek Spark (streaming, dialect support)
# [[asr_providers]]
# name = "xfyun"
# type = "xfyun-spark"
# app_id = "your_app_id"
# api_key = "your_api_key"
# api_secret = "your_api_secret"

# Xiaomi MiMo (batch)
# [[asr_providers]]
# name = "mimo"
# type = "mimo-asr"
# api_key = "your_mimo_api_key"
```

### Hotkey Notes

Voice hotkeys only support keys suitable for global shortcuts:

- Supported: `Alt+Super`, `Ctrl+Alt+Shift`, `F9`, `Alt+F8`, `Tab`, `CapsLock`, etc.
- Not supported: letters, digits, punctuation, Space, and other text-producing keys

macOS hotkey syntax: `Option` = Alt, `Command`/`Cmd` = Super

```toml
push_to_talk = "Option+Command"
```

## Hotkeys

| Hotkey | Action |
|--------|--------|
| Recording hotkey | Start/stop recording |
| `Esc` | Cancel current recording |
| `R` | Retry last recognition error |

## Changelog

See [CHANGELOG.md](CHANGELOG.md).

## Credits

This project is based on [github.com/c/just-talk-go](https://github.com/c/just-talk-go). Thanks to the original author for the open-source contribution.

## License

Audio Talk AI is licensed under the GNU General Public License v3.0.

**No commercial use.** This project is for learning and personal use only. Any commercial use is prohibited.
