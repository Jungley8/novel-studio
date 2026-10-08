import { api } from '../../api/client';

export function createWorkbenchActions(state, notify, helpers) {
  return {
    async runLinter() {
      if (!state.workbench.content) return;
      try {
        state.linterReport = await api.lintAnalyze(state.workbench.content);
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

      const nextIndex = state.chapters.length + 1;
      let conflict = state.workbench.coreConflict?.trim();
      if (!conflict) {
        conflict = `第 ${nextIndex} 章高潮突围与世界法则冲突，主角面临因果阻碍与强敌制衡。`;
        state.workbench.coreConflict = conflict;
      }

      const payload = {
        chapter_index: nextIndex,
        core_conflict: conflict,
        initial_draft: state.workbench.content?.trim() || '',
        words_target: 2000,
        auto_commit: true,
        max_rewrite_loops: 3,
        enable_harmonize: true,
        resume_checkpoint: true,
      };

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
                        state.linterReport = data.audit_report.linter;
                      }
                    }
                  } else if (currentEvent === 'complete') {
                    if (data.content) state.workbench.content = data.content;
                    if (data.beats) state.workbench.beats = data.beats;
                    if (data.audit) state.reviewResult = data.audit;
                    if (data.total_usage) {
                      state.pipelineState.tokens = data.total_usage;
                    }
                    state.pipelineState.message = `第 ${data.chapter_index} 章自主推演完毕并已入库！`;
                    notify(`第 ${data.chapter_index} 章生产完成`, '手稿与因果状态已原子写入正史', 'success');
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
          const result = await res.json();
          if (result.content) state.workbench.content = result.content;
          if (result.beats) state.workbench.beats = result.beats;
          if (result.audit) state.reviewResult = result.audit;
          state.pipelineState.message = `第 ${result.chapter_index} 章自主推演完毕并已入库！`;
          notify(`第 ${result.chapter_index} 章生产完成`, '手稿与因果状态已原子写入正史', 'success');
        }

        if (helpers && helpers.selectProject) {
          await helpers.selectProject(state.currentProject.id);
        }
        state.pipelineState.lastFinished = true;
      } catch (err) {
        console.error('runAutonomousPipeline error:', err);
        notify('自主推演中断', err.message, 'error');
        state.pipelineState.message = '自主推演失败: ' + err.message;
      } finally {
        state.pipelineState.active = false;
      }
    },
  };
}
