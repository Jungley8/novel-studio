package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/Jungley8/novel-studio/internal/engine"
)

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
