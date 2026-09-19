# 笔记资源 tab + 概念内容行内 Markdown（2026-09-19）

本轮交付两块能力：Notes 页面的「资源 tab + iFrame 嵌入」体系（后台可配置），以及概念内容（定义/例子/易错点/笔记）的行内 Markdown 渲染。

## 迭代二（同日）：体验反馈修复

1. **侧边栏管理入口**：原先整个管理区入口指向 `/admin/analytics` 且标签叫「学情分析」，切到其他子页后高亮消失。现改为单一「管理」入口（`to: '/admin'` 前缀匹配，所有 `/admin/*` 子页保持高亮）。
2. **概念编辑器**：移除行内 Markdown 语法提示行（与保存按钮挤在一起、占位）。编辑/预览切换保留。
3. **后台用户管理**：新增行内改名（每行 PenLine 按钮 → 输入框 + 确认/取消，Enter 保存 / Esc 取消）。走既有 `PATCH /api/admin/users/:id` 的 `name` 字段，后端无需改动。
4. **搜索框间距**：`.search` 统一加 `margin-bottom: 8px`，与下方列表/面板留出距离。
5. **笔记页改版**：
   - 标题去重：大标题改为「学习资源」（`notesTitle`），页眉小字仍是「笔记」；导航项不变。
   - 去掉外层滚动：`.notes-page` 高度锁定为视口内（`calc(100vh - 64px)`，配合 workspace padding；移动端 `calc(100dvh - 24px)`），整页 flex 列布局，只有 iframe 内部滚动。同时放开 `max-width: none`，阅览区占满宽度。
   - 单行单元选择器：`.note-items` 改为单行横向滚动 pill 组（segmented 视觉），不再折成两行。
   - 元数据下移/收纳：描述（description）移到 iframe 下方一行省略号小字；原 iframe 顶栏（label+按钮）整行删除。
   - 全屏展开：工具栏右侧新增 Maximize 按钮 → 应用内 fixed 全屏 overlay（非浏览器原生 Fullscreen API，便于做动效）：`noteExpandIn` 缩放淡入 220ms，顶栏含标题 + 新窗口打开 + 退出按钮，Esc 可退出，展开期间锁定 body 滚动。


## 一、笔记资源 tab

### 数据模型（backend/models.go）

- `NoteResource` = Notes 页的一个 tab：`Title / Description / Status(draft|published|archived) / Position`。
- `NoteResourceItem` = tab 内一条资源：`Label / URL / Kind(embed|pdf|image|link) / Position`。
- 种子（`seedNoteResources`，仅空表时执行，幂等）：
  - 「Leo 的复习笔记」：Unit 0–5 六条幕布分享链接（embed）。
  - 「示例：PDF 讲义」：指向 `/files/ap-psych-sample.pdf`（pdf），验证内链文件嵌入，可在后台删除。

### API

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/api/note-resources` | 登录用户 | 仅 published，按 Position 排序，items 归一化为 `[]` |
| GET | `/api/admin/note-resources` | teacher/admin | 全部状态（含 items） |
| POST | `/api/admin/note-resources` | teacher/admin | 新建 tab（`{title, description, status, position?, items:[{label,url,kind}]}`） |
| PATCH | `/api/admin/note-resources/:id` | teacher/admin | 指针式 patch；`items` 非空数组则整体替换 |
| DELETE | `/api/admin/note-resources/:id` | teacher/admin | 硬删除（代码库首个 DELETE 路由，事务内连 items 一起删） |
| POST | `/api/admin/note-resources/upload` | teacher/admin | multipart `file`，≤25MB，内容嗅探仅收 PDF/PNG/JPG/GIF/WebP，落盘 `data/public/`，返回 `{url:"/files/<name>"}` |

- URL 校验：仅 `http(s)://` 与站内绝对路径 `/`；`javascript:`、协议相对 `//` 一律 400。
- CORS 允许方法新增 `DELETE`。

### 静态路由（backend/router.go）

- `/files` → `data/public`（公开只读。iframe/img 子资源无法携带 Authorization 头，故必须公开；单租户校内场景可接受）。
- 顺带修复既有隐患：`/avatars`、`/fonts` 现由 Go 二进制直接服务（此前仅 Vite dev 提供，生产会 404→index.html）。
- `App.PublicDir` 与 db 同目录（`data/public`），测试应用落在 `t.TempDir()` 内互不污染。

### 前端

- `pages/Notes.tsx`：tab 条（下划线激活、可横滚）→ tab 内多 item 时 `.segmented` 切换（如 Unit 0–5）→ 按 kind 渲染：
  - `embed`/`pdf`：全高 iframe + 常驻「新窗口打开」按钮；5 秒内未见 load 事件则自动切换降级卡片（含重试按钮）。注意：X-Frame-Options 拒绝的跨域页面通常仍会触发 load（渲染为空白错误页），父页面无法可靠检测，所以外链按钮常驻。
  - `image`：居中大图，点击新窗口打开；`link`：链接卡片。
- `pages/admin/NoteResources.tsx`（`/admin/notes`，staff 可见）：列表（条目数/状态/上下移/发布/删除）+ 模态编辑器（标题、描述、条目行 = 名称 + 类型下拉 + URL + 上传按钮 + 排序/删除）。上传成功回填 `/files/...` 内链。

### 幕布（Mubu）嵌入结论

搜索与社区案例表明 share.mubu.com 大概率发送 `X-Frame-Options`（同类文档站普遍如此），浏览器会拒绝第三方 iframe。方案已内置降级：若实际被拒，tab 内仍显示单元切换 + 「新窗口打开」按钮。需要浏览器实测确认；若幕布彻底无法内嵌，后续可选方案是从幕布导出 OPML/Markdown 后平台原生渲染。

## 二、概念内容行内 Markdown

- `lib/inlineMarkdown.ts`（纯函数，零依赖）：`**加粗** *斜体* ***粗斜*** ++下划线++ ~~删除线~~ `代码` [链接](url) ![图片](url)`；URL 白名单 http(s)/站内路径；分隔符需紧贴非空白字符（`2 * 3 * 4` 不会变斜体）；`imageOnlyBlock` 判定整块单图。
- 渲染走 React 元素（`components/InlineMarkdown.tsx`），无 innerHTML，天然防 XSS。
- `components/RichContent.tsx`：段落块经 `InlineMarkdown` 渲染，整块单图渲染为 `<figure>`；术语面板与闪卡卡背自动生效，**数据层零改动**（Markdown 原文存于现有 paragraph block）。
- 后台概念编辑器（admin/Content.tsx）：新增「编辑/预览」切换（预览与学生端渲染管线一致）+ 语法提示。
- 实现 bug 记录：解析器曾用模块级 `/g` 正则 + 递归调用，共享 `lastIndex` 导致重复匹配死循环 OOM；改用 `matchAll`（内部克隆正则）修复。

## 测试与验证

- 后端：`backend/handlers_notes_test.go`（种子幂等、staff CRUD、学生 403、draft 不可见、kind/URL 校验、上传嗅探、/files 404 不落 SPA）；`go test ./...` 全绿；`go build` 通过。
- 前端：`vitest`（^3.2.4，注意 v5 在本机 Windows/Node 22.15 worker 崩溃）`src/lib/inlineMarkdown.test.ts` 13 例全绿；`npm run build` 通过。
- 手动验证（http://localhost:8080）：
  1. 登录 → 笔记页：应见「Leo 的复习笔记」（Unit 0–5 切换）与「示例：PDF 讲义」（内嵌 PDF 预览）。
  2. 幕布 tab：若 iframe 空白即被拒，点右上「新窗口打开」。
  3. 管理后台 → 笔记资源：新建 tab、上传 PDF/图片、拖排（上/下移）、发布/删除。
  4. 管理后台 → 概念内容：任选概念，在定义里写 `**bold** *it* ++u++ [AP](https://apstudents.org) ![img](/files/ap-psych-sample.pdf 截图)` 之类，切「预览」，保存后在术语目录/闪卡查看。
