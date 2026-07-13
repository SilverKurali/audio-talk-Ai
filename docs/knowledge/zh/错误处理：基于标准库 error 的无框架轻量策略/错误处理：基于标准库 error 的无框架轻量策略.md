---
kind: error_handling
name: 错误处理：基于标准库 error 的无框架轻量策略
category: error_handling
scope:
    - '**'
source_files:
    - asr/asr.go
    - engine/engine.go
    - cmd/audio-talk-ai/main.go
    - internal/session/server.go
    - plugins/voice/recorder.go
---

本仓库未引入第三方错误处理框架，整体采用 Go 标准库 `error` + `fmt.Errorf("%w")` 包装 + `slog` 结构化日志的轻量模式。核心特征如下：

1. **错误定义与传播**
   - 所有业务错误均以函数返回值形式向上传播，未见任何自定义 `type XxxError struct` 或全局 `ErrXxx = errors.New(...)` 哨兵错误。
   - 跨层包装统一使用 `fmt.Errorf("...: %w", err)`（如 `session: start PTY: %w`、`plugin %s init: %w`），便于调用方用 `errors.Is/As` 判断。
   - ASR 抽象层 `asr.Result` 将流式识别过程中的错误通过 `Result.Error` 字段异步回传，而非直接返回给调用 goroutine。

2. **panic/recover 策略**
   - 全仓搜索未发现任何 `panic()` 或 `recover()` 调用，说明代码不依赖 panic 作为控制流，也未在中间件/插件入口做兜底 recover。

3. **错误分类与上下文**
   - 通过 slog logger 的 key-value 字段为错误附加上下文（如 `"plugin"`, `"error"`, `"code"`, `"message"`），ASR 各驱动对上游厂商错误码做了结构化记录（如 `xfyun-iat ASR error`、`rtasr error`）。
   - 启动阶段遇到配置加载失败、provider 创建失败等致命错误时，main 会打印错误并附带 `printTroubleshooting` 提示后 `os.Exit(1)`。

4. **特定错误的显式处理**
   - `plugins/voice/recorder.go` 中用 `errors.Is(err, os.ErrClosed)` 将底层关闭错误规范化为 `io.EOF`，体现对标准错误类型的匹配习惯。
   - `internal/session/server.go` 对帧大小超限返回裸 `errors.New("frame too large")`，属于协议级校验错误。

5. **架构层面的容错**
   - Engine 在 `Start` 中对每个插件 goroutine 捕获非 `context.Canceled` 的错误并记录日志，避免单个插件崩溃导致整个进程退出。
   - 信号处理仅触发优雅停止（cancel context + Stop），不在 signal handler 中 panic 或 recover。

开发者约定建议：
- 新增错误一律以返回值形式向上冒泡，使用 `%w` 包装保留原始错误链；仅在不可恢复的初始化阶段才考虑 `os.Exit(1)`。
- 不要在业务逻辑中使用 `panic`，也不需要在插件入口加 `defer recover` 兜底。
- 对上游服务错误码应通过 slog 记录结构化字段，必要时在 `asr.Result.Error` 中透传以便上层消费。