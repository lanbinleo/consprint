# 2026-09-20 练习模块系统化升级（MCQ 题组 / AAQ / EBQ）

本轮把练习模块对齐 College Board 2025 新版考试形态：Section I 的 75 道 MCQ（含共享材料题组）、Section II 的 AAQ（一篇文章 + 6 个小问，7 分）与 EBQ（三篇来源材料，论点/证据/解释展开为 5 个作答框，7 分）。分六个小步提交，全部测试通过。

## 官方题型依据

- MCQ：独立题与题组（一段 passage 挂多道小题）混排。
- AAQ（Article Analysis Question）：1 篇摘要研究文章，A–F 六个小问（研究方法 / 变量操作定义 / 统计解释 / 伦理 / 泛化 / 论证应用），F 小问 2 分。
- EBQ（Evidence-Based Question）：3 篇来源材料；论点 + 两条证据 + 两次解释论证，脚手架展开即 5 个作答框，合成一篇论证文章。

## 数据模型（backend/models.go）

- 新增 `Stimulus` 共享材料：`{id, title, kind: passage|article|sources, documents: [{title, text}]}`。MCQ 题组 = 1 篇 passage；AAQ = 1 篇 article；EBQ = 3 个 sources。文档正文是 markdown，可内嵌图片。
- `Question` 新增 `format`（仅 subjective：`frq|aaq|ebq`，存量回填 frq）与 `stimulus_id`（MCQ 是否题组由它派生）；`Question.Concepts` many2many `question_concepts`（题目关联知识点，payload 只带 `{id, term}` 轻量 chips）。
- `QuestionPart` 增加可选 `points`（College Board 分值，仅展示对照）。
- `PracticeAnswer` 新增 `parts`（`[{label, text}]` 按小问作答）与 `partRatings`（按小问自评）；小问自评聚合镜像到 `selfRating`，错题本规则不变（任一 weak 入本、全 proficient 出本）。
- 迁移全部幂等（`migrateQuestionFormats`），794 概念 canary 不受影响。

## API 变化

- 管理：`GET/POST/PATCH /api/admin/stimuli`（列表带搜索与引用计数）；`POST /api/admin/question-images`（题图上传，嗅探类型、5MB 上限、`qimg-{unix}-{rand8}` 不可猜文件名，复用笔记上传的共享助手）；question create/PATCH 接受 `stimulusId/format/concepts[]`，list 支持 `stimulusId/format/topicId` 筛选。
- 学生：`GET /api/practice/attempts`（本人做题记录列表：卷名、进度、得分）；`submitAnswer` 接受 `{parts:[{label,text}]}`（校验 label ⊆ 题目 parts、至少一问非空）；self-rating 接受 `{parts:[{label,rating}]}`；exam 未结束禁止揭示的规则覆盖到小问级（测试钉死）。
- 覆盖聚合：`practiceSetDetail`/admin `getSet` 附带 `coverage`（units/topics/concepts 分布 + 未关联数）；sets 列表附 `unitIds/formats`（一条 group by 查询，顺带消掉了 N+1 计数）；题目行附带 concepts chips。

## 学生端

- Runner 重构为渲染单元：`buildRenderItems()` 把卷面折叠成 独立题 / MCQ 题组 / AAQ / EBQ 四种单元（相邻同 stimulus 的 MCQ 归组；同组里的 aaq/ebq 复合题独立成单元）。
- MCQ 题组：左材料右一次一题（贴近 Bluebook），可拖拽调宽（`SplitPane`，移动端自动堆叠），题号导航带同组徽标。
- AAQ：左文章右 6 小问，每问一个自适应增高 textarea（`AutoGrowTextarea`），失焦/防抖整体保存，带保存状态；instant 模式提交即揭示参考答案+rubric+小问级三档自评。
- EBQ：左三篇 Source 标签页，右 5 框脚手架，同样式作答与揭示。
- 新增 `/practice/history` 做题记录（完成→只读复盘 `/practice/history/:attemptId`，进行中→跳回 runner 继续）；复盘 UI 抽成共享 `AttemptReview` 组件。
- `/practice` 列表筛选：搜索 / Unit / 题型构成 / 我的状态；卡片显示覆盖单元与 AAQ/EBQ chips。

## 管理端

- 题目编辑器：subjective 下选 FRQ/AAQ/EBQ（切到 AAQ/EBQ 自动播种官方小问脚手架与分值）；Stimulus 选择/就地新建（多文档、支持粘贴/拖拽传图）；补上 Topic 下拉；Concepts 搜索多选 chips；Tags 接 `/api/admin/tags` datalist。
- 组卷从 modal 换成全页编辑器 `/admin/sets/:id`：左元数据+覆盖面板（Unit/Topic 条形、concept chips、未关联警示），中卷面按材料自动分块（原生 HTML5 拖拽排序 + 整块上下移，无新依赖），右题库浏览器（接上一直没用的 search 参数 + unit/topic/type/format/tag 筛选 + 完整题目预览 + 单题/整组添加）。

## JSON 导入扩展

- 行级新字段：`format`、`stimulusId`（引用已有材料）、`stimulus`（内联材料：按 title 查找或创建，多行共享同一篇 passage 自动去重）、`concepts`（概念 id 数组，全部必须存在）。
- 题干/材料/材料文档中的 `data:image/*;base64` 内嵌图片在入库时落地为 `/files/qimg-*` 文件并替换 URL（≤5MB；解析失败保留原文留给人工复核）。
- CSV 通道与模板不变（AAQ/EBQ 结构放不进 CSV）。

## 图片方案（为什么不是 base64/外链）

- base64 存 JSON 会让含 10 张图的卷子每次进 attempt 都拉数 MB、无法单独缓存、击穿 8MB 导入上限。
- 外链有失效/防盗链/离线风险，限时考试中途断图不可接受。
- 落地文件 + `/files` 静态服务（已有路由，`<img>` 带不了 Bearer 是有意设计）+ 不可猜随机文件名，可缓存、可离线、题图在发布前不可被枚举。

## 测试

- 后端新增 5 个测试：Stimulus CRUD 与校验；AAQ 全流程（建题→组卷→小问作答→揭示门控→小问自评聚合进错题本）；exam 模式门控 + 做题记录归属；题图上传（嗅探、随机名、重名不碰撞、/files 可取）；JSON 导入（内联材料按 title 去重、缺材料报错、base64 落地）。
- 前端新增 `questionGrouping` 纯函数测试（分组、范围对齐、渲染单元折叠）；全量 vitest 35 通过；tsc/build 通过。

## 后续（本轮明确不做，结构已预留）

- 教师/AI 批改写入端：per-part 打分评语未来经导入通道落到 `PracticeAnswer`（结构已按小问粒度就位）。
- EBQ 整篇/双模式切换、Dashboard 练习卡片、错题本界面重做。
