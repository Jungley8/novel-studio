package domain_test

import (
	"strings"
	"testing"

	"github.com/Jungley8/novel-studio/internal/domain"
)

func TestValidateStoryMilestones(t *testing.T) {
	tests := []struct {
		name          string
		milestones    []domain.StoryMilestone
		totalChapters int
		wantErr       bool
		errSubstring  string
	}{
		{
			name: "合法集：各字段完整、有序且未越界",
			milestones: []domain.StoryMilestone{
				{Key: "OPENING_HOOK", Name: "开局钩子", TargetChapter: 1, Status: domain.MilestoneStatusPending},
				{Key: "INCITING_INCIDENT", Name: "触发事件", TargetChapter: 10, Status: domain.MilestoneStatusPending},
				{Key: "MIDPOINT", Name: "中点", TargetChapter: 50, Status: domain.MilestoneStatusDone},
				{Key: "CLIMAX", Name: "全书高潮", TargetChapter: 100, Status: ""},
			},
			totalChapters: 100,
			wantErr:       false,
		},
		{
			name:          "合法集：空切片直接通过",
			milestones:    []domain.StoryMilestone{},
			totalChapters: 100,
			wantErr:       false,
		},
		{
			name: "key 空：空字符串报错",
			milestones: []domain.StoryMilestone{
				{Key: "", Name: "中点", TargetChapter: 50, Status: domain.MilestoneStatusPending},
			},
			totalChapters: 100,
			wantErr:       true,
			errSubstring:  "key cannot be empty",
		},
		{
			name: "key 空：仅空白字符报错",
			milestones: []domain.StoryMilestone{
				{Key: "   ", Name: "中点", TargetChapter: 50, Status: domain.MilestoneStatusPending},
			},
			totalChapters: 100,
			wantErr:       true,
			errSubstring:  "key cannot be empty",
		},
		{
			name: "key 重复：相同 key 报错",
			milestones: []domain.StoryMilestone{
				{Key: "MIDPOINT", Name: "中点A", TargetChapter: 50, Status: domain.MilestoneStatusPending},
				{Key: "MIDPOINT", Name: "中点B", TargetChapter: 60, Status: domain.MilestoneStatusPending},
			},
			totalChapters: 100,
			wantErr:       true,
			errSubstring:  "duplicate key",
		},
		{
			name: "key 重复：带空白字符归一后相同 key 报错",
			milestones: []domain.StoryMilestone{
				{Key: "CLIMAX", Name: "高潮A", TargetChapter: 80, Status: domain.MilestoneStatusPending},
				{Key: " CLIMAX ", Name: "高潮B", TargetChapter: 90, Status: domain.MilestoneStatusPending},
			},
			totalChapters: 100,
			wantErr:       true,
			errSubstring:  "duplicate key",
		},
		{
			name: "target <= 0：target_chapter 为 0 报错",
			milestones: []domain.StoryMilestone{
				{Key: "OPENING", Name: "开局", TargetChapter: 0, Status: domain.MilestoneStatusPending},
			},
			totalChapters: 100,
			wantErr:       true,
			errSubstring:  "target_chapter must be >= 1",
		},
		{
			name: "target <= 0：target_chapter 为负数报错",
			milestones: []domain.StoryMilestone{
				{Key: "OPENING", Name: "开局", TargetChapter: -5, Status: domain.MilestoneStatusPending},
			},
			totalChapters: 100,
			wantErr:       true,
			errSubstring:  "target_chapter must be >= 1",
		},
		{
			name: "target > total (total > 0 时)：越界报错",
			milestones: []domain.StoryMilestone{
				{Key: "ENDING", Name: "大结局", TargetChapter: 121, Status: domain.MilestoneStatusPending},
			},
			totalChapters: 120,
			wantErr:       true,
			errSubstring:  "exceeds total_planned_chapters",
		},
		{
			name: "total = 0 时不校验上限：target 远大于 0 依然合法",
			milestones: []domain.StoryMilestone{
				{Key: "ENDING", Name: "大结局", TargetChapter: 500, Status: domain.MilestoneStatusPending},
			},
			totalChapters: 0,
			wantErr:       false,
		},
		{
			name: "Status 非法：非法枚举值报错",
			milestones: []domain.StoryMilestone{
				{Key: "MIDPOINT", Name: "中点", TargetChapter: 50, Status: "INVALID_STATUS"},
			},
			totalChapters: 100,
			wantErr:       true,
			errSubstring:  "invalid status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := domain.ValidateStoryMilestones(tt.milestones, tt.totalChapters)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateStoryMilestones() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errSubstring != "" {
				if !strings.Contains(err.Error(), tt.errSubstring) {
					t.Errorf("error %q does not contain expected substring %q", err.Error(), tt.errSubstring)
				}
			}
		})
	}
}
