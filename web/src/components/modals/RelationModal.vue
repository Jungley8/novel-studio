<template>
  <div 
    v-if="state.showRelationModal" 
    class="fixed inset-0 bg-black/80 backdrop-blur-sm flex items-center justify-center p-4 z-50">
    
    <div class="bg-atelier-900 border border-atelier-750 rounded-2xl max-w-lg w-full p-5 space-y-4 shadow-2xl animate-fade-in">
      <div class="flex items-center justify-between border-b border-atelier-750 pb-3">
        <div class="flex items-center gap-2">
          <Share2 class="w-4 h-4 text-brand-amber" />
          <h3 class="text-sm font-serif font-bold text-ink-50">
            人物关系 - {{ state.activeCodexForSub?.name }}
          </h3>
        </div>
        <button 
          @click="state.showRelationModal = false" 
          class="text-ink-400 hover:text-ink-100 p-1 rounded transition cursor-pointer">
          <X class="w-4 h-4" />
        </button>
      </div>

      <!-- 现有关系列表 -->
      <div class="space-y-2.5 max-h-52 overflow-y-auto">
        <div 
          v-for="r in state.activeCodexForSub?.relations" 
          :key="r.id" 
          class="p-3 bg-atelier-950 rounded-lg border border-atelier-800 flex items-center justify-between text-xs">
          <div class="min-w-0 pr-2">
            <div class="flex items-center gap-1.5 font-medium">
              <span class="text-brand-amber font-serif font-bold">对【{{ r.target_name }}】：</span>
              <span class="text-[10px] font-mono px-1.5 py-0.2 rounded bg-atelier-850 text-ink-400 border border-atelier-750">
                {{ r.relation_type }}
              </span>
            </div>
            <p class="text-ink-200 mt-1 font-serif truncate">{{ r.description }}</p>
          </div>
          <button 
            @click="deleteRelation(r.id)" 
            class="text-ink-500 hover:text-rose-400 text-xs transition cursor-pointer shrink-0">
            删除
          </button>
        </div>

        <div 
          v-if="!state.activeCodexForSub?.relations || state.activeCodexForSub.relations.length === 0" 
          class="text-center py-6 text-ink-500 text-xs">
          暂无人物关系记录
        </div>
      </div>

      <!-- 新增关系表单 -->
      <div class="p-3 bg-atelier-950 rounded-xl border border-atelier-800 space-y-2.5">
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
          <select 
            v-model="form.target_entry_id" 
            class="bg-atelier-850 border border-atelier-750 rounded-lg px-2.5 py-1.5 text-xs text-ink-100 focus:outline-none focus:border-brand-amber/60 cursor-pointer">
            <option value="">-- 选择关联条目 --</option>
            <option 
              v-for="c in candidateEntries" 
              :key="c.id" 
              :value="c.id">
              {{ c.name }} ({{ c.category }})
            </option>
          </select>
          <input 
            v-model="form.relation_type" 
            class="bg-atelier-850 border border-atelier-750 rounded-lg px-2.5 py-1.5 text-xs text-ink-100 font-sans focus:outline-none focus:border-brand-amber/60" 
            placeholder="关系标签 (如 宿敌 / 盟友 / 师徒)">
        </div>

        <input 
          v-model="form.description" 
          class="w-full bg-atelier-850 border border-atelier-750 rounded-lg px-2.5 py-1.5 text-xs text-ink-100 focus:outline-none focus:border-brand-amber/60 font-serif" 
          placeholder="关系说明 (如: 曾有救命之恩、暗中同盟)">

        <div class="flex justify-end">
          <button 
            @click="addRelation" 
            :disabled="!form.target_entry_id || !form.description.trim()"
            class="px-3.5 py-1 bg-brand-amber hover:bg-brand-amber-hover text-atelier-950 font-bold text-xs rounded-md shadow-amber-glow transition disabled:opacity-50 cursor-pointer">
            添加关联
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { reactive, computed } from 'vue';
import { state, actions, notify, dialogs } from '../../stores/appState';
import { api } from '../../api/client';
import { Share2, X } from 'lucide-vue-next';

const form = reactive({
  target_entry_id: '',
  relation_type: 'ALLY',
  description: '',
});

const candidateEntries = computed(() => {
  if (!state.activeCodexForSub) return [];
  return state.codexEntries.filter(e => e.id !== state.activeCodexForSub.id);
});

async function addRelation() {
  if (!form.target_entry_id || !form.description.trim() || !state.activeCodexForSub || !state.currentProject) return;
  try {
    const sourceId = state.activeCodexForSub.id;
    await api.createCodexRelation(state.currentProject.id, {
      source_entry_id: sourceId,
      target_entry_id: form.target_entry_id,
      relation_type: form.relation_type,
      description: form.description,
    });
    await actions.loadCodexEntries();
    state.activeCodexForSub = state.codexEntries.find(e => e.id === sourceId) || state.activeCodexForSub;
    form.description = '';
    form.target_entry_id = '';
    notify('关系拓扑已绑定', '', 'success', 2000);
  } catch (e) {
    notify('绑定关系失败', e.message, 'error');
  }
}

async function deleteRelation(relationId) {
  if (!state.currentProject || !state.activeCodexForSub) return;
  const ok = await dialogs.confirm({
    title: '解除人物关系',
    message: '确定解除该人物关联关系拓扑吗？',
    type: 'danger',
    confirmText: '确认解除',
  });
  if (!ok) return;
  try {
    const sourceId = state.activeCodexForSub.id;
    await api.deleteCodexRelation(state.currentProject.id, relationId);
    await actions.loadCodexEntries();
    state.activeCodexForSub = state.codexEntries.find(e => e.id === sourceId) || state.activeCodexForSub;
    notify('关系已删除', '', 'info', 2000);
  } catch (e) {
    notify('删除关系失败', e.message, 'error');
  }
}
</script>
