package engine

import (
	"context"
	"testing"

	"github.com/Jungley8/novel-studio/internal/domain"
)

func TestGenerateVolumeSections_RetriesBadSamples(t *testing.T) {
	valid := `{"sections":[{"title":"缝合所见","from_chapter":1,"to_chapter":7,"goal":"入局","key_events":["e1"]},{"title":"第七口棺","from_chapter":8,"to_chapter":14,"goal":"收束","key_events":["e2"]}]}`
	arc := domain.VolumeArc{VolumeIndex: 1, Title: "槐树胡同第七人", StartChapter: 1, EndChapter: 14, EstimatedChapters: 14}
	proj := &domain.Project{ID: "proj_x", Title: "槐树胡同第七人", TargetPlatform: "知乎盐言"}

	t.Run("recovers from an unrepairable sample", func(t *testing.T) {
		client := &mockSectionsLLMClient{}
		client.responseFn = func(systemPrompt, userPrompt string) (string, error) {
			if client.callCount == 1 {
				return "这次输出完全不是 JSON 结构", nil
			}
			return valid, nil
		}
		sections, _, err := GenerateVolumeSections(context.Background(), client, "muse", proj, arc)
		if err != nil {
			t.Fatalf("expected recovery via regeneration, got %v", err)
		}
		if len(sections) != 2 || sections[0].Title != "缝合所见" || sections[1].ToChapter != 14 {
			t.Fatalf("unexpected sections: %+v", sections)
		}
		if client.callCount != 2 {
			t.Fatalf("expected exactly 2 attempts, got %d", client.callCount)
		}
	})

	t.Run("recovers from a coverage-violating sample", func(t *testing.T) {
		client := &mockSectionsLLMClient{}
		client.responseFn = func(systemPrompt, userPrompt string) (string, error) {
			if client.callCount == 1 {
				return `{"sections":[{"title":"覆盖不足","from_chapter":1,"to_chapter":10,"goal":"g","key_events":["e"]}]}`, nil
			}
			return valid, nil
		}
		sections, _, err := GenerateVolumeSections(context.Background(), client, "muse", proj, arc)
		if err != nil {
			t.Fatalf("expected recovery via regeneration, got %v", err)
		}
		if len(sections) != 2 {
			t.Fatalf("unexpected sections: %+v", sections)
		}
		if client.callCount != 2 {
			t.Fatalf("expected exactly 2 attempts, got %d", client.callCount)
		}
	})

	t.Run("fails closed after exhausting attempts", func(t *testing.T) {
		client := &mockSectionsLLMClient{response: "永远不是 JSON"}
		_, _, err := GenerateVolumeSections(context.Background(), client, "muse", proj, arc)
		if err == nil {
			t.Fatalf("expected fail-closed error after retry budget is spent")
		}
		if client.callCount != volumeSectionsGenerationAttempts {
			t.Fatalf("expected %d attempts, got %d", volumeSectionsGenerationAttempts, client.callCount)
		}
	})
}
