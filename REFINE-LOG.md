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

