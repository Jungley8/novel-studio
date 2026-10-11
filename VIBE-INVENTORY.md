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

## AI-First 自动化闭环改造方案落地（2026-10-11 起）

目标：世界观→角色→卷纲→章节意图自上而下流程化；AI first 自动化闭环；作品之上的管理维度。

| # | 交付 | 模块 | 优先级 | 状态 |
|---|------|------|--------|------|
| 1 | 编排器引擎（bootstrap→种子→意图→自动锁→逐卷批产→halt 语义）| `internal/engine/pipeline.go` + `seed.go` + `cost.go` | P0 | 🟢 已完成（agy-novel12-pipeline，双变异验证）|
| 2 | 编排器 CLI（`novel-studio pipeline`）| `cmd/novel-studio/pipeline.go` + main.go | P0 | 🟢 已完成（agy-novel13-cli，exit 0/1/2 契约 + 变异验证）|
| 3 | 编排器 API（`POST /api/pipeline` + SSE）| `internal/server/handlers_pipeline.go` | P0 | 🟢 已完成（agy-novel13-api，SSE/同步双模式 + 全链路 fake 测试 + 变异验证）|
| 4 | 框架级意图级联（公理/阶梯/立意变更 → 全卷 LOCKED 意图 STALE）| `internal/server/handlers_matrix.go` | P1 | 🟢 已完成（agy-novel12-cascade，变异验证）|
| 5 | Studio 对话持久化（线程/消息落库 + 线程栏 UI）| `internal/store/sqlite_store.go` + `handlers_studio.go` + `StudioChatView.vue` | P2 | 🟢 已完成（agy-novel12-chatpersist/chatweb，双变异验证）|
| 6 | 成本与用量面板（跨作品 token/费用聚合）| `handlers_studio.go` + `StudioLibraryView.vue` | P2 | 🟢 已完成（agy-novel13-cost，引擎公式单一源 + 变异验证）|
| 7 | 全局设置扩展（模型路由/提示词模板/默认批产参数）| `SettingsView.vue` + `handlers_config.go` | P2 | 🔴 待开始 |
| 8 | 排程/无人值守 + 停线待办中心 | launchd 接线 + 新视图 | P3 | 🔴 待开始 |
| 9 | Web pipeline 控制台（启动/暂停/进度/停线点）| `PipelineView.vue` + SSE 消费 | P3 | 🔴 待开始 |

---

## 精炼成果与总进度

- 🟢 **已精炼完成**: 5/5 核心双轨制与割裂架构缺陷（主角、势力阵营、世界公理、关键人物影子、种子伏笔冗余）
- 🟢 **AI-First 改造**: 6/9 交付完成（编排器引擎/CLI/API、框架级级联、对话持久化全栈、成本面板）
- 🟡 **进行中**: 0
- 🔴 **待排期**: 6 项改造交付 + 5 模块（后续 God File 拆分）
- **Go 测试**: `go test ./... -race -count=1` 全绿通过
- **前端构建**: `pnpm --dir web build` 纯净构建通过
