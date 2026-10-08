<template>
  <div 
    v-if="state.showWorkshopChatDrawer" 
    class="fixed inset-y-0 right-0 w-96 bg-atelier-900 border-l border-atelier-750 flex flex-col z-50 shadow-2xl animate-fade-in">
    
    <!-- 抽屉头部 -->
    <div class="p-3.5 border-b border-atelier-750 flex items-center justify-between bg-atelier-950/60">
      <div class="flex items-center gap-2">
        <div class="w-6 h-6 rounded bg-brand-amber/15 text-brand-amber flex items-center justify-center">
          <MessageSquare class="w-3.5 h-3.5" />
        </div>
        <h3 class="text-xs font-serif font-bold text-ink-50 tracking-wide">情境工坊对话 (Workshop Chat)</h3>
      </div>
      <button 
        @click="state.showWorkshopChatDrawer = false" 
        class="text-ink-400 hover:text-ink-100 p-1 rounded transition cursor-pointer">
        <X class="w-4 h-4" />
      </button>
    </div>

    <!-- 角色模式分栏 -->
    <div class="p-3 border-b border-atelier-750 bg-atelier-950/40 space-y-2.5">
      <div class="grid grid-cols-3 gap-1 bg-atelier-900 p-0.5 rounded-lg border border-atelier-750 text-[11px]">
        <button 
          v-for="mode in modes" 
          :key="mode.id"
          @click="chatRole = mode.id"
          :class="chatRole === mode.id 
            ? 'bg-atelier-800 text-brand-amber font-semibold shadow-atelier-sm' 
            : 'text-ink-400 hover:text-ink-200'"
          class="py-1 rounded-md transition text-center cursor-pointer">
          {{ mode.label }}
        </button>
      </div>

      <!-- 角色扮演绑定选择 -->
      <div v-if="chatRole === 'character_roleplay'">
        <label class="block text-[10px] text-ink-400 mb-1">选择扮演人物：</label>
        <select 
          v-model="chatCharacterId" 
          class="w-full bg-atelier-850 border border-atelier-750 rounded-md px-2.5 py-1 text-xs text-ink-100 focus:outline-none focus:border-brand-amber/60 cursor-pointer">
          <option value="">-- 全局主角 (顾渊/默认设定) --</option>
          <option 
            v-for="c in computedState.characterCodexList.value" 
            :key="c.id" 
            :value="c.id">
            {{ c.name }} ({{ c.summary }})
          </option>
        </select>
      </div>
    </div>

    <!-- 消息对话记录气泡流 -->
    <div ref="messagesContainer" class="flex-1 p-3.5 overflow-y-auto space-y-3">
      <div v-if="messages.length === 0" class="text-center py-16 text-ink-500 text-xs space-y-1">
        <Sparkles class="w-6 h-6 mx-auto text-atelier-700 mb-2" />
        <p>开启专属 AI 沉浸式对话推演</p>
        <span class="text-[10px] text-ink-600">系统将自动注入百科人设与当前章节心境</span>
      </div>

      <div 
        v-for="(msg, idx) in messages" 
        :key="idx" 
        class="flex flex-col space-y-1" 
        :class="msg.role === 'user' ? 'items-end' : 'items-start'">
        <span class="text-[10px] font-mono text-ink-500">
          {{ msg.role === 'user' ? '作者' : (chatRole === 'character_roleplay' ? '角色应答' : '工坊智囊') }}
        </span>
        <div 
          :class="[
            'p-3 rounded-xl text-xs leading-relaxed max-w-[88%] whitespace-pre-wrap',
            msg.role === 'user' 
              ? 'bg-brand-amber text-atelier-950 font-medium rounded-tr-none shadow-amber-glow' 
              : 'bg-atelier-850 text-ink-100 border border-atelier-750 rounded-tl-none font-serif'
          ]">
          {{ msg.content }}
        </div>
      </div>

      <div v-if="isLoading" class="flex items-center gap-2 text-xs text-brand-amber py-1">
        <span class="w-3 h-3 border-2 border-brand-amber border-t-transparent rounded-full animate-spin"></span>
        <span class="text-[11px] font-mono">思考推演中...</span>
      </div>
    </div>

    <!-- 底部输入栏 -->
    <div class="p-3 border-t border-atelier-750 bg-atelier-950/70 space-y-2">
      <textarea 
        v-model="inputContent" 
        @keydown.enter.exact.prevent="sendMessage" 
        rows="2" 
        class="w-full bg-atelier-900 border border-atelier-750 rounded-lg p-2.5 text-xs text-ink-100 resize-none focus:outline-none focus:border-brand-amber/60 font-sans" 
        placeholder="输入对话推演内容 (Enter 发送, Shift+Enter 换行)..."></textarea>
      
      <div class="flex items-center justify-between">
        <button 
          @click="clearMessages" 
          class="text-[11px] text-ink-500 hover:text-ink-300 transition cursor-pointer">
          清空记录
        </button>
        <button 
          @click="sendMessage" 
          :disabled="isLoading || !inputContent.trim()" 
          class="px-3.5 py-1.5 bg-brand-amber hover:bg-brand-amber-hover text-atelier-950 font-bold text-xs rounded-md shadow-amber-glow transition disabled:opacity-50 cursor-pointer">
          发送
        </button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, nextTick } from 'vue';
import { state, computedState, notify } from '../../stores/appState';
import { api } from '../../api/client';
import { MessageSquare, X, Sparkles } from 'lucide-vue-next';

const modes = [
  { id: 'character_roleplay', label: '🎭 角色扮演' },
  { id: 'scene_brainstorm', label: '💡 剧情风暴' },
  { id: 'editor_critique', label: '🧐 总编评审' },
];

const chatRole = ref('character_roleplay');
const chatCharacterId = ref('');
const messages = ref([]);
const inputContent = ref('');
const isLoading = ref(false);
const messagesContainer = ref(null);

function clearMessages() {
  messages.value = [];
}

async function sendMessage() {
  const text = inputContent.value.trim();
  if (!text || isLoading.value || !state.currentProject) return;

  messages.value.push({ role: 'user', content: text });
  inputContent.value = '';
  isLoading.value = true;
  await scrollToBottom();

  try {
    const payload = {
      role_mode: chatRole.value,
      character_id: chatCharacterId.value,
      message: text,
      chapter_index: state.chapters.length + 1,
    };
    const res = await api.workshopChat(state.currentProject.id, payload);
    messages.value.push({ role: 'assistant', content: res.reply || res });
  } catch (e) {
    notify('对话回复失败', e.message, 'error');
  } finally {
    isLoading.value = false;
    await scrollToBottom();
  }
}

async function scrollToBottom() {
  await nextTick();
  if (messagesContainer.value) {
    messagesContainer.value.scrollTop = messagesContainer.value.scrollHeight;
  }
}
</script>
