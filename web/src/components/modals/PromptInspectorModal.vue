<template>
  <div 
    v-if="state.showPromptInspectorModal" 
    class="fixed inset-0 bg-black/80 backdrop-blur-sm flex items-center justify-center p-4 z-50">
    
    <div class="bg-atelier-900 border border-atelier-750 rounded-2xl max-w-4xl w-full max-h-[90vh] flex flex-col overflow-hidden shadow-2xl animate-fade-in">
      <!-- 弹窗头部 -->
      <div class="p-4 border-b border-atelier-750 flex items-center justify-between bg-atelier-950/60">
        <div class="flex items-center gap-2.5">
          <div class="w-7 h-7 rounded-lg bg-brand-amber/15 border border-brand-amber/30 flex items-center justify-center text-brand-amber shadow-amber-glow">
            <Search class="w-3.5 h-3.5" />
          </div>
          <div>
            <h3 class="text-sm font-serif font-bold text-ink-50 tracking-wide">提示词积木装配透视 (Prompt Inspector)</h3>
            <p class="text-[10px] text-ink-400">底层发往大模型的四层解耦组装视界与 Token 精确测算</p>
          </div>
        </div>

        <div class="flex items-center gap-3">
          <span 
            v-if="previewData" 
            class="px-2.5 py-1 rounded font-mono text-xs font-bold bg-brand-amber/15 text-brand-amber border border-brand-amber/30 shadow-amber-glow">
            预估 Token: {{ previewData.total_tokens_estimate?.toLocaleString() || 0 }}
          </span>
          <button 
            @click="state.showPromptInspectorModal = false" 
            class="text-ink-400 hover:text-ink-100 p-1 rounded-md transition cursor-pointer">
            <X class="w-4 h-4" />
          </button>
        </div>
      </div>

      <!-- 内容主体 -->
      <div class="flex-1 p-6 overflow-y-auto space-y-5" v-if="previewData">
        <!-- 4 大积木组件网格 -->
        <div class="grid grid-cols-1 md:grid-cols-2 gap-3.5">
          <div 
            v-for="(comp, idx) in previewData.components" 
            :key="idx" 
            class="p-4 bg-atelier-950 rounded-xl border border-atelier-800 space-y-2">
            <div class="flex items-center justify-between">
              <span class="text-xs font-serif font-bold text-brand-amber">{{ comp.name }}</span>
              <span class="text-[10px] font-mono px-2 py-0.5 rounded bg-atelier-850 text-ink-400 border border-atelier-750">
                约 {{ comp.token_estimate }} tokens
              </span>
            </div>
            <p class="text-[10px] text-ink-500 font-sans">{{ comp.description }}</p>
            <pre class="p-2.5 bg-atelier-900 rounded-lg text-[11px] text-ink-300 font-mono max-h-36 overflow-y-auto whitespace-pre-wrap leading-relaxed border border-atelier-800">{{ comp.content }}</pre>
          </div>
        </div>

        <!-- 组装完毕完整 Prompt -->
        <div class="pt-3 border-t border-atelier-800 space-y-3">
          <div class="flex items-center justify-between">
            <span class="text-xs font-serif font-bold text-ink-100">发往大模型的真实全量 Prompt 纯文本：</span>
            <button 
              @click="copyPrompt" 
              class="flex items-center gap-1.5 px-3 py-1 text-xs bg-atelier-850 hover:bg-atelier-800 text-brand-amber rounded-md border border-atelier-750 transition cursor-pointer font-medium">
              <Copy class="w-3.5 h-3.5" />
              <span>复制完整 Prompt</span>
            </button>
          </div>

          <div class="grid grid-cols-1 lg:grid-cols-2 gap-3">
            <div>
              <span class="text-[11px] font-mono text-ink-400 font-bold block mb-1">System Prompt:</span>
              <pre class="p-3 bg-atelier-950 rounded-lg text-[10px] text-ink-200 font-mono max-h-48 overflow-y-auto whitespace-pre-wrap leading-relaxed border border-atelier-800">{{ previewData.system_prompt }}</pre>
            </div>
            <div>
              <span class="text-[11px] font-mono text-ink-400 font-bold block mb-1">User Prompt (组装后):</span>
              <pre class="p-3 bg-atelier-950 rounded-lg text-[10px] text-ink-200 font-mono max-h-48 overflow-y-auto whitespace-pre-wrap leading-relaxed border border-atelier-800">{{ previewData.user_prompt }}</pre>
            </div>
          </div>
        </div>
      </div>

      <!-- 加载状态 -->
      <div v-else class="p-16 text-center text-ink-400 text-xs flex flex-col items-center justify-center gap-2">
        <span class="w-5 h-5 border-2 border-brand-amber border-t-transparent rounded-full animate-spin"></span>
        <span>正在装配当前视界动态提示词...</span>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, watch } from 'vue';
import { state, notify } from '../../stores/appState';
import { api } from '../../api/client';
import { Search, X, Copy } from 'lucide-vue-next';

const previewData = ref(null);

watch(() => state.showPromptInspectorModal, async (open) => {
  if (open && state.currentProject) {
    previewData.value = null;
    try {
      const payload = {
        chapter_index: state.chapters.length + 1,
        core_conflict: state.workbench.coreConflict || '探索世界',
        beats: state.workbench.beats,
        words_target: state.wordsTarget,
      };
      previewData.value = await api.promptPreview(state.currentProject.id, payload);
    } catch (e) {
      notify('装配提示词失败', e.message, 'error');
    }
  }
});

function copyPrompt() {
  if (!previewData.value) return;
  const full = `${previewData.value.system_prompt}\n\n---\n\n${previewData.value.user_prompt}`;
  navigator.clipboard.writeText(full);
  notify('已复制到剪贴板', '完整 System 与 User Prompt 已就绪', 'success');
}
</script>
