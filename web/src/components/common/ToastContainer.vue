<template>
  <div class="fixed bottom-5 right-5 z-[100] flex flex-col gap-2 pointer-events-none max-w-sm w-full">
    <transition-group 
      enter-active-class="transform ease-out duration-300 transition"
      enter-from-class="translate-y-2 opacity-0 sm:translate-y-0 sm:translate-x-2"
      enter-to-class="translate-y-0 opacity-100 sm:translate-x-0"
      leave-active-class="transition ease-in duration-150"
      leave-from-class="opacity-100"
      leave-to-class="opacity-0">
      <div 
        v-for="toast in state.toasts" 
        :key="toast.id" 
        class="pointer-events-auto p-3.5 rounded-lg border shadow-atelier-md flex items-start gap-3 backdrop-blur-md"
        :class="{
          'bg-atelier-900/95 border-atelier-700 text-ink-100': toast.type === 'info',
          'bg-atelier-900/95 border-brand-emerald/40 text-ink-100': toast.type === 'success',
          'bg-atelier-900/95 border-brand-rose/40 text-ink-100': toast.type === 'error'
        }">
        <div class="shrink-0 mt-0.5">
          <span v-if="toast.type === 'success'" class="text-brand-emerald text-sm">✓</span>
          <span v-else-if="toast.type === 'error'" class="text-brand-rose text-sm">✕</span>
          <span v-else class="text-brand-amber text-sm">●</span>
        </div>
        <div class="flex-1 min-w-0">
          <p class="text-xs font-semibold leading-tight text-ink-100">{{ toast.title }}</p>
          <p v-if="toast.message" class="text-[11px] text-ink-300 mt-0.5 leading-normal line-clamp-2">{{ toast.message }}</p>
        </div>
      </div>
    </transition-group>
  </div>
</template>

<script setup>
import { state } from '../../stores/appState';
</script>
