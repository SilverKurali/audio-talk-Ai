---
kind: logging_system
name: 基于 Go 标准库 log/slog 的文本日志系统
category: logging_system
scope:
    - '**'
source_files:
    - cmd/audio-talk-ai/main.go
    - plugins/voice/voice.go
    - asr/asr.go
---

## 1. 使用的系统与框架
整个仓库统一使用 Go 标准库 log/slog 作为结构化日志框架，未引入第三方日志库（如 zap、logrus、zerolog）。所有模块通过全局 slog.Default() 或从 engine.PluginEnv.Logger() 注入的 *slog.Logger 实例进行记录。

## 2. 关键文件与入口
- 日志初始化与默认处理器设置：cmd/audio-talk-ai/main.go。根据 -verbose 标志在 Info/Debug 之间切换级别；TUI 模式下将 TextHandler 输出到 /tmp/audio-talk-ai.log（Windows 为 %TEMP%\audio-talk-ai.log），Daemon 模式同时写入 stderr 与同一文件；通过 slog.New(slog.NewTextHandler(...)) + slog.SetDefault(logger) 完成全局注入。
- 插件侧 logger 来源：plugins/voice/voice.go 等插件在 Init(env) 中通过 env.Logger() 获取 logger 字段并用于各子流程。
- ASR 客户端：asr/asr.go 定义 Factory 签名接收 *slog.Logger，各驱动（doubao、mimoasr、openairealtime、openaiwhisper、xfyun*）均按此约定构造带 logger 的 Client。
- TUI 专用轻量日志通道：plugins/voice/voice.go 中的 SetupTUILog()/pout() 把语音录制状态以字符串追加到内存缓冲，供 TUI 渲染，与 slog 并行存在。

## 3. 架构与约定
- 单一全局 Logger：应用启动时创建唯一 *slog.Logger 并通过 SetDefault 注入，其他包直接调用 slog.Info/Debug/Warn/Error，无需显式传递 logger 指针。
- 结构化字段：所有业务日志均以 key-value 形式附加上下文，例如 provider、model、url、bytes、status、error、text、isFinal、combo、mode、auto_submit、stop_delay_ms 等，便于后续过滤与分析。
- 日志级别策略：默认 Info，仅记录连接、结果、错误等关键事件；开启 -verbose 后降级为 Debug，输出热键队列长度、录音起止、ASR 帧大小等高频细节。
- 输出格式：始终使用 NewTextHandler 输出纯文本行，未启用 JSON 格式；TUI 模式避免向 stderr 输出以免破坏终端界面。
- 插件隔离：每个插件持有独立 *slog.Logger 字段（由 engine 注入），但底层仍指向同一个全局 handler，因此可通过统一 level 控制全量输出。
- 辅助日志通道：voice 插件额外维护 TUILog 回调与环形缓冲区，用于 TUI 面板实时滚动显示“开始录音/已连接/最终结果”等用户态消息，与 slog 互不干扰。

## 4. 开发者应遵循的规则
- 新增模块如需日志，优先直接使用 slog.Info/Debug/Warn/Error 配合结构化字段；若需区分来源，可在 engine.PluginEnv 中传入独立 logger 实例。
- 不要自行再 slog.New 覆盖全局默认 logger，所有自定义 Handler 应在 main 中集中配置。
- 对敏感信息（密钥、完整请求体）一律不要写入日志；现有代码中对长响应体做了截断处理，应继续保持。
- 调试信息统一走 Debug 级别，生产默认 Info，避免在 TUI 模式下向 stdout/stderr 打印原始日志。
- 面向用户的即时反馈（如“即将停止…”、“已上屏”）应通过 voice.pout 通道输出，而非 slog，以保持 UI 与诊断日志分离。