# Audio Talk AI 官方网站 / Official Website

Audio Talk AI 项目的宣传官网。纯静态、零构建、中英双语。

A static, zero-build, bilingual marketing site for the Audio Talk AI project.

## 目录结构 / Structure

```
website/
├── index.html            # 中文主页（默认）/ Chinese (default)
├── index.en.html         # 英文主页 / English
├── assets/
│   ├── style.css         # 样式（设计系统 + 组件 + 响应式 + 动画）
│   ├── app.js            # 交互（滚动动画 / 复制命令 / 移动菜单 / OS 标签页 / 灯箱）
│   ├── favicon.svg       # 站点图标（语音波形 logo）
│   └── images/           # 产品截图（从 docs/ 复制）
├── robots.txt
└── README.md             # 本文件
```

## 本地预览 / Preview locally

无需任何构建。任选一种方式启动静态服务器：

No build step required. Serve the folder with any static server:

```bash
# Python 3（系统自带）
cd website
python3 -m http.server 8080
# 打开 http://localhost:8080

# 或 Node
npx serve website

# 或直接双击 index.html 用浏览器打开（部分交互需要 http 环境，推荐上面的方式）
```

## 部署 / Deploy

`website/` 是一个自包含的静态站点目录，可整体部署到任意静态托管。

The `website/` directory is self-contained and deploys to any static host.

### GitHub Pages

1. 把 `website/` 的内容推送到一个分支（例如 `gh-pages`），或推到仓库根目录后在 Settings → Pages 选择目录。
2. Source 选 `Deploy from a branch`，分支选 `gh-pages` / `(root)`，目录选 `/website`（或你放置的目录）。
3. 几分钟后访问 `https://<user>.github.io/<repo>/`。

### Gitee Pages

1. 推送代码到 Gitee 仓库。
2. 仓库 → 服务 → Gitee Pages → 部署目录填 `website`，分支选 `master`。
3. 启动后访问 Gitee Pages 提供的域名。

> 内容更新后，Gitee Pages 需手动点「重启」；GitHub Pages 推送即更新。

### 其他 / Others

Cloudflare Pages、Vercel、Netlify 等：构建命令留空，输出目录 / publish directory 设为 `website`。

## 语言切换 / Language toggle

- 中文页默认入口：`index.html`
- 英文页：`index.en.html`
- 顶部导航的「中 / EN」按钮在两页之间互链。两种语言共享同一份 `style.css` 与 `app.js`，无 JS 也能渲染内容，且各自有独立可索引的 URL（对 SEO 友好）。

## 更新内容 / Updating content

直接编辑两个 HTML 文件即可。常见更新点：

- **截图**：`assets/images/` 下的文件按**真实内容**命名为 `tui.png`、`webui-config.png`、`webui-history.png`。
  > ⚠️ **注意：仓库 `docs/` 里的源文件名是误导的**（历史上截图被覆盖到错误的文件名上，名字没改回来）。从 `docs/` 重新拷贝时**务必按下表对应**，否则会再次错位：
  >
  > | `docs/` 源文件 | 实际内容 | 网站目标文件 |
  > | --- | --- | --- |
  > | `screenshot-webui.png` | TUI 终端 | `tui.png` |
  > | `screenshot-webui-history.png` | Web 配置页 | `webui-config.png` |
  > | `screenshot-tui.png` | Web 转写历史 | `webui-history.png` |
  >
  > 顺带一提：仓库的**中文 README 引用是对的**（与上表一致），**英文 README 的引用是错的**（它按文件名字面引用，恰好踩中错位）。如需，可把英文 README 的引用改成与中文一致。
- **服务商列表**：编辑 `#providers` 区段；流式 / 批量分别在两个 `.provider-group` 内。
- **安装命令**：编辑 `#quickstart` 区段下各 OS 的 `.os-panel`；如改了 release 版本号，记得同步三处命令。
- **仓库链接**：导航栏、Footer、快速开始里的 URL。

中英两页结构完全一致，改了一页记得对照改另一页，避免内容漂移。

## 设计说明 / Design notes

视觉与产品本体（TUI / WebUI）保持一致：

- 深色基底（`#0d1117`）、绿色（`#3fb950`）作主操作 / 「已支持」状态、琥珀色（`#f0b429`）作指标 / 高亮、青色（`#58a6ff`）作链接。
- 等宽字体用于代码与统计数字，呼应开发者工具的调性。
- 尊重 `prefers-reduced-motion`：开启系统「减少动态效果」时动画自动降级。

## 许可证 / License

与主项目一致：GPL v3.0，禁止商用。
