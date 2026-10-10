<template>
  <div class="flex-1 p-6 overflow-y-auto space-y-6">
    <!-- 头部工具栏 -->
    <div class="flex flex-col md:flex-row md:items-center justify-between gap-4 border-b border-atelier-750 pb-5">
      <div>
        <div class="flex items-center gap-2.5">
          <div class="w-8 h-8 rounded-lg bg-brand-amber/10 border border-brand-amber/25 flex items-center justify-center text-brand-amber shadow-amber-glow">
            <Lightbulb class="w-4 h-4" />
          </div>
          <div>
            <h2 class="text-base font-serif font-bold text-ink-50 tracking-wide">灵感 Inbox</h2>
            <p class="text-xs text-ink-400 mt-0.5">轻量随笔记录故事脑洞与核心冲突，一键跃迁至创世推演。</p>
          </div>
        </div>
      </div>

      <div class="flex items-center gap-2.5">
        <div class="flex items-center gap-2 text-xs font-mono bg-atelier-850 px-3 py-1.5 rounded-md border border-atelier-750">
          <span class="text-ink-400">待转灵感</span>
          <strong class="text-brand-amber">{{ pendingIdeasCount }}</strong>
          <span class="text-atelier-700">/</span>
          <span class="text-ink-400">已转创世</span>
          <span class="text-emerald-400">{{ genesisIdeasCount }}</span>
        </div>
      </div>
    </div>

    <!-- 快速录入卡片 (Inbox Quick Form) -->
    <div class="p-4 bg-atelier-900 border border-atelier-750 rounded-xl space-y-3 shadow-atelier-sm">
      <div class="flex items-center justify-between">
        <span class="text-xs font-semibold text-ink-100 font-serif flex items-center gap-1.5">
          <Sparkles class="w-3.5 h-3.5 text-brand-amber" />
          <span>捕捉新灵感</span>
        </span>
        <span class="text-[10px] font-mono text-ink-400">本地安全存储 · 永久离线保存</span>
      </div>

      <div class="grid grid-cols-1 md:grid-cols-12 gap-3 items-start">
        <div class="md:col-span-4">
          <input 
            v-model="newIdeaForm.title" 
            class="w-full bg-atelier-950 border border-atelier-750 rounded-lg px-3 py-2 text-xs text-ink-100 font-serif font-bold focus:outline-none focus:border-brand-amber/60" 
            placeholder="灵感书名 / 高概念代号 (如: 神经元飞升)...">
        </div>

        <div class="md:col-span-3">
          <select 
            v-model="newIdeaForm.platform" 
            class="w-full bg-atelier-950 border border-atelier-750 rounded-lg px-2.5 py-2 text-xs text-ink-200 focus:outline-none focus:border-brand-amber/60 cursor-pointer">
            <option value="番茄脑洞">番茄脑洞 (快节奏 / 密集反转)</option>
            <option value="起点仙侠">起点仙侠 (严谨战力 / 世界观)</option>
            <option value="知乎盐言">知乎盐言 (强悬疑 / 情绪高压)</option>
            <option value="海外短剧">海外短剧 (高概念 / 强钩锁)</option>
            <option value="通用网文">通用网文 (经典升级流)</option>
          </select>
        </div>

        <div class="md:col-span-4">
          <input 
            v-model="newIdeaForm.pitch" 
            @keydown.enter="addIdea"
            class="w-full bg-atelier-950 border border-atelier-750 rounded-lg px-3 py-2 text-xs text-ink-200 focus:outline-none focus:border-brand-amber/60" 
            placeholder="一句话梗概 / 核心卖点 / 反套路设定...">
        </div>

        <div class="md:col-span-1">
          <button 
            @click="addIdea" 
            :disabled="!newIdeaForm.title.trim()"
            class="w-full py-2 bg-brand-amber hover:bg-brand-amber-hover text-atelier-950 font-bold text-xs rounded-lg shadow-amber-glow transition flex items-center justify-center gap-1 cursor-pointer disabled:opacity-50">
            <Plus class="w-3.5 h-3.5" />
            <span>收录</span>
          </button>
        </div>
      </div>
    </div>

    <!-- 过滤器与统计栏 -->
    <div class="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3">
      <div class="flex items-center gap-1 bg-atelier-900 p-1 rounded-lg border border-atelier-750 text-xs">
        <button 
          v-for="tab in filterTabs" 
          :key="tab.id"
          @click="currentFilter = tab.id"
          :class="currentFilter === tab.id 
            ? 'bg-atelier-800 text-brand-amber font-semibold shadow-atelier-sm' 
            : 'text-ink-400 hover:text-ink-200'" 
          class="px-3 py-1.5 rounded-md transition cursor-pointer flex items-center gap-1.5">
          <span>{{ tab.label }}</span>
          <span class="text-[10px] font-mono opacity-80">({{ tab.count }})</span>
        </button>
      </div>

      <div class="relative w-full sm:w-64">
        <input 
          v-model="searchQuery" 
          type="text" 
          placeholder="搜索灵感标题或一句话..." 
          class="w-full bg-atelier-900 border border-atelier-750 rounded-lg pl-8 pr-3 py-1.5 text-xs text-ink-200 focus:outline-none focus:border-brand-amber/60">
        <Search class="w-3.5 h-3.5 text-ink-400 absolute left-2.5 top-2 pointer-events-none" />
      </div>
    </div>

    <!-- 空态：无任何灵感 -->
    <div 
      v-if="filteredIdeas.length === 0" 
      class="text-center py-16 bg-atelier-900 border border-dashed border-atelier-750 rounded-2xl space-y-4">
      <div class="w-12 h-12 rounded-xl bg-atelier-800 border border-atelier-750 flex items-center justify-center mx-auto text-ink-400">
        <Lightbulb class="w-6 h-6" />
      </div>
      <div class="space-y-1">
        <h3 class="text-sm font-semibold text-ink-100 font-serif">
          {{ ideas.length === 0 ? '灵感 Inbox 空空如也' : '未找到匹配的灵感' }}
        </h3>
        <p class="text-xs text-ink-400 max-w-sm mx-auto">
          {{ ideas.length === 0 ? '随时有灵光一现？在上方输入标题与一句话梗概，即可永久沉淀在本地，随时一键转化长篇大纲。' : '换个关键词试试，或切换筛选状态。' }}
        </p>
      </div>

      <!-- 快速灵感点子启发 -->
      <div v-if="ideas.length === 0" class="pt-3">
        <div class="text-[11px] font-mono text-ink-400 mb-2">快速填入示范灵感：</div>
        <div class="flex flex-wrap items-center justify-center gap-2 max-w-lg mx-auto">
          <button 
            v-for="sample in samplePresets" 
            :key="sample.title"
            @click="applySample(sample)"
            class="px-2.5 py-1 text-[11px] bg-atelier-850 hover:bg-atelier-800 hover:border-brand-amber/50 rounded-md border border-atelier-750 text-ink-300 hover:text-brand-amber transition cursor-pointer">
            + {{ sample.title }} ({{ sample.platform }})
          </button>
        </div>
      </div>
    </div>

    <!-- 灵感卡片网格列表 -->
    <div v-else class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
      <div 
        v-for="idea in filteredIdeas" 
        :key="idea.id" 
        class="bg-atelier-900 border rounded-xl p-4 flex flex-col justify-between space-y-3 shadow-atelier-sm hover:border-brand-amber/40 transition group"
        :class="idea.status === 'GENESIS' ? 'border-emerald-500/30 bg-atelier-900/90' : 'border-atelier-750'">
        
        <!-- 卡片头部信息 -->
        <div class="space-y-2">
          <div class="flex items-center justify-between gap-2">
            <span 
              :class="idea.status === 'GENESIS' 
                ? 'bg-emerald-500/15 text-emerald-400 border-emerald-500/30' 
                : 'bg-brand-amber/15 text-brand-amber border-brand-amber/30'"
              class="text-[10px] font-mono px-2 py-0.5 rounded border flex items-center gap-1 shrink-0">
              <span v-if="idea.status === 'GENESIS'">🌌 已转 GENESIS</span>
              <span v-else>💡 灵感待转</span>
            </span>

            <span class="text-[10px] font-mono text-ink-400 bg-atelier-950 px-2 py-0.5 rounded border border-atelier-800">
              {{ idea.platform }}
            </span>
          </div>

          <h3 class="text-sm font-serif font-bold text-ink-50 group-hover:text-brand-amber transition-colors line-clamp-1">
            {{ idea.title }}
          </h3>

          <p class="text-xs text-ink-300 leading-relaxed line-clamp-3 bg-atelier-950/60 p-2.5 rounded-lg border border-atelier-800/80 font-serif">
            {{ idea.pitch || '（未填写一句话梗概）' }}
          </p>
        </div>

        <!-- 卡片底部操作与时间 -->
        <div class="pt-2 border-t border-atelier-800 flex items-center justify-between gap-2">
          <span class="text-[10px] font-mono text-ink-500">
            {{ formatTime(idea.created_at) }}
          </span>

          <div class="flex items-center gap-1.5">
            <!-- 转 Genesis 创世按钮 (复用现有 NewProjectModal 调用方式) -->
            <button 
              @click="convertToGenesis(idea)" 
              class="px-2.5 py-1 text-xs font-semibold rounded-md transition flex items-center gap-1 cursor-pointer"
              :class="idea.status === 'GENESIS' 
                ? 'bg-atelier-800 hover:bg-atelier-750 text-ink-300 border border-atelier-700' 
                : 'bg-brand-amber hover:bg-brand-amber-hover text-atelier-950 shadow-amber-glow font-bold'"
              :title="idea.status === 'GENESIS' ? '再次打开创世工程弹窗' : '将灵感转为长篇创世工程'">
              <Sparkles class="w-3 h-3" />
              <span>{{ idea.status === 'GENESIS' ? '重开创世' : '转 Genesis' }}</span>
            </button>

            <!-- 删除灵感 -->
            <button 
              @click="deleteIdea(idea.id)" 
              class="p-1 text-ink-500 hover:text-rose-400 hover:bg-rose-500/10 rounded transition cursor-pointer"
              title="删除此灵感">
              <Trash2 class="w-3.5 h-3.5" />
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue';
import { state, actions, notify } from '../stores/appState';
import { 
  Lightbulb, 
  Plus, 
  Sparkles, 
  Search, 
  Trash2 
} from 'lucide-vue-next';

const STORAGE_KEY = 'novel-studio-ideas';

const ideas = ref([]);
const currentFilter = ref('ALL');
const searchQuery = ref('');

const newIdeaForm = reactive({
  title: '',
  pitch: '',
  platform: '番茄脑洞',
});

const samplePresets = [
  {
    title: '神经元飞升：重构赛博天庭',
    platform: '番茄脑洞',
    pitch: '高维算法入侵修真界，灵根被改造成生化芯片，金丹成为微型聚变堆，天道为算力集群。',
  },
  {
    title: '深渊回响：第十九个嫌疑人',
    platform: '知乎盐言',
    pitch: '十五年前警队绝密悬案，主角以法医顾问身份重返故地，在层层迷雾中展开绝地复仇。',
  },
  {
    title: '凡骨弑神：三千年宗门血祭',
    platform: '起点仙侠',
    pitch: '凡人无灵根，天道被不可名状的神明寄生；修士飞升实为祭品，主角以凡骨铁印逆炼仙道。',
  },
];

const pendingIdeasCount = computed(() => {
  return ideas.value.filter(i => i.status !== 'GENESIS').length;
});

const genesisIdeasCount = computed(() => {
  return ideas.value.filter(i => i.status === 'GENESIS').length;
});

const filterTabs = computed(() => [
  { id: 'ALL', label: '全部灵感', count: ideas.value.length },
  { id: 'IDEA', label: '💡 雏形灵感', count: pendingIdeasCount.value },
  { id: 'GENESIS', label: '🌌 已转创世', count: genesisIdeasCount.value },
]);

const filteredIdeas = computed(() => {
  let list = ideas.value;

  if (currentFilter.value === 'IDEA') {
    list = list.filter(i => i.status !== 'GENESIS');
  } else if (currentFilter.value === 'GENESIS') {
    list = list.filter(i => i.status === 'GENESIS');
  }

  const query = searchQuery.value.trim().toLowerCase();
  if (query) {
    list = list.filter(i => {
      const matchTitle = (i.title || '').toLowerCase().includes(query);
      const matchPitch = (i.pitch || '').toLowerCase().includes(query);
      return matchTitle || matchPitch;
    });
  }

  return list;
});

function loadStoredIdeas() {
  if (typeof window === 'undefined') return;
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (raw) {
      const parsed = JSON.parse(raw);
      if (Array.isArray(parsed)) {
        ideas.value = parsed;
        return;
      }
    }
  } catch (e) {
    console.warn('Failed to parse novel-studio-ideas from localStorage:', e);
  }
  ideas.value = [];
}

function persistIdeas() {
  if (typeof window === 'undefined') return;
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(ideas.value));
  } catch (e) {
    console.warn('Failed to persist novel-studio-ideas to localStorage:', e);
  }
}

function addIdea() {
  const title = newIdeaForm.title.trim();
  if (!title) return;

  const item = {
    id: `idea_${Date.now()}_${Math.random().toString(36).substring(2, 6)}`,
    title,
    pitch: newIdeaForm.pitch.trim(),
    platform: newIdeaForm.platform,
    status: 'IDEA', // 初始状态为 IDEA
    created_at: new Date().toISOString(),
  };

  ideas.value.unshift(item);
  persistIdeas();

  // 清空录入表单
  newIdeaForm.title = '';
  newIdeaForm.pitch = '';

  notify('灵感已收录', `${title} 已安全沉淀在本地 Inbox`, 'success');
}

function applySample(sample) {
  newIdeaForm.title = sample.title;
  newIdeaForm.platform = sample.platform;
  newIdeaForm.pitch = sample.pitch;
}

function convertToGenesis(idea) {
  // 1. 将状态更新为 GENESIS 并本地持久化 (IDEA -> GENESIS)
  idea.status = 'GENESIS';
  persistIdeas();

  // 2. 复制灵感核心信息到剪贴板，方便在创世弹窗中快速粘贴
  if (navigator?.clipboard?.writeText) {
    navigator.clipboard.writeText(`${idea.title}\n${idea.pitch}`).catch(() => {});
  }

  // 3. 预填灵感数据并唤起创世弹窗
  state.newProjectPrefill = {
    title: idea.title,
    concept: idea.pitch || '',
    platform: idea.platform || '',
  };
  state.showNewProjectModal = true;

  notify(
    '灵感已转入创世工序',
    `《${idea.title}》状态已更新为 GENESIS，创世弹窗已唤起`,
    'success'
  );
}

function deleteIdea(id) {
  ideas.value = ideas.value.filter(i => i.id !== id);
  persistIdeas();
  notify('灵感已移除', '', 'info', 2000);
}

function formatTime(isoString) {
  if (!isoString) return '';
  try {
    const d = new Date(isoString);
    if (isNaN(d.getTime())) return '';
    const month = String(d.getMonth() + 1).padStart(2, '0');
    const day = String(d.getDate()).padStart(2, '0');
    const hours = String(d.getHours()).padStart(2, '0');
    const minutes = String(d.getMinutes()).padStart(2, '0');
    return `${month}-${day} ${hours}:${minutes}`;
  } catch (_) {
    return '';
  }
}

onMounted(() => {
  loadStoredIdeas();
});
</script>
