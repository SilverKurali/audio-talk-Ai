---
kind: build_system
name: Go 原生构建与本地安装流程
category: build_system
scope:
    - '**'
source_files:
    - Makefile
    - go.mod
    - go.sum
    - cmd/audio-talk-ai/main.go
---

本项目采用 Go 语言原生构建体系，未引入第三方构建工具（如 gox、goreleaser）或容器化方案，所有构建逻辑集中在根目录 Makefile 中。

**构建入口与产物**
- 二进制入口：`cmd/audio-talk-ai/main.go`
- 输出目录：`build/`（当前为空，由 `make build` 生成）
- 可执行文件命名：`audio-talk-ai`（通过 `APP_NAME` 变量控制）

**核心 make 目标**
- `make build`：使用 `go build -o ./build/audio-talk-ai ./cmd/audio-talk-ai` 编译当前平台二进制
- `make install`：先执行 build，再调用应用自身 `--install` 子命令把主程序安装到 `~/.local/bin/audio-talk-ai`，并将仓库内 `bin/daudio-talk-ai` 可断开会话包装脚本复制（cp）到 `~/.local/bin/daudio-talk-ai`
- `make run`：`go run ./cmd/audio-talk-ai` 直接运行源码
- `make test`：`go test ./... -v` 递归执行全部测试
- `make clean`：删除 `build/` 目录
- `make deps`：`go mod tidy && go mod download` 同步依赖

**依赖管理**
- 模块路径：`gitee.com/AY77-OP/audio-talk-ai`
- Go 版本要求：1.25.0（`go.mod` 声明）
- 依赖锁定：`go.sum` 与 `go.mod` 配合，无 vendor 目录
- 关键外部依赖包括 TOML 配置解析（BurntSushi/toml）、跨平台系统调用（golang.org/x/sys）、TUI 框架（charmbracelet/bubbletea）、PTY 会话（creack/pty）、WebSocket 通信等

**交叉编译与发布**
- 未发现任何交叉编译脚本、Dockerfile、GitHub Actions 或其他 CI 配置文件
- 未发现 goreleaser、gox、gox 等工具链配置
- 发布流程目前仅依赖开发者手动在目标平台上执行 `make build` 并分发二进制

**设计决策**
- 保持极简：仅用标准 `go build` + Makefile，避免引入额外复杂度
- 安装流程内嵌：通过应用自身的 `--install` 子命令处理系统级安装逻辑，Makefile 仅负责触发
- 无版本注入：Makefile 未定义 `-ldflags` 注入版本号，版本信息可能由代码内部硬编码或通过 git tag 手工维护