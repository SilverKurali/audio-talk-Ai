---
kind: design
name: 采用 Provider 元数据驱动的动态注册机制
source: session
category: adr
---

# 采用 Provider 元数据驱动的动态注册机制

_来源：fb1ace0 → 0247f5a 提交周期内记录的编码计划——内容为规划时意图，实现可能滞后或有出入。_

**状态：** accepted

## 背景
新增一个 ASR Provider 需要修改 7+ 个文件（asr/xxx/client.go、plugins/voice/voice.go、internal/tui/tui.go 中的 3 处 switch-case、internal/webui/handlers.go 的 PROVIDER_DISPLAY map、internal/webui/static/app.js 的 PROVIDER_FIELDS + PROVIDER_NAMES、internal/webui/static/index.html 的 select option、config/config.go），耦合严重且每增加一个 provider 都要重复改动 UI 和配置层。

## 决策驱动
- 最小化新增 Provider 时的改动范围
- 消除 TUI/WebUI 中的 provider-specific switch-case
- 保持向后兼容已有 TOML 配置

## 备选方案
- **Provider 自描述元数据 + 反射式 UI 生成** — 优点：新增 Provider 只需两步：实现 client.go 并在 init() 中 RegisterWithMeta，以及 drivers/drivers.go 加一行 import；TUI 和 WebUI 完全无需改动；Extra map 支持未来任意字段扩展；缺点：需要在 config.Load/Save 中处理 Extra map 与 typed 字段的合并/分离；encryptSecrets 需依赖 metadata 而非硬编码字段名
- **继续维护 provider-specific switch-case** _（已否决）_ — 优点：实现简单直接，无额外抽象层；缺点：每新增一个 provider 必须同步修改 TUI、WebUI、handlers、index.html 等 7+ 个文件，极易遗漏导致功能缺失

## 决策
在 asr/metadata.go 定义 FieldDef/ProviderMeta/RegisterWithMeta 自描述结构，每个 provider 在 init() 中通过 RegisterWithMeta 注册工厂及其 UI 元数据；在 asr/drivers/drivers.go 集中空白导入所有 provider 以触发 init 注册；TUI 和 WebUI 改为从 asr.AllProviderMeta() 动态渲染表单和选项；config.ASRProviderConfig 保留现有 typed 字段并新增 Extra map[string]string 承载差异化参数，Get/Set 方法统一访问 typed 优先、Extra 兜底。

## 影响
新增 Provider 成本从 7+ 文件降至 2 步，彻底解耦了 provider 实现与 UI/配置层。代价是 config.Load/Save 需要二次解析 TOML 以将未识别键提取到 Extra map，encryptSecrets/decryptSecrets 需遍历 metadata 中标记为 Secret 的键进行加解密，运行时多了一次 metadata 查找开销。