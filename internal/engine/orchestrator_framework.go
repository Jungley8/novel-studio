package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Jungley8/novel-studio/internal/domain"
)

// FrameworkBootstrapRequest encapsulates inputs required to deduce a comprehensive macro framework.
type FrameworkBootstrapRequest struct {
	Title          string `json:"title"`
	TargetPlatform string `json:"target_platform"`
	CoreConcept    string `json:"core_concept"`
}

// BootstrapFramework prompts the reasoning engine to deduce the entire world bible,
// power ladder, factions, dramatis personae, volume arcs, and seed plot hooks.
func (o *Orchestrator) BootstrapFramework(
	ctx context.Context,
	reasoningModel string,
	req FrameworkBootstrapRequest,
) (*domain.ProjectFramework, TokenUsage, error) {
	if strings.TrimSpace(req.Title) == "" {
		return nil, TokenUsage{}, errors.New("title cannot be empty")
	}
	platform := req.TargetPlatform
	if strings.TrimSpace(platform) == "" {
		platform = "通用网文"
	}

	systemPrompt := `你是一名网络小说白金级架构总策划兼世界观架构大师。
你的任务是根据作者提供的书名、目标平台与核心灵感，推演并构建出一套宏大、自洽、严密且极具商业与文学张力的全书顶层架构总纲（Project Framework）。

推演法则：
1. 核心立意与世界规则 (World Axioms)：提炼 3-5 条底层不可逆的世界运转公理与核心铁律（依据小说题材自适应：修仙之天道反噬/科幻之物理公理/悬疑之因果闭环/奇幻之魔法代价/都市之社会法则）。
2. 战力阶梯 (Power Ladder)：设计 6-9 个严密境界或实力层级，包含阶位序号(tier: 1..N)、破坏力表征、突破门槛与代价副作用（拒绝廉价数值堆砌）。
3. 核心势力 (Factions)：设计 3-4 个主要势力宗门或组织财阀，明确其立场、核心主张、独门手段与威胁级别。
4. 关键人物谱系 (Key Characters)：设计 3-5 位与主角命运交织的关键角色（引路导师、宿敌死仇、亦正亦邪同盟、远古残魂）。
5. 分卷宏观大纲 (Volume Arcs)：设计前 3-4 卷的大纲，每卷包含：卷序号(volume_index: 1..N)、卷名、卷主题、核心主线目标、终极大高潮情节、显式起止章(start_chapter, end_chapter)、状态(status: "DRAFT")、预估章数与核心回收伏笔。
6. 开局种子伏笔 (Seed Hooks)：设计 3-5 个开局前 3 章埋下的长线伏笔（包含目标回收章节 10-60 章）。

输出格式：必须且仅输出标准合法的纯 JSON 格式：
{
  "theme_premise": "核心主旨一句话描述",
  "world_axioms": ["世界规则1", "核心公理2", "底层铁律3"],
  "power_ladder": [
    {
      "tier": 1,
      "realm": "境界名或能力阶段名",
      "description": "破坏力与能力表征",
      "bottleneck": "突破门槛与关卡",
      "drawback": "突破代价或能力副作用"
    }
  ],
  "factions": [
    {
      "name": "势力名称",
      "alignment": "阵营立场",
      "doctrine": "核心功法主张与手段",
      "threat_level": "威胁级别：中等/极高/灭顶之灾"
    }
  ],
  "key_characters": [
    {
      "name": "姓名",
      "role": "领路人/宿敌/同盟",
      "realm": "初始境界",
      "goal": "核心动机",
      "fate_arc": "宿命悲剧或终局"
    }
  ],
  "volume_arcs": [
    {
      "volume_index": 1,
      "title": "卷名",
      "theme": "卷主题",
      "core_goal": "本卷必须达成的核心目标",
      "climax": "本卷终极大高潮情节",
      "start_chapter": 1,
      "end_chapter": 40,
      "status": "DRAFT",
      "estimated_chapters": 40,
      "key_payoffs": ["本卷回收的伏笔1"]
    }
  ],
  "seed_hooks": [
    {
      "title": "伏笔标题",
      "details": "伏笔具体细节与暗线线索",
      "created_chapter": 1,
      "target_chapter": 20,
      "status": "OPEN"
    }
  ]
}`

	userPrompt := fmt.Sprintf(`【创世输入】
书名：《%s》
目标平台：%s
核心脑洞与题材灵感：%s

请推演并输出全书宏观创世总纲 JSON。`,
		req.Title, platform, req.CoreConcept,
	)

	ctxRole := ContextWithRole(ctx, RoleReasoner)
	resp, usage, err := o.client.ChatCompletionWithUsage(ctxRole, reasoningModel, systemPrompt, userPrompt, 0.5)
	if err != nil {
		return nil, usage, fmt.Errorf("bootstrap framework LLM call failed: %w", err)
	}

	cleanJSON, err := ExtractAndCleanJSON(resp)
	if err != nil {
		cleanJSON = extractJSON(resp)
	}

	var fw domain.ProjectFramework
	if err := json.Unmarshal([]byte(cleanJSON), &fw); err != nil {
		return nil, usage, fmt.Errorf("parse framework JSON failed (raw: %s): %w", resp, err)
	}

	for i := range fw.PowerLadder {
		if fw.PowerLadder[i].Tier == 0 {
			fw.PowerLadder[i].Tier = i + 1
		}
	}
	fw.VolumeArcs = BackfillVolumeArcs(fw.VolumeArcs)

	return &fw, usage, nil
}

// BackfillVolumeArcs normalizes VolumeIndex (defaults to i+1 if 0), Status (defaults to DRAFT),
// and backfills StartChapter/EndChapter when <= 0 using EstimatedChapters (<=0 defaults to 40).
// If explicit non-zero values are present, it validates StartChapter <= EndChapter;
// if invalid (StartChapter > EndChapter), it fails-closed by falling back to sequential derivation.
// The arithmetic is chapter-for-chapter identical to legacy deriveVolumeSpans accumulation.
func BackfillVolumeArcs(arcs []domain.VolumeArc) []domain.VolumeArc {
	if len(arcs) == 0 {
		return arcs
	}
	res := make([]domain.VolumeArc, len(arcs))
	copy(res, arcs)

	accChapters := 0
	for i := range res {
		arc := &res[i]
		if arc.VolumeIndex == 0 {
			arc.VolumeIndex = i + 1
		}
		if arc.Status == "" {
			arc.Status = domain.VolumeStatusDraft
		}

		est := arc.EstimatedChapters
		if est <= 0 {
			est = 40
		}

		if arc.StartChapter > 0 && arc.EndChapter > 0 && arc.StartChapter <= arc.EndChapter {
			accChapters = arc.EndChapter
		} else {
			arc.StartChapter = accChapters + 1
			arc.EndChapter = accChapters + est
			accChapters += est
		}
	}
	return res
}
