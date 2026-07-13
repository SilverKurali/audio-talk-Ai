---
kind: dependency_management
name: Go 与 Node.js 双栈依赖管理
category: dependency_management
scope:
    - '**'
source_files:
    - go.mod
    - .mimocode/package.json
    - .mimocode/package-lock.json
---

本仓库采用 Go + Node.js 双语言栈，分别使用不同的包管理器管理第三方依赖：

- **Go 依赖**：通过 `go.mod`（module `gitee.com/AY77-OP/audio-talk-ai`，Go 1.25）声明直接依赖与间接依赖，配合 `go.sum` 锁定版本。核心依赖包括 TUI 框架（charmbracelet/bubbletea、bubbles、lipgloss）、WebSocket 库（gorilla/websocket、coder/websocket）、TOML 解析（BurntSushi/toml）、跨平台系统调用（golang.org/x/sys、x/term）以及 Linux uinput 驱动等。未启用 vendor 目录，也未配置 GOPRIVATE/GONOSUMCHECK 等代理或私有仓库环境变量。
- **Node.js 依赖**：仅 `.mimocode/package.json` 中声明单一依赖 `@mimo-ai/plugin@0.1.4`，并通过 `package-lock.json` 锁定其完整依赖树（含 effect、zod、msgpackr-extract 等平台原生可选包），用于 MIMO AI 插件开发辅助工具链。

两个子系统的依赖均通过各自的 lockfile 保证可重现构建，不存在跨语言共享依赖或统一依赖升级流程。