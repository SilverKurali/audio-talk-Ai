# 讯飞语音识别服务配置指南

> 本文仅覆盖讯飞系列服务。如需查看所有服务商（豆包、OpenAI、小米等）的配置指南，请参阅 [ASR 服务商配置指南](asr-providers-guide.md)。

本文档详细介绍如何在 Audio Talk AI 中配置讯飞开放平台的 7 种语音识别服务，包含手把手的开通流程和密钥获取教程。

---

## 第一步：注册讯飞开放平台账号

1. 打开 [讯飞开放平台官网](https://www.xfyun.cn/)
2. 点击右上角 **「注册」**
3. 使用手机号注册，完成短信验证
4. 登录后建议先完成 **实名认证**（个人认证即可使用免费额度）
   - 进入控制台 → 右上角头像 → **账号管理** → **实名认证**
   - 个人认证通常即时通过

---

## 第二步：创建应用

1. 登录 [讯飞开放平台控制台](https://console.xfyun.cn/)
2. 点击左侧菜单 **「我的应用」**
3. 点击右上角 **「创建新应用」**
4. 填写：
   - **应用名称**：随意取名，如 "Audio Talk AI"
   - **应用描述**：随意填写
5. 点击 **「创建」**
6. 创建成功后，你会看到应用列表，记下 **AppID**（每个应用都有唯一的 AppID）

> **AppID** 是所有讯飞服务都需要的公共参数。一个应用可以添加多个服务，每个服务的 APIKey/APISecret 不同。

---

## 第三步：开通语音服务并获取密钥

讯飞的 7 种语音服务使用 **3 种不同类型的密钥**，非常容易搞混。请先看下表确认你需要的服务用哪种密钥：

### 密钥类型速查表

| 服务 | 控制台开通入口 | 密钥类型 | 获取位置 |
|------|--------------|---------|---------|
| 星火语音识别大模型 | [服务页](https://console.xfyun.cn/services/bmc) | AppID + **APIKey** + **APISecret** | 应用详情 → 对应服务 |
| 语音听写（流式版） | [服务页](https://console.xfyun.cn/services/iat) | AppID + **APIKey** + **APISecret** | 应用详情 → 对应服务 |
| 实时转写大模型 | [服务页](https://console.xfyun.cn/services/rtasr) | AppID + **AccessKeyId** + **AccessKeySecret** | AccessKey 管理页面 |
| 实时转写标准版 | [服务页](https://console.xfyun.cn/services/lfasr) | AppID + **APIKey** | 应用详情 → 对应服务 |
| 录音转写标准版 | [服务页](https://console.xfyun.cn/services/lfasr) | AppID + APIKey + **SecretKey** | 语音转写 → 服务管理 |
| 转写大模型 | [服务页](https://console.xfyun.cn/services/lfasr_llm) | AppID + **AccessKeyId** + **AccessKeySecret** | AccessKey 管理页面 |
| 极速转写 | [服务页](https://console.xfyun.cn/services/ost) | AppID + **APIKey** + **APISecret** | 应用详情 → 对应服务 |

### 获取 APIKey + APISecret（星火大模型、语音听写、实时转写标准版、极速转写）

这类密钥绑定在你的 **应用** 上，获取步骤：

1. 在控制台左侧点击 **「我的应用」**
2. 找到你创建的应用 → 点击进入
3. 在应用详情页，你会看到已添加的服务列表
4. 找到对应服务（如"语音听写"），点击右侧的 **「APIKey」** 和 **「APISecret」** 查看
5. 如果该服务还没添加：点击 **「添加新服务」** → 搜索对应服务名 → 添加

> 首次添加服务时会提示领取免费额度，建议领取。

### 获取 AccessKeyId + AccessKeySecret（实时转写大模型、转写大模型）

这类密钥是 **账号级别** 的，不在应用详情里：

1. 在控制台右上角点击你的 **头像**
2. 选择 **「AccessKey 管理」**
3. 或直接访问：[AccessKey 管理页面](https://console.xfyun.cn/services/access-key)
4. 如果没有 AccessKey，点击 **「创建 AccessKey」**
5. 创建后你会看到 **AccessKeyId** 和 **AccessKeySecret** → 复制保存

> AccessKeyId/AccessKeySecret 是账号级别的，所有使用这类密钥的服务共用同一对。

### 获取 SecretKey（录音转写标准版专用）

录音转写标准版的密钥 **不是** 应用详情里的 APISecret，而是单独的 SecretKey：

1. 在控制台左侧找到 **「语音转写」** → **「服务管理」**
2. 或直接访问 [语音转写服务管理页](https://console.xfyun.cn/services/lfasr)
3. 在服务管理页面找到 **SecretKey** → 复制保存

> 很多用户在这里搞混，请记住：**录音转写标准版用的是服务管理页的 SecretKey，不是应用详情里的 APISecret。**

---

## 第四步：在 Audio Talk AI 中配置

你可以通过 TUI/WebUI 界面操作，也可以直接编辑配置文件。

**TUI 方式**：启动 `audio-talk-ai` → 找到 ASR 服务商 → 按 `a` 添加 → 选择讯飞对应服务 → 填入密钥

**WebUI 方式**：浏览器打开 `http://localhost:8391` → 点击「+ 添加服务商」→ 选择对应类型 → 填入密钥

**配置文件方式**：编辑 `~/.config/audio-talk-ai/config.toml`，添加 `[[asr_providers]]` 块（见下方示例）

---

## 服务对比与选择

| Provider 类型 | type 值 | 模式 | 最长时长 | 特点 | 免费额度 |
|---|---|---|---|---|---|
| 星火语音识别大模型 | `xfyun-spark` | 流式 | 60s | 中英+202种方言，动态修正 | 有 |
| 语音听写（流式版） | `xfyun-iat` | 流式 | 60s | 垂直领域，多方言免切，多候选 | 每日500次 |
| 实时转写大模型 | `xfyun-rtasr` | 流式 | 8h | Binary PCM，202种方言，说话人分离 | 有 |
| 实时转写标准版 | `xfyun-rtasr-std` | 流式 | 5h | Binary PCM，APIKey认证 | 有 |
| 录音转写标准版 | `xfyun-lfasr` | 批量 | 5h | 标准版，成熟稳定 | 最多50小时 |
| 转写大模型 | `xfyun-lfasr-llm` | 批量 | 5h | 202种方言/37语种免切 | 有 |
| 极速转写 | `xfyun-lfasr-fast` | 批量 | 5h | 1小时音频约20秒出结果 | 有 |

### 选择建议

- **日常语音输入**：推荐 `xfyun-spark`（最简单）或 `xfyun-iat`（支持方言免切）
- **专业领域识别**：使用 `xfyun-iat`，可设置 domain 为医疗/政务/金融等领域
- **方言免切识别**：`xfyun-iat` 支持 23 种方言免切；`xfyun-spark` 支持 202 种方言
- **长录音实时转写**：`xfyun-rtasr`（大模型版，8小时）或 `xfyun-rtasr-std`（标准版，5小时）
- **长录音批量转写**：`xfyun-lfasr`（标准版）、`xfyun-lfasr-llm`（大模型）或 `xfyun-lfasr-fast`（极速版）

---

## 配置示例

### 讯飞星火语音识别大模型（xfyun-spark）

**适用场景**：日常短语音输入，支持中文、英文和 202 种方言

**密钥类型**：AppID + APIKey + APISecret

```toml
[[asr_providers]]
name = "讯飞星火"
type = "xfyun-spark"
default = true
app_id = "你的AppID"
api_key = "你的APIKey"
api_secret = "你的APISecret"
dwa = "wpgs"  # 动态修正（可选，仅中文支持）
```

**参数说明**：

| 参数 | 必填 | 说明 |
|------|------|------|
| `app_id` | 是 | 应用的 AppID |
| `api_key` | 是 | 应用详情中的 APIKey |
| `api_secret` | 是 | 应用详情中的 APISecret |
| `dwa` | 否 | 动态修正：`"wpgs"` 开启（实时修正结果），留空关闭 |

> **星火大模型与语音听写的区别**：星火大模型基于大模型训练，数据量更丰富，支持 202 种方言免切。语音听写支持垂直领域和多候选。

---

### 讯飞语音听写流式版（xfyun-iat）

**适用场景**：日常语音输入、专业领域（医疗/政务/金融）、方言免切

**密钥类型**：AppID + APIKey + APISecret

**协议**：WebSocket + JSON Text，端点 `iat-api.xfyun.cn/v2/iat`

```toml
[[asr_providers]]
name = "讯飞听写"
type = "xfyun-iat"
app_id = "你的AppID"
api_key = "你的APIKey"
api_secret = "你的APISecret"
domain = "iat"       # 应用领域（见下表）
accent = "mandarin"  # 普通话，或 "xfime-mianqie" 方言免切
dwa = "wpgs"         # 动态修正（可选，仅中文）
```

**domain 参数说明**：

| 值 | 说明 | 备注 |
|---|------|------|
| `iat` | 日常用语 | 默认，无需额外开通 |
| `medical` | 医疗领域 | 需在控制台「高级功能」处开通 |
| `gov-seat-assistant` | 政务坐席助手 | 需在控制台开通 |
| `seat-assistant` | 金融坐席助手 | 需在控制台开通 |
| `gov-ansys` | 政务语音分析 | 需在控制台开通 |
| `gov-nav` | 政务语音导航 | 需在控制台开通 |
| `fin-nav` | 金融语音导航 | 需在控制台开通 |
| `fin-ansys` | 金融语音分析 | 需在控制台开通 |

> 未开通的领域会报错 **11200**。坐席助手适用于电话坐席场景，语音导航适用于机器与人对话场景，语音分析适用于事后录音质检场景。

**accent 方言免切**：设为 `xfime-mianqie` 可支持 23 种方言免切换识别：四川话、河南话、东北话、粤语、闽南话、山东话、贵州话、云南话、客家话、天津话、河北话、太原话、上海话、合肥话、南京话、皖北话、台湾话、甘肃话、陕西话、宁夏话、长沙话、南昌话、武汉话。需在控制台「方言/语种」处添加使用。

---

### 讯飞实时转写大模型（xfyun-rtasr）

**适用场景**：长会议实时记录（最长 8 小时）

**密钥类型**：AppID + **AccessKeyId** + **AccessKeySecret**

**协议**：WebSocket + Binary PCM 帧

```toml
[[asr_providers]]
name = "讯飞实时转写"
type = "xfyun-rtasr"
app_id = "你的AppID"
api_key = "你的AccessKeyId"
api_secret = "你的AccessKeySecret"
lang = "autodialect"  # 语种（见下表）
pd = ""  # 可选领域
```

**lang 语种参数**：

| 值 | 说明 |
|---|------|
| `autodialect` | 中文 + 英文 + 202 种方言自动识别（推荐） |
| `autominor` | 37 种语种免切识别 |

**pd 领域参数**（可选）：

| 值 | 说明 |
|---|------|
| `court` | 法律 |
| `finance` | 金融 |
| `medical` | 医疗 |
| `tech` | 科技 |
| `sport` | 体育 |
| `edu` | 教育 |
| `isp` | 运营商 |
| `gov` | 政府 |
| `game` | 游戏 |
| `ecom` | 电商 |
| `mil` | 军事 |
| `com` | 企业 |
| `life` | 生活 |
| `ent` | 娱乐 |
| `culture` | 人文历史 |
| `car` | 汽车 |

> **注意**：这里 `api_key` 填的是 **AccessKeyId**，`api_secret` 填的是 **AccessKeySecret**，不是应用详情里的 APIKey/APISecret。

**特点**：
- 流式 WebSocket + Binary PCM 协议，边说边出字
- 支持最长 8 小时连续录音
- 支持说话人分离（角色分离）
- 支持 202 种方言免切

---

### 讯飞实时转写标准版（xfyun-rtasr-std）

**适用场景**：长录音实时转写（最长 5 小时），轻量级方案

**密钥类型**：AppID + **APIKey**（注意不是 AccessKeyId）

**协议**：WebSocket + Binary PCM 帧

```toml
[[asr_providers]]
name = "讯飞实时转写标准版"
type = "xfyun-rtasr-std"
app_id = "你的AppID"
api_key = "你的APIKey"
lang = "cn"  # cn=中文，en=英文
pd = ""  # 可选领域参数
```

**lang 语种参数**：

| 值 | 说明 |
|---|------|
| `cn` | 中文（默认） |
| `en` | 英文 |

**pd 领域参数**：`court`、`finance`、`medical`、`tech`、`sport`、`edu`、`gov`

> **与大模型版的区别**：标准版使用普通 APIKey 认证（不是 AccessKeyId），不支持方言免切和说话人分离，最长 5 小时（大模型版 8 小时）。

---

### 讯飞录音转写标准版（xfyun-lfasr）

**适用场景**：长录音事后批量转写（最长 5 小时）

**密钥类型**：AppID + APIKey + **SecretKey**（注意不是 APISecret）

**协议**：HTTP POST（上传音频 → 轮询结果）

```toml
[[asr_providers]]
name = "讯飞录音转写"
type = "xfyun-lfasr"
app_id = "你的AppID"
api_key = "你的APIKey"
api_secret = "你的SecretKey"
pd = ""  # 可选领域参数
```

> **再次强调**：`api_secret` 填的是**语音转写服务管理页的 SecretKey**，不是应用详情里的 APISecret。获取方式见上方"获取 SecretKey"章节。

**pd 领域参数**：`court`、`edu`、`finance`、`medical`、`tech`、`sport`、`gov`、`game`、`ecom`、`car`

**转写耗时参考**：

| 音频时长 | 预计返回时间 |
|---------|------------|
| < 10 分钟 | < 3 分钟 |
| 10-30 分钟 | 3-6 分钟 |
| 30-60 分钟 | 6-10 分钟 |
| > 60 分钟 | 10-20 分钟 |

> 实际返回时间受音频时长和排队任务量影响。建议上传 5 分钟以上的音频。

---

### 讯飞转写大模型（xfyun-lfasr-llm）

**适用场景**：多语种长录音批量转写

**密钥类型**：AppID + **AccessKeyId** + **AccessKeySecret**

```toml
[[asr_providers]]
name = "讯飞转写大模型"
type = "xfyun-lfasr-llm"
app_id = "你的AppID"
api_key = "你的AccessKeyId"
api_secret = "你的AccessKeySecret"
pd = ""
```

> 使用 **AccessKeyId + AccessKeySecret**（在 AccessKey 管理页面获取）。

支持 202 种方言和 37 种语种免切识别。

---

### 讯飞极速转写（xfyun-lfasr-fast）

**适用场景**：快速批量转写

**密钥类型**：AppID + APIKey + APISecret

```toml
[[asr_providers]]
name = "讯飞极速转写"
type = "xfyun-lfasr-fast"
app_id = "你的AppID"
api_key = "你的APIKey"
api_secret = "你的APISecret"
pd = ""
```

1 小时音频约 20 秒出结果，适合快速批量处理。

---

## 批量模式说明

录音文件转写类的三个 provider（`xfyun-lfasr`、`xfyun-lfasr-llm`、`xfyun-lfasr-fast`）采用**批量模式**：

1. 按住热键录音，松开后音频自动上传到讯飞服务器
2. 等待服务端转写完成（参见上方耗时表）
3. 转写结果自动复制到剪贴板或上屏

与流式 provider 不同，批量模式在录音过程中**不会实时显示识别结果**。

---

## 错误码速查

| 错误码 | 含义 | 解决方案 |
|--------|------|---------|
| **11200** | 未授权或功能未开通 | 到控制台对应服务页开通试用或购买 |
| **11201** | 日调用量超限 | 联系商务提高每日调用次数，或等待次日重置 |
| **11202** | 秒级流控超限 | 降低请求频率，或联系商务提高并发 |
| **11203** | 授权过期 | 检查套餐有效期 |
| **10005** | AppID 授权失败 | 确认 AppID 正确，且已开通对应服务 |
| **26600** | 转写业务通用错误 | 检查请求参数是否正确 |
| **26625** | 服务时长不足 | 到控制台领取免费额度或购买 |
| **10200** | 读取数据超时 | 检查是否 10 秒未发送数据且未关闭连接 |

---

## 常见问题

### Q: 讯飞这么多服务，我该选哪个？

- 日常打字聊天 → `xfyun-spark`（配置最简单）
- 需要方言 → `xfyun-spark`（202种）或 `xfyun-iat`（23种免切）
- 需要专业领域 → `xfyun-iat`（医疗/政务/金融）
- 长会议实时记录 → `xfyun-rtasr`（8小时）
- 长录音事后转写 → `xfyun-lfasr` 系列

### Q: 星火大模型和语音听写有什么区别？

| | 星火大模型 | 语音听写 |
|---|---|---|
| 协议 | WebSocket | WebSocket |
| 方言 | 202 种 | 23 种免切 |
| 垂直领域 | 不支持 | 医疗/政务/金融等 |
| 多候选 | 不支持 | 支持（最多5个候选） |
| 动态修正 | 支持 | 支持 |

### Q: APIKey 和 AccessKeyId 有什么区别？

- **APIKey + APISecret**：绑定在具体应用上，在「我的应用」→ 应用详情中查看
- **AccessKeyId + AccessKeySecret**：账号级别，在右上角头像 → 「AccessKey 管理」中查看
- **SecretKey**：仅录音转写标准版使用，在「语音转写」→「服务管理」中查看

### Q: 配置好之后报错怎么办？

1. 先检查 `type` 拼写是否正确
2. 确认密钥类型是否正确（APIKey vs AccessKeyId vs SecretKey）
3. 确认已在控制台开通对应服务并领取免费额度
4. 运行 `audio-talk-ai --doctor` 检查配置

### Q: 免费额度用完了怎么办？

到讯飞控制台对应服务页面购买套餐包，或领取新用户礼包（录音转写标准版最多 50 小时免费时长）。

---

## 相关链接

| 资源 | 链接 |
|------|------|
| 讯飞开放平台控制台 | https://console.xfyun.cn/ |
| AccessKey 管理 | https://console.xfyun.cn/services/access-key |
| 星火语音大模型文档 | https://www.xfyun.cn/doc/spark/spark_zh_iat.html |
| 语音听写（流式版）文档 | https://www.xfyun.cn/doc/asr/ifasr_new/API.html |
| 实时转写大模型文档 | https://www.xfyun.cn/doc/asr/rtasr/development.html |
| 录音转写（标准版）文档 | https://www.xfyun.cn/doc/asr/voicedictation/API.html |
| 转写大模型文档 | https://www.xfyun.cn/doc/spark/asr_llm/Ifasr_llm.html |
| 极速转写文档 | https://www.xfyun.cn/doc/asr/speedTranscription/API.html |
| 错误码查询 | https://www.xfyun.cn/document/error-code |
