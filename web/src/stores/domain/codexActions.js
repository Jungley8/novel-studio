import { api } from '../../api/client';

export function createCodexActions(state, notify) {
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
      if (!confirm('确定删除该百科实体吗？')) return;
      try {
        await api.deleteCodexEntry(state.currentProject.id, entryId);
        state.codexEntries = state.codexEntries.filter(e => e.id !== entryId);
        notify('百科实体已删除', '', 'info', 2000);
      } catch (e) {
        notify('删除实体失败', e.message, 'error');
      }
    },
  };
}
