<template>
    <!-- 手动新增关联模态弹窗 -->
    <div 
      v-if="showCreateRelationModal" 
      class="fixed inset-0 bg-black/80 backdrop-blur-sm flex items-center justify-center p-4 z-50">
      <div class="bg-atelier-900 border border-atelier-750 rounded-2xl max-w-md w-full p-5 space-y-4 shadow-2xl animate-fade-in">
        <div class="flex items-center justify-between border-b border-atelier-750 pb-3">
          <div class="flex items-center gap-2">
            <Share2 class="w-4 h-4 text-brand-amber" />
            <h3 class="text-sm font-serif font-bold text-ink-50">新增实体关联</h3>
          </div>
          <button 
            @click="$emit('close')" 
            class="text-ink-400 hover:text-ink-100 p-1 rounded transition cursor-pointer">
            <X class="w-4 h-4" />
          </button>
        </div>

        <div class="space-y-3 text-xs">
          <div>
            <label class="text-[11px] text-ink-400 block mb-1">源实体 (主体)：</label>
            <select 
              v-model="createRelationForm.source_entry_id"
              class="w-full bg-atelier-950 border border-atelier-750 rounded-lg px-2.5 py-1.5 text-xs text-ink-100 focus:outline-none focus:border-brand-amber/60">
              <option value="">-- 选择源实体 --</option>
              <option v-for="e in allGraphEntries" :key="e.id" :value="e.id">
                {{ e.name }} ({{ formatCategoryName(e.category) }})
              </option>
            </select>
          </div>

          <div>
            <label class="text-[11px] text-ink-400 block mb-1">目标实体 (客体)：</label>
            <select 
              v-model="createRelationForm.target_entry_id"
              class="w-full bg-atelier-950 border border-atelier-750 rounded-lg px-2.5 py-1.5 text-xs text-ink-100 focus:outline-none focus:border-brand-amber/60">
              <option value="">-- 选择目标实体 --</option>
              <option 
                v-for="e in targetCandidateEntries" 
                :key="e.id" 
                :value="e.id">
                {{ e.name }} ({{ formatCategoryName(e.category) }})
              </option>
            </select>
          </div>

          <div>
            <label class="text-[11px] text-ink-400 block mb-1">关系类别：</label>
            <div class="grid grid-cols-3 gap-1.5 mb-1.5">
              <button 
                v-for="relPreset in relationPresets" 
                :key="relPreset"
                type="button"
                @click="createRelationForm.relation_type = relPreset"
                :class="createRelationForm.relation_type === relPreset ? 'bg-brand-amber/20 text-brand-amber border-brand-amber/40' : 'bg-atelier-850 text-ink-400 border-atelier-750'"
                class="px-2 py-1 rounded border text-[11px] font-mono transition cursor-pointer">
                {{ relPreset }}
              </button>
            </div>
            <input 
              v-model="createRelationForm.relation_type" 
              placeholder="自定义关系标签 (如: 执掌 / 誓约守护)"
              class="w-full bg-atelier-950 border border-atelier-750 rounded-lg px-2.5 py-1.5 text-xs text-ink-100 focus:outline-none focus:border-brand-amber/60 font-mono">
          </div>

          <div>
            <label class="text-[11px] text-ink-400 block mb-1">关系背景说明：</label>
            <textarea 
              v-model="createRelationForm.description" 
              rows="3" 
              placeholder="例如：生死大仇，三年前师门被灭之主谋..."
              class="w-full bg-atelier-950 border border-atelier-750 rounded-lg p-2.5 text-xs text-ink-100 font-serif focus:outline-none focus:border-brand-amber/60 resize-none"></textarea>
          </div>
        </div>

        <div class="flex items-center justify-end gap-2 pt-2 border-t border-atelier-750">
          <button 
            @click="$emit('close')" 
            class="px-3.5 py-1.5 bg-atelier-850 hover:bg-atelier-800 text-ink-300 text-xs rounded-lg transition cursor-pointer">
            取消
          </button>
          <button 
            @click="submitCreateRelation" 
            :disabled="!createRelationForm.source_entry_id || !createRelationForm.target_entry_id || !createRelationForm.relation_type.trim()"
            class="px-4 py-1.5 bg-brand-amber hover:bg-brand-amber-hover text-atelier-950 font-bold text-xs rounded-lg shadow-amber-glow transition disabled:opacity-50 cursor-pointer">
            建立关联
          </button>
        </div>
      </div>
    </div>
</template>

<script setup>
import { ref, computed, watch } from 'vue';
import { actions } from '../../stores/appState';
import { Share2, X } from 'lucide-vue-next';

const props = defineProps({
  showCreateRelationModal: {
    type: Boolean,
    default: false,
  },
  sourceId: {
    type: String,
    default: '',
  },
  allGraphEntries: {
    type: Array,
    default: () => [],
  },
  formatCategoryName: {
    type: Function,
    required: true,
  },
});

const emit = defineEmits(['close', 'submitted']);

const relationPresets = ['宿敌', '盟友', '师徒', '从属', '执掌', '亲族'];

const createRelationForm = ref({
  source_entry_id: '',
  target_entry_id: '',
  relation_type: 'ALLY',
  description: '',
});

watch(
  [() => props.showCreateRelationModal, () => props.sourceId],
  ([show, srcId]) => {
    if (show) {
      createRelationForm.value = {
        source_entry_id: srcId || (props.allGraphEntries[0]?.id || ''),
        target_entry_id: '',
        relation_type: 'ALLY',
        description: '',
      };
    }
  },
  { immediate: true }
);

const targetCandidateEntries = computed(() => {
  return (props.allGraphEntries || []).filter(e => e.id !== createRelationForm.value.source_entry_id);
});

async function submitCreateRelation() {
  if (!createRelationForm.value.source_entry_id || !createRelationForm.value.target_entry_id) return;
  await actions.createCodexRelation({
    source_entry_id: createRelationForm.value.source_entry_id,
    target_entry_id: createRelationForm.value.target_entry_id,
    relation_type: createRelationForm.value.relation_type.trim(),
    description: createRelationForm.value.description.trim(),
  });
  await actions.loadCodexRelations();
  emit('close');
  emit('submitted');
}
</script>
