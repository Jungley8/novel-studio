<template>
  <div class="flex-1 p-6 overflow-y-auto max-w-4xl space-y-6">
    <!-- 头部工具栏 -->
    <div class="flex items-center justify-between border-b border-atelier-750 pb-5">
      <div class="flex items-center gap-2.5">
        <div class="w-8 h-8 rounded-lg bg-brand-amber/10 border border-brand-amber/25 flex items-center justify-center text-brand-amber shadow-amber-glow">
          <Settings class="w-4 h-4" />
        </div>
        <div>
          <h2 class="text-base font-serif font-bold text-ink-50 tracking-wide">系统与模型路由 (Engine & LLM Router)</h2>
          <p class="text-xs text-ink-400 mt-0.5">三权分立多通道路由、物理端点解耦与自审盲区防护配置</p>
        </div>
      </div>

      <button 
        @click="actions.saveConfig" 
        class="flex items-center gap-1.5 px-4 py-2 bg-emerald-600 hover:bg-emerald-500 text-white font-bold text-xs rounded-md shadow transition cursor-pointer">
        <Save class="w-3.5 h-3.5" />
        <span>保存系统配置</span>
      </button>
    </div>

    <!-- 主配置表单卡片 -->
    <div class="p-6 bg-atelier-900 border border-atelier-750 rounded-xl space-y-5 shadow-atelier-md">
      <!-- 基础接入点 -->
      <div class="space-y-4">
        <div>
          <label class="block text-xs font-medium text-ink-300 mb-1.5">主接口 Base URL (OpenAI 兼容协议)：</label>
          <input 
            v-model="state.config.api_base" 
            class="w-full bg-atelier-950 border border-atelier-750 rounded-lg px-3.5 py-2 text-xs text-ink-100 font-mono focus:outline-none focus:border-brand-amber/60" 
            placeholder="https://api.deepseek.com/v1">
        </div>

        <div>
          <label class="block text-xs font-medium text-ink-300 mb-1.5">主接口 API Key：</label>
          <input 
            type="password" 
            v-model="state.config.api_key" 
            class="w-full bg-atelier-950 border border-atelier-750 rounded-lg px-3.5 py-2 text-xs text-ink-100 font-mono focus:outline-none focus:border-brand-amber/60" 
            placeholder="sk-...">
        </div>
      </div>

      <!-- 三大角色分工模型绑定 -->
      <div class="pt-2 border-t border-atelier-800">
        <span class="text-xs font-serif font-bold text-brand-amber block mb-3">三大核心岗位角色模型绑定 (Role Model Binding)</span>
        <div class="grid grid-cols-1 sm:grid-cols-3 gap-4">
          <div>
            <label class="block text-[11px] text-ink-400 mb-1">因果推演师 (Reasoning)：</label>
            <input 
              v-model="state.config.reasoning_model" 
              class="w-full bg-atelier-950 border border-atelier-750 rounded-md px-3 py-1.5 text-xs text-ink-100 font-mono focus:outline-none focus:border-brand-amber/60" 
              placeholder="deepseek-reasoner">
          </div>
          <div>
            <label class="block text-[11px] text-ink-400 mb-1">文学渲染师 (Writer)：</label>
            <input 
              v-model="state.config.writer_model" 
              class="w-full bg-atelier-950 border border-atelier-750 rounded-md px-3 py-1.5 text-xs text-ink-100 font-mono focus:outline-none focus:border-brand-amber/60" 
              placeholder="deepseek-chat">
          </div>
          <div>
            <label class="block text-[11px] text-ink-400 mb-1">主审质检总监 (Reviewer)：</label>
            <input 
              v-model="state.config.reviewer_model" 
              class="w-full bg-atelier-950 border border-atelier-750 rounded-md px-3 py-1.5 text-xs text-ink-100 font-mono focus:outline-none focus:border-brand-amber/60" 
              placeholder="deepseek-reasoner">
          </div>
        </div>
      </div>

      <!-- 独立第三方质检通道配置 (Reviewer Provider) -->
      <div class="pt-4 border-t border-atelier-800 space-y-4">
        <div class="flex items-center justify-between">
          <div>
            <div class="flex items-center gap-2">
              <ShieldCheck class="w-4 h-4 text-brand-amber" />
              <span class="text-xs font-serif font-bold text-brand-amber">
                独立质检员通道 (Reviewer Provider - 物理隔离自写自评)
              </span>
            </div>
            <p class="text-[11px] text-ink-400 mt-0.5">
              可配置独立第三方端点（如 Claude 3.5 Sonnet / OpenAI GPT-4o / Kimi），实现写作与审校彻底异构解耦。
            </p>
          </div>
          <label class="flex items-center gap-2 text-xs text-ink-200 cursor-pointer">
            <input 
              type="checkbox" 
              v-model="state.enableReviewerProvider" 
              class="rounded border-atelier-700 bg-atelier-950 text-brand-amber accent-brand-amber">
            <span>启用独立通道</span>
          </label>
        </div>

        <div v-if="state.enableReviewerProvider" class="p-4 bg-atelier-950 rounded-xl border border-atelier-750 space-y-3.5">
          <div>
            <label class="block text-[11px] text-ink-400 mb-1">质检员 API Base URL：</label>
            <input 
              v-model="state.config.reviewer_provider.api_base" 
              class="w-full bg-atelier-900 border border-atelier-750 rounded-md px-3 py-1.5 text-xs text-ink-100 font-mono focus:outline-none focus:border-brand-amber/60" 
              placeholder="https://api.openai.com/v1 或 https://api.anthropic.com/v1">
          </div>
          <div>
            <label class="block text-[11px] text-ink-400 mb-1">质检员 API Key：</label>
            <input 
              type="password" 
              v-model="state.config.reviewer_provider.api_key" 
              class="w-full bg-atelier-900 border border-atelier-750 rounded-md px-3 py-1.5 text-xs text-ink-100 font-mono focus:outline-none focus:border-brand-amber/60" 
              placeholder="sk-...">
          </div>
          <div>
            <label class="block text-[11px] text-ink-400 mb-1">质检员指定模型标识：</label>
            <input 
              v-model="state.config.reviewer_provider.model" 
              class="w-full bg-atelier-900 border border-atelier-750 rounded-md px-3 py-1.5 text-xs text-ink-100 font-mono focus:outline-none focus:border-brand-amber/60" 
              placeholder="gpt-4o / claude-3-5-sonnet-20241022">
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { state, actions } from '../stores/appState';
import { Settings, Save, ShieldCheck } from 'lucide-vue-next';
</script>
