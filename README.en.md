# Audio Talk AI

[中文](README.md)

Audio Talk AI is a desktop voice input tool. It records audio with a global hotkey, sends it to streaming ASR, and then copies the recognized text to the clipboard or submits it directly into the focused input field.

It is built for people who want to type less and speak more while coding, chatting, writing notes, or working with long text.

## Screenshot

![Audio Talk AI TUI](docs/screenshot-tui.png)

## Features

- Global hotkey recording with `toggle` and `hold` modes.
- Voice hotkeys are limited to keys suitable for global shortcuts: modifiers, function keys, Tab, CapsLock, arrow/navigation keys, and similar non-text keys. Letters, digits, punctuation, Space, and other text-producing keys are rejected.
- Doubao streaming ASR with optimized bidirectional streaming and second-pass recognition.
- Clipboard copy and automatic text submission.
- Always-on-top recording status overlay for Wayland, X11, and macOS.
- TUI configuration for hotkeys, mode, auto-submit, stop delay, hotwords, and related settings.
- ASR hotwords for project names, people names, English terms, and domain-specific vocabulary.
- Usage statistics for total sessions, total recognized characters, average speed, and recent speed.

## Platform Status

The current development focus is Linux and macOS desktop support:

| Platform | Status | Notes |
| --- | --- | --- |
| Linux Wayland | Supported | Works with Sway / wlroots; hotkeys use evdev and require input permissions |
| Linux X11 | Supported | Uses native X11 global hotkeys |
| macOS | Supported | Global hotkeys use CGEventTap, recording uses CoreAudio, clipboard uses NSPasteboard, and overlay uses AppKit NSPanel |
| Windows | Not implemented | Not supported yet |

## Build

Audio Talk AI uses native platform APIs, so builds require cgo.

Linux build dependencies:

```bash
# Arch Linux
sudo pacman -S --needed go gcc libx11 libxtst libxext wayland

# Debian / Ubuntu
sudo apt install golang-go build-essential libx11-dev libxtst-dev libxext-dev libxinerama-dev libwayland-dev
```

macOS build dependencies:

```bash
# Apple Command Line Tools provide clang and the macOS SDK. Full Xcode is not required.
xcode-select --install
```

Build for the current platform:

```bash
cd audio-talk-ai
CGO_ENABLED=1 go build -o build/audio-talk-ai ./cmd/audio-talk-ai
```

Install to `~/.local/bin/audio-talk-ai`:

```bash
# Make sure ~/.local/bin is in PATH. If not, add this line to ~/.bashrc or ~/.zshrc.
# export PATH="$HOME/.local/bin:$PATH"
build/audio-talk-ai --install
# or
make install
```

macOS must be built on macOS. The project does not provide a non-cgo build.

## Usage

Start the TUI:

```bash
audio-talk-ai
```

Run without the TUI:

```bash
audio-talk-ai --no-tui
```

Force a backend:

```bash
audio-talk-ai --backend wayland
audio-talk-ai --backend x11
```

## Configuration

Default config path:

```text
~/.config/audio-talk-ai/config.toml
```

Recommended hotkey config:

```toml
[voice]
mode = "toggle"
push_to_talk = "Alt+Super"
```

`Alt+Super` with `toggle` mode is recommended. Press once to start recording, then press again to stop. This avoids hold-mode key conflicts with desktop environments or focused input fields.

Voice hotkeys only support keys suitable for global shortcuts:

- Supported: modifier-only combinations, such as `Alt+Super` and `Ctrl+Alt+Shift`.
- Supported: function keys `F1` through `F24`, such as `F9` and `Alt+F8`.
- Supported: non-text control and navigation keys, such as `Tab`, `Enter`, `Escape`, `Backspace`, `CapsLock`, `Up`, `Down`, `Left`, `Right`, `Home`, `End`, `PageUp`, `PageDown`, `Insert`, and `Delete`.
- Not supported: letters, digits, punctuation, Space, numpad digits, and numpad symbols that can enter text, such as `Alt+G`, `G`, `Alt+1`, and `Alt+Space`.

Hotword example:

```toml
[voice]
hotwords = ["Wayland", "Sway", "wl-copy", "wtype", "audio-talk-ai"]
```

macOS hotkey example:

```toml
[voice]
# Option is Alt; Command/Cmd is Super.
push_to_talk = "Option+Command"
```

## Changelog

See [CHANGELOG.md](CHANGELOG.md).

## Maintenance And Contributions

Audio Talk AI is maintained by `whoamihappyhacking`.

This project does not accept pull requests. Issues are welcome for bug reports, usage feedback, and feature discussion.

## License

Audio Talk AI is licensed under the GNU General Public License v3.0.
