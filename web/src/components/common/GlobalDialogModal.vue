<template>
  <transition
    enter-active-class="transition duration-150 ease-out"
    enter-from-class="opacity-0 scale-95"
    enter-to-class="opacity-100 scale-100"
    leave-active-class="transition duration-100 ease-in"
    leave-from-class="opacity-100 scale-100"
    leave-to-class="opacity-0 scale-95">
    <div 
      v-if="state.dialog.isOpen" 
      class="fixed inset-0 bg-black/80 backdrop-blur-md flex items-center justify-center p-4 z-[200] select-none"
      @click.self="handleCancel">
      
      <div 
        class="bg-atelier-900 border border-atelier-750 rounded-2xl max-w-md w-full p-5 space-y-4 shadow-2xl text-left"
        role="dialog"
        aria-modal="true">
        
        <!-- 弹窗顶栏 -->
        <div class="flex items-start justify-between gap-3">
          <div class="flex items-center gap-3">
            <div 
              class="w-9 h-9 rounded-xl flex items-center justify-center shrink-0 border"
              :class="iconContainerClass">
              <component :is="dialogIcon" class="w-4 h-4" />
            </div>
            <div>
              <h3 class="text-sm font-serif font-bold text-ink-50">
                {{ state.dialog.title }}
              </h3>
              <span class="text-[10px] font-mono uppercase tracking-wider text-ink-400">
                {{ dialogSubtypeLabel }}
              </span>
            </div>
          </div>

          <button 
            @click="handleCancel" 
            class="text-ink-400 hover:text-ink-100 p-1 rounded-lg hover:bg-atelier-850 transition cursor-pointer"
            title="关闭 (Esc)">
            <X class="w-4 h-4" />
          </button>
        </div>

        <!-- 提示文本与详细信息 -->
        <div class="space-y-2 text-xs">
          <p 
            v-if="state.dialog.message" 
            class="text-ink-200 leading-relaxed whitespace-pre-wrap font-sans text-[12.5px]">
            {{ state.dialog.message }}
          </p>

          <div 
            v-if="state.dialog.details" 
            class="p-2.5 bg-atelier-950 rounded-lg border border-atelier-800 text-[11px] text-ink-400 font-mono leading-relaxed whitespace-pre-wrap max-h-36 overflow-y-auto">
            {{ state.dialog.details }}
          </div>
        </div>

        <!-- Prompt 交互输入区 -->
        <div v-if="state.dialog.mode === 'prompt'" class="space-y-1.5 pt-1">
          <textarea 
            v-if="state.dialog.multiline"
            ref="inputRef"
            v-model="state.dialog.input"
            :placeholder="state.dialog.placeholder"
            rows="3"
            class="w-full bg-atelier-950 border border-atelier-750 focus:border-brand-amber/60 rounded-lg p-2.5 text-xs text-ink-100 focus:outline-none resize-none leading-relaxed"
            @keydown.ctrl.enter.prevent="handleConfirm"
            @keydown.meta.enter.prevent="handleConfirm"></textarea>
          <input 
            v-else
            ref="inputRef"
            v-model="state.dialog.input"
            :placeholder="state.dialog.placeholder"
            class="w-full bg-atelier-950 border border-atelier-750 focus:border-brand-amber/60 rounded-lg p-2.5 text-xs text-ink-100 focus:outline-none"
            @keydown.enter.prevent="handleConfirm" />
          
          <div v-if="state.dialog.multiline" class="flex justify-between text-[10px] text-ink-400">
            <span>按 Ctrl+Enter 或 ⌘+Enter 提交</span>
            <span>{{ state.dialog.input.length }} 字</span>
          </div>
        </div>

        <!-- 底部操作按钮 -->
        <div class="flex items-center justify-end gap-2 pt-2 border-t border-atelier-750/60">
          <button 
            v-if="state.dialog.mode !== 'alert'"
            @click="handleCancel"
            class="px-3.5 py-1.5 text-xs font-medium rounded-lg bg-atelier-850 hover:bg-atelier-800 text-ink-300 hover:text-ink-100 border border-atelier-750 transition cursor-pointer">
            {{ state.dialog.cancelText || '取消' }}
          </button>

          <button 
            ref="confirmBtnRef"
            @click="handleConfirm"
            class="px-4 py-1.5 text-xs rounded-lg transition shadow-atelier-sm flex items-center gap-1.5 cursor-pointer"
            :class="confirmBtnClass">
            <Check class="w-3.5 h-3.5" />
            <span>{{ state.dialog.confirmText || '确认' }}</span>
          </button>
        </div>

      </div>
    </div>
  </transition>
</template>

<script setup>
import { ref, computed, watch, nextTick, onMounted, onUnmounted } from 'vue';
import { state } from '../../stores/appState';
import {
  AlertTriangle,
  AlertCircle,
  Info,
  CheckCircle2,
  Sparkles,
  X,
  Check
} from 'lucide-vue-next';

const inputRef = ref(null);
const confirmBtnRef = ref(null);

const dialogIcon = computed(() => {
  if (state.dialog.mode === 'prompt') return Sparkles;
  if (state.dialog.type === 'danger') return AlertTriangle;
  if (state.dialog.type === 'warning') return AlertCircle;
  if (state.dialog.type === 'success') return CheckCircle2;
  return Info;
});

const iconContainerClass = computed(() => {
  if (state.dialog.mode === 'prompt') {
    return 'bg-brand-amber/15 text-brand-amber border-brand-amber/30';
  }
  switch (state.dialog.type) {
    case 'danger':
      return 'bg-brand-rose/15 text-brand-rose border-brand-rose/30';
    case 'warning':
      return 'bg-brand-amber/15 text-brand-amber border-brand-amber/30';
    case 'success':
      return 'bg-brand-emerald/15 text-brand-emerald border-brand-emerald/30';
    default:
      return 'bg-brand-cyan/15 text-brand-cyan border-brand-cyan/30';
  }
});

const dialogSubtypeLabel = computed(() => {
  if (state.dialog.mode === 'prompt') return 'Input Prompt';
  if (state.dialog.mode === 'alert') return 'System Alert';
  if (state.dialog.type === 'danger') return 'Destructive Action';
  if (state.dialog.type === 'warning') return 'Confirmation';
  return 'Notice';
});

const confirmBtnClass = computed(() => {
  if (state.dialog.type === 'danger') {
    return 'bg-brand-rose hover:bg-rose-600 text-white font-bold';
  }
  if (state.dialog.type === 'warning' || state.dialog.mode === 'prompt') {
    return 'bg-gradient-to-r from-brand-amber to-amber-600 hover:from-brand-amber-hover hover:to-amber-500 text-atelier-950 font-bold';
  }
  if (state.dialog.type === 'success') {
    return 'bg-brand-emerald hover:bg-emerald-500 text-atelier-950 font-bold';
  }
  return 'bg-atelier-850 hover:bg-atelier-800 text-ink-100 border border-atelier-700 font-bold';
});

function handleConfirm() {
  if (!state.dialog.isOpen) return;
  const resolve = state.dialog.resolve;
  if (state.dialog.mode === 'prompt') {
    if (resolve) resolve(state.dialog.input);
  } else if (state.dialog.mode === 'confirm') {
    if (resolve) resolve(true);
  } else {
    if (resolve) resolve();
  }
}

function handleCancel() {
  if (!state.dialog.isOpen) return;
  const resolve = state.dialog.resolve;
  if (state.dialog.mode === 'prompt') {
    if (resolve) resolve(null);
  } else if (state.dialog.mode === 'confirm') {
    if (resolve) resolve(false);
  } else {
    if (resolve) resolve();
  }
}

watch(() => state.dialog.isOpen, (open) => {
  if (open) {
    nextTick(() => {
      if (state.dialog.mode === 'prompt' && inputRef.value) {
        inputRef.value.focus();
        if (typeof inputRef.value.select === 'function') {
          inputRef.value.select();
        }
      } else if (confirmBtnRef.value) {
        confirmBtnRef.value.focus();
      }
    });
  }
});

function handleKeydown(e) {
  if (!state.dialog.isOpen) return;
  if (e.key === 'Escape') {
    e.preventDefault();
    handleCancel();
  } else if (e.key === 'Enter' && state.dialog.mode !== 'prompt') {
    e.preventDefault();
    handleConfirm();
  }
}

onMounted(() => {
  window.addEventListener('keydown', handleKeydown);
});

onUnmounted(() => {
  window.removeEventListener('keydown', handleKeydown);
});
</script>
