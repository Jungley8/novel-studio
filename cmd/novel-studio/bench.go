package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/Jungley8/novel-studio/internal/engine"
	"github.com/Jungley8/novel-studio/internal/store"
)

// printBenchReport formats and renders a BenchReport to the given writer.
// Supports both structured human-readable Chinese text and indented JSON format.
func printBenchReport(w io.Writer, r *engine.BenchReport, asJSON bool) {
	if asJSON {
		data, err := json.MarshalIndent(r, "", "  ")
		if err != nil {
			fmt.Fprintf(w, "JSON 序列化失败: %v\n", err)
			return
		}
		fmt.Fprintln(w, string(data))
		return
	}

	fmt.Fprintln(w, "==================================================================")
	fmt.Fprintf(w, "  NovelStudio Bench 长篇质量回归只读体检报告\n")
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

	// 【跨章回声】
	fmt.Fprintln(w, "\n【跨章回声】")
	fmt.Fprintf(w, "  • 跨章高频重复短语: %d 处\n", r.Echo.TotalPhrases)
	fmt.Fprintf(w, "  • 命中重复章节数:   %d 章\n", r.Echo.ChaptersHit)

	// 【文风契约】
	fmt.Fprintln(w, "\n【文风契约】")
	seededStr := "未种子化"
	if r.StyleVaultSeeded {
		seededStr = "已种子化"
	}
	fmt.Fprintf(w, "  • 文风保险库状态:   %s\n", seededStr)
	fmt.Fprintln(w, "==================================================================")
}

// runBenchCLI implements the `novel-studio bench` CLI command.
func runBenchCLI(argv []string, defaultDataDir string) int {
	fs := flag.NewFlagSet("bench", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage of bench:\n")
		fs.PrintDefaults()
	}
	dataDirFlag := fs.String("data-dir", defaultDataDir, "数据持久化目录")
	projectIDFlag := fs.String("project", "", "作品 ID (默认首个已有作品)")
	jsonFlag := fs.Bool("json", false, "以 JSON 格式输出体检报告")
	if err := fs.Parse(argv); err != nil {
		return 1
	}

	dataDir := *dataDirFlag
	dbPath := filepath.Join(dataDir, "novel.db")
	s, err := store.NewSQLiteStore(dbPath)
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

	protagonist, _ := s.GetProtagonistEntry(ctx, projectID)
	styleVault, _ := s.GetStyleVault(ctx, projectID)
	styleVaultSeeded := (styleVault != nil)

	report := engine.RunBenchProbes(project, chapters, hooks, protagonist, styleVaultSeeded)
	printBenchReport(os.Stdout, report, *jsonFlag)
	return 0
}
