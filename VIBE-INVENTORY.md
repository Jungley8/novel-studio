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

## 全局 God File 清理（已完成）

| # | 模块路径 | 拆分结果 | 测试? | 优先级 | 状态 |
|---|---------|---------|-------|--------|------|
| 5 | `internal/store/sqlite_store.go` | 4946 → 14 文件（核心 72 / migrate 451 / chapter 901 / codex 783 / forecast 596 / scene 416 / automation 387 / activity 278 / studio 238 / checkpoint 214 / project 212 / memory 201 / intent 179 / hook 177）| ✅ | P1 | 🟢 已完成（agy-novel16-storesplit，声明集零差异 + 全量 race）|
| 6 | `web/src/views/WorkbenchView.vue` | 1656 → 260 + `components/workbench/` 2 组件（StepsPanel 823 / CanvasPanel 672）| ❌ | P1 | 🟢 已完成（agy-novel16-wbview，模板逐字节 + 引用审计零悬空）|
| 7 | `internal/engine/orchestrator.go` | 1749 → 7 文件（核心 30 / matrix 459 / beats 371 / render 333 / codex 256 / review 187 / framework 172）| ✅ | P2 | 🟢 已完成（agy-novel16-orchsplit，声明集零差异 + 全量 race）|
| 8 | `web/src/views/GraphView.vue` | 1073 → 795 + `components/graph/` 2 组件（DetailDrawer 231 / CreateRelationModal 159）| ❌ | P2 | 🟢 已完成（agy-novel16-graphview，模板逐字节 + 桥接行清单）|
| 9 | `internal/server/handlers_workshop.go` | 1095 → 5 文件（核心 572 / prompt 233 / assist 118 / chat 116 / inline 92）| ✅ | P2 | 🟢 已完成（agy-novel16-wshsplit，声明集零差异 + 全量 race）|

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
| 7 | 全局默认批产参数（words/style/loops/auto-lock 三档优先级解析）| `config.go` + `SettingsView.vue` + CLI/API | P2 | 🟢 已完成（agy-novel14-settings，双变异验证）|
| 8 | 停线待办中心（停线 checkpoint 落盘 + `/api/studio/todos` + 视图）| `workshop.go` + `handlers_studio_todos.go` + `StudioTodoView.vue` | P3 | 🟢 已完成（agy-novel14-todos/console，双变异验证）|
| 9 | Web pipeline 控制台（表单→SSE 进度时间轴→halt 展示→跳工作台）| `PipelineView.vue` + 裸 `data:` SSE 消费者 | P3 | 🟢 已完成（agy-novel14-console）|
| 10 | **排程/无人值守**（应用内调度器：作品续产+新书队列、每作品自定义时刻/配额、停线暂停待人工、macOS 系统通知、catch-up 补跑、串行执行）| `internal/scheduler` + `scheduler_runtime.go` + `internal/notify` + automation/queue API + `StudioAutomationView.vue` | P3 | 🟢 已完成（agy-novel15-schedcore/schedweb/scheduler，变异+自跑变异验证；冒烟通过）|

---

## 跨作品串稿专项（Cross-Project State Isolation，2026-10-12）

症状：工作台切换作品后正文/审校/断点仍停留在上一个作品（用户报告"切换作品正文没有重置"）。
根因：`selectProject` 只重载了列表数据，未重置 ★作品级瞬态状态★；配合 `restoreCheckpoint` 的内容守卫（旧正文非空 → 直接 return）与延迟触发的防抖自动保存，形成完整的数据串稿链路（旧作品正文可被写进新作品的断点）。

| # | 交付 | 模块 | 优先级 | 状态 |
|---|------|------|--------|------|
| 1 | **切换作品整体重置**（单一事实源 `projectScope.js`：workbench/审校/断点/停线/弹窗基质一次清空）| `stores/domain/projectScope.js` + `projectActions.selectProject` | P0 | 🟢 已完成 |
| 2 | **Fail-Closed 切换门禁**（全自动成章/起草/体检/精修/定稿等任务进行中拒绝切换，含下拉框回退与所有跳转入口）| `projectActions` + `AppSidebar` + `PipelineView` + `StudioTodoView` | P0 | 🟢 已完成 |
| 3 | **切换前落盘 + 断点守卫修正**（旧作品在途草稿先写回旧作品；恢复断点显式指定新作品下一章，禁用旧 `editingChapterIndex`；失败回退半切换）| `projectActions` + `workbenchActions` | P0 | 🟢 已完成 |
| 4 | **防抖自动保存防串稿**（切换时取消在途 debounce 与划词改写缓冲）| `WorkbenchCanvasPanel` | P0 | 🟢 已完成 |
| 5 | **视图/弹窗本地态重置**（章节选中/搜索、意图编辑表单、百科扫描、矩阵拆场、图谱搜索与关系弹窗、指标与体检报告）| 10+ 视图与弹窗组件 | P1 | 🟢 已完成 |
| 6 | **通知跳转作品归属**（消息沉淀时记录 `projectId`，跨作品跳转前先切换，拒绝则不跳转）| `appState.notify` + `MessageCenterModal` | P1 | 🟢 已完成 |
| 7 | **后端词条越权防护**（用其它作品的词条 ID 更新 → 409 拒绝，原数据不被覆盖）| `handlers_matrix.go` + `store` | P1 | 🟢 已完成 |
| 8 | **域操作 this 绑定系统修复**（裸引用调用 `actions.foo` 不再因 `this` undefined 崩溃；一键成章/去机械壳等修复）| `appState.bindActions` | P1 | 🟢 已完成 |
| 9 | **`actionHelpers` ReferenceError + 矩阵拆场 payload 修复**（框架推演后 Codex 刷新失效；AI 拆场 400）| `projectActions` + `MatrixView` | P1 | 🟢 已完成 |
| 10 | **回归测试**（Node 原生测试 + resolve loader：切换重置/落盘/Fail-Closed 门禁/断点目标）| `web/test/project-switch.test.mjs`（`pnpm test`）| P0 | 🟢 已完成 |

验证：`pnpm test`（3/3 通过）；`pnpm build` 通过；`go build ./... && go test ./... -race -count=1` 全绿（含新增 `TestCodexUpdate_RejectsForeignProjectEntry`）；`gofmt -w` 完成。

---

## 精炼成果与总进度

- 🟢 **已精炼完成**: 5/5 核心双轨制与割裂架构缺陷（主角、势力阵营、世界公理、关键人物影子、种子伏笔冗余）
- 🟢 **AI-First 改造**: 10/10 交付完成（编排器引擎/CLI/API、框架级级联、对话持久化全栈、成本面板、默认批产参数、停线待办中心、pipeline 控制台、排程/无人值守）
- 🟢 **God File 拆分**: 5/5 完成（sqlite_store 14 文件 / WorkbenchView 3 文件 / orchestrator 7 文件 / GraphView 3 文件 / handlers_workshop 5 文件；纯搬迁，声明集/模板零差异验证）
- 🟡 **进行中**: 0
- 🔴 **待排期**: 无（全部交付完成）
- **Go 测试**: `go test ./... -race -count=1` 全绿通过
- **前端构建**: `pnpm --dir web build` 纯净构建通过
