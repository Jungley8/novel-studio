<template>
  <div class="flex-1 p-6 md:p-8 overflow-y-auto w-full max-w-5xl xl:max-w-6xl mx-auto space-y-6">
    <!-- 头部工具栏 -->
    <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4 border-b border-atelier-750 pb-5">
      <div class="flex items-center gap-3">
        <div class="w-9 h-9 rounded-xl bg-brand-amber/10 border border-brand-amber/25 flex items-center justify-center text-brand-amber shadow-amber-glow">
          <Settings class="w-4 h-4" />
        </div>
        <div>
          <div class="flex items-center gap-2">
            <h2 class="text-base font-serif font-bold text-ink-50 tracking-wide">系统设置</h2>
            <span class="text-[10px] font-mono px-1.5 py-0.5 rounded bg-brand-amber/15 text-brand-amber border border-brand-amber/30">
              系统级 · 一次配置全书通用
            </span>
          </div>
          <p class="text-xs text-ink-400 mt-0.5">全书通用的 AI 模型与接口密钥。</p>
        </div>
      </div>

      <div class="flex items-center gap-2.5">
        <button 
          @click="actions.testConfigConnection('default')" 
          :disabled="state.configTestStatus?.default?.loading"
          class="flex items-center gap-1.5 px-3 py-2 bg-atelier-800 hover:bg-atelier-700 text-ink-200 hover:text-white border border-atelier-700 font-medium text-xs rounded-md shadow transition cursor-pointer disabled:opacity-50">
          <Loader2 v-if="state.configTestStatus?.default?.loading" class="w-3.5 h-3.5 animate-spin text-brand-amber" />
          <Zap v-else class="w-3.5 h-3.5 text-brand-amber" />
          <span>测试主接口</span>
        </button>

        <button 
          @click="actions.saveConfig" 
          class="flex items-center gap-1.5 px-4 py-2 bg-emerald-600 hover:bg-emerald-500 text-white font-bold text-xs rounded-md shadow transition cursor-pointer">
          <Save class="w-3.5 h-3.5" />
          <span>保存配置</span>
        </button>
      </div>
    </div>

    <!-- 1. 全局主接口卡片 -->
    <div class="p-5 bg-atelier-900 border border-atelier-750 rounded-xl space-y-4 shadow-atelier-md">
      <div class="flex items-center justify-between">
        <div>
          <h3 class="text-xs font-serif font-bold text-ink-100 flex items-center gap-2">
            <Cpu class="w-3.5 h-3.5 text-brand-amber" />
            <span>主接口设置</span>
          </h3>
          <p class="text-[11px] text-ink-400 mt-0.5">默认连接端点，未单独配置的岗位自动继承此处。</p>
        </div>
      </div>

      <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
        <div>
          <label class="block text-xs font-medium text-ink-300 mb-1.5">接口地址 (Base URL)：</label>
          <input 
            v-model="state.config.api_base" 
            class="w-full bg-atelier-950 border border-atelier-750 rounded-lg px-3.5 py-2 text-xs text-ink-100 font-mono focus:outline-none focus:border-brand-amber/60" 
            placeholder="https://api.deepseek.com/v1">
        </div>

        <div>
          <label class="block text-xs font-medium text-ink-300 mb-1.5">接口密钥 (API Key)：</label>
          <input 
            type="password" 
            v-model="state.config.api_key" 
            class="w-full bg-atelier-950 border border-atelier-750 rounded-lg px-3.5 py-2 text-xs text-ink-100 font-mono focus:outline-none focus:border-brand-amber/60" 
            placeholder="sk-...">
        </div>
      </div>

      <!-- 主接口诊断状态 -->
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

    <!-- 2. 三大岗位分工设置 -->
    <div class="space-y-3">
      <div class="flex items-center justify-between">
        <div>
          <h3 class="text-xs font-serif font-bold text-ink-100 flex items-center gap-1.5">
            <Sparkles class="w-3.5 h-3.5 text-brand-amber" />
            <span>三大岗位分工</span>
          </h3>
          <p class="text-[11px] text-ink-400 mt-0.5">构思、写作、体检独立分工，支持分别配置独立接口与模型。</p>
        </div>
        <div class="text-[11px] font-mono text-ink-400">
          留空自动继承主接口
        </div>
      </div>

      <div class="grid grid-cols-1 lg:grid-cols-3 gap-4">
        <!-- 岗位 1: 剧情构思 (Reasoner) -->
        <div class="p-4 bg-atelier-900 border border-atelier-750 rounded-xl space-y-3 shadow-atelier-sm flex flex-col justify-between">
          <div class="space-y-3">
            <div class="flex items-center justify-between">
              <div class="flex items-center gap-2">
                <div class="w-6 h-6 rounded-md bg-amber-500/10 border border-amber-500/25 flex items-center justify-center text-amber-400">
                  <Lightbulb class="w-3.5 h-3.5" />
                </div>
                <div>
                  <h4 class="text-xs font-bold text-ink-100">剧情构思</h4>
                  <p class="text-[10px] text-ink-400">推演剧情大纲与转折逻辑</p>
                </div>
              </div>
              <button 
                @click="actions.testConfigConnection('reasoner')"
                :disabled="state.configTestStatus?.reasoner?.loading"
                class="px-2 py-0.5 text-[10px] bg-atelier-850 hover:bg-brand-amber/20 hover:text-brand-amber text-ink-300 border border-atelier-750 rounded transition cursor-pointer disabled:opacity-50">
                <span v-if="state.configTestStatus?.reasoner?.loading">测试中...</span>
                <span v-else>测试</span>
              </button>
            </div>

            <div>
              <label class="block text-[11px] text-ink-300 mb-1">模型名称：</label>
              <input 
                v-model="state.config.reasoning_model" 
                class="w-full bg-atelier-950 border border-atelier-750 rounded-md px-2.5 py-1.5 text-xs text-ink-100 font-mono focus:outline-none focus:border-brand-amber/60" 
                placeholder="deepseek-reasoner">
            </div>

            <!-- 独立接口开关 -->
            <div class="pt-2 border-t border-atelier-800">
              <label class="flex items-center gap-2 text-[11px] text-ink-300 cursor-pointer">
                <input 
                  type="checkbox" 
                  v-model="state.enableReasonerProvider" 
                  class="rounded border-atelier-700 bg-atelier-950 text-brand-amber accent-brand-amber">
                <span>独立接口 (支持异构连接)</span>
              </label>

              <div v-if="state.enableReasonerProvider" class="mt-2.5 p-2.5 bg-atelier-950 rounded-lg border border-atelier-800 space-y-2">
                <div>
                  <label class="block text-[10px] text-ink-400 mb-0.5">专属 Base URL：</label>
                  <input 
                    v-model="state.config.reasoner_provider.api_base" 
                    class="w-full bg-atelier-900 border border-atelier-750 rounded px-2 py-1 text-xs text-ink-100 font-mono focus:outline-none focus:border-brand-amber/60" 
                    placeholder="留空继承主接口">
                </div>
                <div>
                  <label class="block text-[10px] text-ink-400 mb-0.5">专属 API Key：</label>
                  <input 
                    type="password" 
                    v-model="state.config.reasoner_provider.api_key" 
                    class="w-full bg-atelier-900 border border-atelier-750 rounded px-2 py-1 text-xs text-ink-100 font-mono focus:outline-none focus:border-brand-amber/60" 
                    placeholder="留空继承主接口">
                </div>
                <div>
                  <label class="block text-[10px] text-ink-400 mb-0.5">专属 Model：</label>
                  <input 
                    v-model="state.config.reasoner_provider.model" 
                    class="w-full bg-atelier-900 border border-atelier-750 rounded px-2 py-1 text-xs text-ink-100 font-mono focus:outline-none focus:border-brand-amber/60" 
                    placeholder="留空使用上方模型">
                </div>
              </div>
            </div>
          </div>

          <!-- 诊断结果 -->
          <div v-if="state.configTestStatus?.reasoner" class="text-[10px] font-mono mt-2 pt-2 border-t border-atelier-800 flex items-center gap-1.5"
            :class="state.configTestStatus.reasoner.status === 'ok' ? 'text-emerald-400' : (state.configTestStatus.reasoner.status === 'error' ? 'text-rose-400' : 'text-ink-400')">
            <CheckCircle2 v-if="state.configTestStatus.reasoner.status === 'ok'" class="w-3 h-3 shrink-0" />
            <AlertCircle v-else-if="state.configTestStatus.reasoner.status === 'error'" class="w-3 h-3 shrink-0" />
            <Loader2 v-else class="w-3 h-3 animate-spin text-brand-amber shrink-0" />
            <span class="truncate" :title="state.configTestStatus.reasoner.message">{{ state.configTestStatus.reasoner.message }}</span>
          </div>
        </div>

        <!-- 岗位 2: 正文写作 (Writer) -->
        <div class="p-4 bg-atelier-900 border border-atelier-750 rounded-xl space-y-3 shadow-atelier-sm flex flex-col justify-between">
          <div class="space-y-3">
            <div class="flex items-center justify-between">
              <div class="flex items-center gap-2">
                <div class="w-6 h-6 rounded-md bg-emerald-500/10 border border-emerald-500/25 flex items-center justify-center text-emerald-400">
                  <PenTool class="w-3.5 h-3.5" />
                </div>
                <div>
                  <h4 class="text-xs font-bold text-ink-100">正文写作</h4>
                  <p class="text-[10px] text-ink-400">场景描摹、对白与正文起草</p>
                </div>
              </div>
              <button 
                @click="actions.testConfigConnection('writer')"
                :disabled="state.configTestStatus?.writer?.loading"
                class="px-2 py-0.5 text-[10px] bg-atelier-850 hover:bg-brand-amber/20 hover:text-brand-amber text-ink-300 border border-atelier-750 rounded transition cursor-pointer disabled:opacity-50">
                <span v-if="state.configTestStatus?.writer?.loading">测试中...</span>
                <span v-else>测试</span>
              </button>
            </div>

            <div>
              <label class="block text-[11px] text-ink-300 mb-1">模型名称：</label>
              <input 
                v-model="state.config.writer_model" 
                class="w-full bg-atelier-950 border border-atelier-750 rounded-md px-2.5 py-1.5 text-xs text-ink-100 font-mono focus:outline-none focus:border-brand-amber/60" 
                placeholder="deepseek-chat">
            </div>

            <!-- 独立接口开关 -->
            <div class="pt-2 border-t border-atelier-800">
              <label class="flex items-center gap-2 text-[11px] text-ink-300 cursor-pointer">
                <input 
                  type="checkbox" 
                  v-model="state.enableWriterProvider" 
                  class="rounded border-atelier-700 bg-atelier-950 text-brand-amber accent-brand-amber">
                <span>独立接口 (支持异构连接)</span>
              </label>

              <div v-if="state.enableWriterProvider" class="mt-2.5 p-2.5 bg-atelier-950 rounded-lg border border-atelier-800 space-y-2">
                <div>
                  <label class="block text-[10px] text-ink-400 mb-0.5">专属 Base URL：</label>
                  <input 
                    v-model="state.config.writer_provider.api_base" 
                    class="w-full bg-atelier-900 border border-atelier-750 rounded px-2 py-1 text-xs text-ink-100 font-mono focus:outline-none focus:border-brand-amber/60" 
                    placeholder="留空继承主接口">
                </div>
                <div>
                  <label class="block text-[10px] text-ink-400 mb-0.5">专属 API Key：</label>
                  <input 
                    type="password" 
                    v-model="state.config.writer_provider.api_key" 
                    class="w-full bg-atelier-900 border border-atelier-750 rounded px-2 py-1 text-xs text-ink-100 font-mono focus:outline-none focus:border-brand-amber/60" 
                    placeholder="留空继承主接口">
                </div>
                <div>
                  <label class="block text-[10px] text-ink-400 mb-0.5">专属 Model：</label>
                  <input 
                    v-model="state.config.writer_provider.model" 
                    class="w-full bg-atelier-900 border border-atelier-750 rounded px-2 py-1 text-xs text-ink-100 font-mono focus:outline-none focus:border-brand-amber/60" 
                    placeholder="留空使用上方模型">
                </div>
              </div>
            </div>
          </div>

          <!-- 诊断结果 -->
          <div v-if="state.configTestStatus?.writer" class="text-[10px] font-mono mt-2 pt-2 border-t border-atelier-800 flex items-center gap-1.5"
            :class="state.configTestStatus.writer.status === 'ok' ? 'text-emerald-400' : (state.configTestStatus.writer.status === 'error' ? 'text-rose-400' : 'text-ink-400')">
            <CheckCircle2 v-if="state.configTestStatus.writer.status === 'ok'" class="w-3 h-3 shrink-0" />
            <AlertCircle v-else-if="state.configTestStatus.writer.status === 'error'" class="w-3 h-3 shrink-0" />
            <Loader2 v-else class="w-3 h-3 animate-spin text-brand-amber shrink-0" />
            <span class="truncate" :title="state.configTestStatus.writer.message">{{ state.configTestStatus.writer.message }}</span>
          </div>
        </div>

        <!-- 岗位 3: 文风体检 (Reviewer) -->
        <div class="p-4 bg-atelier-900 border border-atelier-750 rounded-xl space-y-3 shadow-atelier-sm flex flex-col justify-between">
          <div class="space-y-3">
            <div class="flex items-center justify-between">
              <div class="flex items-center gap-2">
                <div class="w-6 h-6 rounded-md bg-purple-500/10 border border-purple-500/25 flex items-center justify-center text-purple-400">
                  <ShieldCheck class="w-3.5 h-3.5" />
                </div>
                <div>
                  <h4 class="text-xs font-bold text-ink-100">文风体检</h4>
                  <p class="text-[10px] text-ink-400">揪出AI味与错漏，去机械感</p>
                </div>
              </div>
              <button 
                @click="actions.testConfigConnection('reviewer')"
                :disabled="state.configTestStatus?.reviewer?.loading"
                class="px-2 py-0.5 text-[10px] bg-atelier-850 hover:bg-brand-amber/20 hover:text-brand-amber text-ink-300 border border-atelier-750 rounded transition cursor-pointer disabled:opacity-50">
                <span v-if="state.configTestStatus?.reviewer?.loading">测试中...</span>
                <span v-else>测试</span>
              </button>
            </div>

            <div>
              <label class="block text-[11px] text-ink-300 mb-1">模型名称：</label>
              <input 
                v-model="state.config.reviewer_model" 
                class="w-full bg-atelier-950 border border-atelier-750 rounded-md px-2.5 py-1.5 text-xs text-ink-100 font-mono focus:outline-none focus:border-brand-amber/60" 
                placeholder="deepseek-reasoner">
            </div>

            <!-- 独立接口开关 -->
            <div class="pt-2 border-t border-atelier-800">
              <label class="flex items-center gap-2 text-[11px] text-ink-300 cursor-pointer">
                <input 
                  type="checkbox" 
                  v-model="state.enableReviewerProvider" 
                  class="rounded border-atelier-700 bg-atelier-950 text-brand-amber accent-brand-amber">
                <span>独立接口 (支持异构连接)</span>
              </label>

              <div v-if="state.enableReviewerProvider" class="mt-2.5 p-2.5 bg-atelier-950 rounded-lg border border-atelier-800 space-y-2">
                <div>
                  <label class="block text-[10px] text-ink-400 mb-0.5">专属 Base URL：</label>
                  <input 
                    v-model="state.config.reviewer_provider.api_base" 
                    class="w-full bg-atelier-900 border border-atelier-750 rounded px-2 py-1 text-xs text-ink-100 font-mono focus:outline-none focus:border-brand-amber/60" 
                    placeholder="留空继承主接口">
                </div>
                <div>
                  <label class="block text-[10px] text-ink-400 mb-0.5">专属 API Key：</label>
                  <input 
                    type="password" 
                    v-model="state.config.reviewer_provider.api_key" 
                    class="w-full bg-atelier-900 border border-atelier-750 rounded px-2 py-1 text-xs text-ink-100 font-mono focus:outline-none focus:border-brand-amber/60" 
                    placeholder="留空继承主接口">
                </div>
                <div>
                  <label class="block text-[10px] text-ink-400 mb-0.5">专属 Model：</label>
                  <input 
                    v-model="state.config.reviewer_provider.model" 
                    class="w-full bg-atelier-900 border border-atelier-750 rounded px-2 py-1 text-xs text-ink-100 font-mono focus:outline-none focus:border-brand-amber/60" 
                    placeholder="留空使用上方模型">
                </div>
              </div>
            </div>
          </div>

          <!-- 诊断结果 -->
          <div v-if="state.configTestStatus?.reviewer" class="text-[10px] font-mono mt-2 pt-2 border-t border-atelier-800 flex items-center gap-1.5"
            :class="state.configTestStatus.reviewer.status === 'ok' ? 'text-emerald-400' : (state.configTestStatus.reviewer.status === 'error' ? 'text-rose-400' : 'text-ink-400')">
            <CheckCircle2 v-if="state.configTestStatus.reviewer.status === 'ok'" class="w-3 h-3 shrink-0" />
            <AlertCircle v-else-if="state.configTestStatus.reviewer.status === 'error'" class="w-3 h-3 shrink-0" />
            <Loader2 v-else class="w-3 h-3 animate-spin text-brand-amber shrink-0" />
            <span class="truncate" :title="state.configTestStatus.reviewer.message">{{ state.configTestStatus.reviewer.message }}</span>
          </div>
        </div>
      </div>
    </div>

    <!-- 底部小白贴心提示 -->
    <div class="p-4 bg-atelier-950/70 border border-atelier-800 rounded-xl flex items-center gap-3 text-xs text-ink-300">
      <div class="w-7 h-7 rounded-lg bg-brand-amber/10 border border-brand-amber/20 flex items-center justify-center text-brand-amber shrink-0">
        💡
      </div>
      <div>
        <span class="font-bold text-ink-100">新手建议：</span>
        只需在上方“主接口设置”中填入你的 API Key（例如 DeepSeek 或 OpenAI 兼容 Key），三大岗位即可自动通用，无需逐一设置。
      </div>
    </div>
  </div>
</template>

<script setup>
import { state, actions } from '../stores/appState';
import { 
  Settings, 
  Save, 
  Zap, 
  Loader2, 
  CheckCircle2, 
  AlertCircle, 
  Cpu, 
  Sparkles, 
  Lightbulb, 
  PenTool, 
  ShieldCheck 
} from 'lucide-vue-next';
</script>
