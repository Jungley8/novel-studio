<template>
  <div class="flex-1 flex min-h-0 bg-atelier-950 overflow-hidden select-text">
    <!-- ========================================== -->
    <!-- 左侧栏：会话历史与灵感 Inbox 合二为一       -->
    <!-- ========================================== -->
    <aside class="w-64 border-r border-atelier-750 bg-atelier-900/40 flex flex-col min-h-0 shrink-0">
      <!-- 顶部 "+ 新对话" 与快速操作 -->
      <div class="p-3 border-b border-atelier-750 shrink-0 space-y-2 select-none">
        <button 
          @click="createNewThread" 
          :disabled="isLoading"
          class="w-full flex items-center justify-center gap-2 px-3 py-2 bg-brand-amber/10 hover:bg-brand-amber/20 text-brand-amber border border-brand-amber/30 hover:border-brand-amber/50 rounded-lg text-xs font-semibold transition cursor-pointer disabled:opacity-40 shadow-atelier-xs">
          <Plus class="w-3.5 h-3.5" />
          <span>开启新对话</span>
        </button>

        <!-- 双流切换器：对话历史 vs 灵感 Inbox -->
        <div class="grid grid-cols-2 p-0.5 bg-atelier-950 rounded-lg border border-atelier-750/80 text-[11px] font-mono">
          <button 
            @click="activeLeftTab = 'threads'"
            :class="activeLeftTab === 'threads' 
              ? 'bg-atelier-800 text-brand-amber font-semibold shadow-sm' 
              : 'text-ink-400 hover:text-ink-200'"
            class="py-1 px-1.5 rounded transition flex items-center justify-center gap-1 cursor-pointer">
            <MessagesSquare class="w-3 h-3" />
            <span>会话 ({{ threads.length }})</span>
          </button>
          <button 
            @click="activeLeftTab = 'inbox'"
            :class="activeLeftTab === 'inbox' 
              ? 'bg-atelier-800 text-brand-amber font-semibold shadow-sm' 
              : 'text-ink-400 hover:text-ink-200'"
            class="py-1 px-1.5 rounded transition flex items-center justify-center gap-1 cursor-pointer">
            <Lightbulb class="w-3 h-3" />
            <span>灵感 ({{ ideas.length }})</span>
          </button>
        </div>
      </div>

      <!-- ========================================== -->
      <!-- 视图 A：会话历史列表                       -->
      <!-- ========================================== -->
      <div v-show="activeLeftTab === 'threads'" class="flex-1 flex flex-col min-h-0">
        <div class="px-3 pt-2.5 pb-1 flex items-center justify-between text-[11px] font-mono text-ink-400 select-none">
          <span>历史会话</span>
          <span v-if="threads.length" class="text-ink-500 text-[10px]">{{ threads.length }} 条记录</span>
        </div>

        <div class="flex-1 overflow-y-auto px-2 pb-2 space-y-1">
          <!-- 加载中指示器 -->
          <div v-if="threadsLoading" class="py-6 text-center text-xs text-ink-400 space-y-2 select-none">
            <span class="inline-block w-4 h-4 border-2 border-brand-amber border-t-transparent rounded-full animate-spin"></span>
            <p class="text-[11px] font-mono text-ink-500">加载会话中...</p>
          </div>

          <!-- 线程列表为空时的空态文案 -->
          <div v-else-if="threads.length === 0" class="py-8 px-2 text-center space-y-2 text-ink-500 select-none">
            <MessagesSquare class="w-5 h-5 mx-auto text-ink-600 mb-1" />
            <p class="text-xs font-medium text-ink-400">暂无历史对话</p>
            <p class="text-[11px] text-ink-500 leading-relaxed">开启对话后系统将自动保存推演历史</p>
          </div>

          <!-- 线程列表项 -->
          <div 
            v-else
            v-for="t in threads" 
            :key="t.id"
            @click="selectThread(t.id)"
            class="group relative flex flex-col gap-1 p-2 rounded-lg border text-xs transition cursor-pointer"
            :class="currentThreadId === t.id 
              ? 'bg-brand-amber/15 border-brand-amber/40 text-ink-50 shadow-atelier-xs' 
              : 'border-transparent text-ink-300 hover:bg-atelier-850/60 hover:text-ink-100 hover:border-atelier-750/50'">
            
            <div class="flex items-center justify-between gap-1.5 min-w-0">
              <div class="flex items-center gap-1.5 min-w-0 flex-1">
                <MessageSquare 
                  class="w-3.5 h-3.5 shrink-0" 
                  :class="currentThreadId === t.id ? 'text-brand-amber' : 'text-ink-500 group-hover:text-ink-400'" />
                <span class="truncate font-medium text-xs" :title="t.title || '新对话'">
                  {{ t.title || '新对话' }}
                </span>
              </div>

              <!-- hover 显示删除按钮 -->
              <button 
                @click.stop="deleteThread(t.id, t.title)"
                :disabled="isLoading"
                class="opacity-0 group-hover:opacity-100 p-1 rounded hover:bg-atelier-750/60 text-ink-400 hover:text-rose-400 transition cursor-pointer shrink-0 select-none"
                title="删除此会话">
                <Trash2 class="w-3.5 h-3.5" />
              </button>
            </div>

            <div class="flex items-center justify-between text-[10px] font-mono text-ink-500 pl-5 select-none">
              <span>{{ formatRelativeTime(t.updated_at || t.created_at) }}</span>
              <span v-if="t.message_count !== undefined" class="bg-atelier-800/80 px-1.5 py-0.2 rounded border border-atelier-750/40 text-ink-400">
                {{ t.message_count }}条
              </span>
            </div>
          </div>
        </div>
      </div>

      <!-- ========================================== -->
      <!-- 视图 B：灵感 Inbox (沉淀与快速转创世)       -->
      <!-- ========================================== -->
      <div v-show="activeLeftTab === 'inbox'" class="flex-1 flex flex-col min-h-0">
        <!-- 灵感快速录入卡片 -->
        <div class="p-2.5 border-b border-atelier-750 bg-atelier-950/40 space-y-1.5 select-none">
          <div class="flex items-center justify-between text-[10px] font-mono text-ink-400">
            <span class="flex items-center gap-1">
              <Sparkles class="w-3 h-3 text-brand-amber" />
              <span>捕捉新灵感</span>
            </span>
            <span class="text-ink-500">{{ pendingIdeasCount }} 待转</span>
          </div>
          <input 
            v-model="newIdeaForm.title" 
            placeholder="灵感书名 / 概念代号..." 
            class="w-full bg-atelier-900 border border-atelier-750 rounded px-2 py-1 text-xs text-ink-100 placeholder-ink-500 focus:outline-none focus:border-brand-amber/60 font-serif" />
          <div class="flex items-center gap-1.5">
            <input 
              v-model="newIdeaForm.pitch" 
              @keydown.enter="addIdeaQuick"
              placeholder="一句话梗概 / 核心卖点..." 
              class="flex-1 bg-atelier-900 border border-atelier-750 rounded px-2 py-1 text-[11px] text-ink-200 placeholder-ink-500 focus:outline-none focus:border-brand-amber/60" />
            <button 
              @click="addIdeaQuick" 
              :disabled="!newIdeaForm.title.trim()"
              class="px-2.5 py-1 bg-brand-amber hover:bg-brand-amber-hover text-atelier-950 font-bold text-[11px] rounded transition cursor-pointer disabled:opacity-40 shrink-0">
              收录
            </button>
          </div>
        </div>

        <!-- 灵感列表 -->
        <div class="flex-1 overflow-y-auto p-2 space-y-2">
          <div v-if="ideas.length === 0" class="py-8 text-center text-ink-500 space-y-2 select-none">
            <Lightbulb class="w-5 h-5 mx-auto text-ink-600 mb-1" />
            <p class="text-xs font-medium text-ink-400">灵感 Inbox 暂空</p>
            <p class="text-[11px] text-ink-500 leading-relaxed">在对话推演中点击「收录为灵感」，或在上方快速收录脑洞</p>
          </div>

          <div 
            v-else
            v-for="idea in ideas" 
            :key="idea.id"
            class="p-2.5 bg-atelier-900/90 border rounded-lg space-y-1.5 transition text-xs group"
            :class="idea.status === 'GENESIS' ? 'border-emerald-500/30' : 'border-atelier-750 hover:border-brand-amber/40'">
            <div class="flex items-center justify-between gap-1 select-none">
              <span class="font-bold font-serif text-ink-100 group-hover:text-brand-amber transition-colors truncate">
                {{ idea.title }}
              </span>
              <span 
                :class="idea.status === 'GENESIS' ? 'bg-emerald-500/15 text-emerald-400 border-emerald-500/30' : 'bg-brand-amber/15 text-brand-amber border-brand-amber/30'"
                class="text-[9px] font-mono px-1 py-0.2 rounded border shrink-0">
                {{ idea.status === 'GENESIS' ? '已转创世' : '灵感' }}
              </span>
            </div>

            <p v-if="idea.pitch" class="text-[11px] text-ink-300 leading-relaxed line-clamp-2">
              {{ idea.pitch }}
            </p>

            <div class="flex items-center justify-between pt-1 border-t border-atelier-800 text-[10px] select-none">
              <span class="font-mono text-ink-500">{{ idea.platform || '通用' }}</span>
              <div class="flex items-center gap-1">
                <button 
                  @click="bringIdeaToChat(idea)"
                  class="px-1.5 py-0.5 rounded bg-atelier-800 hover:bg-brand-amber/20 hover:text-brand-amber text-ink-300 transition cursor-pointer"
                  title="将该灵感带入右侧会话继续深度推演">
                  探讨
                </button>
                <button 
                  @click="convertToGenesis(idea)"
                  class="px-1.5 py-0.5 rounded bg-brand-amber/15 hover:bg-brand-amber hover:text-atelier-950 text-brand-amber font-semibold transition cursor-pointer"
                  title="一键跃迁至全书创世大纲推演">
                  转创世
                </button>
                <button 
                  @click="deleteIdea(idea.id)"
                  class="p-0.5 text-ink-500 hover:text-rose-400 transition cursor-pointer"
                  title="移除灵感">
                  <Trash2 class="w-3 h-3" />
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </aside>

    <!-- ========================================== -->
    <!-- 右侧主对话区：支持 Markdown 优雅排版与复制  -->
    <!-- ========================================== -->
    <div class="flex-1 flex flex-col min-h-0 min-w-0 bg-atelier-950 overflow-hidden">
      <!-- 头部工具栏 -->
      <div class="p-3.5 border-b border-atelier-750 bg-atelier-900/60 backdrop-blur-sm space-y-2.5 shrink-0 select-none">
        <div class="flex flex-col md:flex-row md:items-center justify-between gap-3">
          <div class="flex items-center gap-2.5">
            <div class="w-8 h-8 rounded-lg bg-brand-amber/10 border border-brand-amber/25 flex items-center justify-center text-brand-amber shadow-amber-glow">
              <MessagesSquare class="w-4 h-4" />
            </div>
            <div>
              <div class="flex items-center gap-2">
                <h2 class="text-sm font-serif font-bold text-ink-50 tracking-wide">对话记录</h2>
                <span class="text-[10px] font-mono px-1.5 py-0.2 rounded bg-atelier-800 text-ink-300 border border-atelier-700">
                  灵感推演 & Inbox 统一中枢
                </span>
              </div>
              <p class="text-[11px] text-ink-400 mt-0.5">跨项目宏观灵感闲聊、设定碰撞，在对话中随时沉淀灵感并转创世。</p>
            </div>
          </div>

          <!-- 参考作品上下文选择器与推演模式 -->
          <div class="flex items-center gap-2 flex-wrap">
            <div class="flex items-center gap-1.5 text-xs">
              <span class="text-ink-400 font-mono text-[11px] whitespace-nowrap">参考 Horizon:</span>
              <select 
                v-model="activeContextProjectId"
                class="bg-atelier-850 border border-atelier-750 rounded-md px-2 py-1 text-xs text-ink-100 font-medium focus:outline-none focus:border-brand-amber/60 cursor-pointer max-w-[180px] truncate">
                <option value="">-- 全局通用脑暴 --</option>
                <option 
                  v-for="p in state.projects" 
                  :key="p.id" 
                  :value="p.id">
                  {{ p.title }} ({{ p.target_platform || '通用' }})
                </option>
              </select>
            </div>

            <div class="flex items-center gap-1.5 text-xs">
              <span class="text-ink-400 font-mono text-[11px] whitespace-nowrap">模式:</span>
              <select 
                v-model="selectedMode"
                class="bg-atelier-850 border border-atelier-750 rounded-md px-2 py-1 text-xs text-ink-100 font-medium focus:outline-none focus:border-brand-amber/60 cursor-pointer">
                <option value="scene_brainstorm">💡 剧情风暴</option>
                <option value="character_roleplay">🎭 角色推演</option>
                <option value="editor_critique">🧐 总编评审</option>
              </select>
            </div>
          </div>
        </div>
      </div>

      <!-- 消息流工作区 -->
      <div 
        ref="chatContainer"
        class="flex-1 overflow-y-auto p-4 md:p-6 space-y-5 select-text">
        <!-- 会话加载中骨架 -->
        <div v-if="isThreadDetailLoading" class="py-20 text-center space-y-2 select-none">
          <span class="inline-block w-6 h-6 border-2 border-brand-amber border-t-transparent rounded-full animate-spin"></span>
          <p class="text-xs font-mono text-ink-400">正在载入会话记录...</p>
        </div>

        <!-- 空消息引导卡片 -->
        <div v-else-if="messages.length === 0" class="py-12 text-center space-y-4 max-w-lg mx-auto select-none">
          <div class="w-12 h-12 rounded-xl bg-atelier-900 border border-atelier-750 flex items-center justify-center mx-auto text-brand-amber shadow-atelier-sm">
            <Sparkles class="w-6 h-6" />
          </div>
          <div class="space-y-1">
            <h3 class="text-sm font-semibold text-ink-100 font-serif">开启灵感碰撞与脑洞推演</h3>
            <p class="text-xs text-ink-400 leading-relaxed">
              当前挂载：<strong class="text-brand-amber">{{ currentContextProjectTitle }}</strong>。对话中产生的新灵感支持一键收录到左侧 Inbox，并一键跃迁至创世建书。
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

        <!-- 历史与实时对话气泡 -->
        <div 
          v-else
          v-for="(msg, idx) in messages" 
          :key="msg.id || idx" 
          class="flex flex-col space-y-1.5"
          :class="msg.role === 'user' ? 'items-end' : 'items-start'">
          
          <div class="flex items-center gap-2 text-[10px] font-mono text-ink-500 px-1 select-none">
            <span>{{ msg.role === 'user' ? '作者 (你)' : '工坊智囊 (' + modeName(msg.mode) + ')' }}</span>
            <span v-if="msg.sourceProject" class="text-brand-amber/80">· {{ msg.sourceProject }}</span>
            <span>{{ msg.timestamp }}</span>
          </div>

          <!-- 用户消息气泡 -->
          <div 
            v-if="msg.role === 'user'" 
            class="p-3.5 rounded-2xl rounded-tr-none bg-brand-amber text-atelier-950 font-medium text-xs leading-relaxed max-w-[85%] shadow-amber-glow whitespace-pre-wrap select-text">
            {{ msg.content }}
          </div>

          <!-- AI 回复气泡 (深度 Markdown 渲染 + 工具操作条) -->
          <div 
            v-else-if="msg.role === 'assistant'" 
            class="p-4 rounded-2xl rounded-tl-none bg-atelier-900 text-ink-100 border border-atelier-750 text-xs max-w-[92%] shadow-atelier-sm space-y-3 group select-text">
            
            <!-- Markdown 语义化美观排版区 -->
            <div 
              class="markdown-body select-text" 
              v-html="renderMarkdown(msg.content)">
            </div>

            <!-- 底部操作条：一键复制 / 收录为灵感 / 一键转创世 -->
            <div class="pt-2 border-t border-atelier-800 flex items-center justify-between text-[11px] font-mono select-none">
              <span class="text-ink-500 text-[10px]">
                {{ msg.content.length }} 字符 · 支持随时提炼
              </span>

              <div class="flex items-center gap-2">
                <!-- 一键复制 Markdown 文本 -->
                <button 
                  @click="handleCopyMessage(msg)"
                  class="flex items-center gap-1 px-2 py-0.5 rounded bg-atelier-800 hover:bg-atelier-750 text-ink-300 hover:text-ink-100 transition cursor-pointer"
                  title="复制原始 Markdown 格式文本">
                  <Check v-if="copiedMessageId === (msg.id || idx)" class="w-3 h-3 text-emerald-400" />
                  <Copy v-else class="w-3 h-3 text-ink-400" />
                  <span>{{ copiedMessageId === (msg.id || idx) ? '已复制' : '复制' }}</span>
                </button>

                <!-- 一键收录进灵感 Inbox -->
                <button 
                  @click="saveMessageAsIdea(msg)"
                  class="flex items-center gap-1 px-2 py-0.5 rounded bg-brand-amber/10 hover:bg-brand-amber/20 text-brand-amber transition cursor-pointer"
                  title="将本条回复提炼并收录进左侧灵感 Inbox">
                  <Lightbulb class="w-3 h-3" />
                  <span>收录灵感</span>
                </button>

                <!-- 一键直接转创世建书 -->
                <button 
                  @click="convertMessageToGenesis(msg)"
                  class="flex items-center gap-1 px-2 py-0.5 rounded bg-brand-amber/20 hover:bg-brand-amber hover:text-atelier-950 text-brand-amber font-semibold transition cursor-pointer"
                  title="以此脑洞直接唤起新书创世推演">
                  <Sparkles class="w-3 h-3" />
                  <span>转创世</span>
                </button>
              </div>
            </div>
          </div>

          <!-- 错误提示气泡 -->
          <div 
            v-else-if="msg.role === 'error'" 
            class="p-3 rounded-xl bg-rose-500/10 border border-rose-500/30 text-rose-300 text-xs max-w-[85%] flex items-center gap-2 select-text">
            <AlertTriangle class="w-4 h-4 text-rose-400 shrink-0" />
            <span>{{ msg.content }}</span>
          </div>
        </div>

        <!-- 思考推演中指示器 -->
        <div v-if="isLoading" class="flex items-center gap-2 text-xs text-brand-amber py-2 px-1 select-none">
          <span class="w-3.5 h-3.5 border-2 border-brand-amber border-t-transparent rounded-full animate-spin"></span>
          <span class="text-[11px] font-mono">工坊智囊正在因果推演中...</span>
        </div>
      </div>

      <!-- 底部输入栏 -->
      <div class="p-3.5 border-t border-atelier-750 bg-atelier-900/80 backdrop-blur-sm space-y-2 shrink-0 select-none">
        <textarea 
          v-model="inputContent" 
          @keydown.enter.exact.prevent="sendMessage" 
          rows="3" 
          class="w-full bg-atelier-950 border border-atelier-750 rounded-xl p-3 text-xs text-ink-100 placeholder-ink-500 resize-none focus:outline-none focus:border-brand-amber/60 font-sans leading-relaxed select-text" 
          placeholder="输入灵感脑洞推演与情节构思 (Enter 发送, Shift+Enter 换行)..."></textarea>
        
        <div class="flex items-center justify-between">
          <div class="text-[10px] font-mono text-ink-400 flex items-center gap-2">
            <span>当前接口: <code>POST /api/studio/chat</code></span>
            <span>· 文本全选复制已就绪</span>
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
import { renderMarkdown, copyToClipboard } from '../utils/markdown';
import { 
  MessagesSquare, 
  Sparkles, 
  Send, 
  AlertTriangle, 
  Plus, 
  MessageSquare, 
  Trash2,
  Lightbulb,
  Copy,
  Check
} from 'lucide-vue-next';

const STORAGE_IDEAS_KEY = 'novel-studio-ideas';

// 界面状态
const activeLeftTab = ref('threads'); // 'threads' | 'inbox'
const activeContextProjectId = ref('');
const selectedMode = ref('scene_brainstorm');
const messages = ref([]);
const inputContent = ref('');
const isLoading = ref(false);
const chatContainer = ref(null);
const copiedMessageId = ref(null);

// 线程持久化相关
const threads = ref([]);
const currentThreadId = ref(null);
const threadsLoading = ref(false);
const isThreadDetailLoading = ref(false);

// 灵感 Inbox 相关
const ideas = ref([]);
const newIdeaForm = ref({
  title: '',
  pitch: '',
  platform: '番茄脑洞',
});

const pendingIdeasCount = computed(() => {
  return ideas.value.filter(i => i.status !== 'GENESIS').length;
});

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

// 灵感数据存取
function loadStoredIdeas() {
  if (typeof window === 'undefined') return;
  try {
    const raw = localStorage.getItem(STORAGE_IDEAS_KEY);
    if (raw) {
      const parsed = JSON.parse(raw);
      if (Array.isArray(parsed)) {
        ideas.value = parsed;
      }
    }
  } catch (e) {
    console.warn('Failed to parse ideas from localStorage:', e);
  }
}

function persistIdeas() {
  if (typeof window === 'undefined') return;
  try {
    localStorage.setItem(STORAGE_IDEAS_KEY, JSON.stringify(ideas.value));
  } catch (e) {
    console.warn('Failed to persist ideas:', e);
  }
}

function addIdeaQuick() {
  const title = newIdeaForm.value.title.trim();
  if (!title) return;

  const item = {
    id: `idea_${Date.now()}_${Math.random().toString(36).substring(2, 6)}`,
    title,
    pitch: newIdeaForm.value.pitch.trim(),
    platform: newIdeaForm.value.platform || '番茄脑洞',
    status: 'IDEA',
    created_at: new Date().toISOString(),
  };

  ideas.value.unshift(item);
  persistIdeas();

  newIdeaForm.value.title = '';
  newIdeaForm.value.pitch = '';
  notify('灵感已收录', `${title} 已存入 Inbox`, 'success');
}

function deleteIdea(id) {
  ideas.value = ideas.value.filter(i => i.id !== id);
  persistIdeas();
  notify('灵感已移除', '', 'info', 2000);
}

function convertToGenesis(idea) {
  idea.status = 'GENESIS';
  persistIdeas();

  copyToClipboard(`${idea.title}\n${idea.pitch}`);

  state.newProjectPrefill = {
    title: idea.title,
    concept: idea.pitch || '',
    platform: idea.platform || '',
  };
  state.showNewProjectModal = true;

  notify('已转入创世工序', `《${idea.title}》已唤起创世推演弹窗`, 'success');
}

function bringIdeaToChat(idea) {
  activeLeftTab.value = 'threads';
  inputContent.value = `关于我的灵感设想《${idea.title}》（${idea.platform}）：\n${idea.pitch || ''}\n\n请针对这一设定，帮我推演核心看点、金手指机制与前三章主线脉络。`;
}

// 从对话回复直接存为灵感
function saveMessageAsIdea(msg) {
  const text = msg.content || '';
  const firstLine = text.split('\n')[0].replace(/^[#*\s-]+/, '').trim() || '未命名推演灵感';
  const cleanTitle = firstLine.length > 20 ? firstLine.slice(0, 20) + '...' : firstLine;

  const item = {
    id: `idea_${Date.now()}_${Math.random().toString(36).substring(2, 6)}`,
    title: cleanTitle,
    pitch: text.slice(0, 300),
    platform: currentContextProject.value?.target_platform || '通用网文',
    status: 'IDEA',
    created_at: new Date().toISOString(),
  };

  ideas.value.unshift(item);
  persistIdeas();
  activeLeftTab.value = 'inbox';
  notify('已收录为灵感', `《${cleanTitle}》已存入左侧 Inbox`, 'success');
}

// 从对话回复直接转创世
function convertMessageToGenesis(msg) {
  const text = msg.content || '';
  const firstLine = text.split('\n')[0].replace(/^[#*\s-]+/, '').trim() || '推演项目';
  const cleanTitle = firstLine.length > 20 ? firstLine.slice(0, 20) : firstLine;

  state.newProjectPrefill = {
    title: cleanTitle,
    concept: text.slice(0, 500),
    platform: currentContextProject.value?.target_platform || '番茄脑洞',
  };
  state.showNewProjectModal = true;
  notify('已唤起创世推演', '对话设定已自动预填', 'success');
}

// 复制消息内容
async function handleCopyMessage(msg) {
  const ok = await copyToClipboard(msg.content);
  if (ok) {
    copiedMessageId.value = msg.id || messages.value.indexOf(msg);
    setTimeout(() => {
      copiedMessageId.value = null;
    }, 2000);
    notify('复制成功', 'Markdown 内容已复制到剪贴板', 'success', 2000);
  }
}

// 时间解析与格式化
function parseDate(dateInput) {
  if (!dateInput) return null;
  if (dateInput instanceof Date) return dateInput;
  if (typeof dateInput === 'number') {
    return new Date(dateInput > 1e11 ? dateInput : dateInput * 1000);
  }
  if (typeof dateInput === 'string' && /^\d+$/.test(dateInput)) {
    const num = Number(dateInput);
    return new Date(num > 1e11 ? num : num * 1000);
  }
  const parsed = new Date(dateInput);
  return isNaN(parsed.getTime()) ? null : parsed;
}

function formatRelativeTime(dateInput) {
  const date = parseDate(dateInput);
  if (!date) return dateInput ? String(dateInput) : '';

  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffSec = Math.floor(diffMs / 1000);

  if (diffSec < 60) return '刚刚';
  const diffMin = Math.floor(diffSec / 60);
  if (diffMin < 60) return `${diffMin} 分钟前`;
  const diffHour = Math.floor(diffMin / 60);
  if (diffHour < 24) return `${diffHour} 小时前`;
  const diffDay = Math.floor(diffHour / 24);
  if (diffDay < 7) return `${diffDay} 天前`;
  if (diffDay < 30) return `${Math.floor(diffDay / 7)} 周前`;

  const y = date.getFullYear();
  const m = String(date.getMonth() + 1).padStart(2, '0');
  const d = String(date.getDate()).padStart(2, '0');
  return `${y}-${m}-${d}`;
}

function formatMessageTime(dateInput) {
  const date = parseDate(dateInput);
  if (!date) return dateInput ? String(dateInput) : '';

  const now = new Date();
  const isToday = now.toDateString() === date.toDateString();
  const timeStr = date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
  if (isToday) return timeStr;
  const m = date.getMonth() + 1;
  const d = date.getDate();
  return `${m}/${d} ${timeStr}`;
}

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

function createNewThread() {
  if (isLoading.value) return;
  currentThreadId.value = null;
  messages.value = [];
}

async function scrollToBottom() {
  await nextTick();
  if (chatContainer.value) {
    chatContainer.value.scrollTop = chatContainer.value.scrollHeight;
  }
}

async function fetchThreads() {
  threadsLoading.value = true;
  try {
    const res = await api.listStudioThreads();
    threads.value = Array.isArray(res?.threads) ? res.threads : [];
  } catch (err) {
    console.error('StudioChatView fetchThreads error:', err);
    threads.value = [];
  } finally {
    threadsLoading.value = false;
  }
}

async function selectThread(id) {
  if (isLoading.value) return;
  if (currentThreadId.value === id) return;

  currentThreadId.value = id;
  isThreadDetailLoading.value = true;
  try {
    const res = await api.getStudioThread(id);
    if (res?.thread && res.thread.project_id !== undefined && res.thread.project_id !== null) {
      activeContextProjectId.value = res.thread.project_id || '';
    }
    const rawMsgs = Array.isArray(res?.messages) ? res.messages : [];
    messages.value = rawMsgs.map(m => {
      let sourceProj = null;
      if (m.role === 'assistant') {
        if (m.project_id) {
          const p = state.projects.find(proj => proj.id === m.project_id);
          sourceProj = p ? p.title : m.project_id;
        } else {
          sourceProj = '全局脑暴';
        }
      }
      return {
        id: m.id,
        role: m.role,
        content: m.content,
        mode: m.mode || selectedMode.value,
        sourceProject: sourceProj,
        timestamp: formatMessageTime(m.created_at),
      };
    });
    await scrollToBottom();
  } catch (err) {
    console.error('StudioChatView selectThread error:', err);
    notify('加载会话失败', err.message || '网络异常或会话已不存在', 'error');
  } finally {
    isThreadDetailLoading.value = false;
  }
}

async function deleteThread(id, title = '') {
  if (isLoading.value) return;
  const displayName = title ? `「${title}」` : '该对话';
  if (!window.confirm(`确定删除${displayName}吗？`)) {
    return;
  }
  try {
    await api.deleteStudioThread(id);
    notify('会话已删除', displayName, 'info');
    if (currentThreadId.value === id) {
      currentThreadId.value = null;
      messages.value = [];
    }
    await fetchThreads();
  } catch (err) {
    console.error('StudioChatView deleteThread error:', err);
    notify('删除会话失败', err.message || '网络异常', 'error');
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
    timestamp: formatMessageTime(new Date()),
  });

  inputContent.value = '';
  isLoading.value = true;
  await scrollToBottom();

  try {
    const payload = {
      project_id: activeContextProjectId.value || undefined,
      mode: currentMode,
      message: text,
      thread_id: currentThreadId.value || undefined,
    };

    const res = await api.studioChat(payload);
    const replyText = typeof res === 'string' ? res : (res?.reply || JSON.stringify(res));

    messages.value.push({
      role: 'assistant',
      content: replyText,
      mode: res?.mode || currentMode,
      sourceProject: activeContextProjectId.value ? projTitle : '全局脑暴',
      timestamp: formatMessageTime(new Date()),
    });

    if (!currentThreadId.value && res?.thread_id) {
      currentThreadId.value = res.thread_id;
      await fetchThreads();
    } else {
      await fetchThreads();
    }
  } catch (err) {
    console.error('StudioChatView sendMessage error:', err);
    messages.value.push({
      role: 'error',
      content: `推演回复失败: ${err.message || '网络连接超时或后端服务未就绪'}`,
      timestamp: formatMessageTime(new Date()),
    });
    notify('对话推演异常', err.message || '网络断开或后端无响应', 'error');
  } finally {
    isLoading.value = false;
    await scrollToBottom();
  }
}

onMounted(async () => {
  loadStoredIdeas();
  initContextProject();
  await fetchThreads();
});
</script>
