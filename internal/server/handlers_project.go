package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Jungley8/novel-studio/internal/activity"
	"github.com/Jungley8/novel-studio/internal/coord"
	"github.com/Jungley8/novel-studio/internal/domain"
	"github.com/Jungley8/novel-studio/internal/engine"
	"github.com/Jungley8/novel-studio/internal/store"
)

func (s *Server) handleProjects(w http.ResponseWriter, r *http.Request) {
	r = r.WithContext(activity.WithSource(r.Context(), activity.SourceUIManual))
	ctx := r.Context()
	switch r.Method {
	case http.MethodGet:
		list, err := s.store.ListProjects(ctx)
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		if list == nil {
			list = []*domain.Project{}
		}
		jsonResponse(w, http.StatusOK, list)
	case http.MethodPost:
		var req struct {
			domain.Project
			ProtagonistName string `json:"protagonist_name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			errorResponse(w, http.StatusBadRequest, err.Error())
			return
		}
		p := req.Project
		if p.ID == "" {
			p.ID = fmt.Sprintf("proj_%d", time.Now().UnixNano())
		}
		if err := s.store.SaveProject(ctx, &p); err != nil {
			errorResponse(w, http.StatusBadRequest, err.Error())
			return
		}
		if existing, _ := s.store.GetProtagonistEntry(ctx, p.ID); existing == nil {
			protName := strings.TrimSpace(req.ProtagonistName)
			if protName == "" {
				protName = "主角"
			}
			initialRealm := "凡人"
			protEntry := &domain.CodexEntry{
				ID:              fmt.Sprintf("codex_%s_pro", p.ID),
				ProjectID:       p.ID,
				Category:        domain.CategoryCharacter,
				Archetype:       "PROTAGONIST",
				Name:            protName,
				Realm:           initialRealm,
				Aliases:         []string{protName},
				Summary:         fmt.Sprintf("全书核心主角 | 境界: %s", initialRealm),
				DetailsMarkdown: fmt.Sprintf("【全书核心主角档案】\n- 境界：%s\n- 随身底牌与物品：无\n- 当前状态：良好", initialRealm),
				Inventory:       "无",
				HealthStatus:    "良好",
				TrackingMode:    domain.TrackingModeAlwaysInject,
				CreatedAt:       time.Now(),
				UpdatedAt:       time.Now(),
			}
			_ = s.store.SaveCodexEntry(ctx, protEntry)
		}
		jsonResponse(w, http.StatusCreated, p)
	default:
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleProjectRoutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/projects/")
	parts := strings.Split(path, "/")
	projectID := parts[0]
	if projectID == "" {
		errorResponse(w, http.StatusBadRequest, "missing project id")
		return
	}

	r = r.WithContext(activitySourceForProjectRoute(r.Context(), parts))
	ctx := r.Context()

	// 1. /api/projects/:id (GET / DELETE / POST)
	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			p, err := s.store.GetProject(ctx, projectID)
			if err != nil {
				errorResponse(w, http.StatusNotFound, "project not found")
				return
			}
			jsonResponse(w, http.StatusOK, p)
		case http.MethodDelete:
			if s.rejectIfProducing(w, r, projectID, "删除作品") {
				return
			}
			if err := s.store.DeleteProject(ctx, projectID); err != nil {
				errorResponse(w, http.StatusInternalServerError, err.Error())
				return
			}
			jsonResponse(w, http.StatusOK, map[string]string{"status": "deleted"})
		case http.MethodPost:
			var p domain.Project
			if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
				errorResponse(w, http.StatusBadRequest, err.Error())
				return
			}
			p.ID = projectID
			if err := s.store.SaveProject(ctx, &p); err != nil {
				errorResponse(w, http.StatusBadRequest, err.Error())
				return
			}
			jsonResponse(w, http.StatusOK, p)
		default:
			errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	action := parts[1]
	switch action {
	case "bench":
		s.handleProjectBench(w, r, projectID)
	case "structure-audit":
		subParts := []string{}
		if len(parts) > 2 {
			subParts = parts[2:]
		}
		s.handleProjectStructureAudit(w, r, projectID, subParts)
	case "forecast":
		subParts := []string{}
		if len(parts) > 2 {
			subParts = parts[2:]
		}
		s.handleProjectForecast(w, r, projectID, subParts)
	case "intents":
		subParts := []string{}
		if len(parts) > 2 {
			subParts = parts[2:]
		}
		s.handleProjectIntents(w, r, projectID, subParts)
	case "suggest-conflict":
		s.handleSuggestConflict(w, r, projectID)
	case "chapters":
		if len(parts) >= 3 {
			if parts[2] == "suggest-conflict" {
				s.handleSuggestConflict(w, r, projectID)
				return
			}
			s.handleProjectChapterSub(w, r, projectID, parts[2:])
			return
		}
		s.handleProjectChapters(w, r, projectID)
	case "hooks":
		subParts := []string{}
		if len(parts) > 2 {
			subParts = parts[2:]
		}
		s.handleProjectHooks(w, r, projectID, subParts)
	case "statemachine":
		subParts := []string{}
		if len(parts) > 2 {
			subParts = parts[2:]
		}
		s.handleProjectStateMachine(w, r, projectID, subParts)
	case "derive-beats":
		s.handleDeriveBeats(w, r, projectID)
	case "render-scene":
		s.handleRenderScene(w, r, projectID)
	case "review-draft":
		s.handleReviewDraft(w, r, projectID)
	case "rewrite-draft":
		s.handleRewriteDraft(w, r, projectID)
	case "checkpoint":
		s.handleProjectCheckpoint(w, r, projectID)
	case "edit-lease":
		s.handleProjectEditLease(w, r, projectID)
	case "framework":
		s.handleProjectFramework(w, r, projectID, parts)
	case "codex":
		subParts := []string{}
		if len(parts) > 2 {
			subParts = parts[2:]
		}
		s.handleProjectCodex(w, r, projectID, subParts)
	case "matrix":
		subParts := []string{}
		if len(parts) > 2 {
			subParts = parts[2:]
		}
		s.handleProjectMatrix(w, r, projectID, subParts)
	case "scenes":
		subParts := []string{}
		if len(parts) > 2 {
			subParts = parts[2:]
		}
		s.handleProjectScenes(w, r, projectID, subParts)
	case "prompt-preview":
		s.handlePromptPreview(w, r, projectID)
	case "chat":
		s.handleWorkshopChat(w, r, projectID)
	case "analytics":
		subParts := []string{}
		if len(parts) > 2 {
			subParts = parts[2:]
		}
		s.handleAnalytics(w, r, projectID, subParts)
	case "metrics":
		s.handleProjectMetrics(w, r, projectID)
	case "workshop":
		if len(parts) >= 3 && parts[2] == "produce" {
			s.handleWorkshopProduce(w, r, projectID)
			return
		}
		errorResponse(w, http.StatusBadRequest, "invalid workshop action")
	case "activity":
		subParts := []string{}
		if len(parts) > 2 {
			subParts = parts[2:]
		}
		s.handleProjectActivity(w, r, projectID, subParts)
	case "export":
		if len(parts) >= 3 {
			s.handleProjectExport(w, r, projectID, parts[2])
			return
		}
		errorResponse(w, http.StatusBadRequest, "invalid export format")
	case "harmonize":
		s.handleProjectHarmonize(w, r, projectID)
	case "suggest-human-touches":
		s.handleSuggestHumanTouches(w, r, projectID)
	case "sanitize-ai":
		s.handleSanitizeAI(w, r, projectID)
	default:
		errorResponse(w, http.StatusNotFound, "not found")
	}
}

func (s *Server) handleProjectChapters(w http.ResponseWriter, r *http.Request, projectID string) {
	ctx := r.Context()
	switch r.Method {
	case http.MethodGet:
		chapters, err := s.store.ListChapters(ctx, projectID)
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		if chapters == nil {
			chapters = []*domain.Chapter{}
		}
		jsonResponse(w, http.StatusOK, chapters)
	case http.MethodPost:
		var c domain.Chapter
		if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
			errorResponse(w, http.StatusBadRequest, err.Error())
			return
		}
		// Fail-Closed：后台生产锁生效期间拒绝人工定稿（避免并发覆盖正史与状态双写）。
		if s.rejectIfProducing(w, r, projectID, "定稿") {
			return
		}
		c.ProjectID = projectID
		if c.ID == "" {
			c.ID = fmt.Sprintf("ch_%s_%d", projectID, c.ChapterIndex)
		}

		// 手动入库路径（有意设计）：以人工判断为准——状态变更仅经 DryRunStateMutation 账本证据检查，
		// 不跑自动状态验证器；伏笔回收声明已在 /review-draft 经证据门过滤。
		hasMutation := strings.TrimSpace(c.StateMutation.InventoryDelta) != "" ||
			strings.TrimSpace(c.StateMutation.PowerDelta) != "" ||
			len(c.StateMutation.CharacterMutations) > 0
		if hasMutation {
			prot, err := s.store.GetProtagonistEntry(ctx, projectID)
			if err != nil {
				errorResponse(w, http.StatusInternalServerError, err.Error())
				return
			}
			if prot != nil {
				trail, err := domain.DryRunStateMutation(prot, c.StateMutation)
				if err != nil {
					errorResponse(w, http.StatusInternalServerError, err.Error())
					return
				}
				var warnings []string
				for _, line := range trail {
					if strings.Contains(line, "[Ledger Warning]") {
						warnings = append(warnings, line)
						if len(warnings) == 3 {
							break
						}
					}
				}
				if len(warnings) > 0 {
					errorResponse(w, http.StatusBadRequest, strings.Join(warnings, "; ")+": 证据不足，拒绝无证据状态变更")
					return
				}
			}
		}

		updatedProj, err := s.store.CommitChapter(ctx, projectID, &c)
		if err != nil {
			errorResponse(w, http.StatusBadRequest, err.Error())
			return
		}
		_ = s.store.ClearCheckpoint(ctx, projectID, c.ChapterIndex)
		protEntry, _ := s.store.GetProtagonistEntry(ctx, projectID)

		jsonResponse(w, http.StatusCreated, map[string]any{
			"chapter":     c,
			"project":     updatedProj,
			"protagonist": protEntry,
		})
	default:
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// chapterVersionSource 章节版本读取能力（只读；经类型断言获取，避免扩张 store.Store 大接口）。
type chapterVersionSource interface {
	ListChapterVersions(ctx context.Context, projectID string, chapterIndex int) ([]*domain.ChapterVersionMeta, error)
	GetChapterVersion(ctx context.Context, projectID, versionID string) (*domain.ChapterVersion, error)
}

func (s *Server) handleProjectChapterSub(w http.ResponseWriter, r *http.Request, projectID string, subParts []string) {
	chapterIndex, err := strconv.Atoi(subParts[0])
	if err != nil || chapterIndex <= 0 {
		errorResponse(w, http.StatusBadRequest, "invalid chapter index")
		return
	}

	ctx := r.Context()

	// 1. /api/projects/:id/chapters/:index/uncommit (POST)
	if len(subParts) >= 2 && (subParts[1] == "uncommit" || subParts[1] == "revert-draft") {
		if r.Method != http.MethodPost {
			errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		// Fail-Closed：后台生产锁生效期间拒绝撤回归档（状态时钟被生产方掌握）。
		if s.rejectIfProducing(w, r, projectID, "撤回归档") {
			return
		}
		cp, updatedProj, err := s.store.UncommitChapter(ctx, projectID, chapterIndex)
		if err != nil {
			if errors.Is(err, store.ErrNonLatestChapter) {
				errorResponse(w, http.StatusConflict, err.Error())
				return
			}
			errorResponse(w, http.StatusBadRequest, err.Error())
			return
		}
		protEntry, _ := s.store.GetProtagonistEntry(ctx, projectID)
		jsonResponse(w, http.StatusOK, map[string]any{
			"status":      "uncommitted",
			"message":     fmt.Sprintf("第 %d 章已成功从正史撤回为草稿", chapterIndex),
			"checkpoint":  cp,
			"project":     updatedProj,
			"protagonist": protEntry,
		})
		return
	}

	// versions: /api/projects/:id/chapters/:chapterIndex/versions[/:versionId] (GET)
	if len(subParts) >= 2 && subParts[1] == "versions" {
		if r.Method != http.MethodGet {
			errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		vs, ok := s.store.(chapterVersionSource)
		if !ok {
			errorResponse(w, http.StatusInternalServerError, "store does not support chapter versions")
			return
		}
		if len(subParts) >= 3 && strings.TrimSpace(subParts[2]) != "" {
			version, verr := vs.GetChapterVersion(ctx, projectID, subParts[2])
			if verr != nil {
				errorResponse(w, http.StatusInternalServerError, "get chapter version failed: "+verr.Error())
				return
			}
			if version == nil {
				errorResponse(w, http.StatusNotFound, "chapter version not found")
				return
			}
			jsonResponse(w, http.StatusOK, version)
			return
		}
		list, lerr := vs.ListChapterVersions(ctx, projectID, chapterIndex)
		if lerr != nil {
			errorResponse(w, http.StatusInternalServerError, "list chapter versions failed: "+lerr.Error())
			return
		}
		jsonResponse(w, http.StatusOK, list)
		return
	}

	// 2. /api/projects/:id/chapters/:index (GET / DELETE)
	switch r.Method {
	case http.MethodGet:
		chapter, err := s.store.GetChapter(ctx, projectID, chapterIndex)
		if err != nil {
			errorResponse(w, http.StatusNotFound, "chapter not found")
			return
		}
		jsonResponse(w, http.StatusOK, chapter)
	case http.MethodDelete:
		// Fail-Closed：后台生产锁生效期间拒绝删除/撤回章节。
		if s.rejectIfProducing(w, r, projectID, "删除或撤回章节") {
			return
		}
		toDraft := r.URL.Query().Get("to_draft") == "true"
		if toDraft {
			cp, updatedProj, err := s.store.UncommitChapter(ctx, projectID, chapterIndex)
			if err != nil {
				if errors.Is(err, store.ErrNonLatestChapter) {
					errorResponse(w, http.StatusConflict, err.Error())
					return
				}
				errorResponse(w, http.StatusBadRequest, err.Error())
				return
			}
			protEntry, _ := s.store.GetProtagonistEntry(ctx, projectID)
			jsonResponse(w, http.StatusOK, map[string]any{
				"status":      "uncommitted",
				"message":     fmt.Sprintf("第 %d 章已成功撤回为草稿", chapterIndex),
				"checkpoint":  cp,
				"project":     updatedProj,
				"protagonist": protEntry,
			})
			return
		}

		if err := s.store.DeleteChapter(ctx, projectID, chapterIndex); err != nil {
			errorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		jsonResponse(w, http.StatusOK, map[string]any{
			"status":  "deleted",
			"message": fmt.Sprintf("第 %d 章已成功删除", chapterIndex),
		})
	default:
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// rejectIfProducing 在后台生产锁生效期间拒绝正史写入口；返回 true 表示已写出 409。
func (s *Server) rejectIfProducing(w http.ResponseWriter, r *http.Request, projectID, action string) bool {
	lease, err := s.store.GetLiveLease(r.Context(), projectID, coord.KindProduction)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return true
	}
	if lease == nil {
		return false
	}
	chapterNote := ""
	if lease.Chapter > 0 {
		chapterNote = fmt.Sprintf("·第 %d 章", lease.Chapter)
	}
	errorResponse(w, http.StatusConflict, fmt.Sprintf("该作品正在后台生产（%s%s），请等待完成后再%s", lease.Label, chapterNote, action))
	return true
}

// handleProjectEditLease 处理前台编辑租约：
//   - POST：获取/续期（默认 TTL 120s，允许 30~600s）
//   - DELETE：释放
//
// 租约存于 SQLite（跨进程可见）；租约存在期间，守护服务调度器与 CLI
// 生产都不会对同一作品启动（Fail-Closed）。
func (s *Server) handleProjectEditLease(w http.ResponseWriter, r *http.Request, projectID string) {
	ctx := r.Context()
	switch r.Method {
	case http.MethodPost:
		var req struct {
			TTLSeconds int `json:"ttl_seconds"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req) // 允许空 Body（使用默认 TTL）
		ttl := req.TTLSeconds
		if ttl <= 0 {
			ttl = int(coord.EditLeaseTTL / time.Second)
		}
		if ttl < 30 {
			ttl = 30
		}
		if ttl > 600 {
			ttl = 600
		}
		expiry, err := s.store.UpsertEditLease(ctx, projectID, time.Duration(ttl)*time.Second)
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		jsonResponse(w, http.StatusOK, map[string]any{
			"status":     "held",
			"expires_at": expiry.UTC().Format(time.RFC3339),
		})
	case http.MethodDelete:
		if err := s.store.DeleteEditLease(ctx, projectID); err != nil {
			errorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		jsonResponse(w, http.StatusOK, map[string]string{"status": "released"})
	default:
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleProjectCheckpoint(w http.ResponseWriter, r *http.Request, projectID string) {
	ctx := r.Context()
	switch r.Method {
	case http.MethodGet:
		// ?latest=true 返回该作品最近更新的在途断点（切换作品时恢复“最近推进的一章”）。
		if r.URL.Query().Get("latest") == "true" {
			cp, err := s.store.GetLatestCheckpoint(ctx, projectID)
			if err != nil {
				errorResponse(w, http.StatusInternalServerError, err.Error())
				return
			}
			jsonResponse(w, http.StatusOK, cp)
			return
		}
		idxStr := r.URL.Query().Get("chapter_index")
		idx, _ := strconv.Atoi(idxStr)
		if idx <= 0 {
			idx = 1
		}
		cp, err := s.store.GetCheckpoint(ctx, projectID, idx)
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		if cp == nil {
			jsonResponse(w, http.StatusOK, nil)
			return
		}
		jsonResponse(w, http.StatusOK, cp)
	case http.MethodPost:
		var cp domain.ChapterCheckpoint
		if err := json.NewDecoder(r.Body).Decode(&cp); err != nil {
			errorResponse(w, http.StatusBadRequest, err.Error())
			return
		}
		cp.ProjectID = projectID
		if cp.ChapterIndex <= 0 {
			errorResponse(w, http.StatusBadRequest, "invalid chapter_index")
			return
		}
		if cp.UpdatedAt.IsZero() {
			cp.UpdatedAt = time.Now()
		}
		if err := s.store.SaveCheckpoint(ctx, &cp); err != nil {
			errorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		jsonResponse(w, http.StatusOK, cp)
	case http.MethodDelete:
		idxStr := r.URL.Query().Get("chapter_index")
		idx, _ := strconv.Atoi(idxStr)
		if idx <= 0 {
			idx = 1
		}
		if err := s.store.ClearCheckpoint(ctx, projectID, idx); err != nil {
			errorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		jsonResponse(w, http.StatusOK, map[string]string{"status": "cleared"})
	default:
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleProjectHooks(w http.ResponseWriter, r *http.Request, projectID string, subParts []string) {
	ctx := r.Context()
	if len(subParts) > 0 && subParts[0] == "ai-extract" {
		if r.Method != http.MethodPost {
			errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		proj, err := s.store.GetProject(ctx, projectID)
		if err != nil {
			errorResponse(w, http.StatusNotFound, "project not found")
			return
		}
		chapters, _ := s.store.ListChapters(ctx, projectID)
		existingHooks, _ := s.store.ListPlotHooks(ctx, projectID)
		newHooks, _, err := s.orch.ExtractPlotHooks(ctx, s.cfg.ReasoningModel, proj, chapters, existingHooks)
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, "ai extract plot hooks failed: "+err.Error())
			return
		}
		for i := range newHooks {
			_ = s.store.SavePlotHook(ctx, &newHooks[i])
		}
		jsonResponse(w, http.StatusOK, newHooks)
		return
	}

	switch r.Method {
	case http.MethodGet:
		hooks, err := s.store.ListPlotHooks(ctx, projectID)
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		if hooks == nil {
			hooks = []*domain.PlotHook{}
		}
		jsonResponse(w, http.StatusOK, hooks)
	case http.MethodPost:
		var h domain.PlotHook
		if err := json.NewDecoder(r.Body).Decode(&h); err != nil {
			errorResponse(w, http.StatusBadRequest, err.Error())
			return
		}
		// Fail-Closed：已存在的伏笔必须归属本作品，严禁用其它作品的伏笔 ID 覆盖其数据。
		if h.ID != "" {
			owner, err := s.store.PlotHookProjectID(ctx, h.ID)
			if err != nil {
				errorResponse(w, http.StatusInternalServerError, err.Error())
				return
			}
			if owner != "" && owner != projectID {
				errorResponse(w, http.StatusConflict, "plot hook belongs to another project")
				return
			}
		}
		h.ProjectID = projectID
		if h.ID == "" {
			h.ID = fmt.Sprintf("hook_%d", time.Now().UnixNano())
		}
		if err := s.store.SavePlotHook(ctx, &h); err != nil {
			errorResponse(w, http.StatusBadRequest, err.Error())
			return
		}
		jsonResponse(w, http.StatusCreated, h)
	default:
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleProjectStateMachine(w http.ResponseWriter, r *http.Request, projectID string, subParts []string) {
	ctx := r.Context()
	if len(subParts) > 0 && subParts[0] == "ai-analyze" {
		if r.Method != http.MethodPost {
			errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		proj, err := s.store.GetProject(ctx, projectID)
		if err != nil {
			errorResponse(w, http.StatusNotFound, "project not found")
			return
		}
		protEntry, err := s.store.GetProtagonistEntry(ctx, projectID)
		if err != nil || protEntry == nil {
			errorResponse(w, http.StatusNotFound, "protagonist codex entry not found")
			return
		}
		chapters, err := s.store.ListChapters(ctx, projectID)
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		updated, _, err := s.orch.AnalyzeProtagonistState(ctx, s.cfg.ReasoningModel, proj, protEntry, chapters)
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, "analyze protagonist state failed: "+err.Error())
			return
		}
		if err := s.store.SaveCodexEntry(ctx, updated); err != nil {
			errorResponse(w, http.StatusInternalServerError, "save protagonist codex entry failed: "+err.Error())
			return
		}
		jsonResponse(w, http.StatusOK, updated)
		return
	}

	switch r.Method {
	case http.MethodGet:
		protEntry, err := s.store.GetProtagonistEntry(ctx, projectID)
		if err != nil || protEntry == nil {
			errorResponse(w, http.StatusNotFound, "protagonist not found")
			return
		}
		jsonResponse(w, http.StatusOK, protEntry)
	case http.MethodPost:
		var update domain.CodexEntry
		if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
			errorResponse(w, http.StatusBadRequest, "invalid json: "+err.Error())
			return
		}
		protEntry, _ := s.store.GetProtagonistEntry(ctx, projectID)
		if protEntry == nil {
			protEntry = &domain.CodexEntry{
				ID:        fmt.Sprintf("codex_%s_pro", projectID),
				ProjectID: projectID,
				Category:  domain.CategoryCharacter,
				Archetype: "PROTAGONIST",
			}
		}
		if update.Name != "" {
			protEntry.Name = update.Name
		}
		if update.Realm != "" {
			protEntry.Realm = update.Realm
		}
		if update.Inventory != "" {
			protEntry.Inventory = update.Inventory
		}
		if update.HealthStatus != "" {
			protEntry.HealthStatus = update.HealthStatus
		}
		if update.Summary != "" {
			protEntry.Summary = update.Summary
		}
		if update.Want != "" {
			protEntry.Want = update.Want
		}
		if update.Need != "" {
			protEntry.Need = update.Need
		}
		if update.Ghost != "" {
			protEntry.Ghost = update.Ghost
		}
		if update.StructuredLevel != nil {
			protEntry.StructuredLevel = update.StructuredLevel
		}
		if update.StructuredItems != nil {
			protEntry.StructuredItems = update.StructuredItems
		}
		protEntry.Category = domain.CategoryCharacter
		protEntry.Archetype = "PROTAGONIST"
		protEntry.UpdatedAt = time.Now()
		if err := s.store.SaveCodexEntry(ctx, protEntry); err != nil {
			errorResponse(w, http.StatusInternalServerError, "save protagonist failed: "+err.Error())
			return
		}
		jsonResponse(w, http.StatusOK, protEntry)
	default:
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleHookDelete(w http.ResponseWriter, r *http.Request) {
	r = r.WithContext(activity.WithSource(r.Context(), activity.SourceUIManual))
	if r.Method != http.MethodDelete {
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	hookID := strings.TrimPrefix(r.URL.Path, "/api/hooks/")
	if err := s.store.DeletePlotHook(r.Context(), hookID); err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, map[string]string{"status": "deleted"})
}

type BootstrapProjectRequest struct {
	Title          string `json:"title"`
	TargetPlatform string `json:"target_platform"`
	Concept        string `json:"concept"`
	ReasoningModel string `json:"reasoning_model,omitempty"`
}

func (s *Server) handleBootstrapProject(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	bootRunID := activity.NewRunID()
	r = r.WithContext(activity.WithRunID(activity.WithSource(r.Context(), activity.SourceBootstrap), bootRunID))
	var req BootstrapProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}
	if strings.TrimSpace(req.Title) == "" {
		errorResponse(w, http.StatusBadRequest, "project title cannot be empty")
		return
	}
	reasoningModel := req.ReasoningModel
	if reasoningModel == "" {
		reasoningModel = s.cfg.ReasoningModel
	}

	fw, _, err := s.orch.BootstrapFramework(r.Context(), reasoningModel, engine.FrameworkBootstrapRequest{
		Title:          req.Title,
		TargetPlatform: req.TargetPlatform,
		CoreConcept:    req.Concept,
	})
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, "bootstrap framework failed: "+err.Error())
		return
	}

	projectID := fmt.Sprintf("proj_%d", time.Now().UnixNano())
	var worldRules strings.Builder
	worldRules.WriteString("【世界底层规则与核心公理】\n")
	for i, axiom := range fw.WorldAxioms {
		worldRules.WriteString(fmt.Sprintf("%d. %s\n", i+1, axiom))
	}
	if len(fw.PowerLadder) > 0 {
		worldRules.WriteString("\n【战力阶梯与能力代价】\n")
		for _, tier := range fw.PowerLadder {
			worldRules.WriteString(fmt.Sprintf("- %s: 关隘[%s] | 代价[%s]\n", tier.Realm, tier.Bottleneck, tier.Drawback))
		}
	}

	initialRealm := "初入门槛"
	if len(fw.PowerLadder) > 0 && fw.PowerLadder[0].Realm != "" {
		initialRealm = fw.PowerLadder[0].Realm
	}
	proj := &domain.Project{
		ID:             projectID,
		Title:          req.Title,
		TargetPlatform: req.TargetPlatform,
		WorldRules:     strings.TrimSpace(worldRules.String()),
		Framework:      fw,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if err := s.store.SaveProject(r.Context(), proj); err != nil {
		errorResponse(w, http.StatusInternalServerError, "save project failed: "+err.Error())
		return
	}

	// 注册核心主角至设定集 (Codex)
	protName := "主角"
	protWant := ""
	protNeed := ""
	protGhost := ""
	if fw.Protagonist != nil {
		if strings.TrimSpace(fw.Protagonist.Name) != "" {
			protName = strings.TrimSpace(fw.Protagonist.Name)
		}
		protWant = strings.TrimSpace(fw.Protagonist.Want)
		protNeed = strings.TrimSpace(fw.Protagonist.Need)
		protGhost = strings.TrimSpace(fw.Protagonist.Ghost)
	}
	summaryWant := protWant
	if summaryWant == "" {
		summaryWant = fw.ThemePremise
	}

	protDetailsSb := strings.Builder{}
	protDetailsSb.WriteString(fmt.Sprintf("【全书核心主角档案】\n- 初始境界：%s\n- 外在欲望：%s\n", initialRealm, summaryWant))
	if protNeed != "" {
		protDetailsSb.WriteString(fmt.Sprintf("- 真实需求：%s\n", protNeed))
	}
	if protGhost != "" {
		protDetailsSb.WriteString(fmt.Sprintf("- 核心创伤：%s\n", protGhost))
	}
	protDetailsSb.WriteString("- 随身底牌与物品：随身物品待起草\n- 当前状态：良好")

	protEntry := &domain.CodexEntry{
		ID:              fmt.Sprintf("codex_%s_pro", projectID),
		ProjectID:       projectID,
		Category:        domain.CategoryCharacter,
		Archetype:       "PROTAGONIST",
		Name:            protName,
		Realm:           initialRealm,
		Want:            protWant,
		Need:            protNeed,
		Ghost:           protGhost,
		Aliases:         []string{"主角", fmt.Sprintf("%s (%s)", protName, initialRealm)},
		Summary:         fmt.Sprintf("全书核心主角 | 初始境界: %s | 欲望: %s", initialRealm, summaryWant),
		DetailsMarkdown: protDetailsSb.String(),
		Inventory:       "随身物品待起草",
		HealthStatus:    "良好",
		TrackingMode:    domain.TrackingModeAlwaysInject,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	_ = s.store.SaveCodexEntry(r.Context(), protEntry)

	// 同步势力阵营至设定集
	for _, fac := range fw.Factions {
		if strings.TrimSpace(fac.Name) == "" {
			continue
		}
		entry := &domain.CodexEntry{
			ID:              fmt.Sprintf("codex_%s_fac_%d", projectID, time.Now().UnixNano()),
			ProjectID:       projectID,
			Category:        domain.CategoryFaction,
			Name:            fac.Name,
			Alignment:       fac.Alignment,
			ThreatLevel:     fac.ThreatLevel,
			Summary:         fmt.Sprintf("立场: %s | 威胁度: %s", fac.Alignment, fac.ThreatLevel),
			DetailsMarkdown: fac.Doctrine,
			TrackingMode:    domain.TrackingModeAutoMention,
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		}
		_ = s.store.SaveCodexEntry(r.Context(), entry)
	}

	// 同步关键人物至设定集
	for _, kc := range fw.KeyCharacters {
		if strings.TrimSpace(kc.Name) == "" {
			continue
		}
		entry := &domain.CodexEntry{
			ID:              fmt.Sprintf("codex_%s_char_%d", projectID, time.Now().UnixNano()),
			ProjectID:       projectID,
			Category:        domain.CategoryCharacter,
			Name:            kc.Name,
			Want:            kc.Want,
			Need:            kc.Need,
			Ghost:           kc.Ghost,
			Summary:         fmt.Sprintf("定位: %s | 境界: %s | 欲望: %s", kc.Role, kc.Realm, kc.Want),
			DetailsMarkdown: fmt.Sprintf("宿命终局: %s", kc.FateArc),
			TrackingMode:    domain.TrackingModeAutoMention,
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		}
		_ = s.store.SaveCodexEntry(r.Context(), entry)
	}

	for i, sh := range fw.SeedHooks {
		hook := &domain.PlotHook{
			ID:             fmt.Sprintf("hook_%s_%d", projectID, i+1),
			ProjectID:      projectID,
			Title:          sh.Title,
			Details:        sh.Details,
			CreatedChapter: sh.CreatedChapter,
			TargetChapter:  sh.TargetChapter,
			Status:         domain.HookStatusOpen,
			CreatedAt:      time.Now(),
		}
		_ = s.store.SavePlotHook(r.Context(), hook)
	}

	// 全自动级联关系图谱推演 (Extract Codex Relations Cascade)
	codexEntries, _ := s.store.ListCodexEntries(r.Context(), projectID, "")
	if len(codexEntries) >= 2 && s.orch != nil {
		relations, _, relErr := s.orch.ExtractCodexRelations(
			r.Context(),
			s.cfg.ReasoningModel,
			proj,
			codexEntries,
			proj.WorldRules+"\n\n全书核心立意："+fw.ThemePremise,
		)
		if relErr == nil && len(relations) > 0 {
			for _, rel := range relations {
				_ = s.store.SaveCodexRelation(r.Context(), projectID, &rel)
			}
		}
	}

	// 全自动推演第 1 卷第 1 章开局黄金冲突并初始化草稿检查点
	if s.chronicle != nil && s.orch != nil {
		horizon, hErr := s.chronicle.AssembleHorizon(r.Context(), projectID, 1)
		if hErr == nil && horizon != nil {
			conflict1, _, cErr := s.orch.SuggestChapterConflict(r.Context(), s.cfg.ReasoningModel, horizon)
			if cErr == nil && strings.TrimSpace(conflict1) != "" {
				_ = s.store.SaveCheckpoint(r.Context(), &domain.ChapterCheckpoint{
					ProjectID:    projectID,
					ChapterIndex: 1,
					Phase:        domain.CheckpointPhaseInit,
					CoreConflict: conflict1,
					UpdatedAt:    time.Now(),
				})
			}
		}
	}

	// Ledger aggregate marker for the bootstrap run.
	_ = s.store.AppendEvent(r.Context(), &activity.ActivityEvent{
		ProjectID:  projectID,
		RunID:      bootRunID,
		EntityType: activity.EntityFramework,
		EntityID:   projectID,
		EventType:  activity.EventFrameworkBoot,
		Actor:      activity.ActorAIReasoner,
		Source:     activity.SourceBootstrap,
		Title:      fmt.Sprintf("创世总纲已推演《%s》", req.Title),
		Summary: fmt.Sprintf("%d 卷 · %d 势力 · %d 关键人物 · %d 种子伏笔",
			len(fw.VolumeArcs), len(fw.Factions), len(fw.KeyCharacters), len(fw.SeedHooks)),
		CreatedAt: time.Now(),
	})

	// Transient seeds have been persisted to codex_entries and plot_hooks.
	// Clean them from the macro framework to maintain a single source of truth.
	fw.Factions = nil
	fw.KeyCharacters = nil
	fw.SeedHooks = nil
	_ = s.store.SaveProject(r.Context(), proj)

	jsonResponse(w, http.StatusCreated, proj)
}

func (s *Server) handleProjectExport(w http.ResponseWriter, r *http.Request, projectID, format string) {
	ctx := r.Context()
	p, err := s.store.GetProject(ctx, projectID)
	if err != nil {
		errorResponse(w, http.StatusNotFound, "project not found")
		return
	}

	chapters, _ := s.store.ListChapters(ctx, projectID)
	hooks, _ := s.store.ListPlotHooks(ctx, projectID)

	switch format {
	case "markdown":
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("# %s\n\n", p.Title))
		sb.WriteString(fmt.Sprintf("> 目标平台: %s | 导出时间: %s\n\n", p.TargetPlatform, time.Now().Format("2006-01-02 15:04:05")))
		sb.WriteString("## 世界公理与设定\n\n" + p.WorldRules + "\n\n---\n\n")

		for _, c := range chapters {
			sb.WriteString(fmt.Sprintf("## 第 %d 章: %s\n\n", c.ChapterIndex, c.Title))
			sb.WriteString(c.Content + "\n\n---\n\n")
		}

		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s_full.md", p.Title))
		_, _ = w.Write([]byte(sb.String()))

	case "json":
		snapshot := map[string]any{
			"project":   p,
			"chapters":  chapters,
			"hooks":     hooks,
			"export_at": time.Now(),
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s_snapshot.json", p.Title))
		_ = json.NewEncoder(w).Encode(snapshot)

	default:
		errorResponse(w, http.StatusBadRequest, "unsupported export format")
	}
}
