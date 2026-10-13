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

### Step 6: 写作质量提升与质检纵深加固 (`novel-studio-quality-uplift.md` 全量落地)
依据质量提升规范落地 7 项关键质量升级：
1. **套词黑名单扩充与分层预警 (P0)**：
   - 核心级 (`CoreBannedKeywords`) 扩充至 60+ 高频 AI 废词（情绪悬浮、万能动作、时间过渡、比喻套路、打斗模板等），单次命中强制扣分驳回；
   - 预警级 (`WarningKeywords`) 频次监控，单次容忍，单章出现 ≥2 次即触发预警扣分。
2. **RenderScene Prompt 文学工程深度加固 (P0)**：
   - 注入声口个性与方言约束，严禁反派"哈哈哈"模板笑声；
   - 引入五感权重动态分层（战斗：听触视；阴谋：嗅视听；突破：触视听）；
   - 引入张力曲线指令 (`buildTensionCurvePrompt`)，依据 4 拍张力值动态下发短句连射/舒缓铺陈指令；
   - 引入平台特性调性注入 (`buildPlatformStylePrompt`)，支持番茄爽点钩子、起点战力设定、知乎盐言文学感；
   - 增加第三人称限制视角锁定与风格一致性铁律约束。
3. **节拍推演心理学模型扩展 (P1)**：
   - `domain.SceneBeat` 扩展 `ReaderEmotion`（读者情绪期望）、`InfoGap`（角色与读者信息不对称差）、`HookType`（章末留钩类型：悬念/反转/新谜/升级预告）；
   - `DeriveBeatsWithHorizon` 提示词全量对接新结构，推演具有读者情绪过山车设计。
4. **滚动正史文风锚定防漂移 (P1)**：
   - `CanonChronicle.AssembleHorizon` 提取前序章节最后 200 字正史原文作为 `TailAnchor` 注入正史视界；
   - `Orchestrator.RenderSceneWithHorizon` 明确下发承前风格锚定指令，彻底根除长篇连载文风漂移。
5. **动态对话/叙述比例智能调配 (P1)**：
   - `buildDialogueRatioPrompt` 依据节拍张力与剧情场景自动输出比例指令（打斗动作 15-25%，心理博弈 40-55%，日常推进 30-40%）。
6. **QualityGate 4 项算法叙述硬指标纵深 (P1)**：
   - `Linter.Analyze` 新增对话占比 (`DialogueRatio`)、段落方差 (`ParagraphVariance`)、高频 n-gram 重复短语 (`TopRepeatedNgrams`) 与感叹号密度 (`ExclamationDensity`)；
   - 算法指标全量注入 QualityGate 主审提示词与判决扣分门禁，极端值自动驳回重修。
7. **RewriteDraft 差分重写保护 (P2)**：
   - 注入差分精修铁律，保护主审未指出的优秀段落，严格限制修改幅度不超过 35%，集中火力精准清创。

---

## 精炼后指标验证

| 指标 | 精炼前 (Initial) | Refine v1.0 | 评审闭环迭代后 | 质量纵深加固后 (Current) |
| :--- | :--- | :--- | :--- | :--- |
| **模块测试覆盖** | 11 个测试 | 14 个测试 | 23 个测试 | **26 个测试全覆盖 (带 -race, 0 竞态, 0 泄漏)** |
| **写作质量策略** | 4 条简单规则 | 4 条简单规则 | 4 条简单规则 | **声口+五感权重+张力曲线+平台调性+前章文风锚定** |
| **套词拦截纵深** | 14 个扁平词库 | 14 个扁平词库 | 14 个扁平词库 | **60+ 核心级 + 预警级分层拦截** |
| **算法质检维度** | 仅突发度方差 | 突发度 + 14 套词 | 突发度 + 14 套词 | **突发度 + 60套词 + 对话占比 + 段落方差 + 重复n-gram + 感叹号密度** |
| **读者心理建模** | 纯动作节拍 | 纯动作节拍 | 纯动作节拍 | **情绪预期 + 信息差博弈 + 章末钩子结构化** |
| **返工精修模式** | 全文推翻重写 | 全文推翻重写 | 全文推翻重写 | **≤35% 局部差分靶向清创保护** |
| **构建与分发** | 零 CGO 单二进制 | 零 CGO 单二进制 | 零 CGO 单二进制 | **零 CGO 单二进制 (v1.0.0, 31MB 内嵌全套 UI)** |

---

## 宏观创世架构升级 (Genesis Architecture Upgrade - Phase 0 宏观骨架体系)

为了解决传统 AI 写作"缺乏宏观蓝图、盲目落笔第一章导致中途崩盘、战力通胀、主线迷失"的根本性缺陷，完成宏观创世系统 (Phase 0 Genesis) 全闭环建设：

1. **宏观领域模型扩充 (`internal/domain/models.go`)**：
   - 增加 `PowerLadderTier`（阶数、境界名、能力表征、突破瓶颈、反噬代价）；
   - 增加 `Faction`（势力名、立场、权力根基、暗线阴谋）；
   - 增加 `CharacterProfile`（姓名、初始境界、核心驱动、致命缺陷、弑神/金手指契机）；
   - 增加 `VolumeArc`（卷序、卷名、章节起止跨度、本卷核心破局使命、卷终高潮爆发点、活跃势力）；
   - 增加 `ProjectFramework` 创世总纲，挂载至 `Project`。
2. **SQLite 存储零停机迁移 (`internal/store/sqlite_store.go`)**：
   - 自动迁移新增 `framework_json TEXT` 字段；
   - `SaveProject` / `GetProject` / `ListProjects` 完整闭环序列化与反序列化。
3. **分卷视界与境界代价动态装配 (`internal/engine/chronicle.go`)**：
   - `CanonHorizon` 挂载 `CurrentVolume` 与 `ActivePowerTier`；
   - `AssembleHorizon` 依据当前生产的 `targetChapter`，自动判定所属分卷区间，注入当前卷终极使命，并根据主角境界匹配战力反噬代价。
4. **因果推演与双向对齐 (`internal/engine/orchestrator.go`)**：
   - `DeriveBeatsWithHorizon` 将分卷终极目标与战力代价铁律作为第一性约束注入 Reasoning 模型，杜绝偏离主线；
   - 新增 `BootstrapFramework(ctx, reasoningModel, req)` 宏观创世推演核心，由因果推理模型一键生成完整总纲。
5. **REST API 与自动化播种 (`internal/server/server.go`)**：
   - `POST /api/projects/bootstrap`：输入书名与灵感，一键推演总纲并在 SQLite 自动播种开篇种子伏笔池；
   - `GET /api/projects/:id/framework`、`PUT /api/projects/:id/framework`、`POST /api/projects/:id/framework/bootstrap` 支持随时查看、手工微调与再次 AI 推演。
6. **CLI 命令行创世工具 (`cmd/novel-studio/genesis.go`)**：
   - `novel-studio genesis -title "书名" -platform "平台" -concept "灵感"` 终端一行命令推演创世总纲。
7. **桌面工作台全流程打通 (`web/dist/index.html`)**：
   - 新增侧边栏 **创世总纲 (Framework Bible)** 专属面板，支持可视化编辑天道公理、战力天梯、分卷任务卡片与势力暗线谱系；
   - 新建项目弹窗升级为双模式，支持一键 "AI 宏观创世推演"；
   - 生产车间 (Workbench) 实时动态显示当前分卷使命与当前境界代价，全程锚定宏观视界。

---

## 对抗式审查缺陷整改与工业化加固 (Adversarial Code Audit & Deep Hardening)

针对 20 万~100 万字长篇连载场景下暴露出的 8 大并发、状态机与架构致命缺陷，完成全量逐行加固与纯净切换：

### 1. 🔴 多模型真实路由解耦 (LLMRouter 消除门面伪实现)
- **问题实质**：原 `LLMRouter` 的 `ChatCompletion` 接口无条件将全部请求打到 `r.defaultCl`，`Orchestrator` 未携带角色信息，导致配置的多 Provider（推理/创作/审校）解耦降偏形同虚设。
- **加固落地**：
  - `llm_client.go` 引入基于 Context 的角色机制：`ContextWithRole`、`RoleFromContext`、`RoleReasoner`、`RoleWriter`、`RoleReviewer`；
  - `LLMRouter` 升级 `resolveClientAndModel` 真实路由：优先通过 Context 角色映射到配置的 `ReasonerProvider`、`WriterProvider` 或 `ReviewerProvider`，次选模型名前缀匹配；
  - `Orchestrator` 与 `QualityGate` 全线在调用时注入对应角色，并在服务启动时将 `router` 注入 `Orchestrator`，实现真正的三模型物理隔离与交叉审校。

### 2. 🔴 Token 真实度量与成本核算全面打通 (Eliminate Fake Accounting)
- **问题实质**：原流水线中的 Token 使用量全为硬编码常数（800/1200/1500等），前端展示的 Token 和账单完全失真。
- **加固落地**：
  - `Orchestrator.DeriveBeatsWithHorizon`、`RenderScene`、`ReviewDraft`、`RewriteDraft`、`BootstrapFramework` 全面升级为调用底层 `ChatCompletionWithUsage`，返回真实消耗的 `TokenUsage`；
  - `QualityGate.Audit` 与 `AuditWithHooks` 实时统计审校阶段产生的 `TokenUsage`；
  - `ChapterWorkshop.ProduceChapter` 移除全部常数 Mock，动态累加各阶段真实 Token，输出精确的 `TotalUsage` 与 `EstimatedCostUSD`。

### 3. 🔴 状态机道具数量数学化解析与不变量加固 (Mathematical Inventory & Quantities)
- **问题实质**：原实体状态机在更新 `StructuredItems` 时直接将数量写死为 `1`，且字符串匹配存在子串误判，导致长篇连载中道具累积在多章结算后被清零。
- **加固落地**：
  - `domain/ledger.go` 引入词法提取正则（`qtyMultiplierRegex`、`qtyChineseUnitRegex`、`qtyTrailingNumRegex`），精准提取“洗髓丹x3”、“灵石 100块”等数量表达；
  - 加减结算基于结构化 `InventoryItem` 执行数学运算（加法累加，减法按量扣减或耗尽移除）；
  - `Inventory` 自然语言字符串采用规范化格式输出（数量 > 1 时追加 `xN`，数量 = 1 时输出标准品名），彻底杜绝数量清零与不变量崩坏。

### 4. 🟠 SSE 流式通道防泄漏与 Goroutine 安全回收 (Stream Cancellation Leak Prevention)
- **问题实质**：原 `ChatCompletionStream` 中的 `out <- chunk` 缺乏 `ctx.Done()` 监听，前端中断或网络异常时后台 goroutine 永久阻塞，导致连接与文件句柄泄漏。
- **加固落地**：
  - 将所有流式 channel 发送包裹在 `select { case out <- chunk: case <-ctx.Done(): return }`；
  - 客户端取消或连接断开时，goroutine 立即安全返回，确保 `resp.Body` 及时关闭与资源回收。

### 5. 🟠 QualityGate 审校裁决优先级严格收拢 (Fail-Closed Verdict Security)
- **问题实质**：当审校 LLM 明确判定 `REVISION_NEEDED` 时，若综合评分 ≥ 80 分，原代码会粗暴覆盖为 `ACCEPTED` 直接放行。
- **加固落地**：
  - `QualityGate.AuditWithHooks` 建立严格 Fail-Closed 判定：只有当审校模型明确判定 `out.Verdict == ReviewVerdictAccepted`，且综合评分 ≥ 80，且无禁用词与极端格式异常时，方可放行；
  - 语义主审的一票否决权得到绝对保证，存在逻辑漏洞的章节严禁归档。

### 6. 🟠 断点锁死 (Stale Lock) 与返工计数器连续追踪 (Robust Checkpoint Recovery)
- **问题实质**：失败的断点未被及时清除导致同一章节永远读出旧草稿锁死；断点恢复后返工计数器重新从 0 开始计算导致最大返工轮次限制失效。
- **加固落地**：
  - `ChapterWorkshop.ProduceChapter` 严格依据 `req.ResumeCheckpoint` 判断；非恢复模式下主动调用 `ClearCheckpoint` 清理脏数据；
  - 断点恢复时从 `cp.RewriteLoops` 恢复真实轮次，并持续递增追踪，严格保证不超过 `MaxRewriteLoops`。

### 7. 🟡 JSON 修复器词法边界保护 (Tokenizer-Aware JSON Repair)
- **问题实质**：原字符串尾逗号正则与括号统计会破坏文本内部合法的逗号与花括号。
- **加固落地**：
  - 重构 `removeTrailingCommasAware` 与 `countUnclosedTokens`，引入字符串字面量转义感知扫描，完全忽略引号内部的逗号与括号；
  - 正文对话中即便包含“{天元, 归一}”等符号，也不会干扰外部 JSON 结构的提取与修复。

### 8. 🟡 跨卷长线伏笔因果历史场景还原 (Cross-Volume Plot Hook Narrative Recall)
- **问题实质**：跨卷线索此前仅拼接章节标题与核心冲突，缺失埋设伏笔时的具体上下文场景段落。
- **加固落地**：
  - `CanonChronicle.AssembleHorizon` 关联各历史章所埋设的全部活跃伏笔详情（标题、目标章节、状态、要点）；
  - 引入 `extractHookSceneExcerpt` 智能提取器：基于伏笔关键词定位原章正文现场，精准截取前后 150 字场景实况还原注入 Layer 3 历史因果线索，为长线伏笔的伏笔回收与解密提供高保真度上下文。

---

## 终态质量总体验证

- **编译构建**：纯 Go 零 CGO 单二进制通过 (`go build ./...`)
- **全量测试**：`go test -count=1 -race ./...` 100% 通过（29 个测试用例，覆盖并发、状态机、多模型路由、断点恢复、Token 真实统计等核心路径）
- **格式规范**：`gofmt -w .` 全量格式化完成
- **安全与稳定性**：所有状态流转严格遵循 Fail-Closed 原则，无降级垫片，无 Mock 伪实现。

---

## 阶段六：NovelStudio 2.0 对标 Novelcrafter 架构全面重塑与落地

日期：2026-10-08  
核心目标：基于海外 AI 辅助小说标杆 **Novelcrafter** 的核心特性，结合 NovelStudio 原有的因果状态机与单二进制护城河，全面落地 4 大里程碑功能，完成从纯后台推演工具向现代化沉浸式 AI 创作工坊的跨越。

### Milestone 1: The Codex 全域世界观百科
1. **领域实体模型 (`internal/domain/codex.go`)**：
   - `CodexCategory`：支持 `CHARACTER`(角色)、`LOCATION`(地理)、`LORE`(公理)、`ITEM`(道具) 四大基石分类；
   - `TrackingMode`：支持 `AUTO_MENTION`(智能引用自动注入)、`ALWAYS_INJECT`(全量强制注入)、`MANUAL`(手动引用)；
   - `Progression` 阶段演进时间线：支持按 `active_from_chapter` 记录实体的属性、修为、心境与持有物快照，提供 `ActiveProgression(chapterIndex)` 确定性解析特定章节时刻的状态；
   - `EntityRelation` 两两关系拓扑：记录两个实体之间的关系类型 (`NEMESIS`/`ALLY`/`MENTOR` 等) 与因果说明。
2. **零 CGO SQLite 存储实现 (`internal/store/sqlite_store.go`)**：
   - 建立 `codex_entries`、`codex_aliases`、`codex_progressions`、`codex_relations` 四张表及索引；
   - 实现级联删除与事务一致性，提供完整的 CRUD 操作与关系查询。
3. **实体引用智能扫描器 (`internal/engine/mention_scanner.go`)**：
   - 支持别名与主名称的高性能上下文正则匹配与字位定位；
   - 区分 `ALWAYS_INJECT` 与普通命中实体。
4. **动态正史视界四层装配 (`internal/engine/chronicle.go`)**：
   - 在 `AssembleHorizon` 中动态扫描最近正史与尾部锚点，提取活跃百科实体；
   - 智能解析特定章节的演进快照，并仅在两实体于该场景共现时精准激活两两关系，注入 Layer 4 百科视界，严控 Token 膨胀。
5. **API 与测试闭环**：
   - 完整交付 `/api/projects/:id/codex` 相关接口（增删改查、阶段演进、关系网络、正文扫描）。

### Milestone 2: The Matrix 矩阵大纲与原子场次
1. **层次化大纲模型 (`internal/domain/matrix.go`)**：
   - `Scene`：定义原子叙事场次，包含 `SceneIndex`、`Title`、`DramaticGoal`(戏剧目标)、`ConflictBarrier`(冲突阻碍)、`TensionLevel`(1-10 张力阶梯)、`WordCount`；
   - `SceneMarker`：支持在场次中打上 `TODO`、`PLOT_HOLE`、`HOOK_ANCHOR`、`NOTE` 批注；
   - `MatrixVolumeGroup` 与 `MatrixOverview`：实现 Volume ➔ Chapter ➔ Scene 4 层树状结构与全书字数、平均张力多维表格聚合。
2. **SQLite 存储持久化**：
   - 建立 `scenes` 与 `scene_markers` 表与级联外键，实现场次与批注的高效存取。
3. **API 路由**：
   - 交付 `/api/projects/:id/matrix`、`/api/projects/:id/scenes`、`/api/scenes/:id`、`/api/scenes/:id/markers`。

### Milestone 3: Manuscript 手稿协作与透明提示词
1. **划词 Inline AI 伴写动作 (`/api/workshop/inline-action`)**：
   - 支持 `rewrite`(文学润色)、`expand`(细节扩写)、`shorten`(精简提炼)、`sensory`(五感沉浸强化)、`dialogue`(台词机锋打磨)、`custom`(自定义定向指令)；
   - 统一由 `LLMRouter` 调度 `RoleWriter` 工业级输出，无多余寒暄废话。
2. **提示词透视镜 Prompt Inspector (`/api/projects/:id/prompt-preview`)**：
   - 彻底打破黑盒，将发往大模型的提示词解构为 4 大可视化积木卡片：
     1. 世界观与风格公理 (Global Framework)
     2. 因果故事线记忆 (Canon Horizon)
     3. 全域百科与关系 (The Codex Injected)
     4. 场景戏剧规格 (Scene Dramaturgy)
   - 实时输出各积木的 Token 估算与完整组装文本，支持一键复制。

### Milestone 4: Workshop 情境对话与全书态势分析
1. **情境对话抽屉 (`/api/projects/:id/chat`)**：
   - `character_roleplay`：动态绑定百科中该角色的最新设定、当前章节阶段状态与人际关系网，以第一人称严格保持人设与创作者对戏；
   - `scene_brainstorm`：结合世界观规则，协助构思反转高能桥段；
   - `editor_critique`：以严苛总编视角对剧情逻辑与节奏发起质询。
2. **全书态势监控 (`/api/projects/:id/analytics`)**：
   - `heatmap`：基于 `MentionScanner` 计算所有实体在全书各章节的出场频次矩阵，生成角色出场热力图；
   - `tension`：提取全书各章节与场次的张力等级（1-10），生成戏剧张力心流分布直方图。

### 单二进制 Web 终端界面深度集成
- `web/dist/index.html`：
  - 侧边栏新增 **🔲 矩阵大纲 (The Matrix)**、**📖 全域百科 (The Codex)**、**📊 态势分析 (Analytics)** 三大一级视图；
  - 沉浸式手稿编辑器集成提示词透视 (Inspector) 弹窗、情境工坊 (Chat) 侧边抽屉、划词/段落 AI 快速操作栏及差分替换对比框；
  - 零外部运行时依赖，由 Go 单二进制通过 `//go:embed all:dist` 纯净交付。

### 自动化验证与质量门禁
- **单元与集成测试**：全量测试套件通过（`go test -count=1 -race ./...`，覆盖 Domain、Store、Engine、Server 各层，包含新增的 Matrix、Codex、Analytics、PromptPreview 测试）；
- **单二进制构建与体检**：`go build -o novel-studio ./cmd/novel-studio && ./novel-studio doctor` 全部通过（6 项通过，0 项失败）。

---

## 商业级去AI味攻防与国内平台合规体系 (Anti-AI Detection & Censor Harmonizer)

针对长篇正文在腾讯朱雀 AI 检测助手出现 `No human creation detected, suspected AI 100%` 的问题，实施双轨落地方案（一+二全量上线）：

### 1. 商业检测器机理破解与根因定位
- **均匀概率分布陷阱 (Low Perplexity)**：大模型生成的词汇均处于 Top-5 极高概率分布分支，整篇文本缺乏方差；
- **极端纯净叙事陷阱 (Hyper-Purity)**：800 字正文全部集中于物理动作与主线推进，零走神、零生理抗力、零世俗生活摩擦；
- **机械断奏对称排比 (Staccato Cadence)**：强行要求短句导致出现匀称的“7字+逗号+7字”机器鼓点。

### 2. 解法二：国内关键词合规和谐与对抗扰动 (`internal/engine/harmonizer.go`)
- **涉暴与违规敏词平滑替换 (`HarmonizeSensitiveWords`)**：
  - “开膛破肚” ➔ “重创倒地”
  - “血肉模糊” ➔ “一片狼藉”
  - “碎成肉泥” ➔ “筋骨尽断”
  - “尸体” ➔ “残躯”
  - “死人” ➔ “亡者”
  - 粗俗脏话平滑转换为市井武侠俚语。
- **对抗性词汇似然度扰动 (`Perturb`)**：
  - 将高频 AI 动词与连接词（“走过去”、“看着”、“拿出了”）动态扰动为低概率触感动词（“大步踏过去”、“乜斜着盯牢”、“怀里摸出”）；
  - 打散对称逗号，注入不规则破折号与停顿。
- **全自动流水线接入 (`ProduceChapter`)**：在自主章节生产流水线中无缝集成合规与对抗扰动，生成合规审计报告。

### 3. 解法一：人机协同人味杂质注入 (`SuggestHumanTouches`)
- 提供 4 维人味杂质生成与破防原理推荐：
  1. `PHYSIOLOGY` (生理不适偏见)：胃痉挛、冷汗浸透后背、旧伤隐痛、咽喉干咳；
  2. `TRIVIALITY` (生活物质闲笔)：油灯爆芯、粗茶浮沫、鞋底踩入碎石、桌面薄灰指印；
  3. `COLLOQUIALISM` (市井口癖碎屑)：粗鄙俗话、口语叹词、说话停顿；
  4. `CADENCE` (标点断裂顿挫)：短促急停、破折留白，打破机械对称。

### 4. 去 AI 味元提示词升级 (`orchestrator.go` & `linter.go`)
- **状语/副词剥夺法则**：能不用状语就不用状语，能不用副词就不用副词；禁用“...地”修饰动词，强动词顶格；
- **公式化比喻清零**：“像/如/犹如/宛若”清零，超过 2 处直接驳回返工；
- **套路微表情清零**：全面封杀“嘴角僵硬弧度”、“眼角没有笑意”、“瞳孔缩成针尖”等 AI 套路描写。

### 5. 桌面与前端工作台可视化交付
- **Vite + Vue 3 + Tailwind CSS 现代工程构建**：替代单文件 `index.html`，组件化全量重构；
- **合规与扰动弹窗 (`HarmonizeModal.vue`)**：支持 0.1~1.0 扰动强度调节、敏感词替换前后清单对比与一键替换手稿；
- **人味注入建议弹窗 (`HumanTouchesModal.vue`)**：支持 4 类人味杂质筛选、破防原理透视与一键插入当前手稿；
- **手稿顶栏与 Step 4 终审门禁集成**：快速触发质检、合规审查与人味建议。

---

## 跨作品串稿专项精炼（Cross-Project State Isolation）

日期：2026-10-12
触发症状：**工作台切换作品后，正文/分段/审校/停线横幅仍是上一个作品的**（用户报告）。
方法论：`vibe-refine`（先锁定回归测试，再逐层修复）+ 批判式评审（全视图扫描同类串稿面）。

### 根因定位（三层叠加）

1. `selectProject` 只重载列表数据（chapters/hooks/codex/…），**未重置作品级瞬态状态**：`workbench.*`、`reviewResult`、`linterReport`、`editingChapterIndex`、`activeStep`、`pipelineState`、弹窗基质（`editingScene`/`editingCodexEntry`/`activeCodexForSub`/`activeSceneForMarkers`…）。
2. `restoreCheckpoint` 的内容守卫 `if (content && !targetIndex) return;` 在切换场景下被旧正文触发 → **新作品的断点被跳过恢复**，旧正文原样留存；且无参调用会使用旧作品的 `editingChapterIndex` 取错章节断点。
3. 画布防抖自动保存（1.5s）与划词改写缓冲在切换后仍有在途回调，`saveCheckpoint` 按“当前作品”取项目 id → **旧作品正文被写入新作品的断点**（数据串稿）。

### 精炼操作（按提交序列）

1. `fix(web): fail-closed project switch with full project-scoped state reset`
   - 新增 `stores/domain/projectScope.js` 单一事实源：默认工作台/体检报告/流水线状态工厂 + `activeGenerationTask` 互斥任务探测 + `resetProjectScopedState` 整体重置；
   - `selectProject` 重写：生成任务进行中拒绝切换（Fail-Closed，含友好提示）；切换前把旧作品在途草稿显式落盘回旧作品；加载失败回退选中态（杜绝半切换）；
   - 恢复断点显式传 `chapters.length + 1`，彻底禁用旧 `editingChapterIndex` 串稿；
   - `appState.bindActions`：域操作对象方法统一绑定自身，修复裸引用调用（`@click="actions.foo"`）导致的 `this` undefined 崩溃（一键成章/去机械壳受影响）。
2. `fix(web): reset per-view local state and eliminate cross-project payload leaks`
   - 10+ 视图/弹窗（章节目录、意图、百科、矩阵、图谱、指标、体检、时间线、角色对戏、人味建议、合规扰动…）增加作品切换重置；
   - 通知消息记录 `projectId`，跨作品跳转前先切换、拒绝则不跳转；
   - 修复 `bootstrapCurrentFramework` 的 `actionHelpers` ReferenceError（框架推演成功却被报“推演失败”）；
   - 修复矩阵「智能拆解」payload 与后端契约不匹配（原发字符串 → 400），改为 `{chapter_index, chapter_title, core_conflict, prompt}`。
3. `fix(server,store): reject codex updates with foreign-project entry id`
   - 新增 `CodexEntryProjectID` 归属查询；`PUT /api/projects/:id/codex/:entryId` 对其它作品词条返回 409，杜绝越权覆盖。
4. `test(web): add project-switch isolation regression tests`
   - Node 原生测试 + 无扩展名 resolve loader（零新增依赖）：切换整体重置、旧草稿落盘、Fail-Closed 门禁、断点目标 = 新作品章节数 + 1。

### 验证

- `pnpm test`：3/3 通过；
- `go test ./... -race -count=1`：全绿（含新增 `TestCodexUpdate_RejectsForeignProjectEntry`）；
- `pnpm build` + `gofmt -w`：通过。

### 第二轮加固（2026-10-12 续）

1. `fix(server,store): enforce cross-project ownership and add latest-checkpoint endpoint`
   - 伏笔 POST 更新：`PlotHookProjectID` 归属校验，越权 409（upsert 不再借道覆盖）；
   - 词条演进 `progressions`：`CodexEntryProjectID` 校验，越权 404；
   - 关系创建：两端词条必须归属本作品，跨作品引用 400；
   - 场次：GET/PUT/DELETE 校验 `scene.project_id`，越权 404；创建/更新时挂载的已归档章节必须归属本作品；
   - 新增 `GET /api/projects/:id/checkpoint?latest=true`（`GetLatestCheckpoint`，按 `updated_at DESC`）。
2. `feat(web): restore latest in-flight draft on project switch`
   - `restoreCheckpoint` 拆分为 `applyCheckpoint` + 按章恢复；新增 `restoreLatestCheckpoint`，切换作品时恢复“最近推进的一章”（覆盖已归档章节重修稿场景）；
   - 回归测试新增第 4 例：latest 断点恢复断言。

### 第三轮加固：前台编辑 ↔ 后台生产并发互斥（2026-10-12 续）

背景：单二进制内，无人值守调度器与桌面 UI 共享进程。用户正在精修章节
时，调度器可能对同一作品启动自动续产，二者对同章断点/正史并发写入为
last-writer-wins，且 `CommitChapter` 的 `StateMutation` 结算非幂等。

**方案（进程内协调器 + Fail-Closed 写入门禁）**：

1. 新增 `internal/coord`：作品级「编辑租约（TTL/心跳续期）」+「生产锁（单生产者、代际隔离释放）」。
   - `BeginProduction`：编辑租约未过期或已有生产任务 → `coord.ErrBusy`；
   - `AcquireEdit`：生产期间仍可获取（仅表达“人工在场”，冲突在写入口裁决）。
2. 调度器（`scheduler.RunOnce`）：`RunContinuation` 返回 `ErrBusy` 时跳过该作品，
   **不记录 last_run**（双方收工后下一趟 tick 自动衔接重试），不告警、不暂停。
3. 生产登记（`schedulerRuntime.RunContinuation`）：开工前持锁，逐章
   `SetProductionChapter`（整卷模式从 VolumeEvent 跟随章节号）。
4. 正史写入门禁（`handlers_*`）：生产锁生效期间，`定稿 / 撤回归档 / 删除章节 /
   一键成章` 一律 409 并回传生产任务标签与章节号；断点自动保存保持放行
   （正文不丢失，冲突在定稿处裁决）。
5. 前端：`POST/DELETE /api/projects/:id/edit-lease` 心跳（TTL 120s / 40s 续期，
   仅页面可见且在途稿存在时持有；切走/完工自动释放，关闭页面由 TTL 兜底）；
   「一键成章」视为人工接管，先释放自身租约再登记生产锁。
6. 复核结论：`CommitChapter` 覆盖式重定稿的“再次结算 StateMutation + 刷新快照”
   语义经 `TestChapterHistory_Case2_DeduplicationAndOverwrite` 确认为**有意设计**
   （每次定稿都是一次状态贡献，纠错走撤回归档），本轮**不改动**其语义，
   并发正确性由上述互斥层保证。

验证：`internal/coord` 单测 5 例；调度器忙跳过用例；白盒接线测试
（4 个写入口 409 + 租约端点 + 生产结束后 201 恢复）；全量 `go test ./... -race` 全绿。

### 第四轮加固：协调层升级为 SQLite 单一事实源（跨进程互斥，2026-10-12 续）

第三轮的编辑租约/生产锁存于**进程内存**，只覆盖守护服务内的调度器与 UI；
CLI 无头生产（`novel-studio produce`）是独立进程，仍是互斥盲区。
本轮按“单一事实源、无双轨”原则完成升级：

1. **契约收窄**（`internal/coord`）：只保留租约类型（edit/production）、
   `ErrBusy` 哨兵与 TTL/心跳常量；内存实现整体移除。
2. **存储层**（`internal/store/sqlite_store_locks.go` + 迁移 `coordination_leases` 表）：
   - `UpsertEditLease` / `DeleteEditLease` / `GetLiveLease`（过期即不可见）；
   - `AcquireProductionLease`：`BEGIN IMMEDIATE` 事务内清理过期行 + 查在途租约
     （任意存活租约 → 返回阻塞租约 + `ErrBusy`）→ 写入带 holder token 的生产锁；
   - `HeartbeatProductionLease`（holder 隔离续期 + 章节号跟随）、
     `ReleaseProductionLease`（仅 holder 匹配可释放，代际隔离）、
     `CleanupExpiredLeases`（守护服务启动期清理崩溃残留）。
3. **三方接线**：守护调度器（RunContinuation）、UI 一键成章（handleWorkshopProduce）、
   CLI produce 全部改为“持锁生产 + 心跳续期 + defer 释放”；失败路径
   （编辑租约在场 / 已有生产）一律拒绝并展示对方标签与章节号。
   - UI 一键成章视为人工接管：先释放本窗口编辑租约再取生产锁；
   - 浏览器心跳租约写入同一张表，因此 CLI 也能看见“人工在场”并拒绝。
4. **进程崩溃兜底**：租约带 TTL（编辑 120s / 生产 120s，30s 心跳），
   崩溃进程的锁自动过期，不再需要人工清理。

验证：store 双实例（模拟守护/CLI）5 例跨进程测试；CLI 忙碌路径 2 例
（编辑租约/生产锁在场 → 退出码 1 + 文案 + 无残留锁）；全量
`go test ./... -race` 全绿。

### 第五轮加固：开书窗口闭环 —— 创世即登记生产锁（2026-10-12 再续）

遗留缺口：`RunBook` 开书（及 UI/CLI 开书）的 ProjectID 在流水线中途才生成，
项目创世入库到批产结束之间没有生产锁——新作品已出现在作品列表，
人工可此刻点入编辑，与在途开书批产并发写同一作品。

本轮以“创世回调”补齐（单一机制、全部开书入口统一）：

1. **`PipelineRequest.OnProjectCreated`**（`internal/engine/pipeline.go`）：
   `SaveProject` 提交后、任何项目内写入（种子注入/意图/批产）之前
   同步回调携带 ProjectID；返回错误中止流水线（Fail-Closed，
   文档化的半状态：项目已入库、种子未注入）。
2. **`coord.Production` + `coord.BindProduction`**（`internal/coord/production.go`）：
   生产锁的公共持有器（获取 + 心跳续期 + SetChapter + 幂等 Release），
   并提供“hook / trackChapter / release”绑定的统一实现；
   此前三处手工心跳循环（handleWorkshopProduce / RunContinuation / CLI produce）
   全部收敛到该实现。
3. **全部开书入口接入**：排程 RunBook（`排程开书`）、UI 开书端点
   SSE+非 SSE（`浏览器开书`）、CLI pipeline（`CLI 开书`）、
   CLI genesis 创世（`CLI 创世`，种子注入期间持锁）。
   锁在位期间：调度器续产跳过、正史写入口 409、CLI produce 拒绝，
   与第四轮的跨进程语义完全一致。

验证：coord 持有器单测 5 例（忙碌透传/参数校验/心跳跟随章节并随 Release
停止/hook 绑定/忙碌 Fail-Closed）；engine 钩子单测 3 例（时点早于任何批产、
失败中止且不进入生产、真实 SQLite 端到端——批产期间锁对其它进程可见、
Run 结束后归调用方释放）；全量 `go test ./... -race` 全绿。

### 开发环境清理：移除死配置字段、修复开发服务器构建与端口隔离（2026-10-12）

排查背景：上报"开发环境仍会自动打开浏览器"。彻查结论——浏览器自动唤起
的历史来源是 `577ad07`（Wails 原生窗口重构）之前的
`!noBrowserFlag && cfg.AutoOpenBrowser` 启动逻辑（默认 true，且用户
`config.json` 中持久为 true）；该逻辑已于 577ad07 整体移除，与
`ba74e8a`/`f5cc39a` 的 `--no-open` 属不同层面、互不相干（Vite 默认即不
自动开浏览器）。实测 `pnpm dev` 与完整 `wails3 dev` 流程（PATH shim 拦截
`open`/`osascript`/`xdg-open`）均为零浏览器调用；当前 HEAD 无任何自动唤起路径。

本轮清理（消除误导源，非功能修复）：

1. **移除死字段**：`AutoOpenBrowser`（`internal/config/config.go`）自 Wails
   重构后无任何读取方，连同默认值一并删除；`~/.novel-studio/config.json`
   残留键同步清除（`json.Unmarshal` 忽略未知键，删除安全）。
2. **文档纠偏**：README 交互流程图不再宣称"自动唤起系统默认浏览器"
   （现状为原生桌面窗口）；`-no-browser` 帮助文案改为
   "无头模式运行 (不启动原生桌面窗口，仅监听本地 HTTP 服务)"——
   该标志的实际语义是无头服务模式，与浏览器无关。
3. **修复开发服务器构建**：`task run:server`（DEV=true）依赖的前端脚本
   `build:dev` 缺失（此前 `task build:server DEV=true` 必然
   `ERR_PNPM_NO_SCRIPT`），补上
   `vite build --minify false --mode development`（对齐 Wails v3 官方模板）。
4. **开发 Web 端口隔离**：默认 5173 与工作区内常驻的 octarq 系 Vite
   （`--host` 占 `*:5173`）冲突，novel-studio 开发前端端口改为 **5183**
   （`vite.config.js` / `Taskfile.yml` / `build/config.yml` 就绪探测 /
   启动横幅 / README 全量对齐；仍支持 `VITE_PORT` / `WAILS_VITE_PORT` 覆盖）。

验证：`go build ./...`、`go build -tags server`、全量 `go test ./... -race`
全绿，`gofmt -l` 无输出；`wails3 task dev --dry` 解析为 `-port 5183`；
`wails3 task build:server --dry DEV=true` 解析出 `pnpm run build:dev`；
web 侧 `pnpm test` 4/4 通过，`pnpm run build:dev` 可产出非压缩构建。

### 第七轮：LLM 数组输出解析健壮性统一（2026-10-12 夜 · 全自动成书实战暴露）

背景：以「全自动跑出一本 3 万字短篇」为端到端验收时，`pipeline` 在第 2 步
（卷意图卡生成）炸线：推理模型偶发把「要求裸数组」的意图卡输出包成对象
（或用不同包装键），而解析器只接受裸数组 + 固定 `{"intents": …}` 包装，
于是 `json: cannot unmarshal object into []engine.rawIntentCard` 直接失败。

修复：新增 `internal/engine/json_array.go` 泛型助手 `UnmarshalJSONArray[T]`：
- 裸数组直通；
- 对象包装：推荐键（intents/cards/scenes/hooks/sections…）优先、任意键回退；
- 数字键映射按数字序还原为有序切片；
- 空数组视为合法「无条目」结果（如伏笔「无则输出 []」）；
- 完全找不到数组时报出可诊断错误。

统一替换 6 个「期望数组」的 LLM 输出解析点：卷意图卡（`intent_planner.go`）、
关系图谱（`orchestrator_codex.go`）、矩阵拆场与伏笔（`orchestrator_matrix.go`）、
埋点伏笔注册（`hook_registrar.go`）、分卷分节（`outline_sections.go`）。

验证：新增 `json_array_test.go` 7 个边界用例（裸数组/推荐键/任意键/优先键裁决/
数字键排序/空数组/不可定位报错）全绿；全量 `go test ./... -race` 全绿；`gofmt` 完成。

### 第八轮：审校分庭超时加固（2026-10-12 夜 · 全自动成书实战第二障碍）

实战：第 1 章正文渲染成功（3404 字）后，QualityGate 三庭会审炸线——
`logic division LLM review failed: 流式接收中断 (连接超时): context deadline exceeded`。
根因：`ReviewCourt` 三庭（逻辑/文风/连续性）并行会审，每庭硬编码 **45s** 超时；
推理型审校模型对整章草稿 + 启发式指标 + 正史视界的会审耗时远超 45s，
流被 ctx 掐断且无重试 → 章节生产失败 → 卷停线（ERROR）。

修复：分庭超时默认 45s → **180s**（`SetDivisionTimeout` 仍可覆盖；
分庭级重试本次未引入，保持 fail-closed 语义，后续可评估）。

验证：`go test ./internal/engine/ -race` 全绿；自跑实战重验（见书稿档案）。

### 第九轮：审校 JSON 未转义引号修复（2026-10-12 夜 · 全自动成书实战第三障碍）

实战：第 1 章渲染成功、三庭会审完成，但风格庭输出解析炸线——
`parse style division JSON failed: invalid character '您' after array element`。
根因：审校意见会**逐字引用正文台词**（如 `老冯口中的两处（"您猜怎么着——""奇就奇在这儿——"）`），
模型在字符串值内部输出未转义的半角双引号；`ExtractAndCleanJSON` 既有修复
（尾逗号/未闭合括号/围栏）全部以「字符串状态正确」为前提，欠转义输入令状态机
错位 → 修复失败 → 兜底粗切仍非法 → 解析炸线。

修复（双层）：
1. 提取器新增 `repairUnescapedQuotes`：字符串态内遇到半角引号时，仅当其后随
   非空白字节为 `,` `}` `]` `:` 或输入结束才判定为字符串终点，否则判为字面
   内容并转义；对结构合法 JSON 恒等（零回归）；接入 `attemptValidateAndSanitize`
   的第一步，全引擎 JSON 路径受益。
2. 三庭（逻辑/风格/连续性）提示词增补引文规范：字符串值内引用正文一律使用
   中文引号「」/『』，禁止未转义的半角双引号。

验证：新增 `json_quote_repair_test.go`（恒等性/引号短语/连续引号/尾逗号组合/
真实炸线载荷复现）全绿；既有 10 个提取器边界用例零回归；全量
`go test ./... -race` 全绿。

### 第十轮：LLM 瞬态传输故障有界重试（2026-10-12 深夜 · 全自动成书实战第四障碍）

实战：第 1 章渲染调用被上游 EOF 掐断（`do stream HTTP request: … EOF`），
客户端原契约「传输错误不重试、直接冒泡」→ 一次网络抖动废掉整卷。

修复：`HTTPLLMClient` 增补有界重试：
- `isRetryableLLMError`：EOF/意外 EOF/net.Error/超时/连接重置/断管/HTTP 5xx/429
  可重试；HTTP 4xx（非 429）与内容级错误（解析失败等）一律不重试；
- `chatCompletionOnceWithRetry` / `streamOnceWithRetry`：最多 2 次追加尝试，
  2s/4s 退避，全程受调用方 ctx 门控（ctx 已死或分庭预算耗尽即不再重试）；
- 接入 `ChatCompletionWithUsage` 与 `SafeChatCompletionWithStreamFallback`，
  同步/流式全路径自愈。

契约测试更新（原「绝不重试」契约 → 「有界重试后仍 Fail-Closed」）：
- `TestHTTPLLMClient_BoundedRetryThenFailClosedOn429`（持久 429：恰 3 次尝试）；
- `TestStream_NetworkErrorRetriesBoundedThenFails`（持续断流：恰 3 次尝试，
  且重试载荷必须是全新初始请求，绝不伪装成「输出截断续写」）；
- `TestMatrixGenerateFromIntents_PartialFailureContinues` 伪上游改为按章节内容
  持续 500（与重试预算解耦）；
- `TestHTTPLLMClient_TimeoutStopsRetryStorm` 原样通过（ctx 超时不自旋）。

验证：14 例可重试分类表 + 伪 flaky 服务器用例全绿；全量 `go test ./... -race` 全绿。


### 第十一轮：审校模型实战选型与分庭预算调整（2026-10-13 凌晨 · 全自动成书实战第五障碍）

实战：第五轮，连续性分庭在 180s 预算内未完成（`context deadline exceeded`）。

归因实验（真实第 1 章 3024 字草稿 + 逐字提示词，流式横评）：
- `glm-5.3`：4s 收到首事件，300s 内烧掉 7 万字符推理仍无正文（思考失控）→ 不可用；
- `deepseek-v4.1-flash`：~181s；`kimi-k2.6`：~199s（可用但紧）；
- `qwen3.8-flash`：108–153s，两次全过，评审抓出真实状态机漏洞（铜钥匙空间状态跳跃、
  遗物登记动机缺口）→ **选为审校模型**；
- `mimo-v2.6-flash`：~62s（最快且评审扎实，列为备用）；
- `deepseek-v4-flash` / `deepseek-flash` / `glm-5.3-flash`：300s 无正文（同类思考失控）；
- `minimax-m3`：请求即败。

变更：
1. 配置：`reviewer_model` → `qwen3.8-flash`（仍是 Zen/go chat 协议）；
2. 代码：`ReviewCourt` 分庭预算 180s → **360s**，为后段章节递增的上下文本量留
   2x+ 余量；真实挂死仍由预算 Fail-Closed 停线。

验证：`go test ./internal/engine/ -race` 全绿；实战续产验证（见下）。


### 第十二轮：创世去硬编码 —— 主角档案归框架（2026-10-13 凌晨 · 全自动成书实战第六障碍）

实战：（续产第 1 章时）状态结算对账反复报「物品记入顾渊名下，但正文主角为程穗」。
根因：`seed.go` 内 `protagonistName := "顾渊"` 写死（遗留演示数据）——每本书的主角
条目都叫顾渊、随身物品写死仙侠三件套（凡骨铁印/粗布短褐/引路符），还硬编码种入
地点「聚仙楼」与道具「凡骨残印」；配角别名写死楚掌柜/柳依依；渲染提示词示例句
也含「顾渊」。框架推演的配角与正文意图围绕灵感（程穗）展开，主角条目却永远
是顾渊 → 框架级人格分裂 → 状态机错位、审校必然低分、返工闭环无法收敛。

修复（严格 Fail-Closed，无任何硬编码回退）：
1. `domain.ProjectFramework` 新增 `protagonist` 档案（name/identity/initial_realm/
   goal/voice_tone/inventory）；创世提示词新增「0. 主角归属铁律」：灵感点名者即
   主角，严禁改名换人/另立并列主角；key_characters 明示不含主角；
2. `SeedGenesisArtifacts` 全量改为档案驱动；档案缺失 → 直接报错拒绝启动；
   删除全部写死的主角名/仙侠物品/聚仙楼/凡骨残印/配角别名/写死腔调；
3. 种子后一并剥离 `fw.Protagonist`（与 Factions/KeyCharacters/SeedHooks 同规）；
4. 渲染提示词示例句去人物名。

验证：`seed_test` 重写（档案驱动断言 + 无地点/道具自动种入 + 缺档案 Fail-Closed
用例）；mock/fake 夹具同步注入 protagonist；全量 `go test ./... -race` 全绿。


### 第十三轮：/responses 输出配额修复（2026-10-13 凌晨 · 全自动成书实战第七障碍）

实战：第六轮在创世第一击被 muse 端点 503 打回；探针复现为
`429 Output token rate limit exceeded — Set max_output_tokens to avoid being
charged the model maximum`。

根因：responses 协议（reasoner 专用）请求体从未携带 `max_output_tokens`，
网关按模型最大输出预算预留配额，连续高频调用即触发输出 token 限流；
表现为 503/429 或长推演挂起（此前 glm-5.3 系列 300s 无输出的现象亦可能与此同源）。

修复：`chatRequest` 增加 `max_output_tokens`；responses 协议的同步与流式两条
构造路径统一注入 `responsesMaxOutputTokens = 8192`（覆盖 framework/intents/
beats 等最大 JSON 载荷并留余量）；chat 协议行为不变。

验证：新增 `TestResponsesRequestIncludesMaxOutputTokens`（断言请求体携带配额）；
全量 `go test ./... -race` 全绿。


### 第十四轮：分节生成有界重采样（2026-10-13 凌晨 · 全自动成书实战第八障碍）

实战：第七轮主角归位成功（程穗 ✓）、14 张意图卡就绪，但章节前置校验「分卷分节生成」
因一次坏样本提取失败而停线（`clean volume sections json failed`）；复现显示同一提示词
再次调用即返回完整合法 JSON —— 属单次采样质量问题，而非系统缺陷。

修复：`GenerateVolumeSections` 增加有界重采样（最多 3 次尝试）：
- 内容级失败（不可修复 JSON / 覆盖校验不过）→ 2s/4s 退避后重新生成；
- 传输级失败沿用客户端内建重试，仅可重试错误进入外层循环；
- 持久失败仍 Fail-Closed 停线；清理路径补 `extractJSON` 兜底。

验证：新增 `TestGenerateVolumeSections_RetriesBadSamples`（坏样本恢复 / 覆盖违规恢复 /
耗尽预算 Fail-Closed）全绿；全量 `go test ./... -race` 全绿。

### 第十五轮：五阶段创作框架缺口补齐（2026-10-13 凌晨 · 并行批次：2×agy + 2×opencode + 集成方）

对照工业级五阶段创作框架（概念/角色/结构/场景/重写）的评审结论，一波并行补齐五个工作包
（各自独立 worktree，集成方统一复验、rebase 与合并）：

1. **角色内在引擎**（`59091f`→main `0219d9a`）：`core_motivation` 彻底切换为
   `want/need/ghost` 三轴（Pre-v1.0 纯净切换，无兼容垫片）——CodexEntry/
   ProtagonistProfile/CharacterProfile 三结构、存储列（一次性搬运旧值后 DROP 旧列）、
   创世/Codex 提示词、chronicle 注入块、三处前端编辑面板；连续性审校可据此判定角色抉择。
2. **书级结构契约**（`08d418e`→main `a0f1336`）：`story_milestones`（7 类枚举 + 状态/逾期探针）、
   `logline`、`atmosphere_anchor`、`total_planned_chapters`；创世提示词与离线 mock 全量输出；
   节拍提示词注入「全书宏观看位」（命中章强指令）、渲染注入氛围锚点、framework PUT 校验
   （fail-closed）、FrameworkView 编辑区 + AnalyticsView 全书节拍看板。
3. **场景结果闭环**（`a9f9670`→main `e61950a`）：`Scene.dramatic_outcome`（目标→阻碍→结果
   因果链规则进拆场提示词）；逻辑庭新增 reader_emotion/info_gap 逐拍核对；工作台节拍编辑器
   补双输入；矩阵卡与场景弹窗展示结果字段。
4. **章节版本历史**（`2d3887b`→main `100d7cc`）：`GetChapterVersion` 租户化签名
   （id+project_id，跨作品 ID 视为不存在）；versions 列表/详情 GET 端点（窄接口断言，
   不动 store.Store 大接口）；行级 LCS diff 弹窗（无新依赖）+「载入到工作台」重修入口。
5. **母题锚点**（`dcb9e91`→main `d272366`）：`ProjectFramework.motifs`（清洗/去重/上限 8
   fail-closed）、渲染注入「有意母题（RC 精修锚点）」块、FrameworkView 每行一条编辑器。

验证（集成方独立执行，非执行器自述）：五分支各自 build/vet/gofmt/full -race/web test+build
全绿；集成后 main 全量 `-race` 全绿 + `pnpm test && pnpm build` 通过；变异均亲手重放
（sqlite want 列、bench 逾期判定、dramatic_outcome tag、project_id 过滤各自按预期变红后还原）；
`core_motivation` 全库仅剩 migrate 一次性搬运/删除语句。


### 运营备注：低成本模型阵容（2026-10-13 凌晨 · 用户要求降本）

- 推理（responses 协议）：`muse-spark-1.3-contributor`（唯一可用 responses 模型，保持）；
- 写作（chat 协议）：`kimi-k3` → **`qwen3.8-flash`**（flash 档；写作实测 9s/229 字，
  短句白描、无 AI 套词、有钩子收尾，质量达标）；
- 审校（chat 协议）：`glm-5.3`（思考失控）→ `qwen3.8-flash` → **`mimo-v2.6-flash`**
 （横评最快 ~62s/章、评审意见扎实；与写作模型保持异构）。

注：纯 `~/.novel-studio/config.json` 运营配置变更，未动代码；桌面端下次启动自动生效。


### 第十五轮：渲染元素纪律护栏（2026-10-13 凌晨 · 实战：写作侧即兴剧透）

实战：第 1 章三轮返工 64→58→62→64 未收敛。审计拆解显示核心拖累之一是结构性的：
写作侧在初稿中即兴引入了计划后续章节才揭示的关键元素（值班表复印件提前出场、
第七栏「程穗」清晰可见），直接撞上连续性庭的钩子时序审查；返工只能做文字修补，
无法回 retract 已发生的剧透，分数注定震荡。

修复：渲染用户提示词新增【元素纪律】块——只允许本章节拍/已开放伏笔/codex
已登记元素；严禁后续章节人物提前登场、伏笔提前揭示；不确定的关键元素宁可
不写。纯提示词文本变更，零逻辑改动。

验证：`go test ./internal/engine/ -race` 全绿。


### 第十六轮：状态结算有界重采样（2026-10-13 凌晨 · 实战：终结算坏样本停线）

实战：第 1 章三轮返工后终结算时，muse 返回不可提取 JSON（`settle clean json
failed`），单次坏样本直接技术阻断整章；结算路径为单次尝试、无重试、无兜底。

修复：`SettleFromDraftWithFeedback` 增加有界重采样（最多 3 次尝试）：
LLM 调用失败（仅可重试错误向外冒泡即停）、提取失败、解析失败、证据校验失败
均触发 2s/4s 退避后重新生成；持久失败仍 Fail-Closed；token 用量跨尝试累加。
不变量预检保持单次（确定性检查，重试无意义）。

验证：新增 `TestSettler_RetriesBadSampleThenSucceeds` /
`TestSettler_FailsClosedAfterExhaustingAttempts`；既有 5 个结算用例（含 3 个
拒绝语义断言，错误文本经 `%w` 透传保持匹配）全绿；全量 `go test ./... -race` 全绿。

### 后续待办

- （无）——已知互斥缺口全部闭环。

### 第十六轮：Wave 2 剩余缺口补齐（2026-10-13 · 并行批次：2×agy + 1×opencode + 集成方）

首轮五包之后的第二波（B2/C1/C3），各自独立 worktree，集成方统一复验、亲手变异重放、
rebase 线性合并：

1. **B2 角色一致性审校**（`f7e76a1`→main `786b534`）：NEW
   `engine.BuildCharacterMotivationContext`（MentionScanner 筛出场角色，渲染
   Want/Need/Ghost 卡，cap 6 条/2000 rune）；`ChiefJudgeOptions.CharacterContext`
   透传至 `ContinuityDivision`（签名+1 参）；连续性庭 prompt 加第 4 项「角色动机一致性」
   （反派自我正当逻辑、逐判定引用正文 ≥8 字证据），引文规范顺延第 5 项；
   `AuditWithHooks` 加 variadic opts；工作坊首审/复审 + review-draft 三处接线
   （读库失败降级空串，生产零新增阻断；同时激活了此前从未被传入的 TailAnchor）。
2. **C1 全书结构审读器**（`a7a6a94`→main `f73a0d0`）：NEW `domain.StructureFinding`
   （5 类/3 级/3 态 + Validate）→ `engine.RunStructureAudit`（章摘要 300/12000 rune
   上限、伏笔/里程碑/bench 组装、宽容 JSON 解析、非法项丢弃、cap 20）→
   `store.structure_findings` 表（窄接口断言，不动 Store 大接口）→
   `POST/GET structure-audit` + `POST status`（run 成功才删旧存新，LLM 失败旧结果保留；
   无摘要返回空列表 + note 不报错）→ AnalyticsView 第 4 子 tab「结构审读」
   （运行/列表/解决·忽略·重开，作品切换清空，覆盖语义文案）。
   集成方追加 `2072be5`：status 端点按 `StructureFindingProjectID` 做租户作用域
   （跨作品/不存在一律 404，对齐伏笔 PlotHookProjectID 定式）。
3. **C3 提示词预览同源**（`465befe`→main，经集成方重构 `8b3a37b`）：预览加母题/
   里程碑块。执行器原手写第二份里程碑定位逻辑，复验发现边界分歧（空白 status、
   未 trim 人名）且"逐字一致"回执不成立；集成方改为导出
   `engine.BuildMotifConceptPrompt` / `engine.BuildMilestoneContextBlock` 由预览直接嵌入，
   同源从自称变结构保证，并加固守测试（预览改回手写即红）。

验证（集成方独立执行）：三包各自 build/full -race/gofmt/web test+build 全绿；
main 最终 `go test ./... -race` 全绿 + `pnpm test` 4/4；变异亲手重放
（structure_audit severity 门→domain 非法 severity case 红；
 character_motivation ScanText 短路→排除未出场/空输入两 case 红；
 preview 引擎调用短路→生产表头断言红），均还原确认绿。
教训：`herdr agent prompt --wait` 会在 pane 短暂 settle 时提前返回 done——
"done" 以 pane 真 Idle + 树里有提交 + DONE token 三者为准，等待改用被动
`herdr agent wait`，不再向工作中 pane 注消息。

### 第十七轮：Wave 3 审校闭环与回放装配（2026-10-13 · 并行批次：2×agy + 1×opencode + 集成方）

Wave 1 与 Wave 2 之后的第三波闭环升级（P1/P2/P3），各自独立 worktree，集成方统一复验、变异重放、rebase 线性合并与嵌入构建：

1. **P1 审读闭环：OPEN 高优 findings 注入节拍规划提示词**（`0515f52`→rebase main `644fa2d`）：
   - `internal/engine/orchestrator_beats.go`：节拍规划 user prompt 在里程碑块后增加【全书结构审读·待处理高优问题】段（`BuildStructureFindingsPromptBlock`），逐条格式化 `第{start}-{end}章·{type}·{summary}·建议：{suggestion}`，空切片省略整段；
   - `internal/engine/workshop.go`：`ProduceChapter` 内以窄接口断言（`interface{ ListStructureFindings(ctx, projectID string) ([]*domain.StructureFinding, error) }`）best-effort 读库，过滤 `Status==OPEN && Severity==HIGH`，按 chapter_start 排序取前 5 条；读库/断言失败 Fail-Open 降级为空继续，规划增强不阻断生产；
   - `internal/engine/chronicle.go`：`CanonHorizon` 扩展 `StructureFindings []domain.StructureFinding`（`omitempty`）；
   - 测试与变异：`orchestrator_test.go` 与 `workshop_test.go` 覆盖 4 个边界用例（注入与上限 5、非 open/high 省略、空 findings 省略、读库错误优雅降级）；亲手变异 `BuildStructureFindingsPromptBlock` 短路为空串 → 守卫用例红（`workshop_test.go` + `orchestrator_test.go:805`），还原确认绿。
2. **P2 提示词预览拆分：Render / Beats 双装配**（`e0918e7` + 修正 `7304fcf` + 对齐 `7f2d3f8` + dist `c9aa4b2`）：
   - `internal/server/handlers_workshop_prompt.go` + `PromptInspectorModal.vue`：提示词检查弹窗重构拆分为渲染路（Render）与节拍路（Beats）双装配视图；
   - 彻底贯彻 C3 经验：预览侧直接调用生产端构造器（`BuildElementDisciplineBlock`、`BuildMotifConceptPrompt`、`BuildMilestoneContextBlock`、`buildPlatformStylePrompt`），杜绝手工镜像提示词文本带来的漂移；
   - 支持逐路由世界观标题与主角条目逐字镜像；构建并提交前端 `web/dist`。
3. **P3 审庭回放：工作台展示分庭判决与少数派 dissent**（`d0986b6` + dist `57c228e`）：
   - `web/src/components/workbench/WorkbenchStepsPanel.vue`：工作台审计报告区增加分庭判决回放段，展示逻辑/风格/连续性三分庭各自 verdict，并支持展开少数派 dissent 原文；老报告无分庭字段时平滑隐藏；
   - 数据链贯通：`internal/domain/models.go` + `internal/engine/review_court.go` 统一定义 `DivisionName`/`DivisionResult`，`AuditReport` 与 `ReviewResult` 补充 `Dissent` 与 `Divisions`（`omitempty`）；
   - 单测覆盖：`internal/server/handlers_reviewgate_test.go` 增补分庭及 dissent 序列化与端点断言；亲手变异丢弃分庭数据触发单测红后还原；构建并提交前端 `web/dist`。

验证（集成方独立执行）：
- 三包各自独立 worktree 内 build、单测、gofmt、变异重放全绿；
- 合并至 main 后全量 `go test ./... -race` 全绿（含 engine 93s 深度并发套件）；
- `gofmt -l .` 无输出；
- 前端 `cd web && pnpm test` 4/4 通过，`pnpm build` 顺利产出并提交至 `web/dist`；
- 独立 worktree（`novel-studio-courtplayback`, `novel-studio-findingsloop`）与临时分支全部清理完毕。

### 第十八轮：Wave 4 作品删除闭环与全模块 UX 深度提升（2026-10-13）

响应 `/vibe-refine` 要求，打通全流程作品删除生命周期（Fail-Closed 并发守护、10 个无外键表深度级联擦除、前端瞬态重置与平滑切换），并系统性提升全模块用户交互体验（搜索筛选、空态行动点、耗时计时器、字数达标指示与自动保存反馈）。

#### 1. 后端存储级联清理与并发互斥守护 (P0)
- **`internal/store/sqlite_store_project.go`**：`DeleteProject` 显式清除无外键 `ON DELETE CASCADE` 约束的 10 个附随表，防止产生孤儿数据：
  - `chapter_quality_metrics`、`chapter_versions`、`chapter_state_snapshots`、`compaction_cache`、`project_automation`、`memory_fts`、`scenes`、`book_queue`、`studio_messages`、`studio_threads`；
  - 接着执行 `DELETE FROM projects WHERE id = ?` 触发引擎自带的外键级联，同时在 `project_events` 留存账本墓碑审计事件。
  - 单测 `TestDeleteProject_OrphanCleanup`（`sqlite_store_test.go`）：向各孤儿表注入数据后删除作品，断言所有表记录彻底归零。
- **`internal/server/handlers_project.go`**：`DELETE /api/projects/:id` 接入 `rejectIfProducing`：
  - 后台处于批产或排程续产锁定状态时，直接 409 Conflict Fail-Closed 阻断删除；
  - 单测：在 `coord_wiring_test.go` 中加入 `TestCanonWrites_RejectedWhileProductionActive` 的 `delete project` 用例，并新增 `TestDeleteProject_EndToEnd` 白盒端点测试。

#### 2. 前端作品删除动作与状态闭环 (P0)
- **`web/src/stores/domain/projectActions.js`**：新增 `deleteProject(projectId)` 动作：
  - **忙碌守卫**：检查 `activeGenerationTask(state)`，若当前作品有成章或审读任务在途，拒绝删除并提示等待；
  - **危险弹窗**：通过 `dialogs.confirm({ type: 'danger' })` 弹出红色高危确认对话框，详述数据永久擦除后果；
  - **平滑切换**：从 `state.projects` 中剔除；若删除的是当前激活作品，调用 `resetProjectScopedState` 彻底重置瞬态草稿/审校/断点，自动平滑切换至下一部作品；若全删则整洁置空；
- **`web/test/project-switch.test.mjs`**：新增 3 项回归测试：
  - `删除作品：生成任务进行中拒绝删除（Fail-Closed）`
  - `删除作品：确认后删除成功，从列表移除并平滑切换至下一部作品`
  - `删除作品：删除最后一部作品后状态置空`
  - 测试套件扩展至 7/7，全绿通过。

#### 3. 全模块 UX 深度提升 (P1 / P2)
- **多作品总览 `StudioLibraryView.vue`**：
  - 增加书名实时搜索输入框与题材/平台下拉过滤选择器；
  - 表格操作列新增删除按钮（`Trash2` 红色悬浮高亮、tooltip、危险二次确认与列表自动重载）；
  - 修复空列表与搜索无结果时的友好清空重置体验。
- **作品架构设定 `FrameworkView.vue`**：
  - 视图底部新增高亮红框「危险区域 (Danger Zone)」，清晰提示彻底删除后果并提供作品级删除按钮。
- **全局侧边栏 `AppSidebar.vue`**：
  - 头部作品下拉框旁增设快速删除当前作品按钮，联动全局切换与提示。
- **文学创作画布 `WorkbenchCanvasPanel.vue`**：
  - 手稿顶栏增加章节目标字数达标百分比胶囊（达标前琥珀色，达标后翡翠绿）；
  - 右侧状态区增加自动保存微型胶囊（「正在保存...」动画 /「已保存」绿色状态指示及 Cmd+S 存盘快捷键提示）。
- **停线待办中心 `StudioTodoView.vue`**：
  - 全空态时增加专业引导插画文案与双操作按钮（「前往工作台创作」/「查看自动化排程」）。
- **全自动流水线控制台 `PipelineView.vue`**：
  - 顶部运行态增加耗时计时器（`formatElapsedTime` 实时更新 `分:秒`，结束与异常时自动停止）；
  - 时间轴日志头部增加「自动滚屏: 开/关」切换开关，方便滚动排查历史事件。

#### 4. 验证结果
- `go test ./internal/server/... ./internal/store/... -race -count=1` 全绿通过；
- `cd web && pnpm test` 7/7 测试全部通过；
- `pnpm build` 纯净构建通过并更新 `web/dist`；
- `go build ./...` 验证单二进制内嵌 web 打包完全正常。

---

### Wave 5: 全局菜单与作品级菜单完全隔离与页面区分 (Round 19)

#### 1. 作用域单一事实源与元数据设计 (`web/src/stores/domain/projectScope.js`)
- **全局域 (Global Scope / 7 模块)**：
  - `studio-library`: 多作品总览（全库进度、健康度、伏笔债务全景）
  - `studio-ideas`: 灵感 Inbox（轻量随笔、故事脑洞记录与转创世推演）
  - `studio-chat`: 全局对话（跨项目闲聊与宏观设定头脑风暴）
  - `pipeline`: 全自动流水线（创世到批量成章自主推演流水线）
  - `studio-todos`: 停线待办（全自动流水线中需人工介入的质量卡点）
  - `studio-automation`: 自动化排程（全书定时续产与无人值守排程）
  - `config`: AI 系统设置（大语言模型接入、API 密钥与模型路由）
- **作品域 (Project Scope / 13 模块，三部曲生命周期)**：
  - 准备构思：`framework`（作品设定）、`codex`（设定集）、`matrix`（全书大纲）、`intents`（章节意图）
  - 动笔写作：`workbench`（写正文）、`statemachine`（人物状态）、`hooks`（伏笔簿）、`forecast`（剧情树）
  - 成果阅览：`chapters`（章节目录）、`activity`（动态时间线）、`graph`（关系图谱）、`analytics`（全书体检）、`metrics`（质量指标）
- **作用域工具函数与全量元数据字典**：
  - `isGlobalTab(tabId)`、`isProjectTab(tabId)`、`getTabScope(tabId)`；
  - `TAB_META` 字典集中统一全系统 20 个模块的名称、描述、作用域与分组，杜绝零散硬编码。

#### 2. 状态机跨域导航与记忆管理 (`web/src/stores/appState.js` & `projectActions.js`)
- **双向断点记忆**：
  - `state.lastGlobalTab`（默认 `studio-library`）与 `state.lastProjectTab`（默认 `workbench`）各自独立追踪用户在两域中的最近停留点；
  - `actions.switchToGlobal(tab?)` 与 `actions.switchToProject(pid?, tab?)` 互通时自动恢复对应域的上一次工作页面；
- **Fail-Closed 保护与平滑回退**：
  - 作品库为空时，调用 `switchToProject` 严格拒绝进入作品模式，引导弹出新建作品对话框；
  - 当作品在 `deleteProject` 中被全量删除时，系统自动切回全局 `studio-library`，防止用户卡在空白作品页面。

#### 3. 侧边栏物理完全隔离 (`web/src/components/layout/AppSidebar.vue`)
- **全局 Studio 模式 (`isGlobalMode === true`)**：
  - 顶栏展示墨 Logo 与 `STUDIO 全局中枢` 标识，附带金黄 `STUDIO` 徽章；
  - 呈现「活跃作品通道」卡片：显示当前聚焦作品书名、平台，支持快速切换作品，并提供醒目 `进入创作空间 →` 高亮快捷按钮；
  - 导航区严格只渲染 Studio 6 模块 + AI 系统配置，**绝对不混入任何作品级菜单**；
  - 底栏显示全库多作品架构状态与离线安全运行标识（隐藏单作品导出备份按钮）。
- **作品创作模式 (`isGlobalMode === false`)**：
  - 顶栏置顶提供醒目精致的 `← 返回 Studio 全局` 胶囊返回按钮，一键切回多作品总览；
  - 展示单作品切换下拉框、新建作品、以及危险操作二次确认彻底删除当前作品按钮；
  - 导航区严格只渲染作品三部曲工序（准备构思 4 模块、动笔写作 4 模块、成果阅览 5 模块），**绝对不混入任何 Studio 全局菜单**；
  - 底栏专属提供针对当前作品的「导出 JSON 备份」和「导出 Markdown 全书」下载按钮。

#### 4. 顶部 Header 与页面容器隔离 (`AppHeader.vue` & `App.vue`)
- **AppHeader 区分**：
  - 全局模式：展示 `STUDIO 全局` 状态微章、当前功能名称与描述副标；右侧提供直达 `创作《书名》 →` 快捷按钮，**完全隐藏写正文视界切换、专注模式、角色对戏与单作品一键成章**；
  - 作品模式：展示 `作品创作` 空间徽标、当前作品标题、平台、分卷名及当前工序名称；右侧完整保留正文工作台工具链、角色对戏与单作品一键成章。
- **App.vue 容器层零状态兜底**：
  - 当用户处于作品空间但作品库暂无作品或未选中时，统一呈现优雅的引导卡片（提供「返回多作品总览」与「新建小说作品」双入口），彻底杜绝白屏。

#### 5. 专项测试覆盖与全链路验证
- **`web/test/nav-scope-isolation.test.mjs`**：新增 4 项专项回归测试：
  - 作用域严格互斥且全覆盖（20 个 Tab 严格归属，元数据齐备）；
  - `computedState.navScope / isGlobalScope / isProjectScope` 响应式正确性；
  - 双域记忆跳转与断点恢复；
  - 作品库为空时 Fail-Closed 拒绝进入并保持在全局模式。
- **验证结果**：
  - `pnpm --dir web test` 11/11 测试全绿通过；
  - `pnpm --dir web build` 生产构建成功；
  - `go test ./internal/server/... ./internal/store/... -race -count=1` 全部通过。





