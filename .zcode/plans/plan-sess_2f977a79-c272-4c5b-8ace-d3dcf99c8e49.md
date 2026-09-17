# AP Psych 一站式学习平台 v2 重构计划

## 0. 定位与已确认决策

课外一站式 AP 心理学习平台（学习内容 + 闪卡 + 题库练习/模考 + 班级学情分析）。

| 决策项 | 结论 |
|---|---|
| 登录 | Microsoft Entra SSO（学校 Teams 账号）为主 + 保留本地邮箱密码登录（应急/开发） |
| 角色 | admin / teacher / student 三级；`ADMIN_EMAILS=leo.huo_27@tsinglan.org` 自动成为管理员 |
| 租户 | 收敛为单一学校租户（不再每次注册建租户） |
| AI | 全部在本地工作流，平台代码零 AI 功能；将来 AI 改 FRQ/AAQ 是后续独立版本 |
| 闪卡 | 去掉 0–5 数字熟练度，改三档自评：熟练 / 模糊 / 不熟悉；大按钮正反面翻卡 |
| 题库 | CSV 和 JSON 双通道导入 + 后台手动录入；所有题进统一题池，按 tag 批量筛选组卷 |
| 题型 | MCQ（自动判分）+ 主观题 FRQ/AAQ/EBQ（参考答案+评分标准，学生自评不判分） |
| 练习模式 | 不限时即时反馈 + 限时模考（倒计时、一次性出分） |
| 数据 | 运行数据（用户/复习记录）全部重置；794 个概念详解内容保留，从源文件重新导入 |
| 技术栈 | 不换栈。Go 1.25 + Gin + GORM + SQLite；React 19 + Vite + TS；做架构现代化 |

## 1. 后端重构

### 1.1 代码结构拆分（现为 875 行单文件 `backend/app.go`）

```
backend/
  app.go               App 结构、NewApp、AutoMigrate、启动种子
  router.go            路由注册 + 中间件装配
  middleware.go        auth()、requireRole(admin/teacher)、CORS
  auth.go              本地登录/注册 + Entra OIDC 流程
  handlers_concepts.go 单元/主题/概念/内容
  handlers_flashcards.go 复习队列、复习事件、三档状态
  handlers_practice.go 题库、练习卷、作答、判分、错题本
  handlers_analytics.go 学情统计
  handlers_admin.go    用户/角色、题目管理、导入、组卷
  models.go            重构后的数据模型
  importer.go          保留（概念内容导入，适配三档）
  question_import.go   CSV/JSON 题目解析 + 校验
  util.go
```
第 1 步先做纯结构拆分（行为不变、测试保持绿），再逐模块改造。

### 1.2 数据模型（全新建库，无迁移负担）

- **保留**：Course / Unit / Topic / Concept / ConceptContent / ImportRun；**删除**：Card 表（空壳）、UserConceptState 的 Mastery/ManualRating 字段、ReviewEvent 的 MasteryBefore/After/CardID。
- **Tenant**：保留表，启动时固定建一个学校租户，所有用户归属它。
- **User**：+`Provider`(local|entra)、+`EntraOID`(uniqueIndex, nullable)；Role ∈ admin|teacher|student。
- **UserConceptState**：三档 `Status`(proficient|fuzzy|unknown，无记录=未标记) + ReviewCount + ShortTermReview + LastReviewedAt。
- **ReviewEvent**：append-only 保留，Response 改为 proficient|fuzzy|unknown。
- **Tag**：ID + Name(uniqueIndex)。
- **Question**：Type(mcq|subjective)、Stem、`Materials` JSON（主观题材料/文章/来源，AAQ/EBQ 用）、`Choices` JSON（MCQ 选项）、AnswerKey、Explanation、`Parts` JSON（主观题分小问：label/prompt/referenceAnswer/rubric[]，EBQ 的 A/B/C 三问）、UnitID/TopicID（可选挂接单元树，索引）、Status(draft|published|archived)、Source(manual|csv|json)、CreatedBy；Tags 多对多（question_tags）。为将来 AI 改卷预留完整结构化字段。
- **PracticeSet**（练习卷）：Title、Description、Mode(instant|exam)、TimeLimitSec（exam 模式）、Status、CreatedBy；Items = PracticeSetItem(SetID, QuestionID, Position)。
- **PracticeAttempt**：UserID、SetID、Mode 快照、StartedAt、DeadlineAt(exam 模式服务端截止)、FinishedAt、Score(MCQ 部分)、TotalMCQ。
- **PracticeAnswer**：AttemptID、QuestionID、ChoiceKey(MCQ)、TextAnswer(主观题)、IsCorrect、SelfRating(proficient|partial|weak，主观题自评)、AnsweredAt。
- **错题本**：不建表，由 PracticeAnswer 按用户聚合生成（含"后来已答对"状态）。

### 1.3 API 设计

**认证**：`GET /api/auth/entra/login`（302 跳 Microsoft，state+PKCE）→ `GET /api/auth/entra/callback`（换 token、JIT 建用户、按 ADMIN_EMAILS 赋角色、签自家 JWT 回前端）；保留本地 login/register；`GET /api/me` 增加 role/provider。Entra 未配置 env 时端点返回未启用，前端隐藏按钮，本地开发不受影响。

**概念/闪卡**：units/concepts 端点保留；`PATCH /api/concepts/:id/status` 替代 0–5 rating；review/next 队列（fuzzy/unknown 优先）与 review/events 保留并改三档；dashboard 端点改为三档统计。

**练习（学生）**：
- `GET /api/practice/sets`（已发布卷+个人进度）、`GET /api/practice/sets/:id`（题目，不含答案）
- `POST /api/practice/attempts`（开始，exam 模式返回 DeadlineAt）
- `POST /api/practice/attempts/:id/answers`：instant 模式即时返回对错+解析；exam 模式只存储不给反馈
- `POST /api/practice/attempts/:id/finish`：exam 模式统一切判分出分；主观题返回参考答案+评分标准供自评
- `POST /api/practice/attempts/:id/answers/:qid/self-rating`（主观题三档自评）
- `GET /api/practice/wrongbook`、`GET /api/practice/stats`（按 unit/topic/tag 正确率）

**管理端**：
- `GET/POST/PATCH /api/admin/questions`（tag/unit/状态筛选+搜索、单题编辑）
- `POST /api/admin/questions/import/preview`（CSV/JSON 上传，解析校验，返回逐行错误高亮）→ `POST /api/admin/questions/import/commit`（两步式入库）；提供模板下载
- `GET/POST/PATCH /api/admin/sets`（组卷：tag 批量筛选加题、手动调序、发布/下架）
- `GET /api/admin/analytics/overview`（全班活跃、刷题量、正确率、按单元正确率、闪卡三档分布、薄弱概念 Top N）、`GET /api/admin/analytics/users/:id`
- `GET/PATCH /api/admin/users`（角色调整，仅 admin）；保留概念内容编辑与 import 端点（仅 admin）

**权限矩阵**：student=学习/闪卡/练习/错题本/个人统计；teacher=+题库/组卷/全班学情；admin=+用户角色/概念内容/导入。

### 1.4 Entra 集成细节

- OIDC 授权码流（Go 后端 confidential client，新增依赖仅 `golang.org/x/oauth2`，Microsoft v2 端点，PKCE；用户信息经 Graph /me 取 oid/email/name）。
- 新 env：`ENTRA_TENANT_ID` / `ENTRA_CLIENT_ID` / `ENTRA_CLIENT_SECRET` / `ENTRA_REDIRECT_URI`（如 https://域名/api/auth/entra/callback）；可选 `ENTRA_ALLOWED_DOMAINS`（限制 tsinglan.org）。
- `ADMIN_EMAILS`（逗号分隔）启动 bootstrap + SSO 首登赋 admin。
- 倒计时改为 `AP_EXAM_DATE` env 可配置（2026-05 已过，下次考试日期待定）。

## 2. 前端重构

### 2.1 架构升级

- 引入 **react-router**（真实 URL 路由：/login /dashboard /learn /flashcards /practice /practice/sets/:id /wrongbook /profile /admin/*）与 **TanStack Query**（缓存/失效/加载态）。
- `main.tsx`（1226 行）拆为 pages/ + components/ + hooks/ + lib/；API 客户端按域拆分。
- 保留 Notion 风格设计 token 体系（styles.css 扩展），清理死 CSS（.unit-health 等未用类）与未用资源（hero.png、typescript.svg 等）。

### 2.2 页面

**学生侧**：
- /login：Microsoft SSO 大按钮（Entra 已配置时）+ 本地登录折叠区
- /dashboard：三档进度环、刷题/闪卡活动、考试倒计时（可配置）、薄弱区提醒
- /learn：单元→主题→概念浏览+搜索；概念详情含三档标记按钮
- /flashcards：正反面大翻卡（点击/空格翻面），三个大按钮 熟练/模糊/不熟悉（键盘 1/2/3）；会话配置（范围/数量/随机或顺序）；模糊/不熟悉自动进短期队列
- /practice：练习卷列表（题数/模式/个人进度）
- /practice/sets/:id：instant 模式逐题作答即时反馈（对错+解析）；exam 模式倒计时+题号导航面板+交卷一次性出分；主观题文本框作答→展示参考答案与评分标准→三档自评
- /wrongbook：错题本（筛选、"后来已掌握"标记）
- /profile：个人信息/主题/语言

**管理侧 /admin**（按角色显隐）：
- /admin/questions：题库管理（筛选/搜索/单题编辑器，支持 MCQ 与主观题含材料/小问/评分标准）
- /admin/questions/import：上传 CSV/JSON → 预览校验（错误行高亮）→ 确认导入；模板下载
- /admin/sets：组卷编辑器（tag 批量选题、手动加题、排序、发布/下架、限时配置）
- /admin/analytics：全班总览 + 学生明细
- /admin/users（仅 admin）：用户列表、角色调整
- /admin/content（仅 admin）：现有概念内容管理翻新 + 导入状态

## 3. 数据重置与种子

1. 备份后删除 `data/app.db`（备份文件 gitignore，不入库）。
2. 启动时 AutoMigrate 全部新表 → 导入器跑 keyterms.md + unit0/1.md + OPML + ai-enrichment.compact（794 概念 ready，逻辑不变）→ 建学校租户。
3. 后端测试同步改造：保留 794 概念 canary 断言，新增 Entra 回调（mock）、题目导入解析、双模式判分、权限矩阵测试。前端暂以 `tsc && vite build` 为门禁。

## 4. 实施阶段（小提交序列）

1. 后端结构拆分（行为不变，测试绿）
2. 数据模型 v2 + 重置 DB + 导入器适配三档
3. 单租户收敛 + 角色体系 + ADMIN_EMAILS bootstrap
4. Entra OIDC 登录（未配置时优雅降级）
5. 题库模型 + CSV/JSON 两步导入 + 题目管理 API
6. 组卷 + 双模式作答/判分 + 主观题自评 + 错题本 API
7. 学情分析 API
8. 前端架构升级（router + query + 拆分）
9. 学生页面（dashboard / learn / flashcards 三档翻新）
10. 练习 + 模考 + 错题本页面
11. 管理后台页面
12. 整站清理（死代码/资源）+ Docker/env 文档更新 + README

## 5. 需要你提供（不阻塞开发）

- Entra 应用注册信息：Tenant ID、Client ID、Client Secret、重定向 URI（需先定部署域名，OnePanel 现有域名即可）
- 未到位前系统以本地登录完整可用，Entra 按钮自动隐藏；env 加入 `.env.example`

## 6. 本版非目标

- 平台内任何 AI 功能（含 AI 改卷——将来独立版本）
- 阶段性解锁、多学校多租户、正式作业发布与排名
- OPML 颜色语义（必背/易混/情景题标注）深度挖掘（后续可做）