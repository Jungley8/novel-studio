package engine

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Jungley8/novel-studio/internal/activity"
	"github.com/Jungley8/novel-studio/internal/config"
	"github.com/Jungley8/novel-studio/internal/domain"
)

type mockChronicleStore struct {
	project        *domain.Project
	chapters       []*domain.Chapter
	hooks          []*domain.PlotHook
	codexEntries   []*domain.CodexEntry
	codexRelations []domain.EntityRelation
	retros         map[int]*domain.VolumeRetrospective
	intents        []*domain.ChapterIntent
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
	m.chapters = append(m.chapters, chapter)
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
func (m *mockChronicleStore) ListStateSnapshots(ctx context.Context, projectID string) ([]*domain.ChapterStateSnapshot, error) {
	return nil, nil
}

// Chapter Intents (章节意图卡) — 内存实现，与 sqlite_store 语义对齐
func (m *mockChronicleStore) SaveChapterIntent(ctx context.Context, intent *domain.ChapterIntent) error {
	if intent == nil {
		return errors.New("chapter intent cannot be nil")
	}
	intent.UpdatedAt = time.Now()
	for i, it := range m.intents {
		if (intent.ProjectID == "" || it.ProjectID == "" || it.ProjectID == intent.ProjectID) && it.ChapterIndex == intent.ChapterIndex {
			m.intents[i] = intent
			return nil
		}
	}
	m.intents = append(m.intents, intent)
	return nil
}

func (m *mockChronicleStore) GetChapterIntent(ctx context.Context, projectID string, chapterIndex int) (*domain.ChapterIntent, error) {
	for _, it := range m.intents {
		if (projectID == "" || it.ProjectID == "" || it.ProjectID == projectID) && it.ChapterIndex == chapterIndex {
			return it, nil
		}
	}
	return nil, nil
}

func (m *mockChronicleStore) ListChapterIntents(ctx context.Context, projectID string, volumeIndex int) ([]*domain.ChapterIntent, error) {
	var list []*domain.ChapterIntent
	for _, it := range m.intents {
		if (projectID == "" || it.ProjectID == "" || it.ProjectID == projectID) && (volumeIndex <= 0 || it.VolumeIndex == volumeIndex) {
			list = append(list, it)
		}
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].ChapterIndex < list[j].ChapterIndex
	})
	return list, nil
}

func (m *mockChronicleStore) DeleteChapterIntent(ctx context.Context, projectID string, chapterIndex int) error {
	for i, it := range m.intents {
		if (projectID == "" || it.ProjectID == "" || it.ProjectID == projectID) && it.ChapterIndex == chapterIndex {
			m.intents = append(m.intents[:i], m.intents[i+1:]...)
			break
		}
	}
	return nil
}

func (m *mockChronicleStore) MarkIntentsStale(ctx context.Context, projectID string, volumeIndex int) (int64, error) {
	var changed int64
	for _, it := range m.intents {
		if (projectID == "" || it.ProjectID == "" || it.ProjectID == projectID) && it.VolumeIndex == volumeIndex {
			if it.Status == domain.IntentStatusLocked {
				it.Status = domain.IntentStatusStale
				it.UpdatedAt = time.Now()
				changed++
			}
		}
	}
	return changed, nil
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
func (m *mockChronicleStore) ListNeedsHumanCheckpoints(ctx context.Context) ([]*domain.ChapterCheckpoint, error) {
	return nil, nil
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
func (m *mockChronicleStore) GetProtagonistEntry(ctx context.Context, projectID string) (*domain.CodexEntry, error) {
	for _, e := range m.codexEntries {
		if e.Archetype == "PROTAGONIST" || e.ID == "codex_"+projectID+"_pro" {
			return e, nil
		}
	}
	return nil, nil
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
func (m *mockChronicleStore) AppendEvent(ctx context.Context, event *activity.ActivityEvent) error {
	return nil
}
func (m *mockChronicleStore) ListEvents(ctx context.Context, q activity.EventQuery) ([]*activity.ActivityEvent, int64, error) {
	return nil, 0, nil
}
func (m *mockChronicleStore) GetEvent(ctx context.Context, projectID, eventID string) (*activity.ActivityEvent, error) {
	return nil, nil
}
func (m *mockChronicleStore) GetMatrixOverview(ctx context.Context, projectID string) (*domain.MatrixOverview, error) {
	return nil, nil
}
func (m *mockChronicleStore) Close() error { return nil }
func (m *mockChronicleStore) GetVolumeRetrospective(ctx context.Context, projectID string, volumeIndex int) (*domain.VolumeRetrospective, error) {
	if m.retros != nil {
		if r, ok := m.retros[volumeIndex]; ok {
			return r, nil
		}
	}
	return nil, nil
}

func TestCanonChronicle_AssembleHorizon(t *testing.T) {
	mockStore := &mockChronicleStore{
		project: &domain.Project{
			ID:             "p1",
			Title:          "万古长青诀",
			TargetPlatform: "起点仙侠",
			WorldRules:     "天地不仁以万物为刍狗",
		},
		codexEntries: []*domain.CodexEntry{
			{
				ID:        "codex_p1_pro",
				ProjectID: "p1",
				Category:  domain.CategoryCharacter,
				Archetype: "PROTAGONIST",
				Name:      "韩立",
				Realm:     "筑基初期",
				Inventory: "掌天瓶, 绿叶法宝",
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
		},
		codexEntries: []*domain.CodexEntry{
			{
				ID:        "codex_p_callback_test_pro",
				ProjectID: "p_callback_test",
				Category:  domain.CategoryCharacter,
				Archetype: "PROTAGONIST",
				Name:      "张小凡",
				Realm:     "玉清四层",
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
				Realm:        "筑基初期",
				Archetype:    "PROTAGONIST",
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

func TestCanonChronicle_AssembleHorizon_VolumeResolution_Boundary(t *testing.T) {
	mockStore := &mockChronicleStore{
		project: &domain.Project{
			ID:             "p_vol_boundary",
			Title:          "仙道求索",
			TargetPlatform: "起点仙侠",
			WorldRules:     "天道有缺，大道争锋",
			Framework: &domain.ProjectFramework{
				VolumeArcs: []domain.VolumeArc{
					{
						VolumeIndex:       1,
						Title:             "潜龙在渊",
						Theme:             "底层求存",
						CoreGoal:          "拜入宗门筑基成功",
						Climax:            "力斩血蟒夺得筑基丹",
						EstimatedChapters: 10,
						KeyPayoffs:        []string{"身世之谜"},
					},
					{
						VolumeIndex:       2,
						Title:             "龙战于野",
						Theme:             "宗门扬名",
						CoreGoal:          "夺得宗门大比魁首",
						Climax:            "剑败外门第一人",
						EstimatedChapters: 20,
						KeyPayoffs:        []string{"宗门大比"},
					},
				},
			},
		},
	}

	chronicle := NewCanonChronicle(mockStore)

	tests := []struct {
		name          string
		targetChapter int
		wantVolIndex  int
		wantTitle     string
	}{
		{name: "第1卷首章", targetChapter: 1, wantVolIndex: 1, wantTitle: "潜龙在渊"},
		{name: "第1卷尾章", targetChapter: 10, wantVolIndex: 1, wantTitle: "潜龙在渊"},
		{name: "第2卷首章", targetChapter: 11, wantVolIndex: 2, wantTitle: "龙战于野"},
		{name: "第2卷尾章", targetChapter: 30, wantVolIndex: 2, wantTitle: "龙战于野"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			horizon, err := chronicle.AssembleHorizon(context.Background(), "p_vol_boundary", tc.targetChapter)
			if err != nil {
				t.Fatalf("AssembleHorizon failed: %v", err)
			}
			if horizon.CurrentVolume == nil {
				t.Fatalf("expected CurrentVolume not nil for chapter %d", tc.targetChapter)
			}
			if horizon.CurrentVolume.VolumeIndex != tc.wantVolIndex {
				t.Errorf("expected VolumeIndex %d, got %d", tc.wantVolIndex, horizon.CurrentVolume.VolumeIndex)
			}
			if horizon.CurrentVolume.Title != tc.wantTitle {
				t.Errorf("expected Title %q, got %q", tc.wantTitle, horizon.CurrentVolume.Title)
			}
			if !strings.Contains(horizon.GlobalSummary, tc.wantTitle) {
				t.Errorf("expected GlobalSummary to contain %q, got:\n%s", tc.wantTitle, horizon.GlobalSummary)
			}
			if strings.Contains(horizon.GlobalSummary, "超出已规划") {
				t.Errorf("unexpected out of bounds note in GlobalSummary for hit chapter %d", tc.targetChapter)
			}
		})
	}
}

func TestCanonChronicle_AssembleHorizon_VolumeResolution_OutOfBounds(t *testing.T) {
	mockStore := &mockChronicleStore{
		project: &domain.Project{
			ID:             "p_vol_oob",
			Title:          "仙道求索",
			TargetPlatform: "起点仙侠",
			WorldRules:     "天道有缺，大道争锋",
			Framework: &domain.ProjectFramework{
				VolumeArcs: []domain.VolumeArc{
					{
						VolumeIndex:       1,
						Title:             "潜龙在渊",
						EstimatedChapters: 10,
						CoreGoal:          "拜入宗门",
						Climax:            "力斩血蟒",
					},
					{
						VolumeIndex:       2,
						Title:             "龙战于野",
						EstimatedChapters: 20,
						CoreGoal:          "宗门扬名",
						Climax:            "剑败强敌",
					},
				},
			},
		},
	}

	chronicle := NewCanonChronicle(mockStore)

	oobChapters := []int{31, 50, 100}
	for _, ch := range oobChapters {
		t.Run(fmt.Sprintf("超界第%d章", ch), func(t *testing.T) {
			horizon, err := chronicle.AssembleHorizon(context.Background(), "p_vol_oob", ch)
			if err != nil {
				t.Fatalf("AssembleHorizon failed: %v", err)
			}
			// Fail-Closed: 超界章节 CurrentVolume 必须为 nil，严禁静默回退末卷
			if horizon.CurrentVolume != nil {
				t.Fatalf("expected CurrentVolume nil for out of bounds chapter %d, got VolumeIndex %d (%s)",
					ch, horizon.CurrentVolume.VolumeIndex, horizon.CurrentVolume.Title)
			}
			// GlobalSummary 必须带有警示注记
			if !strings.Contains(horizon.GlobalSummary, "超出已规划") {
				t.Errorf("expected GlobalSummary to contain '超出已规划' for chapter %d, got:\n%s", ch, horizon.GlobalSummary)
			}
			if !strings.Contains(horizon.GlobalSummary, "目标章节超出已规划分卷，请先扩展卷纲") {
				t.Errorf("expected exact note in GlobalSummary for chapter %d, got:\n%s", ch, horizon.GlobalSummary)
			}
		})
	}
}

func TestCanonChronicle_AssembleHorizon_VolumeResolution_EmptyArcs(t *testing.T) {
	mockStore := &mockChronicleStore{
		project: &domain.Project{
			ID:             "p_vol_empty",
			Title:          "无卷项目",
			TargetPlatform: "通用网文",
			WorldRules:     "常规法则",
			Framework: &domain.ProjectFramework{
				VolumeArcs: nil,
			},
		},
	}

	chronicle := NewCanonChronicle(mockStore)
	horizon, err := chronicle.AssembleHorizon(context.Background(), "p_vol_empty", 1)
	if err != nil {
		t.Fatalf("AssembleHorizon failed: %v", err)
	}
	if horizon.CurrentVolume != nil {
		t.Fatalf("expected CurrentVolume nil for empty VolumeArcs, got %v", horizon.CurrentVolume)
	}
	if !strings.Contains(horizon.GlobalSummary, "超出已规划") {
		t.Errorf("expected GlobalSummary to contain '超出已规划' for empty arcs, got:\n%s", horizon.GlobalSummary)
	}
}

func TestCanonChronicle_VolumeResolution_GapMiss(t *testing.T) {
	// 缝隙 Miss（两卷之间的人为断档）
	spansWithGap := []domain.VolumeSpan{
		{VolumeIndex: 1, StartChapter: 1, EndChapter: 10},
		{VolumeIndex: 2, StartChapter: 15, EndChapter: 20}, // 11-14 为断档缝隙
	}

	arc1 := domain.VolumeArc{VolumeIndex: 1, Title: "初入仙门"}
	arc2 := domain.VolumeArc{VolumeIndex: 2, Title: "下山历练"}
	arcMap := map[int]*domain.VolumeArc{
		1: &arc1,
		2: &arc2,
	}

	// 缝隙章节必须 Miss 回 nil，永不猜测或回退
	for _, ch := range []int{11, 12, 13, 14} {
		vol := resolveVolumeArc(ch, spansWithGap, arcMap)
		if vol != nil {
			t.Errorf("expected nil for gap chapter %d, got VolumeIndex %d", ch, vol.VolumeIndex)
		}
	}

	// 边界命中验证
	if vol := resolveVolumeArc(10, spansWithGap, arcMap); vol == nil || vol.VolumeIndex != 1 {
		t.Errorf("expected volume 1 for chapter 10, got %v", vol)
	}
	if vol := resolveVolumeArc(15, spansWithGap, arcMap); vol == nil || vol.VolumeIndex != 2 {
		t.Errorf("expected volume 2 for chapter 15, got %v", vol)
	}
}

func TestDeriveVolumeSpans(t *testing.T) {
	arcs := []domain.VolumeArc{
		{
			VolumeIndex:  1,
			Title:        "首卷",
			StartChapter: 1,
			EndChapter:   40,
			Status:       domain.VolumeStatusDraft,
			CoreGoal:     "开局立足",
		},
		{
			VolumeIndex:  2,
			Title:        "次卷",
			StartChapter: 41,
			EndChapter:   65,
			Status:       domain.VolumeStatusLocked,
			CoreGoal:     "进阶宗门",
		},
	}

	spans, arcMap := deriveVolumeSpans(arcs)
	if len(spans) != 2 {
		t.Fatalf("expected 2 spans, got %d", len(spans))
	}

	// Span 1: [1, 40]
	if spans[0].VolumeIndex != 1 || spans[0].StartChapter != 1 || spans[0].EndChapter != 40 {
		t.Errorf("unexpected span 0: %+v", spans[0])
	}
	// Span 2: [41, 65]
	if spans[1].VolumeIndex != 2 || spans[1].StartChapter != 41 || spans[1].EndChapter != 65 {
		t.Errorf("unexpected span 1: %+v", spans[1])
	}

	// arcMap 必须保留原始指针及字段
	if arcMap[1] == nil || arcMap[1].Title != "首卷" || arcMap[1].CoreGoal != "开局立足" {
		t.Errorf("arcMap[1] corrupted: %+v", arcMap[1])
	}
	if arcMap[2] == nil || arcMap[2].Title != "次卷" || arcMap[2].CoreGoal != "进阶宗门" {
		t.Errorf("arcMap[2] corrupted: %+v", arcMap[2])
	}
}

func TestBackfillVolumeArcs(t *testing.T) {
	// 1. 混合测试：有显式范围、有<=0需回填、有非法Start>End
	input := []domain.VolumeArc{
		{
			VolumeIndex:  0, // 应自动推导为 1
			Title:        "第一卷",
			StartChapter: 1,
			EndChapter:   10,
			// Status 为空，应默认为 DRAFT
		},
		{
			VolumeIndex:       2,
			Title:             "第二卷",
			StartChapter:      0, // 需回填
			EndChapter:        0,
			EstimatedChapters: 15,
			Status:            domain.VolumeStatusLocked,
		},
		{
			VolumeIndex:       3,
			Title:             "第三卷",
			StartChapter:      50, // 非法：Start > End，应触发 Fail-Closed 回退推导
			EndChapter:        30,
			EstimatedChapters: 20,
		},
	}

	res := BackfillVolumeArcs(input)
	if len(res) != 3 {
		t.Fatalf("expected 3 arcs, got %d", len(res))
	}

	// Arc 0: 显式保留
	if res[0].VolumeIndex != 1 || res[0].StartChapter != 1 || res[0].EndChapter != 10 || res[0].Status != domain.VolumeStatusDraft {
		t.Errorf("arc 0 mismatch: %+v", res[0])
	}

	// Arc 1: 回填 [11, 25]，Status 保留 LOCKED
	if res[1].VolumeIndex != 2 || res[1].StartChapter != 11 || res[1].EndChapter != 25 || res[1].Status != domain.VolumeStatusLocked {
		t.Errorf("arc 1 mismatch: %+v", res[1])
	}

	// Arc 2: 非法起止章回退回填 [26, 45]
	if res[2].VolumeIndex != 3 || res[2].StartChapter != 26 || res[2].EndChapter != 45 || res[2].Status != domain.VolumeStatusDraft {
		t.Errorf("arc 2 mismatch: %+v", res[2])
	}
}

func TestBackfillVolumeArcs_LegacyEquivalence(t *testing.T) {
	// 铁律校验：同一 VolumeArcs 下旧累加 vs 新回填+定位逐章 1..N 全等
	legacyArcs := []domain.VolumeArc{
		{
			VolumeIndex:       1,
			Title:             "第一卷 潜龙",
			EstimatedChapters: 12,
			CoreGoal:          "宗门筑基",
		},
		{
			VolumeIndex:       2,
			Title:             "第二卷 跃渊",
			EstimatedChapters: 0, // <= 0 延续旧规则默认 40
			CoreGoal:          "争夺秘境",
		},
		{
			VolumeIndex:       0, // 自动推断为 3
			Title:             "第三卷 飞天",
			EstimatedChapters: 25,
			CoreGoal:          "大道圆满",
		},
	}

	// 1. 旧累加逻辑构建基准 spans
	legacySpans := make([]domain.VolumeSpan, 0, len(legacyArcs))
	acc := 0
	for i, arc := range legacyArcs {
		est := arc.EstimatedChapters
		if est <= 0 {
			est = 40
		}
		vIdx := arc.VolumeIndex
		if vIdx == 0 {
			vIdx = i + 1
		}
		legacySpans = append(legacySpans, domain.VolumeSpan{
			VolumeIndex:  vIdx,
			StartChapter: acc + 1,
			EndChapter:   acc + est,
		})
		acc += est
	}
	totalChapters := acc // 12 + 40 + 25 = 77

	// 2. 新回填算法回填
	backfilled := BackfillVolumeArcs(legacyArcs)

	if backfilled[0].StartChapter != 1 || backfilled[0].EndChapter != 12 {
		t.Errorf("arc 0 range mismatch: [%d, %d]", backfilled[0].StartChapter, backfilled[0].EndChapter)
	}
	if backfilled[1].StartChapter != 13 || backfilled[1].EndChapter != 52 {
		t.Errorf("arc 1 range mismatch: [%d, %d]", backfilled[1].StartChapter, backfilled[1].EndChapter)
	}
	if backfilled[2].VolumeIndex != 3 || backfilled[2].StartChapter != 53 || backfilled[2].EndChapter != 77 {
		t.Errorf("arc 2 range mismatch: volIdx=%d [%d, %d]", backfilled[2].VolumeIndex, backfilled[2].StartChapter, backfilled[2].EndChapter)
	}

	// 3. 逐章 1..N+5 验证 AssembleHorizon 定位完全全等
	mockStore := &mockChronicleStore{
		project: &domain.Project{
			ID: "p_equiv",
			Framework: &domain.ProjectFramework{
				VolumeArcs: legacyArcs, // 全零 start/end 输入，检验 AssembleHorizon 回填与定位
			},
		},
	}
	chronicle := NewCanonChronicle(mockStore)

	for ch := 1; ch <= totalChapters+5; ch++ {
		legacyVolIdx := domain.ResolveVolume(ch, legacySpans)

		horizon, err := chronicle.AssembleHorizon(context.Background(), "p_equiv", ch)
		if err != nil {
			t.Fatalf("AssembleHorizon failed for chapter %d: %v", ch, err)
		}

		var newVolIdx int = -1
		if horizon.CurrentVolume != nil {
			newVolIdx = horizon.CurrentVolume.VolumeIndex
		}

		if legacyVolIdx != newVolIdx {
			t.Fatalf("chapter %d equivalence broken: legacy resolved volume %d, new resolved %d",
				ch, legacyVolIdx, newVolIdx)
		}

		// 超界时注记校验
		if legacyVolIdx == -1 {
			if !strings.Contains(horizon.GlobalSummary, "超出已规划") {
				t.Fatalf("chapter %d miss expected '超出已规划' note in GlobalSummary", ch)
			}
		}
	}
}

func TestCanonChronicle_SetConfig_ContextBudget(t *testing.T) {
	mockStore := &mockChronicleStore{
		project: &domain.Project{
			ID:             "p_budget_cfg",
			Title:          "预算测试",
			TargetPlatform: "起点中文网",
		},
		chapters: []*domain.Chapter{
			{
				ChapterIndex: 1,
				Title:        "第一章",
				Content:      "正文测试内容",
			},
		},
	}

	chronicle := NewCanonChronicle(mockStore)
	chronicle.SetConfig(&config.Config{
		ContextBudgetTokens: 5000,
	})

	_, trace, err := chronicle.AssembleHorizonTraced(context.Background(), "p_budget_cfg", 2)
	if err != nil {
		t.Fatalf("expected AssembleHorizonTraced to succeed, got: %v", err)
	}
	if trace == nil {
		t.Fatalf("expected trace to be non-nil")
	}
	if trace.BudgetTokens != 5000 {
		t.Errorf("expected trace.BudgetTokens to be 5000, got %d", trace.BudgetTokens)
	}
}

// Test ①: 构造超大 WorldRules → 返回可操作错误且错误文本含超限项 ID
func TestCanonChronicle_ContextBudget_WorldRulesOverflow(t *testing.T) {
	hugeRules := strings.Repeat("天地初开，法则晦涩。神魔之战绵延三千纪元，血染星空，大道崩塌。", 800)
	mockStore := &mockChronicleStore{
		project: &domain.Project{
			ID:             "p_rules_overflow",
			Title:          "逆天神魔传",
			TargetPlatform: "起点中文网",
			WorldRules:     hugeRules,
		},
		chapters: []*domain.Chapter{
			{
				ChapterIndex: 1,
				Title:        "开天辟地",
				Content:      "混沌未分天地乱，茫茫渺渺无人见。",
			},
		},
	}

	chronicle := NewCanonChronicle(mockStore)
	// Default budget is 12000 tokens. hugeRules is ~24,000 runes = ~12,000 tokens for rules alone,
	// plus chapter and summary makes it strictly overflow.
	horizon, trace, err := chronicle.AssembleHorizonTraced(context.Background(), "p_rules_overflow", 2)
	if err == nil {
		t.Fatalf("expected actionable error for oversized WorldRules, got nil (horizon=%v)", horizon)
	}

	errText := err.Error()
	if !strings.Contains(errText, "global/rules") {
		t.Errorf("expected actionable error to contain overflow item ID 'global/rules', got: %s", errText)
	}
	if !strings.Contains(errText, "context_budget_tokens") {
		t.Errorf("expected actionable error to suggest adjusting 'context_budget_tokens', got: %s", errText)
	}
	if trace == nil || len(trace.ProtectedIDs) == 0 {
		t.Errorf("expected trace with recorded protected IDs even on overflow, got: %v", trace)
	}

	// Also verify that the standard AssembleHorizon signature returns the same actionable error
	_, rawErr := chronicle.AssembleHorizon(context.Background(), "p_rules_overflow", 2)
	if rawErr == nil || !strings.Contains(rawErr.Error(), "global/rules") {
		t.Errorf("expected AssembleHorizon to bubble the actionable error, got: %v", rawErr)
	}
}

// Test ②: 构造 10 条跨卷回调 → 确定性截断（两次运行结果一致）且 DroppedIDs 非空
func TestCanonChronicle_ContextBudget_TenCallbacksTruncationDeterministic(t *testing.T) {
	// Construct 15 chapters (target chapter: 15)
	// Recent chapters: 12, 13, 14
	// Older chapters: 1..11
	chapters := make([]*domain.Chapter, 0, 14)
	for i := 1; i <= 14; i++ {
		chapters = append(chapters, &domain.Chapter{
			ChapterIndex: i,
			Title:        fmt.Sprintf("第 %d 章 历史纪事", i),
			CoreConflict: fmt.Sprintf("历史章节冲突 %d", i),
			Content:      fmt.Sprintf("这是第 %d 章的正史正文内容，记录着曾经发生的惊心动魄的故事与暗流涌动。", i),
		})
	}

	// Construct 10 hooks in older chapters (1..10)
	// Hook 1: Urgent (TargetChapter: 16 <= 15+2)
	// Hook 2: Urgent (TargetChapter: 17 <= 15+2)
	// Hooks 3..10: Non-urgent with increasing target chapters
	hooks := []*domain.PlotHook{
		{
			ID:             "hook_urgent_1",
			Title:          "伏笔1-极度紧急",
			Details:        strings.Repeat("紧急伏笔详情设定，必须在本章或下章立即引爆结算。", 5),
			CreatedChapter: 1,
			TargetChapter:  16,
			Status:         domain.HookStatusOpen,
		},
		{
			ID:             "hook_urgent_2",
			Title:          "伏笔2-紧随其后",
			Details:        strings.Repeat("次紧急伏笔详情设定，属于近期必须处理的重大支线因果。", 5),
			CreatedChapter: 2,
			TargetChapter:  17,
			Status:         domain.HookStatusFermenting,
		},
		{
			ID:             "hook_far_3",
			Title:          "伏笔3-远期20",
			Details:        strings.Repeat("较远期的伏笔线索，涉及下一卷宗门大比的暗线伏笔。", 5),
			CreatedChapter: 3,
			TargetChapter:  20,
			Status:         domain.HookStatusOpen,
		},
		{
			ID:             "hook_far_4",
			Title:          "伏笔4-远期25",
			Details:        strings.Repeat("远期伏笔线索，潜藏在深渊地下的古老妖皇封印。", 5),
			CreatedChapter: 4,
			TargetChapter:  25,
			Status:         domain.HookStatusOpen,
		},
		{
			ID:             "hook_far_5",
			Title:          "伏笔5-远期30",
			Details:        strings.Repeat("极远期伏笔线索，域外天魔降临之兆。", 5),
			CreatedChapter: 5,
			TargetChapter:  30,
			Status:         domain.HookStatusOpen,
		},
		{
			ID:             "hook_far_6",
			Title:          "伏笔6-远期35",
			Details:        strings.Repeat("极远期伏笔线索，远古仙帝佩剑遗失之谜。", 5),
			CreatedChapter: 6,
			TargetChapter:  35,
			Status:         domain.HookStatusOpen,
		},
		{
			ID:             "hook_far_7",
			Title:          "伏笔7-远期40",
			Details:        strings.Repeat("极远期伏笔线索，轮回秘境钥匙碎片之一。", 5),
			CreatedChapter: 7,
			TargetChapter:  40,
			Status:         domain.HookStatusOpen,
		},
		{
			ID:             "hook_far_8",
			Title:          "伏笔8-远期45",
			Details:        strings.Repeat("极远期伏笔线索，主角身世玉佩第二道封印。", 5),
			CreatedChapter: 8,
			TargetChapter:  45,
			Status:         domain.HookStatusOpen,
		},
		{
			ID:             "hook_far_9",
			Title:          "伏笔9-远期50",
			Details:        strings.Repeat("极远期伏笔线索，九重天阙飞升天梯断裂因果。", 5),
			CreatedChapter: 9,
			TargetChapter:  50,
			Status:         domain.HookStatusOpen,
		},
		{
			ID:             "hook_far_10",
			Title:          "伏笔10-远期60",
			Details:        strings.Repeat("全书大后期伏笔，开天神斧最终降世地点线索。", 5),
			CreatedChapter: 10,
			TargetChapter:  60,
			Status:         domain.HookStatusOpen,
		},
	}

	mockStore := &mockChronicleStore{
		project: &domain.Project{
			ID:         "p_10_callbacks",
			Title:      "十方因果录",
			WorldRules: "因果循环，报应不爽。",
		},
		chapters: chapters,
		hooks:    hooks,
	}

	chronicle := NewCanonChronicle(mockStore)
	// Set a tight budget (e.g. 900 tokens) so protected window (~294 tokens at the
	// shared 0.75 tok/rune caliber) passes, but only a few callbacks fit within
	// the remaining ~600 tokens.
	chronicle.SetBudgetTokens(900)

	// Run 1
	horizon1, trace1, err1 := chronicle.AssembleHorizonTraced(context.Background(), "p_10_callbacks", 15)
	if err1 != nil {
		t.Fatalf("Run 1 AssembleHorizonTraced failed: %v", err1)
	}

	// Run 2
	horizon2, trace2, err2 := chronicle.AssembleHorizonTraced(context.Background(), "p_10_callbacks", 15)
	if err2 != nil {
		t.Fatalf("Run 2 AssembleHorizonTraced failed: %v", err2)
	}

	// Verify truncation occurred
	if !trace1.Truncated {
		t.Fatalf("expected trace1.Truncated to be true")
	}
	if len(trace1.DroppedIDs) == 0 {
		t.Fatalf("expected trace1.DroppedIDs to be non-empty, got 0 dropped items")
	}

	// Verify determinism: identical results on consecutive runs
	if !reflect.DeepEqual(trace1.DroppedIDs, trace2.DroppedIDs) {
		t.Errorf("non-deterministic DroppedIDs:\nrun1: %v\nrun2: %v", trace1.DroppedIDs, trace2.DroppedIDs)
	}
	if !reflect.DeepEqual(trace1.CompressibleIDs, trace2.CompressibleIDs) {
		t.Errorf("non-deterministic CompressibleIDs:\nrun1: %v\nrun2: %v", trace1.CompressibleIDs, trace2.CompressibleIDs)
	}
	if trace1.EstimatedTokens != trace2.EstimatedTokens {
		t.Errorf("non-deterministic EstimatedTokens: run1=%d run2=%d", trace1.EstimatedTokens, trace2.EstimatedTokens)
	}
	if horizon1.HistoricalCallbacks != horizon2.HistoricalCallbacks {
		t.Errorf("non-deterministic HistoricalCallbacks text output")
	}

	// Verify priority invariants:
	// Urgent items (hook_urgent_1, hook_urgent_2) must be retained in CompressibleIDs
	hasUrgent1 := false
	hasUrgent2 := false
	for _, id := range trace1.CompressibleIDs {
		if id == "callback/hook_urgent_1" {
			hasUrgent1 = true
		}
		if id == "callback/hook_urgent_2" {
			hasUrgent2 = true
		}
	}
	if !hasUrgent1 || !hasUrgent2 {
		t.Errorf("expected urgent callbacks to be retained in CompressibleIDs, got CompressibleIDs=%v", trace1.CompressibleIDs)
	}

	// The farthest hook (hook_far_10) must be dropped
	hasFar10Dropped := false
	for _, id := range trace1.DroppedIDs {
		if id == "callback/hook_far_10" {
			hasFar10Dropped = true
		}
	}
	if !hasFar10Dropped {
		t.Errorf("expected farthest callback/hook_far_10 to be dropped, got DroppedIDs=%v", trace1.DroppedIDs)
	}
}

// Test ③: Codex 长条目 → 截断标记存在
func TestCanonChronicle_ContextBudget_CodexDetailsTruncation(t *testing.T) {
	longDetails := strings.Repeat("万古魔帝身怀吞天魔功，曾在太古星空深处斩灭神明三千尊，肉身不朽，魂魄不灭。", 15) // ~555 runes > 300 runes
	mockStore := &mockChronicleStore{
		project: &domain.Project{
			ID:         "p_codex_long",
			Title:      "魔帝出世录",
			WorldRules: "神魔皆以强者为尊。",
		},
		chapters: []*domain.Chapter{
			{
				ChapterIndex: 1,
				Title:        "魔帝苏醒",
				CoreConflict: "封印松动",
				Content:      "天地变色，封魔渊下万古魔帝缓缓睁开双眼，修罗魔气滔天而起。",
			},
		},
		codexEntries: []*domain.CodexEntry{
			{
				ID:              "e_mowang",
				Name:            "万古魔帝",
				Category:        domain.CategoryCharacter,
				Archetype:       "ANTAGONIST",
				DetailsMarkdown: longDetails,
				TrackingMode:    domain.TrackingModeAutoMention,
			},
		},
	}

	chronicle := NewCanonChronicle(mockStore)
	horizon, trace, err := chronicle.AssembleHorizonTraced(context.Background(), "p_codex_long", 2)
	if err != nil {
		t.Fatalf("AssembleHorizonTraced failed: %v", err)
	}

	codexText := horizon.CodexContextText
	if !strings.Contains(codexText, "…[truncated]") {
		t.Errorf("expected Codex DetailsMarkdown to be truncated with '…[truncated]', got:\n%s", codexText)
	}

	// Requirement 4: truncated in-place, accounts for tokens, but does NOT enter DroppedIDs
	for _, id := range trace.DroppedIDs {
		if id == "codex/e_mowang" {
			t.Errorf("truncated codex details should NOT enter DroppedIDs, but found: %s", id)
		}
	}

	foundInCompressible := false
	for _, id := range trace.CompressibleIDs {
		if id == "codex/e_mowang" {
			foundInCompressible = true
			break
		}
	}
	if !foundInCompressible {
		t.Errorf("expected codex/e_mowang to be in CompressibleIDs, got: %v", trace.CompressibleIDs)
	}
}

// Test ①: 书在第 3 卷、有第 1、2 卷回顾 → HistoricalCallbacks 文本含【往卷全局记忆 且有两条；trace.CompressibleIDs 含 history/retro_vol_1、history/retro_vol_2
func TestCanonChronicle_VolumeRetrospective_NormalInjection(t *testing.T) {
	mockStore := &mockChronicleStore{
		project: &domain.Project{
			ID:             "p_retro_normal",
			Title:          "万道争锋",
			TargetPlatform: "起点仙侠",
			WorldRules:     "仙道无常",
			Framework: &domain.ProjectFramework{
				VolumeArcs: []domain.VolumeArc{
					{VolumeIndex: 1, Title: "第一卷：踏道", StartChapter: 1, EndChapter: 50, CoreGoal: "筑基", Climax: "斩妖"},
					{VolumeIndex: 2, Title: "第二卷：扬名", StartChapter: 51, EndChapter: 100, CoreGoal: "结丹", Climax: "夺魁"},
					{VolumeIndex: 3, Title: "第三卷：破界", StartChapter: 101, EndChapter: 150, CoreGoal: "化神", Climax: "飞升"},
				},
			},
		},
		chapters: []*domain.Chapter{
			{ChapterIndex: 102, Title: "第102章", Content: "第三卷正文102"},
			{ChapterIndex: 103, Title: "第103章", Content: "第三卷正文103"},
			{ChapterIndex: 104, Title: "第104章", Content: "第三卷正文104"},
		},
		retros: map[int]*domain.VolumeRetrospective{
			1: {
				ProjectID:   "p_retro_normal",
				VolumeIndex: 1,
				FromChapter: 1,
				ToChapter:   50,
				Text:        "第一卷宏观回顾：主角于微末中筑基成功，奠定无上道基。",
			},
			2: {
				ProjectID:   "p_retro_normal",
				VolumeIndex: 2,
				FromChapter: 51,
				ToChapter:   100,
				Text:        "第二卷宏观回顾：主角宗门大比夺得魁首，结成九转金丹。",
			},
		},
	}

	chronicle := NewCanonChronicle(mockStore)
	horizon, trace, err := chronicle.AssembleHorizonTraced(context.Background(), "p_retro_normal", 105)
	if err != nil {
		t.Fatalf("AssembleHorizonTraced failed: %v", err)
	}

	if !strings.Contains(horizon.HistoricalCallbacks, "【往卷全局记忆") {
		t.Errorf("expected HistoricalCallbacks to contain '【往卷全局记忆', got:\n%s", horizon.HistoricalCallbacks)
	}
	if !strings.Contains(horizon.HistoricalCallbacks, "第一卷宏观回顾") {
		t.Errorf("expected HistoricalCallbacks to contain retro vol 1 text")
	}
	if !strings.Contains(horizon.HistoricalCallbacks, "第二卷宏观回顾") {
		t.Errorf("expected HistoricalCallbacks to contain retro vol 2 text")
	}
	if strings.Count(horizon.HistoricalCallbacks, "【往卷回顾·第") != 2 {
		t.Errorf("expected exactly 2 retro descriptions in HistoricalCallbacks, got count %d",
			strings.Count(horizon.HistoricalCallbacks, "【往卷回顾·第"))
	}

	hasRetro1 := false
	hasRetro2 := false
	for _, id := range trace.CompressibleIDs {
		if id == "history/retro_vol_1" {
			hasRetro1 = true
		}
		if id == "history/retro_vol_2" {
			hasRetro2 = true
		}
	}
	if !hasRetro1 || !hasRetro2 {
		t.Errorf("expected trace.CompressibleIDs to contain history/retro_vol_1 and history/retro_vol_2, got: %v", trace.CompressibleIDs)
	}
}

// Test ②: 只给第 3 卷（当前卷）回顾 → 不注入（只取往卷）
func TestCanonChronicle_VolumeRetrospective_CurrentVolumeNotIncluded(t *testing.T) {
	mockStore := &mockChronicleStore{
		project: &domain.Project{
			ID:             "p_retro_cur",
			Title:          "万道争锋",
			TargetPlatform: "起点仙侠",
			WorldRules:     "仙道无常",
			Framework: &domain.ProjectFramework{
				VolumeArcs: []domain.VolumeArc{
					{VolumeIndex: 1, Title: "第一卷：踏道", StartChapter: 1, EndChapter: 50},
					{VolumeIndex: 2, Title: "第二卷：扬名", StartChapter: 51, EndChapter: 100},
					{VolumeIndex: 3, Title: "第三卷：破界", StartChapter: 101, EndChapter: 150},
				},
			},
		},
		chapters: []*domain.Chapter{
			{ChapterIndex: 104, Title: "第104章", Content: "第三卷正文"},
		},
		retros: map[int]*domain.VolumeRetrospective{
			3: {
				ProjectID:   "p_retro_cur",
				VolumeIndex: 3,
				FromChapter: 101,
				ToChapter:   150,
				Text:        "第三卷当前回顾：此卷尚在推进，不应注入。",
			},
		},
	}

	chronicle := NewCanonChronicle(mockStore)
	horizon, trace, err := chronicle.AssembleHorizonTraced(context.Background(), "p_retro_cur", 105)
	if err != nil {
		t.Fatalf("AssembleHorizonTraced failed: %v", err)
	}

	if strings.Contains(horizon.HistoricalCallbacks, "【往卷全局记忆") {
		t.Errorf("expected HistoricalCallbacks not to contain '【往卷全局记忆', got:\n%s", horizon.HistoricalCallbacks)
	}
	if strings.Contains(horizon.HistoricalCallbacks, "第三卷当前回顾") {
		t.Errorf("expected current volume retro not to be injected into HistoricalCallbacks")
	}

	for _, id := range trace.CompressibleIDs {
		if strings.HasPrefix(id, "history/retro_vol_") {
			t.Errorf("expected no retro items in CompressibleIDs, found: %s", id)
		}
	}
	for _, id := range trace.DroppedIDs {
		if strings.HasPrefix(id, "history/retro_vol_") {
			t.Errorf("expected no retro items in DroppedIDs, found: %s", id)
		}
	}
}

// Test ③: 预算极紧（把 budget 调小到只剩 protected）→ retro 被确定性丢弃，trace.DroppedIDs 含之，文本不含；两次运行结果一致
func TestCanonChronicle_VolumeRetrospective_TightBudgetDeterministicDrop(t *testing.T) {
	mockStore := &mockChronicleStore{
		project: &domain.Project{
			ID:             "p_retro_tight",
			Title:          "万道争锋",
			TargetPlatform: "起点仙侠",
			WorldRules:     "仙道无常",
			Framework: &domain.ProjectFramework{
				VolumeArcs: []domain.VolumeArc{
					{VolumeIndex: 1, Title: "第一卷：踏道", StartChapter: 1, EndChapter: 50},
					{VolumeIndex: 2, Title: "第二卷：扬名", StartChapter: 51, EndChapter: 100},
					{VolumeIndex: 3, Title: "第三卷：破界", StartChapter: 101, EndChapter: 150},
				},
			},
		},
		chapters: []*domain.Chapter{
			{ChapterIndex: 104, Title: "第104章", Content: "第三卷正文"},
		},
		retros: map[int]*domain.VolumeRetrospective{
			1: {
				ProjectID:   "p_retro_tight",
				VolumeIndex: 1,
				FromChapter: 1,
				ToChapter:   50,
				Text:        "第一卷宏观回顾：主角筑基。",
			},
			2: {
				ProjectID:   "p_retro_tight",
				VolumeIndex: 2,
				FromChapter: 51,
				ToChapter:   100,
				Text:        "第二卷宏观回顾：主角结丹。",
			},
		},
	}

	chronicle := NewCanonChronicle(mockStore)
	// Query with minimal budget 1 to probe exact protected tokens from fail-closed error
	chronicle.SetBudgetTokens(1)
	_, traceProbe, errProbe := chronicle.AssembleHorizonTraced(context.Background(), "p_retro_tight", 105)
	if errProbe == nil || traceProbe == nil {
		t.Fatalf("expected overflow error to probe protected tokens, got err=%v", errProbe)
	}
	protectedTokens := traceProbe.EstimatedTokens
	if protectedTokens <= 0 {
		t.Fatalf("expected positive protected tokens, got %d", protectedTokens)
	}

	// Set budget to exactly protectedTokens so protected items fit, remainingBudget = 0
	chronicle.SetBudgetTokens(protectedTokens)

	// Run 1
	horizon1, trace1, err1 := chronicle.AssembleHorizonTraced(context.Background(), "p_retro_tight", 105)
	if err1 != nil {
		t.Fatalf("Run 1 failed: %v", err1)
	}

	// Run 2
	horizon2, trace2, err2 := chronicle.AssembleHorizonTraced(context.Background(), "p_retro_tight", 105)
	if err2 != nil {
		t.Fatalf("Run 2 failed: %v", err2)
	}

	// Both retros must be in DroppedIDs
	hasDropped1 := false
	hasDropped2 := false
	for _, id := range trace1.DroppedIDs {
		if id == "history/retro_vol_1" {
			hasDropped1 = true
		}
		if id == "history/retro_vol_2" {
			hasDropped2 = true
		}
	}
	if !hasDropped1 || !hasDropped2 {
		t.Errorf("expected both retro items in trace1.DroppedIDs, got: %v", trace1.DroppedIDs)
	}

	// Text must NOT contain retro content
	if strings.Contains(horizon1.HistoricalCallbacks, "【往卷全局记忆") {
		t.Errorf("expected HistoricalCallbacks not to contain '【往卷全局记忆' under tight budget, got:\n%s", horizon1.HistoricalCallbacks)
	}

	// Determinism check: identical results on consecutive runs
	if !reflect.DeepEqual(trace1.DroppedIDs, trace2.DroppedIDs) {
		t.Errorf("non-deterministic DroppedIDs: run1=%v run2=%v", trace1.DroppedIDs, trace2.DroppedIDs)
	}
	if !reflect.DeepEqual(trace1.CompressibleIDs, trace2.CompressibleIDs) {
		t.Errorf("non-deterministic CompressibleIDs: run1=%v run2=%v", trace1.CompressibleIDs, trace2.CompressibleIDs)
	}
	if trace1.EstimatedTokens != trace2.EstimatedTokens {
		t.Errorf("non-deterministic EstimatedTokens: run1=%d run2=%d", trace1.EstimatedTokens, trace2.EstimatedTokens)
	}
	if horizon1.HistoricalCallbacks != horizon2.HistoricalCallbacks {
		t.Errorf("non-deterministic HistoricalCallbacks output")
	}
}

// Test ④: currentVolume == nil（超界章）→ 不 panic、无注入、行为照旧
func TestCanonChronicle_VolumeRetrospective_NilCurrentVolume(t *testing.T) {
	mockStore := &mockChronicleStore{
		project: &domain.Project{
			ID:             "p_retro_nilvol",
			Title:          "万道争锋",
			TargetPlatform: "起点仙侠",
			WorldRules:     "仙道无常",
			Framework: &domain.ProjectFramework{
				VolumeArcs: []domain.VolumeArc{
					{VolumeIndex: 1, Title: "第一卷：踏道", StartChapter: 1, EndChapter: 50},
					{VolumeIndex: 2, Title: "第二卷：扬名", StartChapter: 51, EndChapter: 100},
				},
			},
		},
		chapters: []*domain.Chapter{
			{ChapterIndex: 100, Title: "第100章", Content: "第二卷终章正文"},
		},
		retros: map[int]*domain.VolumeRetrospective{
			1: {
				ProjectID:   "p_retro_nilvol",
				VolumeIndex: 1,
				FromChapter: 1,
				ToChapter:   50,
				Text:        "第一卷宏观回顾",
			},
			2: {
				ProjectID:   "p_retro_nilvol",
				VolumeIndex: 2,
				FromChapter: 51,
				ToChapter:   100,
				Text:        "第二卷宏观回顾",
			},
		},
	}

	chronicle := NewCanonChronicle(mockStore)
	// Chapter 150 is out of bounds (volumes only cover up to 100) -> currentVolume == nil
	horizon, trace, err := chronicle.AssembleHorizonTraced(context.Background(), "p_retro_nilvol", 150)
	if err != nil {
		t.Fatalf("AssembleHorizonTraced failed: %v", err)
	}
	if horizon.CurrentVolume != nil {
		t.Fatalf("expected CurrentVolume to be nil for chapter 150, got %v", horizon.CurrentVolume)
	}

	if strings.Contains(horizon.HistoricalCallbacks, "【往卷全局记忆") {
		t.Errorf("expected no retro injection when CurrentVolume is nil, got:\n%s", horizon.HistoricalCallbacks)
	}

	for _, id := range trace.CompressibleIDs {
		if strings.HasPrefix(id, "history/retro_vol_") {
			t.Errorf("expected no retro items in CompressibleIDs, found: %s", id)
		}
	}
	for _, id := range trace.DroppedIDs {
		if strings.HasPrefix(id, "history/retro_vol_") {
			t.Errorf("expected no retro items in DroppedIDs, found: %s", id)
		}
	}
}

// TestCanonChronicle_ContextBudget_ProtectedOverflowTruncated verifies the
// fail-closed protected-overflow path marks the returned trace as truncated:
// with a minimal budget the protected items (global/rules from WorldRules
// plus protagonist/rolling context) must exceed it, so AssembleHorizonTraced
// returns a non-nil error together with a trace whose Truncated flag is true.
func TestCanonChronicle_ContextBudget_ProtectedOverflowTruncated(t *testing.T) {
	mockStore := &mockChronicleStore{
		project: &domain.Project{
			ID:             "p_protected_overflow",
			Title:          "截断标记验证录",
			TargetPlatform: "起点仙侠",
			WorldRules:     "天地不仁以万物为刍狗，大道五十天衍四九，仙道贵生无量度人",
		},
		chapters: []*domain.Chapter{
			{
				ChapterIndex: 1,
				Title:        "第一章 开篇",
				CoreConflict: "天机骤变",
				Content:      "夜色渐浓，丹炉内的药香愈发纯粹。韩立长舒一口气，盖上了鼎盖。",
			},
		},
	}

	chronicle := NewCanonChronicle(mockStore)
	chronicle.SetBudgetTokens(1)

	horizon, trace, err := chronicle.AssembleHorizonTraced(context.Background(), "p_protected_overflow", 2)
	if err == nil {
		t.Fatalf("expected protected-overflow error with budget 1, got nil (horizon=%v)", horizon)
	}
	if horizon != nil {
		t.Errorf("expected nil horizon on protected overflow, got %v", horizon)
	}
	if trace == nil {
		t.Fatalf("expected non-nil trace on protected overflow")
	}
	if !trace.Truncated {
		t.Errorf("expected trace.Truncated=true on protected overflow, got false (estimated=%d budget=%d)", trace.EstimatedTokens, trace.BudgetTokens)
	}
	if len(trace.ProtectedIDs) == 0 {
		t.Errorf("expected trace.ProtectedIDs to be recorded on protected overflow")
	}
}

// --- acctfix additions (append-only): anchor + relations accounting ---

// vaultAnchorAcctStore wraps mockChronicleStore with an in-memory StyleVault so
// AssembleHorizonTraced takes the StyleVaultStore branch without SQLite.
type vaultAnchorAcctStore struct {
	*mockChronicleStore
	vault *domain.StyleVault
}

func (s *vaultAnchorAcctStore) GetStyleVault(ctx context.Context, projectID string) (*domain.StyleVault, error) {
	if s.vault != nil {
		return s.vault, nil
	}
	return &domain.StyleVault{ProjectID: projectID}, nil
}

func (s *vaultAnchorAcctStore) SaveStyleVault(ctx context.Context, vault *domain.StyleVault) error {
	s.vault = vault
	return nil
}

func (s *vaultAnchorAcctStore) SetGoldenExcerpts(ctx context.Context, projectID string, excerpts []string) error {
	s.vault = &domain.StyleVault{ProjectID: projectID, GoldenExcerpts: excerpts}
	return nil
}

func newAcctAnchorBase() *mockChronicleStore {
	return &mockChronicleStore{
		project: &domain.Project{
			ID:             "p_acct_anchor",
			Title:          "锚记账录",
			TargetPlatform: "起点仙侠",
			WorldRules:     "天道有常",
		},
		chapters: []*domain.Chapter{
			{
				ChapterIndex: 1,
				Title:        "第一章 开篇",
				CoreConflict: "天机骤变",
				Content:      "开篇正文内容，山雨欲来风满楼。",
			},
		},
	}
}

// TestCanonChronicle_AcctFix_StyleVaultAnchorAccounted verifies the anchor
// secondary gate: with a seeded vault, EstimatedTokens must include the anchor
// bytes, and a budget that fits protected items but not the anchor must fail
// closed with style/vault_anchor in ProtectedIDs.
func TestCanonChronicle_AcctFix_StyleVaultAnchorAccounted(t *testing.T) {
	vault := &domain.StyleVault{
		ProjectID: "p_acct_anchor",
		GoldenExcerpts: []string{
			"苍山负雪，松涛如潮，剑气纵横三万里。",
			"残阳似血，孤雁长鸣，古道西风瘦马。",
			"夜阑听雨，烛火摇曳，旧友书信迟迟未至。",
		},
	}
	anchorText := BuildStyleAnchor(vault)
	if anchorText == "" {
		t.Fatalf("expected non-empty anchor from seeded vault")
	}
	anchorTokens := domain.EstimateTokens(anchorText)
	if anchorTokens <= 0 {
		t.Fatalf("expected positive anchor tokens, got %d", anchorTokens)
	}

	// Full-budget runs with/without vault: diff must equal anchor tokens exactly.
	fullChron := NewCanonChronicle(&vaultAnchorAcctStore{mockChronicleStore: newAcctAnchorBase(), vault: vault})
	fullChron.SetBudgetTokens(12000)
	horizonFull, traceFull, err := fullChron.AssembleHorizonTraced(context.Background(), "p_acct_anchor", 2)
	if err != nil {
		t.Fatalf("full-budget run with vault failed: %v", err)
	}
	if horizonFull.StyleVaultAnchor != anchorText {
		t.Errorf("expected StyleVaultAnchor to equal BuildStyleAnchor(vault), got %q", horizonFull.StyleVaultAnchor)
	}

	emptyChron := NewCanonChronicle(&vaultAnchorAcctStore{
		mockChronicleStore: newAcctAnchorBase(),
		vault:              &domain.StyleVault{ProjectID: "p_acct_anchor"},
	})
	emptyChron.SetBudgetTokens(12000)
	_, traceEmpty, err := emptyChron.AssembleHorizonTraced(context.Background(), "p_acct_anchor", 2)
	if err != nil {
		t.Fatalf("full-budget run without vault failed: %v", err)
	}
	if got := traceFull.EstimatedTokens - traceEmpty.EstimatedTokens; got != anchorTokens {
		t.Errorf("anchor bytes not fully accounted: EstimatedTokens diff=%d, want anchorTokens=%d", got, anchorTokens)
	}

	// Probe exact protected tokens via the fail-closed budget-1 path, then set
	// budget to exactly protected so protected passes but the anchor overflows.
	probeChron := NewCanonChronicle(&vaultAnchorAcctStore{mockChronicleStore: newAcctAnchorBase(), vault: vault})
	probeChron.SetBudgetTokens(1)
	_, traceProbe, errProbe := probeChron.AssembleHorizonTraced(context.Background(), "p_acct_anchor", 2)
	if errProbe == nil || traceProbe == nil {
		t.Fatalf("expected protected-overflow probe error, got err=%v", errProbe)
	}
	protectedTokens := traceProbe.EstimatedTokens
	if protectedTokens <= 0 {
		t.Fatalf("expected positive protected tokens, got %d", protectedTokens)
	}

	tinyChron := NewCanonChronicle(&vaultAnchorAcctStore{mockChronicleStore: newAcctAnchorBase(), vault: vault})
	tinyChron.SetBudgetTokens(protectedTokens)
	horizonTiny, traceTiny, errTiny := tinyChron.AssembleHorizonTraced(context.Background(), "p_acct_anchor", 2)
	if errTiny == nil {
		t.Fatalf("expected anchor secondary-gate error with budget==protected (%d), got nil", protectedTokens)
	}
	if horizonTiny != nil {
		t.Errorf("expected nil horizon on anchor overflow, got %v", horizonTiny)
	}
	if traceTiny == nil {
		t.Fatalf("expected non-nil trace on anchor overflow")
	}
	if !traceTiny.Truncated {
		t.Errorf("expected trace.Truncated=true on anchor overflow")
	}
	foundAnchor := false
	for _, id := range traceTiny.ProtectedIDs {
		if id == "style/vault_anchor" {
			foundAnchor = true
			break
		}
	}
	if !foundAnchor {
		t.Errorf("expected trace.ProtectedIDs to contain style/vault_anchor, got %v", traceTiny.ProtectedIDs)
	}
}

// TestCanonChronicle_AcctFix_CodexRelationsAccounted verifies relations bytes
// are counted pre-loop: two entities plus one relation row must raise
// EstimatedTokens by exactly the embedded relations segment.
func TestCanonChronicle_AcctFix_CodexRelationsAccounted(t *testing.T) {
	rel := domain.EntityRelation{
		ID:            "rel-acct-1",
		ProjectID:     "p_acct_rel",
		SourceEntryID: "e-alice",
		TargetEntryID: "e-bob",
		TargetName:    "罗老二",
		RelationType:  "NEMESIS",
		Description:   "夺妻之恨不共戴天",
	}
	newBase := func(withRel bool) *mockChronicleStore {
		m := &mockChronicleStore{
			project: &domain.Project{
				ID:             "p_acct_rel",
				Title:          "羁绊记账录",
				TargetPlatform: "起点仙侠",
				WorldRules:     "有仇必报",
			},
			chapters: []*domain.Chapter{
				{
					ChapterIndex: 1,
					Title:        "第一章 相逢",
					CoreConflict: "狭路相逢",
					Content:      "开篇正文内容，恩怨纠葛初现端倪。",
				},
			},
			codexEntries: []*domain.CodexEntry{
				{
					ID:           "e-alice",
					Name:         "萧老大",
					Category:     domain.CategoryCharacter,
					TrackingMode: domain.TrackingModeAlwaysInject,
					Summary:      "青龙寨大当家",
				},
				{
					ID:           "e-bob",
					Name:         "罗老二",
					Category:     domain.CategoryCharacter,
					TrackingMode: domain.TrackingModeAlwaysInject,
					Summary:      "黑风寨二当家",
				},
			},
		}
		if withRel {
			m.codexRelations = []domain.EntityRelation{rel}
		}
		return m
	}

	relChron := NewCanonChronicle(newBase(true))
	relChron.SetBudgetTokens(12000)
	horizonRel, traceRel, err := relChron.AssembleHorizonTraced(context.Background(), "p_acct_rel", 2)
	if err != nil {
		t.Fatalf("run with relation failed: %v", err)
	}

	noRelChron := NewCanonChronicle(newBase(false))
	noRelChron.SetBudgetTokens(12000)
	_, traceNoRel, err := noRelChron.AssembleHorizonTraced(context.Background(), "p_acct_rel", 2)
	if err != nil {
		t.Fatalf("run without relation failed: %v", err)
	}

	relLine := fmt.Sprintf("• [%s] ➔ [%s] (%s)：%s\n", "萧老大", "罗老二", "NEMESIS", "夺妻之恨不共戴天")
	relSeg := "\n【实体间羁绊与冲突事实 (Relations)】\n" + relLine
	if got := traceRel.EstimatedTokens - traceNoRel.EstimatedTokens; got != domain.EstimateTokens(relSeg) {
		t.Errorf("relations bytes not fully accounted: EstimatedTokens diff=%d, want segmentTokens=%d",
			got, domain.EstimateTokens(relSeg))
	}
	if !strings.Contains(horizonRel.CodexContextText, relLine) {
		t.Errorf("expected CodexContextText to contain relation line %q, got:\n%s", relLine, horizonRel.CodexContextText)
	}
	if len(horizonRel.CodexRelations) != 1 {
		t.Errorf("expected 1 retained CodexRelations entry, got %d", len(horizonRel.CodexRelations))
	}
}

// TestCanonChronicle_AcctFix_TracedWithExtras verifies that injected extras (plan/intent)
// are accurately accounted as protected context tokens and fail-closed on budget overflow.
func TestCanonChronicle_AcctFix_TracedWithExtras(t *testing.T) {
	mockStore := newAcctAnchorBase()
	chronicle := NewCanonChronicle(mockStore)
	chronicle.SetBudgetTokens(12000)

	// 1. Run without extras
	horizonBase, traceBase, err := chronicle.AssembleHorizonTraced(context.Background(), "p_acct_anchor", 2)
	if err != nil {
		t.Fatalf("AssembleHorizonTraced failed: %v", err)
	}
	if horizonBase == nil || traceBase == nil {
		t.Fatalf("expected non-nil horizon and trace")
	}

	planBlock := buildPlanPromptBlock("主角假意投降魔教，借雷劫暗算魔尊")
	intent := &domain.ChapterIntent{
		Conflict: "魔宗大殿夺宝",
		POV:      "韩立",
		Status:   domain.IntentStatusLocked,
	}
	intentBlock := buildChapterIntentPromptBlock(intent)

	extraPlan := AccountedExtra{ID: "inject/plan", Text: planBlock}
	extraIntent := AccountedExtra{ID: "inject/intent", Text: intentBlock}
	extraEmpty := AccountedExtra{ID: "inject/empty", Text: ""}

	planTokens := domain.EstimateTokens(planBlock)
	intentTokens := domain.EstimateTokens(intentBlock)

	// 2. Run with extras (including empty extra to test that empty extras do not pollute)
	horizonWithExtras, traceWithExtras, err := chronicle.AssembleHorizonTracedWithExtras(
		context.Background(), "p_acct_anchor", 2, extraPlan, extraIntent, extraEmpty,
	)
	if err != nil {
		t.Fatalf("AssembleHorizonTracedWithExtras failed: %v", err)
	}
	if horizonWithExtras == nil || traceWithExtras == nil {
		t.Fatalf("expected non-nil horizon and trace with extras")
	}

	// Verify tokens are precisely accounted
	expectedTokenDiff := planTokens + intentTokens
	actualTokenDiff := traceWithExtras.EstimatedTokens - traceBase.EstimatedTokens
	if actualTokenDiff != expectedTokenDiff {
		t.Errorf("token diff mismatch: got %d, want %d (plan=%d, intent=%d)",
			actualTokenDiff, expectedTokenDiff, planTokens, intentTokens)
	}

	// Verify ProtectedIDs contains inject/plan and inject/intent, but NOT inject/empty
	hasPlan := false
	hasIntent := false
	hasEmpty := false
	for _, id := range traceWithExtras.ProtectedIDs {
		if id == "inject/plan" {
			hasPlan = true
		}
		if id == "inject/intent" {
			hasIntent = true
		}
		if id == "inject/empty" {
			hasEmpty = true
		}
	}
	if !hasPlan {
		t.Errorf("expected ProtectedIDs to contain 'inject/plan', got %v", traceWithExtras.ProtectedIDs)
	}
	if !hasIntent {
		t.Errorf("expected ProtectedIDs to contain 'inject/intent', got %v", traceWithExtras.ProtectedIDs)
	}
	if hasEmpty {
		t.Errorf("expected ProtectedIDs NOT to contain 'inject/empty', got %v", traceWithExtras.ProtectedIDs)
	}

	// 3. Budget overflow test:
	// Probe exact base protected tokens without extras using budget=1
	chronicleProbe := NewCanonChronicle(mockStore)
	chronicleProbe.SetBudgetTokens(1)
	_, traceProbe, errProbe := chronicleProbe.AssembleHorizonTraced(context.Background(), "p_acct_anchor", 2)
	if errProbe == nil || traceProbe == nil {
		t.Fatalf("expected protected-overflow probe error, got: %v", errProbe)
	}
	baseProtectedTokens := traceProbe.EstimatedTokens

	// Set budget to exactly baseProtectedTokens. Without extras it passes protected check.
	// But with extraPlan it must fail-closed with protected-overflow.
	chronicleOverflow := NewCanonChronicle(mockStore)
	chronicleOverflow.SetBudgetTokens(baseProtectedTokens)

	horizonOF, traceOF, errOF := chronicleOverflow.AssembleHorizonTracedWithExtras(
		context.Background(), "p_acct_anchor", 2, extraPlan,
	)
	if errOF == nil {
		t.Fatalf("expected protected-overflow error when budget (%d) < baseProtected (%d) + extraPlan (%d), got nil",
			baseProtectedTokens, baseProtectedTokens, planTokens)
	}
	if horizonOF != nil {
		t.Errorf("expected nil horizon on protected overflow, got %v", horizonOF)
	}
	if traceOF == nil {
		t.Fatalf("expected non-nil trace on protected overflow")
	}
	if !traceOF.Truncated {
		t.Errorf("expected traceOF.Truncated=true")
	}
	if !strings.Contains(errOF.Error(), "inject/plan") {
		t.Errorf("expected error to mention 'inject/plan', got: %v", errOF)
	}
}

// -----------------------------------------------------------------------------
// P0-1 语义压缩管线 (LLM Compactor) 测试
// -----------------------------------------------------------------------------

func setupCompactionTestFixture() (*CanonChronicle, int) {
	mockStore := &mockChronicleStore{
		project: &domain.Project{
			ID:         "p_compaction_test",
			Title:      "万道征程",
			WorldRules: "天道有常，因果轮回。",
		},
		chapters: []*domain.Chapter{
			{ChapterIndex: 1, Title: "第1章", Content: "起步篇章。"},
			{ChapterIndex: 2, Title: "第2章", Content: "磨砺篇章。"},
			{ChapterIndex: 3, Title: "第3章", Content: "蓄势篇章。"},
			{ChapterIndex: 4, Title: "第4章", Content: "暗涌篇章。"},
		},
		hooks: []*domain.PlotHook{
			{
				ID:             "hook_far_drop",
				Title:          "远期超长伏笔",
				Details:        strings.Repeat("潜藏在深渊地下的古老妖皇封印，牵扯万古大劫暗线因果。", 10),
				CreatedChapter: 1,
				TargetChapter:  30,
				Status:         domain.HookStatusOpen,
			},
		},
	}
	chronicle := NewCanonChronicle(mockStore)
	chronicle.SetBudgetTokens(1)
	_, probeTrace, _ := chronicle.AssembleHorizonTraced(context.Background(), "p_compaction_test", 5)
	baseProtectedTokens := probeTrace.EstimatedTokens

	// Budget accommodates base protected + 150 tokens.
	// hook_far_drop is ~240 tokens, so it will be dropped, leaving 150 tokens remaining (>= MinCompactionBudgetTokens 100).
	budget := baseProtectedTokens + 150
	chronicle.SetBudgetTokens(budget)
	return chronicle, budget
}

// 1. TestCanonChronicle_Compaction_InjectsCompiledMemory：小 budget 逼出丢弃 + SetCompactor 假压缩器（返回预算内摘录）
// → 断言 horizon.HistoricalCallbacks 含 【压缩记忆 与摘录文本、trace.CompressibleIDs 含 memory/compiled-compressible、CompactionFailed==false。
func TestCanonChronicle_Compaction_InjectsCompiledMemory(t *testing.T) {
	chronicle, _ := setupCompactionTestFixture()

	excerpt := "【压缩摘录】妖皇深渊封印因果潜伏未消。"
	compactorCalled := false
	chronicle.SetCompactor(func(ctx context.Context, in CompactionInput) (string, error) {
		compactorCalled = true
		if in.ChapterIndex != 5 {
			t.Errorf("expected ChapterIndex=5, got %d", in.ChapterIndex)
		}
		if len(in.Dropped) == 0 {
			t.Errorf("expected non-empty Dropped items")
		}
		if in.BudgetTokens < MinCompactionBudgetTokens {
			t.Errorf("expected BudgetTokens >= %d, got %d", MinCompactionBudgetTokens, in.BudgetTokens)
		}
		return excerpt, nil
	})

	horizon, trace, err := chronicle.AssembleHorizonTraced(context.Background(), "p_compaction_test", 5)
	if err != nil {
		t.Fatalf("AssembleHorizonTraced failed: %v", err)
	}

	if !compactorCalled {
		t.Fatalf("expected compactor to be called")
	}
	if trace.CompactionFailed {
		t.Errorf("expected CompactionFailed==false, got true (note: %s)", trace.CompactionNote)
	}
	if !strings.Contains(horizon.HistoricalCallbacks, "【压缩记忆") {
		t.Errorf("expected HistoricalCallbacks to contain '【压缩记忆', got:\n%s", horizon.HistoricalCallbacks)
	}
	if !strings.Contains(horizon.HistoricalCallbacks, excerpt) {
		t.Errorf("expected HistoricalCallbacks to contain excerpt %q, got:\n%s", excerpt, horizon.HistoricalCallbacks)
	}

	hasCompiledID := false
	for _, id := range trace.CompressibleIDs {
		if id == "memory/compiled-compressible" {
			hasCompiledID = true
			break
		}
	}
	if !hasCompiledID {
		t.Errorf("expected trace.CompressibleIDs to contain 'memory/compiled-compressible', got: %v", trace.CompressibleIDs)
	}
	if len(trace.DroppedIDs) == 0 {
		t.Errorf("expected trace.DroppedIDs to be non-empty")
	}
}

// 2. TestCanonChronicle_Compaction_FailureKeepsDeterministicDrop：假压缩器返回 error
// → CompactionFailed==true、摘录不在、DroppedIDs 非空。
func TestCanonChronicle_Compaction_FailureKeepsDeterministicDrop(t *testing.T) {
	chronicle, _ := setupCompactionTestFixture()

	chronicle.SetCompactor(func(ctx context.Context, in CompactionInput) (string, error) {
		return "", errors.New("simulated LLM timeout")
	})

	horizon, trace, err := chronicle.AssembleHorizonTraced(context.Background(), "p_compaction_test", 5)
	if err != nil {
		t.Fatalf("AssembleHorizonTraced should not fail fatally on compaction error, got: %v", err)
	}

	if !trace.CompactionFailed {
		t.Errorf("expected CompactionFailed==true, got false")
	}
	if !strings.Contains(trace.CompactionNote, "simulated LLM timeout") {
		t.Errorf("expected CompactionNote to mention error, got: %s", trace.CompactionNote)
	}
	if strings.Contains(horizon.HistoricalCallbacks, "【压缩记忆") {
		t.Errorf("expected HistoricalCallbacks NOT to contain '【压缩记忆', got:\n%s", horizon.HistoricalCallbacks)
	}
	if len(trace.DroppedIDs) == 0 {
		t.Errorf("expected trace.DroppedIDs to be non-empty")
	}
}

// 3. TestCanonChronicle_Compaction_OverBudgetOutputRejected：假压缩器返回超预算长文 → Failed。
func TestCanonChronicle_Compaction_OverBudgetOutputRejected(t *testing.T) {
	chronicle, _ := setupCompactionTestFixture()

	chronicle.SetCompactor(func(ctx context.Context, in CompactionInput) (string, error) {
		// Return text far exceeding BudgetTokens
		return strings.Repeat("超长压缩记忆文本，远超剩余预算上限。", in.BudgetTokens+100), nil
	})

	horizon, trace, err := chronicle.AssembleHorizonTraced(context.Background(), "p_compaction_test", 5)
	if err != nil {
		t.Fatalf("AssembleHorizonTraced should succeed, got: %v", err)
	}

	if !trace.CompactionFailed {
		t.Errorf("expected CompactionFailed==true for over-budget output")
	}
	if !strings.Contains(trace.CompactionNote, "超出剩余预算") {
		t.Errorf("expected CompactionNote to mention '超出剩余预算', got: %s", trace.CompactionNote)
	}
	if strings.Contains(horizon.HistoricalCallbacks, "【压缩记忆") {
		t.Errorf("expected HistoricalCallbacks NOT to contain '【压缩记忆', got:\n%s", horizon.HistoricalCallbacks)
	}
	if len(trace.DroppedIDs) == 0 {
		t.Errorf("expected trace.DroppedIDs to be non-empty")
	}
}

// 4. TestCanonChronicle_Compaction_NoCompactorUnchanged：不 SetCompactor → 无压缩段落、CompactionFailed==false（回归保护）。
func TestCanonChronicle_Compaction_NoCompactorUnchanged(t *testing.T) {
	chronicle, _ := setupCompactionTestFixture()
	// Do not call chronicle.SetCompactor

	horizon, trace, err := chronicle.AssembleHorizonTraced(context.Background(), "p_compaction_test", 5)
	if err != nil {
		t.Fatalf("AssembleHorizonTraced failed: %v", err)
	}

	if trace.CompactionFailed {
		t.Errorf("expected CompactionFailed==false, got true")
	}
	if trace.CompactionNote != "" {
		t.Errorf("expected CompactionNote to be empty, got: %s", trace.CompactionNote)
	}
	if strings.Contains(horizon.HistoricalCallbacks, "【压缩记忆") {
		t.Errorf("expected HistoricalCallbacks NOT to contain '【压缩记忆', got:\n%s", horizon.HistoricalCallbacks)
	}
	if len(trace.DroppedIDs) == 0 {
		t.Errorf("expected trace.DroppedIDs to be non-empty")
	}
}

type mockLLMCompactorClient struct {
	capturedSystemPrompt string
	capturedUserPrompt   string
	capturedModel        string
	capturedTemperature  float64
	returnText           string
	returnErr            error
}

func (m *mockLLMCompactorClient) ChatCompletion(ctx context.Context, model, systemPrompt, userPrompt string, temperature float64) (string, error) {
	return m.returnText, m.returnErr
}

func (m *mockLLMCompactorClient) ChatCompletionWithUsage(ctx context.Context, model, systemPrompt, userPrompt string, temperature float64) (string, TokenUsage, error) {
	m.capturedModel = model
	m.capturedSystemPrompt = systemPrompt
	m.capturedUserPrompt = userPrompt
	m.capturedTemperature = temperature
	return m.returnText, TokenUsage{TotalTokens: 100}, m.returnErr
}

func (m *mockLLMCompactorClient) ChatCompletionStream(ctx context.Context, model, systemPrompt, userPrompt string, temperature float64) (<-chan StreamChunk, error) {
	return nil, nil
}

func TestNewLLMCompactor(t *testing.T) {
	t.Run("nil client returns error", func(t *testing.T) {
		compactor := NewLLMCompactor(nil, "model-x")
		_, err := compactor(context.Background(), CompactionInput{
			ChapterIndex: 1,
			Dropped:      []CompactionItem{{ID: "c1", Kind: "callback", Text: "text"}},
			BudgetTokens: 200,
		})
		if err == nil {
			t.Errorf("expected error when client is nil")
		}
	})

	t.Run("empty dropped returns empty string", func(t *testing.T) {
		mockClient := &mockLLMCompactorClient{}
		compactor := NewLLMCompactor(mockClient, "model-x")
		out, err := compactor(context.Background(), CompactionInput{
			ChapterIndex: 1,
			Dropped:      nil,
			BudgetTokens: 200,
		})
		if err != nil || out != "" {
			t.Errorf("expected empty string and nil error on empty dropped, got %q, %v", out, err)
		}
	})

	t.Run("calls client with correct system prompt and temperature", func(t *testing.T) {
		mockClient := &mockLLMCompactorClient{
			returnText: "  精炼记忆摘录  ",
		}
		compactor := NewLLMCompactor(mockClient, "deepseek-reasoner")
		out, err := compactor(context.Background(), CompactionInput{
			ChapterIndex: 3,
			Dropped: []CompactionItem{
				{ID: "callback/hook_1", Kind: "callback", Text: "伏笔详情文本"},
			},
			BudgetTokens: 150,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out != "精炼记忆摘录" {
			t.Errorf("expected trimmed output '精炼记忆摘录', got %q", out)
		}
		if !strings.HasPrefix(mockClient.capturedSystemPrompt, "你是一名长篇小说的记忆压缩器") {
			t.Errorf("expected system prompt to start with '你是一名长篇小说的记忆压缩器', got %q", mockClient.capturedSystemPrompt)
		}
		if mockClient.capturedTemperature != 0.2 {
			t.Errorf("expected temperature 0.2, got %v", mockClient.capturedTemperature)
		}
		if mockClient.capturedModel != "deepseek-reasoner" {
			t.Errorf("expected model 'deepseek-reasoner', got %s", mockClient.capturedModel)
		}
		if !strings.Contains(mockClient.capturedUserPrompt, "callback/hook_1") {
			t.Errorf("expected user prompt to contain item ID, got %s", mockClient.capturedUserPrompt)
		}
	})
}
