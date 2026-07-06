# 项目结构

[中文](PROJECT_STRUCTURE.md) | [English](README.md)

```
audio-talk-ai/
├── cmd/audio-talk-ai/
│   └── main.go                    # 程序入口，CLI 参数解析，启动 TUI/daemon/WebUI
│
├── asr/                           # ASR（语音识别）抽象层
│   ├── asr.go                     # Client 接口定义、Common 配置、Result 类型
│   ├── registry.go                # Provider 注册表（类似 database/sql 驱动机制）
│   ├── doubao/client.go           # 豆包（火山引擎）流式 ASR，二进制 WebSocket 协议
│   ├── openairealtime/client.go   # OpenAI Realtime 流式 ASR，JSON WebSocket，24kHz 重采样
│   ├── openaiwhisper/client.go    # OpenAI Whisper 批量 ASR，REST API，兼容 Ollama
│   ├── xfyunspark/client.go       # 讯飞星火流式 ASR，JSON WebSocket，HMAC-SHA256 鉴权，支持动态修正
│   └── mimoasr/client.go          # 小米 MiMo 批量 ASR，REST API，Chat Completions 格式
│
├── config/
│   └── config.go                  # 配置加载/保存（TOML），热键解析，ASR 服务商管理
│
├── engine/                        # 插件引擎
│   ├── engine.go                  # 引擎生命周期：启动/停止/配置热重载/配置变更通知
│   └── plugin.go                  # Plugin 接口定义，PluginEnv 环境（热键注册/日志/配置）
│
├── hotkey/                        # 全局热键系统
│   ├── hotkey.go                  # Combo/Modifier/KeyCode 类型定义，Event 事件类型
│   ├── keycodes.go                # 所有键码定义和辅助方法（IsModifier, IsTextKey 等）
│   ├── tracker.go                 # 按键状态跟踪器，防止重复触发
│   ├── registry.go                # 热键注册和事件分发
│   ├── provider.go                # Provider 接口和跨平台选择逻辑
│   ├── provider_linux.go          # Linux 后端选择（X11/Wayland）
│   ├── provider_linux_wayland.go  # Wayland 热键：evdev 读取 /dev/input/event*
│   ├── provider_linux_x11.go      # X11 热键：原生 XGrabKey（cgo）
│   ├── provider_linux_x11_stub.go # X11 stub（no_x11 构建标签）
│   ├── provider_darwin.go         # macOS 热键：CGEventTap（cgo）
│   ├── provider_mock.go           # Mock 后端（内部测试用）
│   └── provider_windows.go        # Windows 后端（未实现）
│
├── internal/                      # 内部工具包
│   ├── autotype/                  # 自动上屏（粘贴到当前输入框）
│   │   ├── autotype.go            # 接口定义
│   │   ├── autotype_linux.go      # Linux：wtype 或 uinput
│   │   ├── autotype_darwin.go     # macOS：CGEvent 键盘事件
│   │   └── autotype_windows.go    # Windows（未实现）
│   │
│   ├── clipboard/                 # 剪贴板操作
│   │   ├── clipboard.go           # 接口定义
│   │   ├── clipboard_linux.go     # Linux：wl-clipboard / xclip / xsel
│   │   ├── clipboard_darwin.go    # macOS：NSPasteboard（Objective-C）
│   │   ├── clipboard_cmd.go       # 通用命令行剪贴板工具
│   │   └── clipboard_no_cmd.go    # 无命令行工具时的 stub
│   │
│   ├── doctor/                    # 启动环境检查
│   │   ├── doctor.go              # Doctor 接口和通用检查
│   │   ├── asr_check.go           # ASR 配置验证（检查必填字段）
│   │   ├── doctor_linux.go        # Linux 环境检查（evdev 权限、wl-clipboard 等）
│   │   ├── doctor_darwin.go       # macOS 环境检查（辅助功能权限等）
│   │   ├── doctor_other.go        # 其他平台 stub
│   │   └── commands.go            # doctor 建议的安装命令
│   │
│   ├── session/                   # 可断开会话（di 模式）
│   │   ├── session.go             # 会话元数据、socket 路径、会话列表
│   │   ├── server.go              # PTY server：fork 命令、Unix socket、客户端广播
│   │   └── attach.go              # 客户端 attach：raw terminal、Ctrl+] 断开、fzf 选择
│   │
│   ├── tui/                       # 终端 UI（Bubble Tea）
│   │   └── tui.go                 # 配置界面：热键/模式/服务商管理/统计/日志
│   │
│   └── webui/                     # Web 管理界面
│       ├── server.go              # HTTP server 和路由（go:embed 静态文件）
│       ├── handlers.go            # REST API：配置 CRUD、服务商管理、历史查询、状态
│       ├── history.go             # 转写历史持久化（JSON 文件）
│       └── static/                # 前端文件（嵌入二进制）
│           ├── index.html         # SPA 主页面
│           ├── app.js             # 前端逻辑
│           └── style.css          # 暗色主题样式
│
├── plugins/                       # 引擎插件
│   ├── debug.go                   # 调试插件（--debug 模式）
│   │
│   ├── voice/                     # 语音输入核心插件
│   │   ├── voice.go               # 录音/ASR 流式/剪贴板上屏/统计/热键行为
│   │   ├── recorder.go            # 录音器接口
│   │   ├── recorder_linux.go      # Linux 录音：ALSA（cgo）
│   │   ├── recorder_darwin.go     # macOS 录音：CoreAudio AudioQueue（cgo）
│   │   ├── recorder_command.go    # 外部命令录音（备用）
│   │   ├── recorder_windows.go    # Windows 录音（未实现）
│   │   ├── audioqueue_darwin.c    # macOS AudioQueue C 代码
│   │   └── audioqueue_darwin.h    # macOS AudioQueue 头文件
│   │
│   └── overlay/                   # 录音状态胶囊（悬浮提示）
│       ├── overlay.go             # 插件入口，状态同步
│       ├── backend_linux.go       # Linux 后端选择
│       ├── backend_wayland.go     # Wayland：wlr-layer-shell 协议
│       ├── backend_x11.go         # X11：原生窗口（cgo）
│       ├── backend_darwin.go      # macOS：NSPanel 辅助进程
│       ├── backend_stub.go        # 无 overlay 时的 stub
│       ├── helper_darwin.go       # macOS overlay helper 进程
│       ├── helper_stub.go         # 非 macOS 平台 stub
│       ├── protocols/             # Wayland 协议定义文件
│       │   ├── wlr-layer-shell-unstable-v1.xml
│       │   └── xdg-shell.xml
│       ├── wlr-layer-shell-unstable-v1-client-protocol.h
│       ├── wlr-layer-shell-unstable-v1-protocol.c
│       ├── xdg-shell-client-protocol.h
│       ├── xdg-shell-protocol.c
│       ├── overlay_darwin.m       # macOS Objective-C 代码
│       └── overlay_darwin.h       # macOS 头文件
│
├── bin/
│   └── daudio-talk-ai             # 可断开会话的 shell 包装脚本
│
├── docs/
│   └── screenshot-tui.png         # TUI 截图
│
├── Makefile                       # 构建/安装/测试/清理目标
├── go.mod                         # Go 模块定义
├── go.sum                         # 依赖版本锁定（自动生成，需提交）
├── config.toml.example            # 配置文件示例（含所有服务商）
├── .gitignore                     # Git 忽略规则
├── LICENSE                        # GPLv3 许可证
├── AGENTS.md                      # AI 编码助手指南
├── CHANGELOG.md                   # 更新日志
├── PROJECT_STRUCTURE.md           # 本文件：项目结构说明
├── README.md                      # 中文说明文档
└── README.en.md                   # 英文说明文档
```

## 架构概览

```
用户按热键 → hotkey/ 检测全局快捷键
  → plugins/voice/ 开始录音
  → asr/ 流式发送音频到 ASR 服务商
  → 识别结果 → clipboard/ 复制或 autotype/ 自动上屏
  → plugins/overlay/ 显示录音状态

engine/ 管理插件生命周期和配置热重载
internal/tui/ 提供终端配置界面
internal/webui/ 提供 Web 管理界面
internal/session/ 提供可断开终端会话
```
