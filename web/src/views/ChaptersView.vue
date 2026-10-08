<template>
  <div class="flex-1 p-6 overflow-y-auto space-y-6">
    <!-- 头部工具栏 -->
    <div class="flex flex-col md:flex-row md:items-center justify-between gap-4 border-b border-atelier-750 pb-5">
      <div>
        <div class="flex items-center gap-2.5">
          <div class="w-8 h-8 rounded-lg bg-brand-amber/10 border border-brand-amber/25 flex items-center justify-center text-brand-amber shadow-amber-glow">
            <Archive class="w-4 h-4" />
          </div>
          <div>
            <h2 class="text-base font-serif font-bold text-ink-50 tracking-wide">全本已归档正史 (Canon History)</h2>
            <p class="text-xs text-ink-400 mt-0.5">历次因果封存章节、质检裁决报告与成书正文阅览室</p>
          </div>
        </div>
      </div>

      <div class="flex items-center gap-3">
        <div class="flex items-center gap-2 text-xs font-mono bg-atelier-850 px-3 py-1.5 rounded-md border border-atelier-750">
          <span class="text-ink-400">已归档</span>
          <strong class="text-brand-amber">{{ state.chapters.length }}</strong>
          <span class="text-ink-400">章</span>
          <span class="text-atelier-700">|</span>
          <span class="text-ink-400">总字数</span>
          <strong class="text-brand-amber">{{ totalWords.toLocaleString() }}</strong>
        </div>

        <button 
          @click="toggleExpandAll" 
          class="px-3 py-1.5 bg-atelier-850 hover:bg-atelier-800 text-ink-300 text-xs rounded-md border border-atelier-750 transition cursor-pointer">
          {{ isAllExpanded ? '收起全部正文' : '展开全部正文' }}
        </button>
      </div>
    </div>

    <!-- 章节卡片列表 -->
    <div class="space-y-4">
      <div 
        v-for="c in state.chapters" 
        :key="c.id" 
        class="p-5 bg-atelier-900 border border-atelier-750 rounded-xl space-y-3.5 shadow-atelier-sm hover:border-brand-amber/30 transition">
        
        <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-2.5 border-b border-atelier-800 pb-3">
          <div class="flex items-center gap-3">
            <span class="px-2.5 py-0.5 bg-brand-amber/15 text-brand-amber text-xs font-mono font-bold rounded border border-brand-amber/30">
              第 {{ c.chapter_index }} 章
            </span>
            <h3 class="text-sm font-serif font-bold text-ink-50 tracking-wide">{{ c.title }}</h3>
          </div>

          <div class="flex items-center gap-3 text-xs">
            <span 
              v-if="c.review" 
              class="px-2 py-0.5 rounded font-mono text-[11px] font-medium" 
              :class="c.review.verdict === 'ACCEPTED' 
                ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20' 
                : 'bg-rose-500/10 text-rose-400 border border-rose-500/20'">
              主审: {{ c.review.score }}分 ({{ c.review.verdict }})
            </span>
            <span class="text-ink-400 font-mono text-[11px]">{{ c.word_count }} 字</span>
            <span class="text-ink-400 font-mono text-[11px]">突发度 {{ c.burstiness_score || 50 }} 分</span>
          </div>
        </div>

        <div v-if="c.core_conflict" class="text-xs text-ink-300 flex items-start gap-2 bg-atelier-950/50 p-2.5 rounded-lg border border-atelier-800/80">
          <span class="text-brand-amber font-medium shrink-0">核心冲突:</span>
          <span>{{ c.core_conflict }}</span>
        </div>

        <!-- 正文展示区域 -->
        <div 
          :class="[
            'text-xs text-ink-100 bg-atelier-950 p-4 rounded-lg border border-atelier-800 font-serif leading-relaxed whitespace-pre-wrap transition-all',
            expandedMap[c.id] ? '' : 'line-clamp-4 max-h-28 overflow-hidden'
          ]">
          {{ c.content }}
        </div>

        <div class="flex justify-between items-center text-xs text-ink-400 pt-1">
          <span class="text-[10px] font-mono">归档时间: {{ formatDate(c.created_at) }}</span>
          <button 
            @click="toggleExpand(c.id)" 
            class="text-brand-amber hover:text-brand-amber-hover flex items-center gap-1 font-medium transition cursor-pointer">
            <span>{{ expandedMap[c.id] ? '收起正文' : '展开阅读完整章节' }}</span>
            <ChevronDown class="w-3.5 h-3.5 transition-transform" :class="expandedMap[c.id] ? 'rotate-180' : ''" />
          </button>
        </div>
      </div>
    </div>

    <!-- 空白状态 -->
    <div 
      v-if="state.chapters.length === 0" 
      class="text-center py-20 bg-atelier-900 border border-dashed border-atelier-750 rounded-xl space-y-3">
      <div class="w-12 h-12 rounded-full bg-atelier-800 flex items-center justify-center mx-auto text-ink-400">
        <Archive class="w-6 h-6" />
      </div>
      <h3 class="text-sm font-semibold text-ink-200">暂无已封存归档章节</h3>
      <p class="text-xs text-ink-400 max-w-md mx-auto">
        在“生产手稿”中完成节拍推演、正文渲染与反 AI 质检后，点击“因果封存归档”即可将成果正式收录入本史册。
      </p>
    </div>
  </div>
</template>

<script setup>
import { reactive, computed, ref } from 'vue';
import { state } from '../stores/appState';
import { Archive, ChevronDown } from 'lucide-vue-next';

const expandedMap = reactive({});
const isAllExpanded = ref(false);

const totalWords = computed(() => {
  return state.chapters.reduce((sum, c) => sum + (c.word_count || 0), 0);
});

function toggleExpand(id) {
  expandedMap[id] = !expandedMap[id];
}

function toggleExpandAll() {
  isAllExpanded.value = !isAllExpanded.value;
  state.chapters.forEach(c => {
    expandedMap[c.id] = isAllExpanded.value;
  });
}

function formatDate(isoStr) {
  if (!isoStr) return '未知时间';
  try {
    const d = new Date(isoStr);
    return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')} ${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`;
  } catch {
    return isoStr;
  }
}
</script>
