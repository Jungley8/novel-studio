package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

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

	jsonResponse(w, http.StatusOK, out)
}

func (s *Server) handleRenderScene(w http.ResponseWriter, r *http.Request, projectID string) {
	if r.Method != http.MethodPost {
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		ChapterIndex int                `json:"chapter_index"`
		Beats        []domain.SceneBeat `json:"beats"`
		WordsTarget  int                `json:"words_target"`
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

	content, _, err := s.orch.RenderScene(r.Context(), s.cfg.WriterModel, p, req.ChapterIndex, req.Beats, req.WordsTarget)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, map[string]string{"content": content})
}

func (s *Server) handleReviewDraft(w http.ResponseWriter, r *http.Request, projectID string) {
	if r.Method != http.MethodPost {
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		ChapterIndex int                `json:"chapter_index"`
		Beats        []domain.SceneBeat `json:"beats"`
		DraftText    string             `json:"draft_text"`
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

	if s.qualityGate == nil {
		errorResponse(w, http.StatusInternalServerError, "quality gate not configured")
		return
	}

	audit, _, err := s.qualityGate.Audit(r.Context(), s.cfg.ReasoningModel, p, req.ChapterIndex, req.Beats, req.DraftText)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

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
		ChapterIndex  int                  `json:"chapter_index"`
		OriginalDraft string               `json:"original_draft"`
		Review        *domain.ReviewResult `json:"review"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Review == nil {
		errorResponse(w, http.StatusBadRequest, "review result is required for rewriting")
		return
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

	rewritten, _, err := s.orch.RewriteDraft(r.Context(), s.cfg.WriterModel, p, req.ChapterIndex, req.OriginalDraft, req.Review)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, map[string]string{"content": rewritten})
}

type InlineActionRequest struct {
	Action             string `json:"action"` // rewrite, expand, shorten, sensory, dialogue, custom
	Selection          string `json:"selection"`
	Instruction        string `json:"instruction,omitempty"`
	SurroundingContext string `json:"surrounding_context,omitempty"`
	Model              string `json:"model,omitempty"`
}

type InlineActionResponse struct {
	Result     string            `json:"result"`
	Action     string            `json:"action"`
	TokensUsed engine.TokenUsage `json:"tokens_used"`
}

func (s *Server) handleInlineAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req InlineActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	req.Selection = strings.TrimSpace(req.Selection)
	if req.Selection == "" {
		errorResponse(w, http.StatusBadRequest, "selection cannot be empty")
		return
	}

	if s.llmRouter == nil {
		errorResponse(w, http.StatusInternalServerError, "llm client is not configured")
		return
	}

	var sysPrompt string
	switch req.Action {
	case "rewrite":
		sysPrompt = `你是一名专业文学小说责编与润色大师。请在严格保留核心剧情、人物立场与事实不变的前提下，润色重写所选段落，消除行文冗余与粗糙感，提升语感、节奏和文学张力。严禁输出多余解释或寒暄，仅输出重写后的正文。`
	case "expand":
		sysPrompt = `你是一名具有丰富细节感知力的小说作家。请对所选段落进行生动的细节扩写，丰富微表情、环境声色、内心独白与五感氛围，强化现场感与代入感。严禁输出多余解释，仅输出扩写后的正文。`
	case "shorten":
		sysPrompt = `你是一名犀利严苛的小说编辑。请对所选段落进行精简浓缩，去除多余修饰词、套话及次要描写，保留戏剧冲突与高光时刻，使行文更加紧凑精炼。严禁输出多余解释，仅输出精简后的正文。`
	case "sensory":
		sysPrompt = `你是一名专注于五感沉浸的文学名家。请深入发掘所选段落中的视觉、听觉、嗅觉、触觉及生理体感，用极具质感的物态细节增强沉浸感，严禁使用陈词滥调。仅输出具象化增强后的正文。`
	case "dialogue":
		sysPrompt = `你是一名顶尖电影与小说台词名家。请润色打磨所选段落中的人物对话，强化角色口吻性格特质、言语潜台词与机锋对峙，削弱说明性台词。仅输出打磨后的对话正文。`
	default:
		sysPrompt = `你是一名专业小说创作助理。请根据用户的特别指令对选中文本进行处理。严禁多余废话，仅输出处理后的正文。`
	}

	var userPrompt strings.Builder
	if req.SurroundingContext != "" {
		userPrompt.WriteString("【上下文参考】\n")
		userPrompt.WriteString(req.SurroundingContext)
		userPrompt.WriteString("\n\n")
	}
	if req.Instruction != "" {
		userPrompt.WriteString("【修改要求】\n")
		userPrompt.WriteString(req.Instruction)
		userPrompt.WriteString("\n\n")
	}
	userPrompt.WriteString("【待处理文本】\n")
	userPrompt.WriteString(req.Selection)

	targetModel := req.Model
	if targetModel == "" {
		targetModel = s.cfg.WriterModel
	}

	resultText, usage, err := s.llmRouter.ChatCompletionForRole(r.Context(), engine.RoleWriter, targetModel, sysPrompt, userPrompt.String(), 0.7)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, InlineActionResponse{
		Result:     strings.TrimSpace(resultText),
		Action:     req.Action,
		TokensUsed: usage,
	})
}

type PromptPreviewRequest struct {
	ChapterIndex int      `json:"chapter_index"`
	SceneTitle   string   `json:"scene_title,omitempty"`
	POV          string   `json:"pov,omitempty"`
	DramaticGoal string   `json:"dramatic_goal,omitempty"`
	Conflict     string   `json:"conflict,omitempty"`
	CoreEvents   []string `json:"core_events,omitempty"`
}

type PromptComponentPreview struct {
	Name          string `json:"name"`
	Description   string `json:"description"`
	Content       string `json:"content"`
	TokenEstimate int    `json:"token_estimate"`
}

type PromptPreviewResponse struct {
	SystemPrompt        string                   `json:"system_prompt"`
	UserPrompt          string                   `json:"user_prompt"`
	Components          []PromptComponentPreview `json:"components"`
	TotalTokensEstimate int                      `json:"total_tokens_estimate"`
}

func estimateTokens(text string) int {
	runes := []rune(text)
	if len(runes) == 0 {
		return 0
	}
	toks := len(runes) * 3 / 4
	if toks < 1 {
		return 1
	}
	return toks
}

func (s *Server) handlePromptPreview(w http.ResponseWriter, r *http.Request, projectID string) {
	if r.Method != http.MethodPost {
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req PromptPreviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	if req.ChapterIndex <= 0 {
		req.ChapterIndex = 1
	}

	ctx := r.Context()
	p, err := s.store.GetProject(ctx, projectID)
	if err != nil || p == nil {
		errorResponse(w, http.StatusNotFound, "project not found")
		return
	}

	horizon, err := s.chronicle.AssembleHorizon(ctx, projectID, req.ChapterIndex)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, "assemble horizon: "+err.Error())
		return
	}

	frameworkContent := fmt.Sprintf("【世界观与公理】\n%s\n\n【目标平台调性】\n%s", p.WorldRules, p.TargetPlatform)
	if p.Framework != nil && p.Framework.ThemePremise != "" {
		frameworkContent += fmt.Sprintf("\n\n【核心立意与主线】\n%s", p.Framework.ThemePremise)
	}

	canonContent := fmt.Sprintf("【前序章节剧情记忆】\n%s", horizon.RollingCanonText)
	if horizon.TailAnchor != "" {
		canonContent += fmt.Sprintf("\n\n【上章末尾腔调锚定】\n%s", horizon.TailAnchor)
	}

	codexContent := horizon.CodexContextText
	if codexContent == "" {
		codexContent = "(当前章节未匹配到需强制注入的百科实体)"
	}

	var sceneSpec strings.Builder
	sceneSpec.WriteString(fmt.Sprintf("【目标章节】第 %d 章\n", req.ChapterIndex))
	if req.SceneTitle != "" {
		sceneSpec.WriteString(fmt.Sprintf("【场景标题】%s\n", req.SceneTitle))
	}
	if req.POV != "" {
		sceneSpec.WriteString(fmt.Sprintf("【视角人物(POV)】%s\n", req.POV))
	}
	if req.DramaticGoal != "" {
		sceneSpec.WriteString(fmt.Sprintf("【戏剧目标】%s\n", req.DramaticGoal))
	}
	if req.Conflict != "" {
		sceneSpec.WriteString(fmt.Sprintf("【阻碍与冲突】%s\n", req.Conflict))
	}
	if len(req.CoreEvents) > 0 {
		sceneSpec.WriteString("【核心事件节拍】\n")
		for i, ev := range req.CoreEvents {
			sceneSpec.WriteString(fmt.Sprintf("  %d. %s\n", i+1, ev))
		}
	}

	components := []PromptComponentPreview{
		{
			Name:          "世界观与风格公理 (Global Framework)",
			Description:   "包含世界设定、平台调性约束与核心爽点规则",
			Content:       frameworkContent,
			TokenEstimate: estimateTokens(frameworkContent),
		},
		{
			Name:          "因果故事线记忆 (Canon Horizon)",
			Description:   "包含前序章节滚雪球摘要与上章尾部文风锚点",
			Content:       canonContent,
			TokenEstimate: estimateTokens(canonContent),
		},
		{
			Name:          "全域百科与关联 (The Codex Injected)",
			Description:   "动态扫描注入的角色阶段属性、别名、物品状态及两两关系",
			Content:       codexContent,
			TokenEstimate: estimateTokens(codexContent),
		},
		{
			Name:          "场景戏剧规格 (Scene Dramaturgy)",
			Description:   "当前场次的 POV、戏剧目标、冲突阻碍与核心节拍事实",
			Content:       sceneSpec.String(),
			TokenEstimate: estimateTokens(sceneSpec.String()),
		},
	}

	totalTokens := 0
	for _, c := range components {
		totalTokens += c.TokenEstimate
	}

	systemPrompt := `你是一名冷峻、极具电影镜头感的小说名家。严格遵循因果设定与声口约束，杜绝AI套话与伪高潮。`
	var userPrompt strings.Builder
	userPrompt.WriteString(frameworkContent)
	userPrompt.WriteString("\n\n---\n\n")
	userPrompt.WriteString(canonContent)
	userPrompt.WriteString("\n\n---\n\n")
	userPrompt.WriteString(codexContent)
	userPrompt.WriteString("\n\n---\n\n")
	userPrompt.WriteString(sceneSpec.String())

	jsonResponse(w, http.StatusOK, PromptPreviewResponse{
		SystemPrompt:        systemPrompt,
		UserPrompt:          userPrompt.String(),
		Components:          components,
		TotalTokensEstimate: totalTokens,
	})
}

type WorkshopChatMessage struct {
	Role    string `json:"role"` // "user", "assistant", "system"
	Content string `json:"content"`
}

type WorkshopChatRequest struct {
	Role         string                `json:"role"` // scene_brainstorm, character_roleplay, editor_critique
	CharacterID  string                `json:"character_id,omitempty"`
	ChapterIndex int                   `json:"chapter_index,omitempty"`
	Messages     []WorkshopChatMessage `json:"messages"`
	Model        string                `json:"model,omitempty"`
}

type WorkshopChatResponse struct {
	Message    WorkshopChatMessage `json:"message"`
	TokensUsed engine.TokenUsage   `json:"tokens_used"`
}

func (s *Server) handleWorkshopChat(w http.ResponseWriter, r *http.Request, projectID string) {
	if r.Method != http.MethodPost {
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req WorkshopChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	if len(req.Messages) == 0 {
		errorResponse(w, http.StatusBadRequest, "messages cannot be empty")
		return
	}

	if s.llmRouter == nil {
		errorResponse(w, http.StatusInternalServerError, "llm client is not configured")
		return
	}

	ctx := r.Context()
	p, err := s.store.GetProject(ctx, projectID)
	if err != nil || p == nil {
		errorResponse(w, http.StatusNotFound, "project not found")
		return
	}

	var sysPrompt string
	switch req.Role {
	case "character_roleplay":
		if req.CharacterID != "" {
			entry, err := s.store.GetCodexEntry(ctx, projectID, req.CharacterID)
			if err == nil && entry != nil {
				prog := entry.ActiveProgression(req.ChapterIndex)
				rels, _ := s.store.ListCodexRelations(ctx, projectID, entry.ID)
				var relsStr strings.Builder
				for _, rel := range rels {
					relsStr.WriteString(fmt.Sprintf("- 对【%s】关系: %s (%s)\n", rel.TargetName, rel.RelationType, rel.Description))
				}
				sysPrompt = fmt.Sprintf(`你是小说《%s》中的角色【%s】（别名：%s）。
【角色简介】%s
【详细设定】%s
【当前章节阶段（第 %d 章）】%s
【当前人际关系】
%s
【角色指令】你必须严格代入该角色的第一人称口吻、思维方式、情绪立场和知识边界与创作者进行交谈。严禁跳戏（No breaking character）。`,
					p.Title, entry.Name, strings.Join(entry.Aliases, "、"),
					entry.Summary, entry.DetailsMarkdown,
					req.ChapterIndex, prog.Notes, relsStr.String())
			}
		}
		if sysPrompt == "" {
			sysPrompt = fmt.Sprintf(`你是小说《%s》中的核心角色。请根据上下文保持角色口吻与世界观设定进行对话。`, p.Title)
		}
	case "scene_brainstorm":
		sysPrompt = fmt.Sprintf(`你是小说《%s》的资深剧情策划与头脑风暴助理。
世界观规则：%s
你的职责是协助作者攻克卡文瓶颈、设计反转桥段、推演动机与冲突阻碍。回复必须富有启发性、直接切入戏剧冲突核心。`, p.Title, p.WorldRules)
	default: // editor_critique
		sysPrompt = fmt.Sprintf(`你是小说《%s》的严厉资深责任总编。
请站在严密因果逻辑、读者心流体验与节奏把控的角度，针对创作者的剧情设想进行犀利点评，指出潜在逻辑漏洞、战力崩塌或情节套路，并提出建设性的重构建议。`, p.Title)
	}

	var conversation strings.Builder
	for _, m := range req.Messages {
		conversation.WriteString(fmt.Sprintf("%s: %s\n\n", strings.ToUpper(m.Role), m.Content))
	}

	targetModel := req.Model
	if targetModel == "" {
		targetModel = s.cfg.ReasoningModel
	}

	resText, usage, err := s.llmRouter.ChatCompletionForRole(ctx, engine.RoleReasoner, targetModel, sysPrompt, conversation.String(), 0.7)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, WorkshopChatResponse{
		Message: WorkshopChatMessage{
			Role:    "assistant",
			Content: strings.TrimSpace(resText),
		},
		TokensUsed: usage,
	})
}

func (s *Server) handleProjectHarmonize(w http.ResponseWriter, r *http.Request, projectID string) {
	if r.Method != http.MethodPost {
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		Content   string  `json:"content"`
		Intensity float64 `json:"intensity"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(req.Content) == "" {
		errorResponse(w, http.StatusBadRequest, "content cannot be empty")
		return
	}
	intensity := req.Intensity
	if intensity <= 0 {
		intensity = 0.6
	}

	harmonizer := s.workshop.Harmonizer()
	processed, report := harmonizer.FullProcess(req.Content, intensity)
	jsonResponse(w, http.StatusOK, map[string]any{
		"processed_content": processed,
		"report":            report,
	})
}

func (s *Server) handleSuggestHumanTouches(w http.ResponseWriter, r *http.Request, projectID string) {
	if r.Method != http.MethodPost {
		errorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	harmonizer := s.workshop.Harmonizer()
	suggestions := harmonizer.SuggestHumanTouches(req.Content)
	jsonResponse(w, http.StatusOK, map[string]any{
		"suggestions": suggestions,
	})
}
