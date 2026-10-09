import { api } from '../../api/client';

export function createWorkbenchActions(state, notify, helpers, dialogs) {
  return {
    async runLinter() {
      if (!state.workbench.content) return;
      try {
        const rep = await api.lintAnalyze(state.workbench.content);
        if (rep) {
          state.linterReport = {
            burstiness_score: rep.burstiness_score || 0,
            hit_banned_words: Array.isArray(rep.hit_banned_words) ? rep.hit_banned_words : [],
            top_repeated_ngrams: Array.isArray(rep.top_repeated_ngrams) ? rep.top_repeated_ngrams : [],
            passed: Boolean(rep.passed),
            message: rep.message || '',
            dialogue_ratio: rep.dialogue_ratio || 0,
            paragraph_variance: rep.paragraph_variance || 0,
            exclamation_density: rep.exclamation_density || 0,
          };
        }
      } catch (e) {
        console.error('linter error:', e);
      }
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
      state.pipelineState.message = '正在启动自主闭环流水线...';
      state.pipelineState.tokens = { prompt_tokens: 0, completion_tokens: 0, total_tokens: 0 };
      state.pipelineState.lastFinished = false;

      const nextIndex = state.editingChapterIndex || (state.chapters.length + 1);
      let conflict = state.workbench.coreConflict?.trim();
      if (!conflict) {
        conflict = `第 ${nextIndex} 章高潮突围与世界法则冲突，主角面临因果阻碍与强敌制衡。`;
        state.workbench.coreConflict = conflict;
      }

      // 立即将用户在工坊修改的最新手稿与节拍持久化到当前断点，避免仅在前端内存导致不同步
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
                        };
                      }
                    }
                    if (data.total_usage) {
                      state.pipelineState.tokens = data.total_usage;
                    }
                  } else if (currentEvent === 'error') {
                    throw new Error(data.error || '自主推演遭遇未知异常');
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
              };
            }
          }
        }

        if (completedResult && completedResult.committed) {
          state.pipelineState.message = `第 ${completedResult.chapter_index} 章自主推演完毕并已封存入库！`;
          notify(`第 ${completedResult.chapter_index} 章生产完成`, '手稿与因果状态已原子写入正史', 'success');

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
          state.pipelineState.message = `第 ${nextIndex} 章推演质检评级为 ${verdict} (${score}分)，未达到自动封存入库标准，已保留在工作台中`;
          notify(
            '质检未达到自动归档标准',
            `评级: ${verdict} (${score}分)，手稿未自动归档，已停留在返工工作台等待人工复核`,
            'warning'
          );

          // 保持在当前章节工坊编辑状态，严禁清除 editingChapterIndex 或关闭工坊
          state.editingChapterIndex = nextIndex;
          // 定位至步骤 5 (针对性返工 / 审校面板)，展示问题与修改建议
          state.activeStep = 5;
          // 持久化当前草稿与终审结果至断点，刷新不丢失
          await this.saveCheckpoint({}, true);
        }
        state.pipelineState.lastFinished = true;
      } catch (err) {
        console.error('runAutonomousPipeline error:', err);
        notify('自主推演中断', err.message, 'error');
        state.pipelineState.message = '自主推演失败: ' + err.message;
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
          notify('草稿断点已保存', `第 ${idx} 章当前手稿与工步已持久化存盘`, 'success', 2000);
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
          const detail = cp.draft_text ? `共 ${cp.draft_text.length} 字手稿` : '包含已推演节拍';
          notify('已自动恢复草稿断点', `已载入第 ${cp.chapter_index} 章断点（${detail}）`, 'info');
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
          title: '放弃章节草稿',
          message: `确定彻底废弃并清空第 ${idx} 章的在途草稿与节拍吗？未封存的改动将无法找回。`,
          type: 'danger',
          confirmText: '确认废弃',
        });
        if (!ok) return;
      }
      try {
        await api.clearCheckpoint(state.currentProject.id, idx);
        state.workbench.content = '';
        state.workbench.coreConflict = '';
        state.workbench.beats = [
          { phase: '蓄力压迫', tension: 4, action: '', expectation_broken: '' },
          { phase: '试探下套', tension: 6, action: '', expectation_broken: '' },
          { phase: '绝地反转', tension: 9, action: '', expectation_broken: '' },
          { phase: '章末留钩', tension: 8, action: '', expectation_broken: '' },
        ];
        state.reviewResult = null;
        state.editingChapterIndex = null;
        state.activeStep = 1;
        state.lastSavedAt = null;
        notify('草稿断点已废弃', `第 ${idx} 章在途草稿与节拍已全部清除`, 'info');
      } catch (err) {
        notify('清除断点失败', err.message, 'error');
      }
    },
  };
}
