package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Jungley8/novel-studio/internal/domain"
)

func TestModifyFramework_StoryMilestonesValidation(t *testing.T) {
	srv, s, projectID, initialFw := setupCascadeProject(t)
	ctx := context.Background()

	// 1. 非法里程碑 PUT -> 400 且库中未保存
	invalidFw := initialFw
	invalidFw.TotalPlannedChapters = 50
	invalidFw.StoryMilestones = []domain.StoryMilestone{
		{Key: "CLIMAX", Name: "高潮", TargetChapter: 100, Status: "PENDING"}, // target (100) > total (50) -> 非法
	}

	rawInvalid, err := json.Marshal(invalidFw)
	if err != nil {
		t.Fatalf("marshal invalidFw failed: %v", err)
	}
	reqInvalid := httptest.NewRequest(http.MethodPut, "/api/projects/"+projectID+"/framework", bytes.NewReader(rawInvalid))
	reqInvalid.Header.Set("Content-Type", "application/json")
	wInvalid := httptest.NewRecorder()
	srv.ServeHTTP(wInvalid, reqInvalid)

	if wInvalid.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid milestones PUT, got %d: %s", wInvalid.Code, wInvalid.Body.String())
	}

	// 校验库中仍为旧数据，未被破坏或保存
	projAfterInvalid, err := s.GetProject(ctx, projectID)
	if err != nil {
		t.Fatalf("GetProject failed: %v", err)
	}
	if projAfterInvalid.Framework != nil && len(projAfterInvalid.Framework.StoryMilestones) > 0 {
		t.Fatalf("expected store framework story_milestones to remain empty, got %+v", projAfterInvalid.Framework.StoryMilestones)
	}

	// 2. 合法里程碑 PUT -> 200 且 GET 回读一致
	validFw := initialFw
	validFw.Logline = "边荒少年顾渊隐忍弑神破局"
	validFw.AtmosphereAnchor = "阴冷民俗废土"
	validFw.TotalPlannedChapters = 100
	validFw.StoryMilestones = []domain.StoryMilestone{
		{Key: "OPENING_HOOK", Name: "开局钩子", TargetChapter: 1, Status: domain.MilestoneStatusPending, Note: "开篇断臂"},
		{Key: "MIDPOINT", Name: "中点", TargetChapter: 50, Status: domain.MilestoneStatusPending, Note: "洞悉真相"},
		{Key: "CLIMAX", Name: "全书高潮", TargetChapter: 90, Status: domain.MilestoneStatusDone, Note: "诛灭神明"},
	}

	rawValid, err := json.Marshal(validFw)
	if err != nil {
		t.Fatalf("marshal validFw failed: %v", err)
	}
	reqValid := httptest.NewRequest(http.MethodPut, "/api/projects/"+projectID+"/framework", bytes.NewReader(rawValid))
	reqValid.Header.Set("Content-Type", "application/json")
	wValid := httptest.NewRecorder()
	srv.ServeHTTP(wValid, reqValid)

	if wValid.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid milestones PUT, got %d: %s", wValid.Code, wValid.Body.String())
	}

	// GET 回读一致
	reqGet := httptest.NewRequest(http.MethodGet, "/api/projects/"+projectID+"/framework", nil)
	wGet := httptest.NewRecorder()
	srv.ServeHTTP(wGet, reqGet)

	if wGet.Code != http.StatusOK {
		t.Fatalf("expected 200 for GET framework, got %d: %s", wGet.Code, wGet.Body.String())
	}

	var readFw domain.ProjectFramework
	if err := json.Unmarshal(wGet.Body.Bytes(), &readFw); err != nil {
		t.Fatalf("unmarshal read framework failed: %v", err)
	}

	if readFw.Logline != validFw.Logline {
		t.Errorf("expected Logline=%q, got %q", validFw.Logline, readFw.Logline)
	}
	if readFw.AtmosphereAnchor != validFw.AtmosphereAnchor {
		t.Errorf("expected AtmosphereAnchor=%q, got %q", validFw.AtmosphereAnchor, readFw.AtmosphereAnchor)
	}
	if readFw.TotalPlannedChapters != validFw.TotalPlannedChapters {
		t.Errorf("expected TotalPlannedChapters=%d, got %d", validFw.TotalPlannedChapters, readFw.TotalPlannedChapters)
	}
	if len(readFw.StoryMilestones) != 3 {
		t.Fatalf("expected 3 milestones, got %d", len(readFw.StoryMilestones))
	}
	for i := range validFw.StoryMilestones {
		if readFw.StoryMilestones[i].Key != validFw.StoryMilestones[i].Key ||
			readFw.StoryMilestones[i].TargetChapter != validFw.StoryMilestones[i].TargetChapter ||
			readFw.StoryMilestones[i].Status != validFw.StoryMilestones[i].Status {
			t.Errorf("milestone [%d] mismatch: got %+v, want %+v", i, readFw.StoryMilestones[i], validFw.StoryMilestones[i])
		}
	}
}
