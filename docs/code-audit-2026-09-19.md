# 代码审计：2026-09-19

对前后端全部未提交改动（约 50 个已跟踪文件 +6400 行、34 个未跟踪新文件）做的一次系统审计。方法：四个并行只读审计（后端安全 / 后端正确性 / 前端 / 构建与仓库卫生），发现逐条人工核实后修复。本文记录**已修复项**、**有意不修项**和**数据结论**，供回溯。

## 一、安全类（已修复）

### P0 考试模式参考答案泄露（`backend/handlers_practice.go`）
旧链路：`selfRateAnswer` 不检查 attempt 状态 → 学生在考试进行中把主观题自评为 `weak` → `wrongBook` 对任何 `SelfRating == "weak"` 的主观题直接 `revealAnswer`（含参考答案与评分标准）→ 回到 `submitAnswer` 改写答案再交卷，绕过所有考试门禁。修复三层：
1. `selfRateAnswer`：exam 模式且未 finish 时返回 409；同时拒绝给 MCQ 答案自评（400）。
2. `wrongBook`：主观题条目只有当所属 attempt「非 exam 模式，或已 finish」才视为已揭示；`weak` 评分未揭示时跳过。
3. 新测试 `TestExamSelfRatingGatedUntilFinish` 用哨兵字符串 `SECRET-REFERENCE-ANSWER` 验证泄露路径关闭、finish 后正常揭示。

### P1 生产模式管理员工引导未加门禁（`auth.go` / `app.go` / `entra.go`）
- 开放注册时任何人抢先注册 `ADMIN_EMAILS` 里的地址即可变管理员：现在生产模式下，只有配置了 `REGISTRATION_INVITE_CODE` 时注册才授予 admin，否则降级为 student。
- 「首个注册者成为管理员」「启动时提升第一个用户」两个开发兜底现在都被 `productionMode()`（backend 包新增，镜像 main 的判定）关闭；生产环境的 admin 引导只剩 ADMIN_EMAILS 启动提升与 Entra 首登提升两条受控路径。
- 登录对未知邮箱做 dummy bcrypt 比对，消除时序侧信道。
- `main.go` 新增 `warnInsecureDevConfig`：开发模式下 JWT_SECRET 为空/默认值、或配置了 Entra 但 `ENTRA_ALLOWED_DOMAINS` 为空时打启动警告。

### 仓库卫生（P0 级）
- `data/logs/seed-token.txt` 内有真实可用的 JWT（原 `.gitignore` 只有 `*.log`，一个 `git add .` 就会提交凭证）；已将 `data/logs/`、`*.exe`（根目录 53MB 的 `ap-psych-dev.exe`）、`.scratch/`、`data/public/`、`data/backups/` 全部加入 `.gitignore`。
- `frontend/public/mascot/` 是 9 月已移除的栗子形象的遗留目录（引用它的文档和路由都已删除），本地删除，不提交。

## 二、正确性类（已修复）

### 错题本驱逐语义（`handlers_practice.go`）
旧行为：任何「无结论」的最新答案（考试中未评分的 MCQ、未自评的主观题）都会把题目**挤出**错题本。修复后规则：
- MCQ：`IsCorrect == nil`（未评分）→ 保持原状态不动；正确 → 驱逐；错误 → （重新）收入。
- 主观题：`proficient` → 驱逐；`weak`（已揭示）→ 收入；`partial`/未评 → 保持原状态。
另：最新答案排序加 `id` 决胜；O(n²) 冒泡换 `sort.Slice`。新测试 `TestWrongBookLatestAnswerSemantics` 覆盖四段转移。

### 统计跨 attempt 重复计数（`handlers_analytics.go` / `handlers_practice.go`）
班级总览、按单元、逐学生 answered/correct/accuracy、学生端 byUnit/byTopic、`wrongAnswers` 原来把**每次 attempt** 都计入，重刷 5 遍 = 一题计 5 次。现在全部改为「每 (学生, 题目) 最新一条答案」去重（与错题本同一推导，子查询按 `answered_at desc, id desc` 取最新）；attempt 总数保持累计（衡量练习量）。`TestPracticeStatsLatestPerQuestion` 固定语义。`weakConcepts` 补 `users.deleted_at is null` 过滤。

### CSV BOM（`question_import.go`）
Excel「CSV UTF-8」导出会在首个表头单元格前加 U+FEFF，`TrimSpace` 不去它 → `type` 列找不到 → 主观题行静默按 MCQ 解析、报出误导性校验错误。现剥离 BOM；`TestQuestionImportCSVBOM` 覆盖。

### 列表页标记绕过事件日志（`handlers_concepts.go`）
`setConceptStatus`（Terms 列表三档标记）原来只改 state、不记 `ReviewEvent`、不增 `review_count`，导致 dashboard 今日复习数/最近动态/已复习数全部漏计。现在与翻卡标记走同一事务：记事件、`review_count + 1`、更新 `last_reviewed_at` 与短期复习队列；清空标记（`""`）仍只是重置，不算复习。`app_test.go` 断言更新为 ReviewCount=2、TodayReviews=2。

### 后端测试 30 秒/个 → 1.3 秒/个
元凶：`EnrichFromCompact/V2/Cards` 与 `applyNotes` 每个词条**各自提交两次事务**（约 800 词条 × Windows fsync ≈ 30s），每个 `NewApp` 测试都要全量导入一遍。改为整轮单事务后：单测试 30.4s → 1.3s，全套 `go test ./backend` 14 分钟+超时 → **39.5s 通过**。

### 其他后端
- nil-slice 违例（JSON `null` → `[]`）：`importStatus` 的 `runs`/`byUnit`、dashboard `recent`、import commit `failures`、以及 `stripAnswer` 输出的 `tags`（`loadSetQuestions` 统一 `ensureTags` + `stripAnswer` 兜底空数组）。
- `stripAnswer`：存储的 `parts` JSON 反序列化失败时原来原样透传（可能带参考答案），现删除该键。
- `reviewEventBatch`：批内事件共享同一时间戳导致「每概念最新事件」不稳定，改为逐事件 `time.Now()`。
- `PracticeAnswer.QuestionID` 加独立索引（analytics/wrongbook 的 join 用）。
- 首次启动导入失败原来静默吞掉，现打日志。
- `gofmt` 了审计发现的 4 个未格式化文件。

## 三、前端（已修复）

- **WritingRunner**：`finish()` 无 `catch`（失败无任何提示）→ 补错误横幅 + `busy` 重入守卫；作文无防丢失守卫 → 未提交时 `useBlocker` + `beforeunload` 弹确认，并新增 localStorage 草稿（按 attempt+题目为键，提交即清除，刷新可恢复）；交卷后的 done 页新增主观题参考答案展示 + 三档自评（此前写作考试的主观题永远没有自评入口，进不了错题本）。
- **Flashcards 按键**：按住 1/2/3 会以 ~30 次/秒自动重复，几秒内标记完 200 张卡并经 reviewQueue 永久污染调度数据 → 加 `event.repeat` 与修饰键守卫；Enter/Space 在按钮聚焦时不再劫持为翻面。
- **auth 切换清缓存**：`onAuthed`、主动 logout、401 强登出三条路径现在都 `queryClient.clear()` —— 共享电脑上换账号登录不会再闪现上一个用户的 dashboard/错题本缓存。
- **PracticeRunner**：交卷后失效 `['wrongbook']`（新错题即时可见）；`finish` 加 `busy` 守卫；完成页回顾区为主观题补三档自评按钮。
- **i18n/杂项**：问候语全角逗号按语言切换；admin 三个列表页的裸 `status`/误用 `t.unpublish` 改为 `t.draft/published/archived`；`sourceLabel` 补 `ai-enrichment-v2.compact` 与 `cards.compact`（新增 `sourceCards` 词条）；删除死代码 `StatBucket`、`PracticeStats`、`initials()`、`shortLabel()`、`RunnerQuestion.myCorrect`。

## 四、794 vs 797 之谜（数据结论，非 bug）

`keyterms.md` 解析出 **797** 条，入库后是 **794** 个概念：Unit 5 里 `Delusions`、`Subjective well-being`、`Hallucinations` 各出现两次（逐字重复词条，slug 相同被 upsert 合并）；topic 头 42 个但有一个为空，实际建 41 个。所以 AGENTS.md 的「6 units / 41 topics / 794 concepts」与 `importer_test.go` 的 797 解析断言**都正确**，无数据丢失。若希望源文件层面去掉这 3 条重复，改 `keyterms.md` 即可（注意同步改 importer_test 的 797）。

## 五、有意不修（记录在案）

- 限流、PKCE、请求体大小上限、密码强度策略：本地单租户场景收益低，部署公网前再议。
- `/files` 静态目录无鉴权（文件名含 unix 时间戳，可枚举性弱）：属于按设计公开的教师资料区，保持现状。
- 前端 `tsconfig` 未开 `strict`（开启会引出一批修复，单独一轮做更稳妥）。
- 考试计时用客户端时钟（服务端 410 + resume 自动判分已兜底）；管理员弹窗误点背景丢表单；头像上传无压缩——低危，留待后续。
- LIKE 通配符未转义（搜索 `%` 会全量匹配）：影响仅限搜索噪声。

## 五点五、审计后追加修复（用户复核时指出）

- **日历"无截止日期事件无限延续"**（审计四路均未抓到）：`Dashboard.tsx` 的 `eventCovers` 把 `endDate == null` 当作"延伸到无穷远"，事件从开始日起在此后每一天都显示圆点。修复为无 endDate 只占开始日当天（`end = endDate ?? date`）。后端查询本来就是这个语义，纯前端 bug。
- **日历事件类型细分**：按用户口径重排为 作业 `assignment`（homework/project 归此）、小测 `quiz`、单元测试 `unit-test`（新）、考试 `exam`（新，midterm/final 归此）、假期 `holiday`、活动 `event`；旧的泛化 `assessment` 启动时一次性迁移为 `exam`（`migrateCalendarKinds`），校验与 UI 选项中退役。新增图例/配色（unit-test 青绿 #0e9888，exam 沿用红色）与 zh/en 词条。

## 六、验证

- `go build ./...`、`go vet ./...`、`gofmt` 全部干净；`go test ./backend` 全套通过（39.5s，含 4 个新审计测试）。
- `npx tsc --noEmit` 0 错误；`npm test` 16/16；`npm run build` 成功。
- 新测试文件：`backend/practice_audit_test.go`（考试自评门禁 / 错题本语义 / BOM 导入 / 统计去重）。
