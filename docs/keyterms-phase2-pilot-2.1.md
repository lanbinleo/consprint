# Keyterms Phase 2：卡片重写试点 2.1 Perception（2026-09-19）

状态：24 张卡已导入数据库，等 Leo 浏览器过目验收；验收后按同一规范批量推进其余单元。

## 本轮产出

1. **`data/sources/glossary.json`**：审定的 794 条「概念ID → 中文叫法」映射落库（来源：Leo 审定通过的 docs/glossary-review.md）。后续翻译/内容工作的术语锚点。
2. **`data/sources/cards.compact`**：卡片新源文件，试点 2.1 Perception 24 张。底稿=合订本（Irene 精修版）+ OPML，全部人工重写，不是 AI 批量产物。
3. **`backend/importer.go` → `EnrichFromCards`**：cards.compact 的权威导入器——**覆盖任何旧来源**（包括 unit0/unit1/OPML/AI），因为每张卡都是人工精修的最高优先级内容。confidence=0.9、needs_review=false。已接入 RunAll 与 `--reimport`。
4. **测试**：`TestEnrichFromCardsOverwritesAnySource`（验证权威覆盖两种旧来源、中文块在前）。

## 卡片规范（本轮实际执行版）

- **def**：第一块 = 「中文名词。中文定义一句话」（名词取 glossary.json 审定名，去括号），第二块 = 英文一句话。例：`相似性。外观相似（颜色、形状、大小）的物体，容易被知觉为同一组。` + `Objects that look alike tend to be grouped together.`
- **ex**：你的原例子展开（乱序汉字、大猩猩、问路被换人、斗鸡眼…），单块，自然混排。
- **pit**：只在真有混淆/连接时写——2.1 的 24 张里只有 1 张（Perceptual set vs Mental set），约 4%。
- **note**：跨单元连接与助记（BU/TD 口诀、双眼线索清单、Phi phenomenon 不考）。
- 适度 markdown：`**加粗**` 标对比词、`*斜体*` 标问题句式。
- 留白是特性：没有 pit/note 就空着。
- 词名调整：Similarity 按 Leo 2026-09-19 示例从「相似律」改为「相似性」，glossary.json 已同步。

## 接口备注

此前报的「`?topic=` 过滤失效」系我测试时参数名写错：handler 与前端（Terms.tsx:45）一致使用 `topicId`，`/concepts?topicId=…` 验证返回正确的 24 条，接口本身无需修改。

## 验证结果

`go run . --reimport` 后：concept_contents 中 source=cards.compact 共 24 条；抽样 perceptual-set / top-down-processing / apparent-movement 均为中文定义在前、例子含原梗、pit 稀疏、note 有连接；needs_review 全部为 0。旧服务器进程读同一数据库，浏览器刷新即可看到新卡片。

## 下一步

1. Leo 浏览器验收 2.1（术语页 / 闪卡翻卡看渲染效果，重点：markdown 是否生效、双语块观感、留白是否舒服）。
2. 验收通过后按批推进：建议顺序 2.2 → 2.3-2.7（U2 内存大）→ U1 → U3 → U4 → U5；每单元一个 cards 增量批次 + per-unit review。
3. AP Psych Slides.pdf（截断那份）如重新下载，可补 U5/U1 fact-check 对照。
