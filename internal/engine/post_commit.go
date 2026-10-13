package engine

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Jungley8/novel-studio/internal/domain"
	"github.com/Jungley8/novel-studio/internal/store"
)

// PostCommitReport 记录定稿后增强处理（摘要、分卷回顾、伏笔提取）的执行状态与警告信息。
type PostCommitReport struct {
	SummaryGenerated bool
	Summary          *ChapterSummary
	SummaryUsage     TokenUsage
	RetroGenerated   bool
	Retro            *VolumeRetrospective
	RetroUsage       TokenUsage
	HooksExtracted   int
	HooksRegistered  []string
	HooksUsage       TokenUsage
	RejectedHooks    []PlantedHookRejection
	Issues           []string
	TotalUsage       TokenUsage
}

// RunPostCommitEnhancement 统一承载自动化流水线（workshop.go）与人工手动定稿（handlers_project.go）后的全部增强生命周期：
// 1. 章节摘要生成（SummarizeChapter）并持久化至 chapter_summaries 表；
// 2. 卷末回顾判定与生成（BuildVolumeRetrospective）并持久化至 volume_retrospectives 表；
// 3. 正文新埋伏笔自动提取（ExtractPlantedHooks）并经登记证据门（FilterPlantedHookCandidates）持久化至 plot_hooks 表。
// 遵循系统核心契约：增强层失败以警告记录，绝不阻断或回滚已成功归档入库的正文真相。
func RunPostCommitEnhancement(
	ctx context.Context,
	client LLMClient,
	model string,
	st store.Store,
	project *domain.Project,
	chapter *domain.Chapter,
) (*PostCommitReport, error) {
	report := &PostCommitReport{
		HooksRegistered: []string{},
		Issues:          []string{},
	}
	if chapter == nil {
		return report, nil
	}
	if client == nil || st == nil {
		report.Issues = append(report.Issues, "【后置增强跳过】未配置 LLM Client 或存储层")
		return report, nil
	}
	if strings.TrimSpace(chapter.Content) == "" {
		report.Issues = append(report.Issues, "【后置增强跳过】章节正文为空")
		return report, nil
	}

	projectID := chapter.ProjectID
	if projectID == "" && project != nil {
		projectID = project.ID
	}

	// 1. 生成并保存章节摘要 (SummarizeChapter)
	cs, csUsage, csErr := SummarizeChapter(
		ctx,
		client,
		model,
		project,
		chapter.ChapterIndex,
		chapter.Content,
		chapter.StateMutation,
	)
	addTokenUsage(&report.SummaryUsage, csUsage)
	addTokenUsage(&report.TotalUsage, csUsage)
	if csErr != nil {
		report.Issues = append(report.Issues, fmt.Sprintf("【摘要警告】生成章节摘要失败: %v", csErr))
	} else {
		report.Summary = &cs
		if saver, ok := st.(interface {
			SaveChapterSummary(ctx context.Context, cs *domain.ChapterSummary) error
		}); ok {
			if cs.ProjectID == "" {
				cs.ProjectID = projectID
			}
			if err := saver.SaveChapterSummary(ctx, &cs); err != nil {
				report.Issues = append(report.Issues, fmt.Sprintf("【摘要警告】保存章节摘要失败: %v", err))
			} else {
				report.SummaryGenerated = true
			}
		} else {
			report.Issues = append(report.Issues, "【摘要警告】存储层未实现 SaveChapterSummary 接口")
		}
	}

	// 2. 检查分卷是否完结，若到卷末则生成分卷回顾 (VolumeRetrospective)
	if project != nil && project.Framework != nil && len(project.Framework.VolumeArcs) > 0 {
		project.Framework.VolumeArcs = BackfillVolumeArcs(project.Framework.VolumeArcs)
		spans, arcMap := deriveVolumeSpans(project.Framework.VolumeArcs)
		volIdx := domain.ResolveVolume(chapter.ChapterIndex, spans)
		if volIdx != -1 {
			var currentSpan *domain.VolumeSpan
			for i := range spans {
				if spans[i].VolumeIndex == volIdx {
					currentSpan = &spans[i]
					break
				}
			}
			if currentSpan != nil && chapter.ChapterIndex == currentSpan.EndChapter {
				arc := arcMap[volIdx]
				if arc != nil {
					if vrStore, ok := st.(interface {
						ListChapterSummaries(ctx context.Context, projectID string, from, to int) ([]*domain.ChapterSummary, error)
						SaveVolumeRetrospective(ctx context.Context, retro *domain.VolumeRetrospective) error
					}); ok {
						summaries, lErr := vrStore.ListChapterSummaries(ctx, projectID, currentSpan.StartChapter, currentSpan.EndChapter)
						if lErr != nil {
							report.Issues = append(report.Issues, fmt.Sprintf("【摘要警告】获取分卷章节摘要失败: %v", lErr))
						} else {
							var chSummaries []ChapterSummary
							for _, s := range summaries {
								if s != nil {
									chSummaries = append(chSummaries, *s)
								}
							}
							if len(chSummaries) == 0 {
								report.Issues = append(report.Issues, "【摘要警告】未找到本卷任何章节摘要，跳过分卷回顾")
							} else {
								vr, vrUsage, vrErr := BuildVolumeRetrospective(ctx, client, model, project, *arc, *currentSpan, chSummaries)
								addTokenUsage(&report.RetroUsage, vrUsage)
								addTokenUsage(&report.TotalUsage, vrUsage)
								if vrErr != nil {
									report.Issues = append(report.Issues, fmt.Sprintf("【摘要警告】生成分卷回顾失败: %v", vrErr))
								} else {
									report.Retro = &vr
									if vr.ProjectID == "" {
										vr.ProjectID = projectID
									}
									if sErr := vrStore.SaveVolumeRetrospective(ctx, &vr); sErr != nil {
										report.Issues = append(report.Issues, fmt.Sprintf("【摘要警告】保存分卷回顾失败: %v", sErr))
									} else {
										report.RetroGenerated = true
									}
								}
							}
						}
					} else {
						report.Issues = append(report.Issues, "【摘要警告】存储层未实现分卷回顾持久化接口")
					}
				}
			}
		}
	}

	// 3. 伏笔自动登记 (ExtractPlantedHooks + FilterPlantedHookCandidates)
	existingHooks, lerr := st.ListPlotHooks(ctx, projectID)
	if lerr != nil {
		report.Issues = append(report.Issues, fmt.Sprintf("【伏笔登记警告】读取伏笔台账失败: %v", lerr))
	} else {
		candidates, regUsage, rerr := ExtractPlantedHooks(ctx, client, model, project, chapter.ChapterIndex, chapter.Content, existingHooks)
		addTokenUsage(&report.HooksUsage, regUsage)
		addTokenUsage(&report.TotalUsage, regUsage)
		if rerr != nil {
			report.Issues = append(report.Issues, fmt.Sprintf("【伏笔登记警告】伏笔提取失败: %v", rerr))
		} else {
			valid, rejected := FilterPlantedHookCandidates(candidates, chapter.Content, existingHooks)
			report.RejectedHooks = rejected
			for _, r := range rejected {
				report.Issues = append(report.Issues, fmt.Sprintf("【伏笔登记被拒】「%s」: %s", r.Candidate.Title, r.Reason))
			}
			for i, c := range valid {
				hook := &domain.PlotHook{
					ID:             fmt.Sprintf("hook_auto_%d_%d", time.Now().UnixNano(), i+1),
					ProjectID:      projectID,
					Title:          c.Title,
					Details:        c.Details,
					CreatedChapter: chapter.ChapterIndex,
					TargetChapter:  c.TargetChapter,
					Status:         domain.HookStatusOpen,
					CreatedAt:      time.Now(),
				}
				if serr := st.SavePlotHook(ctx, hook); serr != nil {
					report.Issues = append(report.Issues, fmt.Sprintf("【伏笔登记警告】伏笔「%s」落库失败: %v", c.Title, serr))
					continue
				}
				report.HooksRegistered = append(report.HooksRegistered, c.Title)
			}
			report.HooksExtracted = len(report.HooksRegistered)
			if len(report.HooksRegistered) > 0 {
				report.Issues = append(report.Issues, fmt.Sprintf("【伏笔登记】新增伏笔 %d 条: %s", len(report.HooksRegistered), strings.Join(report.HooksRegistered, "、")))
			}
		}
	}

	return report, nil
}
