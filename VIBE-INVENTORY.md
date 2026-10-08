# Vibe Refine — Module Inventory: NovelStudio

项目：NovelStudio (`github.com/Jungley8/novel-studio`)  
目标：工业化 AI 网络小说创作桌面工作台 (Go + Embedded Web + SQLite)  
日期：2026-10-07  

## 架构规划与模块清单

| # | 模块路径 | 职责定位 | 深度设计 / Seam | 状态 |
|---|---------|---------|----------------|------|
| 1 | `internal/domain` | 实体领域模型 + 实体状态机账本 (`EntityLedger`) | 结构化物品与境界状态转移，前置不变式校验 | 🟢 已深度精炼 |
| 2 | `internal/store` | 纯 Go SQLite 持久化驱动 (零 CGO, 事务性状态更新) | `CommitChapter` 事务原子结算，时光机回滚 | 🟢 已深度精炼 |
| 3 | `internal/engine/quality_gate` | 启发式算法 + 大模型语义统一审校 (`QualityGate`) | 0ms 突发度/套词预检 + 苛刻主审 + 自动驳回门禁 | 🟢 已深度精炼 |
| 4 | `internal/engine/chronicle` | 滚动正史视界组装器 (`CanonChronicle`) | 3 章密封正史视界 + 临期伏笔动态加权组装 | 🟢 已深度精炼 |
| 5 | `internal/engine/workshop` | 全流程章节生产工坊深度模块 (`ChapterWorkshop`) | 单接口驱动节拍推演、渲染、多轮复审返工与原子归档 | 🟢 已深度精炼 |
| 6 | `internal/config` | 系统与模型端点配置持久化 (`~/.novel-studio/config.json`) | 并发安全读取与默认值回落 | 🟢 已深度精炼 |
| 7 | `internal/server` | REST API 路由与错误上下文处理 (Fail-Closed, CORS) | 依赖注入驱动，暴露自主流水线端点 `/workshop/produce` | 🟢 已深度精炼 |
| 8 | `internal/service` | macOS 原生 `launchd` 守护服务 + `doctor` 自检套件 | 开机自启、健康探针、环境自检自愈 | 🟢 已深度精炼 |
| 9 | `cmd/novel-studio` | CLI 单二进制入口与无头批跑指令 (`produce` 子命令) | 支持终端无头全流程自主产章与服务管理 | 🟢 已深度精炼 |
| 10 | `web` | 嵌入式桌面上手稿协作与交互看板 (`//go:embed all:dist`) | 矩阵大纲看板、全域百科管理、态势分析图表、划词伴写与提示词透视 | 🟢 已深度精炼 |
| 11 | `internal/domain/codex.go` | The Codex 全域世界观百科领域模型 | 分类实体、别名网状索引、阶段演进时间线快照、两两因果拓扑关系 | 🟢 已深度精炼 |
| 12 | `internal/domain/matrix.go` | The Matrix 矩阵大纲与原子叙事场次模型 | 卷/幕/章/场 4 层戏剧树、目标与冲突阻碍、1-10 张力阶梯、场次批注 | 🟢 已深度精炼 |
| 13 | `internal/engine/mention_scanner.go` | 实体别名与主名称智能引用扫描器 | 高性能文本扫描、字位定位、自动匹配与强制注入识别 | 🟢 已深度精炼 |
| 14 | `internal/engine/harmonizer.go` | 国内平台合规脱敏与反 AIGC 对抗扰动 (Harmonizer) | 敏感涉暴词平滑替换 + 词汇似然度扰动 + 4维人味杂质建议 | 🟢 已深度精炼 |

## 精炼执行原则
1. **纯净标准库优先**：SQLite 使用 `modernc.org/sqlite`，避免 CGO 编译依赖，支持跨平台单二进制发布。
2. **严格 Fail-Closed**：非法输入与 API 异常携带完整上下文，拒绝静默吞没错误。
3. **三权分立与深度模块**：收拢多步 HTTP 胶水到 Go 后端 ChapterWorkshop，前置算法与语义门禁，保证实体状态机不变量。
4. **透明化与创作者优先 (Novelcrafter 对标)**：全面落地 The Codex 全域百科、The Matrix 矩阵大纲、Prompt Inspector 提示词透视与划词伴写，实现可视化与工业级物理状态机深度统一。

## 精炼总进度 (Refinement Status)

- 🟢 已完成深度精炼: 14/14 模块 (100%)
- 国内合规与去AI对抗体系: `Harmonizer` 敏感涉暴词平滑和谐 + 似然度扰动 + 4 维人味杂质建议 + 剥夺副词/状语元提示词 + 纯净 Vue 3 桌面弹窗集成
- 宏观创世系统 (Phase 0): `ProjectFramework` 创世总纲、天道公理物理门禁、10级严谨战力天梯与代价天平、分卷大纲任务链 (`VolumeArcs`)、四大势力暗线谱系与开篇伏笔池自动播种
- 状态机不变量: `EntityLedger` 数学化数量解析与转移，严禁消耗未拥有道具，彻底根除长篇连载道具数量清零
- 上下文连续性: `CanonChronicle` 3 章密封滚动视界 + 前章 200 字正史尾段文风锚定 (`TailAnchor`) + 分卷宏观使命锚定 (`CurrentVolume`) + 跨卷伏笔历史现场场景段落高保真还原 (`extractHookSceneExcerpt`)
- 全域百科 (The Codex): 4 大基石分类 (`CHARACTER`/`LOCATION`/`LORE`/`ITEM`)、别名索引、`ActiveProgression` 历史各章阶段演进解析、两两关系图谱，以及 Layer 4 动态视界精准注入
- 矩阵大纲 (The Matrix): Volume ➔ Chapter ➔ Scene 4 层树状结构，`DramaticGoal`、`ConflictBarrier`、`TensionLevel` (1-10)、场次批注 (`SceneMarker`) 与多维表格统计
- 提示词透视 (Prompt Inspector): 彻底打破黑盒，将发往大模型的提示词解构为 4 大积木组件并输出 Token 预算与复制功能
- 划词伴写 (Manuscript Inline AI): 快速执行润色 (`rewrite`)、扩写 (`expand`)、精简 (`shorten`)、五感增强 (`sensory`)、台词打磨 (`dialogue`)，提供差分对比与一键替换
- 情境工坊对话 (Workshop Chat): 🎭 角色扮演 (动态绑定阶段设定与人际关系，严格不崩人设)、💡 剧情头脑风暴、🧐 严苛总编评审
- 全书态势监控 (Analytics): 全域实体出场频次热力图 (`heatmap`) 与场景张力阶梯心流分布曲线 (`tension`)
- 多模型真实路由: `LLMRouter` Context Role 映射（推理/写作/审校）真实解耦降偏，彻底废除空壳门面
- 真实账单核算: 流水线各环节全量打通 `ChatCompletionWithUsage`，消除常数硬编码 Token 假数据
- 并发防泄漏: SSE 流式响应与 Goroutine 全量集成 Context 取消选择器，杜绝 TCP/Goroutine 句柄泄漏
- 裁决安全: `QualityGate` 严格遵循 Fail-Closed，审校模型拒止判定具备一票否决权，严禁高分篡改放行
- 健壮断点恢复: `ChapterWorkshop` 支持显式跳过或清理旧断点防死锁，返工轮次连续计数防穿透
- 算法质检纵深: `QualityGate` 突发度 + 60+ 分层套词 + 对话占比 + 段落方差 + 重复短语 + 感叹号密度
- 读者心理模型: `SceneBeat` 扩展读者情绪期望 (`ReaderEmotion`)、信息差 (`InfoGap`)、章末钩子 (`HookType`)
- 深度文学 Prompt: `RenderScene` 声口个性约束、五感权重分层、张力曲线节奏指令、目标平台定制 (番茄/起点/知乎)
- 差分返工保护: `RewriteDraft` 严格限制精修幅度 ≤35%，原样保留未受异议的精彩描写
- 全自主流水线: `ChapterWorkshop` 支持最大 3 轮自主定向返工闭环与断点自愈
- 测试验证: 30+ 个测试用例全部通过，带 `-race` 竞态检测与 0 泄漏
- 分发就绪: 零 CGO 单二进制构建验证完成 (`novel-studio v2.0.0`)，支持 CLI `genesis` / `produce` 与 Web 全功能操控

