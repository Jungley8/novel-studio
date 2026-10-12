package server

import (
	"strings"
	"testing"

	"github.com/Jungley8/novel-studio/internal/domain"
)

// 预览块必须逐字嵌入生产块：preview*Block 若被改回手写拼接（表头/行文与生产块
// 不一致），本测试变红。定位逻辑的唯一事实来源是 engine 包，server 侧禁止第二份实现。
func TestPreviewBlocksEmbedProductionBlocks(t *testing.T) {
	p := &domain.Project{Framework: &domain.ProjectFramework{
		Motifs: []string{"  血色月亮  ", "", "雾中低语"},
		StoryMilestones: []domain.StoryMilestone{
			{Key: "OPENING_HOOK", Name: "  开局钩子  ", TargetChapter: 1, Status: domain.MilestoneStatusDone},
			{Key: "MIDPOINT", Name: "中点", TargetChapter: 15, Status: domain.MilestoneStatusPending},
			{Key: "FINALE", Name: "", TargetChapter: 30, Status: domain.MilestoneStatusPending},
		},
		TotalPlannedChapters: 30,
	}}

	motifBlock := previewMotifBlock(p)
	if !strings.Contains(motifBlock, "【有意母题与符号（RC 精修锚点）】") {
		t.Errorf("motif preview lost production header, got:\n%s", motifBlock)
	}
	if !strings.Contains(motifBlock, "- 血色月亮") || !strings.Contains(motifBlock, "- 雾中低语") {
		t.Errorf("motif preview lost production entries, got:\n%s", motifBlock)
	}

	msBlock := previewMilestoneBlock(p, 10)
	if !strings.Contains(msBlock, "【全书宏观节拍定位】") {
		t.Errorf("milestone preview lost production header, got:\n%s", msBlock)
	}
	if !strings.Contains(msBlock, "『中点』@第 15 章") {
		t.Errorf("milestone preview lost production next-milestone line, got:\n%s", msBlock)
	}
	if !strings.Contains(msBlock, "【全部里程碑一览（预览补充）】") {
		t.Errorf("milestone preview lost appendix list, got:\n%s", msBlock)
	}

	if got := previewMotifBlock(nil); got != "" {
		t.Errorf("nil project motif preview should be empty, got %q", got)
	}
	if got := previewMilestoneBlock(nil, 1); got != "" {
		t.Errorf("nil project milestone preview should be empty, got %q", got)
	}
}
