<template>
  <div class="flex-1 p-6 md:p-8 overflow-y-auto w-full max-w-5xl xl:max-w-6xl mx-auto space-y-6">
    <!-- 头部工具栏 -->
    <div class="flex items-center justify-between border-b border-atelier-750 pb-5">
      <div class="flex items-center gap-2.5">
        <div class="w-8 h-8 rounded-lg bg-brand-amber/10 border border-brand-amber/25 flex items-center justify-center text-brand-amber shadow-amber-glow">
          <Settings class="w-4 h-4" />
        </div>
        <div>
          <h2 class="text-base font-serif font-bold text-ink-50 tracking-wide">AI 配置</h2>
          <p class="text-xs text-ink-400 mt-0.5">设置写作、构思与质检使用的大模型。</p>
        </div>
      </div>

      <div class="flex items-center gap-2.5">
        <button 
          @click="actions.testConfigConnection('default')" 
          :disabled="state.configTestStatus?.default?.loading"
          class="flex items-center gap-1.5 px-3 py-2 bg-atelier-800 hover:bg-atelier-700 text-ink-200 hover:text-white border border-atelier-700 font-medium text-xs rounded-md shadow transition cursor-pointer disabled:opacity-50">
          <Loader2 v-if="state.configTestStatus?.default?.loading" class="w-3.5 h-3.5 animate-spin text-brand-amber" />
          <Zap v-else class="w-3.5 h-3.5 text-brand-amber" />
          <span>测试连接</span>
        </button>

        <button 
          @click="actions.saveConfig" 
          class="flex items-center gap-1.5 px-4 py-2 bg-emerald-600 hover:bg-emerald-500 text-white font-bold text-xs rounded-md shadow transition cursor-pointer">
          <Save class="w-3.5 h-3.5" />
          <span>保存配置</span>
        </button>
      </div>
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

        <!-- 主接口测试诊断状态栏 -->
        <div v-if="state.configTestStatus?.default" class="text-xs rounded-lg p-2.5 flex items-center gap-2"
          :class="{
            'bg-atelier-950 text-ink-300 border border-atelier-800': state.configTestStatus.default.loading,
            'bg-emerald-950/40 text-emerald-300 border border-emerald-800/60': state.configTestStatus.default.status === 'ok',
            'bg-rose-950/40 text-rose-300 border border-rose-800/60': state.configTestStatus.default.status === 'error',
          }">
          <Loader2 v-if="state.configTestStatus.default.loading" class="w-4 h-4 animate-spin text-brand-amber shrink-0" />
          <CheckCircle2 v-else-if="state.configTestStatus.default.status === 'ok'" class="w-4 h-4 text-emerald-400 shrink-0" />
          <AlertCircle v-else class="w-4 h-4 text-rose-400 shrink-0" />
          <span class="font-mono text-[11px] leading-tight break-all">{{ state.configTestStatus.default.message }}</span>
        </div>
      </div>

      <!-- 三大角色分工模型绑定 -->
      <div class="pt-4 border-t border-atelier-800">
        <div class="flex items-center justify-between mb-3">
          <span class="text-xs font-serif font-bold text-brand-amber block">模型分工设置</span>
          <span class="text-[11px] text-ink-400">为构思、写作与体检分配对应的大模型</span>
        </div>

        <div class="grid grid-cols-1 sm:grid-cols-3 gap-4">
          <!-- 构思 -->
          <div class="p-3.5 bg-atelier-950/70 border border-atelier-800 rounded-lg space-y-2">
            <div class="flex items-center justify-between">
              <label class="block text-[11px] font-semibold text-ink-300">剧情构思模型 (Reasoning)：</label>
              <button 
                @click="actions.testConfigConnection('reasoner')"
                :disabled="state.configTestStatus?.reasoner?.loading"
                class="px-2 py-0.5 text-[10px] bg-atelier-850 hover:bg-brand-amber/20 hover:text-brand-amber text-ink-300 border border-atelier-750 rounded transition cursor-pointer disabled:opacity-50">
                <span v-if="state.configTestStatus?.reasoner?.loading">测试中...</span>
                <span v-else>测试</span>
              </button>
            </div>
            <input 
              v-model="state.config.reasoning_model" 
              class="w-full bg-atelier-900 border border-atelier-750 rounded px-2.5 py-1.5 text-xs text-ink-100 font-mono focus:outline-none focus:border-brand-amber/60" 
              placeholder="deepseek-reasoner">
            
            <div v-if="state.configTestStatus?.reasoner" class="text-[10px] font-mono mt-1 flex items-center gap-1.5"
              :class="state.configTestStatus.reasoner.status === 'ok' ? 'text-emerald-400' : (state.configTestStatus.reasoner.status === 'error' ? 'text-rose-400' : 'text-ink-400')">
              <CheckCircle2 v-if="state.configTestStatus.reasoner.status === 'ok'" class="w-3 h-3 shrink-0" />
              <AlertCircle v-else-if="state.configTestStatus.reasoner.status === 'error'" class="w-3 h-3 shrink-0" />
              <Loader2 v-else class="w-3 h-3 animate-spin text-brand-amber shrink-0" />
              <span class="truncate" :title="state.configTestStatus.reasoner.message">{{ state.configTestStatus.reasoner.message }}</span>
            </div>
          </div>

          <!-- 写作 -->
          <div class="p-3.5 bg-atelier-950/70 border border-atelier-800 rounded-lg space-y-2">
            <div class="flex items-center justify-between">
              <label class="block text-[11px] font-semibold text-ink-300">正文写作模型 (Writer)：</label>
              <button 
                @click="actions.testConfigConnection('writer')"
                :disabled="state.configTestStatus?.writer?.loading"
                class="px-2 py-0.5 text-[10px] bg-atelier-850 hover:bg-brand-amber/20 hover:text-brand-amber text-ink-300 border border-atelier-750 rounded transition cursor-pointer disabled:opacity-50">
                <span v-if="state.configTestStatus?.writer?.loading">测试中...</span>
                <span v-else>测试</span>
              </button>
            </div>
            <input 
              v-model="state.config.writer_model" 
              class="w-full bg-atelier-900 border border-atelier-750 rounded px-2.5 py-1.5 text-xs text-ink-100 font-mono focus:outline-none focus:border-brand-amber/60" 
              placeholder="deepseek-chat">
            
            <div v-if="state.configTestStatus?.writer" class="text-[10px] font-mono mt-1 flex items-center gap-1.5"
              :class="state.configTestStatus.writer.status === 'ok' ? 'text-emerald-400' : (state.configTestStatus.writer.status === 'error' ? 'text-rose-400' : 'text-ink-400')">
              <CheckCircle2 v-if="state.configTestStatus.writer.status === 'ok'" class="w-3 h-3 shrink-0" />
              <AlertCircle v-else-if="state.configTestStatus.writer.status === 'error'" class="w-3 h-3 shrink-0" />
              <Loader2 v-else class="w-3 h-3 animate-spin text-brand-amber shrink-0" />
              <span class="truncate" :title="state.configTestStatus.writer.message">{{ state.configTestStatus.writer.message }}</span>
            </div>
          </div>

          <!-- 体检 -->
          <div class="p-3.5 bg-atelier-950/70 border border-atelier-800 rounded-lg space-y-2">
            <div class="flex items-center justify-between">
              <label class="block text-[11px] font-semibold text-ink-300">文风体检模型 (Reviewer)：</label>
              <button 
                @click="actions.testConfigConnection('reviewer')"
                :disabled="state.configTestStatus?.reviewer?.loading"
                class="px-2 py-0.5 text-[10px] bg-atelier-850 hover:bg-brand-amber/20 hover:text-brand-amber text-ink-300 border border-atelier-750 rounded transition cursor-pointer disabled:opacity-50">
                <span v-if="state.configTestStatus?.reviewer?.loading">测试中...</span>
                <span v-else>测试</span>
              </button>
            </div>
            <input 
              v-model="state.config.reviewer_model" 
              class="w-full bg-atelier-900 border border-atelier-750 rounded px-2.5 py-1.5 text-xs text-ink-100 font-mono focus:outline-none focus:border-brand-amber/60" 
              placeholder="deepseek-reasoner">
            
            <div v-if="state.configTestStatus?.reviewer" class="text-[10px] font-mono mt-1 flex items-center gap-1.5"
              :class="state.configTestStatus.reviewer.status === 'ok' ? 'text-emerald-400' : (state.configTestStatus.reviewer.status === 'error' ? 'text-rose-400' : 'text-ink-400')">
              <CheckCircle2 v-if="state.configTestStatus.reviewer.status === 'ok'" class="w-3 h-3 shrink-0" />
              <AlertCircle v-else-if="state.configTestStatus.reviewer.status === 'error'" class="w-3 h-3 shrink-0" />
              <Loader2 v-else class="w-3 h-3 animate-spin text-brand-amber shrink-0" />
              <span class="truncate" :title="state.configTestStatus.reviewer.message">{{ state.configTestStatus.reviewer.message }}</span>
            </div>
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
          <div class="flex items-center justify-between">
            <span class="text-xs font-medium text-ink-300">第三方独立物理端点设置</span>
            <button 
              @click="actions.testConfigConnection('reviewer_provider')"
              :disabled="state.configTestStatus?.reviewer_provider?.loading"
              class="flex items-center gap-1.5 px-3 py-1 bg-atelier-850 hover:bg-atelier-800 text-brand-amber border border-brand-amber/30 text-xs rounded shadow transition cursor-pointer disabled:opacity-50">
              <Loader2 v-if="state.configTestStatus?.reviewer_provider?.loading" class="w-3.5 h-3.5 animate-spin text-brand-amber" />
              <Zap v-else class="w-3.5 h-3.5 text-brand-amber" />
              <span>测试独立通道</span>
            </button>
          </div>

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

          <!-- 独立通道诊断状态栏 -->
          <div v-if="state.configTestStatus?.reviewer_provider" class="text-xs rounded-lg p-2.5 flex items-center gap-2"
            :class="{
              'bg-atelier-900 text-ink-300 border border-atelier-800': state.configTestStatus.reviewer_provider.loading,
              'bg-emerald-950/40 text-emerald-300 border border-emerald-800/60': state.configTestStatus.reviewer_provider.status === 'ok',
              'bg-rose-950/40 text-rose-300 border border-rose-800/60': state.configTestStatus.reviewer_provider.status === 'error',
            }">
            <Loader2 v-if="state.configTestStatus.reviewer_provider.loading" class="w-4 h-4 animate-spin text-brand-amber shrink-0" />
            <CheckCircle2 v-else-if="state.configTestStatus.reviewer_provider.status === 'ok'" class="w-4 h-4 text-emerald-400 shrink-0" />
            <AlertCircle v-else class="w-4 h-4 text-rose-400 shrink-0" />
            <span class="font-mono text-[11px] leading-tight break-all">{{ state.configTestStatus.reviewer_provider.message }}</span>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { state, actions } from '../stores/appState';
import { Settings, Save, ShieldCheck, Zap, Loader2, CheckCircle2, AlertCircle } from 'lucide-vue-next';
</script>
