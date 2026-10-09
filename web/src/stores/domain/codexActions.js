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
          title: '删除百科实体',
          message: '确定删除该百科设定实体吗？删除后此词条的属性与关联将无法恢复。',
          type: 'danger',
          confirmText: '确认删除',
        });
        if (!ok) return;
      }
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
