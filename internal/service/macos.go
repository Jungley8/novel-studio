package service

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/Jungley8/novel-studio/internal/desktop"
)

const (
	ServiceLabel = "com.jungley8.novel-studio"
)

// ServiceManager manages macOS launchd service lifecycle.
type ServiceManager struct {
	binaryPath string
	dataDir    string
	port       int
}

func NewServiceManager(dataDir string, port int) (*ServiceManager, error) {
	binPath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("detect binary path failed: %w", err)
	}
	binPath, err = filepath.EvalSymlinks(binPath)
	if err != nil {
		return nil, fmt.Errorf("eval symlink failed: %w", err)
	}

	if port <= 0 {
		port = 28980
	}

	return &ServiceManager{
		binaryPath: binPath,
		dataDir:    dataDir,
		port:       port,
	}, nil
}

func (m *ServiceManager) plistPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", ServiceLabel+".plist"), nil
}

// Install writes the LaunchAgent plist and loads it via launchctl.
func (m *ServiceManager) Install() error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("service install 当前仅原生支持 macOS (launchd)")
	}

	plistPath, err := m.plistPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(plistPath), 0755); err != nil {
		return fmt.Errorf("create LaunchAgents dir failed: %w", err)
	}
	if err := os.MkdirAll(m.dataDir, 0755); err != nil {
		return fmt.Errorf("create data dir failed: %w", err)
	}

	logPath := filepath.Join(m.dataDir, "service.log")
	errLogPath := filepath.Join(m.dataDir, "service.err.log")

	plistContent := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>%s</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
        <string>-no-browser</string>
        <string>-port</string>
        <string>%d</string>
        <string>-data-dir</string>
        <string>%s</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>WorkingDirectory</key>
    <string>%s</string>
    <key>StandardOutPath</key>
    <string>%s</string>
    <key>StandardErrorPath</key>
    <string>%s</string>
</dict>
</plist>
`, ServiceLabel, m.binaryPath, m.port, m.dataDir, m.dataDir, logPath, errLogPath)

	if err := os.WriteFile(plistPath, []byte(plistContent), 0644); err != nil {
		return fmt.Errorf("write plist failed: %w", err)
	}

	// Unload if previously loaded, then load
	_ = exec.Command("launchctl", "unload", plistPath).Run()
	cmd := exec.Command("launchctl", "load", plistPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl load failed: %s (%w)", string(out), err)
	}

	return nil
}

// Uninstall unloads and removes the LaunchAgent plist.
func (m *ServiceManager) Uninstall() error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("service uninstall 当前仅原生支持 macOS (launchd)")
	}

	plistPath, err := m.plistPath()
	if err != nil {
		return err
	}

	_ = exec.Command("launchctl", "unload", plistPath).Run()
	_ = os.Remove(plistPath)
	return nil
}

// Start instructs launchctl to start the service.
func (m *ServiceManager) Start() error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("service start 当前仅原生支持 macOS (launchd)")
	}

	plistPath, err := m.plistPath()
	if err != nil {
		return err
	}

	if _, err := os.Stat(plistPath); os.IsNotExist(err) {
		// Auto-install if not installed yet
		if err := m.Install(); err != nil {
			return err
		}
	}

	cmd := exec.Command("launchctl", "start", ServiceLabel)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl start failed: %s (%w)", string(out), err)
	}

	return nil
}

// Stop instructs launchctl to stop the service.
func (m *ServiceManager) Stop() error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("service stop 当前仅原生支持 macOS (launchd)")
	}

	cmd := exec.Command("launchctl", "stop", ServiceLabel)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl stop failed: %s (%w)", string(out), err)
	}
	return nil
}

// Status checks service registration and probes /api/health.
func (m *ServiceManager) Status() (running bool, message string) {
	apiURL := fmt.Sprintf("http://127.0.0.1:%d/api/health", m.port)
	client := &http.Client{Timeout: 800 * time.Millisecond}

	start := time.Now()
	resp, err := client.Get(apiURL)
	if err == nil && resp.StatusCode == http.StatusOK {
		_ = resp.Body.Close()
		latency := time.Since(start).Round(time.Millisecond)
		return true, fmt.Sprintf("服务正在运行 [PID/Daemon 正常] (地址: http://127.0.0.1:%d, 延迟: %s)", m.port, latency)
	}

	// Check if plist is installed
	plistPath, _ := m.plistPath()
	if _, err := os.Stat(plistPath); err == nil {
		return false, fmt.Sprintf("服务未运行 (已注册 LaunchAgent: %s，但端口 %d 未响应)", plistPath, m.port)
	}

	return false, "服务未安装且未运行 (可执行: novel-studio service install 进行安装)"
}

// Open launches the browser to view dashboard.
func (m *ServiceManager) Open() error {
	url := fmt.Sprintf("http://127.0.0.1:%d", m.port)
	return desktop.OpenBrowser(url)
}

// Logs reads the latest lines from service logs.
func (m *ServiceManager) Logs(tailLines int) (string, error) {
	logPath := filepath.Join(m.dataDir, "service.log")
	errPath := filepath.Join(m.dataDir, "service.err.log")

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("=== 标准日志 (%s) ===\n", logPath))
	if data, err := os.ReadFile(logPath); err == nil {
		sb.WriteString(tailString(string(data), tailLines))
	} else {
		sb.WriteString("(无内容或未生成)\n")
	}

	sb.WriteString(fmt.Sprintf("\n=== 错误日志 (%s) ===\n", errPath))
	if data, err := os.ReadFile(errPath); err == nil {
		sb.WriteString(tailString(string(data), tailLines))
	} else {
		sb.WriteString("(无内容或未生成)\n")
	}

	return sb.String(), nil
}

func tailString(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) <= n {
		return strings.Join(lines, "\n") + "\n"
	}
	return strings.Join(lines[len(lines)-n:], "\n") + "\n"
}

// RunCLI handles `novel-studio service <subcommand>`.
func RunCLI(argv []string, dataDir string, port int) int {
	var cmd string
	for i := 0; i < len(argv); i++ {
		arg := argv[i]
		if arg == "-port" && i+1 < len(argv) {
			if p, err := strconv.Atoi(argv[i+1]); err == nil && p > 0 {
				port = p
			}
			i++
		} else if strings.HasPrefix(arg, "-port=") {
			if p, err := strconv.Atoi(strings.TrimPrefix(arg, "-port=")); err == nil && p > 0 {
				port = p
			}
		} else if arg == "-data-dir" && i+1 < len(argv) {
			dataDir = argv[i+1]
			i++
		} else if strings.HasPrefix(arg, "-data-dir=") {
			dataDir = strings.TrimPrefix(arg, "-data-dir=")
		} else if !strings.HasPrefix(arg, "-") && cmd == "" {
			cmd = arg
		}
	}

	if cmd == "" {
		printServiceHelp(os.Stdout)
		return 0
	}

	mgr, err := NewServiceManager(dataDir, port)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[Service Error] %v\n", err)
		return 1
	}

	switch cmd {
	case "install":
		if err := mgr.Install(); err != nil {
			fmt.Fprintf(os.Stderr, "✗ 安装 macOS 服务失败: %v\n", err)
			return 1
		}
		fmt.Println("✓ NovelStudio macOS Service 已成功安装并启动！")
		fmt.Printf("  • 配置路径: ~/Library/LaunchAgents/%s.plist\n", ServiceLabel)
		fmt.Printf("  • 服务地址: http://127.0.0.1:%d\n", mgr.port)
		return 0

	case "uninstall":
		if err := mgr.Uninstall(); err != nil {
			fmt.Fprintf(os.Stderr, "✗ 卸载服务失败: %v\n", err)
			return 1
		}
		fmt.Println("✓ NovelStudio macOS Service 已成功卸载。")
		return 0

	case "start":
		if err := mgr.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "✗ 启动服务失败: %v\n", err)
			return 1
		}
		time.Sleep(500 * time.Millisecond)
		running, msg := mgr.Status()
		if running {
			fmt.Printf("✓ %s\n", msg)
		} else {
			fmt.Printf("! 启动指令已发送: %s\n", msg)
		}
		return 0

	case "stop":
		if err := mgr.Stop(); err != nil {
			fmt.Fprintf(os.Stderr, "✗ 停止服务失败: %v\n", err)
			return 1
		}
		fmt.Println("✓ NovelStudio 服务已发送停止信号。")
		return 0

	case "status":
		running, msg := mgr.Status()
		if running {
			fmt.Printf("● %s\n", msg)
			return 0
		}
		fmt.Printf("○ %s\n", msg)
		return 1

	case "open":
		if err := mgr.Open(); err != nil {
			fmt.Fprintf(os.Stderr, "✗ 打开浏览器失败: %v\n", err)
			return 1
		}
		return 0

	case "logs":
		logs, _ := mgr.Logs(25)
		fmt.Println(logs)
		return 0

	default:
		fmt.Fprintf(os.Stderr, "未知服务子命令: %s\n\n", argv[0])
		printServiceHelp(os.Stderr)
		return 2
	}
}

func printServiceHelp(w io.Writer) {
	fmt.Fprintln(w, "NovelStudio macOS Service (launchd) 服务管理")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "用法:")
	fmt.Fprintln(w, "  novel-studio service <command>")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "可用命令:")
	fmt.Fprintln(w, "  install    将 NovelStudio 注册为 macOS 原生后台服务 (开机自启/常驻守护)")
	fmt.Fprintln(w, "  uninstall  卸载并移除 macOS 后台服务")
	fmt.Fprintln(w, "  start      启动后台服务")
	fmt.Fprintln(w, "  stop       停止后台服务")
	fmt.Fprintln(w, "  status     查看后台服务存活与健康探针状态")
	fmt.Fprintln(w, "  open       在系统默认浏览器中打开运行看板")
	fmt.Fprintln(w, "  logs       查看后台服务标准输出与错误日志")
}
