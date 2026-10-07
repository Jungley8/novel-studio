# NovelStudio (故事工厂)

> **2026-2027 工业级 AI 网络小说创作桌面工作台 (Go + Embedded Web + SQLite)**  
> 告别低效脆弱的聊天框提示词复制粘贴，走向“状态机约束 + 双模型解耦 + 反AI检测质检 + 本地三轨持久化”的标准软件体系。

---

## 🌟 核心理念与架构特色

传统 AI 写作在长篇连载（20万~100万字）中必然陷入 **“注意力钝化、战力崩坏、道具凭空产生、AI模式化套词被平台降权”** 的困境。  
NovelStudio 采用现代工业化标准重新设计：

1. **零 CGO、纯 Go 单二进制分发**：
   - 基于 `modernc.org/sqlite` 实现无任何动态链接库依赖的本地嵌入式关系型存储；
   - 利用 Go 原生 `//go:embed` 将现代化桌面 UI 编译打包进单个可执行文件，双击即用，零环境依赖（无需安装 Python / Node）。
2. **状态机与因果账本（Entity State Machine & Plot Ledger）**：
   - 严格追踪主角修为境界、物品栏（Inventory）、随身债务与世界公理底层法则；
   - 每次下发场景任务时自动注入当前物理状态，严禁无中生有与机械降神；
   - 动态伏笔账本：跟踪“埋设 ➔ 发酵 ➔ 回收”全周期，杜绝烂尾。
3. **三权分立审校机制（Drafter ➔ Reviewer ➔ Rewriter）**：
   - **起草者（Writer）**：根据因果节拍渲染场景；
   - **AI 主编终审（Reviewer）**：基于世界公理、人物战力状态机、AI 模式化套话对草稿进行严格审查，输出通过/返工裁决（Verdict）、可量化评分（Score）与具体修改批注；
   - **定向返工（Rewriter）**：针对主编审校批注精准返工重写，复审合格后方可计入正史（Canon）。
4. **反 AI 检测 Linter 引擎（Burstiness & Clichés Scan）**：
   - 实时计算句长标准差（突发度 Burstiness 打分），对抗主流平台（番茄、起点、知乎）的文本困惑度反洗稿探针；
   - 自动化封杀“不由得、仿佛、嘴角勾起、眼神复杂”等统计学模式化废词。
5. **macOS 原生守护服务与环境自检（LaunchAgent & Doctor）**：
   - 原生集成 macOS `launchd`，支持后台常驻、开机自启、日志轮转与存活健康探针；
   - 内置 `doctor` 自检自愈套件，秒级排查环境权限、端口占用、SQLite 驱动与 API 连通性。

---

## 🧭 合理的交互流程设计 (Sensible Interaction Flow)

```
[启动桌面客户端]
双击 novel-studio 可执行文件 (或终端执行 ./novel-studio)
      │
      ▼
本地后台自动加载 SQLite 数据库 (~/.novel-studio/novel.db)
自动唤起系统默认浏览器访问 http://127.0.0.1:28980
      │
      ▼
【六步工业化创作闭环】
Step 1: 【多作品工作区】 (Project Workspace)
        新建/切换作品（番茄脑洞 / 知乎盐言 / 起点仙侠 / 海外短剧），配置全局 API 密钥与端点。
Step 2: 【实体状态机与世界公理】 (State Machine)
        初始化主角等级、物品栏、隐秘目标，声明“物理与修真不可逾越法则”。
Step 3: 【伏笔因果账本】 (Plot Hooks)
        记录伏笔事实，设定目标回收章节，到达节点时系统自动预警。
Step 4: 【章节工坊与节拍推演】 (Chapter Workshop)
        4.1 录入本章核心冲突 / 必须打破的心理预期；
        4.2 点击「自动推演节拍」➔ 推理模型输出 4 个 Scene Beats + 状态结算预测；
        4.3 人工微调节拍 ➔ 点击「启动文学渲染」输出高镜头感正文草稿。
Step 5: 【AI 主编终审与定向返工】 (Review & Rewrite)
        5.1 点击「🛡️ AI 主编终审」➔ 审核模型依据公理与人物状态机独立打分出具审稿单；
        5.2 若需返工，一键点击「🔄 定向返工重写」针对批注自动改写并复审；
        5.3 审核通过后点击「✅ 确认归档」➔ SQLite 自动更新状态机与审校结果、自增章节号。
Step 6: 【多形态资产导出】 (Export)
        一键导出全本 Markdown 文档或工程完整快照 JSON。
```

---

## 🖥️ macOS 原生守护服务与自检 (Service & Doctor)

除了交互式桌面模式，NovelStudio 提供生产级守护服务管理：

### 1. macOS launchd 服务管理

```bash
# 注册为系统原生后台服务 (开机自启、常驻守护)
./novel-studio service install

# 查看服务存活状态与健康探针 (GET /api/health)
./novel-studio service status

# 查看后台服务日志
./novel-studio service logs

# 打开系统默认浏览器访问控制台
./novel-studio service open

# 停止或卸载服务
./novel-studio service stop
./novel-studio service uninstall
```

### 2. 环境自检与故障诊断 (Doctor)

```bash
# 执行完整自检自愈报告 (检测 OS、二进制路径、目录读写权限、SQLite、端口与 API 凭证)
./novel-studio doctor
```

### 3. 无头全流程自主产章 (Produce Pipeline)

```bash
# 命令行无头直接运行章节工坊 (自动提取3章正史、推演节拍、文学渲染、多轮复审返工并原子归档)
./novel-studio produce -conflict "拍卖会反派恶意抬价，主角借机设伏反杀"

# 指定作品 ID 与章节序号
./novel-studio produce -project <project_id> -chapter 5 -conflict "宗门大比首轮遭遇宿敌"
```

---

## 🚀 快速上手 (Quick Start)

### 1. 源码编译

```bash
# 克隆仓库
git clone https://github.com/Jungley8/novel-studio.git
cd novel-studio

# 编译为单二进制执行文件
go build -o novel-studio ./cmd/novel-studio

# 运行桌面服务 (自动唤起浏览器)
./novel-studio
```

### 2. 常用启动参数

```bash
# 指定自定义端口
./novel-studio -port 30000

# 指定数据存储目录 (默认为 ~/.novel-studio)
./novel-studio -data-dir /path/to/custom/data

# 无头/服务器模式运行 (不自动打开浏览器)
./novel-studio -no-browser
```

---

## 🛠️ 技术栈与工程规范

- **后端核心**：Go 1.24+ 标准库
- **持久化驱动**：`modernc.org/sqlite` (Pure Go, ACID, 0 CGO)
- **UI 呈现**：Tailwind CSS + Vue 3 (通过 `embed.FS` 嵌入式分发)
- **模型协议**：兼容所有 OpenAI 规范 API（DeepSeek, OpenAI, Claude-proxy, One-API 等）
- **测试覆盖**：全模块覆盖单元测试与集成测试，原生支持 `-race` 竞态检测

```bash
# 运行完整测试套件
go test -v -race ./...
```

---

## 📄 开源许可证

本项目基于 [MIT 许可证](LICENSE) 开源。
