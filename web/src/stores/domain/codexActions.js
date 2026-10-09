import { api } from '../../api/client';

export function createCodexActions(state, notify, dialogs) {
  return {
    async loadCodexEntries() {
      if (!state.currentProject) return;
      try {
        state.codexEntries = await api.listCodexEntries(state.currentProject.id);
      } catch (e) {
        console.error('load codex error:', e);
      }
    },

    async deleteCodexEntry(entryId) {
      if (dialogs) {
        const ok = await dialogs.confirm({
          title: '删除词条',
          message: '确定删除该设定词条吗？删除后内容将无法恢复。',
          type: 'danger',
          confirmText: '确认删除',
        });
        if (!ok) return;
      }
      try {
        await api.deleteCodexEntry(state.currentProject.id, entryId);
        state.codexEntries = state.codexEntries.filter(e => e.id !== entryId);
        notify('词条已删除', '', 'info', 2000);
      } catch (e) {
        notify('删除词条失败', e.message, 'error');
      }
    },
  };
}
