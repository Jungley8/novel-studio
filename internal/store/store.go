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

	// Chapter lifecycle & atomic state mutation (Deep Module interface)
	CommitChapter(ctx context.Context, projectID string, chapter *domain.Chapter) (*domain.Project, error)
	SaveChapter(ctx context.Context, chapter *domain.Chapter) error
	GetChapter(ctx context.Context, projectID string, chapterIndex int) (*domain.Chapter, error)
	ListChapters(ctx context.Context, projectID string) ([]*domain.Chapter, error)

	// Plot Hooks (伏笔因果账本)
	SavePlotHook(ctx context.Context, hook *domain.PlotHook) error
	ListPlotHooks(ctx context.Context, projectID string) ([]*domain.PlotHook, error)
	DeletePlotHook(ctx context.Context, id string) error

	// Chapter Checkpoint (断点续跑)
	SaveCheckpoint(ctx context.Context, checkpoint *domain.ChapterCheckpoint) error
	GetCheckpoint(ctx context.Context, projectID string, chapterIndex int) (*domain.ChapterCheckpoint, error)
	ClearCheckpoint(ctx context.Context, projectID string, chapterIndex int) error

	// The Codex (全域世界观百科)
	SaveCodexEntry(ctx context.Context, entry *domain.CodexEntry) error
	GetCodexEntry(ctx context.Context, projectID, id string) (*domain.CodexEntry, error)
	ListCodexEntries(ctx context.Context, projectID string, category domain.CodexCategory) ([]*domain.CodexEntry, error)
	DeleteCodexEntry(ctx context.Context, projectID, id string) error
	SaveCodexProgression(ctx context.Context, entryID string, prog *domain.Progression) error
	ListCodexProgressions(ctx context.Context, entryID string) ([]domain.Progression, error)
	SaveCodexRelation(ctx context.Context, projectID string, rel *domain.EntityRelation) error
	ListCodexRelations(ctx context.Context, projectID string, entryID string) ([]domain.EntityRelation, error)
	DeleteCodexRelation(ctx context.Context, projectID string, id string) error

	// The Matrix (场景场次与矩阵大纲)
	SaveScene(ctx context.Context, scene *domain.Scene) error
	GetScene(ctx context.Context, id string) (*domain.Scene, error)
	ListScenes(ctx context.Context, chapterID string) ([]*domain.Scene, error)
	DeleteScene(ctx context.Context, id string) error
	SaveSceneMarker(ctx context.Context, marker *domain.SceneMarker) error
	ListSceneMarkers(ctx context.Context, sceneID string) ([]domain.SceneMarker, error)
	DeleteSceneMarker(ctx context.Context, id string) error
	GetMatrixOverview(ctx context.Context, projectID string) (*domain.MatrixOverview, error)

	// Close database connection
	Close() error
}
