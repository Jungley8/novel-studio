package domain_test

import (
	"testing"

	"github.com/Jungley8/novel-studio/internal/domain"
)

func TestProject_Validate(t *testing.T) {
	tests := []struct {
		name    string
		proj    domain.Project
		wantErr bool
	}{
		{
			name: "valid project",
			proj: domain.Project{
				Title:          "极道仙途",
				TargetPlatform: "番茄脑洞",
			},
			wantErr: false,
		},
		{
			name: "empty title fails",
			proj: domain.Project{
				Title: "   ",
			},
			wantErr: true,
		},
		{
			name: "empty target platform defaults",
			proj: domain.Project{
				Title:          "无名之书",
				TargetPlatform: "",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.proj.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && tt.proj.TargetPlatform == "" {
				t.Errorf("TargetPlatform should not be empty")
			}
		})
	}
}

func TestPlotHook_Validate(t *testing.T) {
	h := domain.PlotHook{
		Title:          "神秘信物",
		CreatedChapter: 3,
		TargetChapter:  2, // less than created
		Status:         "UNKNOWN",
	}

	if err := h.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if h.TargetChapter <= h.CreatedChapter {
		t.Errorf("expected TargetChapter > CreatedChapter, got %d <= %d", h.TargetChapter, h.CreatedChapter)
	}

	if h.Status != domain.HookStatusOpen {
		t.Errorf("expected status default to OPEN, got %s", h.Status)
	}

	emptyH := domain.PlotHook{Title: ""}
	if err := emptyH.Validate(); err == nil {
		t.Errorf("expected error for empty title")
	}
}
