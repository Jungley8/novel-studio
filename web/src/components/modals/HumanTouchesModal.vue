<template>
  <div 
    v-if="state.showHumanTouchesModal" 
    class="fixed inset-0 bg-black/80 backdrop-blur-sm flex items-center justify-center p-4 z-50">
    
    <div class="bg-atelier-900 border border-atelier-750 rounded-2xl max-w-4xl w-full max-h-[90vh] flex flex-col overflow-hidden shadow-2xl animate-fade-in">
      <!-- 弹窗头部 -->
      <div class="p-4 border-b border-atelier-750 flex items-center justify-between bg-atelier-950/60">
        <div class="flex items-center gap-2.5">
          <div class="w-7 h-7 rounded-lg bg-brand-amber/15 border border-brand-amber/30 flex items-center justify-center text-brand-amber shadow-amber-glow">
            <HeartHandshake class="w-3.5 h-3.5" />
          </div>
          <div>
            <h3 class="text-sm font-serif font-bold text-ink-50 tracking-wide">人机协同人味杂质注入 (Human-in-the-Loop)</h3>
            <p class="text-[10px] text-ink-400">打破 AIGC 均匀概率分布，注入生理不适、世俗闲笔、市井口癖与断裂顿挫</p>
          </div>
        </div>

        <div class="flex items-center gap-2">
          <button 
            @click="fetchSuggestions" 
            :disabled="isLoading || !state.workbench.content" 
            class="px-2.5 py-1 text-xs bg-atelier-850 hover:bg-atelier-800 text-brand-amber rounded border border-atelier-750 transition flex items-center gap-1 cursor-pointer disabled:opacity-40">
            <RotateCcw class="w-3 h-3" :class="{ 'animate-spin': isLoading }" />
            <span>{{ isLoading ? '分析中...' : '重新提取建议' }}</span>
          </button>
          <button 
            @click="state.showHumanTouchesModal = false" 
            class="text-ink-400 hover:text-ink-100 p-1 rounded-md transition cursor-pointer">
            <X class="w-4 h-4" />
          </button>
        </div>
      </div>

      <!-- 科学原理提示横幅 -->
      <div class="p-3.5 bg-brand-amber/10 border-b border-brand-amber/20 text-xs text-brand-amber/90 flex items-start gap-2.5">
        <Sparkles class="w-4 h-4 shrink-0 mt-0.5 text-brand-amber" />
        <div class="space-y-0.5 text-[11px] leading-relaxed">
          <p class="font-semibold text-brand-amber">为什么商业检测助手（如腾讯朱雀）会打出 100% 疑似 AI？</p>
          <p class="text-ink-300">
            因为大模型生成的文字呈现高度均匀的 Token 似然度、对称逗号节奏以及绝对干净的推进逻辑。人类写作天然带有<strong>身体生理的抵抗、世俗物质的琐碎摩擦、市井方言的粗粝口癖与残破的断句</strong>。在手稿关键节点采纳并插入以下细节，可直接击碎检测器的概率规律。
          </p>
        </div>
      </div>

      <!-- 分类标签过滤器 -->
      <div class="px-5 py-2.5 bg-atelier-950 border-b border-atelier-800 flex items-center gap-2 overflow-x-auto">
        <button 
          v-for="cat in categories" 
          :key="cat.id" 
          @click="activeCategory = cat.id"
          class="px-3 py-1 rounded-full text-xs font-medium transition cursor-pointer shrink-0"
          :class="activeCategory === cat.id ? 'bg-brand-amber text-atelier-950 font-bold' : 'bg-atelier-850 hover:bg-atelier-800 text-ink-300 border border-atelier-750'">
          {{ cat.label }} ({{ countByCategory(cat.id) }})
        </button>
      </div>

      <!-- 建议列表 -->
      <div class="flex-1 p-6 overflow-y-auto space-y-4">
        <div v-if="isLoading" class="p-16 text-center text-ink-400 text-xs flex flex-col items-center justify-center gap-2">
          <Loader2 class="w-5 h-5 text-brand-amber animate-spin" />
          <span>正在对当前手稿全文进行因果熵与人味杂质扫描...</span>
        </div>

        <div v-else-if="filteredSuggestions.length === 0" class="p-12 text-center text-ink-400 text-xs flex flex-col items-center justify-center gap-2">
          <Check class="w-6 h-6 text-brand-emerald" />
          <span>当前筛选分类下暂无建议，或正文篇幅较短。可切换全部分类或添加更多正文后重试。</span>
        </div>

        <div v-else class="space-y-3">
          <div 
            v-for="(sug, idx) in filteredSuggestions" 
            :key="idx" 
            class="p-4 bg-atelier-950 rounded-xl border border-atelier-800 space-y-2.5 transition hover:border-atelier-700">
            <!-- 头部元信息 -->
            <div class="flex items-center justify-between gap-2">
              <div class="flex items-center gap-2">
                <span class="text-[10px] font-mono font-bold px-2 py-0.5 rounded border" :class="categoryBadgeClass(sug.category)">
                  {{ categoryName(sug.category) }}
                </span>
                <span class="text-[11px] font-serif text-ink-400">
                  语境: <span class="italic text-ink-300">“{{ sug.original_context }}”</span>
                </span>
              </div>
              <div class="flex items-center gap-2">
                <button 
                  @click="copySuggestion(sug.suggestion)" 
                  class="px-2 py-1 text-[11px] bg-atelier-850 hover:bg-atelier-800 text-ink-300 rounded border border-atelier-750 transition flex items-center gap-1 cursor-pointer">
                  <Copy class="w-3 h-3" />
                  <span>复制</span>
                </button>
                <button 
                  @click="insertSuggestion(sug)" 
                  class="px-3 py-1 text-[11px] bg-brand-amber hover:bg-brand-amber-hover text-atelier-950 font-bold rounded transition flex items-center gap-1 cursor-pointer shadow-atelier-sm">
                  <Plus class="w-3 h-3" />
                  <span>采纳并插入正文</span>
                </button>
              </div>
            </div>

            <!-- 建议具体插入正文内容 -->
            <div class="p-3 bg-atelier-900/80 rounded-lg border border-atelier-750 text-xs text-ink-100 font-serif leading-relaxed select-text">
              {{ sug.suggestion }}
            </div>

            <!-- 破防原理提示 -->
            <div class="text-[10px] text-ink-400 flex items-center gap-1.5 pt-0.5">
              <Sparkles class="w-3 h-3 text-brand-amber shrink-0" />
              <span><strong>破除原理：</strong>{{ sug.rationale }}</span>
            </div>
          </div>
        </div>
      </div>

      <!-- 底部关闭栏 -->
      <div class="p-3.5 border-t border-atelier-750 bg-atelier-950/60 flex items-center justify-between text-xs">
        <span class="text-[11px] text-ink-400">
          共生成 {{ suggestions.length }} 条人味建议 · 建议人工挑选 1~2 处注入即达破防临界点
        </span>
        <button 
          @click="state.showHumanTouchesModal = false" 
          class="px-4 py-1.5 bg-atelier-850 hover:bg-atelier-800 text-ink-200 rounded text-xs transition cursor-pointer">
          完成
        </button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch } from 'vue';
import { state, actions, notify } from '../../stores/appState';
import { api } from '../../api/client';
import { HeartHandshake, Sparkles, X, Plus, Copy, Check, RotateCcw, Loader2 } from 'lucide-vue-next';

const isLoading = ref(false);
const suggestions = ref([]);
const activeCategory = ref('ALL');

const categories = [
  { id: 'ALL', label: '全部建议' },
  { id: 'PHYSIOLOGY', label: '🩸 生理不适偏见' },
  { id: 'TRIVIALITY', label: '🍵 生活物质闲笔' },
  { id: 'COLLOQUIALISM', label: '🗣️ 市井口癖碎屑' },
  { id: 'CADENCE', label: '⚡ 标点节奏断裂' },
];

function categoryName(cat) {
  switch (cat) {
    case 'PHYSIOLOGY': return '生理不适偏见';
    case 'TRIVIALITY': return '生活物质闲笔';
    case 'COLLOQUIALISM': return '市井口癖碎屑';
    case 'CADENCE': return '标点节奏断裂';
    default: return cat;
  }
}

function categoryBadgeClass(cat) {
  switch (cat) {
    case 'PHYSIOLOGY': return 'bg-brand-rose/15 text-brand-rose border-brand-rose/30';
    case 'TRIVIALITY': return 'bg-brand-amber/15 text-brand-amber border-brand-amber/30';
    case 'COLLOQUIALISM': return 'bg-brand-cyan/15 text-brand-cyan border-brand-cyan/30';
    case 'CADENCE': return 'bg-brand-emerald/15 text-brand-emerald border-brand-emerald/30';
    default: return 'bg-atelier-800 text-ink-300 border-atelier-750';
  }
}

function countByCategory(catId) {
  if (catId === 'ALL') return suggestions.value.length;
  return suggestions.value.filter(s => s.category === catId).length;
}

const filteredSuggestions = computed(() => {
  if (activeCategory.value === 'ALL') return suggestions.value;
  return suggestions.value.filter(s => s.category === activeCategory.value);
});

watch(() => state.showHumanTouchesModal, async (open) => {
  if (open && state.currentProject && suggestions.value.length === 0) {
    await fetchSuggestions();
  }
});

async function fetchSuggestions() {
  if (!state.currentProject || !state.workbench.content.trim()) {
    return notify('手稿为空', '请先在手稿区生成或编写章节内容', 'info');
  }
  isLoading.value = true;
  try {
    const res = await api.suggestHumanTouches(state.currentProject.id, {
      content: state.workbench.content,
    });
    suggestions.value = res.suggestions || [];
    notify('人味建议已生成', `获得 ${suggestions.value.length} 条反检测杂质建议`, 'success');
  } catch (e) {
    notify('生成建议失败', e.message, 'error');
  } finally {
    isLoading.value = false;
  }
}

function insertSuggestion(sug) {
  const textarea = document.getElementById('prose-textarea');
  if (textarea && textarea.selectionStart !== undefined && textarea.selectionStart !== textarea.selectionEnd) {
    const start = textarea.selectionStart;
    const end = textarea.selectionEnd;
    state.workbench.content = state.workbench.content.substring(0, start) + sug.suggestion + state.workbench.content.substring(end);
  } else if (textarea && textarea.selectionStart !== undefined) {
    const pos = textarea.selectionStart;
    state.workbench.content = state.workbench.content.substring(0, pos) + '\n' + sug.suggestion + '\n' + state.workbench.content.substring(pos);
  } else {
    state.workbench.content = (state.workbench.content.trim() + '\n\n' + sug.suggestion).trim();
  }
  actions.runLinter();
  notify('已注入人味杂质', `${sug.suggestion.slice(0, 18)}... 已插入正文`, 'success');
}

function copySuggestion(text) {
  navigator.clipboard.writeText(text);
  notify('已复制到剪贴板', text.slice(0, 24) + '...', 'success');
}
</script>
