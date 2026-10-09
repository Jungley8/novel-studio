package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Jungley8/novel-studio/internal/config"
	"github.com/Jungley8/novel-studio/internal/domain"
	"github.com/Jungley8/novel-studio/internal/engine"
	"github.com/Jungley8/novel-studio/internal/server"
	"github.com/Jungley8/novel-studio/internal/store"
)

func setupTestServer(t *testing.T) (*server.Server, store.Store, *config.Config) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	cfgPath := filepath.Join(tmpDir, "cfg.json")

	s, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}

	cfg := config.DefaultConfig()
	llmClient := engine.NewHTTPLLMClient(cfg.APIBase, cfg.APIKey)
	orch := engine.NewOrchestrator(llmClient)
	linter := engine.NewLinter(nil)

	srv, err := server.New(cfg, cfgPath, s, llmClient, orch, linter)
	if err != nil {
		t.Fatalf("server.New failed: %v", err)
	}
	return srv, s, cfg
}

func TestServer_ConfigAndProjects(t *testing.T) {
	srv, _, cfg := setupTestServer(t)

	// 1. GET /api/config & Masking Verification
	// Set real key first
	cfg.APIKey = "sk-1234567890abcdef"
	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var gotCfg config.Config
	_ = json.Unmarshal(w.Body.Bytes(), &gotCfg)
	if gotCfg.APIKey == "sk-1234567890abcdef" || !strings.Contains(gotCfg.APIKey, "...") {
		t.Errorf("expected masked API key in GET /api/config, got %s", gotCfg.APIKey)
	}

	// 1.1 POST /api/config with masked key must NOT overwrite real key
	updatePayload := gotCfg
	updatePayload.WriterModel = "deepseek-chat-v2"
	bodyUp, _ := json.Marshal(updatePayload)
	reqUp := httptest.NewRequest(http.MethodPost, "/api/config", bytes.NewReader(bodyUp))
	wUp := httptest.NewRecorder()
	srv.ServeHTTP(wUp, reqUp)
	if wUp.Code != http.StatusOK {
		t.Fatalf("expected 200 on config update, got %d", wUp.Code)
	}
	if cfg.APIKey != "sk-1234567890abcdef" {
		t.Errorf("real API key was clobbered by masked value: %s", cfg.APIKey)
	}
	if cfg.WriterModel != "deepseek-chat-v2" {
		t.Errorf("writer model was not updated: %s", cfg.WriterModel)
	}

	// 2. POST /api/projects
	projPayload := domain.Project{
		Title:          "万古神帝",
		TargetPlatform: "起点仙侠",
		Protagonist: domain.Protagonist{
			NameAndLevel: "张若尘 (黄极境)",
			Inventory:    "沉渊古剑",
		},
	}
	body, _ := json.Marshal(projPayload)
	req = httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader(body))
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", w.Code, w.Body.String())
	}

	var created domain.Project
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	if created.Title != "万古神帝" {
		t.Errorf("expected Title 万古神帝, got %s", created.Title)
	}

	// 3. POST /api/projects/{id}/chapters (Atomic Commit Chapter with State Mutation)
	chPayload := domain.Chapter{
		ChapterIndex: 1,
		Title:        "第一章 觉醒",
		Content:      "少年破茧而出，剑意冲霄。",
		StateMutation: domain.StateMutation{
			InventoryDelta: "+时空晶石x1",
			PowerDelta:     "玄极境初期",
		},
	}
	body, _ = json.Marshal(chPayload)
	req = httptest.NewRequest(http.MethodPost, "/api/projects/"+created.ID+"/chapters", bytes.NewReader(body))
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for chapter commit, got %d: %s", w.Code, w.Body.String())
	}

	var commitResp struct {
		Chapter domain.Chapter `json:"chapter"`
		Project domain.Project `json:"project"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &commitResp)
	if commitResp.Project.Protagonist.Inventory != "沉渊古剑, 时空晶石" {
		t.Errorf("unexpected mutated inventory: %s", commitResp.Project.Protagonist.Inventory)
	}
	if len(commitResp.Project.Protagonist.StructuredItems) != 2 || commitResp.Project.Protagonist.StructuredItems[1].Quantity != 1 {
		t.Errorf("unexpected structured items: %+v", commitResp.Project.Protagonist.StructuredItems)
	}

	// 4. POST /api/linter/analyze
	lintPayload := map[string]string{"text": "他不由得冷笑了一声。"}
	body, _ = json.Marshal(lintPayload)
	req = httptest.NewRequest(http.MethodPost, "/api/linter/analyze", bytes.NewReader(body))
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var linterRes domain.LinterResult
	_ = json.Unmarshal(w.Body.Bytes(), &linterRes)
	if len(linterRes.HitBannedWords) == 0 {
		t.Errorf("expected hit banned words for cliché")
	}

	// 5. GET / (static index.html)
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for index.html, got %d", w.Code)
	}

	// 6. POST /api/projects/:id/workshop/produce (verify validation & routing)
	wsPayload := map[string]any{
		"chapter_index": 1,
		"core_conflict": "宗门考核受阻",
	}
	body, _ = json.Marshal(wsPayload)
	req = httptest.NewRequest(http.MethodPost, "/api/projects/"+created.ID+"/workshop/produce", bytes.NewReader(body))
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 when mock LLM client cannot reach endpoint, got %d", w.Code)
	}

	// 7. GET /api/projects/:id/framework (initially empty)
	req = httptest.NewRequest(http.MethodGet, "/api/projects/"+created.ID+"/framework", nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for framework get, got %d", w.Code)
	}

	// 8. PUT /api/projects/:id/framework
	fwPayload := domain.ProjectFramework{
		ThemePremise: "凡人逆修弑神",
		WorldAxioms:  []string{"天道实为寄生真魔"},
	}
	body, _ = json.Marshal(fwPayload)
	req = httptest.NewRequest(http.MethodPut, "/api/projects/"+created.ID+"/framework", bytes.NewReader(body))
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for framework put, got %d", w.Code)
	}

	// 9. POST /api/projects/:id/codex (Create Codex Entry)
	codexPayload := domain.CodexEntry{
		Name:            "楚枫",
		Category:        domain.CategoryCharacter,
		Aliases:         []string{"白衣修罗", "疯子楚"},
		Summary:         "杀猪少年，心性冷酷果决",
		DetailsMarkdown: "暗藏弑神刀法与荒血后裔。",
	}
	body, _ = json.Marshal(codexPayload)
	req = httptest.NewRequest(http.MethodPost, "/api/projects/"+created.ID+"/codex", bytes.NewReader(body))
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 for codex create, got %d: %s", w.Code, w.Body.String())
	}
	var createdCodex domain.CodexEntry
	_ = json.Unmarshal(w.Body.Bytes(), &createdCodex)

	// 10. GET /api/projects/:id/codex
	req = httptest.NewRequest(http.MethodGet, "/api/projects/"+created.ID+"/codex", nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for codex list, got %d", w.Code)
	}
	var codexList []domain.CodexEntry
	_ = json.Unmarshal(w.Body.Bytes(), &codexList)
	if len(codexList) != 1 || codexList[0].Name != "楚枫" {
		t.Fatalf("expected 1 codex entry named 楚枫, got %+v", codexList)
	}

	// 11. POST /api/projects/:id/codex/scan
	scanPayload := map[string]string{"text": "月夜之下，白衣修罗悄然拔出斩神刀。"}
	body, _ = json.Marshal(scanPayload)
	req = httptest.NewRequest(http.MethodPost, "/api/projects/"+created.ID+"/codex/scan", bytes.NewReader(body))
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for codex scan, got %d", w.Code)
	}
	var scanResult struct {
		MatchedEntries []domain.CodexEntry `json:"matched_entries"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &scanResult)
	if len(scanResult.MatchedEntries) != 1 || scanResult.MatchedEntries[0].Name != "楚枫" {
		t.Fatalf("expected scan to match 楚枫 via alias 白衣修罗, got %+v", scanResult)
	}
}

func TestServer_MatrixScenesAndAnalytics(t *testing.T) {
	srv, s, _ := setupTestServer(t)
	ctx := t.Context()

	// 1. Create Project
	proj := &domain.Project{
		Title:          "大明修仙传",
		TargetPlatform: "起点仙侠",
		WorldRules:     "皇权与道门制衡，灵气日渐稀薄",
	}
	projBody, _ := json.Marshal(proj)
	req := httptest.NewRequest(http.MethodPost, "/api/projects", bytes.NewReader(projBody))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create project failed: %d", w.Code)
	}
	var created domain.Project
	_ = json.Unmarshal(w.Body.Bytes(), &created)

	// 2. Add a chapter
	chap := &domain.Chapter{
		ProjectID:    created.ID,
		ChapterIndex: 1,
		Title:        "紫禁风雪",
		Content:      "风雪之中，白衣修罗手握残刀，冷眼望向城楼上的锦衣卫统领赵天霸。",
		WordCount:    3000,
	}
	if _, err := s.CommitChapter(ctx, created.ID, chap); err != nil {
		t.Fatalf("CommitChapter failed: %v", err)
	}

	// 3. Add a Codex Entry
	codex := &domain.CodexEntry{
		ProjectID:       created.ID,
		Category:        domain.CategoryCharacter,
		Name:            "楚枫",
		Aliases:         []string{"白衣修罗"},
		Summary:         "大明前朝遗孤",
		DetailsMarkdown: "身负血海深仇",
	}
	_ = s.SaveCodexEntry(ctx, codex)

	// 4. Test Matrix Overview
	req = httptest.NewRequest(http.MethodGet, "/api/projects/"+created.ID+"/matrix", nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("matrix overview failed: %d", w.Code)
	}
	var matrix domain.MatrixOverview
	_ = json.Unmarshal(w.Body.Bytes(), &matrix)
	if len(matrix.Volumes) == 0 {
		t.Errorf("expected at least 1 default volume group in matrix, got %d", len(matrix.Volumes))
	}

	// 5. Test Scene Creation
	scPayload := domain.Scene{
		ChapterID:       chap.ID,
		SceneIndex:      1,
		Title:           "城门对峙",
		DramaticGoal:    "突破城门封锁",
		ConflictBarrier: "锦衣卫百户布阵阻拦",
		TensionLevel:    8,
		WordCount:       1200,
	}
	scBody, _ := json.Marshal(scPayload)
	req = httptest.NewRequest(http.MethodPost, "/api/projects/"+created.ID+"/scenes", bytes.NewReader(scBody))
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create scene failed: %d, body: %s", w.Code, w.Body.String())
	}
	var createdScene domain.Scene
	_ = json.Unmarshal(w.Body.Bytes(), &createdScene)
	if createdScene.ID == "" || createdScene.Title != "城门对峙" {
		t.Fatalf("unexpected created scene: %+v", createdScene)
	}

	// 6. Test Scene Retrieval & Update via /api/scenes/:id
	req = httptest.NewRequest(http.MethodGet, "/api/scenes/"+createdScene.ID, nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("get scene failed: %d", w.Code)
	}

	// 7. Test Scene Markers
	markerPayload := domain.SceneMarker{
		MarkerType:     domain.MarkerTypePlotHole,
		Color:          "#ef4444",
		TextRangeStart: 10,
		TextRangeEnd:   20,
		Content:        "此处锦衣卫佩刀型号与朝代设定不符",
	}
	mBody, _ := json.Marshal(markerPayload)
	req = httptest.NewRequest(http.MethodPost, "/api/scenes/"+createdScene.ID+"/markers", bytes.NewReader(mBody))
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create marker failed: %d", w.Code)
	}
	var createdMarker domain.SceneMarker
	_ = json.Unmarshal(w.Body.Bytes(), &createdMarker)

	req = httptest.NewRequest(http.MethodGet, "/api/scenes/"+createdScene.ID+"/markers", nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("list markers failed: %d", w.Code)
	}
	var markers []domain.SceneMarker
	_ = json.Unmarshal(w.Body.Bytes(), &markers)
	if len(markers) != 1 {
		t.Errorf("expected 1 marker, got %d", len(markers))
	}

	// 8. Test Prompt Preview
	previewReq := map[string]any{
		"chapter_index": 1,
		"scene_title":   "夜袭皇城",
		"pov":           "楚枫",
		"dramatic_goal": "潜入藏书阁寻找长生诀",
		"conflict":      "值守太监实力深不可测",
		"core_events":   []string{"避开巡逻", "斩杀哨探", "破除禁制"},
	}
	prevBody, _ := json.Marshal(previewReq)
	req = httptest.NewRequest(http.MethodPost, "/api/projects/"+created.ID+"/prompt-preview", bytes.NewReader(prevBody))
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("prompt preview failed: %d, body: %s", w.Code, w.Body.String())
	}
	var prevResp struct {
		SystemPrompt        string `json:"system_prompt"`
		UserPrompt          string `json:"user_prompt"`
		Components          []any  `json:"components"`
		TotalTokensEstimate int    `json:"total_tokens_estimate"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &prevResp)
	if len(prevResp.Components) != 4 || prevResp.TotalTokensEstimate <= 0 {
		t.Errorf("invalid prompt preview response: %+v", prevResp)
	}

	// 9. Test Analytics Heatmap
	req = httptest.NewRequest(http.MethodGet, "/api/projects/"+created.ID+"/analytics/heatmap", nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("analytics heatmap failed: %d", w.Code)
	}
	var heatmapResp struct {
		Chapters []any `json:"chapters"`
		Entities []struct {
			Name          string      `json:"name"`
			TotalMentions int         `json:"total_mentions"`
			ChapterCounts map[int]int `json:"chapter_counts"`
		} `json:"entities"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &heatmapResp)
	if len(heatmapResp.Entities) != 1 || heatmapResp.Entities[0].TotalMentions != 1 {
		t.Errorf("expected entity 楚枫 mentioned 1 time in chapter 1 via alias, got: %+v", heatmapResp)
	}

	// 10. Test Analytics Tension
	req = httptest.NewRequest(http.MethodGet, "/api/projects/"+created.ID+"/analytics/tension", nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("analytics tension failed: %d", w.Code)
	}
	var tensionResp struct {
		Points     []any   `json:"points"`
		AvgTension float64 `json:"avg_tension"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &tensionResp)
	if len(tensionResp.Points) != 1 || tensionResp.AvgTension <= 0 {
		t.Errorf("expected tension points, got: %+v", tensionResp)
	}
}

func TestServer_ConfigTest(t *testing.T) {
	srv, _, cfg := setupTestServer(t)

	// Mock OpenAI compatible server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer invalid-key" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error": {"message": "invalid api key"}}`))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]string{
						"role":    "assistant",
						"content": "NovelStudio Connection OK",
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	// 1. Validation test: empty base / key / model
	{
		reqBody := `{"target": "custom", "api_base": "", "api_key": "", "model": ""}`
		req := httptest.NewRequest(http.MethodPost, "/api/config/test", strings.NewReader(reqBody))
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		var res server.TestConfigResponse
		_ = json.Unmarshal(w.Body.Bytes(), &res)
		if res.Status != "error" || res.Error == "" {
			t.Errorf("expected validation error, got: %+v", res)
		}
	}

	// 2. Success test with mock server
	{
		testPayload := map[string]string{
			"target":   "default",
			"api_base": mockServer.URL,
			"api_key":  "valid-secret-key",
			"model":    "test-model",
		}
		b, _ := json.Marshal(testPayload)
		req := httptest.NewRequest(http.MethodPost, "/api/config/test", bytes.NewReader(b))
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		var res server.TestConfigResponse
		_ = json.Unmarshal(w.Body.Bytes(), &res)
		if res.Status != "ok" || !strings.Contains(res.Reply, "NovelStudio Connection OK") {
			t.Errorf("expected success test response, got: %+v", res)
		}
	}

	// 3. Unauthorized failure test
	{
		testPayload := map[string]string{
			"target":   "default",
			"api_base": mockServer.URL,
			"api_key":  "invalid-key",
			"model":    "test-model",
		}
		b, _ := json.Marshal(testPayload)
		req := httptest.NewRequest(http.MethodPost, "/api/config/test", bytes.NewReader(b))
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		var res server.TestConfigResponse
		_ = json.Unmarshal(w.Body.Bytes(), &res)
		if res.Status != "error" || !strings.Contains(res.Error, "401") {
			t.Errorf("expected 401 error, got: %+v", res)
		}
	}

	// 4. Role-based fallback test with masked key
	{
		cfg.APIBase = mockServer.URL
		cfg.APIKey = "valid-secret-key"
		cfg.ReasoningModel = "test-reasoner"

		testPayload := map[string]string{
			"target":  "reasoner",
			"api_key": "val...-key", // masked key passed from UI
			"model":   "",
		}
		b, _ := json.Marshal(testPayload)
		req := httptest.NewRequest(http.MethodPost, "/api/config/test", bytes.NewReader(b))
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		var res server.TestConfigResponse
		_ = json.Unmarshal(w.Body.Bytes(), &res)
		if res.Status != "ok" {
			t.Errorf("expected fallback to unmasked real key to succeed, got: %+v", res)
		}
	}
}

func TestServer_CheckpointLifecycle(t *testing.T) {
	srv, s, _ := setupTestServer(t)

	// Create test project
	proj := &domain.Project{
		ID:    "proj-cp-test",
		Title: "断点测试作品",
	}
	if err := s.SaveProject(context.Background(), proj); err != nil {
		t.Fatalf("save project failed: %v", err)
	}

	// 1. GET checkpoint when none exists -> returns 200 with null
	req := httptest.NewRequest(http.MethodGet, "/api/projects/proj-cp-test/checkpoint?chapter_index=1", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if strings.TrimSpace(w.Body.String()) != "null" {
		t.Errorf("expected null, got %s", w.Body.String())
	}

	// 2. Save a checkpoint into store
	cp := &domain.ChapterCheckpoint{
		ProjectID:    "proj-cp-test",
		ChapterIndex: 1,
		Phase:        domain.CheckpointPhaseRewriting,
		CoreConflict: "主角直面神庭使者",
		DraftText:    "寒风卷着冰碴子呼啸而过...",
		AuditReport: &domain.AuditReport{
			Verdict: domain.ReviewVerdictRevision,
			Score:   72,
			Issues:  []string{"实体违规"},
		},
	}
	if err := s.SaveCheckpoint(context.Background(), cp); err != nil {
		t.Fatalf("save checkpoint failed: %v", err)
	}

	// 3. GET checkpoint -> should return the saved checkpoint
	req = httptest.NewRequest(http.MethodGet, "/api/projects/proj-cp-test/checkpoint?chapter_index=1", nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var gotCp domain.ChapterCheckpoint
	if err := json.Unmarshal(w.Body.Bytes(), &gotCp); err != nil {
		t.Fatalf("unmarshal checkpoint failed: %v", err)
	}
	if gotCp.DraftText != cp.DraftText || gotCp.AuditReport == nil || gotCp.AuditReport.Score != 72 {
		t.Errorf("unexpected checkpoint content: %+v", gotCp)
	}

	// 4. DELETE checkpoint -> should clear it
	req = httptest.NewRequest(http.MethodDelete, "/api/projects/proj-cp-test/checkpoint?chapter_index=1", nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 on delete, got %d", w.Code)
	}

	// 5. Verify it is now null
	req = httptest.NewRequest(http.MethodGet, "/api/projects/proj-cp-test/checkpoint?chapter_index=1", nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if strings.TrimSpace(w.Body.String()) != "null" {
		t.Errorf("expected null after delete, got %s", w.Body.String())
	}
}

func TestServer_ChapterUncommit(t *testing.T) {
	srv, s, _ := setupTestServer(t)
	ctx := context.Background()

	// 1. Create project & commit chapter 1
	p := &domain.Project{
		ID:             "proj-uncommit-srv",
		Title:          "撤回草稿测试作品",
		TargetPlatform: "通用网文",
		Protagonist: domain.Protagonist{
			NameAndLevel: "陆青玄 (练气一层)",
			Inventory:    "新手木剑",
		},
	}
	_ = s.SaveProject(ctx, p)

	chap := &domain.Chapter{
		ID:           "ch_uncommit_srv_1",
		ProjectID:    p.ID,
		ChapterIndex: 1,
		Title:        "第 1 章 青萍微末",
		Content:      "细雨湿流光，芳草年年与恨长...",
		Review: &domain.ReviewResult{
			Verdict: domain.ReviewVerdictAccepted,
			Score:   90,
		},
	}
	if _, err := s.CommitChapter(ctx, p.ID, chap); err != nil {
		t.Fatalf("CommitChapter failed: %v", err)
	}

	// 2. POST /api/projects/:id/chapters/1/uncommit
	req := httptest.NewRequest(http.MethodPost, "/api/projects/"+p.ID+"/chapters/1/uncommit", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Status     string                    `json:"status"`
		Message    string                    `json:"message"`
		Checkpoint *domain.ChapterCheckpoint `json:"checkpoint"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if resp.Status != "uncommitted" || resp.Checkpoint == nil || resp.Checkpoint.DraftText != chap.Content {
		t.Errorf("unexpected uncommit response: %+v", resp)
	}

	// 3. Verify chapter is gone from chapters list
	req = httptest.NewRequest(http.MethodGet, "/api/projects/"+p.ID+"/chapters", nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	var chs []*domain.Chapter
	_ = json.Unmarshal(w.Body.Bytes(), &chs)
	if len(chs) != 0 {
		t.Errorf("expected 0 chapters, got %d", len(chs))
	}

	// 4. Verify checkpoint is available via GET checkpoint
	req = httptest.NewRequest(http.MethodGet, "/api/projects/"+p.ID+"/checkpoint?chapter_index=1", nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 on get checkpoint, got %d", w.Code)
	}
}

func TestServer_CheckpointPostAndPersistence(t *testing.T) {
	srv, s, _ := setupTestServer(t)
	ctx := context.Background()

	p := &domain.Project{
		ID:    "proj-cp-post",
		Title: "草稿断点保存测试",
	}
	_ = s.SaveProject(ctx, p)

	// 1. Test POST /api/projects/:id/checkpoint
	cpPayload := map[string]any{
		"chapter_index": 1,
		"phase":         "DRAFTED",
		"core_conflict": "雷雨夜密室寻宝",
		"draft_text":    "一道惊雷撕裂苍穹，照亮了供桌上的斑驳古盒...",
		"beats": []map[string]any{
			{"phase": "蓄力压迫", "tension": 5, "action": "潜入密室"},
		},
	}
	bodyBytes, _ := json.Marshal(cpPayload)
	req := httptest.NewRequest(http.MethodPost, "/api/projects/"+p.ID+"/checkpoint", bytes.NewReader(bodyBytes))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 on save checkpoint, got %d: %s", w.Code, w.Body.String())
	}

	// 2. Test GET /api/projects/:id/checkpoint returns what was saved
	req = httptest.NewRequest(http.MethodGet, "/api/projects/"+p.ID+"/checkpoint?chapter_index=1", nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 on get checkpoint, got %d", w.Code)
	}

	var fetchedCp domain.ChapterCheckpoint
	if err := json.Unmarshal(w.Body.Bytes(), &fetchedCp); err != nil {
		t.Fatalf("unmarshal checkpoint failed: %v", err)
	}
	if fetchedCp.DraftText != "一道惊雷撕裂苍穹，照亮了供桌上的斑驳古盒..." {
		t.Errorf("unexpected draft text: %s", fetchedCp.DraftText)
	}
	if len(fetchedCp.Beats) != 1 || fetchedCp.Beats[0].Action != "潜入密室" {
		t.Errorf("unexpected beats: %+v", fetchedCp.Beats)
	}
}

func TestServer_SuggestConflict(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	cfgPath := filepath.Join(tmpDir, "cfg.json")
	s, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]string{
						"role":    "assistant",
						"content": "核心冲突：青云宗执法堂突然深夜搜山，主角必须在身份暴露前将破损古镜送出禁地。",
					},
				},
			},
			"usage": map[string]int{
				"prompt_tokens":     80,
				"completion_tokens": 40,
				"total_tokens":      120,
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	cfg := config.DefaultConfig()
	cfg.APIBase = mockServer.URL
	cfg.APIKey = "sk-test-key"
	llmClient := engine.NewHTTPLLMClient(mockServer.URL, "sk-test-key")
	orch := engine.NewOrchestrator(llmClient)
	linter := engine.NewLinter(nil)

	srv, err := server.New(cfg, cfgPath, s, llmClient, orch, linter)
	if err != nil {
		t.Fatalf("server.New failed: %v", err)
	}

	ctx := context.Background()
	proj := &domain.Project{
		ID:             "proj_conflict_test",
		Title:          "逆天神尊",
		TargetPlatform: "番茄脑洞",
		WorldRules:     "天道无情，适者生存",
	}
	_ = s.SaveProject(ctx, proj)

	// Call POST /api/projects/:id/suggest-conflict
	reqBody := `{"chapter_index": 1}`
	req := httptest.NewRequest(http.MethodPost, "/api/projects/"+proj.ID+"/suggest-conflict", strings.NewReader(reqBody))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var res map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("unmarshal suggest-conflict response failed: %v", err)
	}

	coreConflict, ok := res["core_conflict"].(string)
	if !ok || coreConflict == "" {
		t.Fatalf("expected non-empty core_conflict, got %+v", res)
	}
	if strings.Contains(coreConflict, "核心冲突：") {
		t.Errorf("expected cleaned prefix, got %s", coreConflict)
	}
	expected := "青云宗执法堂突然深夜搜山，主角必须在身份暴露前将破损古镜送出禁地。"
	if coreConflict != expected {
		t.Errorf("expected %q, got %q", expected, coreConflict)
	}
}

func TestServer_Bootstrap_ProtagonistCodexAndRelations(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	cfgPath := filepath.Join(tmpDir, "cfg.json")
	s, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}

	callCount := 0
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		callCount++
		var content string
		switch callCount {
		case 1:
			// BootstrapFramework response
			content = `{
				"theme_premise": "凡人弑神",
				"world_axioms": ["神明寄生天道"],
				"power_ladder": [{"tier": 1, "realm": "凡胎境", "bottleneck": "无灵根", "drawback": "寿元损耗"}],
				"factions": [{"name": "青云宗", "alignment": "伪善中立", "doctrine": "血祭凡人", "threat_level": "极高"}],
				"key_characters": [{"name": "陆无涯", "role": "宗主", "realm": "化神境", "goal": "飞升成神", "fate_arc": "被主角手刃"}],
				"volume_arcs": [{"volume_index": 1, "title": "青云血祭", "core_goal": "逃离血祭", "climax": "斩杀执事", "estimated_chapters": 30}],
				"seed_hooks": [{"title": "残破铜镜", "details": "古物线索", "created_chapter": 1, "target_chapter": 10}]
			}`
		case 2:
			// ExtractCodexRelations response
			content = `[
				{
					"source_name": "主角",
					"target_name": "青云宗",
					"relation_type": "OPPOSES",
					"description": "隐忍对抗血祭秩序"
				}
			]`
		default:
			// SuggestChapterConflict response
			content = "核心冲突：主角在血祭前夕撞破执事阴谋，必须在一炷香内藏匿残破铜镜。"
		}

		resp := map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]string{
						"role":    "assistant",
						"content": content,
					},
				},
			},
			"usage": map[string]int{"prompt_tokens": 50, "completion_tokens": 50, "total_tokens": 100},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	cfg := config.DefaultConfig()
	cfg.APIBase = mockServer.URL
	cfg.APIKey = "sk-test"
	llmClient := engine.NewHTTPLLMClient(mockServer.URL, "sk-test")
	orch := engine.NewOrchestrator(llmClient)
	linter := engine.NewLinter(nil)

	srv, err := server.New(cfg, cfgPath, s, llmClient, orch, linter)
	if err != nil {
		t.Fatalf("server.New failed: %v", err)
	}

	// Call POST /api/projects/bootstrap
	body := `{"title": "万古凡仙", "target_platform": "起点仙侠", "concept": "凡人弑神"}`
	req := httptest.NewRequest(http.MethodPost, "/api/projects/bootstrap", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", w.Code, w.Body.String())
	}

	var createdProj domain.Project
	if err := json.Unmarshal(w.Body.Bytes(), &createdProj); err != nil {
		t.Fatalf("unmarshal created project failed: %v", err)
	}

	ctx := context.Background()
	// 1. Verify that Protagonist was created as a CodexEntry!
	entries, err := s.ListCodexEntries(ctx, createdProj.ID, "")
	if err != nil {
		t.Fatalf("ListCodexEntries failed: %v", err)
	}
	var hasProtagonist bool
	for _, e := range entries {
		if e.Category == domain.CategoryCharacter && (e.Name == "主角" || strings.Contains(e.Summary, "全书主角")) {
			hasProtagonist = true
			break
		}
	}
	if !hasProtagonist {
		t.Errorf("expected Protagonist to be auto-registered in CodexEntry, got entries: %+v", entries)
	}

	// 2. Verify that relation between Protagonist and 青云宗 was established!
	relations, err := s.ListCodexRelations(ctx, createdProj.ID, "")
	if err != nil {
		t.Fatalf("ListCodexRelations failed: %v", err)
	}
	if len(relations) == 0 {
		t.Errorf("expected at least 1 codex relation involving Protagonist, got 0")
	}

	// 3. Verify chapter 1 initial checkpoint was seeded with conflict
	cp, err := s.GetCheckpoint(ctx, createdProj.ID, 1)
	if err != nil {
		t.Fatalf("GetCheckpoint failed: %v", err)
	}
	if cp == nil || cp.CoreConflict == "" {
		t.Errorf("expected seeded checkpoint for chapter 1, got %+v", cp)
	}
}
