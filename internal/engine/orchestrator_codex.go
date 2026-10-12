package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Jungley8/novel-studio/internal/domain"
)

// CodexGenerateRequest encapsulates inputs for generating or expanding a codex entry.
type CodexGenerateRequest struct {
	Name     string               `json:"name"`
	Category domain.CodexCategory `json:"category"`
	Prompt   string               `json:"prompt"`
}

// GenerateCodexEntry prompts the reasoning engine to draft a rich, worldbuilding-consistent Codex entry.
func (o *Orchestrator) GenerateCodexEntry(
	ctx context.Context,
	reasoningModel string,
	project *domain.Project,
	req CodexGenerateRequest,
) (*domain.CodexEntry, TokenUsage, error) {
	category := req.Category
	if category == "" {
		category = domain.CategoryCharacter
	}

	systemPrompt := `你是一名网络小说白金级百科世界观架构师。
你的任务是根据世界观法则与作者灵感，为小说设定集构筑一个立体、严密、极具戏剧张力与辨识度的实体档案。
实体类别包括：CHARACTER(人物), LOCATION(地点), ITEM(法宝/道具), LORE(功法/规则), FACTION(门派/势力)。

必须以纯 JSON 格式输出，字段如下：
{
  "name": "实体名称",
  "aliases": ["别名或外号1", "外号2"],
  "category": "CHARACTER | LOCATION | ITEM | LORE | FACTION",
  "summary": "一句话核心定义（30字以内）",
  "details_markdown": "详细生平/渊源/物理表征/能力弱点的Markdown文本",
  "tracking_mode": "AUTO_MENTION",
  "archetype": "PROTAGONIST | ANTAGONIST | DEUTERAGONIST | MENTOR | SUPPORTING（仅人物有效，其他留空）",
  "voice_tone": "台词声口与语言特征（如：冷峻寡言，习惯以反问施压）",
  "want": "内在欲望：角色自以为追求的外在目标（可能被执念扭曲；反派必须由自我正义逻辑支撑其 want）",
  "need": "真实需求：角色真正需要面对与成长的内心课题（常与 Want 冲突）",
  "ghost": "核心鬼魂：过去创伤/执念，驱动其恐惧与扭曲行为",
  "current_disposition": "HOSTILE | WARY | NEUTRAL | FRIENDLY | DEVOTED（当前对主角立场）"
}`

	worldInfo := "无特定规则"
	if project != nil {
		worldInfo = fmt.Sprintf("书名：《%s》\n世界公理：%s", project.Title, project.WorldRules)
		if project.Framework != nil && project.Framework.ThemePremise != "" {
			worldInfo += fmt.Sprintf("\n全书主旨：%s", project.Framework.ThemePremise)
		}
	}

	userPrompt := fmt.Sprintf(`【设定背景】
%s

【生成需求】
目标分类：%s
实体名称：%s
设定灵感/要求：%s

请输出该实体的标准纯 JSON 档案。`,
		worldInfo, category, req.Name, req.Prompt,
	)

	ctxRole := ContextWithRole(ctx, RoleReasoner)
	resp, usage, err := o.client.ChatCompletionWithUsage(ctxRole, reasoningModel, systemPrompt, userPrompt, 0.7)
	if err != nil {
		return nil, usage, fmt.Errorf("generate codex entry failed: %w", err)
	}

	cleanJSON, err := ExtractAndCleanJSON(resp)
	if err != nil {
		return nil, usage, fmt.Errorf("clean codex json failed: %w", err)
	}

	var entry domain.CodexEntry
	if err := json.Unmarshal([]byte(cleanJSON), &entry); err != nil {
		return nil, usage, fmt.Errorf("unmarshal codex entry failed: %w", err)
	}

	if project != nil {
		entry.ProjectID = project.ID
	}
	if entry.Category == "" {
		entry.Category = category
	}
	if entry.ID == "" {
		entry.ID = fmt.Sprintf("codex-%d", time.Now().UnixNano())
	}
	if entry.TrackingMode == "" {
		entry.TrackingMode = domain.TrackingModeAutoMention
	}
	entry.CreatedAt = time.Now()
	entry.UpdatedAt = time.Now()

	return &entry, usage, nil
}

// ExtractCodexRelations prompts the reasoning engine to deduce relationships between entities.
func (o *Orchestrator) ExtractCodexRelations(
	ctx context.Context,
	reasoningModel string,
	project *domain.Project,
	entries []*domain.CodexEntry,
	recentContext string,
) ([]domain.EntityRelation, TokenUsage, error) {
	if len(entries) < 2 {
		return nil, TokenUsage{}, errors.New("need at least 2 codex entries to infer relationships")
	}

	var entNames []string
	nameToID := make(map[string]string)
	for _, e := range entries {
		entNames = append(entNames, fmt.Sprintf("- %s (分类: %s, 概述: %s)", e.Name, e.Category, e.Summary))
		nameToID[e.Name] = e.ID
		for _, a := range e.Aliases {
			nameToID[a] = e.ID
		}
	}

	systemPrompt := `你是一名网络小说复杂关系网络分析专家。
你的任务是根据已有实体档案及剧情上下文，提炼实体之间的单向或双向关联关系。
关系类型包含但不限于：
NEMESIS(宿敌死仇), ALLY(生死同盟), MASTER_DISCIPLE(师徒教导), KINSHIP(血脉亲缘), CRUSH(爱慕道侣),
BELONGS_TO(宗门从属), POSSESSES(本命持有), LOCATED_IN(驻扎身处), OPPOSES(暗中对抗)。

必须且仅输出 JSON 数组，格式如下：
[
  {
    "source_name": "实体A名称",
    "target_name": "实体B名称",
    "relation_type": "NEMESIS | ALLY | ...",
    "description": "简要关系描述（20字以内）"
  }
]`

	userPrompt := fmt.Sprintf(`【实体池】
%s

【剧情参考上下文】
%s

请推演实体间切实存在的关联网络，并输出关系数组 JSON。`,
		strings.Join(entNames, "\n"), recentContext,
	)

	ctxRole := ContextWithRole(ctx, RoleReasoner)
	resp, usage, err := o.client.ChatCompletionWithUsage(ctxRole, reasoningModel, systemPrompt, userPrompt, 0.4)
	if err != nil {
		return nil, usage, fmt.Errorf("extract codex relations failed: %w", err)
	}

	cleanJSON, err := ExtractAndCleanJSON(resp)
	if err != nil {
		return nil, usage, fmt.Errorf("clean relations json failed: %w", err)
	}

	type rawRel struct {
		SourceName   string `json:"source_name"`
		TargetName   string `json:"target_name"`
		RelationType string `json:"relation_type"`
		Description  string `json:"description"`
	}

	rawList, err := UnmarshalJSONArray[rawRel](cleanJSON)
	if err != nil {
		return nil, usage, fmt.Errorf("unmarshal relations json failed: %w", err)
	}

	var rels []domain.EntityRelation
	projectID := ""
	if project != nil {
		projectID = project.ID
	}

	resolveID := func(name string) string {
		name = strings.TrimSpace(name)
		if name == "" {
			return ""
		}
		if id, ok := nameToID[name]; ok {
			return id
		}
		for _, e := range entries {
			if strings.Contains(e.Name, name) || strings.Contains(name, e.Name) {
				return e.ID
			}
			for _, a := range e.Aliases {
				if strings.Contains(a, name) || strings.Contains(name, a) {
					return e.ID
				}
			}
		}
		return ""
	}

	for i, r := range rawList {
		srcID := resolveID(r.SourceName)
		tgtID := resolveID(r.TargetName)
		if srcID == "" || tgtID == "" || srcID == tgtID {
			continue
		}
		rels = append(rels, domain.EntityRelation{
			ID:            fmt.Sprintf("rel-%d-%d", time.Now().UnixNano(), i+1),
			ProjectID:     projectID,
			SourceEntryID: srcID,
			TargetEntryID: tgtID,
			TargetName:    r.TargetName,
			RelationType:  r.RelationType,
			Description:   r.Description,
			CreatedAt:     time.Now(),
		})
	}

	return rels, usage, nil
}

func protNameLevel(e *domain.CodexEntry) string {
	if e == nil {
		return "未知"
	}
	return e.NameAndLevel()
}

func protInventory(e *domain.CodexEntry) string {
	if e == nil || strings.TrimSpace(e.Inventory) == "" {
		return "无"
	}
	return e.Inventory
}

func protGoal(e *domain.CodexEntry) string {
	if e == nil {
		return "暂无"
	}
	if strings.TrimSpace(e.Want) != "" {
		return e.Want
	}
	if strings.TrimSpace(e.Summary) != "" {
		return e.Summary
	}
	return "暂无"
}

func protHealth(e *domain.CodexEntry) string {
	if e == nil || strings.TrimSpace(e.HealthStatus) == "" {
		return "正常"
	}
	return e.HealthStatus
}
