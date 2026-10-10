<template>
  <div class="flex-1 flex flex-col min-h-0 bg-atelier-950 overflow-hidden">
    <!-- 头部工具栏与过渡方案警示栏 -->
    <div class="p-4 border-b border-atelier-750 bg-atelier-900/60 backdrop-blur-sm space-y-3 shrink-0">
      <div class="flex flex-col md:flex-row md:items-center justify-between gap-3">
        <div class="flex items-center gap-2.5">
          <div class="w-8 h-8 rounded-lg bg-brand-amber/10 border border-brand-amber/25 flex items-center justify-center text-brand-amber shadow-amber-glow">
            <MessagesSquare class="w-4 h-4" />
          </div>
          <div>
            <div class="flex items-center gap-2">
              <h2 class="text-base font-serif font-bold text-ink-50 tracking-wide">Studio 全局对话</h2>
              <span class="text-[9px] font-mono px-1.5 py-0.5 rounded bg-brand-amber/15 text-brand-amber border border-brand-amber/30">
                TRANSITIONAL
              </span>
            </div>
            <p class="text-xs text-ink-400 mt-0.5">跨项目宏观灵感闲聊、情节碰撞与题材头脑风暴。</p>
          </div>
        </div>

        <!-- 参考作品上下文选择器与操作 -->
        <div class="flex items-center gap-2.5">
          <div class="flex items-center gap-1.5 text-xs">
            <span class="text-ink-400 font-mono text-[11px] whitespace-nowrap">参考 Horizon:</span>
            <select 
              v-model="activeContextProjectId"
              :disabled="state.projects.length === 0"
              class="bg-atelier-850 border border-atelier-700 rounded-md px-2.5 py-1.5 text-xs text-ink-100 font-medium focus:outline-none focus:border-brand-amber/60 cursor-pointer max-w-[200px] truncate">
              <option v-if="state.projects.length === 0" value="">-- 暂无作品工程 --</option>
              <option 
                v-for="p in state.projects" 
                :key="p.id" 
                :value="p.id">
                {{ p.title }} ({{ p.target_platform || '通用' }})
              </option>
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

      <!-- 过渡方案规范提示条 (明确标注这是过渡方案，等待 POST /api/studio/chat) -->
      <!-- TODO: 等待后端完成 POST /api/studio/chat 原生跨项目聚合对话接口 -->
      <div class="p-3 bg-brand-amber/10 border border-brand-amber/25 rounded-xl text-xs text-ink-200 flex items-start gap-2.5">
        <Sparkles class="w-4 h-4 text-brand-amber shrink-0 mt-0.5" />
        <div class="space-y-0.5 leading-relaxed">
          <div class="font-semibold text-brand-amber text-[11px]">
            跨项目灵感对话（过渡方案）说明
          </div>
          <div class="text-[11px] text-ink-300">
            当前复用现有单项目 <code>scene_brainstorm</code> (剧情风暴) 接口作为全局知识源，允许选择单书的设定 Horizon 作为上下文。本模块严格使用现有接口，等待后端上线原生 <code>POST /api/studio/chat</code> 聚合对话服务。
          </div>
        </div>
      </div>
    </div>

    <!-- 零作品工程空态 (防白屏) -->
    <div 
      v-if="state.projects.length === 0" 
      class="flex-1 flex flex-col items-center justify-center p-6 text-center space-y-4">
      <div class="w-14 h-14 rounded-2xl bg-atelier-900 border border-atelier-750 flex items-center justify-center text-ink-400 shadow-atelier-sm">
        <MessagesSquare class="w-7 h-7" />
      </div>
      <div class="space-y-1.5 max-w-md">
        <h3 class="text-sm font-semibold text-ink-100 font-serif">暂无可用作品工程上下文</h3>
        <p class="text-xs text-ink-400 leading-relaxed">
          当前过渡方案依赖现有项目级 <code>/api/projects/:id/chat</code> 接口推演。在建立第一部小说工程前无法挂载 Horizon 上下文。你可以先创建作品，或在灵感 Inbox 记录点子。
        </p>
      </div>
      <div class="flex items-center gap-3 pt-2">
        <button 
          @click="state.showNewProjectModal = true" 
          class="px-4 py-2 bg-brand-amber hover:bg-brand-amber-hover text-atelier-950 font-bold text-xs rounded-lg shadow-amber-glow transition cursor-pointer">
          新建第一部作品
        </button>
        <button 
          @click="state.activeTab = 'studio-ideas'" 
          class="px-4 py-2 bg-atelier-850 hover:bg-atelier-800 text-ink-300 hover:text-ink-100 text-xs rounded-lg border border-atelier-700 transition cursor-pointer">
          前往灵感 Inbox
        </button>
      </div>
    </div>

    <!-- 正常对话聊天工作区 -->
    <template v-else>
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
            <span>{{ msg.role === 'user' ? '作者 (你)' : '工坊智囊 (剧情风暴)' }}</span>
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
          <span class="text-[11px] font-mono">工坊智囊正在结合 Horizon 设定深度推演...</span>
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
            <span>当前接口: <code>/api/projects/:id/chat</code> (scene_brainstorm)</span>
          </div>

          <button 
            @click="sendMessage" 
            :disabled="isLoading || !inputContent.trim() || !activeContextProjectId" 
            class="px-4 py-1.5 bg-brand-amber hover:bg-brand-amber-hover text-atelier-950 font-bold text-xs rounded-lg shadow-amber-glow transition disabled:opacity-40 cursor-pointer flex items-center gap-1.5">
            <Send class="w-3.5 h-3.5" />
            <span>发送推演</span>
          </button>
        </div>
      </div>
    </template>
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
  return currentContextProject.value?.title || '未选择作品';
});

// 初始化上下文作品，优先使用已选项目或列表第一个
function initContextProject() {
  if (state.selectedProjectId && state.projects.some(p => p.id === state.selectedProjectId)) {
    activeContextProjectId.value = state.selectedProjectId;
  } else if (state.projects.length > 0) {
    activeContextProjectId.value = state.projects[0].id;
  }
}

watch(() => state.selectedProjectId, (newId) => {
  if (newId && !activeContextProjectId.value) {
    activeContextProjectId.value = newId;
  }
});

watch(() => state.projects, () => {
  if (!activeContextProjectId.value && state.projects.length > 0) {
    initContextProject();
  }
}, { immediate: true });

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

  if (!activeContextProjectId.value) {
    notify('缺少上下文作品', '请先选择一个作品工程作为知识源 Horizon', 'warning');
    return;
  }

  const projTitle = currentContextProjectTitle.value;
  messages.value.push({
    role: 'user',
    content: text,
    timestamp: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
  });

  inputContent.value = '';
  isLoading.value = true;
  await scrollToBottom();

  try {
    // 复用现有 WorkshopChat 的 scene_brainstorm 接口做知识源
    const payload = {
      role_mode: 'scene_brainstorm',
      character_id: '',
      message: text,
      chapter_index: 1, // 宏观闲聊以全书第 1 章/宏观大纲为参考
    };

    const res = await api.workshopChat(activeContextProjectId.value, payload);
    const replyText = typeof res === 'string' ? res : (res?.reply || JSON.stringify(res));

    messages.value.push({
      role: 'assistant',
      content: replyText,
      sourceProject: projTitle,
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
