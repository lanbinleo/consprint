# 异步交互改造与测试数据（2026-09-19）

本轮改造目标：把「点击后等网络」的同步交互改为乐观/后台异步，提升体感；顺带修复折叠侧边栏的两个视觉问题，并为练习/考试功能灌入可用的测试题库。所有相关代码位置、API 变更、种子数据与验证记录都在本文档，供后续参考。

## 1. Glossary 标记熟练度：乐观更新

**文件**：`frontend/src/pages/Terms.tsx`（`mark()` / `applyState()`）

- 点击 proficient / fuzzy / unknown 后**立即**更新列表 pill 和详情面板（本地写入 query cache + `active` state），随后才发 `PATCH /api/concepts/:id/status`。
- 成功：用服务端返回的 state 覆盖本地值，并失效 `['concepts']`、`['dashboard']`、`['units']` 缓存。
- 失败：**回滚**到点击前状态。带回滚保护——若请求在途中用户又点了另一个等级，旧请求失败不会覆盖新状态；页面顶部红色横幅提示，**再点一次即重试**。
- 请求进行中详情面板状态旁有一个脉冲小圆点（`.sync-dot`）提示"保存中"。

## 2. 闪卡选范围页：卡片数本地计算

**后端**：`GET /api/units` 现在（`backend/handlers_concepts.go`）为每个 topic 附带按当前用户统计的计数：

```json
{
  "id": "topic-…",
  "title": "Set A",
  "counts": { "total": 8, "proficient": 1, "fuzzy": 1, "unknown": 0 }
}
```

`counts` 为 `omitempty`——未登录场景/无概念时缺省。

**前端**：`frontend/src/pages/Flashcards.tsx` 用 `useMemo` 对选中 topics（或全部）按 filter 求和：

- `all` → sum(total)；`unmarked` → total − 三项之和；`review` → fuzzy + unknown；`proficient` → proficient。

**不再**随勾选变化请求 `/api/review/count`（该接口保留未删，供其它用途）。计数刷新时机：Glossary 标记成功、复习队列批量提交成功后 invalidate `['units']`。

## 3. 闪卡做题：立即翻页 + 后台队列

**文件**：`frontend/src/lib/reviewQueue.ts`（队列）、`frontend/src/pages/Flashcards.tsx`（接入）

- 点 1/2/3（或按钮）后**立即**累加 tally、翻下一张，事件进入本地队列。
- 队列 400ms 防抖合并后 `POST /api/review/events/batch`；失败自动重试，间隔 2s × 重试次数、上限 30s。
- `pagehide` 时用 `fetch(..., { keepalive: true })` 把剩余事件一次性兜底送出（带 Authorization header，sendBeacon 做不到这一点所以没用）。
- 页头显示 `.sync-badge`（"N 条待同步 / N unsynced"，i18n key `syncPending`），清零即已同步。
- 批量成功后 invalidate `['dashboard']`、`['units']`。

**一致性取舍**：复习事件是 append-only 的自评记录，兜底仍失败时丢弃该批（用户可重新标记，分析统计可容忍缺口）。同一批内重复 conceptId 只记最后一条（后端去重）。

## 4. 后端 API 变更汇总

| 接口 | 变更 | 说明 |
|---|---|---|
| `GET /api/units` | 扩展 | topic 附 `counts`（见上）；需登录 |
| `POST /api/review/events/batch` | 新增 | body `{"events":[{conceptId,response,durationMs}]}`，≤200 条；响应 `{"recorded":N}`；400 参数错误 / 404 concept 不存在 / 500 事务失败；单条接口 `POST /api/review/events` 保留 |

前端配套：`queryClient` 移到 `frontend/src/lib/queryClient.ts`（供非 React 模块 invalidate）；`API_BASE` 从 `lib/api.ts` 导出。

## 5. 侧边栏修复

- **折叠态 icon 居中**：`.nav-collapsed nav a { justify-content: center; padding-inline: 8px; }`（此前 flex 左对齐 + 左内边距导致偏移）。
- **头像弹出层被裁剪**：`.user-menu` 从 sidebar 内 absolute 改为 **fixed** 定位，坐标在打开时由 chip 的 `getBoundingClientRect()` 计算（`left`、`bottom = viewport 高 − chip 顶 + 8px`），存入 state 作为 inline style。侧边栏的 `overflow` 不再影响弹出层；外点关闭逻辑不变（仍在 dockRef 内判断）。

## 6. 测试种子数据（本地开发库）

- **种子账号**：`dev.seed@tsinglan.org` / `seed-test-2026`，已加入 `.env` 的 `ADMIN_EMAILS`（注册即 admin）。仅存在于本地 `data/app.db`，`.env` 不入库。
- **题库文件**：`data/sample-questions.json`（12 MCQ + 2 主观题：1 道 FRQ 研究设计 3 小问、1 道 AAQ 带阅读材料 3 小问；unit/topic/tags 齐全，可提交入库）。
- **已建两套卷**（published）：
  - `Mixed Review A — Instant Feedback`：instant，8 MCQ，逐题反馈。
  - `Mock Exam B — Timed (20 min)`：exam，1200s，4 MCQ + FRQ + AAQ。

**重新灌入的命令**（在仓库根目录）：

```bash
TOKEN=$(curl -s -X POST localhost:8080/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"dev.seed@tsinglan.org","password":"seed-test-2026"}' \
  | python -c "import sys,json;print(json.load(sys.stdin)['token'])")

# 两步导入（preview → commit）
IMPORT_ID=$(curl -s -X POST localhost:8080/api/admin/questions/import/preview \
  -H "Authorization: Bearer $TOKEN" -F "file=@data/sample-questions.json;type=application/json" \
  | python -c "import sys,json;print(json.load(sys.stdin)['importId'])")
curl -s -X POST localhost:8080/api/admin/questions/import/commit \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d "{\"importId\":\"$IMPORT_ID\"}"

# 题目发布 + 建卷示例
curl -s -X PATCH localhost:8080/api/admin/questions/<qid> \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"status":"published"}'
curl -s -X POST localhost:8080/api/admin/sets \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"title":"…","mode":"exam","timeLimitSec":1200,"status":"published","questionIds":[…]}'
```

## 7. 端到端验证记录（2026-09-19，API 实测）

- `GET /api/units`：6 units / 41 topics 全部带 counts；批量提交 2 条事件后对应 topic 计数 `proficient 0→1, fuzzy 0→1` ✓
- `POST /api/review/events/batch`：合法批 `{"recorded":2}`；非法 response → 400 ✓
- 即时卷：学生载荷**不含** `answerKey`；答对 `isCorrect:true` 且答后回显 key；答错回显 explanation；finish 评分 1/8（2 答 1 对）✓
- 考试卷：deadline 正确设置；考试模式答题**不回显**对错与 key；主观题交前**不泄露** referenceAnswer；自评 `partial` 落库；finish 后 MCQ 判分 1/4、参考答案回显 ✓
- Wrong book 由最新答案派生出 wrong 条目 ✓；练习列表显示 attempts/bestScore ✓

## 8. 已知边界

- `pagehide` keepalive 仍失败时该批事件丢失（可重新标记，见第 3 节取舍）。
- units 计数在批量 flush 成功后才刷新：最长延迟 ≈ 重试窗口（≤30s）+ `staleTime`（30s）。
- Glossary 乐观标记与闪卡队列并发改同一概念时后写覆盖——两者都是用户主观自评，可接受。
