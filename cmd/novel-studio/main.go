package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/Jungley8/novel-studio/internal/config"
	"github.com/Jungley8/novel-studio/internal/desktop"
	"github.com/Jungley8/novel-studio/internal/engine"
	"github.com/Jungley8/novel-studio/internal/server"
	"github.com/Jungley8/novel-studio/internal/service"
	"github.com/Jungley8/novel-studio/internal/store"
)

var (
	version = "v1.0.0"
)

func main() {
	defaultHome, err := os.UserHomeDir()
	if err != nil {
		defaultHome = "."
	}
	defaultDataDir := filepath.Join(defaultHome, ".novel-studio")

	// 0. 支持直接子命令路由 (service, doctor, produce)
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "service":
			os.Exit(service.RunCLI(os.Args[2:], defaultDataDir, 28980))
		case "doctor":
			os.Exit(service.RunDoctorCLI(os.Args[2:], defaultDataDir, 28980))
		case "produce":
			os.Exit(runProduceCLI(os.Args[2:], defaultDataDir))
		case "genesis":
			os.Exit(runGenesisCLI(os.Args[2:], defaultDataDir))
		case "version", "--version", "-v":
			fmt.Printf("NovelStudio %s\n", version)
			os.Exit(0)
		case "help", "--help", "-h":
			printUsage()
			os.Exit(0)
		}
	}

	flag.Usage = printUsage
	portFlag := flag.Int("port", 0, "HTTP 服务端口号 (默认读取配置或 28980)")
	dataDirFlag := flag.String("data-dir", defaultDataDir, "数据与状态机持久化目录")
	noBrowserFlag := flag.Bool("no-browser", false, "启动后不自动唤起桌面浏览器 (适合服务器或无头模式)")
	versionFlag := flag.Bool("version", false, "显示版本号")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("NovelStudio %s\n", version)
		os.Exit(0)
	}

	// 1. 确保数据目录存在
	dataDir := *dataDirFlag
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "[NovelStudio] 创建数据目录失败: %v\n", err)
		os.Exit(1)
	}

	configPath := filepath.Join(dataDir, "config.json")
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[NovelStudio] 加载配置文件异常: %v\n", err)
		os.Exit(1)
	}

	if *portFlag > 0 {
		cfg.ServerPort = *portFlag
	}

	// 2. 初始化纯 Go SQLite 持久化
	dbPath := filepath.Join(dataDir, "novel.db")
	dbStore, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[NovelStudio] 打开 SQLite 数据库失败: %v\n", err)
		os.Exit(1)
	}
	defer dbStore.Close()

	// 3. 构建大模型客户端与编排引擎
	llmClient := engine.NewHTTPLLMClient(cfg.APIBase, cfg.APIKey)
	orch := engine.NewOrchestrator(llmClient)
	linter := engine.NewLinter(nil)

	// 4. 构建 HTTP 服务与内嵌静态资源
	srvHandler, err := server.New(cfg, configPath, dbStore, llmClient, orch, linter)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[NovelStudio] 初始化 HTTP 服务失败: %v\n", err)
		os.Exit(1)
	}

	addr := fmt.Sprintf("127.0.0.1:%d", cfg.ServerPort)
	httpServer := &http.Server{
		Addr:         addr,
		Handler:      srvHandler,
		ReadTimeout:  120 * time.Second,
		WriteTimeout: 120 * time.Second,
	}

	appURL := fmt.Sprintf("http://%s", addr)

	fmt.Println("==================================================================")
	fmt.Printf("  NovelStudio (故事工厂) %s - AI 小说工业化创作桌面工作台\n", version)
	fmt.Printf("  ● 本地持久化数据库: %s\n", dbPath)
	fmt.Printf("  ● 配置文件:         %s\n", configPath)
	fmt.Printf("  ● 桌面访问地址:     %s\n", appURL)
	fmt.Println("  ● 退出快捷键:       按 Ctrl+C 即可安全退出")
	fmt.Println("==================================================================")

	// 4. 自动唤起系统浏览器
	if !*noBrowserFlag && cfg.AutoOpenBrowser {
		go func() {
			time.Sleep(300 * time.Millisecond)
			if err := desktop.OpenBrowser(appURL); err != nil {
				fmt.Printf("[NovelStudio] 自动打开浏览器失败 (请手动访问 %s): %v\n", appURL, err)
			}
		}()
	}

	// 5. 启动服务监听
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "[NovelStudio] HTTP 服务异常退出: %v\n", err)
			os.Exit(1)
		}
	}()

	// 6. 捕获系统退出信号，优雅停机
	if err := desktop.WaitForShutdown(httpServer); err != nil {
		fmt.Fprintf(os.Stderr, "[NovelStudio] 优雅停机超时或异常: %v\n", err)
	}
	fmt.Println("[NovelStudio] 服务已安全关闭。")
}

func printUsage() {
	fmt.Fprintf(os.Stderr, "NovelStudio (故事工厂) %s - AI 小说工业化创作桌面工作台\n\n", version)
	fmt.Fprintf(os.Stderr, "用法:\n")
	fmt.Fprintf(os.Stderr, "  novel-studio [选项]                 直接启动桌面工作台与本地服务\n")
	fmt.Fprintf(os.Stderr, "  novel-studio genesis [选项]         一键推演全书宏观创世总纲与分卷设定集\n")
	fmt.Fprintf(os.Stderr, "  novel-studio produce [选项]         无头命令行一键全流程自主章节生产\n")
	fmt.Fprintf(os.Stderr, "  novel-studio service <子命令>       macOS launchd 原生常驻后台守护服务\n")
	fmt.Fprintf(os.Stderr, "  novel-studio doctor                 运行环境依赖、数据库与健康探针自检报告\n\n")
	fmt.Fprintf(os.Stderr, "服务子命令 (novel-studio service <cmd>):\n")
	fmt.Fprintf(os.Stderr, "  install    注册并加载 LaunchAgent 后台守护服务 (开机自启)\n")
	fmt.Fprintf(os.Stderr, "  uninstall  注销并移除后台服务\n")
	fmt.Fprintf(os.Stderr, "  start      启动后台服务\n")
	fmt.Fprintf(os.Stderr, "  stop       停止后台服务\n")
	fmt.Fprintf(os.Stderr, "  status     检查后台服务存活与健康探针状态\n")
	fmt.Fprintf(os.Stderr, "  open       打开浏览器访问运行看板\n")
	fmt.Fprintf(os.Stderr, "  logs       查看最近的标准与错误日志\n\n")
	fmt.Fprintf(os.Stderr, "选项:\n")
	flag.PrintDefaults()
}
