package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Jungley8/novel-studio/internal/activity"
	"github.com/Jungley8/novel-studio/internal/domain"
	"github.com/Jungley8/novel-studio/internal/engine"
)

type studioThreadStore interface {
	CreateStudioThread(ctx context.Context, t *domain.StudioThread) error
	GetStudioThread(ctx context.Context, id string) (*domain.StudioThread, error)
	ListStudioThreads(ctx context.Context) ([]*domain.StudioThread, error)
	DeleteStudioThread(ctx context.Context, id string) error
	AppendStudioMessage(ctx context.Context, m *domain.StudioMessage) error
	ListStudioMessages(ctx context.Context, threadID string) ([]*domain.StudioMessage, error)
}

// StudioLibraryItem represents the cross-project aggregated status for Studio overview.
type StudioLibraryItem struct {
	ProjectID        string  `json:"project_id"`
	Title            string  `json:"title"`
	Platform         string  `json:"platform"`
	ArchivedChapters int     `json:"archived_chapters"`
	OpenHooks        int     `json:"open_hooks"`
	OverdueHooks     int     `json:"overdue_hooks"`
	TokenUsage       int     `json:"token_usage"`
	ReviewPassRate   float64 `json:"review_pass_rate"`
}

// StudioChatRequest represents the request body for Studio-level assistant dialogue.
type StudioChatRequest struct {
	ProjectID string `json:"project_id,omitempty"`
	Mode      string `json:"mode,omitempty"`
	Message   string `json:"message"`
	ThreadID  string `json:"thread_id,omitempty"`
}

// StudioChatResponse represents the response body for Studio-level assistant dialogue.
type StudioChatResponse struct {
	Reply      string            `json:"reply"`
	Mode       string            `json:"mode"`
	ProjectID  string            `json:"project_id,omitempty"`
	TokensUsed engine.TokenUsage `json:"tokens_used"`
	ThreadID   string            `json:"thread_id"`
	MessageID  string            `json:"message_id"`
}

// handleStudioLibrary aggregates cross-project statistics for the Studio Library overview.
// GET /api/studio/library
func (s *Server) handleStudioLibrary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	ctx := r.Context()
	projects, err := s.store.ListProjects(ctx)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, "list projects: "+err.Error())
		return
	}

	if len(projects) == 0 {
		jsonResponse(w, http.StatusOK, []StudioLibraryItem{})
		return
	}

	// 1. Gather all projects, chapters, and hooks.
	// Overdue is evaluated per project: a hook is overdue only when its OWN
	// project's progress has passed the hook's target chapter. A global max
	// across projects would wrongly mark a slow project's hooks overdue
	// just because another project is further along.

	type projectSnapshot struct {
		project  *domain.Project
		chapters []*domain.Chapter
		hooks    []*domain.PlotHook
	}

	snapshots := make([]projectSnapshot, 0, len(projects))
	for _, p := range projects {
		chapters, err := s.store.ListChapters(ctx, p.ID)
		if err != nil {
			chapters = []*domain.Chapter{}
		}

		hooks, err := s.store.ListPlotHooks(ctx, p.ID)
		if err != nil {
			hooks = []*domain.PlotHook{}
		}

		snapshots = append(snapshots, projectSnapshot{
			project:  p,
			chapters: chapters,
			hooks:    hooks,
		})
	}

	// 2. Aggregate per-project metrics
	items := make([]StudioLibraryItem, 0, len(snapshots))
	for _, snap := range snapshots {
		projMaxChapter := 0
		for _, ch := range snap.chapters {
			if ch.ChapterIndex > projMaxChapter {
				projMaxChapter = ch.ChapterIndex
			}
		}
		openHooks := 0
		overdueHooks := 0
		for _, h := range snap.hooks {
			if h.Status == domain.HookStatusOpen {
				openHooks++
				if projMaxChapter > 0 && h.TargetChapter < projMaxChapter {
					overdueHooks++
				}
			}
		}

		archivedCount := len(snap.chapters)
		passedCount := 0
		for _, ch := range snap.chapters {
			if (ch.Review != nil && ch.Review.Verdict == domain.ReviewVerdictAccepted) || ch.LinterPassed {
				passedCount++
			}
		}
		passRate := 0.0
		if archivedCount > 0 {
			passRate = float64(passedCount) / float64(archivedCount)
		}

		// Sum token usage from activity events
		totalTokens := 0
		if events, _, err := s.store.ListEvents(ctx, activity.EventQuery{ProjectID: snap.project.ID, Limit: 500}); err == nil {
			for _, ev := range events {
				if ev.TokenUsage != "" {
					var tu struct {
						TotalTokens int `json:"total_tokens"`
					}
					if err := json.Unmarshal([]byte(ev.TokenUsage), &tu); err == nil {
						totalTokens += tu.TotalTokens
					}
				}
			}
		}

		platform := snap.project.TargetPlatform
		if platform == "" {
			platform = "通用网文"
		}

		items = append(items, StudioLibraryItem{
			ProjectID:        snap.project.ID,
			Title:            snap.project.Title,
			Platform:         platform,
			ArchivedChapters: archivedCount,
			OpenHooks:        openHooks,
			OverdueHooks:     overdueHooks,
			TokenUsage:       totalTokens,
			ReviewPassRate:   passRate,
		})
	}

	jsonResponse(w, http.StatusOK, items)
}

// handleStudioChat provides cross-project or horizon-backed conversational brainstorming with persistence.
// POST /api/studio/chat
func (s *Server) handleStudioChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	stStore, ok := s.store.(studioThreadStore)
	if !ok {
		errorResponse(w, http.StatusInternalServerError, "store does not support studio threads (fail-closed)")
		return
	}

	var req StudioChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}

	req.Message = strings.TrimSpace(req.Message)
	if req.Message == "" {
		errorResponse(w, http.StatusBadRequest, "message cannot be empty")
		return
	}

	mode := req.Mode
	if mode == "" {
		mode = "scene_brainstorm"
	}
	switch mode {
	case "character_roleplay", "scene_brainstorm", "editor_critique":
	default:
		mode = "scene_brainstorm"
	}

	ctx := r.Context()
	var thread *domain.StudioThread
	if req.ThreadID != "" {
		t, err := stStore.GetStudioThread(ctx, req.ThreadID)
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, "get studio thread: "+err.Error())
			return
		}
		if t == nil {
			errorResponse(w, http.StatusNotFound, "thread not found")
			return
		}
		thread = t
	} else {
		// When creating new thread with requested project_id, fail-closed if project does not exist
		if req.ProjectID != "" {
			p, err := s.store.GetProject(ctx, req.ProjectID)
			if err != nil || p == nil {
				errorResponse(w, http.StatusNotFound, "project not found")
				return
			}
		}

		runes := []rune(req.Message)
		title := req.Message
		if len(runes) > 30 {
			title = string(runes[:30]) + "…"
		}
		now := time.Now()
		thread = &domain.StudioThread{
			ID:        fmt.Sprintf("th_%d", now.UnixNano()),
			Title:     title,
			ProjectID: req.ProjectID,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := stStore.CreateStudioThread(ctx, thread); err != nil {
			errorResponse(w, http.StatusInternalServerError, "create studio thread: "+err.Error())
			return
		}
	}

	userMsg := &domain.StudioMessage{
		ID:        fmt.Sprintf("msg_%d_u", time.Now().UnixNano()),
		ThreadID:  thread.ID,
		Role:      "user",
		Content:   req.Message,
		Mode:      mode,
		ProjectID: thread.ProjectID,
		CreatedAt: time.Now(),
	}
	if err := stStore.AppendStudioMessage(ctx, userMsg); err != nil {
		errorResponse(w, http.StatusInternalServerError, "append user message: "+err.Error())
		return
	}

	var sysPrompt string
	if thread.ProjectID != "" {
		p, err := s.store.GetProject(ctx, thread.ProjectID)
		if err != nil || p == nil {
			errorResponse(w, http.StatusNotFound, "project not found")
			return
		}

		// 薄封装：调用 s.chronicle.AssembleHorizon 获取项目 horizon 上下文（与 preview handler 同式）
		targetChapter := 1
		if chapters, err := s.store.ListChapters(ctx, thread.ProjectID); err == nil && len(chapters) > 0 {
			targetChapter = len(chapters) + 1
		}

		var canonText string
		if s.chronicle != nil {
			if horizon, err := s.chronicle.AssembleHorizon(ctx, thread.ProjectID, targetChapter); err == nil && horizon != nil {
				canonText = horizon.RollingCanonText
				if horizon.TailAnchor != "" {
					canonText += "\n" + horizon.TailAnchor
				}
			}
		}

		switch mode {
		case "character_roleplay":
			sysPrompt = fmt.Sprintf(`你是小说《%s》中的核心角色智囊。
【世界观设定】%s
【目标平台】%s
【剧情记忆与走向】%s
请严格代入该小说的设定与风格，与创作者进行推演交谈。`, p.Title, p.WorldRules, p.TargetPlatform, canonText)
		case "editor_critique":
			sysPrompt = fmt.Sprintf(`你是小说《%s》的严厉资深责任总编。
【世界观设定】%s
【目标平台】%s
【剧情记忆与走向】%s
请站在严密因果逻辑、读者心流体验与节奏把控的角度，针对创作者的剧情设想进行犀利点评，指出潜在逻辑漏洞或情节套路，并提出建设性的重构建议。`, p.Title, p.WorldRules, p.TargetPlatform, canonText)
		default: // scene_brainstorm
			sysPrompt = fmt.Sprintf(`你是小说《%s》的资深剧情策划与头脑风暴助理。
【世界观设定】%s
【目标平台】%s
【剧情记忆与走向】%s
你的职责是结合当前作品的设定与走向，协助作者攻克卡文瓶颈、设计反转桥段、推演动机与冲突阻碍。回复必须富有启发性、直接切入戏剧冲突核心。`, p.Title, p.WorldRules, p.TargetPlatform, canonText)
		}
	} else {
		// 通用脑暴（无 project_id）
		switch mode {
		case "character_roleplay":
			sysPrompt = `你是小说角色扮演与人设推演智囊。请协助作者探讨跨题材的人物心理动机、弧光成长与人设台词。`
		case "editor_critique":
			sysPrompt = `你是资深小说责任总编与作品评审专家。请站在严密因果逻辑、读者心流体验与节奏把控的角度，针对创作者的构思进行犀利点评，指出潜在逻辑漏洞并提出建设性的重构建议。`
		default: // scene_brainstorm
			sysPrompt = `你是全能的小说创作剧情策划与宏观灵感智囊。你的职责是协助作者进行跨项目全局头脑风暴，攻克卡文瓶颈、设计反转桥段、推演动机与世界观冲突。回复必须富有启发性、直接切入戏剧冲突核心。`
		}

		projects, err := s.store.ListProjects(ctx)
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, "list projects: "+err.Error())
			return
		}

		var overview strings.Builder
		overview.WriteString("【全库作品概览（跨项目脑暴上下文）】\n")
		if len(projects) == 0 {
			overview.WriteString("（全库暂无作品）")
		} else {
			for i, p := range projects {
				if i > 0 {
					overview.WriteString("\n")
				}
				var parts []string
				titlePart := fmt.Sprintf("《%s》", p.Title)
				if strings.TrimSpace(p.TargetPlatform) != "" {
					titlePart += fmt.Sprintf("（%s）", strings.TrimSpace(p.TargetPlatform))
				}
				parts = append(parts, titlePart)

				if p.Framework != nil && strings.TrimSpace(p.Framework.ThemePremise) != "" {
					premise := strings.TrimSpace(p.Framework.ThemePremise)
					runes := []rune(premise)
					if len(runes) > 100 {
						premise = string(runes[:100])
					}
					parts = append(parts, fmt.Sprintf("立意：%s", premise))
				}

				chapters, err := s.store.ListChapters(ctx, p.ID)
				if err != nil {
					chapters = nil
				}
				parts = append(parts, fmt.Sprintf("已归档章数：%d", len(chapters)))

				hooks, err := s.store.ListPlotHooks(ctx, p.ID)
				if err != nil {
					hooks = nil
				}
				openHooks := 0
				for _, h := range hooks {
					if h != nil && h.Status == domain.HookStatusOpen {
						openHooks++
					}
				}
				parts = append(parts, fmt.Sprintf("开放钩数：%d", openHooks))

				overview.WriteString("- " + strings.Join(parts, " | "))
			}
		}

		sysPrompt += "\n\n" + overview.String()
	}

	if s.llmRouter == nil {
		errorResponse(w, http.StatusInternalServerError, "llm client is not configured")
		return
	}

	targetModel := s.cfg.ReasoningModel
	resText, usage, err := s.llmRouter.ChatCompletionForRole(ctx, engine.RoleReasoner, targetModel, sysPrompt, req.Message, 0.7)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	usageBytes, _ := json.Marshal(usage)
	asstNow := time.Now()
	if !asstNow.After(userMsg.CreatedAt) {
		asstNow = userMsg.CreatedAt.Add(time.Millisecond)
	}
	assistantMsg := &domain.StudioMessage{
		ID:         fmt.Sprintf("msg_%d_a", asstNow.UnixNano()),
		ThreadID:   thread.ID,
		Role:       "assistant",
		Content:    strings.TrimSpace(resText),
		Mode:       mode,
		ProjectID:  thread.ProjectID,
		TokenUsage: string(usageBytes),
		CreatedAt:  asstNow,
	}
	if err := stStore.AppendStudioMessage(ctx, assistantMsg); err != nil {
		errorResponse(w, http.StatusInternalServerError, "append assistant message: "+err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, StudioChatResponse{
		Reply:      strings.TrimSpace(resText),
		Mode:       mode,
		ProjectID:  thread.ProjectID,
		TokensUsed: usage,
		ThreadID:   thread.ID,
		MessageID:  assistantMsg.ID,
	})
}

// handleStudioThreads lists all studio conversation threads.
// GET /api/studio/threads
func (s *Server) handleStudioThreads(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	stStore, ok := s.store.(studioThreadStore)
	if !ok {
		errorResponse(w, http.StatusInternalServerError, "store does not support studio threads (fail-closed)")
		return
	}

	threads, err := stStore.ListStudioThreads(r.Context())
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, "list studio threads: "+err.Error())
		return
	}
	if threads == nil {
		threads = make([]*domain.StudioThread, 0)
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"threads": threads,
	})
}

// handleStudioThreadRoutes handles individual thread operations (GET/DELETE).
// GET /api/studio/threads/{id}
// DELETE /api/studio/threads/{id}
func (s *Server) handleStudioThreadRoutes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodDelete {
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	stStore, ok := s.store.(studioThreadStore)
	if !ok {
		errorResponse(w, http.StatusInternalServerError, "store does not support studio threads (fail-closed)")
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/studio/threads/")
	id = strings.TrimSpace(id)
	if id == "" {
		errorResponse(w, http.StatusBadRequest, "thread id cannot be empty")
		return
	}

	ctx := r.Context()
	switch r.Method {
	case http.MethodGet:
		thread, err := stStore.GetStudioThread(ctx, id)
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, "get studio thread: "+err.Error())
			return
		}
		if thread == nil {
			errorResponse(w, http.StatusNotFound, "thread not found")
			return
		}

		messages, err := stStore.ListStudioMessages(ctx, id)
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, "list studio messages: "+err.Error())
			return
		}
		if messages == nil {
			messages = make([]*domain.StudioMessage, 0)
		}

		jsonResponse(w, http.StatusOK, map[string]any{
			"thread":   thread,
			"messages": messages,
		})
	case http.MethodDelete:
		if err := stStore.DeleteStudioThread(ctx, id); err != nil {
			if strings.Contains(err.Error(), "not found") {
				errorResponse(w, http.StatusNotFound, "thread not found")
				return
			}
			errorResponse(w, http.StatusInternalServerError, "delete studio thread: "+err.Error())
			return
		}
		jsonResponse(w, http.StatusOK, map[string]string{
			"status": "deleted",
		})
	}
}
