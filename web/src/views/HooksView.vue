<template>
  <div class="flex-1 p-6 overflow-y-auto space-y-6" v-if="state.currentProject">
    <!-- 头部工具栏 -->
    <div class="flex flex-col md:flex-row md:items-center justify-between gap-4 border-b border-atelier-750 pb-5">
      <div>
        <div class="flex items-center gap-2.5">
          <div class="w-8 h-8 rounded-lg bg-brand-amber/10 border border-brand-amber/25 flex items-center justify-center text-brand-amber shadow-amber-glow">
            <Anchor class="w-4 h-4" />
          </div>
          <div>
            <h2 class="text-base font-serif font-bold text-ink-50 tracking-wide">伏笔簿</h2>
            <p class="text-xs text-ink-400 mt-0.5">记录故事埋下的线索，跟进揭开时机。</p>
          </div>
        </div>
      </div>

      <div class="flex items-center gap-3">
        <div class="flex items-center gap-2 text-xs font-mono bg-atelier-850 px-3 py-1.5 rounded-md border border-atelier-750">
          <span class="text-ink-400">活跃伏笔</span>
          <strong class="text-brand-amber">{{ computedState.activeHooksList.value.length }}</strong>
          <span class="text-atelier-700">/</span>
          <span class="text-ink-400">总计</span>
          <span class="text-ink-200">{{ state.hooks.length }}</span>
        </div>

        <button 
          @click="runAIExtractHooks" 
          :disabled="isExtracting || state.chapters.length === 0"
          class="flex items-center gap-1.5 px-3 py-1.5 bg-brand-amber/10 hover:bg-brand-amber/20 text-brand-amber border border-brand-amber/30 font-medium text-xs rounded-md shadow-amber-glow transition cursor-pointer disabled:opacity-50"
          title="AI 智能从全书章节中挖掘未回收伏笔与暗线">
          <Sparkles class="w-3.5 h-3.5" :class="{ 'animate-spin': isExtracting }" />
          <span>{{ isExtracting ? '挖掘中...' : '智能挖掘' }}</span>
        </button>

        <button 
          @click="actions.createPlotHook('新设伏笔', '', (state.chapters.length || 0) + 3)" 
          class="flex items-center gap-1.5 px-3.5 py-1.5 bg-brand-amber hover:bg-brand-amber-hover text-atelier-950 font-bold text-xs rounded-md shadow-amber-glow transition cursor-pointer">
          <Plus class="w-3.5 h-3.5" />
          <span>新增伏笔</span>
        </button>
      </div>
    </div>

    <!-- 伏笔列表 -->
    <div class="space-y-3">
      <div 
        v-for="hook in state.hooks" 
        :key="hook.id" 
        class="p-4 bg-atelier-900 border border-atelier-750 rounded-xl flex flex-col md:flex-row md:items-center justify-between gap-4 hover:border-brand-amber/30 transition shadow-atelier-sm group">
        
        <div class="flex-1 grid grid-cols-1 md:grid-cols-4 gap-3 items-center text-xs">
          <div>
            <label class="text-[10px] text-ink-400 block mb-1">伏笔悬念名称：</label>
            <input 
              v-model="hook.title" 
              @change="actions.updatePlotHook(hook)" 
              class="w-full bg-atelier-950 border border-atelier-750 rounded-md px-2.5 py-1.5 text-ink-100 font-serif font-bold focus:outline-none focus:border-brand-amber/60" 
              placeholder="伏笔名称">
          </div>

          <div class="md:col-span-2">
            <label class="text-[10px] text-ink-400 block mb-1">伏笔线索描述：</label>
            <input 
              v-model="hook.details" 
              @change="actions.updatePlotHook(hook)" 
              class="w-full bg-atelier-950 border border-atelier-750 rounded-md px-2.5 py-1.5 text-ink-200 focus:outline-none focus:border-brand-amber/60" 
              placeholder="描述伏笔的具体线索或事件...">
          </div>

          <div>
            <label class="text-[10px] text-ink-400 block mb-1">章节跨度：</label>
            <div class="flex items-center gap-1 text-[11px] font-mono text-ink-300 bg-atelier-950 px-2.5 py-1.5 rounded-md border border-atelier-800">
              <span class="text-ink-400">第 {{ hook.created_chapter }} 章</span>
              <span class="text-brand-amber">➔</span>
              <span>第</span>
              <input 
                v-model.number="hook.target_chapter" 
                @change="actions.updatePlotHook(hook)" 
                type="number" 
                class="w-10 bg-atelier-850 text-center font-bold text-brand-amber rounded border border-atelier-750 py-0.5 focus:outline-none">
              <span>章</span>
            </div>
          </div>
        </div>

        <div class="flex items-center gap-3 shrink-0 pt-2 md:pt-0 border-t md:border-t-0 border-atelier-800">
          <select 
            v-model="hook.status" 
            @change="actions.updatePlotHook(hook)" 
            :class="hook.status === 'RESOLVED' 
              ? 'text-emerald-400 border-emerald-500/30 bg-emerald-500/10' 
              : (hook.status === 'FERMENTING' 
                ? 'text-rose-400 border-rose-500/30 bg-rose-500/10' 
                : 'text-brand-amber border-brand-amber/30 bg-brand-amber/10')"
            class="rounded-md border px-3 py-1.5 text-xs font-mono font-medium focus:outline-none cursor-pointer">
            <option value="OPEN" class="bg-atelier-900 text-brand-amber">⏳ 开放中 (OPEN)</option>
            <option value="FERMENTING" class="bg-atelier-900 text-rose-400">🔥 发酵中 (FERMENTING)</option>
            <option value="RESOLVED" class="bg-atelier-900 text-emerald-400">✅ 已回收 (RESOLVED)</option>
          </select>

          <button 
            @click="actions.deletePlotHook(hook.id)" 
            class="text-ink-500 hover:text-rose-400 text-xs px-2 py-1 transition cursor-pointer">
            删除
          </button>
        </div>
      </div>
    </div>

    <!-- 空白状态 -->
    <div 
      v-if="state.hooks.length === 0" 
      class="text-center py-20 bg-atelier-900 border border-dashed border-atelier-750 rounded-xl space-y-3">
      <div class="w-12 h-12 rounded-full bg-atelier-800 flex items-center justify-center mx-auto text-ink-400">
        <Anchor class="w-6 h-6" />
      </div>
      <h3 class="text-sm font-semibold text-ink-200">暂无伏笔记录</h3>
      <p class="text-xs text-ink-400 max-w-md mx-auto">
        在创作过程中随手登记草蛇灰线的悬念点，模型在推演剧情时将自动锚定并寻求闭环回收。
      </p>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue';
import { state, computedState, actions } from '../stores/appState';
import { Anchor, Plus, Sparkles } from 'lucide-vue-next';

const isExtracting = ref(false);

async function runAIExtractHooks() {
  isExtracting.value = true;
  try {
    await actions.extractPlotHooks();
  } finally {
    isExtracting.value = false;
  }
}
</script>
