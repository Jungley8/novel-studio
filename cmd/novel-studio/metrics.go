package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Jungley8/novel-studio/internal/domain"
	"github.com/Jungley8/novel-studio/internal/store"
)

// MetricsSummary encapsulates aggregated chapter quality statistics for a project.
type MetricsSummary struct {
	Chapters                       int
	CommittedChapters              int
	NeedsHumanChapters             int
	TechnicalBlockedChapters       int
	ProducedChapters               int
	UnderTargetChapters            int
	RewriteChapters                int
	UnclosedReconciliationChapters int
	CompactionFailedChapters       int
	WordPassRate                   float64 // 1 - UnderTarget/Chapters
	AvgScore                       float64
	RewriteRate                    float64
	HookClaimsTotal                int
	HookClaimsValidated            int
	HookClaimsRejected             int
	HookPassRate                   float64
}

// aggregateMetrics aggregates chapter quality metrics for reporting.
func aggregateMetrics(items []*domain.ChapterQualityMetrics) MetricsSummary {
	var summary MetricsSummary
	if len(items) == 0 {
		return summary
	}

	summary.Chapters = len(items)
	totalScore := 0

	for _, it := range items {
		if it == nil {
			continue
		}
		switch it.Outcome {
		case domain.ChapterOutcomeCommitted:
			summary.CommittedChapters++
		case domain.ChapterOutcomeNeedsHuman:
			summary.NeedsHumanChapters++
		case domain.ChapterOutcomeTechnicalBlocked:
			summary.TechnicalBlockedChapters++
		case domain.ChapterOutcomeProduced:
			summary.ProducedChapters++
		}
		if it.UnderTarget {
			summary.UnderTargetChapters++
		}
		if it.RewriteLoops > 0 {
			summary.RewriteChapters++
		}
		if it.UnresolvedObservations > 0 {
			summary.UnclosedReconciliationChapters++
		}
		if it.CompactionFailed {
			summary.CompactionFailedChapters++
		}
		totalScore += it.Score
		summary.HookClaimsTotal += it.HookClaimsTotal
		summary.HookClaimsValidated += it.HookClaimsValidated
		summary.HookClaimsRejected += it.HookClaimsRejected
	}

	summary.WordPassRate = 1.0 - float64(summary.UnderTargetChapters)/float64(summary.Chapters)
	summary.AvgScore = float64(totalScore) / float64(summary.Chapters)
	summary.RewriteRate = float64(summary.RewriteChapters) / float64(summary.Chapters)

	if summary.HookClaimsTotal > 0 {
		summary.HookPassRate = float64(summary.HookClaimsValidated) / float64(summary.HookClaimsTotal)
	}

	return summary
}

func printProjectMetrics(projectID, projectTitle string, summary MetricsSummary) {
	fmt.Println("==================================================================")
	title := projectTitle
	if title == "" {
		title = projectID
	}
	fmt.Printf("作品: %s (%s)\n", title, projectID)
	fmt.Println("------------------------------------------------------------------")
	if summary.Chapters == 0 {
		fmt.Println("  (未找到任何章节质量指标记录)")
		fmt.Println("==================================================================")
		return
	}
	fmt.Printf("  统计章节数: %d\n", summary.Chapters)
	fmt.Printf("  终态分布: 已入库 %d / 停线待人工 %d / 技术阻断 %d / 已产出未入库 %d\n",
		summary.CommittedChapters, summary.NeedsHumanChapters, summary.TechnicalBlockedChapters, summary.ProducedChapters)
	fmt.Printf("  字数达标率: %.1f%% (未达标章节: %d)\n", summary.WordPassRate*100, summary.UnderTargetChapters)
	fmt.Printf("  平均评审分: %.1f\n", summary.AvgScore)
	fmt.Printf("  重写返工率: %.1f%% (重写章节: %d)\n", summary.RewriteRate*100, summary.RewriteChapters)
	fmt.Printf("  对账未闭合: %d 章\n", summary.UnclosedReconciliationChapters)
	if summary.HookClaimsTotal > 0 {
		fmt.Printf("  伏笔证据门: 声明 %d 条 / 通过 %d 条 / 拒绝 %d 条 (通过率: %.1f%%)\n",
			summary.HookClaimsTotal, summary.HookClaimsValidated, summary.HookClaimsRejected, summary.HookPassRate*100)
	} else {
		fmt.Printf("  伏笔证据门: 声明 0 条 / 通过 0 条 / 拒绝 0 条 (通过率: 0.0%%)\n")
	}
	fmt.Printf("  压缩失败数: %d 章\n", summary.CompactionFailedChapters)
	fmt.Println("==================================================================")
}

// runMetricsCLI implements the `novel-studio metrics` CLI command.
func runMetricsCLI(argv []string, defaultDataDir string) int {
	fs := flag.NewFlagSet("metrics", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "NovelStudio metrics: 章节质量指标聚合评测工具。\n\n")
		fmt.Fprintf(fs.Output(), "Usage of metrics:\n")
		fs.PrintDefaults()
	}
	dataDirFlag := fs.String("data-dir", defaultDataDir, "数据持久化目录")
	projectIDFlag := fs.String("project", "", "作品 ID (可选，为空时打印全部项目)")
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
	targetProjectID := *projectIDFlag

	if targetProjectID != "" {
		p, err := s.GetProject(ctx, targetProjectID)
		pTitle := targetProjectID
		if err == nil && p != nil {
			pTitle = p.Title
		}
		items, err := s.ListChapterMetrics(ctx, targetProjectID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "✗ 查询作品 [%s] 指标失败: %v\n", targetProjectID, err)
			return 1
		}
		summary := aggregateMetrics(items)
		printProjectMetrics(targetProjectID, pTitle, summary)
		return 0
	}

	projects, err := s.ListProjects(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "✗ 列出作品失败: %v\n", err)
		return 1
	}
	if len(projects) == 0 {
		fmt.Println("未找到任何作品。")
		return 0
	}

	for _, p := range projects {
		items, err := s.ListChapterMetrics(ctx, p.ID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "✗ 查询作品 [%s] 指标失败: %v\n", p.ID, err)
			continue
		}
		summary := aggregateMetrics(items)
		printProjectMetrics(p.ID, p.Title, summary)
	}

	return 0
}
