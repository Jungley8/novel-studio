package engine

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Jungley8/novel-studio/internal/domain"
)

type mockChronicleStore struct {
	project        *domain.Project
	chapters       []*domain.Chapter
	hooks          []*domain.PlotHook
	codexEntries   []*domain.CodexEntry
	codexRelations []domain.EntityRelation
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
func (m *mockChronicleStore) UncommitChapter(ctx context.Context, projectID string, chapterIndex int) (*domain.ChapterCheckpoint, *domain.Project, error) {
	for i, c := range m.chapters {
		if c.ChapterIndex == chapterIndex {
			m.chapters = append(m.chapters[:i], m.chapters[i+1:]...)
			return &domain.ChapterCheckpoint{
				ProjectID:    projectID,
				ChapterIndex: chapterIndex,
				DraftText:    c.Content,
			}, m.project, nil
		}
	}
	return nil, nil, errors.New("chapter not found")
}
func (m *mockChronicleStore) DeleteChapter(ctx context.Context, projectID string, chapterIndex int) error {
	for i, c := range m.chapters {
		if c.ChapterIndex == chapterIndex {
			m.chapters = append(m.chapters[:i], m.chapters[i+1:]...)
			break
		}
	}
	return nil
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
func (m *mockChronicleStore) SaveCheckpoint(ctx context.Context, checkpoint *domain.ChapterCheckpoint) error {
	return nil
}
func (m *mockChronicleStore) GetCheckpoint(ctx context.Context, projectID string, chapterIndex int) (*domain.ChapterCheckpoint, error) {
	return nil, nil
}
func (m *mockChronicleStore) ClearCheckpoint(ctx context.Context, projectID string, chapterIndex int) error {
	return nil
}
func (m *mockChronicleStore) SaveCodexEntry(ctx context.Context, entry *domain.CodexEntry) error {
	return nil
}
func (m *mockChronicleStore) GetCodexEntry(ctx context.Context, projectID, id string) (*domain.CodexEntry, error) {
	return nil, nil
}
func (m *mockChronicleStore) ListCodexEntries(ctx context.Context, projectID string, category domain.CodexCategory) ([]*domain.CodexEntry, error) {
	return m.codexEntries, nil
}
func (m *mockChronicleStore) DeleteCodexEntry(ctx context.Context, projectID, id string) error {
	return nil
}
func (m *mockChronicleStore) SaveCodexProgression(ctx context.Context, entryID string, prog *domain.Progression) error {
	return nil
}
func (m *mockChronicleStore) ListCodexProgressions(ctx context.Context, entryID string) ([]domain.Progression, error) {
	return nil, nil
}
func (m *mockChronicleStore) SaveCodexRelation(ctx context.Context, projectID string, rel *domain.EntityRelation) error {
	return nil
}
func (m *mockChronicleStore) ListCodexRelations(ctx context.Context, projectID string, entryID string) ([]domain.EntityRelation, error) {
	return m.codexRelations, nil
}
func (m *mockChronicleStore) DeleteCodexRelation(ctx context.Context, projectID string, id string) error {
	return nil
}
func (m *mockChronicleStore) SaveScene(ctx context.Context, scene *domain.Scene) error { return nil }
func (m *mockChronicleStore) GetScene(ctx context.Context, id string) (*domain.Scene, error) {
	return nil, nil
}
func (m *mockChronicleStore) ListScenes(ctx context.Context, chapterID string) ([]*domain.Scene, error) {
	return nil, nil
}
func (m *mockChronicleStore) DeleteScene(ctx context.Context, id string) error { return nil }
func (m *mockChronicleStore) SaveSceneMarker(ctx context.Context, marker *domain.SceneMarker) error {
	return nil
}
func (m *mockChronicleStore) ListSceneMarkers(ctx context.Context, sceneID string) ([]domain.SceneMarker, error) {
	return nil, nil
}
func (m *mockChronicleStore) DeleteSceneMarker(ctx context.Context, id string) error { return nil }
func (m *mockChronicleStore) GetMatrixOverview(ctx context.Context, projectID string) (*domain.MatrixOverview, error) {
	return nil, nil
}
func (m *mockChronicleStore) Close() error { return nil }

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
			{ChapterIndex: 4, Title: "第四章 炼制灵丹", CoreConflict: "灵药匮乏", Content: "夜色渐浓，丹炉内的药香愈发纯粹。韩立长舒一口气，盖上了鼎盖。"},
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

	if !strings.Contains(horizon.TailAnchor, "盖上了鼎盖") {
		t.Errorf("expected tail anchor to be captured from chapter 4, got: %s", horizon.TailAnchor)
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

	// Historical callbacks check
	if !strings.Contains(horizon.HistoricalCallbacks, "神秘老者信物") {
		t.Errorf("expected historical callbacks to contain hook title, got: %s", horizon.HistoricalCallbacks)
	}
	if !strings.Contains(horizon.HistoricalCallbacks, "考核受阻") {
		t.Errorf("expected historical callbacks to contain origin conflict, got: %s", horizon.HistoricalCallbacks)
	}
}

func TestCanonChronicle_HistoricalCallbacks_NarrativeExcerpt(t *testing.T) {
	mockStore := &mockChronicleStore{
		project: &domain.Project{
			ID:         "p_callback_test",
			Title:      "诛仙记",
			WorldRules: "修道者逆天而行",
			Protagonist: domain.Protagonist{
				NameAndLevel: "张小凡 (玉清四层)",
			},
		},
		chapters: []*domain.Chapter{
			{
				ChapterIndex: 1,
				Title:        "草庙村惨变",
				CoreConflict: "屠村惨祸，拜入青云门",
				Content:      "苍穹如墨，夜风呼啸。普智大师满身血痕，在断壁残垣中将一枚暗红色的嗜血珠递入少年掌心，咳血低语道：'此物至邪至凶，切不可示于人前，寻一深渊丢弃之。'少年双膝跪地，死死握紧了这枚冰凉刺骨的嗜血珠，眼底尽是茫然与绝望。后半夜暴雨倾盆，雷电撕裂了青云山脚下的整片废墟。",
			},
			{
				ChapterIndex: 2,
				Title:        "青云试剑",
				CoreConflict: "资质愚钝受同门嘲讽",
				Content:      "大竹峰后山竹林郁郁葱葱。张小凡手持柴刀奋力劈砍黑节竹，手掌磨出殷红血泡。田不易冷哼一声拂袖离去。",
			},
			{
				ChapterIndex: 3,
				Title:        "幽潭异动",
				CoreConflict: "深潭遇险",
				Content:      "潭水刺骨，寒气逼人。",
			},
			{
				ChapterIndex: 4,
				Title:        "暗流涌动",
				CoreConflict: "魔教谍影",
				Content:      "青云门戒备森严。",
			},
			{
				ChapterIndex: 5,
				Title:        "七脉会武在即",
				CoreConflict: "临阵备战",
				Content:      "钟鸣九响，全门震动。",
			},
		},
		hooks: []*domain.PlotHook{
			{
				Title:          "嗜血珠归属与誓言",
				Details:        "普智临终托付凶物嗜血珠，嘱咐不可示人",
				CreatedChapter: 1,
				TargetChapter:  6,
				Status:         domain.HookStatusOpen,
			},
			{
				Title:          "黑节竹神秘刻痕",
				Details:        "后山竹林深处的古老剑痕",
				CreatedChapter: 2,
				TargetChapter:  10,
				Status:         domain.HookStatusFermenting,
			},
		},
	}

	chronicle := NewCanonChronicle(mockStore)

	// Target Chapter: 6
	// Rolling window (last 3): Chapters 3, 4, 5
	// Older chapters: Chapters 1, 2
	horizon, err := chronicle.AssembleHorizon(context.Background(), "p_callback_test", 6)
	if err != nil {
		t.Fatalf("AssembleHorizon failed: %v", err)
	}

	if len(horizon.RecentChapters) != 3 {
		t.Fatalf("expected 3 recent chapters, got %d", len(horizon.RecentChapters))
	}
	if horizon.RecentChapters[0].ChapterIndex != 3 || horizon.RecentChapters[2].ChapterIndex != 5 {
		t.Errorf("expected chapters 3-5 in window")
	}

	callbacks := horizon.HistoricalCallbacks
	if !strings.Contains(callbacks, "【跨卷历史因果线索 (Historical Callbacks)】") {
		t.Fatalf("expected historical callbacks header, got:\n%s", callbacks)
	}

	// Verify Chapter 1 hook and scene excerpt
	if !strings.Contains(callbacks, "【第 1 章：草庙村惨变】埋设伏笔《嗜血珠归属与誓言》") {
		t.Errorf("missing Chapter 1 hook title in callbacks:\n%s", callbacks)
	}
	if !strings.Contains(callbacks, "普智临终托付凶物嗜血珠") {
		t.Errorf("missing Chapter 1 hook details in callbacks:\n%s", callbacks)
	}
	if !strings.Contains(callbacks, "嗜血珠") || !strings.Contains(callbacks, "普智大师满身血痕") {
		t.Errorf("expected scene excerpt to capture the moment of planting the hook, got:\n%s", callbacks)
	}

	// Verify Chapter 2 hook
	if !strings.Contains(callbacks, "【第 2 章：青云试剑】埋设伏笔《黑节竹神秘刻痕》") {
		t.Errorf("missing Chapter 2 hook in callbacks:\n%s", callbacks)
	}
	if !strings.Contains(callbacks, "后山竹林深处的古老剑痕") {
		t.Errorf("missing Chapter 2 hook details in callbacks:\n%s", callbacks)
	}
}

func TestCanonChronicle_CodexAssembly(t *testing.T) {
	mockStore := &mockChronicleStore{
		project: &domain.Project{
			ID:         "p_codex_chronicle",
			Title:      "修罗弑神传",
			WorldRules: "神明窃取香火，凡人皆为血食",
			Protagonist: domain.Protagonist{
				NameAndLevel: "楚枫 (筑基初期)",
			},
		},
		chapters: []*domain.Chapter{
			{
				ChapterIndex: 14,
				Title:        "第十四章 仇人相见",
				CoreConflict: "在废丹房被霸刀围堵",
				Content:      "月黑风高，废丹房外杀机弥漫。白衣修罗按住刀柄，冷视着堵在门前的霸刀。两人杀气冲天。",
			},
		},
		codexEntries: []*domain.CodexEntry{
			{
				ID:           "e-chufeng",
				Name:         "楚枫",
				Aliases:      []string{"白衣修罗", "疯子楚"},
				Category:     domain.CategoryCharacter,
				TrackingMode: domain.TrackingModeAutoMention,
				Progressions: []domain.Progression{
					{
						ActiveFromChapter: 1,
						StatePayloadJSON:  "炼气三层，外门杂役",
					},
					{
						ActiveFromChapter: 10,
						StatePayloadJSON:  "筑基初期，内门真传",
						Notes:             "修罗血脉苏醒",
					},
				},
			},
			{
				ID:           "e-zhao",
				Name:         "赵天霸",
				Aliases:      []string{"霸刀"},
				Category:     domain.CategoryCharacter,
				Summary:      "黑风寨大当家",
				TrackingMode: domain.TrackingModeAutoMention,
			},
			{
				ID:           "e-danfang",
				Name:         "废丹房",
				Category:     domain.CategoryLocation,
				Summary:      "地火脉废弃药库",
				TrackingMode: domain.TrackingModeAutoMention,
			},
		},
		codexRelations: []domain.EntityRelation{
			{
				SourceEntryID: "e-chufeng",
				TargetEntryID: "e-zhao",
				TargetName:    "赵天霸",
				RelationType:  "NEMESIS",
				Description:   "灭族死仇，必分生死",
			},
		},
	}

	chronicle := NewCanonChronicle(mockStore)
	horizon, err := chronicle.AssembleHorizon(context.Background(), "p_codex_chronicle", 15)
	if err != nil {
		t.Fatalf("AssembleHorizon failed: %v", err)
	}

	if len(horizon.ActiveCodexEntries) != 3 {
		t.Fatalf("expected 3 matched codex entries (楚枫, 赵天霸, 废丹房), got %d", len(horizon.ActiveCodexEntries))
	}

	codexText := horizon.CodexContextText
	if !strings.Contains(codexText, "【出场世界观实体与时空状态 (The Codex)】") {
		t.Fatalf("missing Codex header in text:\n%s", codexText)
	}
	if !strings.Contains(codexText, "楚枫") || !strings.Contains(codexText, "筑基初期，内门真传") {
		t.Errorf("expected 楚枫 chapter 10 progression in codex text:\n%s", codexText)
	}
	if !strings.Contains(codexText, "赵天霸") {
		t.Errorf("expected 赵天霸 in codex text:\n%s", codexText)
	}
	if !strings.Contains(codexText, "废丹房") {
		t.Errorf("expected 废丹房 in codex text:\n%s", codexText)
	}
	if !strings.Contains(codexText, "【实体间羁绊与冲突事实 (Relations)】") || !strings.Contains(codexText, "NEMESIS") {
		t.Errorf("expected NEMESIS relation between 楚枫 and 赵天霸 in codex text:\n%s", codexText)
	}
}
