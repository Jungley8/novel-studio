<template>
  <div 
    v-if="state.showHarmonizeModal" 
    class="fixed inset-0 bg-black/80 backdrop-blur-sm flex items-center justify-center p-4 z-50">
    
    <div class="bg-atelier-900 border border-atelier-750 rounded-2xl max-w-4xl w-full max-h-[90vh] flex flex-col overflow-hidden shadow-2xl animate-fade-in">
      <!-- 弹窗头部 -->
      <div class="p-4 border-b border-atelier-750 flex items-center justify-between bg-atelier-950/60">
        <div class="flex items-center gap-2.5">
          <div class="w-7 h-7 rounded-lg bg-brand-emerald/15 border border-brand-emerald/30 flex items-center justify-center text-brand-emerald shadow-emerald-glow">
            <ShieldCheck class="w-3.5 h-3.5" />
          </div>
          <div>
            <h3 class="text-sm font-serif font-bold text-ink-50 tracking-wide">国内平台合规脱敏与去AI对抗扰动</h3>
            <p class="text-[10px] text-ink-400">过滤涉暴违规高危词，打散高概率 AI 动词与对称句式节奏，瓦解商业检测器似然特征</p>
          </div>
        </div>

        <button 
          @click="state.showHarmonizeModal = false" 
          class="text-ink-400 hover:text-ink-100 p-1 rounded-md transition cursor-pointer">
          <X class="w-4 h-4" />
        </button>
      </div>

      <!-- 参数调节与触发栏 -->
      <div class="p-4 bg-atelier-950 border-b border-atelier-800 flex flex-wrap items-center justify-between gap-4">
        <div class="flex items-center gap-4 flex-1 min-w-[280px]">
          <span class="text-xs text-ink-300 font-medium whitespace-nowrap flex items-center gap-1.5">
            <Sliders class="w-3.5 h-3.5 text-brand-amber" />
            <span>扰动强度 (Intensity):</span>
          </span>
          <input 
            type="range" 
            min="0.1" 
            max="1.0" 
            step="0.1" 
            v-model.number="intensity" 
            class="flex-1 accent-brand-amber cursor-pointer" />
          <span class="font-mono text-xs font-bold text-brand-amber w-8 text-right">{{ intensity }}</span>
          <span class="text-[10px] text-ink-500 hidden sm:inline">
            {{ intensity <= 0.3 ? '轻度和谐' : intensity >= 0.8 ? '深度打散节奏' : '平衡推荐' }}
          </span>
        </div>

        <button 
          @click="runHarmonize" 
          :disabled="isLoading || !state.workbench.content" 
          class="px-4 py-2 bg-brand-emerald hover:bg-emerald-500 text-atelier-950 font-bold text-xs rounded-md shadow-atelier-sm transition flex items-center gap-1.5 cursor-pointer disabled:opacity-40">
          <Loader2 v-if="isLoading" class="w-3.5 h-3.5 animate-spin" />
          <Sparkles v-else class="w-3.5 h-3.5" />
          <span>{{ isLoading ? '正在分析并扰动...' : '执行脱敏和谐与对抗扰动' }}</span>
        </button>
      </div>

      <!-- 内容主体 -->
      <div class="flex-1 p-6 overflow-y-auto space-y-5">
        <!-- 未运行状态提示 -->
        <div v-if="!result" class="p-12 text-center text-ink-400 text-xs flex flex-col items-center justify-center gap-2">
          <ShieldAlert class="w-6 h-6 text-ink-500" />
          <span>点击上方“执行脱敏和谐与对抗扰动”按钮，针对当前手稿进行合规审查与反检测重构</span>
        </div>

        <div v-else class="space-y-4">
          <!-- 核心统计横幅 -->
          <div class="grid grid-cols-2 sm:grid-cols-4 gap-3">
            <div class="p-3 bg-atelier-950 rounded-xl border border-atelier-800">
              <span class="text-[10px] font-mono text-ink-400 block">敏感/涉暴词和谐</span>
              <span class="text-base font-bold font-mono text-brand-emerald">
                {{ result.report?.harmonized_items?.length || 0 }} 处替换
              </span>
            </div>
            <div class="p-3 bg-atelier-950 rounded-xl border border-atelier-800">
              <span class="text-[10px] font-mono text-ink-400 block">人味建议生成</span>
              <span class="text-base font-bold font-mono text-brand-amber">
                {{ result.report?.suggested_touches?.length || 0 }} 条建议
              </span>
            </div>
            <div class="p-3 bg-atelier-950 rounded-xl border border-atelier-800">
              <span class="text-[10px] font-mono text-ink-400 block">处理前字数</span>
              <span class="text-base font-bold font-mono text-ink-200">
                {{ result.report?.original_word_count || state.workbench.content.length }} 字
              </span>
            </div>
            <div class="p-3 bg-atelier-950 rounded-xl border border-atelier-800">
              <span class="text-[10px] font-mono text-ink-400 block">处理后字数</span>
              <span class="text-base font-bold font-mono text-brand-emerald">
                {{ result.report?.processed_word_count || result.processed_content?.length || 0 }} 字
              </span>
            </div>
          </div>

          <!-- 敏感词替换清单 -->
          <div v-if="result.report?.harmonized_items?.length" class="p-4 bg-atelier-950 rounded-xl border border-atelier-800 space-y-2">
            <div class="flex items-center justify-between">
              <span class="text-xs font-serif font-bold text-ink-200 flex items-center gap-1.5">
                <ShieldCheck class="w-3.5 h-3.5 text-brand-emerald" />
                <span>国内平台敏感词平滑和谐明细：</span>
              </span>
              <span class="text-[10px] font-mono text-ink-400">已替换涉暴与违规短语</span>
            </div>
            <div class="flex flex-wrap gap-1.5 pt-1">
              <div 
                v-for="(item, idx) in result.report.harmonized_items" 
                :key="idx" 
                class="px-2 py-1 rounded bg-atelier-900 border border-atelier-750 text-xs flex items-center gap-1.5">
                <span class="line-through text-brand-rose/80 font-mono text-[11px]">{{ item.original }}</span>
                <span class="text-ink-400 text-[10px]">➔</span>
                <span class="text-brand-emerald font-semibold">{{ item.replacement }}</span>
                <span class="text-[9px] px-1 py-0.2 rounded bg-atelier-800 text-ink-400 font-mono">{{ item.category }}</span>
              </div>
            </div>
          </div>
          <div v-else class="p-3 bg-atelier-950/60 rounded-lg border border-atelier-800 text-xs text-brand-emerald flex items-center gap-2">
            <Check class="w-3.5 h-3.5" />
            <span>手稿中未检测到国内平台涉暴、低俗等高危违规关键词。</span>
          </div>

          <!-- 扰动后处理正文预览 -->
          <div class="space-y-2">
            <div class="flex items-center justify-between">
              <span class="text-xs font-serif font-bold text-ink-100">脱敏与去AI扰动后正文预览：</span>
              <button 
                @click="copyProcessed" 
                class="flex items-center gap-1 px-2.5 py-1 text-xs bg-atelier-850 hover:bg-atelier-800 text-brand-amber rounded border border-atelier-750 transition cursor-pointer">
                <Copy class="w-3 h-3" />
                <span>复制文本</span>
              </button>
            </div>
            <div class="p-4 bg-atelier-950 rounded-xl border border-atelier-800 text-xs text-ink-100 font-serif leading-relaxed max-h-64 overflow-y-auto whitespace-pre-wrap select-text">
              {{ result.processed_content }}
            </div>
          </div>
        </div>
      </div>

      <!-- 底部操作按钮 -->
      <div v-if="result" class="p-4 border-t border-atelier-750 bg-atelier-950/60 flex items-center justify-end gap-3">
        <button 
          @click="state.showHarmonizeModal = false" 
          class="px-4 py-2 bg-atelier-850 hover:bg-atelier-800 text-ink-300 rounded-md text-xs transition cursor-pointer">
          关闭
        </button>
        <button 
          @click="applyResult" 
          class="px-5 py-2 bg-brand-emerald hover:bg-emerald-500 text-atelier-950 font-bold rounded-md text-xs shadow-atelier-sm transition flex items-center gap-1.5 cursor-pointer">
          <Check class="w-3.5 h-3.5" />
          <span>✓ 一键采纳并替换当前手稿</span>
        </button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue';
import { state, actions, notify } from '../../stores/appState';
import { api } from '../../api/client';
import { ShieldCheck, ShieldAlert, Sparkles, Sliders, X, Check, Copy, Loader2 } from 'lucide-vue-next';

const intensity = ref(0.6);
const isLoading = ref(false);
const result = ref(null);

async function runHarmonize() {
  if (!state.currentProject || !state.workbench.content.trim()) {
    return notify('手稿为空', '请先在手稿区编写或生成章节正文', 'info');
  }
  isLoading.value = true;
  try {
    const res = await api.harmonize(state.currentProject.id, {
      content: state.workbench.content,
      intensity: intensity.value,
    });
    result.value = res;
    notify('合规与去AI扰动处理完毕', `检出并处理 ${res.report?.harmonized_items?.length || 0} 处词条`, 'success');
  } catch (e) {
    notify('处理失败', e.message, 'error');
  } finally {
    isLoading.value = false;
  }
}

function applyResult() {
  if (!result.value?.processed_content) return;
  state.workbench.content = result.value.processed_content;
  state.showHarmonizeModal = false;
  actions.runLinter();
  notify('已采纳扰动后手稿', '正文已更新，涉暴词与对称机械感已消除', 'success');
}

function copyProcessed() {
  if (!result.value?.processed_content) return;
  navigator.clipboard.writeText(result.value.processed_content);
  notify('已复制到剪贴板', '处理后文本已复制', 'success');
}
</script>
