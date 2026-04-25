# MVP P0 补齐 — 总路线图

> **对标 spec**：`docs/superpowers/specs/2026-04-24-trademark-quote-platform-mvp-design.md`
> **针对**：2026-04-25 验收审查中标为 ❌/⚠️ 的 P0 缺口
> **日期**：2026-04-25

## 背景：历史 plan 情况

仓库 `docs/superpowers/plans/` 里已存在 11 个先前 plan（`2026-04-24-01` 至 `2026-04-25-11`），覆盖了 monorepo 骨架、auth、catalog、customer、pricing、quotation、export（仅 docx + 浏览器打印）、dashboard 等子系统——从代码现状看**均已执行完成**。本路线图是对**那些 plan 完成之后**的 MVP 状态做的验收差距补齐，**不**重新做已完成的工作。

最关键的技术决策变更：`2026-04-25-10-export.md` 显式把"服务端 PDF"标为 out-of-scope（怕嵌入 CJK 字体），本路线图 M1 用 gotenberg 旁路服务重新开启这个能力——字体由 gotenberg 的 chromium 负责，api 镜像保持纯净。

---

## 与 spec 的差异说明

实施过程中有几处与原 spec 的偏差，先对齐再动工：

| spec 原文 | 现实/决策 | 理由 |
|---|---|---|
| PDF 用 `chromedp` headless Chrome | **改用 gotenberg** HTTP 服务 | 用户明确指定轻方案；chromedp 要求镜像里集成 Chromium（镜像~1GB），gotenberg 作为旁路容器让 api 镜像保持纯净 |
| `quotation_items` 独立表 + `source_pricing_entry_id` 溯源 | **保留现有 JSONB snapshot**，在 `SnapshotLine` 里增加 `source_pricing_entry_id` | 现实现已能冻结金额（non-draft 必含 snapshot+total+signature 的 CHECK 约束），再建独立表收益边际；溯源在 JSONB 里足够 |
| 状态机 `submitted → reviewing → approved` | **保持现有 `submitted → approved/rejected` 二态** | 当前代码已部署，引入 `reviewing` 中间态要改迁移/service/handler/前端，工作量不匹配收益 |
| `quotation_reviews` 独立表带 `diff_json` | **扩展 `quotation_status_history` 加 `diff_json` 列** | 一张表够表达"审核流水"，减少 JOIN |
| 报价向导前端 5 步 + `:preview` 不入库路由 | **范围内，单独 milestone M3** | 最大工作量，拆为独立 plan |
| 序列号 `Q + YYYYMMDD + 4 位` | **范围内，并入 M2** | advisory lock + 每日 max+1，放 service 层 |

## 里程碑总览

```
M1 (导出 PDF)  ──┐
                 ├─→ M5 (E2E)
M2 (调价流) ─────┤
                 │
M3 (向导前端) ───┘
                  
M4 (溯源增强)  → 独立，不阻塞其他
```

| 里程碑 | 范围 | 预计任务数 | 依赖 | 独立交付价值 |
|---|---|---|---|---|
| **M1** | 导出 PDF（gotenberg）+ export_files 表 + 24h 下载链接 + 双语 HTML 模板 | 12 | 无 | 补齐验收 #6；reviewer/业务员可下载 PDF |
| **M2** | Adjust（调价）+ Withdraw + Copy + Serial No + `quotation_status_history.diff_json` | 14 | 无 | 补齐验收 #5；完整 reviewer 工作流 |
| **M3** | 报价向导 5 步前端 + `:preview` API + export dialog 接入 PDF | 16 | M1、M2 | 补齐验收 #4；业务员端到端可用 |
| **M4** | `SnapshotLine.SourcePricingEntryID` 溯源 + 历史价回查 | 6 | 无 | 强化验收 #7；审计可见每条明细出自哪个 pricing entry |
| **M5** | Playwright E2E：登录→客户→向导→提交→审核→调价→通过→下载 PDF | 8 | M1+M2+M3 | 补齐验收 #9；CI 可持续验证完整链路 |

**建议执行顺序**：M1 → M2 → M4 → M3 → M5（M1/M2/M4 并行也可）

## 不在本路线图内

- **`:claim` 和 `reviewing` 中间态**：见「差异说明」；如确要保留 spec 原样，另开 plan。
- **导出文件清理 cron**：MVP 不跑，spec §8.4 允许；M1 只做过期标记。
- **OpenAPI + openapi-typescript 生成**：用户选"手写 types"，不在本轮。
- **钟表清理、通知、多租户**：spec §17 已列为范围外。

## 详细 plan 文件

- M1：`docs/superpowers/plans/2026-04-25-m1-export-pdf.md`  ← **本次先写**
- M2：待用户确认再写
- M3：待用户确认再写
- M4：待用户确认再写
- M5：待用户确认再写

---

## 验收映射（plan vs spec §DoD）

| DoD # | 当前 | M1 后 | M2 后 | M3 后 | M4 后 | M5 后 |
|---|:---:|:---:|:---:|:---:|:---:|:---:|
| 1 登录 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| 2 字典 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| 3 版本化 pricing | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| 4 向导 5 步 | ❌ | ❌ | ❌ | ✅ | ✅ | ✅ |
| 5 审核流 | ⚠️ | ⚠️ | ✅ | ✅ | ✅ | ✅ |
| 6 PDF+Word | ⚠️ | ✅ | ✅ | ✅ | ✅ | ✅ |
| 7 快照 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| 8 权限 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| 9 e2e | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ |
| 10 docker | ✅ | ✅ (+gotenberg) | ✅ | ✅ | ✅ | ✅ |

全部 ✅ 条件：M1+M2+M3+M5 完成（M4 可选）。
