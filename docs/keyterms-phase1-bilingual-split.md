# Keyterms Phase 1：中文在前的双语拆分（2026-09-19）

状态：已完成并导入数据库。前置背景见 `docs/keyterms-content-audit.md`。

## 本轮决定（与 Leo 确认）

- definition 双语两段：**中文段在前、英文段在后**。
- 四字段定位：定义 = 一句话；例子 = 展开；易错点 = 只在真正有混淆点时写（目标 5-10% 的卡片有）；笔记 = 额外信息/链接（后续截图等）。
- 双语允许自然混排（以 Leo 的 OPML 口吻为标杆）；字段允许留白，不强求填满。
- 内容写作可用 inline markdown：`**粗**`、`*斜*`、`***粗斜***`、`++下划线++`、`~~删除~~`、`` `code` ``、`[文字](url)`、整块 `![](url)` 图片（见 `frontend/src/lib/inlineMarkdown.ts`）。

## 改动清单

1. **`data/sources/ai-enrichment-v2.compact`（新源文件）**：由 `ai-enrichment.compact` 机械拆分生成（生成脚本 `.scratch/gen_v2.py`）。规则：行内「EN / 中文」拆成两条同键行，中文行在前。1604/3176 行完成拆分，零歧义。v1 原文件未动（保留原始来源）。
2. **`backend/importer.go`**：
   - compact 解析抽成共享函数 `parseCompactEntries`（v1/v2 共用）；
   - 新增 `EnrichFromCompactV2`：只覆盖 `source ∈ {ai-enrichment.compact, ai-enrichment-v2.compact}` 的词条内容，**unit0.md / unit1.md / OPML 的人类笔记永不覆盖**；幂等（v2 识别自己的输出，重复导入收敛）。
3. **`main.go`**：新增 `--reimport` 启动参数（强制重跑全部导入器后退出）。注意 `RunAll()` 平时只在 concepts 表为空时执行（app.go），种子数据修正必须走源文件 + `--reimport`，直接改数据库会在重灌时丢失。
4. **`backend/importer_test.go`**：`TestEnrichFromCompactV2` —— 验证中文段在前、AI 来源被更新、人类来源不被触碰、重复运行收敛。

## 导入与验证结果

`go run . --reimport` 执行后（数据核对脚本见 `.scratch`）：

- 来源分布：ai-enrichment-v2.compact 563 / unit1.md 145 / unit0.md 49 / OPML 37，总 794。
- slash 混排段落残留：0；中文段在前的双语对：563/563。
- 抽样正确：insomnia、reciprocal-determinism 等均为 [中文段, 英文段]。
- 22 条无中文定义未在本轮处理（多为人类来源，留待逐单元修订 + glossary 一起补）。

## 术语中文 glossary（同轮完成）

产出 `docs/glossary-review.md`：794 条全覆盖的「英文术语 → 中文叫法」审阅表。来源构成：

- 你的 OPML/笔记提取 181 条（69 条标题直取 + 112 条上下文提取；含少量你笔记里的玩笑写法如「河马校园」，审阅时顺手改）；
- 现有中文定义开头截取 47 条；
- AI 草译 566 条（标 ⚠重点审）。

挖掘脚本：`.scratch/mine_glossary.py`（标题/括号模式）+ `.scratch/mine_context.py`（带自检断言的上下文挖掘）+ `.scratch/merge_glossary.py`（合并出审阅表）。审阅后定稿将落为 `data/sources/glossary.json` 供后续管线使用。

## 术语中文 glossary（2026-09-19 二次重做）

第一版用启发式挖掘+教科书译名，被否（太书本、太多英文、没结合笔记）。第二版改为：**完整通读** `data/sources/合订本，一口气看到大结局 Unit 0-5.pdf`（Irene 精修版，116 页，已存入 sources）+ 新版 OPML 全文，794 条中文叫法逐条以两份笔记为锚（用户原生叫法优先，如「多看你一眼效应」「战逃僵」），每条带笔记依据与来源标注。产出 `docs/glossary-review.md`（838 行，中文在前的四列表）。手工映射存于 `.scratch/zh_map_*.json`，生成器 `.scratch/build_review.py`（覆盖校验 794/794）。文末记录了 6 条笔记内部矛盾待用户拍板（Perceptual set/Mental set 撞名、OPML 4.6 SDT 定义错位等）。提炼稿：`.scratch/irene_clean.txt` / `.scratch/opml_full.txt`。

## 遗留 / 下一步

- Leo 审阅 `docs/glossary-review.md` → 落库 glossary。
- ~~PDF~~ 合订本已到位；截断的 AP Psych Slides.pdf 若重下可做补充对照。
- 试点 2.1 Perception 逐条修订：以合订本词条为底稿（定义一句话 + 你的例子 + 5-10% 易错 + 笔记放链接），人工对一条验收后批量。
