package engine

import (
	"strings"
	"testing"

	"github.com/Jungley8/novel-studio/internal/domain"
)

func TestBackfillStoryMilestones(t *testing.T) {
	// 乱序排序、target<=0 丢弃、dup key 保留首个、Name 补全、Status 归一
	input := []domain.StoryMilestone{
		{Key: "CLIMAX", Name: "", TargetChapter: 90, Status: ""},
		{Key: "INVALID_TARGET_1", Name: "负数章", TargetChapter: -1, Status: "PENDING"},
		{Key: "INVALID_TARGET_2", Name: "零章", TargetChapter: 0, Status: "PENDING"},
		{Key: "MIDPOINT", Name: "中点", TargetChapter: 50, Status: "DONE"},
		{Key: "MIDPOINT", Name: "中点重复", TargetChapter: 60, Status: "PENDING"}, // 排序后在后，应丢弃
		{Key: "OPENING_HOOK", Name: "", TargetChapter: 1, Status: "UNKNOWN"},  // 补名为开局钩子，Status 归一为 PENDING
		{Key: "CUSTOM_UNKNOWN", Name: "", TargetChapter: 20, Status: ""},      // 未知 key 保留 key 原文作为 Name
	}

	res := BackfillStoryMilestones(input)

	if len(res) != 4 {
		t.Fatalf("expected 4 normalized milestones, got %d", len(res))
	}

	// 1. target 升序检查
	expectedTargets := []int{1, 20, 50, 90}
	for i, expected := range expectedTargets {
		if res[i].TargetChapter != expected {
			t.Errorf("res[%d].TargetChapter = %d, want %d", i, res[i].TargetChapter, expected)
		}
	}

	// 2. target <= 0 丢弃检查
	for _, m := range res {
		if m.TargetChapter <= 0 {
			t.Errorf("unexpected milestone with target <= 0: %+v", m)
		}
	}

	// 3. dup key 保留首个检查
	if res[2].Key != "MIDPOINT" || res[2].TargetChapter != 50 || res[2].Name != "中点" {
		t.Errorf("expected MIDPOINT at chapter 50 preserved, got %+v", res[2])
	}

	// 4. Name 补全与未知 key
	if res[0].Key != "OPENING_HOOK" || res[0].Name != "开局钩子" {
		t.Errorf("res[0] expected OPENING_HOOK with Name=开局钩子, got %+v", res[0])
	}
	if res[1].Key != "CUSTOM_UNKNOWN" || res[1].Name != "CUSTOM_UNKNOWN" {
		t.Errorf("res[1] expected CUSTOM_UNKNOWN with Name=CUSTOM_UNKNOWN, got %+v", res[1])
	}

	// 5. Status 归一
	if res[0].Status != domain.MilestoneStatusPending {
		t.Errorf("res[0] status expected PENDING, got %q", res[0].Status)
	}
	if res[2].Status != domain.MilestoneStatusDone {
		t.Errorf("res[2] status expected DONE, got %q", res[2].Status)
	}
	if res[3].Status != domain.MilestoneStatusPending {
		t.Errorf("res[3] status expected PENDING, got %q", res[3].Status)
	}
}

func TestBuildMilestoneContextBlock(t *testing.T) {
	t.Run("无里程碑或无工程返回空串", func(t *testing.T) {
		if out := BuildMilestoneContextBlock(nil, 1); out != "" {
			t.Errorf("expected empty string for nil project, got %q", out)
		}
		pNoFw := &domain.Project{ID: "p1"}
		if out := BuildMilestoneContextBlock(pNoFw, 1); out != "" {
			t.Errorf("expected empty string for nil framework, got %q", out)
		}
		pEmptyMs := &domain.Project{
			ID: "p1",
			Framework: &domain.ProjectFramework{
				StoryMilestones: []domain.StoryMilestone{},
			},
		}
		if out := BuildMilestoneContextBlock(pEmptyMs, 1); out != "" {
			t.Errorf("expected empty string for empty milestones, got %q", out)
		}
	})

	milestones := []domain.StoryMilestone{
		{Key: "OPENING_HOOK", Name: "开局钩子", TargetChapter: 1, Status: domain.MilestoneStatusPending},
		{Key: "INCITING_INCIDENT", Name: "触发事件", TargetChapter: 10, Status: domain.MilestoneStatusPending},
		{Key: "MIDPOINT", Name: "中点", TargetChapter: 50, Status: domain.MilestoneStatusPending},
		{Key: "CLIMAX", Name: "全书高潮", TargetChapter: 90, Status: domain.MilestoneStatusPending},
	}

	t.Run("命中本章（TargetChapter==N 且文案含必须）", func(t *testing.T) {
		p := &domain.Project{
			ID: "p1",
			Framework: &domain.ProjectFramework{
				TotalPlannedChapters: 100,
				StoryMilestones:      milestones,
			},
		}
		out := BuildMilestoneContextBlock(p, 50)
		if !strings.Contains(out, "【全书宏观节拍定位】") {
			t.Errorf("expected header, got:\n%s", out)
		}
		if !strings.Contains(out, "全书计划：100 章（本章为第 50 章，约 50%）") {
			t.Errorf("expected plan percentage, got:\n%s", out)
		}
		if !strings.Contains(out, "本章里程碑：『中点』@第 50 章 —— 正文必须在本章兑现该里程碑事件与情绪高潮") {
			t.Errorf("expected current milestone with '必须', got:\n%s", out)
		}
		if !strings.Contains(out, "最近未兑现里程碑：『触发事件』@第 10 章（PENDING，已越过）") {
			t.Errorf("expected overdue milestone (max target < 50), got:\n%s", out)
		}
		if !strings.Contains(out, "下一里程碑：『全书高潮』@第 90 章（间隔 40 章）") {
			t.Errorf("expected next milestone, got:\n%s", out)
		}
	})

	t.Run("本章无里程碑锚点", func(t *testing.T) {
		p := &domain.Project{
			ID: "p1",
			Framework: &domain.ProjectFramework{
				TotalPlannedChapters: 100,
				StoryMilestones:      milestones,
			},
		}
		out := BuildMilestoneContextBlock(p, 25)
		if !strings.Contains(out, "- 本章无里程碑锚点") {
			t.Errorf("expected '- 本章无里程碑锚点', got:\n%s", out)
		}
		if !strings.Contains(out, "最近未兑现里程碑：『触发事件』@第 10 章（PENDING，已越过）") {
			t.Errorf("expected recent pending milestone, got:\n%s", out)
		}
		if !strings.Contains(out, "下一里程碑：『中点』@第 50 章（间隔 25 章）") {
			t.Errorf("expected next milestone, got:\n%s", out)
		}
	})

	t.Run("total=0 时百分比段省略", func(t *testing.T) {
		p := &domain.Project{
			ID: "p1",
			Framework: &domain.ProjectFramework{
				TotalPlannedChapters: 0,
				StoryMilestones:      milestones,
			},
		}
		out := BuildMilestoneContextBlock(p, 10)
		if strings.Contains(out, "%") {
			t.Errorf("expected percentage omitted when total=0, got:\n%s", out)
		}
		if !strings.Contains(out, "全书计划：未定（本章为第 10 章）") {
			t.Errorf("expected undetermined plan text, got:\n%s", out)
		}
		if !strings.Contains(out, "本章里程碑：『触发事件』@第 10 章 —— 正文必须在本章兑现该里程碑事件与情绪高潮") {
			t.Errorf("expected matched milestone, got:\n%s", out)
		}
	})
}

func TestBuildStoryPremiseBlock(t *testing.T) {
	t.Run("空内容返回空串", func(t *testing.T) {
		if out := buildStoryPremiseBlock(nil); out != "" {
			t.Errorf("expected empty string for nil project, got %q", out)
		}
		pNoFw := &domain.Project{ID: "p1"}
		if out := buildStoryPremiseBlock(pNoFw); out != "" {
			t.Errorf("expected empty string for nil framework, got %q", out)
		}
		pEmpty := &domain.Project{
			ID: "p1",
			Framework: &domain.ProjectFramework{
				ThemePremise: "   ",
				Logline:      "",
			},
		}
		if out := buildStoryPremiseBlock(pEmpty); out != "" {
			t.Errorf("expected empty string for empty premise and logline, got %q", out)
		}
	})

	t.Run("有内容（立意与梗概皆有）", func(t *testing.T) {
		p := &domain.Project{
			ID: "p1",
			Framework: &domain.ProjectFramework{
				ThemePremise: "凡人弑神",
				Logline:      "边荒少年隐忍破局",
			},
		}
		expected := "【全书立意与梗概】\n立意：凡人弑神\n梗概：边荒少年隐忍破局"
		if out := buildStoryPremiseBlock(p); out != expected {
			t.Errorf("expected %q, got %q", expected, out)
		}
	})

	t.Run("仅有立意", func(t *testing.T) {
		p := &domain.Project{
			ID: "p1",
			Framework: &domain.ProjectFramework{
				ThemePremise: "凡人弑神",
			},
		}
		expected := "【全书立意与梗概】\n立意：凡人弑神"
		if out := buildStoryPremiseBlock(p); out != expected {
			t.Errorf("expected %q, got %q", expected, out)
		}
	})

	t.Run("仅有梗概", func(t *testing.T) {
		p := &domain.Project{
			ID: "p1",
			Framework: &domain.ProjectFramework{
				Logline: "边荒少年隐忍破局",
			},
		}
		expected := "【全书立意与梗概】\n梗概：边荒少年隐忍破局"
		if out := buildStoryPremiseBlock(p); out != expected {
			t.Errorf("expected %q, got %q", expected, out)
		}
	})
}

func TestBuildAtmosphereAnchorPrompt(t *testing.T) {
	t.Run("空内容返回空串", func(t *testing.T) {
		if out := buildAtmosphereAnchorPrompt(nil); out != "" {
			t.Errorf("expected empty string for nil project, got %q", out)
		}
		pNoFw := &domain.Project{ID: "p1"}
		if out := buildAtmosphereAnchorPrompt(pNoFw); out != "" {
			t.Errorf("expected empty string for nil framework, got %q", out)
		}
		pEmpty := &domain.Project{
			ID: "p1",
			Framework: &domain.ProjectFramework{
				AtmosphereAnchor: "   ",
			},
		}
		if out := buildAtmosphereAnchorPrompt(pEmpty); out != "" {
			t.Errorf("expected empty string for empty anchor, got %q", out)
		}
	})

	t.Run("有内容输出规范指令", func(t *testing.T) {
		p := &domain.Project{
			ID: "p1",
			Framework: &domain.ProjectFramework{
				AtmosphereAnchor: "阴冷民俗废土",
			},
		}
		expected := "【视觉与氛围锚点（全书恒定基调，严禁漂移）】\n阴冷民俗废土\n所有场景的环境、光影、色彩与声音渲染必须贴合此基调，严禁氛围漂移。"
		if out := buildAtmosphereAnchorPrompt(p); out != expected {
			t.Errorf("expected %q, got %q", expected, out)
		}
	})
}
