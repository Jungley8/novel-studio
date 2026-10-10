<template>
  <div class="flex-1 flex flex-col min-h-0 bg-atelier-950 overflow-hidden">
    <!-- 头部工具栏 -->
    <div class="p-4 border-b border-atelier-750 bg-atelier-900/60 backdrop-blur-sm space-y-3 shrink-0">
      <div class="flex flex-col md:flex-row md:items-center justify-between gap-3">
        <div class="flex items-center gap-2.5">
          <div class="w-8 h-8 rounded-lg bg-brand-amber/10 border border-brand-amber/25 flex items-center justify-center text-brand-amber shadow-amber-glow">
            <MessagesSquare class="w-4 h-4" />
          </div>
          <div>
            <div class="flex items-center gap-2">
              <h2 class="text-base font-serif font-bold text-ink-50 tracking-wide">Studio 全局对话</h2>
            </div>
            <p class="text-xs text-ink-400 mt-0.5">跨项目宏观灵感闲聊、情节碰撞与题材头脑风暴。</p>
          </div>
        </div>

        <!-- 参考作品上下文选择器与推演模式 -->
        <div class="flex items-center gap-2.5 flex-wrap">
          <div class="flex items-center gap-1.5 text-xs">
            <span class="text-ink-400 font-mono text-[11px] whitespace-nowrap">参考 Horizon:</span>
            <select 
              v-model="activeContextProjectId"
              class="bg-atelier-850 border border-atelier-700 rounded-md px-2.5 py-1.5 text-xs text-ink-100 font-medium focus:outline-none focus:border-brand-amber/60 cursor-pointer max-w-[200px] truncate">
              <option value="">-- 全局通用脑暴 (不挂载单书) --</option>
              <option 
                v-for="p in state.projects" 
                :key="p.id" 
                :value="p.id">
                {{ p.title }} ({{ p.target_platform || '通用' }})
              </option>
            </select>
          </div>

          <div class="flex items-center gap-1.5 text-xs">
            <span class="text-ink-400 font-mono text-[11px] whitespace-nowrap">推演模式:</span>
            <select 
              v-model="selectedMode"
              class="bg-atelier-850 border border-atelier-700 rounded-md px-2.5 py-1.5 text-xs text-ink-100 font-medium focus:outline-none focus:border-brand-amber/60 cursor-pointer">
              <option value="scene_brainstorm">剧情风暴 (Brainstorm)</option>
              <option value="character_roleplay">角色推演 (Roleplay)</option>
              <option value="editor_critique">总编评审 (Critique)</option>
            </select>
          </div>

          <button 
            @click="clearMessages" 
            :disabled="messages.length === 0"
            class="px-2.5 py-1.5 text-xs text-ink-400 hover:text-ink-200 bg-atelier-850 hover:bg-atelier-800 rounded-md border border-atelier-750 transition cursor-pointer disabled:opacity-40">
            清空记录
          </button>
        </div>
      </div>

      <!-- 演进史：原借用单项目 /api/projects/:id/chat (scene_brainstorm) 顶替，现已升级为原生 POST /api/studio/chat 跨项目聚合对话接口 -->
      <div class="p-3 bg-brand-amber/10 border border-brand-amber/25 rounded-xl text-xs text-ink-200 flex items-start gap-2.5">
        <Sparkles class="w-4 h-4 text-brand-amber shrink-0 mt-0.5" />
        <div class="space-y-0.5 leading-relaxed">
          <div class="font-semibold text-brand-amber text-[11px]">
            {{ !activeContextProjectId ? '已注入全库作品概览上下文（跨项目脑暴）' : 'Studio 原生跨项目对话智囊已就绪' }}
          </div>
          <div class="text-[11px] text-ink-300">
            <span v-if="!activeContextProjectId">系统已自动注入全库所有作品的题材、立意、归档进度与开放伏笔概览，支持跨项目全局灵感碰撞。</span>
            <span v-else>已挂载《{{ currentContextProjectTitle }}》单书 Horizon 上下文，原生接入 <code>POST /api/studio/chat</code> 服务。</span>
          </div>
        </div>
      </div>
    </div>

    <!-- 正常对话聊天工作区 -->
    <div class="flex-1 flex flex-col min-h-0">
      <!-- 消息气泡流容器 -->
      <div ref="chatContainer" class="flex-1 p-5 overflow-y-auto space-y-4">
        <!-- 对话初始引导卡片 -->
        <div v-if="messages.length === 0" class="py-12 text-center space-y-4 max-w-lg mx-auto">
          <div class="w-12 h-12 rounded-xl bg-atelier-900 border border-atelier-750 flex items-center justify-center mx-auto text-brand-amber shadow-atelier-sm">
            <Sparkles class="w-6 h-6" />
          </div>
          <div class="space-y-1">
            <h3 class="text-sm font-semibold text-ink-100 font-serif">开启全局灵感头脑风暴</h3>
            <p class="text-xs text-ink-400 leading-relaxed">
              当前挂载上下文：<strong class="text-brand-amber">{{ currentContextProjectTitle }}</strong>。你可以探讨设定融合、高潮反转、伏笔设计或文风基调。
            </p>
          </div>

          <!-- 快速灵感提问卡片 -->
          <div class="pt-2 text-left space-y-2">
            <div class="text-[10px] font-mono text-ink-400 text-center">点击快速展开推演：</div>
            <div class="grid grid-cols-1 gap-2">
              <button 
                v-for="preset in quickPrompts" 
                :key="preset"
                @click="applyQuickPrompt(preset)"
                class="p-2.5 bg-atelier-900 hover:bg-atelier-850 border border-atelier-750 hover:border-brand-amber/40 rounded-lg text-xs text-ink-300 hover:text-ink-100 text-left transition cursor-pointer">
                💡 {{ preset }}
              </button>
            </div>
          </div>
        </div>

        <!-- 历史对话气泡 -->
        <div 
          v-for="(msg, idx) in messages" 
          :key="idx" 
          class="flex flex-col space-y-1"
          :class="msg.role === 'user' ? 'items-end' : 'items-start'">
          
          <div class="flex items-center gap-2 text-[10px] font-mono text-ink-500 px-1">
            <span>{{ msg.role === 'user' ? '作者 (你)' : '工坊智囊 (' + modeName(msg.mode) + ')' }}</span>
            <span v-if="msg.sourceProject" class="text-brand-amber/80">· {{ msg.sourceProject }}</span>
            <span>{{ msg.timestamp }}</span>
          </div>

          <!-- 用户消息 -->
          <div 
            v-if="msg.role === 'user'" 
            class="p-3.5 rounded-2xl rounded-tr-none bg-brand-amber text-atelier-950 font-medium text-xs leading-relaxed max-w-[85%] shadow-amber-glow whitespace-pre-wrap">
            {{ msg.content }}
          </div>

          <!-- AI 回复 -->
          <div 
            v-else-if="msg.role === 'assistant'" 
            class="p-4 rounded-2xl rounded-tl-none bg-atelier-900 text-ink-100 border border-atelier-750 text-xs leading-relaxed max-w-[88%] shadow-atelier-sm font-serif whitespace-pre-wrap">
            {{ msg.content }}
          </div>

          <!-- 错误提示气泡 (防白屏) -->
          <div 
            v-else-if="msg.role === 'error'" 
            class="p-3 rounded-xl bg-rose-500/10 border border-rose-500/30 text-rose-300 text-xs max-w-[85%] flex items-center gap-2">
            <AlertTriangle class="w-4 h-4 text-rose-400 shrink-0" />
            <span>{{ msg.content }}</span>
          </div>
        </div>

        <!-- 思考推演中指示器 -->
        <div v-if="isLoading" class="flex items-center gap-2 text-xs text-brand-amber py-2 px-1">
          <span class="w-3.5 h-3.5 border-2 border-brand-amber border-t-transparent rounded-full animate-spin"></span>
          <span class="text-[11px] font-mono">工坊智囊正在深度推演中...</span>
        </div>
      </div>

      <!-- 底部输入栏 -->
      <div class="p-4 border-t border-atelier-750 bg-atelier-900/80 backdrop-blur-sm space-y-2 shrink-0">
        <textarea 
          v-model="inputContent" 
          @keydown.enter.exact.prevent="sendMessage" 
          rows="3" 
          class="w-full bg-atelier-950 border border-atelier-750 rounded-xl p-3 text-xs text-ink-100 placeholder-ink-500 resize-none focus:outline-none focus:border-brand-amber/60 font-sans leading-relaxed" 
          placeholder="输入全局灵感探讨与情节构思 (Enter 发送, Shift+Enter 换行)..."></textarea>
        
        <div class="flex items-center justify-between">
          <div class="text-[10px] font-mono text-ink-400 flex items-center gap-2">
            <span>当前接口: <code>POST /api/studio/chat</code> ({{ selectedMode }})</span>
          </div>

          <button 
            @click="sendMessage" 
            :disabled="isLoading || !inputContent.trim()" 
            class="px-4 py-1.5 bg-brand-amber hover:bg-brand-amber-hover text-atelier-950 font-bold text-xs rounded-lg shadow-amber-glow transition disabled:opacity-40 cursor-pointer flex items-center gap-1.5">
            <Send class="w-3.5 h-3.5" />
            <span>发送推演</span>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch, nextTick, onMounted } from 'vue';
import { state, notify } from '../stores/appState';
import { api } from '../api/client';
import { 
  MessagesSquare, 
  Sparkles, 
  Send, 
  AlertTriangle 
} from 'lucide-vue-next';

const activeContextProjectId = ref('');
const selectedMode = ref('scene_brainstorm');
const messages = ref([]);
const inputContent = ref('');
const isLoading = ref(false);
const chatContainer = ref(null);

const quickPrompts = [
  '结合当前作品题材，如何设计一个反套路又具备高强情绪张力的核心冲突？',
  '请为主角构思一个开篇第 1 章的章末钩子 (Cliffhanger)，使读者产生强烈追更欲。',
  '如果引入不可名状的高维存在，世界底层公理与战力阶梯应如何保持自洽？',
];

const currentContextProject = computed(() => {
  return state.projects.find(p => p.id === activeContextProjectId.value);
});

const currentContextProjectTitle = computed(() => {
  return currentContextProject.value?.title || '全局通用（未挂载单书）';
});

function modeName(mode) {
  switch (mode) {
    case 'character_roleplay':
      return '角色推演';
    case 'editor_critique':
      return '总编评审';
    default:
      return '剧情风暴';
  }
}

// 初始化上下文作品，优先使用已选项目
function initContextProject() {
  if (state.selectedProjectId && state.projects.some(p => p.id === state.selectedProjectId)) {
    activeContextProjectId.value = state.selectedProjectId;
  }
}

watch(() => state.selectedProjectId, (newId) => {
  if (newId && !activeContextProjectId.value) {
    activeContextProjectId.value = newId;
  }
});

function applyQuickPrompt(text) {
  inputContent.value = text;
}

function clearMessages() {
  messages.value = [];
}

async function scrollToBottom() {
  await nextTick();
  if (chatContainer.value) {
    chatContainer.value.scrollTop = chatContainer.value.scrollHeight;
  }
}

async function sendMessage() {
  const text = inputContent.value.trim();
  if (!text || isLoading.value) return;

  const projTitle = currentContextProjectTitle.value;
  const currentMode = selectedMode.value;

  messages.value.push({
    role: 'user',
    content: text,
    timestamp: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
  });

  inputContent.value = '';
  isLoading.value = true;
  await scrollToBottom();

  try {
    // 演进史：原借用单项目 /api/projects/:id/chat (scene_brainstorm) 顶替，现已升级为原生 POST /api/studio/chat 跨项目聚合对话接口
    const payload = {
      project_id: activeContextProjectId.value || undefined,
      mode: currentMode,
      message: text,
    };

    const res = await api.studioChat(payload);
    const replyText = typeof res === 'string' ? res : (res?.reply || JSON.stringify(res));

    messages.value.push({
      role: 'assistant',
      content: replyText,
      mode: currentMode,
      sourceProject: activeContextProjectId.value ? projTitle : '全局脑暴',
      timestamp: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
    });
  } catch (err) {
    console.error('StudioChatView sendMessage error:', err);
    messages.value.push({
      role: 'error',
      content: `推演回复失败: ${err.message || '网络连接超时或后端服务未就绪'}`,
      timestamp: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
    });
    notify('对话推演异常', err.message || '网络断开或后端无响应', 'error');
  } finally {
    isLoading.value = false;
    await scrollToBottom();
  }
}

onMounted(() => {
  initContextProject();
});
</script>
