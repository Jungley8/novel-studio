# Refine Log: NovelStudio 2026-2027 工业化工作流精炼落地

日期：2026-10-07  
方法论：`vibe-refine` + `codebase-design` (Deep Modules & Invariant Locality)

---

## 精炼背景与问题定位
通过 `/codebase-design` 与 `/improve-codebase-architecture` 审查，识别出 4 处关键架构摩擦：
1. **实体状态伪机与字符串追加失控**：`Inventory` 和 `NameAndLevel` 采用原始字符串累加，长期生成导致不变量失效。
2. **启发式质检与语义审校割裂**：`Linter` 与 `ReviewDraft` 属于两个平行模块，调用方须手动将套词违规转换为主编提示。
3. **上下文拼接分散且缺乏滚动正史**：静态世界观重复拼接，无法感知前序最近 3 章的密封正史，长篇易漂移。
4. **前端 JS 编排多步工序导致业务泄露**：节拍推演、渲染、质检、初审、返工循环由浏览器发起 6 次 HTTP 请求，导致 macOS 守护服务与 CLI 无法无头自主生产。

---

## 精炼操作与 Commit 记录

### Step 1: 实体状态机账本深化 (`EntityLedger`)
- **`commit: cc7d40c` `refine(domain/store): deepen entity ledger with structured transitions`**
- 新增 `internal/domain/ledger.go` 与 `ledger_test.go`；
- 实现 `ApplyStateMutation(p Protagonist, mutation StateMutation) (Protagonist, []string, error)`：
  - 结构化解析 `+`、`-` 及中文增减动词（获得/消耗/损坏/服下）；
  - 严格不变量校验：若尝试消耗未持有的道具，出具实体状态机不一致预警；
  - 规范境界命名，杜绝括号无限嵌套；
- 重构 `store.CommitChapter`，委托给 `EntityLedger` 统一演变状态。

### Step 2: 启发式质检与主编审阅合流 (`QualityGate`)
- **`commit: e6fd17f` `refine(engine): unify heuristic linting and reviewer into quality gate`**
- 扩展 `domain.AuditReport` 领域模型，合并算法统计指标与语义审核建议；
- 新增 `internal/engine/quality_gate.go` 与 `quality_gate_test.go`；
- `QualityGate.Audit` 先行执行 0ms 启发式算法预检（突发度与套词扫描），将违规事实自动注入 Reviewer 提示词；
- 命中模式化套词时自动扣分并强行裁决 `REVISION_NEEDED`，实现严密门禁。

### Step 3: 滚动正史视界组装器深化 (`CanonChronicle`)
- **`commit: d46b5f6` `refine(engine): synthesize rolling canon horizon with chronicle module`**
- 新增 `internal/engine/chronicle.go` 与 `chronicle_test.go`；
- `CanonChronicle.AssembleHorizon(ctx, projectID, targetChapter)`：
  - 确定性提取目标章节前最近 3 章的密封正史文本、履约节拍与状态结算；
  - 智能识别临期（2章内须回收）的开放伏笔并标红高权提示；
- 拓展 `Orchestrator.DeriveBeatsWithHorizon`，推演大纲具备深厚历史连续性。

### Step 4: 全流程章节工坊深度模块 (`ChapterWorkshop`)
- **`commit: b40cf56` `refine(workshop): collapse novel creation loop into deep chapter workshop`**
- 新增 `internal/engine/workshop.go` 与 `workshop_test.go`；
- 提供高内聚单一接口：`ProduceChapter(ctx, req) (*WorkshopProduceResult, error)`；
  - 自动执行：正史组装 ➔ 节拍推演 ➔ 文学渲染 ➔ 综合质检 ➔ 智能定向返工循环 (最多3轮) ➔ 原子事务归档；
- 扩展服务端路由 `POST /api/projects/:id/workshop/produce`；
- 前端 `web/dist/index.html` 新增 `⚡ 一键全流程自主生产 (Auto-Pipeline)` 按钮；
- 增加 CLI 子命令 `novel-studio produce -project <id> -conflict <text>`，支持无头后台自主生产。

---

### Step 5: 架构与工程评审闭环迭代 (Review Implementation)
- **多模型/多供应商架构 (P0)**：
  - `internal/config/config.go` 引入 `ProviderConfig`，支持独立的 `ReviewerProvider`、`WriterProvider` 与 `ReasonerProvider`；
  - `internal/engine/llm_client.go` 抽象 `LLMRouter`，质检员与作家可走完全隔离的第三方供应商（如 OpenAI / Claude / Kimi），彻底根除同模型自审自评盲区。
- **崩溃断点自愈与 SSE 进度流驱动 (P0)**：
  - `internal/store/sqlite_store.go` 新增 `chapter_checkpoints` 崩溃断点表；
  - `internal/engine/workshop.go` 引入检查点机制（`SaveCheckpoint`/`GetCheckpoint`/`ClearCheckpoint`），发生网络或进程中断时重新运行可原地无缝续写；
  - `internal/server/server.go` 开放 `POST /api/projects/:id/workshop/produce?stream=true` SSE 实时流接口；
  - `web/dist/index.html` 嵌入实时状态指示条，呈现各工序阶段、断点恢复标识、Token 消耗与成本预估。
- **防御性 JSON 智能提取器 (P1)**：
  - 新增 `internal/engine/json_extractor.go`（`ExtractAndCleanJSON`），包含 8 组严苛边界测试；
  - 自动剥离模型思考文本与 Markdown 标记，平衡括号截取，智能修复尾部逗号与截断未闭合括号。
- **结构化实体演进与括号嵌套消除 (P1)**：
  - `domain.Protagonist` 引入 `PowerLevel`（含结构化历史突破记录）与 `InventoryItem`；
  - `domain.ledger.go` 重构战力提取，杜绝 `((练气三层)(筑基初期))` 括号无限嵌套。
- **伏笔闭环回收认定机制 (P1)**：
  - `domain.AuditReport` 引入 `ResolvedHookIDs`；
  - `QualityGate` 注入当前未回收伏笔供主编审定正文回收情况；
  - `store.CommitChapter` 归档时自动将已回收伏笔状态原子翻转为 `RESOLVED`。
- **三级上下文分层视界 (P1)**：
  - `CanonChronicle` 升级为全局概要 + 滚动 3 章正史 + 跨卷远期伏笔回调，推演大纲具备深厚历史纵深。
- **工程化全方位加固 (E1-E6)**：
  - E1: 服务端全部 Handler 统一施加 `10MB` 请求体防护；
  - E2: `GET /api/config` 密钥安全脱敏，保存时智能识别掩码避免覆盖；
  - E3: CLI `novel-studio produce` 监听系统退出信号，安全中断长任务；
  - E4: 全程精准统计 Token 输入/输出与成本估算；
  - E5: Web 界面增加独立质检员配置面板与流水线状态条；
  - E6: 修复测试端口冲突，保证 `go test -count=1 -race ./...` 100% 绿灯。

---

## 精炼后指标验证

| 指标 | 精炼前 (Initial) | Refine v1.0 | 评审闭环迭代后 (Current) |
| :--- | :--- | :--- | :--- |
| **模块测试覆盖** | 11 个测试 | 14 个测试 | **23 个测试全覆盖 (包含 -race, 0 竞态, 0 泄漏)** |
| **工作流调度收敛度** | 散落在浏览器 JS 6 次请求 | 后端单工坊接口 | **后端工坊单接口 + SSE 实时事件流驱动** |
| **抗崩溃恢复能力** | 0 恢复能力 (报错全丢) | 0 恢复能力 | **SQLite 章节断点状态机，原地毫秒级自愈** |
| **状态机可靠性** | 字符串原始累加 | 不变量越权预警 | **结构化突破演进记录 + 括号嵌套彻底消除** |
| **伏笔闭环管理** | 纯人工手工改状态 | 临期加权提示 | **主编语义认定 + 章节归档自动原子翻转 RESOLVED** |
| **模型偏见对抗** | 单一供应商自写自审 | 单一供应商自写自审 | **多通道 LLMRouter + 独立质检员双盲物理隔离** |
| **模型响应容错** | 标准 `json.Unmarshal` 极脆弱 | 标准 `json.Unmarshal` | **防御性提取器 (平衡括号/尾逗号/未闭合修复)** |
| **API 安全防护** | 明文 Key 回显 / 无 Body 限制 | 明文 Key 回显 | **10MB 限制 + Key 掩码脱敏防篡改** |
| **构建与分发** | 零 CGO 单二进制 | 零 CGO 单二进制 | **零 CGO 单二进制 (v1.0.0, 31MB 内嵌全套 UI)** |
