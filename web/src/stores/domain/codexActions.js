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

    async generateCodexEntry(data) {
      if (!state.currentProject) return null;
      state.isLoading = true;
      try {
        const entry = await api.generateCodexEntry(state.currentProject.id, data);
        state.codexEntries.unshift(entry);
        notify('词条生成成功', `已收录【${entry.name}】至设定集`, 'success');
        return entry;
      } catch (e) {
        notify('AI 生成词条失败', e.message, 'error');
        return null;
      } finally {
        state.isLoading = false;
      }
    },

    async loadCodexRelations() {
      if (!state.currentProject) return;
      try {
        state.codexRelations = await api.listCodexRelations(state.currentProject.id);
      } catch (e) {
        console.error('load relations error:', e);
      }
    },

    async createCodexRelation(data) {
      if (!state.currentProject) return;
      try {
        const rel = await api.createCodexRelation(state.currentProject.id, data);
        state.codexRelations.push(rel);
        notify('关系已建立', '', 'success', 2000);
        return rel;
      } catch (e) {
        notify('建立关系失败', e.message, 'error');
        return null;
      }
    },

    async deleteCodexRelation(relId) {
      if (!state.currentProject) return;
      try {
        await api.deleteCodexRelation(state.currentProject.id, relId);
        state.codexRelations = state.codexRelations.filter(r => r.id !== relId);
        notify('关系已删除', '', 'info', 2000);
      } catch (e) {
        notify('删除关系失败', e.message, 'error');
      }
    },

    async extractCodexRelations() {
      if (!state.currentProject) return;
      state.isLoading = true;
      try {
        const newRels = await api.extractCodexRelations(state.currentProject.id);
        await this.loadCodexRelations();
        notify('关系图谱推演完成', `已智能识别并更新 ${newRels.length} 条实体关系`, 'success');
      } catch (e) {
        notify('AI 推演关系失败', e.message, 'error');
      } finally {
        state.isLoading = false;
      }
    },
  };
}
