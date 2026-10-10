package main

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Jungley8/novel-studio/internal/domain"
	"github.com/Jungley8/novel-studio/internal/engine"
	"github.com/Jungley8/novel-studio/internal/store"
)

// 测试 printBenchReport 输出包含六大中文关键分区标题以及 JSON 模式
func TestPrintBenchReport_ContainsAllSections(t *testing.T) {
	report := &engine.BenchReport{
		ProjectID:    "proj-bench-001",
		ProjectTitle: "万道独尊",
		Chapters:     5,
		WordCount: engine.BenchWordCount{
			Avg:         2100,
			Min:         1500,
			Max:         2600,
			Target:      2000,
			UnderTarget: 1,
		},
		ChapterTypes: engine.BenchChapterTypes{
			Distribution: map[string]int{
				"ESCALATION": 4,
				"CLIMAX":     1,
			},
			MaxConsecutive: 4,
			MaxRunChapters: []int{1, 2, 3, 4},
		},
		Hooks: engine.BenchHooks{
			Open:       2,
			Fermenting: 1,
			Resolved:   1,
			Abandoned:  0,
			Overdue: []engine.OverdueHook{
				{
					ID:            "hook-1",
					Title:         "神秘残图",
					TargetChapter: 3,
					Status:        domain.HookStatusOpen,
				},
			},
		},
		LedgerReplay: engine.BenchLedgerReplay{
			Clean:  false,
			Drifts: []string{"道具「灵石」: 重放=2 当前=5"},
			Caveat: engine.BenchLedgerCaveat,
		},
		Echo: engine.BenchEcho{
			TotalPhrases: 1,
			ChaptersHit:  1,
		},
		StyleVaultSeeded: true,
	}

	var buf bytes.Buffer
	printBenchReport(&buf, report, false)
	out := buf.String()

	requiredSections := []string{
		"字数体检",
		"章型节奏",
		"伏笔账本",
		"账本重放",
		"跨章回声",
		"文风契约",
	}

	for _, sec := range requiredSections {
		if !strings.Contains(out, sec) {
			t.Errorf("expected output to contain section title %q, output:\n%s", sec, out)
		}
	}

	// 检查关键内容对齐展示
	if !strings.Contains(out, "万道独尊") {
		t.Errorf("expected project title in output")
	}
	if !strings.Contains(out, "神秘残图") {
		t.Errorf("expected overdue hook in output")
	}
	if !strings.Contains(out, "道具「灵石」: 重放=2 当前=5") {
		t.Errorf("expected drift item in output")
	}

	// JSON 输出模式验证
	var jsonBuf bytes.Buffer
	printBenchReport(&jsonBuf, report, true)
	var parsed map[string]any
	if err := json.Unmarshal(jsonBuf.Bytes(), &parsed); err != nil {
		t.Fatalf("failed to unmarshal JSON output: %v", err)
	}
	if parsed["project_id"] != "proj-bench-001" {
		t.Errorf("expected project_id 'proj-bench-001', got %v", parsed["project_id"])
	}
}

// Smoke 测试：建临时 SQLite + 一个项目（无章），验证 runBenchCLI 返回 0
func TestRunBenchCLI_Smoke(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "novel.db")

	s, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("failed to open sqlite store: %v", err)
	}

	ctx := context.Background()
	proj := &domain.Project{
		ID:                    "proj-smoke-bench",
		Title:                 "体检自检作品",
		TargetPlatform:        "起点仙侠",
		DefaultWordsTarget:    2000,
		DefaultNarrativeStyle: "hardboiled",
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}
	if err := s.SaveProject(ctx, proj); err != nil {
		_ = s.Close()
		t.Fatalf("failed to create project: %v", err)
	}
	_ = s.Close()

	// 1. runBenchCLI 对存在项目返回 0 (文本模式)
	code := runBenchCLI([]string{"-data-dir", tempDir, "-project", "proj-smoke-bench"}, tempDir)
	if code != 0 {
		t.Errorf("expected exit code 0 for existing project, got %d", code)
	}

	// 2. runBenchCLI 对存在项目返回 0 (JSON 模式)
	codeJSON := runBenchCLI([]string{"-data-dir", tempDir, "-project", "proj-smoke-bench", "-json"}, tempDir)
	if codeJSON != 0 {
		t.Errorf("expected exit code 0 for JSON mode, got %d", codeJSON)
	}

	// 3. runBenchCLI 默认不指定 -project 取首项目返回 0
	codeDefaultProj := runBenchCLI([]string{"-data-dir", tempDir}, tempDir)
	if codeDefaultProj != 0 {
		t.Errorf("expected exit code 0 when defaulting to first project, got %d", codeDefaultProj)
	}

	// 4. 不存在项目返回 1
	codeNonExistent := runBenchCLI([]string{"-data-dir", tempDir, "-project", "proj-not-exist"}, tempDir)
	if codeNonExistent != 1 {
		t.Errorf("expected exit code 1 for non-existent project, got %d", codeNonExistent)
	}
}
