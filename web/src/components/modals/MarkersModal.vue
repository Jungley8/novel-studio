<template>
  <div 
    v-if="state.showMarkersModal" 
    class="fixed inset-0 bg-black/80 backdrop-blur-sm flex items-center justify-center p-4 z-50">
    
    <div class="bg-atelier-900 border border-atelier-750 rounded-2xl max-w-lg w-full p-5 space-y-4 shadow-2xl animate-fade-in">
      <div class="flex items-center justify-between border-b border-atelier-750 pb-3">
        <div class="flex items-center gap-2">
          <Bookmark class="w-4 h-4 text-brand-amber" />
          <h3 class="text-sm font-serif font-bold text-ink-50">
            场次批注备忘 - {{ state.activeSceneForMarkers?.title }}
          </h3>
        </div>
        <button 
          @click="state.showMarkersModal = false" 
          class="text-ink-400 hover:text-ink-100 p-1 rounded transition cursor-pointer">
          <X class="w-4 h-4" />
        </button>
      </div>

      <!-- 现有批注列表 -->
      <div class="space-y-2 max-h-52 overflow-y-auto">
        <div 
          v-for="m in state.sceneMarkersList" 
          :key="m.id" 
          class="p-3 bg-atelier-950 rounded-lg border border-atelier-800 flex items-center justify-between text-xs">
          <div class="flex items-center gap-2.5 min-w-0">
            <span 
              class="px-2 py-0.5 rounded text-[10px] font-mono font-bold shrink-0" 
              :class="m.marker_type === 'PLOT_HOLE' 
                ? 'bg-rose-500/15 text-rose-400 border border-rose-500/30' 
                : (m.marker_type === 'HOOK_ANCHOR' 
                  ? 'bg-purple-500/15 text-purple-400 border border-purple-500/30' 
                  : 'bg-brand-amber/15 text-brand-amber border border-brand-amber/30')">
              {{ m.marker_type }}
            </span>
            <span class="text-ink-200 truncate">{{ m.content }}</span>
          </div>
          <button 
            @click="deleteMarker(m.id)" 
            class="text-ink-500 hover:text-rose-400 text-xs transition cursor-pointer shrink-0">
            删除
          </button>
        </div>

        <div v-if="state.sceneMarkersList.length === 0" class="text-center py-6 text-ink-500 text-xs">
          暂无行内批注与创作备忘
        </div>
      </div>

      <!-- 新增批注表单 -->
      <div class="p-3 bg-atelier-950 rounded-xl border border-atelier-800 space-y-2.5">
        <div class="flex flex-col sm:flex-row gap-2">
          <select 
            v-model="newMarker.marker_type" 
            class="bg-atelier-850 border border-atelier-750 rounded-lg px-2.5 py-1.5 text-xs text-ink-100 focus:outline-none focus:border-brand-amber/60 cursor-pointer">
            <option value="TODO">待办 (TODO)</option>
            <option value="PLOT_HOLE">设定漏洞 (PLOT_HOLE)</option>
            <option value="HOOK_ANCHOR">伏笔锚点 (HOOK_ANCHOR)</option>
            <option value="NOTE">创作备忘 (NOTE)</option>
          </select>
          <input 
            v-model="newMarker.content" 
            class="flex-1 bg-atelier-850 border border-atelier-750 rounded-lg px-2.5 py-1.5 text-xs text-ink-100 focus:outline-none focus:border-brand-amber/60" 
            placeholder="批注内容说明...">
        </div>
        <div class="flex justify-end">
          <button 
            @click="addMarker" 
            :disabled="!newMarker.content.trim()"
            class="px-3.5 py-1 bg-brand-amber hover:bg-brand-amber-hover text-atelier-950 font-bold text-xs rounded-md shadow-amber-glow transition disabled:opacity-50 cursor-pointer">
            添加批注
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { reactive } from 'vue';
import { state, actions, notify, dialogs } from '../../stores/appState';
import { api } from '../../api/client';
import { Bookmark, X } from 'lucide-vue-next';

const newMarker = reactive({
  marker_type: 'TODO',
  content: '',
});

async function addMarker() {
  if (!newMarker.content.trim() || !state.activeSceneForMarkers) return;
  try {
    const scId = state.activeSceneForMarkers.id;
    await api.createSceneMarker(scId, { ...newMarker });
    state.sceneMarkersList = await api.listSceneMarkers(scId);
    newMarker.content = '';
    await actions.loadMatrixOverview();
    notify('批注已添加', '', 'success', 2000);
  } catch (e) {
    notify('添加批注失败', e.message, 'error');
  }
}

async function deleteMarker(markerId) {
  if (!state.activeSceneForMarkers) return;
  const ok = await dialogs.confirm({
    title: '删除场次批注',
    message: '确定删除该场次标记批注吗？',
    type: 'danger',
    confirmText: '确认删除',
  });
  if (!ok) return;
  try {
    const scId = state.activeSceneForMarkers.id;
    await api.deleteSceneMarker(scId, markerId);
    state.sceneMarkersList = state.sceneMarkersList.filter(m => m.id !== markerId);
    await actions.loadMatrixOverview();
    notify('批注已删除', '', 'info', 2000);
  } catch (e) {
    notify('删除批注失败', e.message, 'error');
  }
}
</script>
