<template>
  <div 
    v-if="state.showSceneModal" 
    class="fixed inset-0 bg-black/80 backdrop-blur-sm flex items-center justify-center p-4 z-50">
    
    <div class="bg-atelier-900 border border-atelier-750 rounded-2xl max-w-md w-full p-5 space-y-4 shadow-2xl animate-fade-in">
      <div class="flex items-center justify-between border-b border-atelier-750 pb-3">
        <h3 class="text-sm font-serif font-bold text-ink-50">
          {{ state.editingScene ? '编辑场景场次' : '新建场景场次' }}
        </h3>
        <button 
          @click="state.showSceneModal = false" 
          class="text-ink-400 hover:text-ink-100 p-1 rounded transition cursor-pointer">
          <X class="w-4 h-4" />
        </button>
      </div>

      <div class="space-y-3 text-xs">
        <div>
          <label class="text-ink-300 font-medium">场次标题：</label>
          <input 
            v-model="form.title" 
            class="w-full mt-1 bg-atelier-950 border border-atelier-750 rounded-lg p-2.5 text-xs text-ink-100 focus:outline-none focus:border-brand-amber/60" 
            placeholder="例如：城门斩将 / 暗夜探库">
        </div>

        <div>
          <label class="text-ink-300 font-medium">场次序号：</label>
          <input 
            type="number" 
            v-model.number="form.scene_index" 
            class="w-full mt-1 bg-atelier-950 border border-atelier-750 rounded-lg p-2.5 text-xs text-ink-100 font-mono focus:outline-none focus:border-brand-amber/60">
        </div>

        <div>
          <label class="text-ink-300 font-medium">戏剧目标 (Dramatic Goal)：</label>
          <input 
            v-model="form.dramatic_goal" 
            class="w-full mt-1 bg-atelier-950 border border-atelier-750 rounded-lg p-2.5 text-xs text-ink-100 focus:outline-none focus:border-brand-amber/60" 
            placeholder="主角在该场次试图达成的具体物理或心理目标">
        </div>

        <div>
          <label class="text-ink-300 font-medium">阻碍与冲突 (Conflict Barrier)：</label>
          <input 
            v-model="form.conflict_barrier" 
            class="w-full mt-1 bg-atelier-950 border border-atelier-750 rounded-lg p-2.5 text-xs text-ink-100 focus:outline-none focus:border-brand-amber/60" 
            placeholder="阻挡主角达标的反派、环境阻力或道德两难">
        </div>

        <div>
          <div class="flex justify-between items-center text-ink-300 mb-1">
            <span>戏剧张力 (1-10)：</span>
            <span class="font-bold font-mono text-brand-amber text-sm">{{ form.tension_level }} / 10</span>
          </div>
          <input 
            type="range" 
            min="1" 
            max="10" 
            v-model.number="form.tension_level" 
            class="w-full mt-1 accent-brand-amber cursor-pointer">
        </div>
      </div>

      <div class="flex justify-end gap-2.5 pt-3 border-t border-atelier-750">
        <button 
          @click="state.showSceneModal = false" 
          class="px-3.5 py-1.5 bg-atelier-850 hover:bg-atelier-800 text-xs text-ink-300 rounded-lg transition cursor-pointer">
          取消
        </button>
        <button 
          @click="saveScene" 
          class="px-4 py-1.5 bg-brand-amber hover:bg-brand-amber-hover text-atelier-950 font-bold text-xs rounded-lg shadow-amber-glow transition cursor-pointer">
          保存场次
        </button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { reactive, watch } from 'vue';
import { state, actions, notify } from '../../stores/appState';
import { api } from '../../api/client';
import { X } from 'lucide-vue-next';

const form = reactive({
  title: '',
  scene_index: 1,
  dramatic_goal: '',
  conflict_barrier: '',
  tension_level: 6,
});

watch(() => state.showSceneModal, (open) => {
  if (open) {
    if (state.editingScene) {
      Object.assign(form, {
        title: state.editingScene.title || '',
        scene_index: state.editingScene.scene_index || 1,
        dramatic_goal: state.editingScene.dramatic_goal || '',
        conflict_barrier: state.editingScene.conflict_barrier || '',
        tension_level: state.editingScene.tension_level || 6,
      });
    } else {
      Object.assign(form, {
        title: '',
        scene_index: 1,
        dramatic_goal: '',
        conflict_barrier: '',
        tension_level: 6,
      });
    }
  }
});

async function saveScene() {
  try {
    if (state.editingScene?.id) {
      await api.updateScene(state.editingScene.id, form);
      notify('场景场次已更新', form.title, 'success');
    } else {
      await api.createScene(state.currentProject.id, {
        ...form,
        chapter_id: state.targetChapterIdForNewScene,
      });
      notify('场景场次已创建', form.title, 'success');
    }
    await actions.loadMatrixOverview();
    state.showSceneModal = false;
  } catch (e) {
    notify('保存场次失败', e.message, 'error');
  }
}
</script>
