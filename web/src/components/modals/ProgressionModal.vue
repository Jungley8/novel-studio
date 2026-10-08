<template>
  <div 
    v-if="state.showProgressionModal" 
    class="fixed inset-0 bg-black/80 backdrop-blur-sm flex items-center justify-center p-4 z-50">
    
    <div class="bg-atelier-900 border border-atelier-750 rounded-2xl max-w-lg w-full p-5 space-y-4 shadow-2xl animate-fade-in">
      <div class="flex items-center justify-between border-b border-atelier-750 pb-3">
        <div class="flex items-center gap-2">
          <GitCommit class="w-4 h-4 text-brand-amber" />
          <h3 class="text-sm font-serif font-bold text-ink-50">
            演进阶段管理 - {{ state.activeCodexForSub?.name }}
          </h3>
        </div>
        <button 
          @click="state.showProgressionModal = false" 
          class="text-ink-400 hover:text-ink-100 p-1 rounded transition cursor-pointer">
          <X class="w-4 h-4" />
        </button>
      </div>

      <!-- 现有演进阶段时间线 -->
      <div class="space-y-2.5 max-h-52 overflow-y-auto">
        <div 
          v-for="p in state.activeCodexForSub?.progressions" 
          :key="p.id" 
          class="p-3 bg-atelier-950 rounded-lg border border-atelier-800 text-xs space-y-1">
          <div class="flex items-center justify-between font-mono font-bold text-brand-amber">
            <span>第 {{ p.active_from_chapter }} 章起生效</span>
          </div>
          <p class="text-ink-200 font-serif leading-relaxed">{{ p.notes }}</p>
        </div>

        <div 
          v-if="!state.activeCodexForSub?.progressions || state.activeCodexForSub.progressions.length === 0" 
          class="text-center py-6 text-ink-500 text-xs">
          暂无动态阶段演进记录
        </div>
      </div>

      <!-- 新增阶段表单 -->
      <div class="p-3 bg-atelier-950 rounded-xl border border-atelier-800 space-y-2.5">
        <div class="flex flex-col sm:flex-row gap-2">
          <input 
            type="number" 
            v-model.number="form.active_from_chapter" 
            class="w-full sm:w-28 bg-atelier-850 border border-atelier-750 rounded-lg px-2.5 py-1.5 text-xs text-ink-100 font-mono focus:outline-none focus:border-brand-amber/60" 
            placeholder="生效起始章">
          <input 
            v-model="form.notes" 
            class="flex-1 bg-atelier-850 border border-atelier-750 rounded-lg px-2.5 py-1.5 text-xs text-ink-100 focus:outline-none focus:border-brand-amber/60 font-serif" 
            placeholder="该阶段状态说明 (如: 突破至筑基中期，断臂重塑，心境转入杀戮道)">
        </div>
        <div class="flex justify-end">
          <button 
            @click="addProgression" 
            :disabled="!form.notes.trim()"
            class="px-3.5 py-1 bg-brand-amber hover:bg-brand-amber-hover text-atelier-950 font-bold text-xs rounded-md shadow-amber-glow transition disabled:opacity-50 cursor-pointer">
            添加演进阶段
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { reactive } from 'vue';
import { state, actions, notify } from '../../stores/appState';
import { api } from '../../api/client';
import { GitCommit, X } from 'lucide-vue-next';

const form = reactive({
  active_from_chapter: 1,
  notes: '',
});

async function addProgression() {
  if (!form.notes.trim() || !state.activeCodexForSub || !state.currentProject) return;
  try {
    const entryId = state.activeCodexForSub.id;
    await api.createCodexProgression(state.currentProject.id, entryId, { ...form });
    await actions.loadCodexEntries();
    // Update active entry
    state.activeCodexForSub = state.codexEntries.find(e => e.id === entryId) || state.activeCodexForSub;
    form.notes = '';
    notify('演进阶段已记录', '', 'success', 2000);
  } catch (e) {
    notify('添加阶段失败', e.message, 'error');
  }
}
</script>
