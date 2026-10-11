package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Jungley8/novel-studio/internal/engine"
	"github.com/Jungley8/novel-studio/internal/store"
	sqlite "modernc.org/sqlite"
)

type benchJSONReport struct {
	SnapshotPath string `json:"snapshot_path,omitempty"`
	*engine.BenchReport
}

type onlineBackuper interface {
	NewBackup(dstUri string) (*sqlite.Backup, error)
}

// openBenchSnapshot creates an isolated online backup snapshot of the SQLite database
// via a read-only source connection and returns the snapshot file path along with a cleanup function.
func openBenchSnapshot(src string) (string, func(), error) {
	if _, err := os.Stat(src); err != nil {
		return "", nil, err
	}

	tmpDir, err := os.MkdirTemp("", "novel-bench-*")
	if err != nil {
		return "", nil, fmt.Errorf("创建临时目录失败: %w", err)
	}

	snapshotPath := filepath.Join(tmpDir, "novel.db")
	cleanup := func() {
		_ = os.RemoveAll(tmpDir)
	}

	dsn := src + "?mode=ro&_pragma=busy_timeout(5000)&_pragma=query_only(1)"

	srcDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("打开源数据库失败: %w", err)
	}
	defer srcDB.Close()

	ctx := context.Background()
	conn, err := srcDB.Conn(ctx)
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("获取源数据库连接失败: %w", err)
	}
	defer conn.Close()

	err = conn.Raw(func(driverConn any) error {
		b, ok := driverConn.(onlineBackuper)
		if !ok {
			return fmt.Errorf("驱动连接 %T 未实现 onlineBackuper 接口", driverConn)
		}

		bck, err := b.NewBackup(snapshotPath)
		if err != nil {
			return fmt.Errorf("创建在线备份失败: %w", err)
		}

		if _, err := bck.Step(-1); err != nil {
			_ = bck.Finish()
			return fmt.Errorf("执行在线备份失败: %w", err)
		}

		dstConn, err := bck.Commit()
		if err != nil {
			return fmt.Errorf("提交在线备份失败: %w", err)
		}

		if err := dstConn.Close(); err != nil {
			return fmt.Errorf("关闭快照目标连接失败: %w", err)
		}
		return nil
	})
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("备份数据库失败: %w", err)
	}

	return snapshotPath, cleanup, nil
}

// printBenchReport formats and renders a BenchReport to the given writer.
// 在临时快照上运行，不触碰生产库。
// Supports both structured human-readable Chinese text and indented JSON format.
func printBenchReport(w io.Writer, r *engine.BenchReport, asJSON bool, snapshotPath ...string) {
	snap := ""
	if len(snapshotPath) > 0 {
		snap = snapshotPath[0]
	}

	if asJSON {
		wrapper := benchJSONReport{
			SnapshotPath: snap,
			BenchReport:  r,
		}
		data, err := json.MarshalIndent(wrapper, "", "  ")
		if err != nil {
			fmt.Fprintf(w, "JSON 序列化失败: %v\n", err)
			return
		}
		fmt.Fprintln(w, string(data))
		return
	}

	if snap != "" {
		fmt.Fprintf(w, "快照路径: %s\n", snap)
	}
	fmt.Fprintln(w, "==================================================================")
	fmt.Fprintf(w, "  NovelStudio Bench 长篇质量回归体检报告\n")
	fmt.Fprintf(w, "  作品: 《%s》 (ID: %s) | 已归档: %d 章\n", r.ProjectTitle, r.ProjectID, r.Chapters)
	fmt.Fprintln(w, "==================================================================")

	// 【字数体检】
	fmt.Fprintln(w, "\n【字数体检】")
	fmt.Fprintf(w, "  • 目标字数:     %d 字\n", r.WordCount.Target)
	fmt.Fprintf(w, "  • 平均字数:     %d 字\n", r.WordCount.Avg)
	fmt.Fprintf(w, "  • 最少字数:     %d 字\n", r.WordCount.Min)
	fmt.Fprintf(w, "  • 最多字数:     %d 字\n", r.WordCount.Max)
	fmt.Fprintf(w, "  • 低于及格线:   %d 章 (<80%% 目标字数)\n", r.WordCount.UnderTarget)

	// 【章型节奏】
	fmt.Fprintln(w, "\n【章型节奏】")
	fmt.Fprintf(w, "  • 最长连续同型: %d 章", r.ChapterTypes.MaxConsecutive)
	if len(r.ChapterTypes.MaxRunChapters) > 0 {
		fmt.Fprintf(w, " (章号: %v)\n", r.ChapterTypes.MaxRunChapters)
	} else {
		fmt.Fprintln(w)
	}
	fmt.Fprintln(w, "  • 章型分布统计:")
	if len(r.ChapterTypes.Distribution) == 0 {
		fmt.Fprintln(w, "    (暂无章型数据)")
	} else {
		keys := make([]string, 0, len(r.ChapterTypes.Distribution))
		for k := range r.ChapterTypes.Distribution {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(w, "    - %-12s: %d 章\n", k, r.ChapterTypes.Distribution[k])
		}
	}

	// 【伏笔账本】
	fmt.Fprintln(w, "\n【伏笔账本】")
	fmt.Fprintf(w, "  • 活跃伏笔 (OPEN):       %d 条\n", r.Hooks.Open)
	fmt.Fprintf(w, "  • 发酵伏笔 (FERMENTING): %d 条\n", r.Hooks.Fermenting)
	fmt.Fprintf(w, "  • 已回收 (RESOLVED):     %d 条\n", r.Hooks.Resolved)
	fmt.Fprintf(w, "  • 已废弃 (ABANDONED):    %d 条\n", r.Hooks.Abandoned)
	fmt.Fprintf(w, "  • 逾期未回收伏笔:        %d 条\n", len(r.Hooks.Overdue))
	for _, ov := range r.Hooks.Overdue {
		fmt.Fprintf(w, "    - [%s] 目标第 %d 章 | %s (ID: %s)\n", ov.Status, ov.TargetChapter, ov.Title, ov.ID)
	}

	// 【账本重放】
	fmt.Fprintln(w, "\n【账本重放】")
	statusStr := "纯净 (CLEAN)"
	if !r.LedgerReplay.Clean {
		statusStr = "存在漂移 (DRIFT)"
	}
	fmt.Fprintf(w, "  • 状态机审计结论: %s\n", statusStr)
	fmt.Fprintf(w, "  • 提示说明:       %s\n", r.LedgerReplay.Caveat)
	if len(r.LedgerReplay.Drifts) > 0 {
		fmt.Fprintf(w, "  • 漂移明细 (%d 条):\n", len(r.LedgerReplay.Drifts))
		for _, d := range r.LedgerReplay.Drifts {
			fmt.Fprintf(w, "    - %s\n", d)
		}
	} else {
		fmt.Fprintln(w, "  • 漂移明细:       无漂移")
	}
	if len(r.LedgerReplay.Notes) > 0 {
		fmt.Fprintf(w, "  • 备注 (%d 条):\n", len(r.LedgerReplay.Notes))
		for _, n := range r.LedgerReplay.Notes {
			fmt.Fprintf(w, "    - %s\n", n)
		}
	}

	// 【Lint 存量体检】（只读存量字段，不跑 LLM/linter）
	fmt.Fprintln(w, "\n【Lint 存量体检】")
	fmt.Fprintf(w, "  • Linter 通过率:   %.1f%% (%d/%d 章)\n", r.Lint.LinterPassRate*100, r.Lint.LinterPassed, r.Lint.Chapters)
	fmt.Fprintf(w, "  • 平均 Burstiness: %.2f\n", r.Lint.AvgBurstinessScore)
	fmt.Fprintf(w, "  • Review 覆盖:     %d 章\n", r.Lint.ReviewCovered)

	// 【跨章回声】
	fmt.Fprintln(w, "\n【跨章回声】")
	fmt.Fprintf(w, "  • 跨章高频重复短语: %d 处\n", r.Echo.TotalPhrases)
	fmt.Fprintf(w, "  • 命中重复章节数:   %d 章\n", r.Echo.ChaptersHit)

	// 【文风契约】（直读 report 字段；drift 前缀扫描仅作旧 report 兼容保留）
	fmt.Fprintln(w, "\n【文风契约】")
	seededStr := "未种子化"
	if r.StyleVaultSeeded {
		seededStr = "已种子化"
	}
	if r.StyleVaultError != "" {
		seededStr = fmt.Sprintf("风格库读取失败：%s", r.StyleVaultError)
	} else {
		for _, d := range r.LedgerReplay.Drifts {
			if strings.HasPrefix(d, "风格库读取失败：") {
				seededStr = d
				break
			}
		}
	}
	fmt.Fprintf(w, "  • 文风保险库状态:   %s\n", seededStr)
	fmt.Fprintln(w, "==================================================================")
}

// runBenchCLI implements the `novel-studio bench` CLI command.
func runBenchCLI(argv []string, defaultDataDir string) int {
	fs := flag.NewFlagSet("bench", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "NovelStudio bench: 在临时快照上运行，不触碰生产库（将复制到临时目录）。\n\n")
		fmt.Fprintf(fs.Output(), "Usage of bench:\n")
		fs.PrintDefaults()
	}
	dataDirFlag := fs.String("data-dir", defaultDataDir, "数据持久化目录（将复制到临时目录进行快照体检）")
	projectIDFlag := fs.String("project", "", "作品 ID (默认首个已有作品)")
	jsonFlag := fs.Bool("json", false, "以 JSON 格式输出体检报告")
	if err := fs.Parse(argv); err != nil {
		return 1
	}

	dataDir := *dataDirFlag
	dbPath := filepath.Join(dataDir, "novel.db")
	snapshotPath, cleanup, err := openBenchSnapshot(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "✗ 打开 SQLite 数据库失败: %v\n", err)
		return 1
	}
	defer cleanup()

	s, err := store.NewSQLiteStore(snapshotPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "✗ 打开 SQLite 数据库失败: %v\n", err)
		return 1
	}
	defer s.Close()

	ctx := context.Background()
	projectID := *projectIDFlag
	if projectID == "" {
		projects, err := s.ListProjects(ctx)
		if err != nil || len(projects) == 0 {
			fmt.Fprintf(os.Stderr, "✗ 未找到任何作品，请先创建作品\n")
			return 1
		}
		projectID = projects[0].ID
	}

	project, err := s.GetProject(ctx, projectID)
	if err != nil || project == nil {
		fmt.Fprintf(os.Stderr, "✗ 获取作品 [%s] 失败: %v\n", projectID, err)
		return 1
	}

	chapters, err := s.ListChapters(ctx, projectID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "✗ 获取章节列表失败: %v\n", err)
		return 1
	}

	hooks, err := s.ListPlotHooks(ctx, projectID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "✗ 获取伏笔列表失败: %v\n", err)
		return 1
	}

	snapshots, snapErr := s.ListStateSnapshots(ctx, projectID)
	if snapErr != nil {
		snapshots = nil
	}

	protagonist, protErr := s.GetProtagonistEntry(ctx, projectID)
	styleVault, styleVaultErr := s.GetStyleVault(ctx, projectID)
	styleVaultSeeded := (styleVault != nil)

	report := engine.RunBenchProbes(project, chapters, hooks, protagonist, styleVaultSeeded, snapshots)
	if snapErr != nil {
		report.LedgerReplay.Notes = append(report.LedgerReplay.Notes, fmt.Sprintf("快照读取失败：%v", snapErr))
		report.LedgerReplay.Clean = false
	}
	if styleVaultErr != nil {
		report.StyleVaultError = styleVaultErr.Error()
	}
	if protErr != nil {
		report.LedgerReplay.Clean = false
		report.LedgerReplay.Drifts = append(report.LedgerReplay.Drifts, fmt.Sprintf("主角读取失败：%v", protErr))
		report.LedgerReplay.Caveat = engine.BenchLedgerCaveat + fmt.Sprintf("（主角读取失败：%v）", protErr)
	}
	if styleVaultErr != nil {
		report.LedgerReplay.Clean = false
		report.LedgerReplay.Drifts = append(report.LedgerReplay.Drifts, fmt.Sprintf("风格库读取失败：%v", styleVaultErr))
	}

	printBenchReport(os.Stdout, report, *jsonFlag, snapshotPath)
	return 0
}
