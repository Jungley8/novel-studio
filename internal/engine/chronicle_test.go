package engine

import (
	"context"
	"strings"
	"testing"

	"github.com/Jungley8/novel-studio/internal/domain"
)

type mockChronicleStore struct {
	project  *domain.Project
	chapters []*domain.Chapter
	hooks    []*domain.PlotHook
}

func (m *mockChronicleStore) GetProject(ctx context.Context, id string) (*domain.Project, error) {
	return m.project, nil
}
func (m *mockChronicleStore) ListProjects(ctx context.Context) ([]*domain.Project, error) {
	return []*domain.Project{m.project}, nil
}
func (m *mockChronicleStore) SaveProject(ctx context.Context, project *domain.Project) error {
	return nil
}
func (m *mockChronicleStore) DeleteProject(ctx context.Context, id string) error { return nil }
func (m *mockChronicleStore) CommitChapter(ctx context.Context, projectID string, chapter *domain.Chapter) (*domain.Project, error) {
	return m.project, nil
}
func (m *mockChronicleStore) SaveChapter(ctx context.Context, chapter *domain.Chapter) error {
	return nil
}
func (m *mockChronicleStore) GetChapter(ctx context.Context, projectID string, chapterIndex int) (*domain.Chapter, error) {
	for _, c := range m.chapters {
		if c.ChapterIndex == chapterIndex {
			return c, nil
		}
	}
	return nil, nil
}
func (m *mockChronicleStore) ListChapters(ctx context.Context, projectID string) ([]*domain.Chapter, error) {
	return m.chapters, nil
}
func (m *mockChronicleStore) SavePlotHook(ctx context.Context, hook *domain.PlotHook) error {
	return nil
}
func (m *mockChronicleStore) ListPlotHooks(ctx context.Context, projectID string) ([]*domain.PlotHook, error) {
	return m.hooks, nil
}
func (m *mockChronicleStore) DeletePlotHook(ctx context.Context, id string) error { return nil }
func (m *mockChronicleStore) Close() error                                        { return nil }

func TestCanonChronicle_AssembleHorizon(t *testing.T) {
	mockStore := &mockChronicleStore{
		project: &domain.Project{
			ID:             "p1",
			Title:          "万古长青诀",
			TargetPlatform: "起点仙侠",
			WorldRules:     "天地不仁以万物为刍狗",
			Protagonist: domain.Protagonist{
				NameAndLevel: "韩立 (筑基初期)",
				Inventory:    "掌天瓶, 绿叶法宝",
			},
		},
		chapters: []*domain.Chapter{
			{ChapterIndex: 1, Title: "第一章 入门试炼", CoreConflict: "考核受阻"},
			{ChapterIndex: 2, Title: "第二章 险象环生", CoreConflict: "妖兽围攻"},
			{ChapterIndex: 3, Title: "第三章 绝处逢生", CoreConflict: "误入古洞府"},
			{ChapterIndex: 4, Title: "第四章 炼制灵丹", CoreConflict: "灵药匮乏"},
		},
		hooks: []*domain.PlotHook{
			{Title: "神秘老者信物", CreatedChapter: 1, TargetChapter: 5, Status: domain.HookStatusOpen},
			{Title: "宗门大比伏笔", CreatedChapter: 2, TargetChapter: 15, Status: domain.HookStatusOpen},
			{Title: "已废弃线索", CreatedChapter: 1, TargetChapter: 3, Status: domain.HookStatusAbandoned},
		},
	}

	chronicle := NewCanonChronicle(mockStore)

	// Target Chapter: 5 (should extract chapters 2, 3, 4 as the 3-chapter rolling window)
	horizon, err := chronicle.AssembleHorizon(context.Background(), "p1", 5)
	if err != nil {
		t.Fatalf("AssembleHorizon failed: %v", err)
	}

	if len(horizon.RecentChapters) != 3 {
		t.Errorf("expected 3 rolling chapters, got %d", len(horizon.RecentChapters))
	}
	if horizon.RecentChapters[0].ChapterIndex != 2 || horizon.RecentChapters[2].ChapterIndex != 4 {
		t.Errorf("expected chapters 2, 3, 4 in window, got %d to %d",
			horizon.RecentChapters[0].ChapterIndex, horizon.RecentChapters[len(horizon.RecentChapters)-1].ChapterIndex)
	}

	if !strings.Contains(horizon.RollingCanonText, "第二章 险象环生") {
		t.Errorf("expected rolling canon text to contain Chapter 2")
	}

	// Urgent hooks check: TargetChapter 5 should flag hook with TargetChapter 5
	if len(horizon.UrgentHooks) != 1 {
		t.Errorf("expected 1 urgent hook, got %d", len(horizon.UrgentHooks))
	}
	if horizon.UrgentHooks[0].Title != "神秘老者信物" {
		t.Errorf("unexpected urgent hook: %s", horizon.UrgentHooks[0].Title)
	}

	// Active hooks count (Abandoned excluded)
	if len(horizon.AllActiveHooks) != 2 {
		t.Errorf("expected 2 active hooks, got %d", len(horizon.AllActiveHooks))
	}
}
