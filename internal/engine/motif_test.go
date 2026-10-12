package engine

import (
	"strings"
	"testing"

	"github.com/Jungley8/novel-studio/internal/domain"
)

func TestBuildMotifConceptPrompt(t *testing.T) {
	// 1. nil / 空 framework / 空母题 → ""
	if got := BuildMotifConceptPrompt(nil); got != "" {
		t.Fatalf("nil project should produce empty block, got %q", got)
	}
	if got := BuildMotifConceptPrompt(&domain.Project{}); got != "" {
		t.Fatalf("nil framework should produce empty block, got %q", got)
	}
	if got := BuildMotifConceptPrompt(&domain.Project{Framework: &domain.ProjectFramework{}}); got != "" {
		t.Fatalf("no motifs should produce empty block, got %q", got)
	}

	// 2. 仅空白条目 → ""
	blank := &domain.Project{Framework: &domain.ProjectFramework{Motifs: []string{"   ", ""}}}
	if got := BuildMotifConceptPrompt(blank); got != "" {
		t.Fatalf("blank-only motifs should produce empty block, got %q", got)
	}

	// 3. 有母题 → 头部 + 逐条 + 规则约束
	project := &domain.Project{Framework: &domain.ProjectFramework{Motifs: []string{"冰冷光效", " 机械蜂鸣 "}}}
	got := BuildMotifConceptPrompt(project)
	if !strings.Contains(got, "【有意母题与符号（RC 精修锚点）】") {
		t.Fatalf("missing header: %q", got)
	}
	if !strings.Contains(got, "- 冰冷光效") {
		t.Fatalf("missing first motif line: %q", got)
	}
	if !strings.Contains(got, "- 机械蜂鸣") || strings.Contains(got, "-  机械蜂鸣") {
		t.Fatalf("second motif must be trimmed into bullet form: %q", got)
	}
	if !strings.Contains(got, "严禁机械复读") {
		t.Fatalf("missing anti-mechanical-repetition rule: %q", got)
	}
}
