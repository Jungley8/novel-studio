package domain

import (
	"fmt"
	"strings"
)

// StoryMilestone 全书宏观节拍定位（三幕/救猫咪语义的书级里程碑）。
type StoryMilestone struct {
	Key           string `json:"key"`            // 稳定标识（建议值见 MilestoneKeyNames，允许自定义但需非空唯一）
	Name          string `json:"name"`           // 展示名（如「中点」「灵魂黑夜」）
	TargetChapter int    `json:"target_chapter"` // 目标章节（1-based）
	Status        string `json:"status,omitempty"`
	Note          string `json:"note,omitempty"`
}

const (
	MilestoneStatusPending = "PENDING"
	MilestoneStatusDone    = "DONE"
)

// MilestoneKeyNames 建议枚举与中文名（创世归一化用于补名）。
var MilestoneKeyNames = map[string]string{
	"OPENING_HOOK":      "开局钩子",
	"INCITING_INCIDENT": "触发事件",
	"ACT1_TURN":         "第一幕转折",
	"MIDPOINT":          "中点",
	"DARK_NIGHT":        "灵魂黑夜",
	"CLIMAX":            "全书高潮",
	"ENDING":            "结局",
}

// ValidateStoryMilestones 严格校验（API 写入 fail-closed）：
// key trim 非空且唯一；target_chapter >= 1；Status ∈ {PENDING, DONE, ""}；
// totalChapters > 0 时 target_chapter <= totalChapters。
func ValidateStoryMilestones(ms []StoryMilestone, totalChapters int) error {
	seenKeys := make(map[string]struct{}, len(ms))
	for i, m := range ms {
		trimmedKey := strings.TrimSpace(m.Key)
		if trimmedKey == "" {
			return fmt.Errorf("story milestone [%d] key cannot be empty", i)
		}
		if _, exists := seenKeys[trimmedKey]; exists {
			return fmt.Errorf("story milestone [%d] duplicate key: %q", i, trimmedKey)
		}
		seenKeys[trimmedKey] = struct{}{}

		if m.TargetChapter < 1 {
			return fmt.Errorf("story milestone [%d] target_chapter must be >= 1, got %d", i, m.TargetChapter)
		}
		if totalChapters > 0 && m.TargetChapter > totalChapters {
			return fmt.Errorf("story milestone [%d] target_chapter %d exceeds total_planned_chapters %d", i, m.TargetChapter, totalChapters)
		}
		switch m.Status {
		case MilestoneStatusPending, MilestoneStatusDone, "":
		default:
			return fmt.Errorf("story milestone [%d] invalid status: %q", i, m.Status)
		}
	}
	return nil
}
