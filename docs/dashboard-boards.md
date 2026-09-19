# Dashboard 内容化改造（2026-09）

本轮把首页从「统计仪表盘」改造成「长期学习门户」：公告栏、问候卡、作业/考试日历、收藏词条，统计只留一根考试倒计时进度条。（期间短暂引入过吉祥物「栗子」，后按用户要求整体移除，见文末第三轮记录。）

## 改了什么

### Dashboard（`frontend/src/pages/Dashboard.tsx` 重写）

- **删除**：每日复习/过去 24 小时两张图表、4 个指标卡、熟练度进度带、薄弱主题提醒条、弱单元/弱主题卡。后端 `/api/dashboard/trends`、`/api/dashboard/alerts` 端点保留未动（admin 学情分析仍在用同族查询）。
- **新布局**：
  - Row1：公告板（大卡）+ 栗子问候卡（时段问候 + 每日心理一句 + 出处）
  - Row2：迷你月历（彩色事件点、hover tooltip、点日期弹 Modal 看当天事项）+ 收藏词条卡（点击跳 `/terms?concept=<id>` 深链定位）
  - 尾部：考试倒计时细进度条（`meta.examDate` 未配置时自动隐藏）
- 统计按「长期学习、不制造焦虑」原则全部退场；连续天数等以后想加回栗子卡是一行事。

### 公告（新后端）

- 模型 `Announcement{Title, Body, Pinned, Status(draft/published/archived), CreatedBy, AuthorName}`。
- 学生 `GET /api/announcements`（published、置顶优先、limit 10）；staff `POST/PATCH/DELETE /api/admin/announcements(/:id)`。
- 正文支持彩色 marker 富文本：`==柠檬高亮==`、`==tangerine|橘色|mint|sky|pink==`，加粗/斜体/链接照旧（`lib/inlineMarkdown.ts` 扩展，渲染为 `<mark class="mk-*">`）。
- 空库自动种子一条栗子的欢迎公告（幂等）。

### 日历（新后端，与练习卷解耦）

- 模型 `CalendarEvent{Title, Date(YYYY-MM-DD), EndDate?, Time?(HH:MM), Kind(assignment|quiz|unit-test|exam|holiday|event), Note}`，日期存本地字符串不受时区影响。
- 学生 `GET /api/calendar?month=YYYY-MM`（重叠该月，含跨月多天事件）；staff `GET/POST/PATCH/DELETE /api/admin/calendar`。
- 颜色：作业=橘、考试=红、小测=紫、假期=绿、活动=蓝。

### 每日一句（新后端）

- 模型 `Quote{TextZh, TextEn, Source}`，种子 40 条双语心理一句话（Miller、Ebbinghaus、Stroop……）。
- 挂在 `GET /api/dashboard/summary` 的 `quote` 字段，按上海时区 `年内第几天 % 40` 确定性轮换，全站同一天同一句。

### 词条收藏 star（新功能）

- `UserConceptState` 加 `Starred` 列（表本就按用户物化全量概念行，零成本）。
- `PATCH /api/concepts/:id/star {starred:bool}`；`GET /api/dashboard/starred`（limit 8，带单元/主题）。
- 入口：Terms 详情面板标题旁星形按钮（乐观更新）；闪卡正面右上角小星（点击不翻面）。
- `/api/review/next` 现在返回 `{...concept, state}` 扁平结构（向后兼容，老字段不变）。

### 管理后台：公告与日历（`/admin/boards`，teacher+admin）

- 公告编辑器：标题 + 正文 + 工具条（加粗、5 色蜡笔按钮——选中文本自动包语法）+ 实时预览 + 置顶开关 + 草稿/发布。
- 日历编辑器：标题、起止日期、可选时间、类型选色、备注。

### 栗子点位（图片未放时全部优雅降级：隐藏图片保留文案）

登录页 hero、Dashboard 问候卡、错题本空(cheer)、练习/写作无卷(read)、笔记空/无条目(sleep)、公告空(sleep)、闪卡完成(cheer+每日轮换夸奖句)、练习出分 ≥80% cheer / 其余 think、写作完成(cheer)、SSO 回调等待(wave)、笔记内嵌失败(think)。

- 素材规范与生成指南：**[docs/lizi-mascot-guide.md](lizi-mascot-guide.md)**（放图到 `frontend/public/mascot/` 即生效）。
- 侧栏 Logo 自动优先 `lizi-icon.png`（`components/Logo.tsx` 回退 AP 紫标）。
- 生产二进制已挂 `/mascot` 静态路由（`router.go`）。

## API 一览（新增）

| 方法 | 路径 | 角色 | 说明 |
| --- | --- | --- | --- |
| GET | `/api/announcements` | 登录用户 | 已发布公告（置顶优先） |
| GET/POST/PATCH/DELETE | `/api/admin/announcements(/:id)` | teacher/admin | 公告管理 |
| GET | `/api/calendar?month=YYYY-MM` | 登录用户 | 当月事件（默认当月） |
| GET/POST/PATCH/DELETE | `/api/admin/calendar(/:id)` | teacher/admin | 日历管理 |
| GET | `/api/dashboard/starred` | 登录用户 | 收藏词条（8 条） |
| GET | `/api/dashboard/recent` | 登录用户 | 最近复习词条（每个概念一行，8 条） |
| PATCH | `/api/concepts/:id/star` | 登录用户 | 收藏/取消收藏 |
| GET | `/api/dashboard/summary` | 登录用户 | 新增 `quote` 字段 |

## 验证

```powershell
cd backend && go test ./...        # 含新的 TestBoardsSeedAndQuote / TestAnnouncementCRUD / TestCalendarCRUD / TestConceptStar，794 canary 不变
cd frontend && npm test            # inlineMarkdown 含 marker 高亮 16 例
cd frontend && npm run build       # 产物已构建
go build -o ap-psych-dev.exe .     # 二进制交付
```

浏览器自测路径：登录 → 首页（公告/栗子/日历/收藏/倒计时）→ 术语表收藏一个词回到首页看收藏卡 → 管理后台「公告与日历」发一条带 `==tangerine|…==` 高亮的公告并建两个日历事件 → 回首页看公告渲染和日历点。

## 第二轮：官方素材入库 + 克制收敛（2026-09-19）

你提供的 5 张定稿图已识别、压缩并上线，同时按「克制使用」收敛了点位。

**素材**（1024×1024 透明 PNG → 512×512 WebP q82，共约 113KB，`frontend/public/mascot/`）：

| 原图 | 文件 | 内容 |
| --- | --- | --- |
| (7) | `lizi-idle.webp` | 全身端坐微笑（兜底） |
| (12) | `lizi-icon.webp` | 猫头特写开心笑 |
| (10) | `lizi-sleep.webp` | 蜷着打盹 Zzz |
| (8) | `lizi-read.webp` | 坐着看橘色的书 |
| (11) | `lizi-think.webp` | 托腮思考 + 问号 |

**代码变更**：
- `Mascot.tsx`：变体集合改为实际的 `idle | sleep | read | think | icon`，资源链 `variant.webp → variant.png → idle.webp → idle.png → 隐藏`。
- `Logo.tsx`：**还原纯紫色 AP 标**（栗子只当吉祥物，不做 logo），favicon 不动。
- 克制收敛——撤掉：SSO 回调等待猫、笔记内嵌失败 think 猫、Dashboard 公告空/收藏空 sleep 猫（同屏已有问候卡）、笔记 tab 内层空猫。
- 保留点位（每屏最多一只）：登录 hero（icon）、Dashboard 问候卡（idle）、练习/写作无卷（read）、笔记页空（sleep）、错题本空 / 复习完成 / 写作完成 / 出分 ≥80%（icon）、出分 <80%（think）。

**验证**：`npm test` 16/16、`npm run build` 通过；服务端 `/mascot/*.webp` 全部 200，SPA 已切新 bundle `index-diV6-wEm.js`。

## 第三轮：吉祥物整体移除（2026-09-19）

用户决定不再使用栗子形象：**图片与吉祥物相关代码全部移除，功能全部保留**，首页问候区留纯文字版待用户后续自行设计。

- 删除 `frontend/public/mascot/`（5 张 WebP）与 `components/Mascot.tsx`；后端 `/mascot` 静态路由移除。
- 全部点位撤图留文：登录页欢迎语改为文字胶囊（`.auth-greet`）、首页问候卡改为纯文本 `greet-card`（问候 + 每日一句）、练习/写作/笔记空态回退为图标 + 文字、错题本空/复习完成/写作完成/出分鼓励改为纯文字（`.empty-state.celebrate` / `.result-celebrate`）。
- i18n 去栗子化（双语）：`announcementsEmpty`、`praise1/3/6`、`authBubble`。
- 种子公告去栗子化（标题/正文/作者改为 "Psych Hub"），并同步更新了开发库 `data/app.db` 中已播种的那条公告。
- 删除 `docs/lizi-mascot-guide.md`；Logo/favicon 始终未动（紫色 AP 标）。

保留不变：公告（彩色 marker 富文本）、日历、收藏 star、每日一句 quote、考试倒计时条、Admin「公告与日历」管理页及其全部 API。

## 第四轮：首页精细化（2026-09）

按用户反馈对首页做了一轮产品与视觉层面的打磨，布局改为稳定网格：

- **Row1（固定 264px）**：左侧问候卡改为左对齐——日期小字 + 大号时段问候 + 每日一句（中段，可滚动）+ 底部三个学习数据格（今日复习 / 连续天数 / 短期队列，点击跳闪卡）；右侧公告卡改为**轮播**：一次一条，6 秒自动切换、悬停暂停、底部圆点可手动跳转，横向滑动过渡。
- **Row2**：考试倒计时条从页尾上移到问候/公告与日历之间；`AP_EXAM_DATE` 未配置时后端默认 `2027-05-14T12:00:00+08:00`（2027 年 5 月 14 日周五），倒计时不再隐藏。
- **Row3（固定 372px）**：迷你月历 + 收藏词条。日历日期去掉灰底按钮样式，纯数字、无边框，hover 轻微变灰并带 140ms transition（有事件的日期保持紫底 hover）；收藏列表超高内部滚动。
- **Row4（固定 264px）**：新增「最近复习」（新端点 `GET /api/dashboard/recent`，按概念去重、带三档状态 pill 与复习时间）与「待巩固词条」（复用 `/api/dashboard/alerts`，`weakConcepts` 改为带单元/主题/状态的行结构，前端旧字段无消费方）。
- 卡片内空态统一为 `.board-empty` 纯文字（不再嵌套 `.empty-state` 卡片）。
- ≤1080px 时固定高度解除、栏目纵向堆叠。
