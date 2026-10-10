# Vibe Refine — Module Inventory

项目：novel-studio
扫描日期：2026-10-10
总行数：29,857 行 (Go 后端 19,474 行, Vue 前端 10,279 行)
测试覆盖：引擎/存储 ~78%, 服务层 ~44%, API ~35%

## 架构级清理专项：消灭双轨制与数据割裂 (Dual-Track Elimination)

基于 Pre-v1.0 架构准则（严禁保留 fallback、软降级或双轨制兼容垫片，严格 Fail-Closed），全面消灭与“主角问题”同类的双轨存储与图谱数据脱节缺陷。

| # | 专项模块 | 涉及文件 | 解决的典型症状 | 优先级 | 状态 |
|---|---------|---------|--------------|--------|------|
| 0 | **主角档案 (Protagonist)** 收编至 Codex | `internal/domain/models.go`<br>`internal/store/sqlite_store.go`<br>`internal/engine/chronicle.go`<br>`web/src/stores/appState.js` | 🔴 消除 `projects.protagonist_json` 双轨，主角原生作为 `CodexEntry(PROTAGONIST)` 注入知识图谱与状态机 | **P0** | 🟢 已完成 |
| 1 | **势力阵营 (Factions)** 双轨清理与收编 | `internal/domain/codex.go`<br>`internal/store/sqlite_store.go`<br>`web/src/views/FrameworkView.vue`<br>`web/src/stores/domain/codexActions.js` | 🔴 彻底移除 `framework.factions` 双轨，势力归拢为 `CodexEntry(FACTION)`，作品设定界面直接读写图谱数据，移除虚假“手动同步税” | **P0** | 🟢 已完成 |
| 2 | **世界公理 (WorldRules)** 读写联动统一 | `internal/server/handlers_matrix.go`<br>`web/src/stores/domain/projectActions.js` | 🔴 消除 `WorldRules` 与 `world_axioms` 失步缺陷，前端修改公理原子化同步 `world_rules`，确保 AI Prompt 读取最新世界规则 | **P0** | 🟢 已完成 |
| 3 | **关键人物 (KeyCharacters)** 影子剥离 | `internal/server/handlers_project.go`<br>`internal/server/handlers_matrix.go`<br>`cmd/novel-studio/genesis.go` | 🟡 创世瞬态种子入库后立即从 `framework_json` 剥离置空，角色演进唯一由 `codex_entries` 掌管 | **P1** | 🟢 已完成 |
| 4 | **种子伏笔 (SeedHooks)** 冗余剥离 | `internal/server/handlers_project.go`<br>`internal/server/handlers_matrix.go` | 🟡 种子伏笔入库后立即从 `framework_json` 剥离置空，全生命周期唯一由 `plot_hooks` 表管理 | **P1** | 🟢 已完成 |

---

## 全局 God File 清理排期（后续优化）

| # | 模块路径 | 行数 | 测试? | 热度 | 症状数 | 优先级 | 状态 |
|---|---------|------|-------|------|--------|--------|------|
| 5 | `internal/store/sqlite_store.go` | 2890 | ✅ 78% | 🔥🔥🔥 | 4 (God file, 庞大单体) | P1 | 待开始 |
| 6 | `web/src/views/WorkbenchView.vue` | 1582 | ❌ | 🔥🔥🔥 | 5 (巨型视图组件) | P1 | 待开始 |
| 7 | `internal/engine/orchestrator.go` | 1456 | ✅ 78% | 🔥🔥 | 3 (God file, Prompt 拼接) | P2 | 待开始 |
| 8 | `web/src/views/GraphView.vue` | 1073 | ❌ | 🔥🔥 | 3 (大单体视图) | P2 | 待开始 |
| 9 | `internal/server/handlers_workshop.go` | 1020 | ✅ 35% | 🔥🔥 | 3 (庞大 Handler) | P2 | 待开始 |

---

## 精炼成果与总进度

- 🟢 **已精炼完成**: 5/5 核心双轨制与割裂架构缺陷（主角、势力阵营、世界公理、关键人物影子、种子伏笔冗余）
- 🟡 **进行中**: 0
- 🔴 **待排期**: 5 模块（后续 God File 拆分）
- **Go 测试**: `go test ./... -race -count=1` 全绿通过
- **前端构建**: `pnpm --dir web build` 2.15s 纯净构建通过
