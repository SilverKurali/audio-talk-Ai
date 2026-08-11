# ASR 服务商配置指南

本文档面向初次使用的用户，手把手教你如何获取各服务商的 API 密钥，并在 Audio Talk AI 中完成配置。

> **什么是 ASR？** ASR（Automatic Speech Recognition）即自动语音识别，把你说过的话转成文字。Audio Talk AI 支持 12 个 ASR 服务商，你可以按需选择。

## 快速选择

**不知道该用哪个？看这里：**

| 你的需求 | 推荐服务商 | 理由 |
|---------|-----------|------|
| 国内用户，想要实时出字 | **豆包**（doubao） | 免费额度大、延迟低、配置简单 |
| 有 OpenAI API Key | **OpenAI Realtime** | 流式实时转写，准确率高 |
| 需要方言 / 专业领域 | **讯飞语音听写**（xfyun-iat） | 支持医疗/政务/金融领域 + 23 种方言免切 |
| 长会议实时转写 | **讯飞实时转写**（xfyun-rtasr） | 支持最长 8 小时连续录音 |
| 本地部署，不想联网 | **OpenAI Whisper**（兼容 Ollama） | 支持本地 Whisper 模型 |
| 想要最简单的配置 | **小米 MiMo** | 只需一个 API Key |

## 服务商总览

Audio Talk AI 支持以下 12 个 ASR 服务商：

| 服务商 | 注册名 | 模式 | 密钥数量 | 免费额度 |
|--------|--------|------|----------|----------|
| 豆包（火山引擎） | `doubao` | 流式 | App Key + Access Key | 有（50 小时/月） |
| OpenAI Realtime | `openai-realtime` | 流式 | API Key | 付费 |
| OpenAI Whisper | `openai-whisper` | 批量 | API Key | 付费 |
| 讯飞星火 | `xfyun-spark` | 流式 | AppID + APIKey + APISecret | 有（试用额度） |
| 讯飞语音听写 | `xfyun-iat` | 流式 | AppID + APIKey + APISecret | 有（试用额度） |
| 讯飞实时转写大模型 | `xfyun-rtasr` | 流式 | AppID + AccessKeyId + AccessKeySecret | 有（试用额度） |
| 讯飞实时转写标准版 | `xfyun-rtasr-std` | 流式 | AppID + APIKey | 有（试用额度） |
| 讯飞录音转写标准版 | `xfyun-lfasr` | 批量 | AppID + APIKey + SecretKey | 有（试用额度） |
| 讯飞转写大模型 | `xfyun-lfasr-llm` | 批量 | AppID + AccessKeyId + AccessKeySecret | 有（试用额度） |
| 讯飞极速转写 | `xfyun-lfasr-fast` | 批量 | AppID + APIKey + APISecret | 有（试用额度） |
| 小米 MiMo | `xiaomi-mimo-asr` | 批量 | API Key | 有（试用额度） |
| 小米 MiMo Token Plan | `xiaomi-mimo-asr-TokenPlan` | 批量 | API Key | 有（试用额度） |

> **流式 vs 批量**：流式在录音时实时返回文字（边说边出字）；批量需要等录完后才上传识别（适合长录音）。

---

## 一、豆包（火山引擎）

豆包是字节跳动火山引擎的 ASR 服务，**免费额度大、延迟低**，是国内用户的首选。

### 1. 注册账号

打开 [火山引擎官网](https://www.volcengine.com/)，点击右上角「免费注册」，用手机号注册即可。

### 2. 开通语音识别服务

1. 登录后在顶部搜索框搜索「语音识别」
2. 或直接访问：[语音识别控制台](https://console.volcengine.com/speech/app)
3. 点击「开通服务」→ 选择「流式语音识别」→ 确认开通

### 3. 创建应用、获取密钥

1. 进入 [语音识别控制台](https://console.volcengine.com/speech/app)
2. 点击「创建应用」
3. 填写应用名称（随意），选择「语音识别」能力
4. 创建成功后，你会看到：
   - **App Key**（即 App ID）← 复制保存
   - **Access Token**（即 Access Key）← 复制保存

### 4. 配置示例

在 TUI 中添加，或直接编辑配置文件 `~/.config/audio-talk-ai/config.toml`（Windows 为 `%APPDATA%\audio-talk-ai\config.toml`）：

```toml
[[asr_providers]]
name = "豆包"
type = "doubao"
default = true
app_key = "你复制的App Key"
access_key = "你复制的Access Token"
```

> 豆包只需 2 个字段，配置最简单。免费额度每月 50 小时，日常使用完全够用。

---

## 二、OpenAI

OpenAI 提供两种语音识别：**Realtime**（流式实时转写）和 **Whisper**（批量转写，录完后识别）。

### 1. 注册账号

1. 打开 [platform.openai.com](https://platform.openai.com/)
2. 用邮箱或 Google/Microsoft 账号注册
3. 需要绑定信用卡（国际卡），或购买 API 额度

### 2. 获取 API Key

1. 登录后进入 [API Keys 页面](https://platform.openai.com/api-keys)
2. 点击「Create new secret key」
3. 给密钥起个名字 → 点击「Create secret key」
4. **立即复制保存**（页面关闭后无法再次查看）

### 3. 配置示例

**OpenAI Realtime（推荐，流式实时出字）：**

```toml
[[asr_providers]]
name = "OpenAI Realtime"
type = "openai-realtime"
default = true
api_key = "sk-你复制的API Key"
model = "gpt-4o-mini-transcribe"
```

可选模型：`gpt-4o-transcribe`（高精度）、`gpt-4o-mini-transcribe`（快速低价）、`whisper-1`（经典模型）。

**OpenAI Whisper（批量，录完后识别）：**

```toml
[[asr_providers]]
name = "OpenAI Whisper"
type = "openai-whisper"
api_key = "sk-你复制的API Key"
model = "whisper-1"
```

**兼容 OpenAI 格式的第三方服务（如本地 Ollama、vLLM 等）：**

```toml
[[asr_providers]]
name = "本地 Whisper"
type = "openai-whisper"
api_key = "ollama"
model = "whisper"
endpoint = "http://localhost:11434/v1/audio/transcriptions"
```

> 使用本地服务时 `api_key` 可以填任意值（不能为空），`endpoint` 填本地服务地址（与 `config.toml.example` 一致）。

---

## 三、讯飞开放平台

讯飞提供 7 种语音识别服务，覆盖日常输入、专业领域、长录音等场景。它们的密钥获取流程类似，但**密钥类型不完全相同**，请注意区分。

### 1. 注册账号

1. 打开 [讯飞开放平台](https://www.xfyun.cn/)
2. 点击右上角「注册」→ 用手机号注册
3. 完成实名认证（个人认证即可使用免费额度）

### 2. 创建应用

1. 登录后进入 [控制台](https://console.xfyun.cn/)
2. 点击左侧「我的应用」→「创建新应用」
3. 填写应用名称和描述 → 确认创建
4. 创建后你会看到 **AppID** ← 先复制保存

### 3. 开通服务并获取密钥

> **重要提示**：讯飞不同服务的密钥名称不同！有的是 APIKey + APISecret，有的是 AccessKeyId + AccessKeySecret，还有的是 SecretKey。请仔细对照下表。

| 服务 | 开通入口 | 需要的密钥 |
|------|---------|-----------|
| 星火语音大模型 | [服务页](https://console.xfyun.cn/services/bmc) | AppID + APIKey + APISecret |
| 语音听写（流式版） | [服务页](https://console.xfyun.cn/services/iat) | AppID + APIKey + APISecret |
| 实时转写大模型 | [服务页](https://console.xfyun.cn/services/rtasr) | AppID + **AccessKeyId** + **AccessKeySecret** |
| 实时转写标准版 | [服务页](https://console.xfyun.cn/services/lfasr) | AppID + **APIKey** |
| 录音转写标准版 | [服务页](https://console.xfyun.cn/services/lfasr) | AppID + APIKey + **SecretKey** |
| 转写大模型 | [服务页](https://console.xfyun.cn/services/lfasr_llm) | AppID + **AccessKeyId** + **AccessKeySecret** |
| 极速转写 | [服务页](https://console.xfyun.cn/services/ost) | AppID + APIKey + APISecret |

**获取 APIKey / APISecret 的步骤：**

1. 在控制台点击左侧「我的应用」
2. 找到你的应用 → 点击对应服务右侧的「APIKey」或「APISecret」
3. 复制保存

**获取 AccessKeyId / AccessKeySecret 的步骤（实时转写大模型、转写大模型）：**

1. 在控制台右上角点击头像 → 「AccessKey 管理」
2. 或直接访问：[AccessKey 管理页面](https://console.xfyun.cn/services/access-key)
3. 创建或查看 AccessKey → 复制 **AccessKeyId** 和 **AccessKeySecret**

**获取 SecretKey 的步骤（录音转写标准版）：**

1. 在控制台进入「语音转写」→「服务管理」
2. 查看 **SecretKey**（注意不是应用详情里的 APISecret）

### 4. 选择适合你的讯飞服务

| 你的场景 | 推荐服务 | 原因 |
|---------|---------|------|
| 日常语音输入 | `xfyun-spark` 或 `xfyun-iat` | 流式实时出字，延迟低 |
| 需要方言免切 | `xfyun-iat` | 支持 23 种方言免切 |
| 医疗/政务/金融 | `xfyun-iat` | 可设置专业领域提高准确率 |
| 长会议实时记录 | `xfyun-rtasr`（8h）或 `xfyun-rtasr-std`（5h） | 流式实时出字，支持超长录音 |
| 长录音事后转写 | `xfyun-lfasr` 系列 | 批量上传识别 |

### 5. 配置示例

#### 讯飞星火（xfyun-spark）— 日常语音输入

```toml
[[asr_providers]]
name = "讯飞星火"
type = "xfyun-spark"
default = true
app_id = "你的AppID"
api_key = "你的APIKey"
api_secret = "你的APISecret"
dwa = "wpgs"
```

> `dwa = "wpgs"` 开启语音纠偏（动态修正），可以让识别结果更准确。留空则关闭。

#### 讯飞语音听写（xfyun-iat）— 专业领域 + 方言

```toml
[[asr_providers]]
name = "讯飞听写"
type = "xfyun-iat"
app_id = "你的AppID"
api_key = "你的APIKey"
api_secret = "你的APISecret"
domain = "iat"
accent = "mandarin"
dwa = "wpgs"
```

**domain 可选值：**

| 值 | 说明 | 备注 |
|---|------|------|
| `iat` | 日常用语 | 默认，无需额外开通 |
| `medical` | 医疗领域 | 需在控制台单独开通 |
| `gov-seat-assistant` | 政务坐席 | 需在控制台单独开通 |
| `seat-assistant` | 金融坐席 | 需在控制台单独开通 |
| `gov-ansys` | 政务分析 | 需在控制台单独开通 |
| `gov-nav` | 政务导航 | 需在控制台单独开通 |
| `fin-nav` | 金融导航 | 需在控制台单独开通 |
| `fin-ansys` | 金融分析 | 需在控制台单独开通 |

**accent 方言免切**：设为 `xfime-mianqie` 可支持 23 种方言免切换识别，包括四川话、河南话、东北话、粤语、闽南话、山东话、贵州话、云南话、客家话、天津话、上海话等。

> 未开通的领域会报错 11200。

#### 讯飞实时转写大模型（xfyun-rtasr）— 长会议实时记录

```toml
[[asr_providers]]
name = "讯飞实时转写"
type = "xfyun-rtasr"
app_id = "你的AppID"
api_key = "你的AccessKeyId"
api_secret = "你的AccessKeySecret"
lang = "autodialect"
```

> **注意**：这里填的是 **AccessKeyId + AccessKeySecret**，不是 APIKey + APISecret！

- `lang = "autodialect"` — 中英 + 方言自动识别
- `lang = "autominor"` — 37 种语种免切
- `pd` 可填领域：`court`（法律）、`edu`（教育）、`finance`（金融）、`medical`（医疗）、`tech`（科技）等

#### 讯飞实时转写标准版（xfyun-rtasr-std）— 轻量级长录音

```toml
[[asr_providers]]
name = "讯飞实时转写标准版"
type = "xfyun-rtasr-std"
app_id = "你的AppID"
api_key = "你的APIKey"
lang = "cn"
```

> 与实时转写大模型不同，标准版使用普通 **APIKey** 认证。`lang` 可选 `cn`（中文）或 `en`（英文）。

#### 讯飞录音转写标准版（xfyun-lfasr）— 批量转写

```toml
[[asr_providers]]
name = "讯飞录音转写"
type = "xfyun-lfasr"
app_id = "你的AppID"
api_key = "你的APIKey"
api_secret = "你的SecretKey"
```

> **注意**：`api_secret` 填的是**服务管理页面的 SecretKey**，不是应用详情里的 APISecret。

#### 讯飞转写大模型（xfyun-lfasr-llm）— 多语种批量

```toml
[[asr_providers]]
name = "讯飞转写大模型"
type = "xfyun-lfasr-llm"
app_id = "你的AppID"
api_key = "你的AccessKeyId"
api_secret = "你的AccessKeySecret"
```

> 同样使用 **AccessKeyId + AccessKeySecret**。支持 202 种方言 / 37 种语种免切。

#### 讯飞极速转写（xfyun-lfasr-fast）— 快速批量

```toml
[[asr_providers]]
name = "讯飞极速转写"
type = "xfyun-lfasr-fast"
app_id = "你的AppID"
api_key = "你的APIKey"
api_secret = "你的APISecret"
```

> 1 小时音频约 20 秒出结果，适合快速批量处理。

### 6. 讯飞密钥对照速查表

| 服务 | app_id | api_key 填什么 | api_secret 填什么 |
|------|--------|---------------|-----------------|
| xfyun-spark | AppID | APIKey | APISecret |
| xfyun-iat | AppID | APIKey | APISecret |
| xfyun-rtasr | AppID | **AccessKeyId** | **AccessKeySecret** |
| xfyun-rtasr-std | AppID | APIKey | *不需要* |
| xfyun-lfasr | AppID | APIKey | **SecretKey**（服务管理页） |
| xfyun-lfasr-llm | AppID | **AccessKeyId** | **AccessKeySecret** |
| xfyun-lfasr-fast | AppID | APIKey | APISecret |

---

## 四、小米 MiMo

小米 MiMo 是小米推出的语音识别服务，配置简单，只需一个 API Key。

### 1. 注册账号

1. 打开 [小米 MiMo 开放平台](https://platform.xiaomimimo.com/)
2. 用小米账号登录（没有的话先注册小米账号）
3. 完成实名认证

### 2. 获取 API Key

1. 登录后进入 [控制台](https://platform.xiaomimimo.com/console)
2. 找到「API Key 管理」或「密钥管理」
3. 创建新的 API Key → 复制保存

### 3. 配置示例

**标准版：**

```toml
[[asr_providers]]
name = "小米 MiMo"
type = "xiaomi-mimo-asr"
api_key = "你的MiMo API Key"
model = "mimo-v2.5-asr"
```

**Token Plan 版（国内节点）：**

```toml
[[asr_providers]]
name = "小米 MiMo Token"
type = "xiaomi-mimo-asr-TokenPlan"
api_key = "你的MiMo API Key"
model = "mimo-v2.5-asr"
```

> MiMo 是批量模式：按住热键录音，松开后音频上传识别，等待几秒出结果。

---

## 批量模式说明

录音文件转写类的 Provider 采用**批量模式**：

1. 按住热键录音，松开后音频自动上传到服务器
2. 等待服务端转写完成（通常几秒到几分钟）
3. 转写结果自动复制到剪贴板或上屏

与流式 Provider 不同，批量模式在录音过程中**不会实时显示识别结果**。

属于批量模式的 Provider：`openai-whisper`、`xiaomi-mimo-asr`、`xiaomi-mimo-asr-TokenPlan`、`xfyun-lfasr`、`xfyun-lfasr-llm`、`xfyun-lfasr-fast`。

---

## 常见问题

### Q: 密钥填错了会怎样？

会报认证错误（如 401、鉴权失败等）。请对照上方表格确认每个字段应该填什么密钥。讯飞的密钥类型特别多，很容易搞混。

### Q: 密钥安全吗？

Audio Talk AI 首次启动时会自动把配置文件中的明文密钥加密存储（使用本地生成的 AES 密钥），磁盘上不会保留明文。你可以在配置文件中看到 `enc:` 前缀表示已加密。

### Q: 报错 "unknown ASR provider" 怎么办？

确认 `type` 字段拼写正确。所有合法的 type 值：
`doubao`、`openai-realtime`、`openai-whisper`、`xfyun-spark`、`xfyun-iat`、`xfyun-rtasr`、`xfyun-rtasr-std`、`xfyun-lfasr`、`xfyun-lfasr-llm`、`xfyun-lfasr-fast`、`xiaomi-mimo-asr`、`xiaomi-mimo-asr-TokenPlan`。

### Q: 讯飞报错 11200 怎么办？

错误码 11200 表示「未授权的服务」。请到讯飞控制台对应服务页面开通试用或购买。注意部分垂直领域（如医疗、政务）需要在控制台单独申请。

### Q: 流式和批量有什么区别？

- **流式**（如豆包、OpenAI Realtime、讯飞星火）：边说边出字，延迟低，适合日常快速输入
- **批量**（如 OpenAI Whisper、讯飞录音转写、小米 MiMo）：录完后才识别，适合长录音或需要更高准确率的场景

### Q: 可以配置多个服务商吗？

可以。在配置文件中写多个 `[[asr_providers]]` 块即可。在 TUI 中可以随时切换默认服务商。

### Q: 配置文件在哪里？

`~/.config/audio-talk-ai/config.toml`（Windows 为 `%APPDATA%\audio-talk-ai\config.toml`）。也可以直接在 TUI 界面或 WebUI（`http://localhost:8391`）中操作，不需要手动编辑文件。

### Q: 讯飞哪个服务免费额度最多？

星火语音大模型和语音听写的免费额度通常比较充裕。实时转写和录音转写的免费额度相对较少，适合试用后按需购买。

---

## 相关链接

| 服务商 | 注册/控制台 | 文档 |
|--------|-----------|------|
| 豆包 | [火山引擎控制台](https://console.volcengine.com/speech/app) | [语音识别文档](https://www.volcengine.com/docs/6561) |
| OpenAI | [platform.openai.com](https://platform.openai.com/) | [Audio API 文档](https://platform.openai.com/docs/guides/speech-to-text) |
| 讯飞 | [讯飞开放平台](https://console.xfyun.cn/) | [讯飞 ASR 详细指南](xfyun-guide.md) |
| 小米 MiMo | [MiMo 控制台](https://platform.xiaomimimo.com/console) | [MiMo 文档](https://platform.xiaomimimo.com/) |
