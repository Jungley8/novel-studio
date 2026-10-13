<template>
  <div class="flex-1 flex flex-col h-full bg-atelier-950 overflow-hidden select-text font-mono text-xs">
    <!-- 头部工具栏 -->
    <div class="px-5 py-2.5 bg-atelier-900 border-b border-atelier-750 flex items-center justify-between shrink-0 select-none">
      <div class="flex items-center gap-3">
        <div class="w-7 h-7 rounded-lg bg-brand-amber/10 border border-brand-amber/25 flex items-center justify-center text-brand-amber shadow-amber-glow shrink-0">
          <GitCompare class="w-4 h-4" />
        </div>
        <div class="flex items-center gap-2 flex-wrap">
          <span class="font-serif font-bold text-sm text-ink-50">改写对比 (Git Diff)</span>
          <span class="text-[10px] font-mono px-2 py-0.5 rounded bg-atelier-800 text-ink-300 border border-atelier-750">
            原 {{ stats.oldWords }} 字 ➔ 新 {{ stats.newWords }} 字
          </span>
          <span 
            :class="stats.deltaWords >= 0 ? 'bg-emerald-500/15 text-emerald-300 border-emerald-500/30' : 'bg-rose-500/15 text-rose-300 border-rose-500/30'"
            class="text-[10px] font-mono font-bold px-1.5 py-0.5 rounded border">
            {{ stats.deltaWords >= 0 ? '+' + stats.deltaWords : stats.deltaWords }} 字
          </span>
        </div>
      </div>

      <!-- 右侧控制区 -->
      <div class="flex items-center gap-2">
        <!-- 左右并排 (Split) / 单列统一 (Unified) 切换 -->
        <div class="flex items-center bg-atelier-850 p-0.5 rounded-lg border border-atelier-750 text-[11px]">
          <button 
            @click="viewMode = 'split'"
            :class="viewMode === 'split' ? 'bg-brand-amber text-atelier-950 font-bold' : 'text-ink-300 hover:text-ink-100'"
            class="px-2.5 py-1 rounded transition cursor-pointer flex items-center gap-1"
            title="左右双栏对比，严格对齐增删改行（推荐）">
            <Columns class="w-3.5 h-3.5" />
            <span>左右对比 (Split)</span>
          </button>
          <button 
            @click="viewMode = 'unified'"
            :class="viewMode === 'unified' ? 'bg-brand-amber text-atelier-950 font-bold' : 'text-ink-300 hover:text-ink-100'"
            class="px-2.5 py-1 rounded transition cursor-pointer flex items-center gap-1"
            title="单列上下流对比">
            <Rows class="w-3.5 h-3.5" />
            <span>单列统一 (Unified)</span>
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

    <!-- 主比对画布 (第三方专业组件 v-code-diff，支持 Git 左右对比及行内词级高亮) -->
    <div class="flex-1 min-h-0 overflow-auto bg-atelier-950">
      <CodeDiff
        :old-string="oldContent"
        :new-string="newContent"
        :output-format="viewMode === 'split' ? 'side-by-side' : 'line-by-line'"
        theme="dark"
        diff-style="word"
        language="markdown"
        filename="原文 (改写前)"
        new-filename="改写稿 (当前)"
        :force-inline-comparison="false"
        :no-diff-line-feed="true"
        :hide-header="false"
        class="h-full"
      />
    </div>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue';
import { CodeDiff } from 'v-code-diff';
import { GitCompare, Columns, Rows, Check, RotateCcw, X } from 'lucide-vue-next';

const props = defineProps({
  oldText: { type: String, default: '' },
  newText: { type: String, default: '' },
  originalText: { type: String, default: '' },
  modifiedText: { type: String, default: '' },
});

defineEmits(['apply', 'revert', 'close']);

// 默认直接开启用户要求的 Git 左右对比视图 (side-by-side)
const viewMode = ref('split');

const oldContent = computed(() => props.oldText || props.originalText || '');
const newContent = computed(() => props.newText || props.modifiedText || '');

const stats = computed(() => {
  const oldWords = oldContent.value.length;
  const newWords = newContent.value.length;
  const deltaWords = newWords - oldWords;
  return {
    oldWords,
    newWords,
    deltaWords,
  };
});
</script>

<style scoped>
:deep(.v-code-diff) {
  border: none !important;
  border-radius: 0 !important;
  font-family: inherit !important;
}
:deep(.v-code-diff .d-code) {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace !important;
  font-size: 12px !important;
  line-height: 1.6 !important;
}
</style>
