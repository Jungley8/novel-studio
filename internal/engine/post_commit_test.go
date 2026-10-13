package engine

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Jungley8/novel-studio/internal/domain"
)

type postCommitTestStore struct {
	*mockSummaryWorkshopStore
	savedHooks []*domain.PlotHook
}

func (s *postCommitTestStore) SavePlotHook(ctx context.Context, hook *domain.PlotHook) error {
	s.savedHooks = append(s.savedHooks, hook)
	return nil
}

func (s *postCommitTestStore) ListPlotHooks(ctx context.Context, projectID string) ([]*domain.PlotHook, error) {
	return s.savedHooks, nil
}

type mockPostCommitLLMClient struct {
	responder func(system, user string) (string, error)
}

func (m *mockPostCommitLLMClient) ChatCompletion(ctx context.Context, model, system, user string, temp float64) (string, error) {
	if m.responder != nil {
		return m.responder(system, user)
	}
	return "{}", nil
}

func (m *mockPostCommitLLMClient) ChatCompletionWithUsage(ctx context.Context, model, system, user string, temp float64) (string, TokenUsage, error) {
	resp, err := m.ChatCompletion(ctx, model, system, user, temp)
	return resp, TokenUsage{TotalTokens: 42}, err
}

func (m *mockPostCommitLLMClient) ChatCompletionStream(ctx context.Context, model, system, user string, temp float64) (<-chan StreamChunk, error) {
	ch := make(chan StreamChunk)
	close(ch)
	return ch, nil
}

func TestRunPostCommitEnhancement_Success(t *testing.T) {
	content := "林凡握紧残铁古剑，在洞府门口迎战血煞宗弟子。残铁古剑在这一刻爆发出璀璨神辉！敌人仓皇退走。"
	chapter := &domain.Chapter{
		ProjectID:    "p1",
		ChapterIndex: 1,
		Title:        "第一章 神剑初醒",
		Content:      content,
		StateMutation: domain.StateMutation{
			InventoryDelta: "+残铁古剑",
			PowerDelta:     "炼气三层",
		},
	}
	proj := &domain.Project{
		ID:    "p1",
		Title: "诛仙志",
		Framework: &domain.ProjectFramework{
			VolumeArcs: []domain.VolumeArc{
				{
					VolumeIndex:  1,
					Title:        "第一卷 青云惊变",
					StartChapter: 1,
					EndChapter:   1, // 第1章即卷末，触发回顾
				},
			},
		},
	}

	rawStore := newMockSummaryWorkshopStore(proj)
	st := &postCommitTestStore{mockSummaryWorkshopStore: rawStore}

	client := &mockPostCommitLLMClient{
		responder: func(system, user string) (string, error) {
			if strings.Contains(system, "伏笔登记员") {
				return `[{"title":"残铁古剑异动","details":"剑身神辉后续必有大秘","quote":"残铁古剑在这一刻爆发出璀璨神辉！","target_chapter":5}]`, nil
			}
			if strings.Contains(system, "大卷剧情复盘专家") || strings.Contains(system, "分卷回顾") {
				return `{"happened":"青云宗初战告捷","remaining":"突破炼气三层","unresolved":"血煞宗余孽未除","text":"【发生】青云宗初战告捷。\n【留下】突破炼气三层。\n【未回收】血煞宗余孽未除。"}`, nil
			}
			// 章节摘要
			return `{"summary":"林凡持残铁古剑击退血煞宗弟子，古剑神辉惊现，余孽未除仍有隐患。"}`, nil
		},
	}

	report, err := RunPostCommitEnhancement(context.Background(), client, "test-model", st, proj, chapter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	t.Logf("report issues: %v", report.Issues)
	if !report.SummaryGenerated {
		t.Errorf("expected summary to be generated")
	}
	if st.chapterSummaries[1] == nil || st.chapterSummaries[1].ChapterIndex != 1 {
		t.Errorf("expected savedSummary for chapter 1, got %+v", st.chapterSummaries)
	}
	if report.HooksExtracted != 1 {
		t.Errorf("expected 1 hook extracted, got %d", report.HooksExtracted)
	}
	if len(st.savedHooks) != 1 || st.savedHooks[0].Title != "残铁古剑异动" {
		t.Errorf("expected saved hook '残铁古剑异动', got %+v", st.savedHooks)
	}
	if !report.RetroGenerated {
		t.Errorf("expected volume retrospective to be generated")
	}
	if st.volumeRetros[1] == nil {
		t.Errorf("expected saved retro in store, got %+v", st.volumeRetros)
	}
}

func TestRunPostCommitEnhancement_GracefulOnErrors(t *testing.T) {
	chapter := &domain.Chapter{
		ProjectID:    "p1",
		ChapterIndex: 1,
		Title:        "第一章",
		Content:      "林凡在石阶上静坐冥想。",
	}
	proj := &domain.Project{ID: "p1"}
	rawStore := newMockSummaryWorkshopStore(proj)
	rawStore.saveSummaryErr = errors.New("disk full")
	st := &postCommitTestStore{mockSummaryWorkshopStore: rawStore}

	client := &mockPostCommitLLMClient{
		responder: func(system, user string) (string, error) {
			return "", errors.New("llm service unavailable")
		},
	}

	report, err := RunPostCommitEnhancement(context.Background(), client, "test-model", st, proj, chapter)
	if err != nil {
		t.Fatalf("RunPostCommitEnhancement should not fail closed hard, but err was: %v", err)
	}
	if len(report.Issues) == 0 {
		t.Errorf("expected issues reported for failures")
	}
}
