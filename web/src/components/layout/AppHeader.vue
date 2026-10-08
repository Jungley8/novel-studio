<template>
  <header class="h-14 border-b border-atelier-750 px-6 flex items-center justify-between bg-atelier-900/60 backdrop-blur-md select-none shrink-0 z-10">
    <!-- 左侧：作品与状态 -->
    <div class="flex items-center gap-3 min-w-0" v-if="state.currentProject">
      <h2 class="text-sm font-bold font-serif text-ink-50 truncate tracking-wide">
        {{ state.currentProject.title }}
      </h2>
      <span class="text-[10px] font-medium px-2 py-0.5 bg-atelier-800 text-ink-300 rounded border border-atelier-700 shrink-0">
        {{ state.currentProject.target_platform }}
      </span>
      <span v-if="computedState.currentVolume.value" class="text-xs text-ink-400 hidden md:inline truncate">
        · {{ computedState.currentVolume.value.title }}
      </span>
    </div>
    <div v-else class="text-xs text-ink-400">
      请选择或新建小说作品
    </div>

    <!-- 右侧：全局快捷工具与流水线驱动 -->
    <div class="flex items-center gap-2.5 shrink-0">
      <!-- 真实 Token 成本计数器 -->
      <div 
        v-if="state.pipelineState.tokens.total_tokens > 0"
        class="hidden lg:flex items-center gap-2 px-2.5 py-1 bg-atelier-850 rounded-md border border-atelier-750 text-[11px] font-mono text-ink-300">
        <span>Token: <strong class="text-ink-100 font-semibold">{{ state.pipelineState.tokens.total_tokens.toLocaleString() }}</strong></span>
        <span class="text-ink-500">|</span>
        <span>${{ computedState.pipelineCostUSD.value.toFixed(4) }}</span>
      </div>

      <!-- 提示词透视按钮 -->
      <button 
        @click="state.showPromptInspectorModal = true"
        class="flex items-center gap-1.5 px-2.5 py-1.5 text-xs font-medium bg-atelier-850 hover:bg-atelier-800 text-ink-200 rounded-md border border-atelier-700 transition cursor-pointer"
        title="透视查看发往大模型的四层组装提示词与 Token 预算">
        <Eye class="w-3.5 h-3.5 text-brand-amber" />
        <span class="hidden sm:inline">提示词透视</span>
      </button>

      <!-- 情境工坊对话按钮 -->
      <button 
        @click="state.showWorkshopChatDrawer = true"
        class="flex items-center gap-1.5 px-2.5 py-1.5 text-xs font-medium bg-atelier-850 hover:bg-atelier-800 text-ink-200 rounded-md border border-atelier-700 transition cursor-pointer"
        title="与角色对戏、头脑风暴或主编质询">
        <MessageSquareText class="w-3.5 h-3.5 text-brand-cyan" />
        <span class="hidden sm:inline">情境对话</span>
      </button>

      <!-- 一键全流程自主生产 (Auto-Pipeline) 触发器 -->
      <button 
        @click="$emit('trigger-pipeline')"
        :disabled="state.pipelineState.active || !state.currentProject"
        class="flex items-center gap-1.5 px-3.5 py-1.5 text-xs font-bold rounded-md transition shadow-atelier-sm cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed"
        :class="state.pipelineState.active 
          ? 'bg-amber-500/20 text-brand-amber border border-brand-amber/40 animate-subtle-pulse' 
          : 'bg-gradient-to-r from-brand-amber to-amber-600 hover:from-brand-amber-hover hover:to-amber-500 text-atelier-950'">
        <Loader2 v-if="state.pipelineState.active" class="w-3.5 h-3.5 animate-spin" />
        <Zap v-else class="w-3.5 h-3.5 fill-current" />
        <span>{{ state.pipelineState.active ? state.pipelineState.message || '自主生产中...' : '自主推演闭环' }}</span>
      </button>
    </div>
  </header>
</template>

<script setup>
import { state, computedState } from '../../stores/appState';
import { Eye, MessageSquareText, Zap, Loader2 } from 'lucide-vue-next';

defineEmits(['trigger-pipeline']);
</script>
