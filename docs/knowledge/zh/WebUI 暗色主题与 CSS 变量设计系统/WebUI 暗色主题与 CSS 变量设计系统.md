---
kind: frontend_style
name: WebUI 暗色主题与 CSS 变量设计系统
category: frontend_style
scope:
    - '**'
source_files:
    - internal/webui/static/style.css
    - internal/webui/static/index.html
    - internal/webui/static/app.js
    - internal/webui/static/datepicker.js
---

本仓库的前端样式集中在 internal/webui/static/ 目录，采用纯 CSS + 原生 JavaScript 的轻量方案，通过 Go embed 打包进二进制。整体风格为深色系（deep slate + indigo/cyan 渐变强调），围绕 CSS 自定义属性构建统一的设计令牌体系。

**样式系统与约定**
- 设计令牌：所有颜色、圆角、阴影、缓动曲线均定义在 :root 下的 CSS 变量中（如 --bg、--accent、--radius、--ease），形成单一事实来源，便于全局换肤或扩展浅色主题。
- 命名空间：类名使用 BEM 风格的短前缀组合（.nav-*、.card、.btn-*、.provider-item、.stat、.modal、.toast、.history-toolbar），无框架约束，按页面区域划分。
- 组件化结构：以 .card 作为内容容器，配合 .form-grid 双列表单布局；按钮通过 .btn-primary / .btn-secondary / .btn-danger / .btn-small 修饰符复用；状态栏 .status-bar 固定底部并带毛玻璃效果。
- 图标策略：卡片标题与统计块的装饰图标全部内联 SVG data URI，避免额外资源请求，并通过 data-icon="voice|provider|stats|history" 等语义化属性选择器注入。
- 动画与交互：统一的缓动函数族（--ease、--ease-spring、--ease-soft）贯穿 hover、focus、pageIn、cardIn、modalIn、toastIn 等关键帧；卡片入场采用 nth-child 延迟实现交错动画。
- 响应式：仅含一个断点 @media (max-width: 640px)，将双列表单切换为单列、堆叠 provider 操作区、纵向排列 stats 卡片。

**关键文件**
- internal/webui/static/style.css — 全部样式与设计令牌
- internal/webui/static/index.html — 嵌入页面的 HTML 骨架
- internal/webui/static/app.js — 前端交互逻辑（配置保存、提供商 CRUD、历史分页、状态轮询）
- internal/webui/static/datepicker.js — 自定义日期选择器 UI

**开发者规范**
- 新增颜色/尺寸/阴影请优先写入 :root 变量，禁止硬编码魔法值。
- 新组件沿用现有类名前缀与修饰符模式，保持 hover/focus 过渡一致。
- 图标统一以内联 SVG data URI 形式通过 data-icon 属性注入，不引入外部字体图标库。
- 动画时长与缓动曲线从 --ease* 变量读取，确保节奏统一。