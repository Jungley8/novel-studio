package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/Jungley8/novel-studio/internal/engine"
)

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
