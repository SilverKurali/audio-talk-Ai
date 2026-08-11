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
- `make build`：使用 `go build -o ./build/audio-talk-ai ./cmd/audio-talk-ai` 编译当前平台二进制（Windows 上产物为 audio-talk-ai.exe，纯 Go 无 cgo；Linux/macOS 需启用 cgo）
- `make install`：先执行 build，再调用应用自身 `--install` 子命令把主程序安装到 `~/.local/bin/audio-talk-ai`（Windows 上安装到 `%LOCALAPPDATA%\Programs\audio-talk-ai\`），并将仓库内 `bin/daudio-talk-ai` 可断开会话包装脚本复制（cp）到 `~/.local/bin/daudio-talk-ai`（仅 Linux/macOS）
- `make run`：`go run ./cmd/audio-talk-ai` 直接运行源码
- `make test`：`go test ./... -v` 递归执行全部测试
- `make clean`：删除 `build/` 目录
- `make deps`：`go mod tidy && go mod download` 同步依赖

**依赖管理**
- 模块路径：`gitee.com/AY77-OP/audio-talk-ai`
- Go 版本要求：1.25.0（`go.mod` 声明）
- 依赖锁定：`go.sum` 与 `go.mod` 配合，无 vendor 目录
- 关键外部依赖包括 TOML 配置解析（BurntSushi/toml）、跨平台系统调用（golang.org/x/sys，Windows 侧经其调用 user32/kernel32/winmm）、Windows 剪贴板（atotto/clipboard）、TUI 框架（charmbracelet/bubbletea）、PTY 会话（creack/pty，仅 Linux/macOS）、WebSocket 通信等

**交叉编译与发布**
- 仓库已配置 CI：`.github/workflows/ci.yml`（Linux 以 no_x11 tag 构建测试、macOS 以 cgo 构建测试）与 `.gitee/workflows/ci.yml`（Gitee Go 仅 Linux runner）；Windows 尚未纳入 CI 矩阵，可在本地以 `GOOS=windows GOARCH=amd64 go build ./cmd/audio-talk-ai` 交叉编译（纯 Go 无 cgo，交叉编译无需额外工具链）
- 未发现 goreleaser、gox 等发布工具链配置
- 发布流程目前仅依赖开发者手动在目标平台上执行 `make build`（或交叉编译）并分发二进制

**设计决策**
- 保持极简：仅用标准 `go build` + Makefile，避免引入额外复杂度
- 安装流程内嵌：通过应用自身的 `--install` 子命令处理系统级安装逻辑，Makefile 仅负责触发
- 无版本注入：Makefile 未定义 `-ldflags` 注入版本号，版本信息可能由代码内部硬编码或通过 git tag 手工维护