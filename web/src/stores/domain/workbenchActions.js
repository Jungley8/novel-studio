import { api } from '../../api/client';

export function createWorkbenchActions(state, notify, helpers, dialogs) {
  return {
    async runLinter() {
      if (!state.workbench.content) return;
      state.isLinting = true;
      try {
        const rep = await api.lintAnalyze(state.workbench.content);
        if (rep) {
          state.linterReport = {
            burstiness_score: rep.burstiness_score || 0,
            hit_banned_words: Array.isArray(rep.hit_banned_words) ? rep.hit_banned_words : [],
            top_repeated_ngrams: Array.isArray(rep.top_repeated_ngrams) ? rep.top_repeated_ngrams : [],
            empirical_tells: Array.isArray(rep.empirical_tells) ? rep.empirical_tells : [],
            passed: Boolean(rep.passed),
            message: rep.message || '',
            dialogue_ratio: rep.dialogue_ratio || 0,
            paragraph_variance: rep.paragraph_variance || 0,
            exclamation_density: rep.exclamation_density || 0,
          };
        }
      } catch (e) {
        console.error('linter error:', e);
      } finally {
        state.isLinting = false;
      }
    },

    async suggestChapterConflict(targetIndex) {
      if (!state.currentProject) {
        notify('未选择作品', '请先在左侧选择或新建小说作品', 'warning');
        return;
      }
      const idx = targetIndex || state.editingChapterIndex || (state.chapters?.length ? state.chapters.length + 1 : 1);
      state.isSuggestingConflict = true;
      try {
        const res = await api.suggestConflict(state.currentProject.id, { chapter_index: idx });
        if (res && res.core_conflict) {
          state.workbench.coreConflict = res.core_conflict;
          notify('构思就绪', `已为第 ${idx} 章推演核心冲突`, 'success');
          await this.saveCheckpoint({ core_conflict: res.core_conflict }, true);
        }
      } catch (err) {
        notify('构思失败', err.message || '推演冲突遇到问题，请重试', 'error');
      } finally {
        state.isSuggestingConflict = false;
      }
    },

    async startNextChapter() {
      if (!state.currentProject) return;
      state.pipelineState.justCommitted = false;
      state.editingChapterIndex = null;
      state.workbench.content = '';
      state.workbench.coreConflict = '';
      state.reviewResult = null;
      state.activeStep = 1;
      const nextIdx = state.chapters.length + 1;
      await this.suggestChapterConflict(nextIdx);
    },

    async runAutonomousPipeline() {
      if (!state.currentProject) {
        notify('未选择作品', '请先在左侧选择或新建小说作品', 'warning');
        return;
      }
      if (state.pipelineState.active) {
        return;
      }

      state.activeTab = 'workbench';
      state.pipelineState.active = true;
      state.pipelineState.phase = 'INIT';
      state.pipelineState.message = '正在一键成章，请稍候...';
      state.pipelineState.tokens = { prompt_tokens: 0, completion_tokens: 0, total_tokens: 0 };
      state.pipelineState.lastFinished = false;
      state.pipelineState.justCommitted = false;

      const nextIndex = state.editingChapterIndex || (state.chapters.length + 1);
      let conflict = state.workbench.coreConflict?.trim() || '';
      if (conflict.startsWith(`第 ${nextIndex} 章`) && conflict.includes('剧情冲突与关键转折')) {
        conflict = '';
      }

      // 立即将用户修改的最新手稿与分段保存至草稿，避免刷新后不同步
      if (state.workbench.content?.trim() || state.workbench.coreConflict?.trim()) {
        await this.saveCheckpoint({}, true);
      }

      const payload = {
        chapter_index: nextIndex,
        core_conflict: conflict,
        beats: (state.workbench.beats && state.workbench.beats.length > 0) ? state.workbench.beats : undefined,
        initial_draft: state.workbench.content?.trim() || '',
        words_target: state.wordsTarget || 2000,
        narrative_style: state.narrativeStyle || 'hardboiled',
        auto_commit: true,
        max_rewrite_loops: 3,
        enable_harmonize: true,
        resume_checkpoint: true,
      };

      let completedResult = null;

      try {
        const url = `/api/projects/${state.currentProject.id}/workshop/produce?stream=true`;
        const res = await fetch(url, {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            'Accept': 'text/event-stream',
          },
          body: JSON.stringify(payload),
        });

        if (!res.ok) {
          let errText = `HTTP Error ${res.status}`;
          try {
            const errJson = await res.json();
            errText = errJson.error || errText;
          } catch (_) {}
          throw new Error(errText);
        }

        if (res.body && typeof res.body.getReader === 'function') {
          const reader = res.body.getReader();
          const decoder = new TextDecoder();
          let buffer = '';

          while (true) {
            const { done, value } = await reader.read();
            if (done) break;
            buffer += decoder.decode(value, { stream: true });

            const lines = buffer.split('\n');
            buffer = lines.pop() || '';

            let currentEvent = '';
            for (const line of lines) {
              const trimmed = line.trim();
              if (trimmed.startsWith('event:')) {
                currentEvent = trimmed.replace('event:', '').trim();
              } else if (trimmed.startsWith('data:') && currentEvent) {
                const dataStr = trimmed.replace('data:', '').trim();
                try {
                  const data = JSON.parse(dataStr);
                  if (currentEvent === 'progress') {
                    state.pipelineState.phase = data.phase || state.pipelineState.phase;
                    state.pipelineState.message = data.message || state.pipelineState.message;
                    if (data.core_conflict) {
                      state.workbench.coreConflict = data.core_conflict;
                    }
                    if (data.total_tokens) {
                      state.pipelineState.tokens.total_tokens = data.total_tokens;
                    }
                    if (data.beats && data.beats.length > 0) {
                      state.workbench.beats = data.beats;
                    }
                    if (data.draft_text) {
                      state.workbench.content = data.draft_text;
                    }
                    if (data.audit_report) {
                      state.reviewResult = data.audit_report;
                      if (data.audit_report.linter) {
                        state.linterReport = {
                          ...data.audit_report.linter,
                          hit_banned_words: Array.isArray(data.audit_report.linter.hit_banned_words) ? data.audit_report.linter.hit_banned_words : [],
                          top_repeated_ngrams: Array.isArray(data.audit_report.linter.top_repeated_ngrams) ? data.audit_report.linter.top_repeated_ngrams : [],
                          empirical_tells: Array.isArray(data.audit_report.linter.empirical_tells) ? data.audit_report.linter.empirical_tells : [],
                        };
                      }
                    }
                  } else if (currentEvent === 'complete') {
                    completedResult = data;
                    if (data.content) state.workbench.content = data.content;
                    if (data.beats) state.workbench.beats = data.beats;
                    if (data.audit) {
                      state.reviewResult = data.audit;
                      if (data.audit.linter) {
                        state.linterReport = {
                          ...data.audit.linter,
                          hit_banned_words: Array.isArray(data.audit.linter.hit_banned_words) ? data.audit.linter.hit_banned_words : [],
                          top_repeated_ngrams: Array.isArray(data.audit.linter.top_repeated_ngrams) ? data.audit.linter.top_repeated_ngrams : [],
                          empirical_tells: Array.isArray(data.audit.linter.empirical_tells) ? data.audit.linter.empirical_tells : [],
                        };
                      }
                    }
                    if (data.total_usage) {
                      state.pipelineState.tokens = data.total_usage;
                    }
                  } else if (currentEvent === 'error') {
                    throw new Error(data.error || '生成章节遭遇异常');
                  }
                } catch (parseErr) {
                  if (currentEvent === 'error') throw parseErr;
                }
              }
            }
          }
        } else {
          completedResult = await res.json();
          if (completedResult.content) state.workbench.content = completedResult.content;
          if (completedResult.beats) state.workbench.beats = completedResult.beats;
          if (completedResult.audit) {
            state.reviewResult = completedResult.audit;
            if (completedResult.audit.linter) {
              state.linterReport = {
                ...completedResult.audit.linter,
                hit_banned_words: Array.isArray(completedResult.audit.linter.hit_banned_words) ? completedResult.audit.linter.hit_banned_words : [],
                top_repeated_ngrams: Array.isArray(completedResult.audit.linter.top_repeated_ngrams) ? completedResult.audit.linter.top_repeated_ngrams : [],
                empirical_tells: Array.isArray(completedResult.audit.linter.empirical_tells) ? completedResult.audit.linter.empirical_tells : [],
              };
            }
          }
        }

        if (completedResult && completedResult.committed) {
          state.pipelineState.message = `第 ${completedResult.chapter_index} 章已完成并保存至章节目录！`;
          state.pipelineState.justCommitted = true;
          state.pipelineState.lastCommittedChapter = completedResult.chapter_index;
          notify(`第 ${completedResult.chapter_index} 章已完成`, '本章手稿已成功定稿存入目录', 'success');

          if (helpers && helpers.selectProject) {
            await helpers.selectProject(state.currentProject.id);
          }
          state.editingChapterIndex = null;
          state.workbench.content = '';
          state.workbench.coreConflict = '';
          state.reviewResult = null;
          state.activeStep = 1;
        } else {
          // 质检不达标（REVISION_NEEDED / REJECTED）或未触发入库
          const audit = completedResult?.audit || state.reviewResult;
          const verdict = audit?.verdict || '待返工';
          const score = audit?.score ?? 0;
          state.pipelineState.message = `第 ${nextIndex} 章体检结果为 ${verdict} (${score}分)，已保留在工作台供修改`;
          notify(
            '体检未达标',
            `得分: ${score}分，手稿已保留在工作台，可直接精修`,
            'warning'
          );

          // 保持在当前章节工坊编辑状态，严禁清除 editingChapterIndex 或关闭工坊
          state.editingChapterIndex = nextIndex;
          // 定位至步骤 5 (精修面板)，展示问题与修改建议
          state.activeStep = 5;
          // 保存当前草稿与体检建议，刷新不丢失
          await this.saveCheckpoint({}, true);
        }
        state.pipelineState.lastFinished = true;
      } catch (err) {
        console.error('runAutonomousPipeline error:', err);
        notify('生成中断', err.message, 'error');
        state.pipelineState.message = '生成失败: ' + err.message;
        state.editingChapterIndex = nextIndex;
      } finally {
        state.pipelineState.active = false;
      }
    },

    async saveCheckpoint(customData = {}, silent = true) {
      if (!state.currentProject) return;
      const idx = state.editingChapterIndex || (state.chapters ? state.chapters.length + 1 : 1);
      state.isSavingDraft = true;
      try {
        const payload = {
          project_id: state.currentProject.id,
          chapter_index: idx,
          phase: state.reviewResult ? 'AUDITED' : (state.workbench.content ? 'DRAFTED' : 'BEATS_DERIVED'),
          core_conflict: state.workbench.coreConflict || '',
          beats: state.workbench.beats || [],
          state_mutation: state.workbench.stateMutation || { inventory_delta: '', power_delta: '' },
          draft_text: state.workbench.content || '',
          audit_report: state.reviewResult || null,
          rewrite_loops: state.rewriteLoopCount || 0,
          ...customData,
        };
        await api.saveCheckpoint(state.currentProject.id, payload);
        state.lastSavedAt = new Date();
        if (!silent) {
          notify('草稿已保存', `第 ${idx} 章当前进度已保存`, 'success', 2000);
        }
      } catch (err) {
        console.error('save checkpoint failed:', err);
        if (!silent) {
          notify('保存草稿失败', err.message, 'error');
        }
      } finally {
        state.isSavingDraft = false;
      }
    },

    async restoreCheckpoint(targetIndex) {
      if (!state.currentProject) return;
      if (state.workbench.content && state.workbench.content.trim() && !targetIndex) return;
      const idx = targetIndex || state.editingChapterIndex || (state.chapters ? state.chapters.length + 1 : 1);
      try {
        const cp = await api.getCheckpoint(state.currentProject.id, idx);
        if (cp && (cp.draft_text || (cp.beats && cp.beats.length) || cp.core_conflict)) {
          if (cp.draft_text) {
            state.workbench.content = cp.draft_text;
          }
          state.editingChapterIndex = cp.chapter_index;
          if (cp.core_conflict && !state.workbench.coreConflict) {
            state.workbench.coreConflict = cp.core_conflict;
          }
          if (cp.beats && cp.beats.length) {
            state.workbench.beats = cp.beats;
          }
          if (cp.state_mutation) {
            state.workbench.stateMutation = cp.state_mutation;
          }
          if (cp.audit_report) {
            state.reviewResult = cp.audit_report;
            if (cp.audit_report.linter) {
              state.linterReport = {
                ...cp.audit_report.linter,
                hit_banned_words: Array.isArray(cp.audit_report.linter.hit_banned_words) ? cp.audit_report.linter.hit_banned_words : [],
                top_repeated_ngrams: Array.isArray(cp.audit_report.linter.top_repeated_ngrams) ? cp.audit_report.linter.top_repeated_ngrams : [],
                empirical_tells: Array.isArray(cp.audit_report.linter.empirical_tells) ? cp.audit_report.linter.empirical_tells : [],
              };
            }
          }
          if (cp.rewrite_loops) {
            state.rewriteLoopCount = cp.rewrite_loops;
          }
          if (cp.phase === 'AUDITED' || cp.phase === 'REWRITING') {
            state.activeStep = cp.audit_report?.verdict === 'ACCEPTED' ? 6 : 5;
          } else if (cp.phase === 'DRAFTED' || cp.draft_text) {
            state.activeStep = 3;
          } else if (cp.phase === 'BEATS_DERIVED' || (cp.beats && cp.beats.length)) {
            state.activeStep = 2;
          }
          if (this.runLinter && state.workbench.content) {
            this.runLinter();
          }
          state.lastSavedAt = new Date();
          const detail = cp.draft_text ? `共 ${cp.draft_text.length} 字` : '包含分段构思';
          notify('已恢复草稿', `已载入第 ${cp.chapter_index} 章进度（${detail}）`, 'info');
        }
      } catch (err) {
        console.warn('restore checkpoint failed:', err);
      }
    },

    async discardCheckpoint(targetIndex) {
      if (!state.currentProject) return;
      const idx = targetIndex || state.editingChapterIndex || (state.chapters ? state.chapters.length + 1 : 1);
      if (dialogs) {
        const ok = await dialogs.confirm({
          title: '放弃本章草稿',
          message: `确定清空第 ${idx} 章的在途草稿与分段吗？未保存的内容将无法找回。`,
          type: 'danger',
          confirmText: '确认清空',
        });
        if (!ok) return;
      }
      try {
        await api.clearCheckpoint(state.currentProject.id, idx);
        state.workbench.content = '';
        state.workbench.coreConflict = '';
        state.workbench.beats = [
          { phase: '开端压迫', tension: 4, action: '', expectation_broken: '' },
          { phase: '冲突升级', tension: 6, action: '', expectation_broken: '' },
          { phase: '绝地反转', tension: 9, action: '', expectation_broken: '' },
          { phase: '留存悬念', tension: 8, action: '', expectation_broken: '' },
        ];
        state.reviewResult = null;
        state.editingChapterIndex = null;
        state.activeStep = 1;
        state.lastSavedAt = null;
        notify('草稿已清空', `第 ${idx} 章草稿与分段已清空`, 'info');
      } catch (err) {
        notify('清空草稿失败', err.message, 'error');
      }
    },

    async runDeterministicSanitize() {
      if (!state.currentProject || !state.workbench.content?.trim()) {
        notify('正文为空', '手稿区暂无正文可供去壳', 'warning');
        return;
      }
      state.isSanitizing = true;
      try {
        const res = await api.sanitizeAI(state.currentProject.id, {
          content: state.workbench.content,
        });
        if (res && res.sanitized_content) {
          state.workbench.content = res.sanitized_content;
          await this.runLinter();
          await this.saveCheckpoint({}, true);
          if (res.items_count > 0) {
            notify(
              '去机械壳完成',
              `已优化 ${res.items_count} 处机械套话与从句`,
              'success',
              3500,
              { taskType: 'sanitize', count: res.items_count }
            );
          } else {
            notify('去机械壳完成', '正文自然流畅，未发现机械腔调', 'info', 3000, { taskType: 'sanitize' });
          }
        }
      } catch (err) {
        console.error('sanitizeAI failed:', err);
        notify('去壳失败', err.message, 'error', 4500, { taskType: 'sanitize' });
      } finally {
        state.isSanitizing = false;
      }
    },
  };
}
