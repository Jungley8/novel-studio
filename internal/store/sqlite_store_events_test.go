package store_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Jungley8/novel-studio/internal/activity"
	"github.com/Jungley8/novel-studio/internal/domain"
	"github.com/Jungley8/novel-studio/internal/store"
)

func newEventTestStore(t *testing.T) store.Store {
	t.Helper()
	s, err := store.NewSQLiteStore(filepath.Join(t.TempDir(), "events.db"))
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func seedEventProject(t *testing.T, s store.Store, ctx context.Context) {
	t.Helper()
	p := &domain.Project{
		ID:             "proj-evt",
		Title:          "账本验证录",
		TargetPlatform: "起点仙侠",
		WorldRules:     "剑气不得离体三丈",
	}
	if err := s.SaveProject(ctx, p); err != nil {
		t.Fatalf("seed SaveProject failed: %v", err)
	}
}

func TestEvents_ProjectCreatedThenUpdated(t *testing.T) {
	ctx := context.Background()
	s := newEventTestStore(t)
	seedEventProject(t, s, ctx)

	list, next, err := s.ListEvents(ctx, activity.EventQuery{ProjectID: "proj-evt"})
	if err != nil {
		t.Fatalf("ListEvents failed: %v", err)
	}
	if next != 0 {
		t.Fatalf("expected no next cursor for single event, got %d", next)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 event, got %d", len(list))
	}
	if list[0].EventType != activity.EventProjectCreated {
		t.Fatalf("expected project.created, got %s", list[0].EventType)
	}
	if list[0].Seq != 1 {
		t.Fatalf("expected seq 1, got %d", list[0].Seq)
	}
	if list[0].Before != "" {
		t.Fatalf("created event must have empty Before in list view")
	}

	// Update triggers project.updated with a new seq.
	p, _ := s.GetProject(ctx, "proj-evt")
	p.WorldRules = "剑气不得离体十丈"
	if err := s.SaveProject(ctx, p); err != nil {
		t.Fatalf("SaveProject update failed: %v", err)
	}
	list, _, err = s.ListEvents(ctx, activity.EventQuery{ProjectID: "proj-evt"})
	if err != nil {
		t.Fatalf("ListEvents failed: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 events, got %d", len(list))
	}
	if list[0].EventType != activity.EventProjectUpdated || list[0].Seq != 2 {
		t.Fatalf("expected newest project.updated seq=2, got %s seq=%d", list[0].EventType, list[0].Seq)
	}
	found := false
	for _, f := range list[0].FieldsChanged {
		if f == "world_rules" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected fields_changed to contain world_rules, got %v", list[0].FieldsChanged)
	}

	// Detail view decodes the full before snapshot.
	detail, err := s.GetEvent(ctx, "proj-evt", list[0].ID)
	if err != nil {
		t.Fatalf("GetEvent failed: %v", err)
	}
	if !strings.Contains(detail.Before, "三丈") || !strings.Contains(detail.After, "十丈") {
		t.Fatalf("detail must carry full before/after snapshots")
	}
}

func TestEvents_ChapterCommitCarriesFullText(t *testing.T) {
	ctx := activity.WithActor(context.Background(), activity.ActorAIWriter)
	ctx = activity.WithSource(ctx, activity.SourceAutoPipe)
	s := newEventTestStore(t)
	seedEventProject(t, s, ctx)

	bigContent := strings.Repeat("残阳如血，长街之上唯余剑鸣。", 200)
	ch := &domain.Chapter{
		ID:           "ch_evt_1",
		ProjectID:    "proj-evt",
		ChapterIndex: 1,
		Title:        "血色黄昏",
		CoreConflict: "宿敌堵截",
		Content:      bigContent,
		StateMutation: domain.StateMutation{
			InventoryDelta: "+断魂酒x1",
			PowerDelta:     "筑基初期→筑基中期",
		},
		Review: &domain.ReviewResult{
			Verdict:    domain.ReviewVerdictAccepted,
			Score:      88,
			ReviewedAt: time.Now(),
		},
	}
	if _, err := s.CommitChapter(ctx, "proj-evt", ch); err != nil {
		t.Fatalf("CommitChapter failed: %v", err)
	}

	list, _, err := s.ListEvents(ctx, activity.EventQuery{ProjectID: "proj-evt", EntityType: activity.EntityChapter})
	if err != nil {
		t.Fatalf("ListEvents failed: %v", err)
	}
	if len(list) != 1 || list[0].EventType != activity.EventChapterCommitted {
		t.Fatalf("expected 1 chapter.committed, got %+v", list)
	}
	ev := list[0]
	if ev.Actor != activity.ActorAIWriter || ev.Source != activity.SourceAutoPipe {
		t.Fatalf("ctx attribution lost: actor=%s source=%s", ev.Actor, ev.Source)
	}
	if ev.ExcerptAfter == "" || ev.CharsAdded <= 0 {
		t.Fatalf("expected excerpt + char delta, got excerpt=%q added=%d", ev.ExcerptAfter, ev.CharsAdded)
	}
	if ev.Before != "" {
		t.Fatalf("list view must not carry full snapshots")
	}
	if !strings.Contains(ev.Summary, "88") {
		t.Fatalf("summary must carry audit score, got %q", ev.Summary)
	}

	detail, err := s.GetEvent(ctx, "proj-evt", ev.ID)
	if err != nil {
		t.Fatalf("GetEvent failed: %v", err)
	}
	if detail.Codec != activity.CodecGzip {
		t.Fatalf("large chapter snapshot must be gzipped, got %q", detail.Codec)
	}
	if detail.After == "" || !strings.Contains(detail.After, bigContent[:60]) {
		t.Fatalf("detail must decode full chapter content")
	}
}

func TestEvents_CommitFailureLeavesNoLedgerTrace(t *testing.T) {
	ctx := context.Background()
	s := newEventTestStore(t)
	ch := &domain.Chapter{
		ProjectID:    "proj-missing",
		ChapterIndex: 1,
		Title:        "孤章",
		Content:      "无根之木",
	}
	if _, err := s.CommitChapter(ctx, "proj-missing", ch); err == nil {
		t.Fatalf("expected commit against missing project to fail")
	}
	list, _, err := s.ListEvents(ctx, activity.EventQuery{ProjectID: "proj-missing"})
	if err != nil {
		t.Fatalf("ListEvents failed: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("failed tx must leave no events, got %d", len(list))
	}
}

func TestEvents_CheckpointSavedClearedAndFiltered(t *testing.T) {
	ctx := context.Background()
	s := newEventTestStore(t)
	seedEventProject(t, s, ctx)

	cp := &domain.ChapterCheckpoint{
		ProjectID:    "proj-evt",
		ChapterIndex: 2,
		Phase:        domain.CheckpointPhaseAudited,
		CoreConflict: "夜探藏经阁",
		DraftText:    strings.Repeat("阁楼深处传来翻书声。", 100),
		AuditReport: &domain.AuditReport{
			Verdict: domain.ReviewVerdictRevision,
			Score:   72,
		},
		RewriteLoops: 1,
	}
	if err := s.SaveCheckpoint(ctx, cp); err != nil {
		t.Fatalf("SaveCheckpoint failed: %v", err)
	}

	withCP, _, err := s.ListEvents(ctx, activity.EventQuery{ProjectID: "proj-evt", IncludeCheckpoint: true})
	if err != nil {
		t.Fatalf("ListEvents failed: %v", err)
	}
	var cpEv *activity.ActivityEvent
	for _, e := range withCP {
		if e.EventType == activity.EventCheckpointSaved {
			cpEv = e
		}
	}
	if cpEv == nil {
		t.Fatalf("checkpoint.saved must be visible, got %d events", len(withCP))
	}
	if !strings.Contains(cpEv.Title, "72") {
		t.Fatalf("checkpoint title must carry audit score, got %q", cpEv.Title)
	}

	withoutCP, _, err := s.ListEvents(ctx, activity.EventQuery{ProjectID: "proj-evt"})
	if err != nil {
		t.Fatalf("ListEvents failed: %v", err)
	}
	for _, e := range withoutCP {
		if strings.HasPrefix(e.EventType, "checkpoint.") {
			t.Fatalf("default list must exclude checkpoints, found %s", e.EventType)
		}
	}

	// Clearing a missing checkpoint stays silent.
	before, _, _ := s.ListEvents(ctx, activity.EventQuery{ProjectID: "proj-evt", IncludeCheckpoint: true})
	if err := s.ClearCheckpoint(ctx, "proj-evt", 99); err != nil {
		t.Fatalf("ClearCheckpoint failed: %v", err)
	}
	after, _, _ := s.ListEvents(ctx, activity.EventQuery{ProjectID: "proj-evt", IncludeCheckpoint: true})
	if len(before) != len(after) {
		t.Fatalf("clearing a missing checkpoint must not emit, %d -> %d", len(before), len(after))
	}

	// Clearing an existing one emits checkpoint.cleared.
	if err := s.ClearCheckpoint(ctx, "proj-evt", 2); err != nil {
		t.Fatalf("ClearCheckpoint failed: %v", err)
	}
	final, _, _ := s.ListEvents(ctx, activity.EventQuery{ProjectID: "proj-evt", IncludeCheckpoint: true})
	if len(final) != len(after)+1 || final[0].EventType != activity.EventCheckpointCleared {
		t.Fatalf("expected checkpoint.cleared newest, got %+v", final[0])
	}
}

func TestEvents_HookStatusTransitionTypes(t *testing.T) {
	ctx := context.Background()
	s := newEventTestStore(t)
	seedEventProject(t, s, ctx)

	h := &domain.PlotHook{
		ID:             "hook_evt_1",
		ProjectID:      "proj-evt",
		Title:          "玉佩残魂",
		Details:        "第一章埋设",
		CreatedChapter: 1,
		TargetChapter:  5,
		Status:         domain.HookStatusOpen,
	}
	if err := s.SavePlotHook(ctx, h); err != nil {
		t.Fatalf("SavePlotHook failed: %v", err)
	}
	h.Status = domain.HookStatusResolved
	if err := s.SavePlotHook(ctx, h); err != nil {
		t.Fatalf("SavePlotHook resolve failed: %v", err)
	}

	list, _, err := s.ListEvents(ctx, activity.EventQuery{ProjectID: "proj-evt", EntityType: activity.EntityHook})
	if err != nil {
		t.Fatalf("ListEvents failed: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 hook events, got %d", len(list))
	}
	if list[0].EventType != activity.EventHookResolved {
		t.Fatalf("status transition must emit hook.resolved, got %s", list[0].EventType)
	}
	if list[1].EventType != activity.EventHookCreated {
		t.Fatalf("first save must emit hook.created, got %s", list[1].EventType)
	}
}

func TestEvents_PaginationCursor(t *testing.T) {
	ctx := context.Background()
	s := newEventTestStore(t)
	seedEventProject(t, s, ctx)

	for i := 0; i < 5; i++ {
		h := &domain.PlotHook{
			ID:             strings.Repeat("h", 3) + string(rune('a'+i)),
			ProjectID:      "proj-evt",
			Title:          "批量伏笔",
			CreatedChapter: 1,
			TargetChapter:  9,
			Status:         domain.HookStatusOpen,
		}
		if err := s.SavePlotHook(ctx, h); err != nil {
			t.Fatalf("SavePlotHook failed: %v", err)
		}
	}

	page1, cursor, err := s.ListEvents(ctx, activity.EventQuery{ProjectID: "proj-evt", Limit: 3, IncludeCheckpoint: true})
	if err != nil {
		t.Fatalf("ListEvents failed: %v", err)
	}
	if len(page1) != 3 || cursor == 0 {
		t.Fatalf("expected 3 events + cursor, got %d cursor=%d", len(page1), cursor)
	}
	page2, cursor2, err := s.ListEvents(ctx, activity.EventQuery{ProjectID: "proj-evt", Limit: 3, Cursor: cursor, IncludeCheckpoint: true})
	if err != nil {
		t.Fatalf("ListEvents failed: %v", err)
	}
	if cursor2 != 0 {
		t.Fatalf("expected exhausted cursor 0, got %d", cursor2)
	}
	seen := map[string]bool{}
	for _, e := range append(page1, page2...) {
		if seen[e.ID] {
			t.Fatalf("duplicate event %s across pages", e.ID)
		}
		seen[e.ID] = true
	}
	// 1 project.created + 5 hooks = 6 total
	if len(seen) != 6 {
		t.Fatalf("expected 6 distinct events, got %d", len(seen))
	}
}

func TestEvents_LedgerNeverBreaksUnattributableWrites(t *testing.T) {
	ctx := context.Background()
	s := newEventTestStore(t)

	// Scene.Validate does not require ProjectID: such writes must keep
	// succeeding, the ledger simply skips rows it cannot attribute.
	seedEventProject(t, s, ctx)
	if err := s.SaveChapter(ctx, &domain.Chapter{
		ID: "ch_orphan_parent", ProjectID: "proj-evt", ChapterIndex: 9,
		Title: "父章", Content: "父章正文",
	}); err != nil {
		t.Fatalf("seed SaveChapter failed: %v", err)
	}
	sc := &domain.Scene{
		ID:         "sc_orphan",
		ChapterID:  "ch_orphan_parent",
		SceneIndex: 1,
		Title:      "孤场",
	}
	if err := s.SaveScene(ctx, sc); err != nil {
		t.Fatalf("SaveScene without project must still succeed, got %v", err)
	}
	all, _, err := s.ListEvents(ctx, activity.EventQuery{ProjectID: "proj-evt", EntityType: activity.EntityScene, IncludeCheckpoint: true})
	if err != nil {
		t.Fatalf("ListEvents failed: %v", err)
	}
	if len(all) != 0 {
		t.Fatalf("unattributable scene write must leave no ledger row, got %d", len(all))
	}
	if err := s.DeleteChapter(ctx, "", 1); err != nil {
		t.Fatalf("DeleteChapter without project must still succeed, got %v", err)
	}
	if err := s.DeleteProject(ctx, ""); err != nil {
		t.Fatalf("DeleteProject without id must still succeed, got %v", err)
	}
}

func TestEvents_UncommitAndDeletesLeaveTombstones(t *testing.T) {
	ctx := context.Background()
	s := newEventTestStore(t)
	seedEventProject(t, s, ctx)

	ch := &domain.Chapter{
		ProjectID:    "proj-evt",
		ChapterIndex: 1,
		Title:        "可撤回章",
		Content:      "正文待撤回",
	}
	if _, err := s.CommitChapter(ctx, "proj-evt", ch); err != nil {
		t.Fatalf("CommitChapter failed: %v", err)
	}
	if _, _, err := s.UncommitChapter(ctx, "proj-evt", 1); err != nil {
		t.Fatalf("UncommitChapter failed: %v", err)
	}
	if err := s.DeletePlotHook(ctx, "hook_never_existed"); err != nil {
		t.Fatalf("DeletePlotHook failed: %v", err)
	}

	list, _, err := s.ListEvents(ctx, activity.EventQuery{ProjectID: "proj-evt", IncludeCheckpoint: true})
	if err != nil {
		t.Fatalf("ListEvents failed: %v", err)
	}
	types := map[string]bool{}
	for _, e := range list {
		types[e.EventType] = true
	}
	for _, want := range []string{activity.EventChapterCommitted, activity.EventChapterUncommitted} {
		if !types[want] {
			t.Fatalf("expected %s in timeline, got %v", want, types)
		}
	}

	// Deleting the project keeps its ledger.
	if err := s.DeleteProject(ctx, "proj-evt"); err != nil {
		t.Fatalf("DeleteProject failed: %v", err)
	}
	kept, _, err := s.ListEvents(ctx, activity.EventQuery{ProjectID: "proj-evt", IncludeCheckpoint: true})
	if err != nil {
		t.Fatalf("ListEvents after delete failed: %v", err)
	}
	if len(kept) == 0 || kept[0].EventType != activity.EventProjectDeleted {
		t.Fatalf("ledger must survive project deletion with tombstone newest")
	}
}
