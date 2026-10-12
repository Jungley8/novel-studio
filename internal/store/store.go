package store

import (
	"context"
	"errors"
	"time"

	"github.com/Jungley8/novel-studio/internal/activity"
	"github.com/Jungley8/novel-studio/internal/domain"
)

// ErrNotFound indicates that the requested entity was not found in the store.
var ErrNotFound = errors.New("not found")

// Store defines the persistence operations for NovelStudio.
type Store interface {
	// Project lifecycle
	GetProject(ctx context.Context, id string) (*domain.Project, error)
	ListProjects(ctx context.Context) ([]*domain.Project, error)
	SaveProject(ctx context.Context, project *domain.Project) error
	DeleteProject(ctx context.Context, id string) error

	// Chapter lifecycle & atomic state mutation (Deep Module interface)
	CommitChapter(ctx context.Context, projectID string, chapter *domain.Chapter) (*domain.Project, error)
	UncommitChapter(ctx context.Context, projectID string, chapterIndex int) (*domain.ChapterCheckpoint, *domain.Project, error)
	SaveChapter(ctx context.Context, chapter *domain.Chapter) error
	GetChapter(ctx context.Context, projectID string, chapterIndex int) (*domain.Chapter, error)
	ChapterProjectID(ctx context.Context, id string) (string, error)
	ListChapters(ctx context.Context, projectID string) ([]*domain.Chapter, error)
	DeleteChapter(ctx context.Context, projectID string, chapterIndex int) error
	ListStateSnapshots(ctx context.Context, projectID string) ([]*domain.ChapterStateSnapshot, error) // 快照链只读供给（bench 用，按 chapter_index 升序）
	SaveChapterMetrics(ctx context.Context, m *domain.ChapterQualityMetrics) error
	ListChapterMetrics(ctx context.Context, projectID string) ([]*domain.ChapterQualityMetrics, error) // 按 chapter_index 升序

	// Chapter Intents (章节意图卡)
	SaveChapterIntent(ctx context.Context, intent *domain.ChapterIntent) error
	GetChapterIntent(ctx context.Context, projectID string, chapterIndex int) (*domain.ChapterIntent, error)    // 不存在返回 (nil, nil)
	ListChapterIntents(ctx context.Context, projectID string, volumeIndex int) ([]*domain.ChapterIntent, error) // volumeIndex<=0 全体；按 chapter_index 升序
	DeleteChapterIntent(ctx context.Context, projectID string, chapterIndex int) error
	MarkIntentsStale(ctx context.Context, projectID string, volumeIndex int) (int64, error) // 仅 LOCKED→STALE，返回变更行数
	SearchMemory(ctx context.Context, projectID, query string, limit int) ([]*domain.MemoryHit, error)

	// Plot Hooks (伏笔因果账本)
	SavePlotHook(ctx context.Context, hook *domain.PlotHook) error
	ListPlotHooks(ctx context.Context, projectID string) ([]*domain.PlotHook, error)
	DeletePlotHook(ctx context.Context, id string) error
	PlotHookProjectID(ctx context.Context, id string) (string, error)

	// Chapter Checkpoint (断点续跑)
	SaveCheckpoint(ctx context.Context, checkpoint *domain.ChapterCheckpoint) error
	GetCheckpoint(ctx context.Context, projectID string, chapterIndex int) (*domain.ChapterCheckpoint, error)
	GetLatestCheckpoint(ctx context.Context, projectID string) (*domain.ChapterCheckpoint, error)
	ClearCheckpoint(ctx context.Context, projectID string, chapterIndex int) error
	ListNeedsHumanCheckpoints(ctx context.Context) ([]*domain.ChapterCheckpoint, error)

	// Activity Ledger (内容生产与变更事件账本, append-only)
	AppendEvent(ctx context.Context, event *activity.ActivityEvent) error
	ListEvents(ctx context.Context, q activity.EventQuery) ([]*activity.ActivityEvent, int64, error)
	GetEvent(ctx context.Context, projectID, eventID string) (*activity.ActivityEvent, error)

	// The Codex (全域世界观百科)
	GetProtagonistEntry(ctx context.Context, projectID string) (*domain.CodexEntry, error)
	SaveCodexEntry(ctx context.Context, entry *domain.CodexEntry) error
	GetCodexEntry(ctx context.Context, projectID, id string) (*domain.CodexEntry, error)
	CodexEntryProjectID(ctx context.Context, id string) (string, error)
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

	// Automation & Book Queue (无人值守排程与新书队列)
	ListProjectAutomations(ctx context.Context) ([]*domain.ProjectAutomation, error)
	GetProjectAutomation(ctx context.Context, projectID string) (*domain.ProjectAutomation, error)
	SaveProjectAutomation(ctx context.Context, a *domain.ProjectAutomation) error
	SetAutomationPaused(ctx context.Context, projectID string, paused bool, reason string) error
	RecordAutomationRun(ctx context.Context, projectID, status, message string, at time.Time) error
	ListBookQueue(ctx context.Context) ([]*domain.BookQueueItem, error)
	GetBookQueueItem(ctx context.Context, id string) (*domain.BookQueueItem, error)
	SaveBookQueueItem(ctx context.Context, item *domain.BookQueueItem) error
	DeleteBookQueueItem(ctx context.Context, id string) error
	ResetStaleQueueRunning(ctx context.Context) (int64, error)
	GetSchedulerState(ctx context.Context, key string) (string, error)
	SetSchedulerState(ctx context.Context, key, value string) error

	// Close database connection
	Close() error
}
