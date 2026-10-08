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
  };
}
