# Keyterms 内容审计与改进方案（2026-09-19）

状态：审计完成；方案已拍板并部分落地（Phase 1 双语拆分已完成，见 `docs/keyterms-phase1-bilingual-split.md`；glossary 审阅表已产出 `docs/glossary-review.md`）。数据来源：`data/app.db` 只读快照 + `data/sources/ai-enrichment.compact` + 新版 OPML（`E:\Downloads\AP Psychology(2).opml`）。审计脚本在 `.scratch/`（audit_content.py / audit_round2.py / audit_pdf.py），报告 JSON 在 `.scratch/report_*.json`。

## 一、现状量化（794 条全量扫描，非抽样）

### 来源构成
| 来源 | 条数 | 占比 | 集中在 |
|---|---|---|---|
| ai-enrichment.compact | 563 | 71% | U2-U5 几乎全部（120/139/123/160） |
| unit1.md（真笔记） | 145 | 18% | U1（142 条） |
| unit0.md（真笔记） | 48 | 6% | Science Practices（47 条） |
| AP Psychology Notes.opml | 37 | 5% | 散布 |
| manual | 1 | — | — |

### 已确认的三个问题
1. **强行填满**：AI 的 563 条 100% 四字段全满（def+ex+pit+note），compact 文件里 794/794 无一留白。对比真笔记条目：examples 空 206、pitfalls 空 225、notes 空 156——**空缺全部集中在人类来源，是健康的**。note 字段废话率最高（多为 "AP asks you to…" 式应试套话）。
2. **双语三种风格并存**：
   - 563 条 AI 定义是 `EN / 中文` 挤在同一段（前端原样显示一行，阅读体验差）；
   - 真笔记是自然中英混排多段（如 Scatterplot：EN 定义一段 + "Correlational study 画出data的方式" 一段）；
   - 22 条 definition 完全无中文（清单见 `.scratch/report_round2.json`，集中在 SP 早期批次和 U1）；
   - ex/pit/note 三个字段：仅 236/213/258 条含中文，其余纯英文——早期 AI 批次只有 definition 双语，后期批次全双语，风格断裂明显。
3. **翻译腔**：AI 内容是"教科书腔+机翻腔"，与 OPML 里Leo本人的口吻（自然混排、鲜活例子）差距大。长度不是问题（definition 均值 95 字符，最长词条 640），价值密度才是。

概念硬伤：12 条人工抽检未发现事实性错误，但 U5 障碍分类、U1 神经部分尚未系统核查（PDF 缺失，见下）。

## 二、两份新材料评估

- **OPML（新版）**：与仓库副本节点数相同（2431），仅小改。内容为 Leo 的结构化笔记树，约 7.6 万字，中英混排。**最佳用法：口吻标杆 + 真实例子库 + 中文术语叫法锚点**（如 Perceptual set=思维定势），不是批量定义来源。
- **AP Psych Slides.pdf**：**文件损坏——是截断的下载**。141MB 内无 xref/trailer/%%EOF，页面树对象丢失，pymupdf 打开 0 页；14:41 后大小未变。**需要重新下载**，之后可按单元提取文本层做权威对照。

## 三、改进方案（待拍板）

### Phase 0 — 规范决定
- A. 双语形态：definition 改为两段（EN 一段 + 中文一段），与前端现有渲染和真笔记风格兼容；废除 ` / ` 单段式。ex/pit/note 不强制逐句对译，以 OPML 口吻做自然混排。
- B. 留白政策：字段允许为空；AI 输出中无价值的 pit/note 走"删除清单"而非静默删。
- C. 中文术语 glossary：从 OPML 提取"术语→Leo 的中文叫法"映射表，作为全库翻译锚点。

### Phase 1 — 机械修复（无需 AI，低风险，可先做）
- 563 条 ` / ` 定义拆两段；22 条无中文定义补译。
- 产出 per-unit before/after review md → 人工确认 → 走现有导入管线。

### Phase 2 — 逐单元 AI 修订（tools/ 本地工作流，遵守 AI cost guardrails）
- 每单元一批，输入 = 现内容 + OPML 对应章节 +（PDF 文本层，待补）+ glossary。
- 任务：① 用 Leo 口吻重写翻译腔 ② fact-check ③ 删废话 ④ 用真笔记例子替换 AI 编造例子。
- 输出 compact diff → `data/ai-output/` → 两步导入 → needs_review 标记 → per-unit review 文档。
- 建议先试点 2.1 Perception（OPML 有现成好材料，便于对齐口吻），验收后再批量。

### Phase 3 — 专项核查
- 高风险清单 fact-check：U5 障碍/治疗、U1 神经、U4 理论归属（对照 slides）。
- 新增 canary：① 无 ` / ` 段落残留 ② glossary 中文术语覆盖率。

### 待拍板
1. 双语形态：EN+ZH 两段 + ex/pit/note 自然混排？（建议：是）
2. 试点范围：2.1 试点还是直接 U5 开干？（建议：先 2.1）
3. PDF 重新下载后补一轮对照？（建议：是）
4. 是否先出 glossary 映射表过目？（建议：是）
