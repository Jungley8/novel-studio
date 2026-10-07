package store

import (
	"context"

	"github.com/Jungley8/novel-studio/internal/domain"
)

// Store defines the persistence operations for NovelStudio.
type Store interface {
	// Project lifecycle
	GetProject(ctx context.Context, id string) (*domain.Project, error)
	ListProjects(ctx context.Context) ([]*domain.Project, error)
	SaveProject(ctx context.Context, project *domain.Project) error
	DeleteProject(ctx context.Context, id string) error

	// Chapter persistence
	SaveChapter(ctx context.Context, chapter *domain.Chapter) error
	GetChapter(ctx context.Context, projectID string, chapterIndex int) (*domain.Chapter, error)
	ListChapters(ctx context.Context, projectID string) ([]*domain.Chapter, error)

	// Plot Hooks (伏笔因果账本)
	SavePlotHook(ctx context.Context, hook *domain.PlotHook) error
	ListPlotHooks(ctx context.Context, projectID string) ([]*domain.PlotHook, error)
	DeletePlotHook(ctx context.Context, id string) error

	// Close database connection
	Close() error
}
