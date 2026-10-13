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
| 11 | **租户归属校验全量补齐**（伏笔/词条演进/关系/场次的跨作品 ID 借道写入一律拒绝；场次挂载章节校验归属）| `handlers_project.go` + `handlers_matrix.go` + `store`（`handlers_tenancy_scope_test.go`）| P1 | 🟢 已完成 |
| 12 | **切换恢复“最近推进的一章”**（`checkpoint?latest=true`：兼顾下一章草稿与已归档章节重修稿，不再只看下一章）| `store` + `handlers_project.go` + `api/client.js` + `workbenchActions` | P1 | 🟢 已完成 |
| 13 | **前台编辑 ↔ 后台生产并发互斥**（`internal/coord`：心跳编辑租约 + 生产锁；调度器遇忙跳过下趟重试；定稿/撤回/删除/一键成章均 409 Fail-Closed）| `internal/coord` + `scheduler` + `scheduler_runtime.go` + `handlers_*` + `appState` 心跳 | P0 | 🟢 已完成 |
| 14 | **并发互斥回归测试**（coord 单测 5 例；调度器跳过且不记 last_run；白盒接线：4 个正史写入口 409 + 租约端点往返 + 生产结束后恢复写入）| `coord_test.go` + `scheduler_test.go` + `coord_wiring_test.go` | P0 | 🟢 已完成 |
| 15 | **跨进程协调升级**（内存租约 → SQLite `coordination_leases` 单一事实源：edit/production 双租约 + holder token + TTL 心跳；CLI `produce` 接入排斥门禁；调度器/SSE/CLI 三方互斥）| `domain/coordination.go` + `store` 租约方法 + `cmd/novel-studio/produce.go` | P0 | 🟢 已完成 |
| 16 | **跨进程回归测试**（双 store 实例模拟守护/CLI：租约可见性、双向排斥、holder 隔离、过期接管与清理；CLI 忙碌路径退出码 1 + 忙碌文案 + 无残留锁）| `sqlite_store_locks_test.go` + `cmd/.../produce_test.go` | P0 | 🟢 已完成 |
| 17 | **开书窗口闭环**（`OnProjectCreated` 创世回调 + `coord.Production`/`BindProduction` 公共持有器；RunBook/UI 开书/CLI pipeline/CLI genesis 全部“创世即登记生产锁”）| `pipeline.go` + `coord/production.go` + 4 个开书入口 | P0 | 🟢 已完成 |

验证：`pnpm test`（3/3 通过）；`pnpm build` 通过；`go build ./... && go test ./... -race -count=1` 全绿（含新增 `TestCodexUpdate_RejectsForeignProjectEntry`）；`gofmt -w` 完成。

---

## 作品删除与各模块 UX 提升专项（Wave 4 / Round 23-24）

目标：支持删除作品（彻底清理附随数据、Fail-Closed 并发守护、前端全局重置与切换、二次确认弹窗），全面提升各核心模块（StudioLibrary、Workbench、Framework、StudioTodo、Pipeline、AppSidebar）UX 交互体验。

| # | 交付 | 模块 / Spec | 优先级 | 状态 |
|---|------|------------|--------|------|
| 1 | **后端作品删除级联清理**（显式清理无 FK 的 10 个附随表：metrics/versions/snapshots/compaction/automation/memory_fts/scenes/book_queue/threads/messages，账本墓碑留存） | `internal/store/sqlite_store_project.go` + `agy-novel23-delproj-backend.md` | **P0** | 🟢 已完成 |
| 2 | **作品删除并发锁守卫**（`DELETE /api/projects/:id` 接入 `rejectIfProducing`，后台生产/开书中 409 Fail-Closed） | `internal/server/handlers_project.go` + `coord_wiring_test.go` | **P0** | 🟢 已完成 |
| 3 | **前端删除作品动作与状态闭环**（`deleteProject`：activeTask 忙碌检查 Fail-Closed + 危险确认弹窗 + 移除本地作品 + 激活作品删除时重置瞬态平滑切换至其它作品/置空） | `web/src/stores/domain/projectActions.js` + `agy-novel23-delproj-web.md` | **P0** | 🟢 已完成 |
| 4 | **前端删除作品回归测试**（生成忙碌拒绝 409/本地提示、取消确认、删除成功切换下一个作品、删除最后一部作品状态清空） | `web/test/project-switch.test.mjs` | **P0** | 🟢 已完成 |
| 5 | **多作品总览交互与删除入口**（操作列删除按钮 + 危险二次确认 + 库数据刷新 + 标题/平台搜索筛选） | `web/src/views/StudioLibraryView.vue` + `agy-novel24-ux-studio.md` | **P1** | 🟢 已完成 |
| 6 | **作品设定危险区域（Danger Zone）**（作品总览底端红色高危卡片：明确永久删除提示与删除按钮） | `web/src/views/FrameworkView.vue` | **P1** | 🟢 已完成 |
| 7 | **侧边栏作品快捷管理 UX**（作品选择器平滑切换、删除/设置入口联动、空作品库友好提示） | `web/src/components/layout/AppSidebar.vue` | **P1** | 🟢 已完成 |
| 8 | **工作台画布与质检 UX 提升**（自动保存状态胶囊、章节字数达标进度条、分庭判决与少数派 dissent 视觉升级） | `web/src/components/workbench/` + `agy-novel24-ux-workbench.md` | **P2** | 🟢 已完成 |
| 9 | **待办中心与流水线控制台 UX 提升**（待办中心清空友好插画/快速直达工作台；流水线耗时计时器与自动滚动切换） | `StudioTodoView.vue` + `PipelineView.vue` | **P2** | 🟢 已完成 |

---

## 全局菜单与作品级菜单完全物理隔离专项（Wave 5 / Round 25）

目标：将 Studio 全局菜单（总览、灵感、对话、流水线、待办、排程、配置）与单作品创作生命周期菜单（构思 4 模块、写作 4 模块、成果 5 模块）彻底物理隔离，顶部 Header 与侧边栏 AppSidebar 完全区分，提供清晰醒目的模式切换与返回机制。

| # | 交付 | 模块 / Spec | 优先级 | 状态 |
|---|------|------------|--------|------|
| 1 | **作用域单一事实源与元数据字典**（`GLOBAL_TABS` 7 模块 + `PROJECT_TABS` 13 模块严格互斥无交集、`TAB_META` 全元数据字典） | `web/src/stores/domain/projectScope.js` | **P0** | 🟢 已完成 |
| 2 | **全局与作品双域状态记忆与切换动作**（`lastGlobalTab`/`lastProjectTab` 独立断点追踪，`switchToGlobal`/`switchToProject` 平滑互通与 Fail-Closed 保护） | `web/src/stores/appState.js` + `projectActions.js` | **P0** | 🟢 已完成 |
| 3 | **侧边栏物理完全隔离**（全局模式：仅 Studio + 配置，带活跃作品通道直达卡片；作品模式：置顶返回 Studio 胶囊按钮，仅三部曲 13 模块，无任何混杂） | `web/src/components/layout/AppSidebar.vue` | **P0** | 🟢 已完成 |
| 4 | **顶部 Header 页面状态完全区分**（全局模式：Studio 空间微章 + 页面功能描述 + 直达当前书创作胶囊；作品模式：作品名 + 平台 + 当前工序 + 完整正文工具链） | `web/src/components/layout/AppHeader.vue` | **P0** | 🟢 已完成 |
| 5 | **主视图容器与作品级零状态兜底**（作品创作空间无有效作品时，优雅引导卡片返回总览或新建作品，杜绝白屏） | `web/src/App.vue` + `WorkbenchView.vue` + `StudioLibraryView.vue` | **P1** | 🟢 已完成 |
| 6 | **作用域物理隔离专项回归测试**（互斥与全覆盖断言、响应式计算属性验证、双域记忆断点恢复、空作品库保护） | `web/test/nav-scope-isolation.test.mjs` | **P0** | 🟢 已完成 |

---

## 对话记录与灵感 Inbox 合二为一、文本自由复制与 Markdown 渲染优化专项（Wave 6 / Round 26）

目标：将分散的 Inbox 与 灵感 彻底合二为一为「对话记录」，彻底消除根节点 `select-none` 释放文本复制自由，引入 `marked` 引擎进行深度 Markdown 优雅排版与一键复制。

| # | 交付 | 模块 / Spec | 优先级 | 状态 |
|---|------|------------|--------|------|
| 1 | **Inbox 与 灵感 合二为一**（全局模块精炼为 6 大中枢，`studio-chat` 升级为「对话记录」双流统一工作区） | `web/src/stores/domain/projectScope.js` + `AppSidebar.vue` | **P0** | 🟢 已完成 |
| 2 | **对话记录双流交互体验**（左栏支持【💬 会话记录】与【💡 灵感 Inbox】双流切换、消息一键收录为灵感/一键转创世） | `web/src/views/StudioChatView.vue` | **P0** | 🟢 已完成 |
| 3 | **页面文本划选复制彻底解禁**（移除 `App.vue` 根节点 `select-none`，主工作区开启 `select-text`，解决无法复制顽疾） | `web/src/App.vue` + `web/src/assets/main.css` | **P0** | 🟢 已完成 |
| 4 | **Marked 引擎与 Markdown 渲染排版**（引入 `marked`，封装 `renderMarkdown` 与 `copyToClipboard`，注入 `.markdown-body` 暗黑美学样式） | `web/src/utils/markdown.js` + `main.css` | **P0** | 🟢 已完成 |
| 5 | **情境工坊对话同步升级**（单作品角色对戏抽屉接入 Markdown 渲染与一键复制，支持文本自由划选） | `web/src/components/modals/WorkshopChatDrawer.vue` | **P1** | 🟢 已完成 |
| 6 | **专项回归测试与渲染断言**（作用域 6 大核心模块全覆盖、Markdown 标题/列表/代码块/引用块解析断言） | `web/test/nav-scope-isolation.test.mjs` | **P0** | 🟢 已完成 |

---

## 创世推演异步化与即时跳转响应架构专项（Wave 7 / Round 27）

目标：消灭新建作品「创世推演」30~90 秒同步阻塞与前端 HTTP 超时挂起，实现秒级响应 `201 Created`、平滑切入新书工作台、后台 goroutine 异步推演落地、生产协调锁护航与前端全自动状态感知刷新。

| # | 交付 | 模块 / Spec | 优先级 | 状态 |
|---|------|------------|--------|------|
| 1 | **即时作品持久化与 201 响应**（`POST /api/projects/bootstrap` 秒级落盘基础作品与主角占位，立即返回） | `internal/server/handlers_project.go` | **P0** | 🟢 已完成 |
| 2 | **后台异步创世推进与生产协调锁**（goroutine 异步执行总纲/阵营/人物/图谱/首章冲突推演，`coord.AcquireProduction` 生产锁保护） | `internal/server/handlers_project.go` | **P0** | 🟢 已完成 |
| 3 | **推演状态查询端点**（`GET /api/projects/:id/bootstrap-status` 暴露 running/completed/failed 与分步阶段） | `internal/server/handlers_project.go` + `server.go` | **P0** | 🟢 已完成 |
| 4 | **平滑跳转与异步监视器**（前端新建后立即 `switchToProject` 切入工作台，`startBootstrapMonitor` 轮询并在完成后自动全量刷新） | `web/src/stores/domain/projectActions.js` + `NewProjectModal.vue` | **P0** | 🟢 已完成 |
| 5 | **多场景创世状态感知 UX**（顶部 Header 任务指示条、作品设定页动态推演光效卡片、手稿画布快捷横幅） | `FrameworkView.vue` + `WorkbenchCanvasPanel.vue` + `appState.js` | **P1** | 🟢 已完成 |
| 6 | **端到端专项测试**（前端 `async-bootstrap.test.mjs` 覆盖状态机与刷新，后端 `server_test.go` 验证异步推演与状态端点） | `web/test/async-bootstrap.test.mjs` + `internal/server/server_test.go` | **P0** | 🟢 已完成 |

---

## 精炼成果与总进度

- 🟢 **已精炼完成**: 5/5 核心双轨制与割裂架构缺陷（主角、势力阵营、世界公理、关键人物影子、种子伏笔冗余）
- 🟢 **AI-First 改造**: 10/10 交付完成（编排器引擎/CLI/API、框架级级联、对话持久化全栈、成本面板、默认批产参数、停线待办中心、pipeline 控制台、排程/无人值守）
- 🟢 **God File 拆分**: 5/5 完成（sqlite_store 14 文件 / WorkbenchView 3 文件 / orchestrator 7 文件 / GraphView 3 文件 / handlers_workshop 5 文件）
- 🟢 **作品删除与各模块 UX 专项 (Wave 4)**: 9/9 交付完成（后端全级联擦除、409 生产互斥门禁、前端状态切换闭环、7/7 回归单测、全场景危险确认与各模块 UX 增强）
- 🟢 **全局与作品级菜单完全隔离 (Wave 5)**: 6/6 交付完成（双域单一事实源、Sidebar/Header 物理隔离、视图零状态兜底）
- 🟢 **对话记录/灵感合并、文本复制解禁与Markdown优化 (Wave 6)**: 6/6 交付完成（Inbox与灵感合一、select-none移除、marked解析排版、12/12 单测全绿）
- 🟢 **创世推演异步化与即时跳转 (Wave 7)**: 6/6 交付完成（即时响应、平滑跳入工作台、后台goroutine与生产锁、状态轮询自愈刷新、13/13 单测全绿）
- 🟡 **进行中**: 0
- 🔴 **待排期**: 无
- **Go 测试**: `go test ./... -race -count=1` 全库全绿通过
- **前端测试**: `pnpm test`（13/13 全部通过）
- **前端构建**: `pnpm --dir web build` 纯净构建通过




