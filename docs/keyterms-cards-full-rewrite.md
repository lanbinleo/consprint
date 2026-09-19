# Keyterms 卡片全量重写（2026-09-19 完成）

状态：**794/794 张卡全部完成并导入系统**。目标达成。前置：Phase 1 双语拆分（`keyterms-phase1-bilingual-split.md`）、Phase 2 试点（`keyterms-phase2-pilot-2.1.md`）。

## 交付物

- **`data/sources/cards.compact`**：794 张卡的唯一权威源。底稿=合订本（Irene 精修版）+ OPML，逐张人工重写。
- 分单元草稿存于 `.scratch/cards_{sp,u1a,u1b,u1c,u2a,u2b,u3a,u3b,u3c,u4a,u4b,u5a,u5b,u5c,u5d}.compact`（合并脚本校验 794 无缺无重无多余）。
- 导入走既有 `EnrichFromCards`（权威覆盖，needs_review=false，confidence=0.9）。

## 卡片规范（最终版，与 2.1 试点一致）

- def 第一块 = `中文名词。中文一句话`（名词取 glossary.json，去括号；GABA 这类标准拉丁名保留原文）；第二块 = 英文一句话。
- ex = 笔记原例子展开（咖啡因、大猩猩、香菜奶茶、囤囤鼠……），自然混排。
- pit = 只写真混淆：全库 45 张带 pit（6%，落在约定的 5-10%），全部来自 Leo 笔记里的黄色「易错」标注（IV/DV、随机抽样vs分配、corr≠causation、负强化≠惩罚、habituation≠感官适应、FAE vs AOB、OCD vs OCPD、双相I/II、精神分裂≠人格分裂等）。
- note = 跨单元连接与助记（3M、68-95-99.7、Weird/Wild/Worried、OCEAN、SSRI 反复出现）。
- 没内容就留空。

## 验证结果（2026-09-19 18:03 reimport）

- source 分布：cards.compact **794/794**（unit0/unit1/OPML/AI 来源全部被权威覆盖）。
- 中文词前缀定义：793/794 通过（唯一例外 GABA，名字本身是拉丁字母，按设计）。
- pit 密度 6%。
- Go 测试：TestEnrichFromCards / TestEnrichFromCompactV2 / TestParseKeyterms 全过。
- 线上 API 抽查（运行中的旧服务器，同库）：794/794 返回 cards 来源；Social loafing=「社会懈怠。贡献没法量化就偷懒。」等格式正确。

## 后续可选

- Leo 浏览器全库过一遍，发现个别叫法/例子要改的直接改 `cards.compact` 再 `go run . --reimport`（几秒钟）。
- AP Psych Slides.pdf（截断那份）如重新下载，可对 U5/U1 做事实核查对照——本轮内容以合订本+OPML 为准，未经第三源复核。
- 建议下一轮 commit 拆分：cards.compact + importer + glossary + docs。
