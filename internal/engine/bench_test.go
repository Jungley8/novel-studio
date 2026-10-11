package engine

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/Jungley8/novel-studio/internal/domain"
)

// ① 字数统计（含 UnderTarget 边界）
func TestRunBenchProbes_WordCountStats(t *testing.T) {
	proj := &domain.Project{
		ID:                 "proj-wordcount",
		Title:              "字数体检测试",
		DefaultWordsTarget: 2000,
	}

	// 80% 及格线为 1600 字。
	// ch1: 2000 字 (及格)
	// ch2: 1600 字 (边界：正好及格，不计入 UnderTarget)
	// ch3: 1599 字 (边界：低于 80%，计入 UnderTarget)
	// ch4: WordCount=0，Content 为 1000 字符 (回退 rune 长度，计入 UnderTarget)
	chapters := []*domain.Chapter{
		{ChapterIndex: 1, WordCount: 2000},
		{ChapterIndex: 2, WordCount: 1600},
		{ChapterIndex: 3, WordCount: 1599},
		{ChapterIndex: 4, WordCount: 0, Content: strings.Repeat("字", 1000)},
	}

	report := RunBenchProbes(proj, chapters, nil, nil, false, nil)

	if report.Chapters != 4 {
		t.Fatalf("expected 4 chapters, got %d", report.Chapters)
	}
	if report.WordCount.Target != 2000 {
		t.Errorf("expected Target=2000, got %d", report.WordCount.Target)
	}
	// Total: 2000 + 1600 + 1599 + 1000 = 6199; Avg = 6199 / 4 = 1549
	if report.WordCount.Avg != 1549 {
		t.Errorf("expected Avg=1549, got %d", report.WordCount.Avg)
	}
	if report.WordCount.Min != 1000 {
		t.Errorf("expected Min=1000, got %d", report.WordCount.Min)
	}
	if report.WordCount.Max != 2000 {
		t.Errorf("expected Max=2000, got %d", report.WordCount.Max)
	}
	if report.WordCount.UnderTarget != 2 {
		t.Errorf("expected UnderTarget=2 (1599 和 1000 字的两章), got %d", report.WordCount.UnderTarget)
	}
}

// ② 4 连 ESCALATION + 1 CLIMAX 的 beats 序列 → MaxConsecutive==4 且 MaxRunChapters 正确
func TestRunBenchProbes_ChapterTypesRhythm(t *testing.T) {
	proj := &domain.Project{
		ID:    "proj-rhythm",
		Title: "节奏体检测试",
	}

	// ESCALATION: 中高张力爬坡 (avgT > 5.0, 未满足 CLIMAX)
	escalationBeats := []domain.SceneBeat{
		{Tension: 6},
		{Tension: 7},
	}
	// CLIMAX: 末拍张力 >= 8 且挂钩子
	climaxBeats := []domain.SceneBeat{
		{Tension: 8, HookType: "CLIFFHANGER"},
	}

	// ch1..ch4: ESCALATION, ch5: CLIMAX, ch6: 无 beats（跳过）
	chapters := []*domain.Chapter{
		{ChapterIndex: 1, Beats: escalationBeats},
		{ChapterIndex: 2, Beats: escalationBeats},
		{ChapterIndex: 3, Beats: escalationBeats},
		{ChapterIndex: 4, Beats: escalationBeats},
		{ChapterIndex: 5, Beats: climaxBeats},
		{ChapterIndex: 6, Beats: nil},
	}

	report := RunBenchProbes(proj, chapters, nil, nil, false, nil)

	if report.ChapterTypes.MaxConsecutive != 4 {
		t.Errorf("expected MaxConsecutive=4, got %d", report.ChapterTypes.MaxConsecutive)
	}
	expectedRun := []int{1, 2, 3, 4}
	if !reflect.DeepEqual(report.ChapterTypes.MaxRunChapters, expectedRun) {
		t.Errorf("expected MaxRunChapters=%v, got %v", expectedRun, report.ChapterTypes.MaxRunChapters)
	}
	if report.ChapterTypes.Distribution["ESCALATION"] != 4 {
		t.Errorf("expected 4 ESCALATION, got %d", report.ChapterTypes.Distribution["ESCALATION"])
	}
	if report.ChapterTypes.Distribution["CLIMAX"] != 1 {
		t.Errorf("expected 1 CLIMAX, got %d", report.ChapterTypes.Distribution["CLIMAX"])
	}
}

// ③ 伏笔逾期边界（TargetChapter == max 不算逾期，max-1 算）与四类计数
func TestRunBenchProbes_HooksOverdueAndCounts(t *testing.T) {
	proj := &domain.Project{
		ID:    "proj-hooks",
		Title: "伏笔体检测试",
	}

	// 最大归档章号为 10
	chapters := []*domain.Chapter{
		{ChapterIndex: 1},
		{ChapterIndex: 5},
		{ChapterIndex: 10},
	}

	hooks := []*domain.PlotHook{
		// TargetChapter == 10: 等于最大已归档章号，不算逾期
		{ID: "h-open-boundary", Title: "本章待解线索", Status: domain.HookStatusOpen, TargetChapter: 10},
		// TargetChapter == 9 (max - 1): 算逾期
		{ID: "h-open-overdue", Title: "失落的古剑", Status: domain.HookStatusOpen, TargetChapter: 9},
		// TargetChapter == 8: 算逾期
		{ID: "h-ferm-overdue", Title: "身世秘辛", Status: domain.HookStatusFermenting, TargetChapter: 8},
		// 已回收，虽 TargetChapter < 10 也不算逾期
		{ID: "h-resolved", Title: "宗门大比伏笔", Status: domain.HookStatusResolved, TargetChapter: 5},
		// 已废弃，虽 TargetChapter < 10 也不算逾期
		{ID: "h-abandoned", Title: "弃用暗线", Status: domain.HookStatusAbandoned, TargetChapter: 4},
	}

	report := RunBenchProbes(proj, chapters, hooks, nil, false, nil)

	if report.Hooks.Open != 2 {
		t.Errorf("expected Open=2, got %d", report.Hooks.Open)
	}
	if report.Hooks.Fermenting != 1 {
		t.Errorf("expected Fermenting=1, got %d", report.Hooks.Fermenting)
	}
	if report.Hooks.Resolved != 1 {
		t.Errorf("expected Resolved=1, got %d", report.Hooks.Resolved)
	}
	if report.Hooks.Abandoned != 1 {
		t.Errorf("expected Abandoned=1, got %d", report.Hooks.Abandoned)
	}

	if len(report.Hooks.Overdue) != 2 {
		t.Fatalf("expected 2 overdue hooks, got %d", len(report.Hooks.Overdue))
	}
	if report.Hooks.Overdue[0].ID != "h-open-overdue" || report.Hooks.Overdue[0].TargetChapter != 9 {
		t.Errorf("expected overdue hook h-open-overdue, got %+v", report.Hooks.Overdue[0])
	}
	if report.Hooks.Overdue[1].ID != "h-ferm-overdue" || report.Hooks.Overdue[1].TargetChapter != 8 {
		t.Errorf("expected overdue hook h-ferm-overdue, got %+v", report.Hooks.Overdue[1])
	}
}

// 快照链测试共用 helper：在副本上应用 mutation 并封成快照（模拟 CommitChapter 写入语义）。
func benchApplyCopy(t *testing.T, base *domain.CodexEntry, m domain.StateMutation) *domain.CodexEntry {
	t.Helper()
	data, err := json.Marshal(base)
	if err != nil {
		t.Fatalf("marshal base protagonist failed: %v", err)
	}
	var cp domain.CodexEntry
	if err := json.Unmarshal(data, &cp); err != nil {
		t.Fatalf("unmarshal base protagonist failed: %v", err)
	}
	if _, err := domain.ApplyStateMutation(&cp, m); err != nil {
		t.Fatalf("ApplyStateMutation failed: %v", err)
	}
	return &cp
}

func benchSnapOf(idx int, e *domain.CodexEntry) *domain.ChapterStateSnapshot {
	b, err := json.Marshal(e)
	if err != nil {
		panic(fmt.Sprintf("marshal snapshot protagonist failed: %v", err))
	}
	return &domain.ChapterStateSnapshot{
		ID:              fmt.Sprintf("snap-bench-%d", idx),
		ProjectID:       "proj-ledger",
		ChapterIndex:    idx,
		ProtagonistJSON: string(b),
		HooksJSON:       "[]",
	}
}

// 三章快照链固件：base(灵石x2/炼气) → ch1(+灵石x5) → ch2(-灵石x3) → ch3(突破筑基)。
// 返回 chapters、snapshots(1..3)、终态 protagonist（== snap3）。
func benchSnapshotChainFixture(t *testing.T) ([]*domain.Chapter, []*domain.ChapterStateSnapshot, *domain.CodexEntry) {
	t.Helper()
	base := &domain.CodexEntry{
		ID:              "prot-bench",
		ProjectID:       "proj-ledger",
		Name:            "顾渊",
		Realm:           "炼气",
		StructuredLevel: &domain.PowerLevel{Realm: "炼气"},
		StructuredItems: []domain.InventoryItem{{Name: "灵石", Quantity: 2}},
	}
	mut1 := domain.StateMutation{InventoryDelta: "+灵石x5"}
	mut2 := domain.StateMutation{InventoryDelta: "-灵石x3"}
	mut3 := domain.StateMutation{PowerDelta: "突破筑基"}

	s1 := benchApplyCopy(t, base, mut1)
	s2 := benchApplyCopy(t, s1, mut2)
	s3 := benchApplyCopy(t, s2, mut3)

	chapters := []*domain.Chapter{
		{ChapterIndex: 1, StateMutation: mut1},
		{ChapterIndex: 2, StateMutation: mut2},
		{ChapterIndex: 3, StateMutation: mut3},
	}
	snapshots := []*domain.ChapterStateSnapshot{benchSnapOf(1, s1), benchSnapOf(2, s2), benchSnapOf(3, s3)}
	return chapters, snapshots, s3
}

// ④(改写，快照链语义)：三章过渡精确一致 + 终态一致 → Clean、无漂移；
// 第 1 章无前置快照仅记 Notes（旧"空基线重放"期望已按新语义改写：创世种子与 REMOVE 不再是系统性假阳性）。
func TestRunBenchProbes_SnapshotChainClean(t *testing.T) {
	proj := &domain.Project{ID: "proj-ledger", Title: "账本重放测试"}
	chapters, snapshots, protagonist := benchSnapshotChainFixture(t)

	report := RunBenchProbes(proj, chapters, nil, protagonist, false, snapshots)

	if !report.LedgerReplay.Clean {
		t.Errorf("expected Clean=true on exact snapshot chain, got drifts: %v", report.LedgerReplay.Drifts)
	}
	if len(report.LedgerReplay.Drifts) != 0 {
		t.Errorf("expected no drifts, got %v", report.LedgerReplay.Drifts)
	}
	foundCh1Note := false
	for _, n := range report.LedgerReplay.Notes {
		if strings.Contains(n, "第1章缺前置/后置快照") {
			foundCh1Note = true
		}
	}
	if !foundCh1Note {
		t.Errorf("expected Notes to record ch1 skip (no snapshot[0]), got %v", report.LedgerReplay.Notes)
	}

	// 空输入零值测试（防御性防 panic）
	zeroReport := RunBenchProbes(nil, nil, nil, nil, false, nil)
	if zeroReport == nil {
		t.Fatal("expected non-nil report on nil inputs")
	}
	if zeroReport.Chapters != 0 {
		t.Errorf("expected 0 chapters, got %d", zeroReport.Chapters)
	}
	if !zeroReport.LedgerReplay.Clean {
		t.Errorf("expected Clean=true when protagonist is nil, got false")
	}
	if !strings.Contains(zeroReport.LedgerReplay.Caveat, "无主角实体，跳过") {
		t.Errorf("expected caveat to contain '无主角实体，跳过', got %q", zeroReport.LedgerReplay.Caveat)
	}
}

// 篡改检出：改 snap[3] 后态（灵石 4→99），终态 protagonist 保持被篡改态以隔离出过渡校验 → 第3章过渡漂移。
func TestRunBenchProbes_SnapshotTamperDetected(t *testing.T) {
	proj := &domain.Project{ID: "proj-ledger", Title: "账本篡改测试"}
	chapters, snapshots, _ := benchSnapshotChainFixture(t)

	var tampered domain.CodexEntry
	if err := json.Unmarshal([]byte(snapshots[2].ProtagonistJSON), &tampered); err != nil {
		t.Fatalf("unmarshal snap3 failed: %v", err)
	}
	for i := range tampered.StructuredItems {
		if tampered.StructuredItems[i].Name == "灵石" {
			tampered.StructuredItems[i].Quantity = 99
		}
	}
	tamperedBytes, _ := json.Marshal(&tampered)
	snapshots[2] = &domain.ChapterStateSnapshot{
		ID:              snapshots[2].ID,
		ProjectID:       snapshots[2].ProjectID,
		ChapterIndex:    3,
		ProtagonistJSON: string(tamperedBytes),
		HooksJSON:       "[]",
	}
	// 终态 protagonist 取被篡改态：终态校验通过，只有第3章过渡校验应漂移。
	protagonist := &tampered

	report := RunBenchProbes(proj, chapters, nil, protagonist, false, snapshots)

	if report.LedgerReplay.Clean {
		t.Errorf("expected Clean=false on tampered snapshot, got true")
	}
	foundCh3Drift := false
	for _, d := range report.LedgerReplay.Drifts {
		if strings.Contains(d, "第3章") && strings.Contains(d, "灵石") {
			foundCh3Drift = true
		}
	}
	if !foundCh3Drift {
		t.Errorf("expected drift row for ch3 灵石 mismatch, got %v", report.LedgerReplay.Drifts)
	}
}

// 空快照：nil 快照 + 有 mutation 章 + 主角非空 → Clean=false + Notes 明示"无快照数据，无法验证"；
// 零章 / 零 mutation 章 → Clean=true + Notes。
func TestRunBenchProbes_EmptySnapshotsNotClean(t *testing.T) {
	proj := &domain.Project{ID: "proj-ledger", Title: "空快照测试"}
	chapters := []*domain.Chapter{
		{ChapterIndex: 1, StateMutation: domain.StateMutation{InventoryDelta: "+灵石x5"}},
		{ChapterIndex: 2, StateMutation: domain.StateMutation{InventoryDelta: "-灵石x3"}},
	}
	protagonist := &domain.CodexEntry{
		Name:            "顾渊",
		StructuredItems: []domain.InventoryItem{{Name: "灵石", Quantity: 5}},
	}

	report := RunBenchProbes(proj, chapters, nil, protagonist, false, nil)
	if report.LedgerReplay.Clean {
		t.Errorf("expected Clean=false with nil snapshots and mutation chapters, got true")
	}
	foundNoSnapNote := false
	for _, n := range report.LedgerReplay.Notes {
		if strings.Contains(n, "无快照数据，无法验证") {
			foundNoSnapNote = true
		}
	}
	if !foundNoSnapNote {
		t.Errorf("expected Notes to contain '无快照数据，无法验证', got %v", report.LedgerReplay.Notes)
	}
	if len(report.LedgerReplay.Drifts) != 0 {
		t.Errorf("expected no drifts on skip path, got %v", report.LedgerReplay.Drifts)
	}

	// 零章 + 主角非空 → Clean=true + Notes
	emptyChapters := RunBenchProbes(proj, nil, nil, protagonist, false, nil)
	if !emptyChapters.LedgerReplay.Clean {
		t.Errorf("expected Clean=true for zero chapters, got false (notes=%v)", emptyChapters.LedgerReplay.Notes)
	}
	if len(emptyChapters.LedgerReplay.Notes) == 0 {
		t.Errorf("expected Notes for zero chapters, got none")
	}

	// 零 mutation 章 + 主角非空 → Clean=true + Notes
	plainChapters := []*domain.Chapter{{ChapterIndex: 1}, {ChapterIndex: 2}}
	noMut := RunBenchProbes(proj, plainChapters, nil, protagonist, false, nil)
	if !noMut.LedgerReplay.Clean {
		t.Errorf("expected Clean=true for zero-mutation chapters, got false")
	}
	if len(noMut.LedgerReplay.Notes) == 0 {
		t.Errorf("expected Notes for zero-mutation chapters, got none")
	}
}

// 撤回残留：两章快照俱在但正史只剩 ch1（ch2 已撤回，主角恢复至 ch1 态），
// 孤儿 snap[2] 超出正史前沿 → 忽略记 Notes，不报 drift，Clean。
func TestRunBenchProbes_OrphanSnapshotBeyondFrontier(t *testing.T) {
	proj := &domain.Project{ID: "proj-ledger", Title: "撤回残留测试"}
	chapters, snapshots, _ := benchSnapshotChainFixture(t)

	// 正史前沿退到 ch1：只测 ch1；主角为 snap[1] 解码态（UncommitChapter 恢复态）。
	chapters = chapters[:1]
	var restored domain.CodexEntry
	if err := json.Unmarshal([]byte(snapshots[0].ProtagonistJSON), &restored); err != nil {
		t.Fatalf("unmarshal snap1 failed: %v", err)
	}

	report := RunBenchProbes(proj, chapters, nil, &restored, false, snapshots)

	if !report.LedgerReplay.Clean {
		t.Errorf("expected Clean=true with orphan snapshot ignored, got drifts: %v", report.LedgerReplay.Drifts)
	}
	if len(report.LedgerReplay.Drifts) != 0 {
		t.Errorf("expected no drifts, got %v", report.LedgerReplay.Drifts)
	}
	foundFrontierNote := false
	for _, n := range report.LedgerReplay.Notes {
		if strings.Contains(n, "第2章快照超出正史前沿") {
			foundFrontierNote = true
		}
	}
	if !foundFrontierNote {
		t.Errorf("expected frontier note for snap[2], got %v", report.LedgerReplay.Notes)
	}
}

// 终态校验：当前 protagonist 与末章快照不一致（模拟界面直接修改）→ drift 含"与末章快照不一致"。
func TestRunBenchProbes_FinalStateDrift(t *testing.T) {
	proj := &domain.Project{ID: "proj-ledger", Title: "终态校验测试"}
	chapters, snapshots, protagonist := benchSnapshotChainFixture(t)

	protagonist.StructuredItems = []domain.InventoryItem{{Name: "灵石", Quantity: 100}}

	report := RunBenchProbes(proj, chapters, nil, protagonist, false, snapshots)

	if report.LedgerReplay.Clean {
		t.Errorf("expected Clean=false on final-state drift, got true")
	}
	foundFinal := false
	for _, d := range report.LedgerReplay.Drifts {
		if strings.Contains(d, "与末章快照不一致") {
			foundFinal = true
		}
	}
	if !foundFinal {
		t.Errorf("expected drift containing '与末章快照不一致', got %v", report.LedgerReplay.Drifts)
	}
}

// Lint 节：只读存量字段计数（LinterPassed 通过率 + BurstinessScore 均值 + Review 覆盖）。
func TestRunBenchProbes_LintSection(t *testing.T) {
	proj := &domain.Project{ID: "proj-lint", Title: "Lint 体检测试"}
	chapters := []*domain.Chapter{
		{ChapterIndex: 1, LinterPassed: true, BurstinessScore: 80, Review: &domain.ReviewResult{Verdict: domain.ReviewVerdictAccepted, Score: 90}},
		{ChapterIndex: 2, LinterPassed: false, BurstinessScore: 60},
		{ChapterIndex: 3, LinterPassed: true, BurstinessScore: 70, Review: &domain.ReviewResult{Verdict: domain.ReviewVerdictRevision, Score: 70}},
	}

	report := RunBenchProbes(proj, chapters, nil, nil, false, nil)

	if report.Lint.Chapters != 3 {
		t.Errorf("expected Lint.Chapters=3, got %d", report.Lint.Chapters)
	}
	if report.Lint.LinterPassed != 2 {
		t.Errorf("expected Lint.LinterPassed=2, got %d", report.Lint.LinterPassed)
	}
	if want, got := 2.0/3.0, report.Lint.LinterPassRate; got != want {
		t.Errorf("expected Lint.LinterPassRate=%v, got %v", want, got)
	}
	if want, got := 70.0, report.Lint.AvgBurstinessScore; got != want {
		t.Errorf("expected Lint.AvgBurstinessScore=%v, got %v", want, got)
	}
	if report.Lint.ReviewCovered != 2 {
		t.Errorf("expected Lint.ReviewCovered=2, got %d", report.Lint.ReviewCovered)
	}
}

// ⑤ 回声：第 4 章含前 3 章共现短语 → Total ≥1、ChaptersHit==1；干净序列 → 0
func TestRunBenchProbes_EchoProbes(t *testing.T) {
	proj := &domain.Project{
		ID:    "proj-echo",
		Title: "回声体检测试",
	}

	// DetectEchoPhrases 判定条件：前序 >= 3 章中出现，且出现占比 <= 50%（排除 >50% 通用词）。
	// 构建 6 章前序历史（1..6）：其中 3 章含有「寒风如刀」（3/6 = 50% 刚好命中），
	// 其余 3 章为平铺叙事；第 7 章（作为当前章）含有「寒风如刀」。
	chapters := []*domain.Chapter{
		{ChapterIndex: 1, Content: "修士漫步雪原之上，寒风如刀刺入肌理。"},
		{ChapterIndex: 2, Content: "寻常叙事，山川风物，修士静坐吐纳。"},
		{ChapterIndex: 3, Content: "山巅极寒之境，寒风如刀凛冽彻骨。"},
		{ChapterIndex: 4, Content: "寻常叙事，各方势力暗流涌动。"},
		{ChapterIndex: 5, Content: "冰谷深渊之下，寒风如刀割裂虚空。"},
		{ChapterIndex: 6, Content: "寻常叙事，灵泉流淌仙雾缭绕。"},
		{ChapterIndex: 7, Content: "风雪漫天席卷，寒风如刀。顾渊负手而立，冷眼看着敌人。"},
	}

	report := RunBenchProbes(proj, chapters, nil, nil, false, nil)

	if report.Echo.TotalPhrases < 1 {
		t.Errorf("expected TotalPhrases >= 1, got %d", report.Echo.TotalPhrases)
	}
	if report.Echo.ChaptersHit != 1 {
		t.Errorf("expected ChaptersHit == 1, got %d", report.Echo.ChaptersHit)
	}

	// 干净序列测试：各章无跨章重复 4-gram
	cleanChapters := []*domain.Chapter{
		{ChapterIndex: 1, Content: "甲乙丙丁戊己庚辛壬癸子丑寅卯辰巳午未申酉戌亥。"},
		{ChapterIndex: 2, Content: "赵钱孙李周吴郑王冯陈褚卫蒋沈韩杨朱秦尤许何吕。"},
		{ChapterIndex: 3, Content: "天地玄黄宇宙洪荒日月盈昃辰宿列张寒来暑往秋收。"},
		{ChapterIndex: 4, Content: "金生丽水玉出昆冈剑号巨阙珠称夜光果珍李柰菜重。"},
	}
	cleanReport := RunBenchProbes(proj, cleanChapters, nil, nil, false, nil)
	if cleanReport.Echo.TotalPhrases != 0 {
		t.Errorf("expected clean TotalPhrases=0, got %d", cleanReport.Echo.TotalPhrases)
	}
	if cleanReport.Echo.ChaptersHit != 0 {
		t.Errorf("expected clean ChaptersHit=0, got %d", cleanReport.Echo.ChaptersHit)
	}
}
