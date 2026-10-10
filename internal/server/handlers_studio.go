package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/Jungley8/novel-studio/internal/activity"
	"github.com/Jungley8/novel-studio/internal/domain"
	"github.com/Jungley8/novel-studio/internal/engine"
)

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
}

// StudioChatResponse represents the response body for Studio-level assistant dialogue.
type StudioChatResponse struct {
	Reply      string            `json:"reply"`
	Mode       string            `json:"mode"`
	ProjectID  string            `json:"project_id,omitempty"`
	TokensUsed engine.TokenUsage `json:"tokens_used"`
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

	// 1. Gather all projects, chapters, and hooks; determine global max archived chapter (全库最大已归档章)
	globalMaxArchivedChapter := 0

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
		for _, ch := range chapters {
			if ch.ChapterIndex > globalMaxArchivedChapter {
				globalMaxArchivedChapter = ch.ChapterIndex
			}
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
		openHooks := 0
		overdueHooks := 0
		for _, h := range snap.hooks {
			if h.Status == domain.HookStatusOpen {
				openHooks++
				if globalMaxArchivedChapter > 0 && h.TargetChapter < globalMaxArchivedChapter {
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

// handleStudioChat provides cross-project or horizon-backed conversational brainstorming.
// POST /api/studio/chat
func (s *Server) handleStudioChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
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
	var sysPrompt string

	if req.ProjectID != "" {
		p, err := s.store.GetProject(ctx, req.ProjectID)
		if err != nil || p == nil {
			errorResponse(w, http.StatusNotFound, "project not found")
			return
		}

		// 薄封装：调用 s.chronicle.AssembleHorizon 获取项目 horizon 上下文（与 preview handler 同式）
		targetChapter := 1
		if chapters, err := s.store.ListChapters(ctx, req.ProjectID); err == nil && len(chapters) > 0 {
			targetChapter = len(chapters) + 1
		}

		var canonText string
		if s.chronicle != nil {
			if horizon, err := s.chronicle.AssembleHorizon(ctx, req.ProjectID, targetChapter); err == nil && horizon != nil {
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

	jsonResponse(w, http.StatusOK, StudioChatResponse{
		Reply:      strings.TrimSpace(resText),
		Mode:       mode,
		ProjectID:  req.ProjectID,
		TokensUsed: usage,
	})
}
