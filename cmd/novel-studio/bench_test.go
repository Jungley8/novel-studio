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

// TestOpenBenchSnapshot 验证真实 SQLite 快照逻辑等价性、清理逻辑与不存在文件报错
func TestOpenBenchSnapshot(t *testing.T) {
	// 1. 不存在的文件应该报错
	_, _, err := openBenchSnapshot(filepath.Join(t.TempDir(), "non-existent.db"))
	if err == nil {
		t.Fatalf("expected error for non-existent source, got nil")
	}

	// 2. 真实 SQLite fixture 逻辑等价性与清理验证
	tempDir := t.TempDir()
	srcPath := filepath.Join(tempDir, "novel.db")
	s, err := store.NewSQLiteStore(srcPath)
	if err != nil {
		t.Fatalf("failed to init store: %v", err)
	}

	ctx := context.Background()
	proj := &domain.Project{
		ID:                    "proj-snap-fixture",
		Title:                 "真实快照作品",
		TargetPlatform:        "起点仙侠",
		DefaultWordsTarget:    2000,
		DefaultNarrativeStyle: "hardboiled",
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}
	if err := s.SaveProject(ctx, proj); err != nil {
		_ = s.Close()
		t.Fatalf("failed to save project: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("failed to close store: %v", err)
	}

	snapPath, cleanup, err := openBenchSnapshot(srcPath)
	if err != nil {
		t.Fatalf("openBenchSnapshot failed: %v", err)
	}
	if snapPath == "" {
		t.Fatalf("expected non-empty snapshot path")
	}

	// 断言快照中项目可读且逻辑等价
	snapStore, err := store.NewSQLiteStore(snapPath)
	if err != nil {
		t.Fatalf("failed to open snapshot store: %v", err)
	}
	snapProj, err := snapStore.GetProject(ctx, proj.ID)
	_ = snapStore.Close()
	if err != nil {
		t.Fatalf("failed to get project from snapshot: %v", err)
	}
	if snapProj == nil || snapProj.Title != proj.Title {
		t.Errorf("expected project title %q, got %v", proj.Title, snapProj)
	}

	// 执行 cleanup 并断言临时目录被清理
	snapDir := filepath.Dir(snapPath)
	cleanup()
	if _, err := os.Stat(snapDir); !os.IsNotExist(err) {
		t.Errorf("expected snapshot directory %s to be removed by cleanup, got err: %v", snapDir, err)
	}
}

// TestOpenBenchSnapshot_SeesUncheckpointedWAL 验证未 checkpoint 的 WAL 帧在快照中可见，且源库零写入
func TestOpenBenchSnapshot_SeesUncheckpointedWAL(t *testing.T) {
	tempDir := t.TempDir()
	src := filepath.Join(tempDir, "novel.db")

	s, err := store.NewSQLiteStore(src)
	if err != nil {
		t.Fatalf("failed to init store: %v", err)
	}
	defer s.Close()

	ctx := context.Background()
	proj := &domain.Project{
		ID:                    "proj-wal-001",
		Title:                 "WAL未检查点作品",
		TargetPlatform:        "起点仙侠",
		DefaultWordsTarget:    2000,
		DefaultNarrativeStyle: "hardboiled",
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}
	if err := s.SaveProject(ctx, proj); err != nil {
		t.Fatalf("save project failed: %v", err)
	}

	uniqueContent := "UNIQUE-WAL-UNCHECKPOINTED-PAYLOAD-9876543210"
	ch := &domain.Chapter{
		ID:           "ch-wal-001",
		ProjectID:    proj.ID,
		ChapterIndex: 1,
		Title:        "第一章：雪夜长剑",
		Content:      uniqueContent,
		WordCount:    2100,
		Beats: []domain.SceneBeat{
			{Phase: "绝地反杀", Tension: 9, Action: "拔剑斩敌"},
		},
		CreatedAt: time.Now(),
	}
	if err := s.SaveChapter(ctx, ch); err != nil {
		t.Fatalf("save chapter failed: %v", err)
	}

	// 断言 src+"-wal" 存在且 Size() > 0（确保场景真的是未 checkpoint 的 WAL）
	walPath := src + "-wal"
	walFi, err := os.Stat(walPath)
	if err != nil {
		t.Fatalf("expected WAL file to exist, got err: %v", err)
	}
	if walFi.Size() <= 0 {
		t.Fatalf("expected WAL file size > 0, got %d", walFi.Size())
	}

	// 记录跑前源目录文件名集合与源文件字节
	entriesBefore, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("failed to read dir before snapshot: %v", err)
	}
	filesBefore := make(map[string]bool)
	for _, e := range entriesBefore {
		filesBefore[e.Name()] = true
	}

	dbBytesBefore, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("failed to read source db before snapshot: %v", err)
	}
	walBytesBefore, err := os.ReadFile(walPath)
	if err != nil {
		t.Fatalf("failed to read source wal before snapshot: %v", err)
	}

	// 生成快照
	snapPath, cleanup, err := openBenchSnapshot(src)
	if err != nil {
		t.Fatalf("openBenchSnapshot failed: %v", err)
	}
	defer cleanup()

	// 用 NewSQLiteStore 打开快照并断言章节与其唯一内容可见
	snapStore, err := store.NewSQLiteStore(snapPath)
	if err != nil {
		t.Fatalf("failed to open snapshot store: %v", err)
	}
	gotCh, err := snapStore.GetChapter(ctx, ch.ProjectID, ch.ChapterIndex)
	_ = snapStore.Close()
	if err != nil {
		t.Fatalf("failed to get chapter from snapshot: %v", err)
	}
	if gotCh == nil || gotCh.Content != uniqueContent {
		t.Fatalf("expected chapter content %q in snapshot, got %+v", uniqueContent, gotCh)
	}

	// 再读源两文件字节（after）→ 与 before 逐字节相等
	// 注：-shm 是读标记共享内存、天然可能变化，不应纳入断言
	dbBytesAfter, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("failed to read source db after snapshot: %v", err)
	}
	if !bytes.Equal(dbBytesBefore, dbBytesAfter) {
		t.Fatalf("source db bytes modified during snapshot backup!")
	}

	walBytesAfter, err := os.ReadFile(walPath)
	if err != nil {
		t.Fatalf("failed to read source wal after snapshot: %v", err)
	}
	if !bytes.Equal(walBytesBefore, walBytesAfter) {
		t.Fatalf("source wal bytes modified during snapshot backup!")
	}

	// 记录源目录文件名集合（跑前/跑后）→ 断言无新增文件（不许 -wal/-shm 等落在数据目录）
	entriesAfter, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("failed to read dir after snapshot: %v", err)
	}
	for _, e := range entriesAfter {
		if !filesBefore[e.Name()] {
			t.Errorf("unexpected new file in source data directory after snapshot: %s", e.Name())
		}
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

// TestRunBenchCLI_SnapshotChainClean 验证真实 CommitChapter 写入的快照链下 bench 报 CLEAN，
// 且 Lint 节随报告输出（含既有快照隔离模式：原库哈希跑后不变）。
func TestRunBenchCLI_SnapshotChainClean(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "novel.db")

	s, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("failed to init store: %v", err)
	}
	ctx := context.Background()
	proj := &domain.Project{
		ID:                    "proj-bench-chain",
		Title:                 "快照链体检书",
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
	if err := s.SaveCodexEntry(ctx, &domain.CodexEntry{
		ID:        "codex_proj-bench-chain_pro",
		ProjectID: proj.ID,
		Category:  domain.CategoryCharacter,
		Archetype: "PROTAGONIST",
		Name:      "顾渊",
		Realm:     "炼气",
	}); err != nil {
		_ = s.Close()
		t.Fatalf("save protagonist failed: %v", err)
	}

	ch1 := &domain.Chapter{
		ID:           "ch-chain-1",
		ProjectID:    proj.ID,
		ChapterIndex: 1,
		Title:        "第一章：雪夜得宝",
		Content:      "大雪漫天，顾渊于雪原之中拾得灵石五枚。",
		StateMutation: domain.StateMutation{
			InventoryDelta: "+灵石x5",
		},
		CreatedAt: time.Now(),
	}
	if _, err := s.CommitChapter(ctx, proj.ID, ch1); err != nil {
		_ = s.Close()
		t.Fatalf("commit chapter 1 failed: %v", err)
	}
	ch2 := &domain.Chapter{
		ID:           "ch-chain-2",
		ProjectID:    proj.ID,
		ChapterIndex: 2,
		Title:        "第二章：市集换物",
		Content:      "顾渊于市集中耗去灵石三枚，换得一卷残缺剑谱。",
		StateMutation: domain.StateMutation{
			InventoryDelta: "-灵石x3",
		},
		CreatedAt: time.Now(),
	}
	if _, err := s.CommitChapter(ctx, proj.ID, ch2); err != nil {
		_ = s.Close()
		t.Fatalf("commit chapter 2 failed: %v", err)
	}

	// 快照链供给直查：两章快照俱在
	snaps, err := s.ListStateSnapshots(ctx, proj.ID)
	if err != nil || len(snaps) != 2 {
		_ = s.Close()
		t.Fatalf("expected 2 state snapshots, got %d, err=%v", len(snaps), err)
	}
	_ = s.Close()

	beforeBytes, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("failed to read db file before bench: %v", err)
	}

	// JSON 模式：ledger_replay.clean 必须为 true，lint 节存在
	var code int
	outJSON := captureStdout(t, func() {
		code = runBenchCLI([]string{"-data-dir", tempDir, "-project", proj.ID, "-json"}, tempDir)
	})
	if code != 0 {
		t.Fatalf("expected bench exit code 0, got %d", code)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(outJSON), &parsed); err != nil {
		t.Fatalf("parse JSON failed: %v, raw:\n%s", err, outJSON)
	}
	ledger, ok := parsed["ledger_replay"].(map[string]any)
	if !ok {
		t.Fatalf("ledger_replay missing or wrong type")
	}
	if ledger["clean"] != true {
		t.Errorf("expected ledger_replay.clean=true on exact snapshot chain, got %v (drifts=%v notes=%v)",
			ledger["clean"], ledger["drifts"], ledger["notes"])
	}
	notes, _ := ledger["notes"].([]any)
	foundCh1Note := false
	for _, n := range notes {
		if strings.Contains(fmt.Sprintf("%v", n), "第1章缺前置/后置快照") {
			foundCh1Note = true
		}
	}
	if !foundCh1Note {
		t.Errorf("expected notes to record ch1 skip, got %v", notes)
	}
	lint, ok := parsed["lint"].(map[string]any)
	if !ok {
		t.Fatalf("lint section missing in JSON report")
	}
	if lint["chapters"] != float64(2) {
		t.Errorf("expected lint.chapters=2, got %v", lint["chapters"])
	}

	// 文本模式：结论纯净 + Lint 节标题 + 原库哈希不变
	var codeText int
	outText := captureStdout(t, func() {
		codeText = runBenchCLI([]string{"-data-dir", tempDir, "-project", proj.ID}, tempDir)
	})
	if codeText != 0 {
		t.Fatalf("expected bench text exit code 0, got %d", codeText)
	}
	if !strings.Contains(outText, "纯净 (CLEAN)") {
		t.Errorf("expected text output to conclude '纯净 (CLEAN)', got:\n%s", outText)
	}
	if !strings.Contains(outText, "Lint 存量体检") {
		t.Errorf("expected text output to contain 'Lint 存量体检' section, got:\n%s", outText)
	}

	afterBytes, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("failed to read db file after bench: %v", err)
	}
	if !bytes.Equal(beforeBytes, afterBytes) {
		t.Fatalf("production db was modified during bench!")
	}
}

// TestRunBenchCLI_DirectEditDrift 验证绕过 CommitChapter 直接改主角（模拟界面直接修改）后，
// bench 终态校验报 drift（含"与末章快照不一致"）。
func TestRunBenchCLI_DirectEditDrift(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "novel.db")

	s, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("failed to init store: %v", err)
	}
	ctx := context.Background()
	proj := &domain.Project{
		ID:                    "proj-bench-drift",
		Title:                 "直接修改检出书",
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
	if err := s.SaveCodexEntry(ctx, &domain.CodexEntry{
		ID:        "codex_proj-bench-drift_pro",
		ProjectID: proj.ID,
		Category:  domain.CategoryCharacter,
		Archetype: "PROTAGONIST",
		Name:      "顾渊",
		Realm:     "炼气",
	}); err != nil {
		_ = s.Close()
		t.Fatalf("save protagonist failed: %v", err)
	}
	ch1 := &domain.Chapter{
		ID:           "ch-drift-1",
		ProjectID:    proj.ID,
		ChapterIndex: 1,
		Title:        "第一章",
		Content:      "顾渊拾得灵石五枚。",
		StateMutation: domain.StateMutation{
			InventoryDelta: "+灵石x5",
		},
		CreatedAt: time.Now(),
	}
	if _, err := s.CommitChapter(ctx, proj.ID, ch1); err != nil {
		_ = s.Close()
		t.Fatalf("commit chapter 1 failed: %v", err)
	}
	// 模拟界面直接修改：绕过 CommitChapter 重写主角背包
	prot, err := s.GetProtagonistEntry(ctx, proj.ID)
	if err != nil || prot == nil {
		_ = s.Close()
		t.Fatalf("get protagonist failed: prot=%v, err=%v", prot, err)
	}
	prot.StructuredItems = []domain.InventoryItem{{Name: "灵石", Quantity: 100}}
	if err := s.SaveCodexEntry(ctx, prot); err != nil {
		_ = s.Close()
		t.Fatalf("direct edit protagonist failed: %v", err)
	}
	_ = s.Close()

	var code int
	outJSON := captureStdout(t, func() {
		code = runBenchCLI([]string{"-data-dir", tempDir, "-project", proj.ID, "-json"}, tempDir)
	})
	if code != 0 {
		t.Fatalf("expected bench exit code 0, got %d", code)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(outJSON), &parsed); err != nil {
		t.Fatalf("parse JSON failed: %v, raw:\n%s", err, outJSON)
	}
	ledger, ok := parsed["ledger_replay"].(map[string]any)
	if !ok {
		t.Fatalf("ledger_replay missing or wrong type")
	}
	if ledger["clean"] != false {
		t.Errorf("expected ledger_replay.clean=false after direct edit, got %v", ledger["clean"])
	}
	drifts, _ := ledger["drifts"].([]any)
	foundFinal := false
	for _, d := range drifts {
		if strings.Contains(fmt.Sprintf("%v", d), "与末章快照不一致") {
			foundFinal = true
		}
	}
	if !foundFinal {
		t.Errorf("expected drift containing '与末章快照不一致', got %v", drifts)
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
