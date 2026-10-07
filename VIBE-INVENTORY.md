# Vibe Refine — Module Inventory: NovelStudio

项目：NovelStudio (`github.com/Jungley8/novel-studio`)  
目标：工业化 AI 网络小说创作桌面工作台 (Go + Embedded Web + SQLite)  
日期：2026-10-07  

## 架构规划与模块清单

| # | 模块路径 | 职责定位 | 测试设计 | 优先级 |
|---|---------|---------|---------|--------|
| 1 | `internal/domain` | 实体领域模型与参数合法性校验 (Project, Protagonist, Beats, Hook, Chapter) | 单元测试 (Validation & Serialization) | P0 |
| 2 | `internal/store` | 纯 Go SQLite 持久化驱动 (零 CGO, 事务性状态更新与时光机回滚) | 内存/文件 SQLite 集成测试 | P0 |
| 3 | `internal/engine` | 双模型调度编排 (Reasoning+Writer) + 抗 AI 检测 Linter (突发度方差) | 单元测试 (Linter 指标算法 + Mock LLM) | P0 |
| 4 | `internal/config` | 系统与模型端点配置持久化 (`~/.novel-studio/config.json`) | 单元测试 (读写与默认值 Fallback) | P1 |
| 5 | `internal/server` | REST API 路由与错误上下文处理 (Fail-Closed, CORS, SPA 映射) | HTTP 接口端到端测试 | P0 |
| 6 | `internal/desktop` | 桌面应用生命周期管理 (跨平台浏览器唤起、信号优雅停机) | 单元测试 | P1 |
| 7 | `web` | 嵌入式桌面前端 (`//go:embed all:dist`) 与交互面板 | 静态构建集成与渲染验证 | P0 |
| 8 | `cmd/novel-studio` | 命令行入口与启动参数绑定 | CLI 启动测试 | P1 |

## 精炼执行原则
1. **纯净标准库优先**：SQLite 使用 `modernc.org/sqlite`，避免 CGO 编译依赖，支持跨平台单二进制发布。
2. **严格 Fail-Closed**：非法输入与 API 异常携带完整上下文，拒绝静默吞没错误。
3. **三轨持久化保真**：状态机、章节草稿与因果伏笔写入 SQLite 本地事务。

## 精炼总进度 (Refinement Status)

- 🟢 已完成深度精炼: 8/8 模块 (100%)
- 状态机不变量: 事务级闭环 (`store.CommitChapter`)
- 依赖注入: 全链路解耦 (`server.New` 依赖注入)
- 测试验证: 11 个测试用例全部通过，带 `-race` 竞态检测
- 分发就绪: 零 CGO 单二进制构建验证完成 (`novel-studio v1.0.0`)

