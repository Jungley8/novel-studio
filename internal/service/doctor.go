package service

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/Jungley8/novel-studio/internal/config"
	_ "modernc.org/sqlite"
)

type CheckResult struct {
	Name    string
	Status  string // "PASS", "WARN", "FAIL"
	Message string
	Detail  string
}

// RunDoctor performs end-to-end diagnostics on the NovelStudio runtime environment.
func RunDoctor(dataDir string, port int) int {
	fmt.Println("==================================================================")
	fmt.Println("  NovelStudio (故事工厂) - 运行环境与服务自检自愈报告 (Doctor)")
	fmt.Println("==================================================================")

	var results []CheckResult

	// 1. OS & Architecture Check
	osArch := fmt.Sprintf("%s/%s (%s)", runtime.GOOS, runtime.GOARCH, runtime.Version())
	results = append(results, CheckResult{
		Name:    "操作系统与运行时",
		Status:  "PASS",
		Message: osArch,
	})

	// 2. Binary Path
	binPath, err := os.Executable()
	if err == nil {
		if realPath, rerr := filepath.EvalSymlinks(binPath); rerr == nil {
			binPath = realPath
		}
		results = append(results, CheckResult{
			Name:    "主程序可执行文件路径",
			Status:  "PASS",
			Message: binPath,
		})
	} else {
		results = append(results, CheckResult{
			Name:    "主程序可执行文件路径",
			Status:  "FAIL",
			Message: fmt.Sprintf("无法定位执行程序: %v", err),
		})
	}

	// 3. Data Directory & Write Permissions
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		results = append(results, CheckResult{
			Name:    "数据持久化目录读写权限",
			Status:  "FAIL",
			Message: fmt.Sprintf("目录无法创建: %s (%v)", dataDir, err),
		})
	} else {
		testFile := filepath.Join(dataDir, fmt.Sprintf(".doctor_probe_%d", time.Now().UnixNano()))
		if err := os.WriteFile(testFile, []byte("ok"), 0644); err != nil {
			results = append(results, CheckResult{
				Name:    "数据持久化目录读写权限",
				Status:  "FAIL",
				Message: fmt.Sprintf("数据目录无写入权限: %s (%v)", dataDir, err),
			})
		} else {
			_ = os.Remove(testFile)
			results = append(results, CheckResult{
				Name:    "数据持久化目录读写权限",
				Status:  "PASS",
				Message: fmt.Sprintf("正常 (读写正常: %s)", dataDir),
			})
		}
	}

	// 4. SQLite Engine Verification
	dbPath := filepath.Join(dataDir, "novel.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		results = append(results, CheckResult{
			Name:    "SQLite 本地存储引擎",
			Status:  "FAIL",
			Message: fmt.Sprintf("数据库驱动异常: %v", err),
		})
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		var val int
		if err := db.QueryRowContext(ctx, "SELECT 1;").Scan(&val); err != nil {
			results = append(results, CheckResult{
				Name:    "SQLite 本地存储引擎",
				Status:  "FAIL",
				Message: fmt.Sprintf("数据库连接/查询异常: %v", err),
			})
		} else {
			results = append(results, CheckResult{
				Name:    "SQLite 本地存储引擎",
				Status:  "PASS",
				Message: fmt.Sprintf("正常 (无CGO纯Go引擎, 库路径: %s)", dbPath),
			})
		}
		cancel()
		_ = db.Close()
	}

	// 5. Configuration & LLM API Keys
	cfgPath := filepath.Join(dataDir, "config.json")
	cfg, err := config.Load(cfgPath)
	if err != nil {
		results = append(results, CheckResult{
			Name:    "配置文件",
			Status:  "WARN",
			Message: fmt.Sprintf("加载失败 (使用默认): %v", err),
		})
	} else {
		targetPort := port
		if targetPort <= 0 {
			targetPort = cfg.ServerPort
		}
		if targetPort <= 0 {
			targetPort = 28980
		}
		port = targetPort

		apiKeyStatus := "未配置"
		hasKey := false
		if strings.TrimSpace(cfg.APIKey) != "" {
			masked := maskKey(cfg.APIKey)
			apiKeyStatus = fmt.Sprintf("已配置 (提供商: %s, 掩码: %s)", cfg.APIBase, masked)
			hasKey = true
		} else {
			// Check env vars
			for _, envKey := range []string{"OPENAI_API_KEY", "DEEPSEEK_API_KEY", "ANTHROPIC_API_KEY", "GEMINI_API_KEY"} {
				if v := os.Getenv(envKey); strings.TrimSpace(v) != "" {
					apiKeyStatus = fmt.Sprintf("读取自环境变量 %s (%s)", envKey, maskKey(v))
					hasKey = true
					break
				}
			}
		}

		if hasKey {
			results = append(results, CheckResult{
				Name:    "大模型 API 凭证",
				Status:  "PASS",
				Message: apiKeyStatus,
			})
		} else {
			results = append(results, CheckResult{
				Name:    "大模型 API 凭证",
				Status:  "WARN",
				Message: "未配置 API Key (可使用内置 mock 模型进行离线全流程试用，或在设置面板配置 Key)",
			})
		}
	}

	// 6. Network Port Availability & Live Health Probe
	healthURL := fmt.Sprintf("http://127.0.0.1:%d/api/health", port)
	client := &http.Client{Timeout: 600 * time.Millisecond}
	resp, err := client.Get(healthURL)
	if err == nil && resp.StatusCode == http.StatusOK {
		_ = resp.Body.Close()
		results = append(results, CheckResult{
			Name:    fmt.Sprintf("服务端口与存活探针 (%d)", port),
			Status:  "PASS",
			Message: fmt.Sprintf("在线运行中 (健康探针 GET %s 正常响应)", healthURL),
		})
	} else {
		// Test if port can be bound
		ln, berr := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if berr != nil {
			results = append(results, CheckResult{
				Name:    fmt.Sprintf("服务端口与存活探针 (%d)", port),
				Status:  "WARN",
				Message: fmt.Sprintf("端口被占用但健康探针未响应: %v", berr),
				Detail:  "可能被其他应用程序占用，可在启动时指定 -port <其他端口>",
			})
		} else {
			_ = ln.Close()
			results = append(results, CheckResult{
				Name:    fmt.Sprintf("服务端口与存活探针 (%d)", port),
				Status:  "PASS",
				Message: fmt.Sprintf("空闲可用 (服务当前未运行，端口 %d 监听就绪)", port),
			})
		}
	}

	// 7. macOS LaunchAgent Service Registration (if macOS)
	if runtime.GOOS == "darwin" {
		mgr, _ := NewServiceManager(dataDir, port)
		if mgr != nil {
			plistPath, _ := mgr.plistPath()
			if _, serr := os.Stat(plistPath); serr == nil {
				// Plist exists, check launchctl list
				out, _ := exec.Command("launchctl", "list").Output()
				if strings.Contains(string(out), ServiceLabel) {
					results = append(results, CheckResult{
						Name:    "macOS launchd 后台守护服务",
						Status:  "PASS",
						Message: fmt.Sprintf("已注册且在 launchd 列表中 (%s)", plistPath),
					})
				} else {
					results = append(results, CheckResult{
						Name:    "macOS launchd 后台守护服务",
						Status:  "WARN",
						Message: fmt.Sprintf("已生成配置文件但未加载至 launchctl: %s", plistPath),
						Detail:  "可执行 `novel-studio service start` 重新加载并启动",
					})
				}
			} else {
				results = append(results, CheckResult{
					Name:    "macOS launchd 后台守护服务",
					Status:  "PASS",
					Message: "未配置后台服务 (可执行 `novel-studio service install` 一键开启原生系统常驻)",
				})
			}
		}
	}

	// Print Results
	passCount := 0
	warnCount := 0
	failCount := 0

	for _, r := range results {
		var icon string
		switch r.Status {
		case "PASS":
			icon = "✓ [通过]"
			passCount++
		case "WARN":
			icon = "! [提示]"
			warnCount++
		case "FAIL":
			icon = "✗ [失败]"
			failCount++
		}
		fmt.Printf("%-9s %-24s : %s\n", icon, r.Name, r.Message)
		if r.Detail != "" {
			fmt.Printf("          ↳ 说明: %s\n", r.Detail)
		}
	}

	fmt.Println("------------------------------------------------------------------")
	fmt.Printf("自检汇总: %d 项通过, %d 项提示, %d 项失败\n", passCount, warnCount, failCount)

	if failCount > 0 {
		fmt.Println("检测到环境异常阻断项，请参照上方提示修正后再启动。")
		return 1
	}

	fmt.Println("系统环境就绪，状态良好！")
	return 0
}

func maskKey(k string) string {
	k = strings.TrimSpace(k)
	if len(k) <= 8 {
		return "******"
	}
	return k[:4] + "...." + k[len(k)-4:]
}

// RunDoctorCLI parses command line flags and executes the diagnostic suite.
func RunDoctorCLI(argv []string, defaultDataDir string, defaultPort int) int {
	dataDir := defaultDataDir
	port := defaultPort

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
		}
	}

	return RunDoctor(dataDir, port)
}
