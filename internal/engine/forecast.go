package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Jungley8/novel-studio/internal/domain"
)

// AssembleForecastHorizon wraps chronicle.AssembleHorizon as a read-only context synthesis seam.
// If AssembleHorizonTraced is merged in future budget branches, it can be seamlessly delegated here.
func AssembleForecastHorizon(ctx context.Context, chronicle *CanonChronicle, projectID string, targetChapter int) (*CanonHorizon, error) {
	if chronicle == nil {
		return nil, errors.New("chronicle is not initialized")
	}
	return chronicle.AssembleHorizon(ctx, projectID, targetChapter)
}

// CreateForecast generates multiple speculative narrative branches from the read-only canon horizon
// using ContextWithRole(ctx, RoleReasoner). It strictly requires branchCount (2-5) and horizon (3-5).
// Fail-Closed: If LLM output does not match branchCount exactly, it reports an error without truncating or padding.
func CreateForecast(
	ctx context.Context,
	client LLMClient,
	model string,
	horizonCtx *CanonHorizon,
	divergence string,
	branchCount int,
	horizon int,
) (*domain.Forecast, error) {
	if client == nil {
		return nil, errors.New("llm client cannot be nil")
	}
	if horizonCtx == nil {
		return nil, errors.New("chronicle horizon context cannot be nil")
	}
	divergence = strings.TrimSpace(divergence)
	if divergence == "" {
		return nil, errors.New("divergence cannot be empty")
	}
	if branchCount < 2 || branchCount > 5 {
		return nil, fmt.Errorf("branchCount must be between 2 and 5, got %d", branchCount)
	}
	if horizon < 3 || horizon > 5 {
		return nil, fmt.Errorf("horizon must be between 3 and 5, got %d", horizon)
	}

	projectTitle := "小说项目"
	projectID := ""
	if horizonCtx.Project != nil {
		projectID = horizonCtx.Project.ID
		if strings.TrimSpace(horizonCtx.Project.Title) != "" {
			projectTitle = strings.TrimSpace(horizonCtx.Project.Title)
		}
	}

	targetChapter := horizonCtx.TargetChapter
	if targetChapter <= 0 {
		targetChapter = 1
	}
	baseChapter := targetChapter - 1
	if baseChapter < 0 {
		baseChapter = 0
	}
	startChapter := targetChapter
	endChapter := targetChapter + horizon - 1

	protName := "主角"
	protRealm := "凡人"
	protInv := "无"
	if horizonCtx.ProtagonistEntry != nil {
		if horizonCtx.ProtagonistEntry.Name != "" {
			protName = horizonCtx.ProtagonistEntry.Name
		}
		if horizonCtx.ProtagonistEntry.Realm != "" {
			protRealm = horizonCtx.ProtagonistEntry.Realm
		}
		if horizonCtx.ProtagonistEntry.Inventory != "" {
			protInv = horizonCtx.ProtagonistEntry.Inventory
		}
	}

	canonSummary := horizonCtx.GlobalSummary
	if canonSummary == "" {
		canonSummary = horizonCtx.RollingCanonText
	}
	if canonSummary == "" {
		canonSummary = "暂无正史章节，开篇前瞻推演"
	}

	var hooksSummary string
	if len(horizonCtx.AllActiveHooks) > 0 {
		var hl []string
		for _, h := range horizonCtx.AllActiveHooks {
			hl = append(hl, fmt.Sprintf("- [%s] %s: %s (预计回收: 第%d章)", h.Status, h.Title, h.Details, h.TargetChapter))
		}
		hooksSummary = strings.Join(hl, "\n")
	} else {
		hooksSummary = "暂无开放伏笔"
	}

	systemPrompt := fmt.Sprintf(`你是一名顶级网络小说剧情推演架构师与因果推演专家。
你的任务是基于正史基线与作者提出的剧情分歧点（Divergence），严密推演出恰好 %d 个各具看点、因果严密的未来剧情候选分支。

推演核心铁律：
1. 分支数量严格性：必须且仅输出恰好 %d 个分支！多于或少于此数量均为严重不合格！
2. 跨度覆盖性：每个分支必须覆盖未来 %d 章（第 %d 章 ~ 第 %d 章）的发展脉络。
3. 候选隔离性：分支是探索候选，不得直接视为既定正史。
4. 每个分支必须包含以下字段：
   - id: 分支标识（如 branch_1, branch_2...）
   - beats_summary: 覆盖未来 %d 章的核心节拍与戏剧冲突概述
   - decisions: 主角与核心配角面对分歧时的关键抉择
   - expected_deltas: 该分支预期带来的战力突破、法宝得失或人物关系质变
   - risks: 此选择潜藏的暗流危机、因果代价或反噬风险
   - intent_match: 与作者分歧意图的契合度及看点评估
5. 必须且仅输出标准 JSON 格式，严禁带有 Markdown 解释或前后赘述：
{
  "branches": [
    {
      "id": "branch_1",
      "beats_summary": "未来节拍概述...",
      "decisions": "人物关键抉择...",
      "expected_deltas": "预期变迁...",
      "risks": "潜在风险...",
      "intent_match": "意图匹配度..."
    }
  ]
}`, branchCount, branchCount, horizon, startChapter, endChapter, horizon)

	userPrompt := fmt.Sprintf(`【项目信息】《%s》
【当前基准章节】第 %d 章
【推演目标跨度】未来 %d 章（第 %d 章 ~ 第 %d 章）
【主角当前基线】姓名: %s | 境界: %s | 随身道具: %s
【近期正史脉络】
%s
【活跃因果伏笔】
%s
【剧情分歧点 / 意图】
%s

请严格按照上述要求，输出恰好 %d 个剧情推演分支的 JSON。`,
		projectTitle, baseChapter, horizon, startChapter, endChapter,
		protName, protRealm, protInv, canonSummary, hooksSummary, divergence, branchCount)

	ctxRole := ContextWithRole(ctx, RoleReasoner)
	resp, err := client.ChatCompletion(ctxRole, model, systemPrompt, userPrompt, 0.7)
	if err != nil {
		return nil, fmt.Errorf("llm chat completion failed: %w", err)
	}

	cleanJSON, err := ExtractAndCleanJSON(resp)
	if err != nil {
		return nil, fmt.Errorf("extract and clean json failed: %w", err)
	}

	var payload struct {
		Branches []domain.ForecastBranch `json:"branches"`
	}
	if err := json.Unmarshal([]byte(cleanJSON), &payload); err != nil {
		return nil, fmt.Errorf("unmarshal forecast branches failed: %w", err)
	}

	// Strictly verify branch count (no automatic truncation or padding)
	if len(payload.Branches) != branchCount {
		return nil, fmt.Errorf("forecast branch count mismatch: expected exactly %d, got %d", branchCount, len(payload.Branches))
	}

	for i := range payload.Branches {
		if strings.TrimSpace(payload.Branches[i].ID) == "" {
			payload.Branches[i].ID = fmt.Sprintf("branch_%d", i+1)
		}
	}

	fc := &domain.Forecast{
		ID:          fmt.Sprintf("fc_%d", time.Now().UnixNano()),
		ProjectID:   projectID,
		BaseChapter: strconv.Itoa(baseChapter),
		Divergence:  divergence,
		BranchCount: branchCount,
		Horizon:     horizon,
		Status:      domain.ForecastStatusActive,
		Branches:    payload.Branches,
		CreatedAt:   time.Now(),
	}

	return fc, nil
}
