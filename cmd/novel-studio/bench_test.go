package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Jungley8/novel-studio/internal/domain"
	"github.com/Jungley8/novel-studio/internal/engine"
	"github.com/Jungley8/novel-studio/internal/store"
	_ "modernc.org/sqlite"
)

// captureStdout redirects os.Stdout during fn execution and returns the captured output.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdout = w

	outChan := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		outChan <- buf.String()
	}()

	fn()

	_ = w.Close()
	os.Stdout = oldStdout
	out := <-outChan
	_ = r.Close()
	return out
}

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

// TestOpenBenchSnapshot 验证快照复制一致性、清理逻辑与不存在文件报错
func TestOpenBenchSnapshot(t *testing.T) {
	// 1. 不存在的文件应该报错
	_, _, err := openBenchSnapshot(filepath.Join(t.TempDir(), "non-existent.db"))
	if err == nil {
		t.Fatalf("expected error for non-existent source, got nil")
	}

	// 2. 正常文件流式复制与清理验证
	tempDir := t.TempDir()
	srcPath := filepath.Join(tempDir, "source.db")
	content := []byte("SQLITE-TEST-PAYLOAD-FOR-SNAPSHOT-ISOLATION-0123456789")
	if err := os.WriteFile(srcPath, content, 0644); err != nil {
		t.Fatalf("failed to write source fixture: %v", err)
	}

	snapPath, cleanup, err := openBenchSnapshot(srcPath)
	if err != nil {
		t.Fatalf("openBenchSnapshot failed: %v", err)
	}
	if snapPath == "" {
		t.Fatalf("expected non-empty snapshot path")
	}

	// 断言快照文件存在且内容一致
	snapData, err := os.ReadFile(snapPath)
	if err != nil {
		t.Fatalf("failed to read snapshot file: %v", err)
	}
	if !bytes.Equal(content, snapData) {
		t.Errorf("snapshot content mismatch: expected %q, got %q", content, snapData)
	}

	// 执行 cleanup 并断言临时目录被清理
	snapDir := filepath.Dir(snapPath)
	cleanup()
	if _, err := os.Stat(snapDir); !os.IsNotExist(err) {
		t.Errorf("expected snapshot directory %s to be removed by cleanup, got err: %v", snapDir, err)
	}
}

// TestRunBenchCLI_SnapshotIsolation 验证原库字节哈希跑前跑后完全一致，零写入
func TestRunBenchCLI_SnapshotIsolation(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "novel.db")

	s, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("failed to init store: %v", err)
	}

	ctx := context.Background()
	proj := &domain.Project{
		ID:                    "proj-iso-001",
		Title:                 "万道快照书",
		TargetPlatform:        "起点仙侠",
		DefaultWordsTarget:    2000,
		DefaultNarrativeStyle: "hardboiled",
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}
	if err := s.SaveProject(ctx, proj); err != nil {
		_ = s.Close()
		t.Fatalf("save project failed: %v", err)
	}

	ch := &domain.Chapter{
		ID:           "ch-iso-001",
		ProjectID:    proj.ID,
		ChapterIndex: 1,
		Title:        "第一章：雪夜长剑",
		Content:      "大雪漫天，长剑出鞘。剑光如虹，照亮黑夜。",
		WordCount:    2100,
		Beats: []domain.SceneBeat{
			{Phase: "绝地反杀", Tension: 9, Action: "拔剑斩敌"},
		},
		CreatedAt: time.Now(),
	}
	if err := s.SaveChapter(ctx, ch); err != nil {
		_ = s.Close()
		t.Fatalf("save chapter failed: %v", err)
	}
	_ = s.Close()

	// 跑前读取原库文件字节与计算 SHA256 哈希
	beforeBytes, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("failed to read db file before bench: %v", err)
	}
	beforeHash := sha256.Sum256(beforeBytes)

	// 跑 bench 逻辑 (文本模式)，捕获输出断言成功产出
	var codeText int
	outText := captureStdout(t, func() {
		codeText = runBenchCLI([]string{"-data-dir", tempDir, "-project", proj.ID}, tempDir)
	})
	if codeText != 0 {
		t.Fatalf("expected bench exit code 0, got %d", codeText)
	}
	if !strings.Contains(outText, "快照路径:") {
		t.Errorf("expected text output to note snapshot path on first line, got:\n%s", outText)
	}
	if !strings.Contains(outText, "《万道快照书》") {
		t.Errorf("expected report to contain project title")
	}

	// 跑后断言原库文件字节与哈希 100% 相同（快照隔离保证原库零写入）
	afterBytes, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("failed to read db file after bench: %v", err)
	}
	afterHash := sha256.Sum256(afterBytes)
	if beforeHash != afterHash || !bytes.Equal(beforeBytes, afterBytes) {
		t.Fatalf("production db was modified during bench! beforeHash=%x, afterHash=%x", beforeHash, afterHash)
	}

	// 断言数据目录下未遗留 WAL 副作用文件
	if _, err := os.Stat(filepath.Join(tempDir, "novel.db-wal")); !os.IsNotExist(err) {
		t.Errorf("expected no WAL file in production dataDir, got err: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tempDir, "novel.db-shm")); !os.IsNotExist(err) {
		t.Errorf("expected no SHM file in production dataDir, got err: %v", err)
	}

	// 跑 bench 逻辑 (JSON 模式)
	var codeJSON int
	outJSON := captureStdout(t, func() {
		codeJSON = runBenchCLI([]string{"-data-dir", tempDir, "-project", proj.ID, "-json"}, tempDir)
	})
	if codeJSON != 0 {
		t.Fatalf("expected bench JSON exit code 0, got %d", codeJSON)
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(outJSON), &parsed); err != nil {
		t.Fatalf("failed to parse bench JSON output: %v, raw:\n%s", err, outJSON)
	}
	if snap, ok := parsed["snapshot_path"].(string); !ok || !strings.Contains(snap, "novel-bench-") {
		t.Errorf("expected snapshot_path in top level of JSON report, got %v", parsed["snapshot_path"])
	}
	if parsed["project_id"] != proj.ID {
		t.Errorf("expected project_id %s, got %v", proj.ID, parsed["project_id"])
	}

	// JSON 模式后再次断言原库哈希未变
	afterJSONBytes, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("failed to read db file after JSON bench: %v", err)
	}
	if !bytes.Equal(beforeBytes, afterJSONBytes) {
		t.Fatalf("production db was modified during JSON bench!")
	}
}

// TestRunBenchCLI_Cleanup 验证 runBenchCLI 执行完毕后临时快照目录已被正确清理
func TestRunBenchCLI_Cleanup(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "novel.db")

	s, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("failed to init store: %v", err)
	}
	ctx := context.Background()
	proj := &domain.Project{
		ID:                    "proj-cleanup-test",
		Title:                 "清理验证作品",
		TargetPlatform:        "起点仙侠",
		DefaultWordsTarget:    2000,
		DefaultNarrativeStyle: "hardboiled",
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}
	_ = s.SaveProject(ctx, proj)
	_ = s.Close()

	// 运行前记录现存临时目录
	matchesBefore, _ := filepath.Glob(filepath.Join(os.TempDir(), "novel-bench-*"))
	existingDirs := make(map[string]bool)
	for _, m := range matchesBefore {
		existingDirs[m] = true
	}

	// 执行 runBenchCLI
	code := runBenchCLI([]string{"-data-dir", tempDir, "-project", proj.ID}, tempDir)
	if code != 0 {
		t.Fatalf("expected code 0, got %d", code)
	}

	// 运行后检查是否有新产生的 novel-bench-* 目录未被删除
	matchesAfter, _ := filepath.Glob(filepath.Join(os.TempDir(), "novel-bench-*"))
	for _, m := range matchesAfter {
		if !existingDirs[m] {
			t.Fatalf("temporary snapshot directory leaked: %s was not cleaned up", m)
		}
	}
}

// TestRunBenchCLI_DBReadErrorExplicitReport 验证主角或风格库读失败时作为显性项报告
func TestRunBenchCLI_DBReadErrorExplicitReport(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "novel.db")

	s, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("init store failed: %v", err)
	}
	ctx := context.Background()
	proj := &domain.Project{
		ID:                    "proj-err-001",
		Title:                 "错误捕获作品",
		TargetPlatform:        "起点仙侠",
		DefaultWordsTarget:    2000,
		DefaultNarrativeStyle: "hardboiled",
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}
	_ = s.SaveProject(ctx, proj)
	_ = s.Close()

	// 注入破坏性数据：
	// 1. codex_entries 插入非法日期，使 GetProtagonistEntry 在 scan 到 time.Time 时发生解析错误
	// 2. style_vault 插入非法 JSON，使 GetStyleVault 发生 unmarshal 错误
	rawDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open raw db failed: %v", err)
	}
	if _, err := rawDB.Exec(`
		INSERT INTO codex_entries (
			id, project_id, category, name, color_tag, summary, details_markdown, tracking_mode,
			archetype, created_at, updated_at
		) VALUES (
			'entry-bad-prot', 'proj-err-001', 'CHARACTER', '主角', '', '', '', 'AUTOMATIC',
			'PROTAGONIST', 'NOT_A_VALID_TIMESTAMP', 'NOT_A_VALID_TIMESTAMP'
		);
		CREATE TABLE IF NOT EXISTS style_vault (project_id TEXT PRIMARY KEY, golden_excerpts TEXT, updated_at TIMESTAMP);
		INSERT OR REPLACE INTO style_vault (project_id, golden_excerpts, updated_at) VALUES ('proj-err-001', '{bad-json-excerpts}', CURRENT_TIMESTAMP);
	`); err != nil {
		rawDB.Close()
		t.Fatalf("inject corrupted tables failed: %v", err)
	}
	rawDB.Close()

	// 跑文本模式
	outText := captureStdout(t, func() {
		code := runBenchCLI([]string{"-data-dir", tempDir, "-project", proj.ID}, tempDir)
		if code != 0 {
			t.Fatalf("expected bench exit 0 even with partial query errors, got %d", code)
		}
	})

	// 断言显性错误被记录到明细中，且文风契约不再得出"未种子化"结论
	if !strings.Contains(outText, "主角读取失败：") {
		t.Errorf("expected output to contain '主角读取失败：', got:\n%s", outText)
	}
	if !strings.Contains(outText, "风格库读取失败：") {
		t.Errorf("expected output to contain '风格库读取失败：', got:\n%s", outText)
	}
	if strings.Contains(outText, "文风保险库状态:   未种子化") {
		t.Errorf("expected output NOT to conclude '未种子化' on query failure, got:\n%s", outText)
	}

	// 跑 JSON 模式
	outJSON := captureStdout(t, func() {
		code := runBenchCLI([]string{"-data-dir", tempDir, "-project", proj.ID, "-json"}, tempDir)
		if code != 0 {
			t.Fatalf("expected bench JSON exit 0, got %d", code)
		}
	})
	var parsed map[string]any
	if err := json.Unmarshal([]byte(outJSON), &parsed); err != nil {
		t.Fatalf("parse JSON failed: %v", err)
	}
	ledger, ok := parsed["ledger_replay"].(map[string]any)
	if !ok {
		t.Fatalf("ledger_replay missing or wrong type")
	}
	if ledger["clean"] != false {
		t.Errorf("expected ledger_replay.clean to be false when query errors exist, got %v", ledger["clean"])
	}
	drifts, _ := ledger["drifts"].([]any)
	driftsStr := strings.Join([]string{
		fmt.Sprintf("%v", drifts),
	}, " ")
	if !strings.Contains(driftsStr, "主角读取失败：") || !strings.Contains(driftsStr, "风格库读取失败：") {
		t.Errorf("expected drifts to contain both query failures, got %v", drifts)
	}
}

// TestRunBenchCLI_NonExistentDB 验证源数据库不存在时直接报错且不新建空库
func TestRunBenchCLI_NonExistentDB(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "novel.db")

	code := runBenchCLI([]string{"-data-dir", tempDir}, tempDir)
	if code != 1 {
		t.Errorf("expected exit code 1 for non-existent database file, got %d", code)
	}

	// 断言未在目标目录新建空库
	if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
		t.Errorf("expected novel.db NOT to be created on error, but it exists")
	}
}

// TestPrintBenchReport_WithSnapshotPath 验证 printBenchReport 显式携带快照路径时的文本与 JSON 行为
func TestPrintBenchReport_WithSnapshotPath(t *testing.T) {
	report := &engine.BenchReport{
		ProjectID:    "proj-snap-001",
		ProjectTitle: "快照展示书",
		Chapters:     0,
	}

	snapFile := "/tmp/mock-snapshot/novel.db"

	// 文本模式：首行必须是快照路径
	var textBuf bytes.Buffer
	printBenchReport(&textBuf, report, false, snapFile)
	textOut := textBuf.String()
	lines := strings.Split(strings.TrimSpace(textOut), "\n")
	if len(lines) == 0 || lines[0] != "快照路径: "+snapFile {
		t.Errorf("expected first line to be '快照路径: %s', got %q", snapFile, lines[0])
	}

	// JSON 模式：顶层必须存在 snapshot_path 字段
	var jsonBuf bytes.Buffer
	printBenchReport(&jsonBuf, report, true, snapFile)
	var parsed map[string]any
	if err := json.Unmarshal(jsonBuf.Bytes(), &parsed); err != nil {
		t.Fatalf("unmarshal json failed: %v", err)
	}
	if parsed["snapshot_path"] != snapFile {
		t.Errorf("expected snapshot_path %q, got %v", snapFile, parsed["snapshot_path"])
	}
	if parsed["project_id"] != "proj-snap-001" {
		t.Errorf("expected project_id 'proj-snap-001', got %v", parsed["project_id"])
	}
}
