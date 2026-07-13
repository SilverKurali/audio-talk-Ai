# 讯飞语音识别服务配置指南

> 本文仅覆盖讯飞系列服务。如需查看所有服务商（豆包、OpenAI、小米等）的配置指南，请参阅 [ASR 服务商配置指南](asr-providers-guide.md)。

本文档介绍如何在 Audio Talk AI 中配置和使用讯飞开放平台的各类语音识别服务。

## 服务对比

| Provider 类型 | 注册名 | 模式 | 最长时长 | 特点 | 适用场景 |
|---|---|---|---|---|---|
| 星火语音识别大模型 | `xfyun-spark` | 流式 | 60s | 中英+202种方言，动态修正 | 日常短语音输入（已实现） |
| 语音听写（流式版） | `xfyun-iat` | 流式 | 60s | 垂直领域，多方言免切，多候选 | 医疗/政务/金融等专业场景 |
| 实时语音转写大模型 | `xfyun-rtasr` | 流式 | 8h | Binary PCM，202种方言，说话人分离 | 长录音实时转写 |
| 实时语音转写（标准版） | `xfyun-rtasr-std` | 流式 | 5h | Binary PCM，apiKey认证，简单接入 | 长录音实时转写（标准版） |
| 录音文件转写（标准版） | `xfyun-lfasr` | 批量 | 5h | 标准版，成熟稳定 | 长录音转写 |
| 录音文件转写大模型 | `xfyun-lfasr-llm` | 批量 | 5h | 202种方言/37语种免切 | 多语种长录音 |
| 极速录音转写大模型 | `xfyun-lfasr-fast` | 批量 | 5h | 1小时音频约20秒出结果 | 快速批量转写 |

### 选择建议

- **日常语音输入**：推荐 `xfyun-spark`（已有）或 `xfyun-iat`，流式实时出字
- **专业领域识别**：使用 `xfyun-iat`，可设置 `domain` 为医疗/政务/金融等领域
- **方言免切识别**：`xfyun-iat` 支持 23 种方言免切（设置 `accent = "xfime-mianqie"`）
- **长录音实时转写**：使用 `xfyun-rtasr`（大模型版，8 小时）或 `xfyun-rtasr-std`（标准版，5 小时），实时出字
- **长录音批量转写**：根据需求选择 `xfyun-lfasr`（标准版）、`xfyun-lfasr-llm`（大模型）或 `xfyun-lfasr-fast`（极速版）

## 申请 API 凭证

### 通用步骤

1. 注册 [讯飞开放平台](https://www.xfyun.cn/) 账号
2. 进入 [控制台](https://console.xfyun.cn/)
3. 创建应用，获取 **AppID**、**APIKey**、**APISecret**

### 各服务开通入口

| 服务 | 控制台入口 | 密钥说明 |
|---|---|---|
| 星火语音识别大模型 | [服务页](https://console.xfyun.cn/services/bmc) | AppID + APIKey + APISecret |
| 语音听写（流式版） | [服务页](https://console.xfyun.cn/services/iat) | AppID + APIKey + APISecret |
| 实时语音转写大模型 | [服务页](https://console.xfyun.cn/services/rtasr) | AppID + **AccessKeyId** + **AccessKeySecret** |
| 实时语音转写（标准版） | [服务页](https://console.xfyun.cn/services/lfasr) | AppID + **API Key**（HMAC-SHA1+MD5 认证） |
| 录音文件转写（标准版） | [服务页](https://console.xfyun.cn/services/lfasr) | AppID + APIKey + **SecretKey**（注意不是 APISecret） |
| 录音文件转写大模型 | [服务页](https://console.xfyun.cn/services/lfasr_llm) | AppID + **AccessKeyId** + **AccessKeySecret** |
| 极速录音转写大模型 | [服务页](https://console.xfyun.cn/services/ost) | AppID + APIKey + APISecret |

> **注意**：录音文件转写标准版的密钥是 `SecretKey`（在服务管理页面），不是 `APISecret`。录音文件转写大模型使用的是 `AccessKeyId` + `AccessKeySecret`。

## 配置示例

### 讯飞星火语音识别大模型（xfyun-spark）

```toml
[[asr_providers]]
name = "xfyun-spark"
type = "xfyun-spark"
default = true
app_id = "your_app_id"
api_key = "your_api_key"
api_secret = "your_api_secret"
dwa = "wpgs"  # 动态修正（可选）
```

### 讯飞语音听写流式版（xfyun-iat）

```toml
[[asr_providers]]
name = "xfyun-iat"
type = "xfyun-iat"
app_id = "your_app_id"
api_key = "your_api_key"
api_secret = "your_api_secret"
domain = "iat"       # 可选领域，见下方说明
accent = "mandarin"  # 普通话，或 "xfime-mianqie" 方言免切
dwa = "wpgs"         # 动态修正（可选，仅中文）
```

**domain 参数说明**：

| 值 | 说明 |
|---|---|
| `iat` | 日常用语（默认） |
| `medical` | 医疗领域 |
| `gov-seat-assistant` | 政务坐席助手 |
| `seat-assistant` | 金融坐席助手 |
| `gov-ansys` | 政务语音分析 |
| `gov-nav` | 政务语音导航 |
| `fin-nav` | 金融语音导航 |
| `fin-ansys` | 金融语音分析 |

> 除 `iat` 外的领域需在控制台单独开通，未授权会报错 11200。

### 讯飞实时语音转写大模型（xfyun-rtasr）

```toml
[[asr_providers]]
name = "xfyun-rtasr"
type = "xfyun-rtasr"
app_id = "your_app_id"
api_key = "your_access_key_id"      # AccessKeyId
api_secret = "your_access_key_secret"  # AccessKeySecret
lang = "autodialect"  # 中英+方言，或 "autominor" 多语种
pd = ""  # 可选领域
```

**特点**：
- 流式 WebSocket + Binary PCM 协议，实时出字
- 支持最长 8 小时连续录音
- 支持 202 种方言免切（`autodialect`）和 37 种语种（`autominor`）
- 支持说话人分离（角色分离）

**pd 领域参数**：`court`（法律）、`finance`（金融）、`medical`（医疗）、`tech`（科技）、`sport`（体育）、`edu`（教育）、`isp`（运营商）、`gov`（政府）、`game`（游戏）、`ecom`（电商）、`mil`（军事）、`com`（企业）、`life`（生活）、`ent`（娱乐）、`culture`（人文历史）、`car`（汽车）

> 注意：实时转写大模型使用 **AccessKeyId + AccessKeySecret**，不是普通的 APIKey/APISecret。

**accent 方言免切**：

设置 `accent = "xfime-mianqie"` 可支持 23 种方言免切换识别：四川话、河南话、东北话、粤语、闽南话、山东话、贵州话、云南话、客家话、天津话、河北话、太原话、上海话、合肥话、南京话、皖北话、台湾话、甘肃话、陕西话、宁夏话、长沙话、南昌话、武汉话。

### 讯飞实时语音转写标准版（xfyun-rtasr-std）

```toml
[[asr_providers]]
name = "xfyun-rtasr-std"
type = "xfyun-rtasr-std"
app_id = "your_app_id"
api_key = "your_api_key"  # API Key（HMAC-SHA1+MD5 认证）
lang = "cn"  # cn=中文，en=英文
pd = ""  # 可选领域参数
```

**特点**：
- 流式 WebSocket + Binary PCM 协议，实时出字
- 支持最长 5 小时连续录音
- 使用 API Key 认证（与 xfyun-rtasr 的 AccessKeyId 不同）
- 与 xfyun-rtasr（大模型版）功能类似，但不支持方言免切和说话人分离

**pd 领域参数**：`court`（法律）、`finance`（金融）、`medical`（医疗）、`tech`（科技）、`sport`（体育）、`edu`（教育）、`gov`（政府）

### 讯飞录音文件转写标准版（xfyun-lfasr）

```toml
[[asr_providers]]
name = "xfyun-lfasr"
type = "xfyun-lfasr"
app_id = "your_app_id"
api_key = "your_api_key"
api_secret = "your_secret_key"  # 注意：这是 SecretKey，不是 APISecret
pd = ""  # 可选领域参数
```

**pd 领域参数**：`court`（法律）、`edu`（教育）、`finance`（金融）、`medical`（医疗）、`tech`（科技）、`sport`（体育）、`gov`（政府）、`game`（游戏）、`ecom`（电商）、`car`（汽车）

### 讯飞录音文件转写大模型（xfyun-lfasr-llm）

```toml
[[asr_providers]]
name = "xfyun-llm"
type = "xfyun-lfasr-llm"
app_id = "your_app_id"
api_key = "your_access_key_id"      # AccessKeyId
api_secret = "your_access_key_secret"  # AccessKeySecret
pd = ""
```

支持 `autodialect`（中英+202种方言）和 `autominor`（37种语种免切）。

### 讯飞极速录音转写大模型（xfyun-lfasr-fast）

```toml
[[asr_providers]]
name = "xfyun-fast"
type = "xfyun-lfasr-fast"
app_id = "your_app_id"
api_key = "your_api_key"
api_secret = "your_api_secret"
pd = ""
```

1 小时音频最快约 20 秒出结果。

## 批量模式说明

录音文件转写类的三个 provider（`xfyun-lfasr`、`xfyun-lfasr-llm`、`xfyun-lfasr-fast`）采用**批量模式**：

1. 按住热键录音，松开后音频自动上传到讯飞服务器
2. 等待服务端转写完成（通常几秒到几分钟，取决于音频时长）
3. 转写结果自动复制到剪贴板或上屏

与流式 provider 不同，批量模式在录音过程中不会实时显示识别结果。

## 常见问题

### Q: 流式 provider 和批量 provider 有什么区别？

流式 provider（`xfyun-spark`、`xfyun-iat`）在录音时实时返回识别结果，适合日常快速语音输入。批量 provider 需要等录音结束后才上传识别，适合需要更高准确率或长录音的场景。

### Q: xfyun-iat 和 xfyun-spark 有什么区别？

两者都是流式 WebSocket 协议，但：
- `xfyun-spark` 使用讯飞星火大模型端点，支持 202 种方言
- `xfyun-iat` 使用语音听写端点，支持垂直领域（医疗、政务、金融等）和 23 种方言免切
- 两者需要在控制台分别开通不同的服务

### Q: 录音文件转写标准版的 api_secret 为什么和 APISecret 不同？

标准版转写使用的是服务管理页面中的 **SecretKey**，而非应用详情中的 APISecret。请在讯飞控制台 → 语音转写 → 服务管理页面获取。

### Q: 报错 "unknown ASR provider" 怎么办？

确认 provider 的 `type` 字段拼写正确：`xfyun-spark`、`xfyun-iat`、`xfyun-lfasr`、`xfyun-lfasr-llm`、`xfyun-lfasr-fast`。

### Q: 报错 11200 怎么办？

错误码 11200 表示未授权的服务。请到讯飞控制台对应服务页面开通试用或购买。

### Q: 批量模式等待时间很长？

录音文件转写是异步的，标准版最大耗时不超过 5 小时（通常几分钟），极速版 1 小时音频约 20 秒。如果等待时间异常长，可能是服务端排队高峰。

## 相关链接

- [讯飞开放平台控制台](https://console.xfyun.cn/)
- [星火语音识别大模型文档](https://www.xfyun.cn/doc/spark/spark_zh_iat.html)
- [语音听写（流式版）文档](https://www.xfyun.cn/doc/asr/ifasr_new/API.html)
- [录音文件转写（标准版）文档](https://www.xfyun.cn/doc/asr/voicedictation/API.html)
- [录音文件转写大模型文档](https://www.xfyun.cn/doc/spark/asr_llm/Ifasr_llm.html)
- [极速录音转写大模型文档](https://www.xfyun.cn/doc/asr/speedTranscription/API.html)
