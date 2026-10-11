package domain

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

type AutomationQuotaKind string

const (
	QuotaKindChapters AutomationQuotaKind = "chapters"
	QuotaKindVolumes  AutomationQuotaKind = "volumes"
)

// ProjectAutomation 是作品级无人值守排程配置与运行状态。
type ProjectAutomation struct {
	ProjectID    string              `json:"project_id"`
	ProjectTitle string              `json:"project_title"` // 只读：List 时 JOIN projects 带出，Save 忽略
	Enabled      bool                `json:"enabled"`
	Times        []string            `json:"times"` // 每日触发时刻 "HH:MM"（本地时区），可多条
	QuotaKind    AutomationQuotaKind `json:"quota_kind"`
	QuotaCount   int                 `json:"quota_count"`
	Paused       bool                `json:"paused"`
	PausedReason string              `json:"paused_reason,omitempty"`
	LastRunAt    string              `json:"last_run_at,omitempty"` // RFC3339
	LastStatus   string              `json:"last_status,omitempty"` // ok|halted|error
	LastMessage  string              `json:"last_message,omitempty"`
}

// BookQueueItem 是新书队列条目：到触发时刻自动开书并批产。
type BookQueueItem struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Concept     string `json:"concept"`
	Platform    string `json:"platform"`
	Volumes     int    `json:"volumes"`   // 开书后自动批产卷数
	AutoLock    bool   `json:"auto_lock"` // 开书后自动锁定意图卡（无人值守必须 true）
	Status      string `json:"status"`    // pending|running|done|halted|error
	ProjectID   string `json:"project_id,omitempty"`
	LastMessage string `json:"last_message,omitempty"`
	CreatedAt   string `json:"created_at"` // RFC3339
	UpdatedAt   string `json:"updated_at"`
}

const (
	QueueStatusPending = "pending"
	QueueStatusRunning = "running"
	QueueStatusDone    = "done"
	QueueStatusHalted  = "halted"
	QueueStatusError   = "error"
)

// ValidateDailyTimes 校验 "HH:MM" 列表（每条必须 len==5、2 位时 2 位分、00-23/00-59；列表允许空；最多 48 条）。
func ValidateDailyTimes(times []string) error {
	if len(times) > 48 {
		return fmt.Errorf("daily times count %d exceeds maximum of 48", len(times))
	}
	for _, t := range times {
		if len(t) != 5 || t[2] != ':' {
			return fmt.Errorf("invalid time format %q, expected HH:MM", t)
		}
		if t[0] < '0' || t[0] > '9' || t[1] < '0' || t[1] > '9' {
			return fmt.Errorf("invalid hour digits in %q", t)
		}
		hour := int(t[0]-'0')*10 + int(t[1]-'0')
		if hour < 0 || hour > 23 {
			return fmt.Errorf("hour %02d out of range (00-23) in %q", hour, t)
		}

		if t[3] < '0' || t[3] > '9' || t[4] < '0' || t[4] > '9' {
			return fmt.Errorf("invalid minute digits in %q", t)
		}
		minute := int(t[3]-'0')*10 + int(t[4]-'0')
		if minute < 0 || minute > 59 {
			return fmt.Errorf("minute %02d out of range (00-59) in %q", minute, t)
		}
	}
	return nil
}

// ValidateAutomation 校验：QuotaKind ∈ {chapters,volumes}；chapters→QuotaCount 1..100，volumes→1..10；
// Enabled && len(Times)==0 → error；Times 走 ValidateDailyTimes。
func ValidateAutomation(a *ProjectAutomation) error {
	if a == nil {
		return errors.New("project automation cannot be nil")
	}

	switch a.QuotaKind {
	case QuotaKindChapters:
		if a.QuotaCount < 1 || a.QuotaCount > 100 {
			return fmt.Errorf("quota_count for chapters must be 1..100, got %d", a.QuotaCount)
		}
	case QuotaKindVolumes:
		if a.QuotaCount < 1 || a.QuotaCount > 10 {
			return fmt.Errorf("quota_count for volumes must be 1..10, got %d", a.QuotaCount)
		}
	default:
		return fmt.Errorf("invalid quota_kind %q, must be chapters or volumes", a.QuotaKind)
	}

	if a.Enabled && len(a.Times) == 0 {
		return errors.New("times cannot be empty when automation is enabled")
	}

	if err := ValidateDailyTimes(a.Times); err != nil {
		return err
	}

	return nil
}

// ValidateBookQueueItem 校验：Title trim 非空且 ≤200 rune；Concept trim 非空且 ≤4000 rune；Platform ≤100 rune（可空）；
// Volumes 1..20。不校验 Status（由 store/调度器内部流转）。
func ValidateBookQueueItem(item *BookQueueItem) error {
	if item == nil {
		return errors.New("book queue item cannot be nil")
	}

	trimmedTitle := strings.TrimSpace(item.Title)
	if trimmedTitle == "" {
		return errors.New("title cannot be empty")
	}
	if utf8.RuneCountInString(trimmedTitle) > 200 {
		return fmt.Errorf("title exceeds maximum length of 200 runes, got %d", utf8.RuneCountInString(trimmedTitle))
	}

	trimmedConcept := strings.TrimSpace(item.Concept)
	if trimmedConcept == "" {
		return errors.New("concept cannot be empty")
	}
	if utf8.RuneCountInString(trimmedConcept) > 4000 {
		return fmt.Errorf("concept exceeds maximum length of 4000 runes, got %d", utf8.RuneCountInString(trimmedConcept))
	}

	if utf8.RuneCountInString(item.Platform) > 100 {
		return fmt.Errorf("platform exceeds maximum length of 100 runes, got %d", utf8.RuneCountInString(item.Platform))
	}

	if item.Volumes < 1 || item.Volumes > 20 {
		return fmt.Errorf("volumes must be 1..20, got %d", item.Volumes)
	}

	return nil
}
