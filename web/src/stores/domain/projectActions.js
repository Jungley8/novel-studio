import { api } from '../../api/client';

export function createProjectActions(state, notify, helpers) {
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
        notify('项目设定已保存', '实体物理状态与规则已持久化', 'success');
      } catch (e) {
        notify('保存项目失败', e.message, 'error');
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
        notify('伏笔已记录', '因果账本已更新', 'success');
      } catch (e) {
        notify('新建伏笔失败', e.message, 'error');
      }
    },

    async updatePlotHook(hook) {
      if (!state.currentProject) return;
      try {
        await api.updateHook(state.currentProject.id, hook);
        notify('伏笔状态已更新', '', 'success', 2000);
      } catch (e) {
        notify('更新伏笔失败', e.message, 'error');
      }
    },

    async deletePlotHook(id) {
      if (!confirm('确定删除该伏笔记录吗？')) return;
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
      if (!confirm(`确定将第 ${chapterIndex} 章撤回为草稿吗？\n\n该操作将：\n1. 从全本已归档正史中移出该章\n2. 回滚本章对主角战力与背包物品的变迁账本\n3. 将本章成稿与因果节拍恢复为工坊草稿\n4. 还原在途质检报告供重新返工打磨`)) {
        return;
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
            state.linterReport = res.checkpoint.audit_report.linter;
          }
        }
        state.editingChapterIndex = chapterIndex;
        state.activeTab = 'workbench';
        state.activeStep = state.reviewResult ? 5 : 3;
        notify('已撤回为草稿', `第 ${chapterIndex} 章已移出正史并恢复为工作台草稿`, 'success');
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
      state.reviewResult = chapter.review || null;
      state.editingChapterIndex = chapter.chapter_index;
      state.activeTab = 'workbench';
      state.activeStep = chapter.review ? 5 : 3;
      notify('已载入工作台', `第 ${chapter.chapter_index} 章成稿已填入画布`, 'success');
    },
  };
}
