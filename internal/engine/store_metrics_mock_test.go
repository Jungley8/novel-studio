package engine

import (
	"context"

	"github.com/Jungley8/novel-studio/internal/domain"
)

// SaveChapterMetrics stubs the store.Store interface method for mockChronicleStore.
func (m *mockChronicleStore) SaveChapterMetrics(ctx context.Context, metrics *domain.ChapterQualityMetrics) error {
	return nil
}

// ListChapterMetrics stubs the store.Store interface method for mockChronicleStore.
func (m *mockChronicleStore) ListChapterMetrics(ctx context.Context, projectID string) ([]*domain.ChapterQualityMetrics, error) {
	return nil, nil
}
