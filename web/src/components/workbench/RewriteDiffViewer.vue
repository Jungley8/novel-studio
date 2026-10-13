<template>
  <div class="flex-1 flex flex-col h-full bg-atelier-950 overflow-hidden select-text font-mono text-xs">
    <!-- 头部工具栏 -->
    <div class="px-5 py-2.5 bg-atelier-900 border-b border-atelier-750 flex items-center justify-between shrink-0 select-none">
      <div class="flex items-center gap-3">
        <div class="w-7 h-7 rounded-lg bg-brand-amber/10 border border-brand-amber/25 flex items-center justify-center text-brand-amber shadow-amber-glow shrink-0">
          <GitCompare class="w-4 h-4" />
        </div>
        <div class="flex items-center gap-2 flex-wrap">
          <span class="font-serif font-bold text-sm text-ink-50">改写对比 (VSCode Diff)</span>
          <span class="text-[10px] font-mono px-2 py-0.5 rounded bg-atelier-800 text-ink-300 border border-atelier-750">
            原 {{ stats.oldWords }} 字 ➔ 新 {{ stats.newWords }} 字
          </span>
          <span 
            :class="stats.deltaWords >= 0 ? 'bg-emerald-500/15 text-emerald-300 border-emerald-500/30' : 'bg-rose-500/15 text-rose-300 border-rose-500/30'"
            class="text-[10px] font-mono font-bold px-1.5 py-0.5 rounded border">
            {{ stats.deltaWords >= 0 ? '+' + stats.deltaWords : stats.deltaWords }} 字
          </span>
          <span class="text-[10px] font-mono text-ink-400">
            +{{ stats.addedLines }} 行 / -{{ stats.deletedLines }} 行
          </span>
        </div>
      </div>

      <!-- 右侧控制区 -->
      <div class="flex items-center gap-2">
        <!-- 单流 / 双栏 切换 -->
        <div class="hidden sm:flex items-center bg-atelier-850 p-0.5 rounded-lg border border-atelier-750 text-[11px]">
          <button 
            @click="viewMode = 'unified'"
            :class="viewMode === 'unified' ? 'bg-brand-amber text-atelier-950 font-bold' : 'text-ink-300 hover:text-ink-100'"
            class="px-2 py-1 rounded transition cursor-pointer">
            统一流 (Unified)
          </button>
          <button 
            @click="viewMode = 'split'"
            :class="viewMode === 'split' ? 'bg-brand-amber text-atelier-950 font-bold' : 'text-ink-300 hover:text-ink-100'"
            class="px-2 py-1 rounded transition cursor-pointer">
            并排双栏 (Split)
          </button>
        </div>

        <button 
          @click="$emit('revert')" 
          class="px-3 py-1 bg-atelier-850 hover:bg-atelier-800 text-ink-300 hover:text-ink-100 border border-atelier-750 rounded-lg text-xs transition cursor-pointer flex items-center gap-1"
          title="放弃本次改写，恢复改写前原稿">
          <RotateCcw class="w-3.5 h-3.5" />
          <span>还原原文</span>
        </button>

        <button 
          @click="$emit('apply')" 
          class="px-3.5 py-1 bg-brand-emerald hover:bg-emerald-500 text-atelier-950 font-bold rounded-lg shadow-emerald-glow text-xs transition cursor-pointer flex items-center gap-1">
          <Check class="w-3.5 h-3.5" />
          <span>采用改写</span>
        </button>

        <button 
          @click="$emit('close')" 
          class="p-1 text-ink-400 hover:text-ink-100 transition cursor-pointer"
          title="关闭对比视图返回编辑">
          <X class="w-4 h-4" />
        </button>
      </div>
    </div>

    <!-- 主比对画布 -->
    <div class="flex-1 overflow-auto bg-atelier-950">
      <!-- 统一流模式 (Unified Diff) -->
      <div v-if="viewMode === 'unified'" class="divide-y divide-atelier-850/40 min-w-full">
        <div 
          v-for="(row, idx) in diffRows" 
          :key="idx"
          :class="[
            'flex items-start font-mono leading-relaxed px-3 py-0.5 text-xs select-text transition-colors',
            row.kind === 'add' ? 'bg-emerald-500/10 text-emerald-200 border-l-2 border-emerald-500' :
            row.kind === 'del' ? 'bg-rose-500/10 text-rose-300 border-l-2 border-rose-500' :
            'text-ink-300 hover:bg-atelier-900/40'
          ]">
          <!-- 行号栏 -->
          <div class="w-12 text-right text-ink-600 select-none pr-2 shrink-0 text-[10px]">
            {{ row.oldLineNum || '' }}
          </div>
          <div class="w-12 text-right text-ink-600 select-none pr-3 shrink-0 text-[10px]">
            {{ row.newLineNum || '' }}
          </div>
          <!-- 符号栏 -->
          <div class="w-4 select-none shrink-0 font-bold" :class="row.kind === 'add' ? 'text-emerald-400' : row.kind === 'del' ? 'text-rose-400' : 'text-ink-600'">
            {{ row.kind === 'add' ? '+' : row.kind === 'del' ? '−' : ' ' }}
          </div>
          <!-- 行正文 (支持词级高亮) -->
          <div class="flex-1 whitespace-pre-wrap break-words min-w-0 font-serif text-[13px] leading-relaxed">
            <template v-if="row.wordDiff && row.wordDiff.length">
              <span 
                v-for="(w, widx) in row.wordDiff" 
                :key="widx"
                :class="w.kind === 'del' ? 'bg-rose-500/30 text-rose-100 px-0.5 rounded line-through' : w.kind === 'add' ? 'bg-emerald-500/35 text-emerald-100 px-0.5 rounded font-semibold underline' : ''">
                {{ w.text }}
              </span>
            </template>
            <template v-else>
              {{ row.text }}
            </template>
          </div>
        </div>
      </div>

      <!-- 并排双栏模式 (Split Diff) -->
      <div v-else class="grid grid-cols-2 h-full divide-x divide-atelier-750">
        <!-- 左侧：原文 -->
        <div class="flex flex-col h-full overflow-hidden">
          <div class="px-3 py-1 bg-atelier-900 border-b border-atelier-800 text-[11px] font-bold text-rose-400 flex items-center justify-between shrink-0 select-none">
            <span>原文 (改写前)</span>
            <span class="text-ink-500">{{ stats.oldWords }} 字</span>
          </div>
          <div class="flex-1 overflow-auto divide-y divide-atelier-850/40">
            <div 
              v-for="(row, idx) in diffRows" 
              :key="'l-' + idx"
              v-show="row.kind !== 'add'"
              :class="[
                'flex items-start px-2 py-0.5 text-xs font-mono leading-relaxed',
                row.kind === 'del' ? 'bg-rose-500/10 text-rose-300 border-l-2 border-rose-500' : 'text-ink-300'
              ]">
              <span class="w-10 text-right text-ink-600 select-none pr-2 shrink-0 text-[10px]">{{ row.oldLineNum || '' }}</span>
              <span class="w-3 select-none shrink-0 font-bold text-rose-400">{{ row.kind === 'del' ? '−' : ' ' }}</span>
              <span class="flex-1 whitespace-pre-wrap break-words font-serif text-[13px] leading-relaxed">{{ row.oldText || row.text }}</span>
            </div>
          </div>
        </div>

        <!-- 右侧：改写后 -->
        <div class="flex flex-col h-full overflow-hidden">
          <div class="px-3 py-1 bg-atelier-900 border-b border-atelier-800 text-[11px] font-bold text-emerald-400 flex items-center justify-between shrink-0 select-none">
            <span>改写稿 (当前)</span>
            <span class="text-ink-500">{{ stats.newWords }} 字</span>
          </div>
          <div class="flex-1 overflow-auto divide-y divide-atelier-850/40">
            <div 
              v-for="(row, idx) in diffRows" 
              :key="'r-' + idx"
              v-show="row.kind !== 'del'"
              :class="[
                'flex items-start px-2 py-0.5 text-xs font-mono leading-relaxed',
                row.kind === 'add' ? 'bg-emerald-500/10 text-emerald-200 border-l-2 border-emerald-500' : 'text-ink-300'
              ]">
              <span class="w-10 text-right text-ink-600 select-none pr-2 shrink-0 text-[10px]">{{ row.newLineNum || '' }}</span>
              <span class="w-3 select-none shrink-0 font-bold text-emerald-400">{{ row.kind === 'add' ? '+' : ' ' }}</span>
              <span class="flex-1 whitespace-pre-wrap break-words font-serif text-[13px] leading-relaxed">{{ row.newText || row.text }}</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue';
import { computeLineDiff, computeDiffStats } from '../../utils/diff';
import { GitCompare, Check, RotateCcw, X } from 'lucide-vue-next';

const props = defineProps({
  originalText: { type: String, default: '' },
  modifiedText: { type: String, default: '' },
});

defineEmits(['apply', 'revert', 'close']);

const viewMode = ref('unified');

const diffRows = computed(() => {
  return computeLineDiff(props.originalText, props.modifiedText);
});

const stats = computed(() => {
  return computeDiffStats(props.originalText, props.modifiedText, diffRows.value);
});
</script>
