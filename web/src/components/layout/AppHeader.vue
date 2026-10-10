<template>
  <header class="h-14 border-b border-atelier-750 px-5 flex items-center justify-between bg-atelier-900/60 backdrop-blur-md select-none shrink-0 z-10" style="-webkit-app-region: drag;">
    <!-- 左侧：作品与状态 -->
    <div class="flex items-center gap-3 min-w-0" v-if="state.currentProject" style="-webkit-app-region: no-drag;">
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
    <div v-else class="text-xs text-ink-400 font-serif" style="-webkit-app-region: no-drag;">
      请选择或新建作品
    </div>

    <!-- 右侧：全局快捷工具与流水线驱动 -->
    <div class="flex items-center gap-2 shrink-0" style="-webkit-app-region: no-drag;">
      <!-- 面板视界与专注模式开关 (仅手稿工作台) -->
      <div v-if="state.activeTab === 'workbench'" class="flex items-center bg-atelier-850 border border-atelier-750 rounded-md p-0.5 mr-1">
        <button 
          @click="state.showWorkflowPanel = !state.showWorkflowPanel"
          :class="state.showWorkflowPanel ? 'bg-atelier-750 text-brand-amber' : 'text-ink-400 hover:text-ink-200'"
          class="p-1.5 rounded transition cursor-pointer"
          title="展开或折叠左侧工序与参考">
          <PanelLeft class="w-3.5 h-3.5" />
        </button>
        <button 
          @click="state.isZenMode = true"
          class="p-1.5 rounded text-ink-400 hover:text-brand-amber hover:bg-atelier-750 transition cursor-pointer"
          title="专注模式 (按 ESC 退出)">
          <Maximize2 class="w-3.5 h-3.5" />
        </button>
      </div>

      <!-- 真实 Token 成本计数器 -->
      <div 
        v-if="state.pipelineState.tokens.total_tokens > 0"
        class="hidden xl:flex items-center gap-2 px-2.5 py-1 bg-atelier-850 rounded-md border border-atelier-750 text-[11px] font-mono text-ink-300">
        <span>Token: <strong class="text-ink-100 font-semibold">{{ state.pipelineState.tokens.total_tokens.toLocaleString() }}</strong></span>
        <span class="text-ink-500">|</span>
        <span>${{ computedState.pipelineCostUSD.value.toFixed(4) }}</span>
      </div>

      <!-- 进行中的后台任务快捷状态条 -->
      <button 
        v-if="computedState.activeTasks.value.length > 0"
        @click="state.showMessageCenterModal = true"
        class="hidden lg:flex items-center gap-1.5 px-2.5 py-1 bg-brand-amber/15 border border-brand-amber/40 text-brand-amber rounded-md text-[11px] font-mono animate-subtle-pulse cursor-pointer"
        title="查看正在进行的后台任务">
        <Loader2 class="w-3 h-3 animate-spin" />
        <span class="truncate max-w-[140px]">{{ computedState.activeTasks.value[0].title }}</span>
      </button>

      <!-- 消息中心入口 -->
      <button 
        @click="state.showMessageCenterModal = true"
        class="relative flex items-center gap-1.5 px-2.5 py-1.5 text-xs font-medium bg-atelier-850 hover:bg-atelier-800 text-ink-200 rounded-md border border-atelier-700 transition cursor-pointer"
        title="查看任务进度与通知">
        <Loader2 v-if="computedState.activeTasks.value.length > 0" class="w-3.5 h-3.5 text-brand-amber animate-spin" />
        <Bell v-else class="w-3.5 h-3.5 text-brand-amber" />
        <span class="hidden sm:inline">消息</span>
        <span 
          v-if="computedState.unreadMessageCount.value > 0"
          class="px-1.5 py-0.2 min-w-4 text-[9px] font-mono font-bold bg-brand-rose text-white rounded-full flex items-center justify-center">
          {{ computedState.unreadMessageCount.value }}
        </span>
      </button>

      <!-- 提示词透视按钮 -->
      <button 
        @click="state.showPromptInspectorModal = true"
        class="flex items-center gap-1.5 px-2.5 py-1.5 text-xs font-medium bg-atelier-850 hover:bg-atelier-800 text-ink-200 rounded-md border border-atelier-700 transition cursor-pointer"
        title="查看发给 AI 的完整指令">
        <Eye class="w-3.5 h-3.5 text-brand-amber" />
        <span class="hidden sm:inline">查看指令</span>
      </button>

      <!-- 情境工坊对话按钮 -->
      <button 
        @click="state.showWorkshopChatDrawer = true"
        class="flex items-center gap-1.5 px-2.5 py-1.5 text-xs font-medium bg-atelier-850 hover:bg-atelier-800 text-ink-200 rounded-md border border-atelier-700 transition cursor-pointer"
        title="与书中人物实时对话">
        <MessageSquareText class="w-3.5 h-3.5 text-brand-cyan" />
        <span class="hidden sm:inline">角色对戏</span>
      </button>

      <!-- 一键成章 (Auto-Pipeline) 触发器 -->
      <button 
        @click="$emit('trigger-pipeline')"
        :disabled="state.pipelineState.active || !state.currentProject"
        class="flex items-center gap-1.5 px-3 py-1.5 text-xs font-bold rounded-md transition shadow-atelier-sm cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed"
        :title="'由 AI 自动完成构思、写作与体检'"
        :class="state.pipelineState.active 
          ? 'bg-amber-500/20 text-brand-amber border border-brand-amber/40 animate-subtle-pulse' 
          : 'bg-gradient-to-r from-brand-amber to-amber-600 hover:from-brand-amber-hover hover:to-amber-500 text-atelier-950'">
        <Loader2 v-if="state.pipelineState.active" class="w-3.5 h-3.5 animate-spin" />
        <Zap v-else class="w-3.5 h-3.5 fill-current" />
        <span>{{ state.pipelineState.active ? state.pipelineState.message || '正在创作...' : '一键成章' }}</span>
      </button>
    </div>
  </header>
</template>

<script setup>
import { state, computedState } from '../../stores/appState';
import { Eye, MessageSquareText, Zap, Loader2, PanelLeft, PanelRight, Maximize2, Bell } from 'lucide-vue-next';

defineEmits(['trigger-pipeline']);
</script>
