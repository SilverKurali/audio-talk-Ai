# Changelog

All notable project changes are tracked here.

## Unreleased

### New

- Multi-provider ASR system: support 5 speech recognition services with dynamic switching in TUI.
  - **Doubao** (ByteDance Volcengine): streaming ASR via binary WebSocket protocol.
  - **OpenAI Realtime**: streaming ASR via JSON WebSocket, supports gpt-4o-transcribe / gpt-4o-mini-transcribe / whisper-1.
  - **OpenAI Whisper**: batch transcription via REST, compatible with Ollama, vLLM, and other OpenAI-format services.
  - **iFlytek Spark** (讯飞星火): streaming ASR via JSON WebSocket, supports dynamic correction (wpgs) and 202 dialects.
  - **Xiaomi MiMo**: batch transcription via REST, supports Chinese/English and dialects.
- ASR provider registry with self-registration via `init()` (similar to `database/sql` drivers).
- TUI dynamically shows provider-specific credential fields when switching providers.
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

## 2026-05-30

- Initial Linux-focused development snapshot.
- Linux Wayland hotkeys via evdev.
- Linux X11 hotkeys via native X11 grabs.
- Doubao streaming ASR integration.
- TUI configuration interface.
- Automatic clipboard copy and auto-submit.
