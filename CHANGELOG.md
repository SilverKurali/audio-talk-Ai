# Changelog

All notable project changes are tracked here.

## Unreleased

### New

- **Windows 平台支持**：Windows 10/11 现已完整支持，无需 CGO 编译。
  - 全局热键：`WH_KEYBOARD_LL` 低级键盘钩子 + `GetAsyncKeyState` 轮询双机制，可靠捕获包括 Win 键在内的修饰键组合。
  - 录音：通过 `ffmpeg`（dshow）或 `sox`（waveaudio）子进程采集 PCM 16kHz 单声道音频。
  - 剪贴板：基于 `atotto/clipboard` 库的 Win32 剪贴板 API。
  - 自动上屏：`SendInput` 模拟 Ctrl+V 粘贴，正确处理 32/64 位 INPUT 结构体布局。
  - 叠加层：原生 Win32 分层窗口（`WS_EX_LAYERED` + `UpdateLayeredWindow`），支持圆角矩形、阴影、波形动画和文本渲染，3 倍超采样抗锯齿。
  - 诊断：检查麦克风设备（`waveInGetNumDevs`）、录音后端（ffmpeg/sox）和 ASR 配置。
  - 配置路径使用 `%APPDATA%/audio-talk-ai/`，状态目录使用 `%LOCALAPPDATA%/audio-talk-ai/`，日志写入 `%TEMP%/audio-talk-ai.log`。
  - 单实例锁基于 `O_CREATE|O_EXCL` 原子文件创建（Windows 无 `flock`）。
  - `--install` 安装到 `%LOCALAPPDATA%\Programs\audio-talk-ai\`，使用 rename-backup 模式避免 "file in use" 错误。
  - 可分离会话模式（`--d`/`--di`）在 Windows 上不可用，调用时返回明确的错误提示。
- At-rest encryption for API secrets in `config.toml`: plaintext credentials are now encrypted with AES-256-GCM on first run and stored as `enc:<base64>`; a per-user key file (`~/.config/audio-talk-ai/key`, mode 0600) is used for decryption. Migration is safe: the original config is backed up and the encrypted bytes are verified to round-trip before any file is overwritten.
- Doctor `密钥加密` check reports plaintext-secrets warnings, missing/over-permissive key files, and overall at-rest safety.
- macOS recording device selection: the `device` config field is now honored on macOS via CoreAudio; `ListDevices` enumerates real audio input devices (previously it always recorded from the default device).
- Multi-provider ASR system: support 6 speech recognition services with dynamic switching in TUI.
  - **Doubao** (ByteDance Volcengine): streaming ASR via binary WebSocket protocol.
  - **OpenAI Realtime**: streaming ASR via JSON WebSocket, supports gpt-4o-transcribe / gpt-4o-mini-transcribe / whisper-1.
  - **OpenAI Whisper**: batch transcription via REST, compatible with Ollama, vLLM, and other OpenAI-format services.
  - **iFlytek Spark** (讯飞星火): streaming ASR via JSON WebSocket, supports dynamic correction (wpgs) and 202 dialects.
  - **Xiaomi MiMo**: batch transcription via REST, supports Chinese/English and dialects.
  - **Xiaomi MiMo Token Plan**: batch transcription, domestic (China) endpoint.
- ASR provider registry with self-registration via `init()` (similar to `database/sql` drivers).
- Web management UI at `http://localhost:8391` for configuring providers, voice settings, and viewing transcription history.
- Transcription history persistence (`~/.local/state/audio-talk-ai/history.json`).
- Detachable session mode (`--d`/`--di`/`--list`/`--detach`), `Ctrl+]` or `b` to detach, `q` to end session.
- TUI and WebUI real-time sync: WebUI config changes auto-refresh TUI display.
- Single-instance lock for normal TUI/daemon mode.
- TUI provider management: add, remove, and switch ASR providers with dynamic credential fields.
- TUI select fields show `(e 切换)` hint to indicate interactivity.
- Visual separator between provider config and add-provider sections in TUI.
- iFlytek Spark DWA (dynamic correction) changed from free-text to yes/no select.
- Default hotkey changed from `Alt+Super` to `F9`.
- Added `config.toml.example` with all provider configurations.

### Changed

- Project renamed from "Just Talk" to "Audio Talk AI".
- Module path changed from `github.com/c/just-talk-go` to `gitee.com/AY77-OP/audio-talk-ai`.
- Config binary, log files, and state directories renamed from `just-talk` to `audio-talk-ai`.
- ASR client code moved from `plugins/voice/asr.go` to `asr/doubao/client.go`.
- Doctor check now validates provider-specific required fields per provider type.
- README rewritten with multi-provider configuration documentation.

### Fixed

- Fixed `openai-whisper` custom endpoint being ignored: the client now falls back to the `base_url` key written by TUI/WebUI forms (previously only a hand-written `endpoint` key in config.toml took effect, so endpoints entered through the UI were silently ignored and requests went to the default OpenAI API).
- Fixed `lang` setting for `xfyun-rtasr` / `xfyun-rtasr-std` being ignored: the clients now accept both `lang` and `language` keys (config emits the typed `lang` field as `language`, so non-default languages like `en` / `autominor` were previously silently dropped).
- Batch ASR providers (Whisper, MiMo) now trigger API request on `SendAudio(isLast=true)` instead of `Close()`, fixing a 15-second timeout hang and potential result loss.
- TUI switching providers now auto-saves to disk, preventing loss of unsaved credential edits.
- TUI `save()` writes current provider fields back before copying config, ensuring no edits are missed.
- TUI switching providers clears add-provider preview, avoiding empty fields overlaying saved credentials.
- Hotkey double-fire for key-only combos (e.g. F9) fixed in KeyStateTracker.
- macOS `--doctor` now actually checks microphone permission via AVFoundation (previously it always reported "可用").
- macOS overlay helper log moved from `/tmp/` to `~/Library/Logs/audio-talk-ai/overlay.log`.
- macOS CGEventTap lock ordering fixed (provider mutex before tracker mutex) to avoid a potential deadlock during hotkey (un)registration.
- Evdev multi-device duplicate key events deduplicated with 2ms window.
- Ctrl+] detach now works in GNOME Terminal (CSI u 3-part format support).
- WebUI config field name casing fixed (lowercase API response).
- WebUI save voice config no longer overwrites provider entries.
- iFlytek Spark ASR complete fix:
  - Switched from `coder/websocket` to `gorilla/websocket` — iFlytek server silently ignored frames from `coder/websocket` (likely due to permessage-deflate negotiation); gorilla/websocket works correctly.
  - Audio volume reduced by ÷3 before sending — microphone produces saturated/clipped PCM (constant ±32768), which iFlytek's VAD cannot detect as speech. MiMo handles this via deep learning, but iFlytek requires cleaner audio.
  - Dynamic correction (dwa=wpgs) parsing fixed: `pgs` and `rg` fields are inside the base64-decoded text JSON, not in the outer response JSON. Now uses a segment map (`sn` → text) to correctly handle `apd` (append) and `rpl` (replace) operations, preventing duplicate text accumulation.
  - Empty results (status=2 with no text) now correctly signal `Final()` channel, preventing 15-second timeout.

## 2026-05-30

- Initial Linux-focused development snapshot.
- Linux Wayland hotkeys via evdev.
- Linux X11 hotkeys via native X11 grabs.
- Doubao streaming ASR integration.
- TUI configuration interface.
- Automatic clipboard copy and auto-submit.
