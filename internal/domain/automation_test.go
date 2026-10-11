package domain_test

import (
	"strings"
	"testing"

	"github.com/Jungley8/novel-studio/internal/domain"
)

func TestValidateDailyTimes(t *testing.T) {
	tests := []struct {
		name    string
		times   []string
		wantErr bool
	}{
		{
			name:    "empty slice is valid",
			times:   []string{},
			wantErr: false,
		},
		{
			name:    "nil slice is valid",
			times:   nil,
			wantErr: false,
		},
		{
			name:    "single valid time",
			times:   []string{"09:00"},
			wantErr: false,
		},
		{
			name:    "boundary times 00:00 and 23:59",
			times:   []string{"00:00", "23:59"},
			wantErr: false,
		},
		{
			name:    "multiple sorted times",
			times:   []string{"02:00", "08:30", "14:15", "22:45"},
			wantErr: false,
		},
		{
			name:    "invalid length - 4 chars 9:00",
			times:   []string{"9:00"},
			wantErr: true,
		},
		{
			name:    "invalid length - 6 chars 09:000",
			times:   []string{"09:000"},
			wantErr: true,
		},
		{
			name:    "invalid delimiter",
			times:   []string{"09-00"},
			wantErr: true,
		},
		{
			name:    "hour out of range 24:00",
			times:   []string{"24:00"},
			wantErr: true,
		},
		{
			name:    "hour out of range 25:00",
			times:   []string{"25:00"},
			wantErr: true,
		},
		{
			name:    "minute out of range 12:60",
			times:   []string{"12:60"},
			wantErr: true,
		},
		{
			name:    "non digit hour",
			times:   []string{"ab:00"},
			wantErr: true,
		},
		{
			name:    "non digit minute",
			times:   []string{"12:cd"},
			wantErr: true,
		},
		{
			name: "too many entries > 48",
			times: func() []string {
				list := make([]string, 49)
				for i := range list {
					list[i] = "10:00"
				}
				return list
			}(),
			wantErr: true,
		},
		{
			name: "exact 48 entries is valid",
			times: func() []string {
				list := make([]string, 48)
				for i := range list {
					list[i] = "10:00"
				}
				return list
			}(),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := domain.ValidateDailyTimes(tt.times)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateDailyTimes(%v) error = %v, wantErr %v", tt.times, err, tt.wantErr)
			}
		})
	}
}

func TestValidateAutomation(t *testing.T) {
	tests := []struct {
		name    string
		auto    *domain.ProjectAutomation
		wantErr bool
	}{
		{
			name:    "nil automation",
			auto:    nil,
			wantErr: true,
		},
		{
			name: "valid disabled with empty times",
			auto: &domain.ProjectAutomation{
				ProjectID:  "proj-1",
				Enabled:    false,
				Times:      []string{},
				QuotaKind:  domain.QuotaKindChapters,
				QuotaCount: 5,
			},
			wantErr: false,
		},
		{
			name: "valid enabled with times",
			auto: &domain.ProjectAutomation{
				ProjectID:  "proj-1",
				Enabled:    true,
				Times:      []string{"09:00", "18:00"},
				QuotaKind:  domain.QuotaKindChapters,
				QuotaCount: 10,
			},
			wantErr: false,
		},
		{
			name: "invalid enabled with empty times",
			auto: &domain.ProjectAutomation{
				ProjectID:  "proj-1",
				Enabled:    true,
				Times:      []string{},
				QuotaKind:  domain.QuotaKindChapters,
				QuotaCount: 5,
			},
			wantErr: true,
		},
		{
			name: "invalid enabled with nil times",
			auto: &domain.ProjectAutomation{
				ProjectID:  "proj-1",
				Enabled:    true,
				Times:      nil,
				QuotaKind:  domain.QuotaKindChapters,
				QuotaCount: 5,
			},
			wantErr: true,
		},
		{
			name: "unknown quota kind",
			auto: &domain.ProjectAutomation{
				ProjectID:  "proj-1",
				Enabled:    false,
				QuotaKind:  "words",
				QuotaCount: 5,
			},
			wantErr: true,
		},
		{
			name: "chapters quota count zero",
			auto: &domain.ProjectAutomation{
				ProjectID:  "proj-1",
				QuotaKind:  domain.QuotaKindChapters,
				QuotaCount: 0,
			},
			wantErr: true,
		},
		{
			name: "chapters quota count 100 is valid",
			auto: &domain.ProjectAutomation{
				ProjectID:  "proj-1",
				QuotaKind:  domain.QuotaKindChapters,
				QuotaCount: 100,
			},
			wantErr: false,
		},
		{
			name: "chapters quota count 101 exceeds limit",
			auto: &domain.ProjectAutomation{
				ProjectID:  "proj-1",
				QuotaKind:  domain.QuotaKindChapters,
				QuotaCount: 101,
			},
			wantErr: true,
		},
		{
			name: "volumes quota count 1 is valid",
			auto: &domain.ProjectAutomation{
				ProjectID:  "proj-1",
				QuotaKind:  domain.QuotaKindVolumes,
				QuotaCount: 1,
			},
			wantErr: false,
		},
		{
			name: "volumes quota count 10 is valid",
			auto: &domain.ProjectAutomation{
				ProjectID:  "proj-1",
				QuotaKind:  domain.QuotaKindVolumes,
				QuotaCount: 10,
			},
			wantErr: false,
		},
		{
			name: "volumes quota count 0 is invalid",
			auto: &domain.ProjectAutomation{
				ProjectID:  "proj-1",
				QuotaKind:  domain.QuotaKindVolumes,
				QuotaCount: 0,
			},
			wantErr: true,
		},
		{
			name: "volumes quota count 11 exceeds limit",
			auto: &domain.ProjectAutomation{
				ProjectID:  "proj-1",
				QuotaKind:  domain.QuotaKindVolumes,
				QuotaCount: 11,
			},
			wantErr: true,
		},
		{
			name: "invalid daily times format",
			auto: &domain.ProjectAutomation{
				ProjectID:  "proj-1",
				Enabled:    true,
				Times:      []string{"25:00"},
				QuotaKind:  domain.QuotaKindChapters,
				QuotaCount: 5,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := domain.ValidateAutomation(tt.auto)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateAutomation() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateBookQueueItem(t *testing.T) {
	tests := []struct {
		name    string
		item    *domain.BookQueueItem
		wantErr bool
	}{
		{
			name:    "nil item",
			item:    nil,
			wantErr: true,
		},
		{
			name: "valid normal item",
			item: &domain.BookQueueItem{
				ID:       "queue-1",
				Title:    "剑开天门",
				Concept:  "传统玄幻，剑修逆天修行",
				Platform: "起点中文网",
				Volumes:  3,
				AutoLock: true,
			},
			wantErr: false,
		},
		{
			name: "empty title",
			item: &domain.BookQueueItem{
				Title:   "",
				Concept: "修仙故事",
				Volumes: 1,
			},
			wantErr: true,
		},
		{
			name: "whitespace title",
			item: &domain.BookQueueItem{
				Title:   "   \t\n  ",
				Concept: "修仙故事",
				Volumes: 1,
			},
			wantErr: true,
		},
		{
			name: "title exceeds 200 runes",
			item: &domain.BookQueueItem{
				Title:   strings.Repeat("字", 201),
				Concept: "修仙故事",
				Volumes: 1,
			},
			wantErr: true,
		},
		{
			name: "title exact 200 runes is valid",
			item: &domain.BookQueueItem{
				Title:   strings.Repeat("字", 200),
				Concept: "修仙故事",
				Volumes: 1,
			},
			wantErr: false,
		},
		{
			name: "empty concept",
			item: &domain.BookQueueItem{
				Title:   "剑开天门",
				Concept: "",
				Volumes: 1,
			},
			wantErr: true,
		},
		{
			name: "whitespace concept",
			item: &domain.BookQueueItem{
				Title:   "剑开天门",
				Concept: "   \n\t  ",
				Volumes: 1,
			},
			wantErr: true,
		},
		{
			name: "concept exceeds 4000 runes",
			item: &domain.BookQueueItem{
				Title:   "剑开天门",
				Concept: strings.Repeat("界", 4001),
				Volumes: 1,
			},
			wantErr: true,
		},
		{
			name: "concept exact 4000 runes is valid",
			item: &domain.BookQueueItem{
				Title:   "剑开天门",
				Concept: strings.Repeat("界", 4000),
				Volumes: 1,
			},
			wantErr: false,
		},
		{
			name: "platform empty is valid",
			item: &domain.BookQueueItem{
				Title:    "剑开天门",
				Concept:  "修仙故事",
				Platform: "",
				Volumes:  1,
			},
			wantErr: false,
		},
		{
			name: "platform exceeds 100 runes",
			item: &domain.BookQueueItem{
				Title:    "剑开天门",
				Concept:  "修仙故事",
				Platform: strings.Repeat("台", 101),
				Volumes:  1,
			},
			wantErr: true,
		},
		{
			name: "volumes 0 is invalid",
			item: &domain.BookQueueItem{
				Title:   "剑开天门",
				Concept: "修仙故事",
				Volumes: 0,
			},
			wantErr: true,
		},
		{
			name: "volumes 1 is valid",
			item: &domain.BookQueueItem{
				Title:   "剑开天门",
				Concept: "修仙故事",
				Volumes: 1,
			},
			wantErr: false,
		},
		{
			name: "volumes 20 is valid",
			item: &domain.BookQueueItem{
				Title:   "剑开天门",
				Concept: "修仙故事",
				Volumes: 20,
			},
			wantErr: false,
		},
		{
			name: "volumes 21 is invalid",
			item: &domain.BookQueueItem{
				Title:   "剑开天门",
				Concept: "修仙故事",
				Volumes: 21,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := domain.ValidateBookQueueItem(tt.item)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateBookQueueItem() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
