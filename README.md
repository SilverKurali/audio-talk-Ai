# Audio Talk AI

[English](README.en.md)

减少用键盘的次数，改用口喷吧。

Audio Talk AI 是一个面向桌面环境的语音输入工具。它通过全局快捷键录音，把语音识别结果复制到剪贴板，或直接上屏到当前输入框。适合写代码、聊天、记笔记和处理长文本输入。

支持多个语音识别服务商，可在 TUI 中一键切换。

## 功能

- 全局快捷键录音，支持 `toggle`（按一下开始、再按一下停止）和 `hold`（按住说话）两种模式。
- 支持 6 个语音识别服务商，可在 TUI 中动态切换：
  - **豆包**（火山引擎）— 流式 ASR，实时出字
  - **OpenAI Realtime** — 流式 ASR，WebSocket 实时转写
  - **OpenAI Whisper** — 批量转写，录完后识别，兼容 Ollama 等第三方
  - **讯飞星火** — 流式 ASR，支持动态修正和 202 种方言
  - **小米 MiMo** — 批量转写，支持中英双语和方言
  - **小米 MiMo Token Plan** — 批量转写，国内节点
- 自动复制到剪贴板，支持自动上屏到当前输入框。
- Wayland / X11 / macOS 顶层录音状态胶囊提示。
- TUI 配置界面，支持添加/删除/切换 ASR 服务商，动态显示对应凭据字段，所有下拉选项均有交互提示。
- 热词增强识别，适合项目名、人名、英文术语和专有名词。
- 录音历史统计，包括历史次数、总字数、平均速度和最近速度。

## 平台状态

| 平台 | 状态 | 说明 |
| --- | --- | --- |
| Linux Wayland | 已支持 | 快捷键基于 evdev，需要 input 权限；剪贴板用 wl-clipboard，上屏用 wtype 或 uinput |
| Linux X11 | 已支持 | 使用 X11 原生全局热键和 XTest 上屏 |
| macOS | 已支持 | 快捷键基于 CGEventTap，录音用 CoreAudio，剪贴板用 NSPasteboard |
| Windows | 未实现 | 暂不支持 |

## 构建

Audio Talk AI 依赖平台原生能力，构建时需要启用 cgo。

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
make build          # 或 go build -o build/audio-talk-ai ./cmd/audio-talk-ai
make install        # 安装到 ~/.local/bin/
```

## 使用

```bash
audio-talk-ai              # TUI 模式（默认，单实例）
audio-talk-ai --no-tui     # 后台模式
audio-talk-ai --doctor     # 环境检查
audio-talk-ai --backend wayland   # 强制 Wayland
audio-talk-ai --backend x11      # 强制 X11
```

### 可断开会话

按 `Ctrl+]` 或 `b` 断开，Audio Talk AI 继续后台运行（热键仍有效），想看界面时再恢复。需要 `fzf` 来选择会话。

```bash
audio-talk-ai --d          # 启动可断开的 TUI 会话
audio-talk-ai --di         # 恢复已有会话（fzf 选择）
audio-talk-ai --list       # 列出活跃会话
audio-talk-ai --detach 1   # 按编号停止会话
```

### Web 管理界面

启动后浏览器打开 `http://localhost:8391`，可以：

- 配置语音设置（热键、模式、自动上屏等）
- 添加/删除/编辑 ASR 服务商
- 查看转写历史记录

端口可在 TUI 中修改，或在配置文件中设置：

```toml
[web]
enabled = true
port = 8391
```

## 配置

配置文件路径：`~/.config/audio-talk-ai/config.toml`

### 基本配置

```toml
[voice]
mode = "toggle"
push_to_talk = "F9"              # 默认 F9；也可用 Alt+Super 等修饰键组合
language = "zh-CN"
auto_submit = true               # 自动上屏；false 则只复制到剪贴板
# hotwords = ["项目名", "人名", "术语"]
```

### ASR 服务商配置

可以配置多个服务商，在 TUI 中切换。不配置 `[[asr_providers]]` 时，直接在 `[voice]` 中写 `app_key` / `access_key` 也能用（兼容旧配置）。

```toml
# 豆包 ASR（流式，推荐）
[[asr_providers]]
name = "doubao"
type = "doubao"
default = true
app_key = "your_app_key"
access_key = "your_access_key"

# OpenAI Realtime（流式）
# [[asr_providers]]
# name = "openai"
# type = "openai-realtime"
# api_key = "sk-..."
# model = "gpt-4o-mini-transcribe"

# OpenAI Whisper（批量，兼容 Ollama 等第三方）
# [[asr_providers]]
# name = "whisper"
# type = "openai-whisper"
# api_key = "sk-..."
# model = "whisper-1"
# endpoint = "http://localhost:11434/v1/audio/transcriptions"

# 讯飞星火（流式，支持方言）
# [[asr_providers]]
# name = "xfyun"
# type = "xfyun-spark"
# app_id = "your_app_id"
# api_key = "your_api_key"
# api_secret = "your_api_secret"

# 小米 MiMo（批量）
# [[asr_providers]]
# name = "mimo"
# type = "xiaomi-mimo-asr"
# api_key = "your_mimo_api_key"
# model = "mimo-v2.5-asr"

# 小米 MiMo Token Plan（批量，国内节点）
# [[asr_providers]]
# name = "mimo-token"
# type = "xiaomi-mimo-asr-TokenPlan"
# api_key = "your_mimo_api_key"
# model = "mimo-v2.5-asr"
```

### Web 管理界面配置

```toml
[web]
enabled = true
port = 8391
```

### 热键说明

语音热键只支持适合作为全局快捷键的按键：

- 支持：`Alt+Super`、`Ctrl+Alt+Shift`、`F9`、`Alt+F8`、`Tab`、`CapsLock` 等
- 不支持：字母、数字、标点、空格等会输入文本的按键

macOS 热键写法：`Option` = Alt，`Command`/`Cmd` = Super

```toml
push_to_talk = "Option+Command"
```

## 快捷键

| 快捷键 | 作用 |
|--------|------|
| 录音热键 | 开始/停止录音 |
| `Esc` | 取消当前录音 |
| `R` | 重试上次识别错误 |

## 更新日志

见 [CHANGELOG.md](CHANGELOG.md)。

## 致谢

- 语音输入核心基于 [github.com/whoamihappyhacking/just-talk-go](https://github.com/whoamihappyhacking/just-talk-go) 开发
- 可断开会话（di 模式）基于 [github.com/whoamihappyhacking/di](https://github.com/whoamihappyhacking/di) 实现

## 许可证

Audio Talk AI 使用 GNU General Public License v3.0 开源。

**禁止商用。** 本项目仅供学习和个人使用，不得用于任何商业用途。
