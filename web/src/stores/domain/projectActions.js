import { api } from '../../api/client';

export function createProjectActions(state, notify, helpers, dialogs) {
  return {
    async loadProjects() {
      try {
        state.projects = await api.listProjects();
        if (state.projects.length > 0 && !state.selectedProjectId) {
          state.selectedProjectId = state.projects[0].id;
          await this.selectProject(state.projects[0].id);
        }
      } catch (e) {
        console.error('load projects error:', e);
      }
    },

    async selectProject(id) {
      if (!id) return;
      state.selectedProjectId = id;
      try {
        state.currentProject = await api.getProject(id);
        state.chapters = await api.listChapters(id);
        state.hooks = await api.listHooks(id);
        if (helpers && helpers.loadMatrixOverview) {
          await helpers.loadMatrixOverview();
        }
        if (helpers && helpers.loadCodexEntries) {
          await helpers.loadCodexEntries();
        }
        if (helpers && helpers.loadAnalytics) {
          await helpers.loadAnalytics();
        }
        if (helpers && helpers.restoreCheckpoint) {
          await helpers.restoreCheckpoint();
        }
      } catch (e) {
        console.error('select project error:', e);
        notify('加载项目失败', e.message, 'error');
      }
    },

    async saveCurrentProject() {
      if (!state.currentProject) return;
      try {
        await api.updateProject(state.currentProject);
        notify('作品设定已保存', '基础信息与规则已保存', 'success');
      } catch (e) {
        notify('保存作品失败', e.message, 'error');
      }
    },

    async saveFramework() {
      if (!state.currentProject?.framework) return;
      try {
        await api.updateFramework(state.currentProject.id, state.currentProject.framework);
        notify('创世总纲已保存', '天道公理与战力阶梯已同步', 'success');
      } catch (e) {
        notify('保存创世总纲失败', e.message, 'error');
      }
    },

    async bootstrapCurrentFramework(concept) {
      if (!state.currentProject) return;
      state.isLoading = true;
      try {
        const c = concept || state.currentProject.framework?.core_concept || state.currentProject.title;
        state.currentProject.framework = await api.bootstrapFramework(state.currentProject.id, c);
        notify('创世推演完成', '天道法则与战力天平已自动构建', 'success');
      } catch (e) {
        notify('创世推演失败', e.message, 'error');
      } finally {
        state.isLoading = false;
      }
    },

    async createPlotHook(title, details, targetChapter) {
      if (!state.currentProject) return;
      try {
        const newHook = {
          title: title || '未命名伏笔',
          details: details || '',
          created_chapter: state.chapters.length + 1,
          target_chapter: targetChapter || state.chapters.length + 3,
          status: 'OPEN',
        };
        await api.createHook(state.currentProject.id, newHook);
        state.hooks = await api.listHooks(state.currentProject.id);
        notify('伏笔已记录', '已添加到伏笔簿', 'success');
      } catch (e) {
        notify('新建伏笔失败', e.message, 'error');
      }
    },

    async updatePlotHook(hook) {
      if (!state.currentProject) return;
      try {
        await api.updateHook(state.currentProject.id, hook);
        notify('伏笔已更新', '', 'success', 2000);
      } catch (e) {
        notify('更新伏笔失败', e.message, 'error');
      }
    },

    async deletePlotHook(id) {
      if (dialogs) {
        const ok = await dialogs.confirm({
          title: '删除伏笔',
          message: '确定删除该伏笔吗？删除后相关关联将被清除。',
          type: 'danger',
          confirmText: '确认删除',
        });
        if (!ok) return;
      }
      try {
        await api.deleteHook(id);
        state.hooks = state.hooks.filter(h => h.id !== id);
        notify('伏笔已删除', '', 'info', 2000);
      } catch (e) {
        notify('删除伏笔失败', e.message, 'error');
      }
    },

    async revertChapterToDraft(chapterIndex) {
      if (!state.currentProject) return;
      if (dialogs) {
        const ok = await dialogs.confirm({
          title: `撤回第 ${chapterIndex} 章为草稿`,
          message: `确定将第 ${chapterIndex} 章撤回为草稿吗？\n\n该操作将：\n1. 从已定稿章节中移出本章\n2. 恢复本章之前的人物状态\n3. 将本章正文与分段放回工作台\n4. 保留体检建议供重新打磨`,
          type: 'warning',
          confirmText: '确认撤回',
        });
        if (!ok) return;
      }
      state.isLoading = true;
      try {
        const res = await api.uncommitChapter(state.currentProject.id, chapterIndex);
        state.chapters = await api.listChapters(state.currentProject.id);
        state.hooks = await api.listHooks(state.currentProject.id);
        state.currentProject = await api.getProject(state.currentProject.id);

        if (res.checkpoint) {
          state.workbench.content = res.checkpoint.draft_text || '';
          state.workbench.beats = res.checkpoint.beats || [];
          state.workbench.coreConflict = res.checkpoint.core_conflict || '';
          state.workbench.stateMutation = res.checkpoint.state_mutation || { inventory_delta: '', power_delta: '' };
          state.reviewResult = res.checkpoint.audit_report || null;
          if (res.checkpoint.audit_report?.linter) {
            state.linterReport = {
              ...res.checkpoint.audit_report.linter,
              hit_banned_words: Array.isArray(res.checkpoint.audit_report.linter.hit_banned_words) ? res.checkpoint.audit_report.linter.hit_banned_words : [],
              top_repeated_ngrams: Array.isArray(res.checkpoint.audit_report.linter.top_repeated_ngrams) ? res.checkpoint.audit_report.linter.top_repeated_ngrams : [],
            };
          }
        }
        state.editingChapterIndex = chapterIndex;
        state.activeTab = 'workbench';
        state.activeStep = state.reviewResult ? 5 : 3;
        notify('已撤回为草稿', `第 ${chapterIndex} 章已恢复为草稿，可重新修改`, 'success');
      } catch (e) {
        console.error('revert chapter to draft error:', e);
        notify('撤回归档失败', e.message, 'error');
      } finally {
        state.isLoading = false;
      }
    },

    loadChapterToWorkbench(chapter) {
      if (!chapter) return;
      state.workbench.content = chapter.content || '';
      state.workbench.beats = chapter.beats || [];
      state.workbench.coreConflict = chapter.core_conflict || '';
      state.workbench.stateMutation = chapter.state_mutation || { inventory_delta: '', power_delta: '' };
      state.reviewResult = null; // 载入工坊重修时清空历史终审，要求修改后重新触发质检
      state.editingChapterIndex = chapter.chapter_index;
      state.activeTab = 'workbench';
      state.activeStep = 3; // 定位至手稿编辑画布
      notify('已载入工作台', `第 ${chapter.chapter_index} 章已载入，可直接修改润色`, 'success');
    },
  };
}
