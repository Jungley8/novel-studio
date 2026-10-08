# Vibe Refine — Module Inventory

项目：novel-studio
扫描日期：2026-10-08
总行数：~17,400 (Go: 12,387 行, Vue/JS: ~5,000 行)
当前整体测试覆盖：Go ~65%, Web 0%

## 模块清单与技术债分诊

| # | 模块路径 | 行数 | 测试覆盖 | 热度 | 症状数 | 优先级 | 主要症状 / 风险 |
|---|---------|------|---------|------|--------|--------|---------------|
| 1 | `internal/server` | 1931 | 34.5% ❌ | 🔥🔥🔥 | 4 | **P0** | 1930行超大 God File、手动 `strings.Split` 脆弱路由、配置无锁写、吞错误 |
| 2 | `internal/engine/llm_client.go` | 895 | 77.0% 🟡 | 🔥🔥🔥 | 3 | **P0** | 120s 单一硬超时、缺少指数退避重试（DeepSeek R1易超时断开）、长流熔断缺少 |
| 3 | `web/src/stores/appState.js` | 468 | 0.0% ❌ | 🔥🔥🔥 | 3 | **P1** | 前端单一 God Store、跨组件事件契约缺失（此前按钮静默失效）、无 TS 校验 |
| 4 | `internal/store/sqlite_store.go` | 1342 | 83.1% ✅ | 🔥🔥 | 2 | **P1** | 缺少增量版本迁移（Schema Version）、多实体级联变更大事务长耗时 |
| 5 | `internal/engine/workshop.go` | 401 | 77.0% ✅ | 🔥🔥 | 1 | **P1** | 缺少分支推演（Branching / Tree of Thoughts）机制 |
| 6 | `internal/engine/chronicle.go` | 460 | 77.0% ✅ | 🔥 | 2 | **P2** | 超长篇连载（>100章）伏笔全表扫描、长正文历史缺少层级摘要压缩 |
| 7 | `internal/desktop` | 112 | 10.0% 🟡 | 🔥 | 1 | **P2** | 桌面环境与 CLI 路径自愈（已加固 `EnsureCLIInPATH`） |

## 总进度

- 🟢 已精炼: 7/7 模块 
  1. `internal/desktop`: CLI 自愈修复（彻底根除 `too many levels of symbolic links` 自引用软链），支持在桌面启动时自动接入 `~/.local/bin/novel-studio`。
  2. `cmd/novel-studio`: 移除 `http.Server` 全局 120s `WriteTimeout` 限制，支持长周期 SSE 流；新增 `health` / `doctor` 诊断子命令路由。
  3. `internal/server`: 彻底切分超大文件，新增 `handlers_config.go`，支持 `POST /api/config/test` 真实模型端点测速与诊断；SSE 流注入 15s 心跳保持机制。
  4. `internal/store`: 启用 `PRAGMA busy_timeout = 5000` 与纯 Go 单写多读并发排队连接池。
  5. `internal/engine/llm_client.go`: 超时上限提至 300s，增加 3 次指数退避重试 (429/5xx)。
  6. `internal/engine/chronicle.go`: 伏笔注意力权重与上下文预算控制。
  7. `web/src/stores`: 领域动作拆治，流水线注入 `resume_checkpoint: true` 断点优雅恢复，设置页提供一键「测试模型连通性」与延迟反馈。

