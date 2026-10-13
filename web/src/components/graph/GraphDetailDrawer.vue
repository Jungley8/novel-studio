<template>
        <!-- 右侧实体 / 关系详情抽屉 -->
        <transition 
          enter-active-class="transition duration-200 ease-out" 
          enter-from-class="translate-x-full opacity-0" 
          enter-to-class="translate-x-0 opacity-100" 
          leave-active-class="transition duration-150 ease-in" 
          leave-from-class="translate-x-0 opacity-100" 
          leave-to-class="translate-x-full opacity-0">
          <div 
            v-if="selectedNode || selectedLink" 
            @click.stop 
            class="absolute top-0 right-0 bottom-0 w-80 bg-atelier-900/95 border-l border-atelier-750 p-5 overflow-y-auto space-y-4 shadow-2xl backdrop-blur-md z-20">
            
            <!-- 实体详情展示 -->
            <div v-if="selectedNode" class="space-y-4">
              <div class="flex items-start justify-between border-b border-atelier-750 pb-3">
                <div class="flex items-center gap-2.5">
                  <span 
                    class="w-3.5 h-3.5 rounded-full ring-2 ring-atelier-800 shrink-0" 
                    :style="{ backgroundColor: selectedNode.color?.border || selectedNode.color }"></span>
                  <div>
                    <div class="flex items-center gap-1.5">
                      <h3 class="text-sm font-serif font-bold text-ink-50">{{ selectedNode.name }}</h3>
                      <span 
                        v-if="selectedNode.isProtagonist"
                        class="text-[9px] font-mono px-1.5 py-0.2 rounded bg-brand-amber/15 text-brand-amber border border-brand-amber/30 font-semibold">
                        主角
                      </span>
                    </div>
                    <span class="text-[10px] font-mono px-1.5 py-0.2 rounded bg-atelier-800 text-ink-300 border border-atelier-750">
                      {{ formatCategoryName(selectedNode.category) }}
                    </span>
                  </div>
                </div>
                <button 
                  @click="closeDetailDrawer" 
                  class="p-1 text-ink-400 hover:text-ink-100 rounded transition cursor-pointer">
                  <X class="w-4 h-4" />
                </button>
              </div>

              <!-- 摘要与别名 -->
              <div class="space-y-2 text-xs">
                <div v-if="selectedNode.raw?.summary" class="p-2.5 bg-atelier-950 rounded-lg border border-atelier-800 text-ink-300 leading-relaxed font-serif">
                  {{ selectedNode.raw.summary }}
                </div>

                <div v-if="selectedNode.raw?.aliases && selectedNode.raw.aliases.length > 0" class="flex flex-wrap gap-1">
                  <span 
                    v-for="al in selectedNode.raw.aliases" 
                    :key="al" 
                    class="text-[10px] font-mono bg-atelier-850 text-ink-400 px-1.5 py-0.5 rounded border border-atelier-800">
                    {{ al }}
                  </span>
                </div>
              </div>

              <!-- 当前实体关联线列表 -->
              <div class="space-y-2.5 pt-2 border-t border-atelier-750">
                <div class="flex items-center justify-between">
                  <span class="text-xs font-serif font-bold text-brand-amber">关联网络 ({{ nodeRelations.length }})</span>
                  <button 
                    @click="openCreateRelationModal(selectedNode.id)" 
                    class="text-[11px] text-brand-amber hover:text-brand-amber-hover flex items-center gap-1 cursor-pointer">
                    <Plus class="w-3 h-3" />
                    <span>添加关系</span>
                  </button>
                </div>

                <div class="space-y-2 max-h-60 overflow-y-auto">
                  <div 
                    v-for="rel in nodeRelations" 
                    :key="rel.id" 
                    class="p-2.5 bg-atelier-950 rounded-lg border border-atelier-800 text-xs space-y-1">
                    <div class="flex items-center justify-between">
                      <span class="font-medium text-ink-200">
                        {{ rel.source_entry_id === selectedNode.id ? '向' : '来自' }}
                        <strong class="text-brand-amber">【{{ getEntityName(rel.source_entry_id === selectedNode.id ? rel.target_entry_id : rel.source_entry_id) }}】</strong>
                      </span>
                      <button 
                        @click="deleteRelation(rel.id)" 
                        class="text-ink-500 hover:text-rose-400 p-0.5 transition cursor-pointer"
                        title="删除此关系">
                        <Trash2 class="w-3 h-3" />
                      </button>
                    </div>
                    <div class="flex items-center gap-1.5 text-[11px]">
                      <span class="font-mono text-emerald-400 px-1 rounded bg-emerald-500/10 border border-emerald-500/20 text-[10px]">
                        {{ formatRelationLabel(rel.relation_type) }}
                      </span>
                      <span class="text-ink-400 truncate">{{ rel.description || '无备注说明' }}</span>
                    </div>
                  </div>

                  <div v-if="nodeRelations.length === 0" class="text-center py-4 text-ink-500 text-xs">
                    暂无已建立的关联关系
                  </div>
                </div>
              </div>

              <!-- 跳转设定集编辑 -->
              <div class="pt-3 border-t border-atelier-750">
                <button 
                  v-if="selectedNode.isProtagonist"
                  @click="jumpToProtagonistEdit" 
                  class="w-full py-1.5 px-3 bg-brand-amber/10 hover:bg-brand-amber/20 text-brand-amber border border-brand-amber/30 rounded-lg text-xs font-medium transition flex items-center justify-center gap-1.5 cursor-pointer">
                  <User class="w-3.5 h-3.5" />
                  <span>前往总纲设定修改主角档案</span>
                </button>
                <button 
                  v-else
                  @click="jumpToCodexEdit(selectedNode.raw)" 
                  class="w-full py-1.5 px-3 bg-atelier-850 hover:bg-atelier-800 text-ink-200 rounded-lg text-xs font-medium border border-atelier-750 transition flex items-center justify-center gap-1.5 cursor-pointer">
                  <BookOpen class="w-3.5 h-3.5" />
                  <span>在设定集中查看/编辑完整设定</span>
                </button>
              </div>
            </div>

            <!-- 连线详情展示 -->
            <div v-else-if="selectedLink" class="space-y-4">
              <div class="flex items-start justify-between border-b border-atelier-750 pb-3">
                <div>
                  <h3 class="text-sm font-serif font-bold text-ink-50">关系脉络</h3>
                  <span class="text-[10px] font-mono text-brand-amber">定向实体关联</span>
                </div>
                <button 
                  @click="closeDetailDrawer" 
                  class="p-1 text-ink-400 hover:text-ink-100 rounded transition cursor-pointer">
                  <X class="w-4 h-4" />
                </button>
              </div>

              <div class="p-3 bg-atelier-950 rounded-xl border border-atelier-800 space-y-3 text-xs">
                <div class="flex items-center justify-between text-ink-100 font-serif">
                  <span class="font-bold text-brand-amber">【{{ getEntityName(selectedLink.source_entry_id) }}】</span>
                  <ArrowRight class="w-3.5 h-3.5 text-ink-400" />
                  <span class="font-bold text-emerald-400">【{{ getEntityName(selectedLink.target_entry_id) }}】</span>
                </div>

                <div>
                  <span class="text-[10px] text-ink-400 block mb-1">关系类别：</span>
                  <span class="px-2 py-0.5 rounded bg-brand-amber/15 text-brand-amber border border-brand-amber/30 text-xs font-mono font-bold">
                    {{ formatRelationLabel(selectedLink.relation_type) }}
                  </span>
                </div>

                <div>
                  <span class="text-[10px] text-ink-400 block mb-1">背景说明：</span>
                  <p class="text-ink-300 font-serif leading-relaxed">
                    {{ selectedLink.description || '暂无详细背景说明' }}
                  </p>
                </div>
              </div>

              <button 
                @click="deleteRelation(selectedLink.id)" 
                class="w-full py-1.5 px-3 bg-rose-500/10 hover:bg-rose-500/20 text-rose-400 border border-rose-500/30 rounded-lg text-xs font-medium transition flex items-center justify-center gap-1.5 cursor-pointer">
                <Trash2 class="w-3.5 h-3.5" />
                <span>删除此关联记录</span>
              </button>
            </div>
          </div>
        </transition>
</template>

<script setup>
import { state, actions } from '../../stores/appState';
import { 
  X, 
  Plus, 
  Trash2, 
  User, 
  BookOpen, 
  ArrowRight 
} from 'lucide-vue-next';

defineProps({
  selectedNode: {
    type: Object,
    default: null,
  },
  selectedLink: {
    type: Object,
    default: null,
  },
  nodeRelations: {
    type: Array,
    default: () => [],
  },
  closeDetailDrawer: {
    type: Function,
    required: true,
  },
  deleteRelation: {
    type: Function,
    required: true,
  },
  formatRelationLabel: {
    type: Function,
    required: true,
  },
  getEntityName: {
    type: Function,
    required: true,
  },
  openCreateRelationModal: {
    type: Function,
    required: true,
  },
  formatCategoryName: {
    type: Function,
    required: true,
  },
});

function jumpToProtagonistEdit() {
  actions.setActiveTab('framework');
}

function jumpToCodexEdit(rawEntry) {
  if (rawEntry?.isProtagonistNode && rawEntry?.isVirtual) {
    actions.setActiveTab('framework');
    return;
  }
  state.editingCodexEntry = { ...rawEntry };
  actions.setActiveTab('codex');
  state.showCodexModal = true;
}
</script>
