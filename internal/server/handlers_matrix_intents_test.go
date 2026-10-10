package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Jungley8/novel-studio/internal/config"
	"github.com/Jungley8/novel-studio/internal/domain"
	"github.com/Jungley8/novel-studio/internal/engine"
	"github.com/Jungley8/novel-studio/internal/server"
	"github.com/Jungley8/novel-studio/internal/store"
)

func setupTestServerForMatrixIntents(t *testing.T, llmHandler http.HandlerFunc) (*server.Server, store.Store, *httptest.Server) {
	mockServer := httptest.NewServer(llmHandler)
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	cfgPath := filepath.Join(tmpDir, "cfg.json")

	s, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}

	cfg := config.DefaultConfig()
	cfg.APIBase = mockServer.URL
	cfg.APIKey = "sk-test-key"
	cfg.ReasoningModel = "test-reasoner"
	llmClient := engine.NewHTTPLLMClient(mockServer.URL, "sk-test-key")
	orch := engine.NewOrchestrator(llmClient)
	linter := engine.NewLinter(nil)

	srv, err := server.New(cfg, cfgPath, s, llmClient, orch, linter)
	if err != nil {
		t.Fatalf("server.New failed: %v", err)
	}
	return srv, s, mockServer
}

func mockMatrixScenesResponse(scenesJSON string) map[string]any {
	return map[string]any{
		"choices": []map[string]any{
			{
				"message": map[string]string{
					"role":    "assistant",
					"content": scenesJSON,
				},
			},
		},
		"usage": map[string]int{
			"prompt_tokens":     100,
			"completion_tokens": 200,
			"total_tokens":      300,
		},
	}
}

func TestMatrixAIGenerate_WithIntent(t *testing.T) {
	var capturedUserPrompt string
	var mu sync.Mutex

	mockScenesJSON := `[
		{
			"scene_index": 1,
			"title": "破阵前夕",
			"dramatic_goal": "探寻守阵弱点",
			"conflict_barrier": "敌方结界严密阻挡",
			"tension_level": 7,
			"prose_content": "夜色深沉，寒风如刀，主角悄然靠近阵法边缘。"
		},
		{
			"scene_index": 2,
			"title": "绝地反击",
			"dramatic_goal": "摧毁主阵眼",
			"conflict_barrier": "守关大将突然现身",
			"tension_level": 9,
			"prose_content": "长剑破空，阵纹崩碎，惊天对决一触即发。"
		}
	]`

	srv, s, mockServer := setupTestServerForMatrixIntents(t, func(w http.ResponseWriter, r *http.Request) {
		var reqBody struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&reqBody)
		mu.Lock()
		for _, m := range reqBody.Messages {
			if m.Role == "user" {
				capturedUserPrompt = m.Content
			}
		}
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockMatrixScenesResponse(mockScenesJSON))
	})
	defer mockServer.Close()

	ctx := context.Background()
	projectID := "proj-matrix-intent-ai"
	p := &domain.Project{
		ID:             projectID,
		Title:          "矩阵AI拆场意图联动项目",
		TargetPlatform: "起点仙侠",
		WorldRules:     "修真界，宗门林立",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	if err := s.SaveProject(ctx, p); err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}

	// 提前在数据库中为第1章落一张 ChapterIntent
	intentID := "intent-custom-101"
	intent := &domain.ChapterIntent{
		ID:            intentID,
		ProjectID:     projectID,
		VolumeIndex:   1,
		ChapterIndex:  1,
		Conflict:      "宗门大阵遭异族强攻，危机迫在眉睫",
		POV:           "林凡",
		EmotionCurve:  []string{"凝重", "激荡", "决死"},
		TensionTarget: 8,
		HookPlants:    []string{"神秘阵纹异动"},
		HookPayoffs:   []string{"旧伤初愈爆发"},
		Status:        domain.IntentStatusDraft,
	}
	if err := s.SaveChapterIntent(ctx, intent); err != nil {
		t.Fatalf("SaveChapterIntent failed: %v", err)
	}

	// POST /api/projects/{id}/matrix/ai-generate
	// 传空的 CoreConflict，测试：CoreConflict 取自意图，且 Prompt 追加意图约束
	generateReq := engine.MatrixSceneGenerateRequest{
		VolumeIndex:  1,
		ChapterIndex: 1,
		ChapterTitle: "第一章",
		CoreConflict: "", // 留空
		Prompt:       "用户原始补充指令",
	}
	raw, _ := json.Marshal(generateReq)
	req := httptest.NewRequest(http.MethodPost, "/api/projects/"+projectID+"/matrix/ai-generate", bytes.NewReader(raw))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST ai-generate: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var scenes []domain.Scene
	if err := json.Unmarshal(w.Body.Bytes(), &scenes); err != nil {
		t.Fatalf("unmarshal scenes failed: %v", err)
	}
	if len(scenes) != 2 {
		t.Fatalf("expected 2 scenes, got %d", len(scenes))
	}

	// 验证每个 scene 均含 chapter_intent_id
	for i, sc := range scenes {
		if sc.ChapterIntentID != intentID {
			t.Errorf("scene %d expected ChapterIntentID %q, got %q", i, intentID, sc.ChapterIntentID)
		}
		if sc.ChapterID != "ch_1" {
			t.Errorf("scene %d expected ChapterID 'ch_1', got %q", i, sc.ChapterID)
		}
	}

	// 验证 capturedUserPrompt：CoreConflict 取自意图，且包含意图约束
	mu.Lock()
	promptCopy := capturedUserPrompt
	mu.Unlock()

	if !strings.Contains(promptCopy, "宗门大阵遭异族强攻，危机迫在眉睫") {
		t.Errorf("expected CoreConflict in userPrompt taken from intent, prompt was: %s", promptCopy)
	}
	if !strings.Contains(promptCopy, "【章节意图约束】") {
		t.Errorf("expected 【章节意图约束】 in userPrompt, prompt was: %s", promptCopy)
	}
	if !strings.Contains(promptCopy, "视角人物(POV): 林凡") {
		t.Errorf("expected POV constraint in prompt, got: %s", promptCopy)
	}
	if !strings.Contains(promptCopy, "情绪曲线: 凝重 -> 激荡 -> 决死") {
		t.Errorf("expected EmotionCurve constraint in prompt, got: %s", promptCopy)
	}
	if !strings.Contains(promptCopy, "张力目标: 8/10") {
		t.Errorf("expected TensionTarget constraint in prompt, got: %s", promptCopy)
	}
	if !strings.Contains(promptCopy, "埋设伏笔/钩子: 神秘阵纹异动") {
		t.Errorf("expected HookPlants in prompt, got: %s", promptCopy)
	}
	if !strings.Contains(promptCopy, "回收伏笔/钩子: 旧伤初愈爆发") {
		t.Errorf("expected HookPayoffs in prompt, got: %s", promptCopy)
	}
}

func TestMatrixAIGenerate_WithoutIntent_NoIntentID(t *testing.T) {
	mockScenesJSON := `[
		{
			"scene_index": 1,
			"title": "无意图场次",
			"dramatic_goal": "目标",
			"conflict_barrier": "阻碍",
			"tension_level": 5,
			"prose_content": "粗纲正文"
		}
	]`

	srv, s, mockServer := setupTestServerForMatrixIntents(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockMatrixScenesResponse(mockScenesJSON))
	})
	defer mockServer.Close()

	ctx := context.Background()
	projectID := "proj-matrix-no-intent"
	p := &domain.Project{
		ID:        projectID,
		Title:     "无意图矩阵项目",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	_ = s.SaveProject(ctx, p)

	// 第 5 章无 intent
	generateReq := engine.MatrixSceneGenerateRequest{
		VolumeIndex:  1,
		ChapterIndex: 5,
		ChapterTitle: "第五章",
		CoreConflict: "既有核心冲突",
	}
	raw, _ := json.Marshal(generateReq)
	req := httptest.NewRequest(http.MethodPost, "/api/projects/"+projectID+"/matrix/ai-generate", bytes.NewReader(raw))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST ai-generate: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var scenes []domain.Scene
	_ = json.Unmarshal(w.Body.Bytes(), &scenes)
	if len(scenes) != 1 {
		t.Fatalf("expected 1 scene, got %d", len(scenes))
	}
	// 无意图卡时不得打上伪 ChapterIntentID，否则前端「意图联动」徽章会误亮
	if scenes[0].ChapterIntentID != "" {
		t.Errorf("expected empty ChapterIntentID when no intent exists, got %q", scenes[0].ChapterIntentID)
	}
}

func TestMatrixGenerateFromIntents_TwoCardsSuccess(t *testing.T) {
	mockScenesJSON := `[
		{
			"scene_index": 1,
			"title": "场次一",
			"dramatic_goal": "达成第一目标",
			"conflict_barrier": "遭遇阻碍一",
			"tension_level": 6,
			"prose_content": "场次内容粗纲一"
		},
		{
			"scene_index": 2,
			"title": "场次二",
			"dramatic_goal": "达成第二目标",
			"conflict_barrier": "遭遇阻碍二",
			"tension_level": 8,
			"prose_content": "场次内容粗纲二"
		}
	]`

	var llmCallCount int32
	srv, s, mockServer := setupTestServerForMatrixIntents(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&llmCallCount, 1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockMatrixScenesResponse(mockScenesJSON))
	})
	defer mockServer.Close()

	ctx := context.Background()
	projectID := "proj-gen-from-intents"
	p := &domain.Project{
		ID:             projectID,
		Title:          "从意图批量拆场项目",
		TargetPlatform: "起点仙侠",
		WorldRules:     "仙侠世界",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	if err := s.SaveProject(ctx, p); err != nil {
		t.Fatalf("SaveProject failed: %v", err)
	}

	// 存入真实已归档章节以测试 real ChapterID 解析
	ch1 := &domain.Chapter{
		ID:           "chap_real_id_1",
		ProjectID:    projectID,
		ChapterIndex: 1,
		Title:        "真实第一章",
		CreatedAt:    time.Now(),
	}
	ch2 := &domain.Chapter{
		ID:           "chap_real_id_2",
		ProjectID:    projectID,
		ChapterIndex: 2,
		Title:        "真实第二章",
		CreatedAt:    time.Now(),
	}
	_ = s.SaveChapter(ctx, ch1)
	_ = s.SaveChapter(ctx, ch2)

	// 录入 2 张卷 1 的意图卡 (第 1 章与第 2 章)
	card1 := &domain.ChapterIntent{
		ID:            "intent-vol1-ch1",
		ProjectID:     projectID,
		VolumeIndex:   1,
		ChapterIndex:  1,
		Conflict:      "第一章核心矛盾：家族刁难",
		POV:           "主角",
		TensionTarget: 7,
		Status:        domain.IntentStatusDraft,
	}
	card2 := &domain.ChapterIntent{
		ID:            "intent-vol1-ch2",
		ProjectID:     projectID,
		VolumeIndex:   1,
		ChapterIndex:  2,
		Conflict:      "第二章核心矛盾：初入深林遇险",
		POV:           "主角",
		TensionTarget: 8,
		Status:        domain.IntentStatusDraft,
	}
	if err := s.SaveChapterIntent(ctx, card1); err != nil {
		t.Fatalf("SaveChapterIntent 1 failed: %v", err)
	}
	if err := s.SaveChapterIntent(ctx, card2); err != nil {
		t.Fatalf("SaveChapterIntent 2 failed: %v", err)
	}

	// POST /api/projects/{id}/matrix/generate-from-intents {"volume_index": 1}
	reqBody, _ := json.Marshal(map[string]int{"volume_index": 1})
	req := httptest.NewRequest(http.MethodPost, "/api/projects/"+projectID+"/matrix/generate-from-intents", bytes.NewReader(reqBody))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST generate-from-intents: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		VolumeIndex int `json:"volume_index"`
		TotalScenes int `json:"total_scenes"`
		Results     []struct {
			ChapterIndex  int    `json:"chapter_index"`
			OK            bool   `json:"ok"`
			ScenesCreated int    `json:"scenes_created"`
			Error         string `json:"error"`
		} `json:"results"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal resp failed: %v", err)
	}

	if resp.VolumeIndex != 1 {
		t.Errorf("expected volume_index 1, got %d", resp.VolumeIndex)
	}
	if resp.TotalScenes != 4 {
		t.Errorf("expected total_scenes 4, got %d", resp.TotalScenes)
	}
	if len(resp.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(resp.Results))
	}

	r1 := resp.Results[0]
	if r1.ChapterIndex != 1 || !r1.OK || r1.ScenesCreated != 2 || r1.Error != "" {
		t.Errorf("unexpected result 0: %+v", r1)
	}

	r2 := resp.Results[1]
	if r2.ChapterIndex != 2 || !r2.OK || r2.ScenesCreated != 2 || r2.Error != "" {
		t.Errorf("unexpected result 1: %+v", r2)
	}

	if atomic.LoadInt32(&llmCallCount) != 2 {
		t.Errorf("expected 2 LLM calls, got %d", atomic.LoadInt32(&llmCallCount))
	}

	// 验证库中生成保存的 scenes
	scenes1, err := s.ListScenes(ctx, "chap_real_id_1")
	if err != nil {
		t.Fatalf("ListScenes ch1 failed: %v", err)
	}
	if len(scenes1) != 2 {
		t.Fatalf("expected 2 scenes for chapter 1, got %d", len(scenes1))
	}

	scenes2, err := s.ListScenes(ctx, "chap_real_id_2")
	if err != nil {
		t.Fatalf("ListScenes ch2 failed: %v", err)
	}
	if len(scenes2) != 2 {
		t.Fatalf("expected 2 scenes for chapter 2, got %d", len(scenes2))
	}
}

func TestMatrixGenerateFromIntents_EmptyIntents400(t *testing.T) {
	srv, s, mockServer := setupTestServerForMatrixIntents(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{}})
	})
	defer mockServer.Close()

	ctx := context.Background()
	projectID := "proj-matrix-empty-intents"
	p := &domain.Project{
		ID:        projectID,
		Title:     "空意图矩阵测试",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	_ = s.SaveProject(ctx, p)

	// 卷 1 没有意图卡 -> 400 "该卷暂无意图卡，请先推演"
	reqBody, _ := json.Marshal(map[string]int{"volume_index": 1})
	req := httptest.NewRequest(http.MethodPost, "/api/projects/"+projectID+"/matrix/generate-from-intents", bytes.NewReader(reqBody))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty volume intents, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "该卷暂无意图卡，请先推演") {
		t.Errorf("expected error message containing '该卷暂无意图卡，请先推演', got %s", w.Body.String())
	}
}

func TestMatrixGenerateFromIntents_MethodNotAllowed405(t *testing.T) {
	srv, s, mockServer := setupTestServerForMatrixIntents(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{})
	})
	defer mockServer.Close()

	ctx := context.Background()
	projectID := "proj-matrix-method-check"
	p := &domain.Project{
		ID:        projectID,
		Title:     "方法校验测试",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	_ = s.SaveProject(ctx, p)

	// GET generate-from-intents -> 405
	req := httptest.NewRequest(http.MethodGet, "/api/projects/"+projectID+"/matrix/generate-from-intents", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 Method Not Allowed for GET, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMatrixGenerateFromIntents_PartialFailureContinues(t *testing.T) {
	var callCount int32
	srv, s, mockServer := setupTestServerForMatrixIntents(t, func(w http.ResponseWriter, r *http.Request) {
		idx := atomic.AddInt32(&callCount, 1)
		if idx == 1 {
			// 第一章 LLM 报错
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error": "LLM upstream rate limit"}`))
			return
		}
		// 第二章成功
		w.Header().Set("Content-Type", "application/json")
		mockScenesJSON := `[
			{
				"scene_index": 1,
				"title": "成功场次",
				"dramatic_goal": "达成目标",
				"conflict_barrier": "阻碍",
				"tension_level": 7,
				"prose_content": "内容"
			}
		]`
		_ = json.NewEncoder(w).Encode(mockMatrixScenesResponse(mockScenesJSON))
	})
	defer mockServer.Close()

	ctx := context.Background()
	projectID := "proj-matrix-partial-err"
	p := &domain.Project{
		ID:        projectID,
		Title:     "部分失败测试",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	_ = s.SaveProject(ctx, p)

	card1 := &domain.ChapterIntent{
		ID:            "intent-part-ch1",
		ProjectID:     projectID,
		VolumeIndex:   1,
		ChapterIndex:  1,
		Conflict:      "第一章冲突",
		Status:        domain.IntentStatusDraft,
		TensionTarget: 6,
	}
	card2 := &domain.ChapterIntent{
		ID:            "intent-part-ch2",
		ProjectID:     projectID,
		VolumeIndex:   1,
		ChapterIndex:  2,
		Conflict:      "第二章冲突",
		Status:        domain.IntentStatusDraft,
		TensionTarget: 7,
	}
	_ = s.SaveChapterIntent(ctx, card1)
	_ = s.SaveChapterIntent(ctx, card2)

	reqBody, _ := json.Marshal(map[string]int{"volume_index": 1})
	req := httptest.NewRequest(http.MethodPost, "/api/projects/"+projectID+"/matrix/generate-from-intents", bytes.NewReader(reqBody))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 even with partial failure, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		VolumeIndex int `json:"volume_index"`
		TotalScenes int `json:"total_scenes"`
		Results     []struct {
			ChapterIndex  int    `json:"chapter_index"`
			OK            bool   `json:"ok"`
			ScenesCreated int    `json:"scenes_created"`
			Error         string `json:"error"`
		} `json:"results"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal resp failed: %v", err)
	}

	if resp.TotalScenes != 1 {
		t.Errorf("expected total_scenes 1, got %d", resp.TotalScenes)
	}
	if len(resp.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(resp.Results))
	}
	if resp.Results[0].OK {
		t.Errorf("expected result 0 OK=false on LLM error")
	}
	if !resp.Results[1].OK || resp.Results[1].ScenesCreated != 1 {
		t.Errorf("expected result 1 OK=true with 1 scene, got: %+v", resp.Results[1])
	}
}
