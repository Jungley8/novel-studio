import { api } from '../../api/client';

export function createMatrixActions(state, notify, helpers, dialogs) {
  return {
    async loadMatrixOverview() {
      if (!state.currentProject) return;
      try {
        state.matrixOverview = await api.getMatrixOverview(state.currentProject.id);
      } catch (e) {
        console.error('load matrix error:', e);
      }
    },

    async deleteScene(sceneId) {
      if (dialogs) {
        const ok = await dialogs.confirm({
          title: '删除剧情场次',
          message: '确定删除该场次吗？删除后此场次内的情节点与角色变迁记录将同步移除。',
          type: 'danger',
          confirmText: '确认删除',
        });
        if (!ok) return;
      }
      try {
        await api.deleteScene(sceneId);
        if (helpers && helpers.loadMatrixOverview) {
          await helpers.loadMatrixOverview();
        } else {
          await this.loadMatrixOverview();
        }
        notify('场次已删除', '', 'info', 2000);
      } catch (e) {
        notify('删除场次失败', e.message, 'error');
      }
    },

    async loadAnalytics() {
      if (!state.currentProject) return;
      try {
        state.analyticsHeatmap = await api.getAnalyticsHeatmap(state.currentProject.id);
        state.analyticsTension = await api.getAnalyticsTension(state.currentProject.id);
      } catch (e) {
        console.error('load analytics error:', e);
      }
    },
  };
}
