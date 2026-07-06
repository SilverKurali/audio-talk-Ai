# Changelog

All notable project changes are tracked here.

## Unreleased

### New

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

- Batch ASR providers (Whisper, MiMo) now trigger API request on `SendAudio(isLast=true)` instead of `Close()`, fixing a 15-second timeout hang and potential result loss.
- TUI switching providers now auto-saves to disk, preventing loss of unsaved credential edits.
- TUI `save()` writes current provider fields back before copying config, ensuring no edits are missed.
- TUI switching providers clears add-provider preview, avoiding empty fields overlaying saved credentials.
- Hotkey double-fire for key-only combos (e.g. F9) fixed in KeyStateTracker.
- Evdev multi-device duplicate key events deduplicated with 2ms window.
- Ctrl+] detach now works in GNOME Terminal (CSI u 3-part format support).
- WebUI config field name casing fixed (lowercase API response).
- WebUI save voice config no longer overwrites provider entries.

## 2026-05-30

- Initial Linux-focused development snapshot.
- Linux Wayland hotkeys via evdev.
- Linux X11 hotkeys via native X11 grabs.
- Doubao streaming ASR integration.
- TUI configuration interface.
- Automatic clipboard copy and auto-submit.
