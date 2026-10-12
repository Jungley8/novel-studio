package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
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
0. 主角归属铁律 (Protagonist)：若核心灵感中已点名具体人物（含姓名或明确身份定位），全书主角必须且只能是该人物（姓名、身份、性别严格一致），严禁改名换人、严禁另立并列主角；若灵感确未点名，方可自行设计。主角档案只能出现在 protagonist 字段中，key_characters 严禁重复收录主角。
1. 核心立意与世界规则 (World Axioms)：提炼 3-5 条底层不可逆的世界运转公理与核心铁律（依据小说题材自适应：修仙之天道反噬/科幻之物理公理/悬疑之因果闭环/奇幻之魔法代价/都市之社会法则）。
2. 战力阶梯 (Power Ladder)：设计 6-9 个严密境界或实力层级，包含阶位序号(tier: 1..N)、破坏力表征、突破门槛与代价副作用（拒绝廉价数值堆砌）。
3. 核心势力 (Factions)：设计 3-4 个主要势力宗门或组织财阀，明确其立场、核心主张、独门手段与威胁级别。
4. 关键人物谱系 (Key Characters)：设计 3-5 位与主角命运交织的关键角色（引路导师、宿敌死仇、亦正亦邪同盟、远古残魂），不含主角本人。
5. 分卷宏观大纲 (Volume Arcs)：设计前 3-4 卷的大纲，每卷包含：卷序号(volume_index: 1..N)、卷名、卷主题、核心主线目标、终极大高潮情节、显式起止章(start_chapter, end_chapter)、状态(status: "DRAFT")、预估章数与核心回收伏笔。
6. 开局种子伏笔 (Seed Hooks)：设计 3-5 个开局前 3 章埋下的长线伏笔（包含目标回收章节 10-60 章）。
7. 一句话梗概 (Logline)：一句话概括：主角+核心冲突+致命危机（60字内）。
8. 氛围锚点 (Atmosphere Anchor)：全书恒定的视觉/感官基调（如「高反差冷色调赛博空间」或「阴冷民俗废土」，30字内）。
9. 全书计划总章数 (Total Planned Chapters)：正整数，参考分卷预估之和。
10. 全书节拍里程碑 (Story Milestones)：3-7 条，key 用建议枚举（OPENING_HOOK/INCITING_INCIDENT/ACT1_TURN/MIDPOINT/DARK_NIGHT/CLIMAX/ENDING），target_chapter 落在总章数内且整体呈越过时间线递增，status 一律 "PENDING"，note 为该里程碑的事件要点。

输出格式：必须且仅输出标准合法的纯 JSON 格式：
{
  "theme_premise": "核心主旨一句话描述",
  "logline": "一句话概括：主角+核心冲突+致命危机（60字内）",
  "atmosphere_anchor": "全书恒定的视觉/感官基调（如「高反差冷色调赛博空间」或「阴冷民俗废土」，30字内）",
  "protagonist": {
    "name": "主角姓名（灵感已点名则必须沿用）",
    "identity": "职业/身份定位",
    "initial_realm": "初始境界或实力/职级阶段（题材自适应）",
    "want": "外在目标",
    "need": "成长课题",
    "ghost": "创伤执念",
    "voice_tone": "语言与行事风格",
    "inventory": ["初始随身物（题材自适应，可为空数组）"]
  },
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
      "want": "外在目标",
      "need": "成长课题",
      "ghost": "创伤执念",
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
  "total_planned_chapters": 120,
  "story_milestones": [
    {
      "key": "MIDPOINT",
      "name": "中点",
      "target_chapter": 60,
      "status": "PENDING",
      "note": "主角首次遭遇全盘溃败并洞悉世界真相"
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
	fw.StoryMilestones = BackfillStoryMilestones(fw.StoryMilestones)

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

// BackfillStoryMilestones 创世 LLM 输出的宽容归一化（纯函数）：
// 丢弃 target<=0；按 target 升序排序；key 重复保留排序后首个；
// Name 为空时用 MilestoneKeyNames 补中文名（未知 key 保留 key 原文）；Status 空补 PENDING（非法值归一为 PENDING）。
func BackfillStoryMilestones(ms []domain.StoryMilestone) []domain.StoryMilestone {
	if len(ms) == 0 {
		return ms
	}
	var valid []domain.StoryMilestone
	for _, m := range ms {
		if m.TargetChapter > 0 {
			valid = append(valid, m)
		}
	}
	if len(valid) == 0 {
		return valid
	}

	sort.SliceStable(valid, func(i, j int) bool {
		return valid[i].TargetChapter < valid[j].TargetChapter
	})

	seenKeys := make(map[string]bool)
	var deduped []domain.StoryMilestone
	for _, m := range valid {
		k := strings.TrimSpace(m.Key)
		if seenKeys[k] {
			continue
		}
		seenKeys[k] = true
		deduped = append(deduped, m)
	}

	for i := range deduped {
		m := &deduped[i]
		m.Key = strings.TrimSpace(m.Key)
		if strings.TrimSpace(m.Name) == "" {
			if cnName, ok := domain.MilestoneKeyNames[m.Key]; ok {
				m.Name = cnName
			} else {
				m.Name = m.Key
			}
		}
		switch m.Status {
		case domain.MilestoneStatusPending, domain.MilestoneStatusDone:
		default:
			m.Status = domain.MilestoneStatusPending
		}
	}
	return deduped
}
