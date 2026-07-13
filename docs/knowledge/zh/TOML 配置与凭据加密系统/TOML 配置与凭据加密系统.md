---
kind: configuration_system
name: TOML 配置与凭据加密系统
category: configuration_system
scope:
    - '**'
source_files:
    - config/config.go
    - config/crypto.go
    - cmd/audio-talk-ai/main.go
    - config.toml.example
---

## 系统概述

本仓库采用基于 TOML 的集中式配置系统，由 config/ 包统一负责配置文件发现、加载、默认值合并、热键解析以及敏感凭据的 AES-256-GCM 磁盘加密存储。启动时通过命令行参数 -config 指定路径，否则按优先级在 ./config.toml、~/.config/audio-talk-ai/config.toml、$XDG_CONFIG_HOME/audio-talk-ai/config.toml 中查找。

## 核心文件与职责

- config/config.go：定义 Config 根结构体及 VoiceConfig、ASRProviderConfig、DebugConfig、OverlayConfig、WebConfig 等子结构；实现 Default()、Load()、Save()、FindConfig()、MigratePlaintextSecrets()、ParseHotkey() 等入口函数；提供 ResolveASRProviders() / DefaultASRProvider() 将 [voice] 向后兼容为单 provider。
- config/crypto.go：实现 per-user AES-256-GCM 密钥管理（~/.config/audio-talk-ai/key），以及 encryptString / decryptString、HasEncryptedSecrets / HasPlaintextSecrets 等工具。
- cmd/audio-talk-ai/main.go：应用启动入口，调用 config.Load → 检测明文凭据并触发迁移 → 注入 engine.Engine → 通过 eng.WatchConfig 监听文件变更实现运行时热重载。
- config.toml.example：完整示例，覆盖所有 ASR 提供商（豆包、OpenAI Realtime/Whisper、讯飞星火/iat/rtasr/lfasr/mimo 等）以及 overlay/web/debug 开关。

## 架构与设计决策

1. 分层加载：Default() 给出全量默认值（如 mode=toggle、push_to_talk=F9、overlay.position=bottom-center、web.port=8391）。Load() 读取 TOML 后以 toml.Unmarshal 覆盖默认值；若未找到配置文件则直接返回默认配置。

2. 向后兼容 + 多 Provider 并存：旧版 [voice].app_key/access_key/resource_id 仍被识别，ResolveASRProviders() 会将其包装为单个 doubao provider。新版通过 [[asr_providers]] 列表声明多个厂商，每个条目含 name/type/default 及各自字段；未知字段自动落入 Extra map[string]string，由 extractExtra 二次解析保留。

3. 运行时热重载：main.go 在 engine.New(...) 之后调用 eng.WatchConfig(path)，配合 TUI/WebUI 保存回调 OnSave = eng.ReloadConfig，实现不重启进程更新热键、provider、overlay 等配置。

4. 凭据加密与迁移：所有敏感字段（app_key、access_key、resource_id、api_key、api_secret、app_id、dwa 以及 Extra 中的密文）以 enc: 前缀标记。首次运行检测到明文凭据时，MigratePlaintextSecrets 先备份原文件为 .bak，对副本执行 encrypt→decrypt round-trip 校验一致后再写入，失败则保持原文件不变。每次 Save() 都会对内存中的明文配置做深拷贝再加密落盘，确保运行期进程持有的仍是明文以便 ASR 客户端直接使用。

5. 热键字符串解析器：ParseHotkey("Ctrl+Shift+F9") 将修饰键名（ctrl/alt/shift/super/cmd/command/win/option）与按键名映射到 hotkey.Modifier / hotkey.KeyCode，支持大小写不敏感与别名。

## 开发者约定

- 新增 ASR 字段：在 ASRProviderConfig 上添加带 toml:"xxx" tag 的字段，并在 knownASRKeys 与 typedFieldMap() 中登记，同时在 ProviderCfgMap() 中输出给工厂。
- 新增敏感字段：同步加入 encryptSecrets / decryptSecrets 遍历列表以及 hasPlaintextSecrets / secretsEqual 比较逻辑，避免明文泄露或迁移遗漏。
- 新增非敏感字段：只需在 toMap() 序列化分支中添加对应 key-value，无需改动加密流程。
- 配置项命名：TOML key 使用 snake_case（如 push_to_talk、stop_delay_ms），与示例保持一致。
- 环境变量覆盖：仅 JUST_TALK_BACKEND 用于运行时强制后端，其余配置均走 TOML，不在代码中硬编码环境变量回退。