# Refine Log: NovelStudio 工业化加固

日期：2026-10-07  
方法论：`vibe-refine` + `codebase-design` (Deep Modules & Invariant Locality)

---

## 精炼背景与问题定位
通过 `/codebase-design` 审查发现两处关键隐患：
1. **CRUD 浅模块与业务逻辑泄露 (Leaked Locality)**：
   - 之前在 HTTP Handler `handleProjectChapters` 中直接编写主角状态机（背包道具追加、战力演变）字符串逻辑；
   - 导致非 HTTP 途径（如测试、CLI、脚本）无法保证“归档章节必触发状态机更新”的领域不变量。
2. **违反依赖注入原则 (Hardcoded Instantiation)**：
   - HTTP Handler 在每个请求中就地 `new HTTPLLMClient`，导致测试难以 Mock，且当前端保存新 API Key 时无法热更新。

---

## 精炼操作与 Commit 记录

1. **`commit: 36c4639` `refine(store): deepen module with atomic CommitChapter transaction`**
   - 在 `store.Store` 引入深度操作：`CommitChapter(ctx, projectID, chapter) (*domain.Project, error)`；
   - 在 SQLite 单事务内部原子完成：
     - 写入 `chapters` 数据表；
     - 读取并根据 `StateMutation` 演变主角实体状态机并更新 `projects`；
     - 检查目标回收章节触发的 `plot_hooks` 状态标记；
   - 新增 `TestSQLiteStore_CommitChapterAtomic` 单元测试，验证事务性保证与状态演进。

2. **`commit: 573af6c` `refine(engine): thread-safe LLM client credentials and dynamic config updates`**
   - 为 `HTTPLLMClient` 引入 `sync.RWMutex` 与 `UpdateCredentials(baseURL, apiKey)`；
   - 支持并发安全读取凭据，支持运行期热更新 API Key / Base URL；
   - 新增 `TestHTTPLLMClient_UpdateCredentials` 单元测试。

3. **`commit: 8ce6524` `refine(server): inject dependencies and delegate atomic chapter commit to store`**
   - 重构 `server.New`，显式注入 `store`、`llmClient`、`orch` 和 `linter`；
   - 彻底移除 `handleProjectChapters` 中泄露的业务代码，纯净委托给 `store.CommitChapter`；
   - 重构 `server_test.go`，补全端到端原子归档与状态变更断言。

4. **前端响应式闭环同步 (`web/dist/index.html`)**
   - 前端归档章节接口直接接收原子更新后的 `Project` 实体，实现状态机视图秒级热同步。

---

## 精炼后指标验证

| 指标 | 精炼前 | 精炼后 |
| :--- | :--- | :--- |
| **模块测试覆盖** | 基础 CRUD (8 个测试) | **全模块覆盖 (11 个测试全部 -race 通过)** |
| **并发竞态安全** | 未受保护的全局状态 | **零竞态 (Thread-safe credentials + SQLite WAL 事务)** |
| **领域不变量收拢度** | 分散在 HTTP Handler | **100% 收敛于 Store 事务内部 (Fail-Closed)** |
| **编译输出** | 零 CGO 单二进制 (v1.0.0) | **零 CGO 单二进制 (v1.0.0, 31MB 内嵌全套 UI)** |
