package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Jungley8/novel-studio/internal/coord"
	"github.com/Jungley8/novel-studio/internal/domain"
	"github.com/Jungley8/novel-studio/internal/engine"
)

func (s *Server) handleDeriveBeats(w http.ResponseWriter, r *http.Request, projectID string) {
	if r.Method != http.MethodPost {
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		ChapterIndex int    `json:"chapter_index"`
		CoreConflict string `json:"core_conflict"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	if s.orch == nil || s.chronicle == nil {
		errorResponse(w, http.StatusInternalServerError, "orchestrator or chronicle is not configured")
		return
	}

	horizon, err := s.chronicle.AssembleHorizon(r.Context(), projectID, req.ChapterIndex)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	out, err := s.orch.DeriveBeatsWithHorizon(r.Context(), s.cfg.ReasoningModel, horizon, req.CoreConflict)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Auto-persist checkpoint on successful derive beats
	existingCp, _ := s.store.GetCheckpoint(r.Context(), projectID, req.ChapterIndex)
	cp := &domain.ChapterCheckpoint{
		ProjectID:     projectID,
		ChapterIndex:  req.ChapterIndex,
		Phase:         domain.CheckpointPhaseBeats,
		CoreConflict:  req.CoreConflict,
		Beats:         out.Beats,
		StateMutation: out.StateMutation,
		UpdatedAt:     time.Now(),
	}
	if existingCp != nil {
		cp.DraftText = existingCp.DraftText
		cp.AuditReport = existingCp.AuditReport
		cp.RewriteLoops = existingCp.RewriteLoops
	}
	_ = s.store.SaveCheckpoint(r.Context(), cp)

	jsonResponse(w, http.StatusOK, out)
}

func (s *Server) handleRenderScene(w http.ResponseWriter, r *http.Request, projectID string) {
	if r.Method != http.MethodPost {
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		ChapterIndex   int                `json:"chapter_index"`
		Beats          []domain.SceneBeat `json:"beats"`
		WordsTarget    int                `json:"words_target"`
		NarrativeStyle string             `json:"narrative_style"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	p, err := s.store.GetProject(r.Context(), projectID)
	if err != nil {
		errorResponse(w, http.StatusNotFound, "project not found")
		return
	}

	if s.orch == nil {
		errorResponse(w, http.StatusInternalServerError, "narrative orchestrator is not configured")
		return
	}

	var horizon *engine.CanonHorizon
	if s.chronicle != nil {
		h, err := s.chronicle.AssembleHorizon(r.Context(), projectID, req.ChapterIndex)
		if err == nil {
			horizon = h
		}
	}
	if horizon == nil {
		prot, _ := s.store.GetProtagonistEntry(r.Context(), projectID)
		horizon = &engine.CanonHorizon{
			Project:          p,
			TargetChapter:    req.ChapterIndex,
			ProtagonistEntry: prot,
			WorldRules:       p.WorldRules,
		}
	}
	if req.NarrativeStyle != "" {
		horizon.NarrativeTone = req.NarrativeStyle
	}

	isSSE := strings.Contains(r.Header.Get("Accept"), "text/event-stream") || r.URL.Query().Get("stream") == "true"
	if isSSE {
		rc := http.NewResponseController(w)
		_ = rc.SetWriteDeadline(time.Time{})

		w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		var writeMu sync.Mutex
		safeWrite := func(data string) {
			writeMu.Lock()
			defer writeMu.Unlock()
			_, _ = fmt.Fprint(w, data)
			flusher.Flush()
		}

		stopHeartbeat := make(chan struct{})
		defer close(stopHeartbeat)
		go func() {
			ticker := time.NewTicker(3 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-stopHeartbeat:
					return
				case <-ticker.C:
					safeWrite(": keep-alive\n\n")
				}
			}
		}()

		content, _, err := s.orch.RenderSceneStream(r.Context(), s.cfg.WriterModel, horizon, req.Beats, req.WordsTarget, func(delta string) {
			deltaPayload, _ := json.Marshal(map[string]string{"delta": delta})
			safeWrite(fmt.Sprintf("event: delta\ndata: %s\n\n", string(deltaPayload)))
		})
		if err != nil {
			errPayload, _ := json.Marshal(map[string]string{"error": err.Error()})
			safeWrite(fmt.Sprintf("event: error\ndata: %s\n\n", string(errPayload)))
			return
		}

		existingRenderCp, _ := s.store.GetCheckpoint(r.Context(), projectID, req.ChapterIndex)
		renderCp := &domain.ChapterCheckpoint{
			ProjectID:    projectID,
			ChapterIndex: req.ChapterIndex,
			Phase:        domain.CheckpointPhaseDrafted,
			Beats:        req.Beats,
			DraftText:    content,
			UpdatedAt:    time.Now(),
		}
		if existingRenderCp != nil {
			renderCp.CoreConflict = existingRenderCp.CoreConflict
			renderCp.StateMutation = existingRenderCp.StateMutation
			renderCp.RewriteLoops = existingRenderCp.RewriteLoops
			renderCp.AuditReport = existingRenderCp.AuditReport
		}
		_ = s.store.SaveCheckpoint(r.Context(), renderCp)

		donePayload, _ := json.Marshal(map[string]any{
			"content":    content,
			"checkpoint": renderCp,
		})
		safeWrite(fmt.Sprintf("event: complete\ndata: %s\n\n", string(donePayload)))
		safeWrite(fmt.Sprintf("event: done\ndata: %s\n\n", string(donePayload)))
		return
	}

	content, _, err := s.orch.RenderSceneWithHorizon(r.Context(), s.cfg.WriterModel, horizon, req.Beats, req.WordsTarget)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Auto-persist checkpoint on successful render
	existingRenderCp, _ := s.store.GetCheckpoint(r.Context(), projectID, req.ChapterIndex)
	renderCp := &domain.ChapterCheckpoint{
		ProjectID:    projectID,
		ChapterIndex: req.ChapterIndex,
		Phase:        domain.CheckpointPhaseDrafted,
		Beats:        req.Beats,
		DraftText:    content,
		UpdatedAt:    time.Now(),
	}
	if existingRenderCp != nil {
		renderCp.CoreConflict = existingRenderCp.CoreConflict
		renderCp.StateMutation = existingRenderCp.StateMutation
		renderCp.RewriteLoops = existingRenderCp.RewriteLoops
		renderCp.AuditReport = existingRenderCp.AuditReport
	}
	_ = s.store.SaveCheckpoint(r.Context(), renderCp)

	jsonResponse(w, http.StatusOK, map[string]any{
		"content":    content,
		"checkpoint": renderCp,
	})
}

func (s *Server) handleReviewDraft(w http.ResponseWriter, r *http.Request, projectID string) {
	if r.Method != http.MethodPost {
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		ChapterIndex  int                `json:"chapter_index"`
		Beats         []domain.SceneBeat `json:"beats"`
		DraftText     string             `json:"draft_text"`
		Content       string             `json:"content"`
		ReviewerModel string             `json:"reviewer_model"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	if strings.TrimSpace(req.DraftText) == "" && strings.TrimSpace(req.Content) != "" {
		req.DraftText = req.Content
	}

	p, err := s.store.GetProject(r.Context(), projectID)
	if err != nil {
		errorResponse(w, http.StatusNotFound, "project not found")
		return
	}

	if s.qualityGate == nil {
		errorResponse(w, http.StatusInternalServerError, "quality gate not configured")
		return
	}

	reviewerModel := strings.TrimSpace(req.ReviewerModel)
	if reviewerModel == "" {
		reviewerModel = strings.TrimSpace(s.cfg.ReviewerModel)
	}
	if reviewerModel == "" {
		reviewerModel = strings.TrimSpace(s.cfg.ReasoningModel)
	}
	if reviewerModel == "" {
		reviewerModel = strings.TrimSpace(s.cfg.WriterModel)
	}

	prot, _ := s.store.GetProtagonistEntry(r.Context(), projectID)

	// 伏笔证据门前置：手动审阅与生产路径同口径——注入 open/fermenting 伏笔，评审声明须过确定性证据门。
	hooks, herr := s.store.ListPlotHooks(r.Context(), projectID)
	if herr != nil {
		errorResponse(w, http.StatusInternalServerError, fmt.Sprintf("伏笔台账读取失败，拒绝审阅（Fail-Closed）: %v", herr))
		return
	}
	activeHooks := make([]*domain.PlotHook, 0, len(hooks))
	for _, h := range hooks {
		if h != nil && (h.Status == domain.HookStatusOpen || h.Status == domain.HookStatusFermenting) {
			activeHooks = append(activeHooks, h)
		}
	}

	audit, _, err := s.qualityGate.AuditWithHooks(r.Context(), reviewerModel, p, prot, req.ChapterIndex, req.Beats, activeHooks, req.DraftText)
	if err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "Model is unavailable") {
			errStr = fmt.Sprintf("体检失败: 模型 [%s] 在当前接口不可用。请前往【设置】将“文风体检”或“剧情构思”更新为你当前可用的模型名称: %s", reviewerModel, errStr)
		}
		errorResponse(w, http.StatusInternalServerError, errStr)
		return
	}

	engine.GateResolvedHookClaims(audit, activeHooks, req.DraftText)

	// Auto-persist checkpoint on review
	existingAuditCp, _ := s.store.GetCheckpoint(r.Context(), projectID, req.ChapterIndex)
	auditCp := &domain.ChapterCheckpoint{
		ProjectID:    projectID,
		ChapterIndex: req.ChapterIndex,
		Phase:        domain.CheckpointPhaseAudited,
		Beats:        req.Beats,
		DraftText:    req.DraftText,
		AuditReport:  audit,
		UpdatedAt:    time.Now(),
	}
	if existingAuditCp != nil {
		auditCp.CoreConflict = existingAuditCp.CoreConflict
		auditCp.StateMutation = existingAuditCp.StateMutation
		auditCp.RewriteLoops = existingAuditCp.RewriteLoops
	}
	_ = s.store.SaveCheckpoint(r.Context(), auditCp)

	jsonResponse(w, http.StatusOK, audit.ToReviewResult())
}

func (s *Server) handleWorkshopProduce(w http.ResponseWriter, r *http.Request, projectID string) {
	if r.Method != http.MethodPost {
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)

	var req engine.WorkshopProduceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	req.ProjectID = projectID
	if req.ReasoningModel == "" {
		req.ReasoningModel = s.cfg.ReasoningModel
	}
	if req.WriterModel == "" {
		req.WriterModel = s.cfg.WriterModel
	}
	if req.ReviewerModel == "" {
		req.ReviewerModel = s.cfg.ReviewerModel
		if req.ReviewerModel == "" {
			req.ReviewerModel = s.cfg.ReasoningModel
		}
	}

	if s.workshop == nil {
		errorResponse(w, http.StatusInternalServerError, "workshop not configured")
		return
	}

	// Fail-Closed：并发生产互斥。人工点击“一键成章”视为人工接管：
	// 先释放本窗口的编辑租约，再登记跨进程生产锁（SQLite 单一事实源），
	// 调度器/CLI 在此期间会因锁在位而拒绝/跳过同一作品。
	ctx := r.Context()
	_ = s.store.DeleteEditLease(ctx, projectID)
	holder := fmt.Sprintf("ui-%d", time.Now().UnixNano())
	blocking, err := s.store.AcquireProductionLease(ctx, projectID, holder, "一键成章", req.ChapterIndex, coord.ProductionLeaseTTL)
	if err != nil {
		if errors.Is(err, coord.ErrBusy) {
			switch {
			case blocking != nil && blocking.Kind == coord.KindEdit:
				errorResponse(w, http.StatusConflict, "作品存在人工编辑租约（另一窗口正在编辑），已拒绝并发生产")
			case blocking != nil && blocking.Chapter > 0:
				errorResponse(w, http.StatusConflict, fmt.Sprintf("该作品正在生产（%s·第 %d 章），请等待完成后再试", blocking.Label, blocking.Chapter))
			case blocking != nil:
				errorResponse(w, http.StatusConflict, fmt.Sprintf("该作品正在生产（%s），请等待完成后再试", blocking.Label))
			default:
				errorResponse(w, http.StatusConflict, "该作品已有在途生产任务，请等待完成后再试")
			}
		} else {
			errorResponse(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	defer func() { _ = s.store.ReleaseProductionLease(context.Background(), projectID, holder) }()

	// 生产锁心跳：SSE 长连接可能持续数分钟，按固定间隔续期并跟随章节号。
	hbCtx, hbCancel := context.WithCancel(ctx)
	defer hbCancel()
	go func() {
		ticker := time.NewTicker(coord.ProductionHeartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-hbCtx.Done():
				return
			case <-ticker.C:
				_, _ = s.store.HeartbeatProductionLease(hbCtx, projectID, holder, req.ChapterIndex, coord.ProductionLeaseTTL)
			}
		}
	}()

	isSSE := strings.Contains(r.Header.Get("Accept"), "text/event-stream") || r.URL.Query().Get("stream") == "true"
	if isSSE {
		// Clear any write deadline for this long-lived SSE streaming connection
		rc := http.NewResponseController(w)
		_ = rc.SetWriteDeadline(time.Time{})

		w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		var writeMu sync.Mutex
		safeWrite := func(data string) {
			writeMu.Lock()
			defer writeMu.Unlock()
			_, _ = fmt.Fprint(w, data)
			flusher.Flush()
		}

		// Background heartbeat to prevent idle connection drop / ERR_INCOMPLETE_CHUNKED_ENCODING during long LLM calls
		stopHeartbeat := make(chan struct{})
		defer close(stopHeartbeat)
		go func() {
			ticker := time.NewTicker(15 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					safeWrite(": ping\n\n")
				case <-stopHeartbeat:
					return
				case <-r.Context().Done():
					return
				}
			}
		}()

		req.OnProgress = func(ev engine.WorkshopEvent) {
			evBytes, _ := json.Marshal(ev)
			safeWrite(fmt.Sprintf("event: progress\ndata: %s\n\n", string(evBytes)))
		}

		res, err := s.workshop.ProduceChapter(r.Context(), req)
		if err != nil {
			errPayload, _ := json.Marshal(map[string]string{"error": err.Error()})
			safeWrite(fmt.Sprintf("event: error\ndata: %s\n\n", string(errPayload)))
			return
		}

		resultPayload, _ := json.Marshal(res)
		safeWrite(fmt.Sprintf("event: complete\ndata: %s\n\n", string(resultPayload)))
		return
	}

	res, err := s.workshop.ProduceChapter(r.Context(), req)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, res)
}

func (s *Server) handleRewriteDraft(w http.ResponseWriter, r *http.Request, projectID string) {
	if r.Method != http.MethodPost {
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		ChapterIndex   int                  `json:"chapter_index"`
		OriginalDraft  string               `json:"original_draft"`
		Content        string               `json:"content"`
		Issues         []string             `json:"issues"`
		Suggestions    string               `json:"suggestions"`
		Review         *domain.ReviewResult `json:"review"`
		WordsTarget    int                  `json:"words_target"`
		NarrativeStyle string               `json:"narrative_style"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	if strings.TrimSpace(req.OriginalDraft) == "" && strings.TrimSpace(req.Content) != "" {
		req.OriginalDraft = req.Content
	}
	if strings.TrimSpace(req.OriginalDraft) == "" {
		errorResponse(w, http.StatusBadRequest, "original draft cannot be empty")
		return
	}

	if req.Review == nil {
		if len(req.Issues) > 0 || strings.TrimSpace(req.Suggestions) != "" {
			req.Review = &domain.ReviewResult{
				Verdict:     domain.ReviewVerdictRevision,
				Score:       70,
				Issues:      req.Issues,
				Suggestions: req.Suggestions,
				ReviewedAt:  time.Now(),
			}
		} else {
			req.Review = &domain.ReviewResult{
				Verdict:     domain.ReviewVerdictRevision,
				Score:       70,
				Issues:      []string{"需根据去AI味与语法健全原则进行深度精修"},
				Suggestions: "请重塑叙事节奏，补齐主谓宾完整结构，消除机械断句与模式化废词，增强骨肉质感与真实呼吸感。",
				ReviewedAt:  time.Now(),
			}
		}
	}

	p, err := s.store.GetProject(r.Context(), projectID)
	if err != nil {
		errorResponse(w, http.StatusNotFound, "project not found")
		return
	}

	if s.orch == nil {
		errorResponse(w, http.StatusInternalServerError, "orchestrator not configured")
		return
	}

	isSSE := strings.Contains(r.Header.Get("Accept"), "text/event-stream") || r.URL.Query().Get("stream") == "true"
	if isSSE {
		rc := http.NewResponseController(w)
		_ = rc.SetWriteDeadline(time.Time{})

		w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		var writeMu sync.Mutex
		safeWrite := func(data string) {
			writeMu.Lock()
			defer writeMu.Unlock()
			_, _ = fmt.Fprint(w, data)
			flusher.Flush()
		}

		stopHeartbeat := make(chan struct{})
		defer close(stopHeartbeat)
		go func() {
			ticker := time.NewTicker(3 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-stopHeartbeat:
					return
				case <-ticker.C:
					safeWrite(": keep-alive\n\n")
				}
			}
		}()

		rewritten, _, err := s.orch.RewriteDraftStream(r.Context(), s.cfg.WriterModel, p, req.ChapterIndex, req.OriginalDraft, req.Review, engine.RewriteOptions{
			WordsTarget:    req.WordsTarget,
			NarrativeStyle: req.NarrativeStyle,
		}, func(delta string) {
			deltaPayload, _ := json.Marshal(map[string]string{"delta": delta})
			safeWrite(fmt.Sprintf("event: delta\ndata: %s\n\n", string(deltaPayload)))
		})
		if err != nil {
			errPayload, _ := json.Marshal(map[string]string{"error": err.Error()})
			safeWrite(fmt.Sprintf("event: error\ndata: %s\n\n", string(errPayload)))
			return
		}

		existingRewriteCp, _ := s.store.GetCheckpoint(r.Context(), projectID, req.ChapterIndex)
		loops := 1
		if existingRewriteCp != nil {
			loops = existingRewriteCp.RewriteLoops + 1
		}
		rewriteCp := &domain.ChapterCheckpoint{
			ProjectID:    projectID,
			ChapterIndex: req.ChapterIndex,
			Phase:        domain.CheckpointPhaseRewriting,
			DraftText:    rewritten,
			RewriteLoops: loops,
			UpdatedAt:    time.Now(),
		}
		if existingRewriteCp != nil {
			rewriteCp.CoreConflict = existingRewriteCp.CoreConflict
			rewriteCp.Beats = existingRewriteCp.Beats
			rewriteCp.StateMutation = existingRewriteCp.StateMutation
			rewriteCp.AuditReport = existingRewriteCp.AuditReport
		}
		_ = s.store.SaveCheckpoint(r.Context(), rewriteCp)

		donePayload, _ := json.Marshal(map[string]any{
			"content":       rewritten,
			"rewrite_loops": loops,
			"checkpoint":    rewriteCp,
		})
		safeWrite(fmt.Sprintf("event: complete\ndata: %s\n\n", string(donePayload)))
		safeWrite(fmt.Sprintf("event: done\ndata: %s\n\n", string(donePayload)))
		return
	}

	rewritten, _, err := s.orch.RewriteDraftWithOptions(r.Context(), s.cfg.WriterModel, p, req.ChapterIndex, req.OriginalDraft, req.Review, engine.RewriteOptions{
		WordsTarget:    req.WordsTarget,
		NarrativeStyle: req.NarrativeStyle,
	})
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Auto-persist checkpoint on rewrite
	existingRewriteCp, _ := s.store.GetCheckpoint(r.Context(), projectID, req.ChapterIndex)
	loops := 1
	if existingRewriteCp != nil {
		loops = existingRewriteCp.RewriteLoops + 1
	}
	rewriteCp := &domain.ChapterCheckpoint{
		ProjectID:    projectID,
		ChapterIndex: req.ChapterIndex,
		Phase:        domain.CheckpointPhaseRewriting,
		DraftText:    rewritten,
		RewriteLoops: loops,
		UpdatedAt:    time.Now(),
	}
	if existingRewriteCp != nil {
		rewriteCp.CoreConflict = existingRewriteCp.CoreConflict
		rewriteCp.Beats = existingRewriteCp.Beats
		rewriteCp.StateMutation = existingRewriteCp.StateMutation
		rewriteCp.AuditReport = existingRewriteCp.AuditReport
	}
	_ = s.store.SaveCheckpoint(r.Context(), rewriteCp)

	jsonResponse(w, http.StatusOK, map[string]any{
		"content":       rewritten,
		"rewrite_loops": loops,
		"checkpoint":    rewriteCp,
	})
}
