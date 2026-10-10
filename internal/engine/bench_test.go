package engine

import (
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

	report := RunBenchProbes(proj, chapters, nil, nil, false)

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

	report := RunBenchProbes(proj, chapters, nil, nil, false)

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

	report := RunBenchProbes(proj, chapters, hooks, nil, false)

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

// ④ 重放：两章 +5/-3 → 重放数量 2；当前主角数量 5 → 漂移行含 灵石；非法 mutation（消耗未持有）→ Drifts 有记录不 panic；空输入零值
func TestRunBenchProbes_LedgerReplayAndZeroValues(t *testing.T) {
	proj := &domain.Project{
		ID:    "proj-ledger",
		Title: "账本重放测试",
	}

	chapters := []*domain.Chapter{
		{
			ChapterIndex: 1,
			StateMutation: domain.StateMutation{
				InventoryDelta: "+灵石x5",
			},
		},
		{
			ChapterIndex: 2,
			StateMutation: domain.StateMutation{
				InventoryDelta: "-灵石x3",
			},
		},
		{
			ChapterIndex: 3,
			StateMutation: domain.StateMutation{
				InventoryDelta: "-诛仙剑x1", // 非法：消耗未持有的道具
			},
		},
	}

	// 当前主角持有的道具与重放不一致（数量 5 vs 2）
	protagonist := &domain.CodexEntry{
		Name: "顾渊",
		StructuredItems: []domain.InventoryItem{
			{Name: "灵石", Quantity: 5},
		},
	}

	report := RunBenchProbes(proj, chapters, nil, protagonist, false)

	if report.LedgerReplay.Clean {
		t.Errorf("expected Clean=false on drift, got true")
	}

	foundLingshiDrift := false
	foundIllegalWarning := false
	for _, drift := range report.LedgerReplay.Drifts {
		if strings.Contains(drift, "灵石") {
			foundLingshiDrift = true
		}
		if strings.Contains(drift, "诛仙剑") || strings.Contains(drift, "未持有") || strings.Contains(drift, "异常") {
			foundIllegalWarning = true
		}
	}

	if !foundLingshiDrift {
		t.Errorf("expected drift row containing '灵石', got drifts: %v", report.LedgerReplay.Drifts)
	}
	if !foundIllegalWarning {
		t.Errorf("expected drift row recording illegal consumption of 诛仙剑, got: %v", report.LedgerReplay.Drifts)
	}

	// 空输入零值测试（防御性防 panic）
	zeroReport := RunBenchProbes(nil, nil, nil, nil, false)
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

	report := RunBenchProbes(proj, chapters, nil, nil, false)

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
	cleanReport := RunBenchProbes(proj, cleanChapters, nil, nil, false)
	if cleanReport.Echo.TotalPhrases != 0 {
		t.Errorf("expected clean TotalPhrases=0, got %d", cleanReport.Echo.TotalPhrases)
	}
	if cleanReport.Echo.ChaptersHit != 0 {
		t.Errorf("expected clean ChaptersHit=0, got %d", cleanReport.Echo.ChaptersHit)
	}
}
