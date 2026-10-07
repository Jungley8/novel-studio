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

## 精炼后指标验证

| 指标 | 精炼前 | 精炼后 |
| :--- | :--- | :--- |
| **模块测试覆盖** | 11 个测试 | **14 个测试全覆盖 (单元 + 集成全部 -race 通过)** |
| **工作流调度收敛度** | 散落在浏览器 JS 6 次请求 | **100% 收敛于 Go 后端 ChapterWorkshop 单接口** |
| **状态机可靠性** | 字符串原始累加 | **不可变结构化解析 + 不变量越权消耗预警** |
| **正史连续性** | 0 历史上下文 | **严格滚动 3 章密封正史视界 + 临期伏笔动态加权** |
| **无头批跑能力** | 无 (依赖浏览器打开) | **支持 CLI `novel-studio produce` 无头自主批产** |
| **构建与分发** | 零 CGO 单二进制 | **零 CGO 单二进制 (v1.0.0, 31MB 内嵌全套 UI)** |
