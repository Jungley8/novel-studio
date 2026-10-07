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
| 10 | `web` | 嵌入式桌面前端 (`//go:embed all:dist`) 与交互看板 | 支持一键全流程自主生产 (Auto-Pipeline) 与单步工序 | 🟢 已深度精炼 |

## 精炼执行原则
1. **纯净标准库优先**：SQLite 使用 `modernc.org/sqlite`，避免 CGO 编译依赖，支持跨平台单二进制发布。
2. **严格 Fail-Closed**：非法输入与 API 异常携带完整上下文，拒绝静默吞没错误。
3. **三权分立与深度模块**：收拢多步 HTTP 胶水到 Go 后端 ChapterWorkshop，前置算法与语义门禁，保证实体状态机不变量。

## 精炼总进度 (Refinement Status)

- 🟢 已完成深度精炼: 10/10 模块 (100%)
- 状态机不变量: `EntityLedger` 结构化转移，严禁消耗未拥有道具
- 上下文连续性: `CanonChronicle` 3 章密封滚动视界
- 审校门禁: `QualityGate` 算法质检与主审合流，套词自动扣分驳回
- 全自主流水线: `ChapterWorkshop` 支持最大 3 轮自主定向返工闭环
- 测试验证: 14 个测试用例全部通过，带 `-race` 竞态检测
- 分发就绪: 零 CGO 单二进制构建验证完成 (`novel-studio v1.0.0`)
