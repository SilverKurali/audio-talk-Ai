# Changelog

All notable project changes are tracked here.

## Unreleased

### New

- **Windows 平台支持**：Windows 10/11 现已完整支持，无需 CGO 编译。
  - 全局热键：`WH_KEYBOARD_LL` 低级键盘钩子 + 消息泵，可靠捕获包括 Win 键在内的修饰键组合。
  - 录音：通过 `ffmpeg`（dshow）或 `sox`（waveaudio）子进程采集 PCM 16kHz 单声道音频。
  - 剪贴板：基于 `atotto/clipboard` 库的 Win32 剪贴板 API（原生实现，无命令行回退）。
  - 自动上屏：先写入剪贴板，再用 `SendInput` 模拟 Ctrl+V 粘贴，正确处理 32/64 位 INPUT 结构体布局。
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
- Audio input device selection in WebUI and TUI (Windows/Linux/macOS). A new `GET /api/devices/audio` endpoint enumerates input devices via the platform backend (ffmpeg dshow / arecord / CoreAudio); the WebUI voice-settings card and the TUI config form now have a device dropdown (with "系统默认" and a manual free-text fallback when enumeration fails), a refresh button / key (`r` in TUI) for hot-plugged USB mics, and the chosen device is persisted to the existing `voice.device` config field.
- Fixed Windows `voice.ListDevices()` swallowing the ffmpeg-missing error: it now returns a real error so the UI can prompt "请安装 ffmpeg" instead of showing an empty dropdown.
- Press-the-combo hotkey capture in WebUI and TUI. The WebUI hotkey row now has a "⚡ 按键捕获" button: click it, then press the combo you want (e.g. Ctrl+Shift+F9) and it is captured, validated, and filled in automatically — no more typing `"Ctrl+Shift+F9"` by hand. The TUI gets the same via a new `c` key on the hotkey field. Plain modifier-only combos (e.g. Ctrl+Alt) are unreliable to capture in the browser/terminal; the UI detects this and guides you to the preset dropdown / manual entry. A new `GET /api/hotkey/validate` endpoint powers live validation.
- Fixed Windows single-instance lock wedging the app after a crash/kill. The old lock was a plain `O_EXCL` file with no owner tracking, so any non-graceful exit (crash, `Stop-Process`, `os.Exit`) left an orphan `.lock` in `%TEMP%` and every subsequent launch falsely reported "already running". The lock now records the owner's PID; a stale lock whose PID is no longer running (verified via `OpenProcess`/`GetExitCodeProcess`) is reclaimed automatically.
- Fixed Windows auto-submit destroying the user's clipboard. Unlike the Linux X11 path, the Windows `pastePlatform` overwrote the clipboard with the transcribed text and never restored the prior contents. It now saves the original clipboard, pastes, and restores the original after a short settle delay — matching the Linux behavior.
- Fixed macOS auto-submit destroying the user's clipboard (same class of bug as Windows). The macOS `pastePlatform` (`autotype_darwin.go`) now saves the prior pasteboard contents before writing the transcribed text and restores them after Cmd+V settles, consistent with Linux and Windows.
- Fixed X11 modifier-only hotkey combos (e.g. Alt+Super, Ctrl+Alt) re-firing on keyboard auto-repeat (typematic). The old `pressedKeys` heuristic only suppressed repeats for non-modifier keys, so holding a modifier-only combo flipped toggle/hold state on every repeat tick — the same class of bug Windows had. The X11 provider now uses event timestamps to detect repeat bursts (X11 synthesizes repeat KeyRelease+KeyPress pairs with identical `time`), suppressing the whole burst including modifiers.
- Hardened Linux auto-submit clipboard handling: the X11 path no longer silently swallows `xclip` read failures (it logs them and skips restore instead of leaving the prior clipboard silently overwritten), and `xclip primary` `Start()` failure no longer nil-panics on cleanup. The Wayland path (`pasteWayland`) now also saves/restores the prior clipboard via `wl-paste`, matching X11/Windows/macOS.

### Fixed (Linux)

- Linux hotkey backend detection now matches the rest of the stack. `hotkey.NewProvider` previously checked only `XDG_SESSION_TYPE` when deciding between Wayland and X11, while doctor / autotype / clipboard also honored `WAYLAND_DISPLAY`. On sessions where `WAYLAND_DISPLAY` is set but `XDG_SESSION_TYPE` is empty (custom launchers, some embedded compositors), the hotkey backend picked X11 while everything else picked Wayland, so the hotkey silently failed to fire. `NewProvider` now uses the same `WAYLAND_DISPLAY || XDG_SESSION_TYPE == "wayland"` test.
- Linux X11 auto-submit (`pasteX11`) now restores the prior clipboard even when the simulated paste (XTest Shift+Insert) fails. Previously the failure path returned early and left the user's clipboard overwritten with the transcribed text; it now best-effort restores the saved contents and logs the restore error if any.
- Linux Wayland auto-submit (`pasteWayland`) no longer silently ignores clipboard-restore failures; restore errors are now logged at debug level.
- Linux doctor no longer reports a misleading "input 组存在" message when `LookupGroup` returns a non-numeric Gid (LDAP/NIS). It now reports the concrete Gid and, when the user is not in the `input` group, says so explicitly.
- Linux doctor under `-tags no_x11` no longer asks the user to install `xclip`. When the session is X11 but the X11 backend was compiled out, doctor now reports that the X11 backend is disabled at build time and recommends switching to Wayland or rebuilding without `no_x11`.
- Linux Wayland clipboard detection now falls back to X11 tools when running under XWayland (`DISPLAY` set). A Wayland session lacking `wl-clipboard` but having `xclip` previously ended up with no working clipboard; now both candidates are tried.
- Linux Wayland hotkey provider no longer opens every `/dev/input/event*` node. `findKeyboardDevices` now probes each device with `EVIOCGBIT(EV_KEY)` and keeps only nodes that advertise a real keyboard key (KEY_BACKSPACE). On a typical laptop this drops the open-FD count from ~24 (mouse, touchpad, accelerometer, etc.) to the handful of actual keyboards, eliminating spurious "cannot open device" warnings and wasted FDs.

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
- Fixed Windows toggle mode flipping recording state wildly on every key press and hold mode never stopping recording after releasing the hotkey.
  - The Windows hotkey provider was re-architected from `WH_KEYBOARD_LL` event-driven to **5 ms `GetAsyncKeyState` polling as the source of truth, with the low-level hook demoted to a modifier-only arbiter**. The hook is no longer trusted to deliver `KeyUp` reliably; instead the provider computes combo active/inactive state each tick and emits the corresponding edge event. This makes auto-repeat (typematic) inherently a no-op (a held key just reads "still pressed"), which is the root cause of the toggle-mode flip storm (every stop with `remaining_bytes=0`) and of the hold-mode "can't stop" symptom.
  - Also corrected a fatal constant: `llkhfUp` was `0x8000`, but `KBDLLHOOKSTRUCT.flags` carries `LLKHF_UP` in bit 7 (`0x0080`). The wrong value made the hook misclassify nearly every key release as a key press — the most direct cause of "KeyUp never fires" on Windows.
  - Added left/right modifier VK codes (`vkLControl`/`vkRControl`/`vkLMenu`/`vkRMenu`/`vkLShift`/`vkRShift`), F13–F24, and PrintScreen/ScrollLock/Pause/NumLock/numpad operators so the hook layer recognizes the full key set.
  - Stale hook modifier state (e.g., a lost Super key-up) is reconciled against physical `GetAsyncKeyState` after a 50 ms lag or when another modifier is pressed, preventing modifier-only combos (Alt+Super) from getting stuck or falsely activating.
  - Backport of the upstream fix shipped in the `just-talk-go` lineage (commits `223b7b8`, `d174747`, `c476659`).
- `KeyStateTracker.KeyDown` now ignores auto-repeat of an already-held key on all platforms (macOS CGEventTap, Linux X11/Wayland). Windows no longer uses the tracker, but this prevents the same typematic re-fire bug on the other desktop platforms.
- Fixed key-only hotkeys (e.g. F9) firing two `KeyDown` events per physical press in `KeyStateTracker.KeyDown`: the standard-combo scan path no longer picks up `Mods == ModNone` combos, leaving key-only combos to the dedicated solo path.

## 2026-05-30

- Initial Linux-focused development snapshot.
- Linux Wayland hotkeys via evdev.
- Linux X11 hotkeys via native X11 grabs.
- Doubao streaming ASR integration.
- TUI configuration interface.
- Automatic clipboard copy and auto-submit.
