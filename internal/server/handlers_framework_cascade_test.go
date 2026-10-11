package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Jungley8/novel-studio/internal/domain"
	"github.com/Jungley8/novel-studio/internal/server"
	"github.com/Jungley8/novel-studio/internal/store"
)

func setupCascadeProject(t *testing.T) (*server.Server, store.Store, string, domain.ProjectFramework) {
	srv, s, _ := setupTestServer(t)
	ctx := context.Background()

	initialFw := domain.ProjectFramework{
		ThemePremise: "凡人逆天破命",
		WorldAxioms:  []string{"公理一：灵气守恒", "公理二：神魂不灭"},
		PowerLadder: []domain.PowerLadderTier{
			{Tier: 1, Realm: "练气", Description: "引气入体"},
			{Tier: 2, Realm: "筑基", Description: "道基初成"},
		},
		VolumeArcs: []domain.VolumeArc{
			{VolumeIndex: 1, Title: "第一卷 边陲少年", StartChapter: 1, EndChapter: 10},
			{VolumeIndex: 2, Title: "第二卷 宗门试炼", StartChapter: 11, EndChapter: 20},
		},
	}

	projectID := "proj-cascade-" + strings.ReplaceAll(t.Name(), "/", "-")
	p := &domain.Project{
		ID:             projectID,
		Title:          "级联测试小说",
		TargetPlatform: "通用网文",
		Framework:      &initialFw,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	if err := s.SaveProject(ctx, p); err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}

	intent1 := &domain.ChapterIntent{
		ProjectID:     projectID,
		ChapterIndex:  1,
		VolumeIndex:   1,
		Conflict:      "少年觉醒",
		TensionTarget: 7,
		Status:        domain.IntentStatusLocked,
	}
	if err := s.SaveChapterIntent(ctx, intent1); err != nil {
		t.Fatalf("SaveChapterIntent 1 failed: %v", err)
	}

	intent2 := &domain.ChapterIntent{
		ProjectID:     projectID,
		ChapterIndex:  11,
		VolumeIndex:   2,
		Conflict:      "初入宗门",
		TensionTarget: 8,
		Status:        domain.IntentStatusLocked,
	}
	if err := s.SaveChapterIntent(ctx, intent2); err != nil {
		t.Fatalf("SaveChapterIntent 2 failed: %v", err)
	}

	return srv, s, projectID, initialFw
}

func putFramework(t *testing.T, srv *server.Server, projectID string, fw domain.ProjectFramework) {
	raw, err := json.Marshal(fw)
	if err != nil {
		t.Fatalf("marshal framework failed: %v", err)
	}
	req := httptest.NewRequest(http.MethodPut, "/api/projects/"+projectID+"/framework", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("PUT /api/projects/%s/framework expected 200, got %d: %s", projectID, w.Code, w.Body.String())
	}
}

func getIntentStatus(t *testing.T, s store.Store, projectID string, chapterIndex int) domain.IntentStatus {
	ctx := context.Background()
	intent, err := s.GetChapterIntent(ctx, projectID, chapterIndex)
	if err != nil {
		t.Fatalf("GetChapterIntent failed for chapter %d: %v", chapterIndex, err)
	}
	if intent == nil {
		t.Fatalf("intent not found for chapter %d", chapterIndex)
	}
	return intent.Status
}

func TestFrameworkCascade(t *testing.T) {
	t.Run("Case1_AxiomsChange_AllIntentsStale", func(t *testing.T) {
		srv, s, projectID, fw := setupCascadeProject(t)
		fw.WorldAxioms = []string{"公理一：灵气守恒", "公理二：神魂可灭"}
		putFramework(t, srv, projectID, fw)

		status1 := getIntentStatus(t, s, projectID, 1)
		status2 := getIntentStatus(t, s, projectID, 11)
		if status1 != domain.IntentStatusStale {
			t.Errorf("expected intent 1 status STALE, got %s", status1)
		}
		if status2 != domain.IntentStatusStale {
			t.Errorf("expected intent 2 status STALE, got %s", status2)
		}
	})

	t.Run("Case2_LadderChange_AllIntentsStale", func(t *testing.T) {
		srv, s, projectID, fw := setupCascadeProject(t)
		fw.PowerLadder = []domain.PowerLadderTier{
			{Tier: 1, Realm: "练气", Description: "引气入体"},
			{Tier: 2, Realm: "筑基", Description: "道基大成"},
		}
		putFramework(t, srv, projectID, fw)

		status1 := getIntentStatus(t, s, projectID, 1)
		status2 := getIntentStatus(t, s, projectID, 11)
		if status1 != domain.IntentStatusStale {
			t.Errorf("expected intent 1 status STALE, got %s", status1)
		}
		if status2 != domain.IntentStatusStale {
			t.Errorf("expected intent 2 status STALE, got %s", status2)
		}
	})

	t.Run("Case3_PremiseChange_AllIntentsStale", func(t *testing.T) {
		srv, s, projectID, fw := setupCascadeProject(t)
		fw.ThemePremise = "顺天应命，无为而治"
		putFramework(t, srv, projectID, fw)

		status1 := getIntentStatus(t, s, projectID, 1)
		status2 := getIntentStatus(t, s, projectID, 11)
		if status1 != domain.IntentStatusStale {
			t.Errorf("expected intent 1 status STALE, got %s", status1)
		}
		if status2 != domain.IntentStatusStale {
			t.Errorf("expected intent 2 status STALE, got %s", status2)
		}
	})

	t.Run("Case4_VolumeRangeChange_OnlyChangedVolumeStale", func(t *testing.T) {
		srv, s, projectID, fw := setupCascadeProject(t)
		fw.VolumeArcs = []domain.VolumeArc{
			{VolumeIndex: 1, Title: "第一卷 边陲少年", StartChapter: 1, EndChapter: 10},
			{VolumeIndex: 2, Title: "第二卷 宗门试炼", StartChapter: 11, EndChapter: 25},
		}
		putFramework(t, srv, projectID, fw)

		status1 := getIntentStatus(t, s, projectID, 1)
		status2 := getIntentStatus(t, s, projectID, 11)
		if status1 != domain.IntentStatusLocked {
			t.Errorf("expected intent 1 to remain LOCKED, got %s", status1)
		}
		if status2 != domain.IntentStatusStale {
			t.Errorf("expected intent 2 status STALE, got %s", status2)
		}
	})

	t.Run("Case5_NoChange_AllRemainLocked", func(t *testing.T) {
		srv, s, projectID, fw := setupCascadeProject(t)
		putFramework(t, srv, projectID, fw)

		status1 := getIntentStatus(t, s, projectID, 1)
		status2 := getIntentStatus(t, s, projectID, 11)
		if status1 != domain.IntentStatusLocked {
			t.Errorf("expected intent 1 to remain LOCKED, got %s", status1)
		}
		if status2 != domain.IntentStatusLocked {
			t.Errorf("expected intent 2 to remain LOCKED, got %s", status2)
		}
	})

	t.Run("Case6_DraftIntent_RemainsDraftOnAxiomsChange", func(t *testing.T) {
		srv, s, projectID, fw := setupCascadeProject(t)
		ctx := context.Background()

		draftIntent := &domain.ChapterIntent{
			ProjectID:     projectID,
			ChapterIndex:  2,
			VolumeIndex:   1,
			Conflict:      "草稿冲突",
			TensionTarget: 5,
			Status:        domain.IntentStatusDraft,
		}
		if err := s.SaveChapterIntent(ctx, draftIntent); err != nil {
			t.Fatalf("SaveChapterIntent draft failed: %v", err)
		}

		fw.WorldAxioms = []string{"公理一：灵气守恒", "公理二：神魂可灭"}
		putFramework(t, srv, projectID, fw)

		draftStatus := getIntentStatus(t, s, projectID, 2)
		if draftStatus != domain.IntentStatusDraft {
			t.Errorf("expected draft intent to remain DRAFT, got %s", draftStatus)
		}
		lockedStatus := getIntentStatus(t, s, projectID, 1)
		if lockedStatus != domain.IntentStatusStale {
			t.Errorf("expected locked intent 1 to become STALE, got %s", lockedStatus)
		}
	})
}
