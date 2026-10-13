<template>
  <div class="flex-1 p-6 overflow-y-auto space-y-6">
    <!-- 头部工具栏 -->
    <div class="flex flex-col md:flex-row md:items-center justify-between gap-4 border-b border-atelier-750 pb-5">
      <div>
        <div class="flex items-center gap-2.5">
          <div class="w-8 h-8 rounded-lg bg-brand-amber/10 border border-brand-amber/25 flex items-center justify-center text-brand-amber shadow-amber-glow">
            <Library class="w-4 h-4" />
          </div>
          <div>
            <h2 class="text-base font-serif font-bold text-ink-50 tracking-wide">Studio 多作品总览</h2>
            <p class="text-xs text-ink-400 mt-0.5">全库小说创作进度、伏笔债务与多作品健康度全景。</p>
          </div>
        </div>
      </div>

      <div class="flex flex-wrap items-center gap-2.5">
        <!-- 搜索与筛选 -->
        <div class="relative">
          <Search class="w-3.5 h-3.5 text-ink-400 absolute left-2.5 top-1/2 -translate-y-1/2 pointer-events-none" />
          <input 
            v-model="searchQuery" 
            placeholder="搜索书名..." 
            class="pl-8 pr-2.5 py-1.5 bg-atelier-850 text-ink-100 placeholder-ink-500 text-xs rounded-md border border-atelier-750 focus:border-brand-amber/60 focus:outline-none transition w-32 sm:w-40" />
        </div>

        <select 
          v-if="availablePlatforms.length > 0"
          v-model="selectedPlatform" 
          class="px-2.5 py-1.5 bg-atelier-850 text-ink-300 text-xs rounded-md border border-atelier-750 focus:border-brand-amber/60 focus:outline-none transition cursor-pointer">
          <option value="ALL">全部题材/平台</option>
          <option v-for="plat in availablePlatforms" :key="plat" :value="plat">{{ plat }}</option>
        </select>

        <button 
          @click="fetchLibraryData" 
          :disabled="isLoading"
          class="flex items-center gap-1.5 px-3 py-1.5 bg-atelier-850 hover:bg-atelier-800 text-ink-300 hover:text-ink-100 text-xs rounded-md border border-atelier-750 transition cursor-pointer disabled:opacity-50"
          title="刷新全库作品数据">
          <RefreshCw class="w-3.5 h-3.5 text-ink-400" :class="{ 'animate-spin': isLoading }" />
          <span>{{ isLoading ? '刷新中...' : '刷新总览' }}</span>
        </button>

        <button 
          @click="state.showNewProjectModal = true" 
          class="flex items-center gap-1.5 px-3.5 py-1.5 bg-brand-amber hover:bg-brand-amber-hover text-atelier-950 font-bold text-xs rounded-md shadow-amber-glow transition cursor-pointer">
          <Plus class="w-3.5 h-3.5" />
          <span>新建作品</span>
        </button>
      </div>
    </div>

    <!-- 异常/断网错误提示栏 (防白屏) -->
    <div 
      v-if="errorMessage" 
      class="p-4 bg-rose-500/10 border border-rose-500/30 rounded-xl flex items-center justify-between gap-3 text-xs text-rose-300">
      <div class="flex items-center gap-2">
        <AlertTriangle class="w-4 h-4 text-rose-400 shrink-0" />
        <span>{{ errorMessage }}</span>
      </div>
      <button 
        @click="fetchLibraryData" 
        class="px-2.5 py-1 bg-rose-500/20 hover:bg-rose-500/30 text-rose-200 rounded font-medium transition cursor-pointer">
        重试连接
      </button>
    </div>

    <!-- 顶层指标汇总卡片 -->
    <div class="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-7 gap-3">
      <div class="p-3.5 bg-atelier-900 border border-atelier-750 rounded-xl shadow-atelier-sm">
        <div class="text-[11px] font-mono text-ink-400">作品总数</div>
        <div class="text-lg font-serif font-bold text-ink-50 mt-1">{{ totalProjects }} 部</div>
      </div>

      <div class="p-3.5 bg-atelier-900 border border-atelier-750 rounded-xl shadow-atelier-sm">
        <div class="text-[11px] font-mono text-ink-400">已完稿总章数</div>
        <div class="text-lg font-serif font-bold text-brand-amber mt-1">{{ totalChapters }} 章</div>
      </div>

      <div class="p-3.5 bg-atelier-900 border border-atelier-750 rounded-xl shadow-atelier-sm">
        <div class="text-[11px] font-mono text-ink-400">活跃 OPEN 钩</div>
        <div class="text-lg font-serif font-bold text-ink-100 mt-1">{{ totalOpenHooks }} 条</div>
      </div>

      <div class="p-3.5 bg-atelier-900 border border-atelier-750 rounded-xl shadow-atelier-sm">
        <div class="text-[11px] font-mono text-ink-400">逾期钩债务</div>
        <div 
          :class="totalOverdueHooks > 0 ? 'text-rose-400 font-bold' : 'text-emerald-400'"
          class="text-lg font-serif mt-1">
          {{ totalOverdueHooks }} 条
        </div>
      </div>

      <div class="p-3.5 bg-atelier-900 border border-atelier-750 rounded-xl shadow-atelier-sm">
        <div class="text-[11px] font-mono text-ink-400 flex items-center justify-between">
          <span>Token 花费</span>
          <span class="text-[9px] text-ink-400 font-normal">全库累计</span>
        </div>
        <div class="text-lg font-serif font-bold text-ink-50 mt-1">{{ totalTokens.toLocaleString() }}</div>
      </div>

      <div class="p-3.5 bg-atelier-900 border border-atelier-750 rounded-xl shadow-atelier-sm">
        <div class="text-[11px] font-mono text-ink-400 flex items-center justify-between">
          <span>预估费用</span>
          <span class="text-[9px] text-ink-400 font-normal">全库累计</span>
        </div>
        <div class="text-lg font-serif font-bold text-ink-50 mt-1">${{ totalCostUSD.toFixed(4) }}</div>
      </div>

      <div class="p-3.5 bg-atelier-900 border border-atelier-750 rounded-xl shadow-atelier-sm">
        <div class="text-[11px] font-mono text-ink-400 flex items-center justify-between">
          <span>评审通过率</span>
          <span class="text-[9px] text-ink-400 font-normal">全库平均</span>
        </div>
        <div class="text-lg font-serif font-bold text-ink-50 mt-1">{{ avgReviewPassRate }}</div>
      </div>
    </div>

    <!-- 加载中状态 -->
    <div v-if="isLoading && libraryItems.length === 0" class="py-20 text-center space-y-3">
      <div class="w-7 h-7 border-2 border-brand-amber border-t-transparent rounded-full animate-spin mx-auto"></div>
      <p class="text-xs text-ink-400 font-mono">正在聚合全书库作品指标与伏笔状态...</p>
    </div>

    <!-- 空态：无作品工程 -->
    <div 
      v-else-if="!isLoading && libraryItems.length === 0" 
      class="text-center py-20 bg-atelier-900 border border-dashed border-atelier-750 rounded-2xl space-y-4">
      <div class="w-14 h-14 rounded-2xl bg-atelier-800 border border-atelier-750 flex items-center justify-center mx-auto text-ink-400 shadow-atelier-sm">
        <Library class="w-7 h-7" />
      </div>
      <div class="space-y-1">
        <h3 class="text-sm font-semibold text-ink-100 font-serif">作品库暂无小说工程</h3>
        <p class="text-xs text-ink-400 max-w-sm mx-auto">
          尚未创建任何长篇小说工程。你可以新建第一部作品，或前往灵感 Inbox 将零散想法转化为创世工程。
        </p>
      </div>
      <div class="flex items-center justify-center gap-3 pt-2">
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

    <!-- 作品数据列表 -->
    <div v-else class="bg-atelier-900 border border-atelier-750 rounded-xl overflow-hidden shadow-atelier-md">
      <div class="px-5 py-3 border-b border-atelier-750 bg-atelier-950/40 flex items-center justify-between">
        <span class="text-xs font-semibold text-ink-100 font-serif">全库作品健康度与进度一览</span>
        <span class="text-[11px] font-mono text-ink-400">共 {{ libraryItems.length }} 部作品</span>
      </div>

      <div class="overflow-x-auto">
        <table class="w-full text-left text-xs border-collapse">
          <thead>
            <tr class="border-b border-atelier-750 bg-atelier-950/20 text-ink-400 font-mono text-[11px]">
              <th class="py-3 px-4 font-semibold">书名 / 题材</th>
              <th class="py-3 px-4 font-semibold">当前章 / 进度</th>
              <th class="py-3 px-4 font-semibold text-center">OPEN 钩数</th>
              <th class="py-3 px-4 font-semibold text-center">超期钩数 (钩债务)</th>
              <th class="py-3 px-4 font-semibold text-center">Token / 预估费用</th>
              <th class="py-3 px-4 font-semibold text-center">评审通过率</th>
              <th class="py-3 px-4 font-semibold text-right">操作</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-atelier-750/60">
            <tr 
              v-for="item in libraryItems" 
              :key="item.project_id"
              class="hover:bg-atelier-850/60 transition group cursor-pointer"
              @click="openProject(item.project_id)">
              
              <!-- 书名与平台定位 -->
              <td class="py-3.5 px-4">
                <div class="flex items-center gap-2">
                  <span class="font-serif font-bold text-ink-50 group-hover:text-brand-amber transition-colors">
                    {{ item.title }}
                  </span>
                  <span class="text-[10px] font-mono px-1.5 py-0.5 rounded bg-atelier-800 text-ink-300 border border-atelier-700/80 shrink-0">
                    {{ item.platform || '通用网文' }}
                  </span>
                  <span 
                    v-if="state.selectedProjectId === item.project_id" 
                    class="text-[9px] font-mono px-1.5 py-0.2 rounded bg-brand-amber/15 text-brand-amber border border-brand-amber/30">
                    当前激活
                  </span>
                </div>
              </td>

              <!-- 当前章 -->
              <td class="py-3.5 px-4 font-mono text-ink-200">
                <span v-if="item.archived_chapters > 0" class="text-brand-amber font-semibold">
                  第 {{ item.archived_chapters }} 章
                </span>
                <span v-else class="text-ink-500">
                  尚未完稿 (草拟中)
                </span>
              </td>

              <!-- OPEN 钩数 -->
              <td class="py-3.5 px-4 text-center font-mono">
                <span 
                  :class="item.open_hooks > 0 ? 'bg-atelier-800 text-ink-200' : 'text-ink-500'"
                  class="px-2 py-0.5 rounded text-[11px]">
                  {{ item.open_hooks }}
                </span>
              </td>

              <!-- 超期钩数 (钩债务) -->
              <td class="py-3.5 px-4 text-center font-mono">
                <span 
                  v-if="item.overdue_hooks > 0" 
                  class="px-2 py-0.5 rounded text-[11px] font-bold bg-rose-500/15 text-rose-400 border border-rose-500/30">
                  {{ item.overdue_hooks }}
                </span>
                <span v-else class="text-emerald-400/80 text-[11px]">
                  0
                </span>
              </td>

              <!-- Token 与预估费用 (双行紧凑显示，保持表格不溢出) -->
              <td class="py-3.5 px-4 text-center font-mono">
                <div class="text-ink-200">
                  {{ item.token_usage > 0 ? item.token_usage.toLocaleString() : '0' }}
                </div>
                <div class="text-[11px] text-ink-400 mt-0.5">
                  ${{ (item.estimated_cost_usd || 0).toFixed(4) }}
                </div>
              </td>

              <!-- 评审通过率 -->
              <td class="py-3.5 px-4 text-center font-mono text-ink-200">
                {{ item.archived_chapters > 0 ? `${Math.round((item.review_pass_rate || 0) * 100)}%` : '—' }}
              </td>

              <!-- 操作 -->
              <td class="py-3.5 px-4 text-right" @click.stop>
                <div class="flex items-center justify-end gap-1.5">
                  <button 
                    @click="openProject(item.project_id)"
                    class="px-2.5 py-1 bg-atelier-800 hover:bg-brand-amber hover:text-atelier-950 text-ink-300 font-medium text-xs rounded border border-atelier-700 transition cursor-pointer flex items-center gap-1">
                    <span>进入工作台</span>
                    <ArrowUpRight class="w-3 h-3" />
                  </button>
                  <button 
                    @click.stop="handleDeleteProject(item)"
                    class="p-1.5 bg-atelier-800 hover:bg-rose-500/15 text-ink-400 hover:text-rose-400 rounded border border-atelier-700 hover:border-rose-500/30 transition cursor-pointer"
                    title="彻底删除此作品">
                    <Trash2 class="w-3.5 h-3.5" />
                  </button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue';
import { state, actions } from '../stores/appState';
import { api } from '../api/client';
import { 
  Library, 
  RefreshCw, 
  Plus, 
  AlertTriangle, 
  ArrowUpRight,
  Trash2,
  Search
} from 'lucide-vue-next';

const isLoading = ref(false);
const errorMessage = ref('');
const rawItems = ref([]);
const searchQuery = ref('');
const selectedPlatform = ref('ALL');

const availablePlatforms = computed(() => {
  const set = new Set();
  (rawItems.value || []).forEach(item => {
    if (item.platform) set.add(item.platform);
  });
  return Array.from(set);
});

const libraryItems = computed(() => {
  let list = rawItems.value || [];
  if (selectedPlatform.value !== 'ALL') {
    list = list.filter(item => item.platform === selectedPlatform.value);
  }
  if (searchQuery.value.trim()) {
    const q = searchQuery.value.trim().toLowerCase();
    list = list.filter(item => (item.title || '').toLowerCase().includes(q));
  }
  return list;
});

const totalProjects = computed(() => rawItems.value.length);
const totalChapters = computed(() => {
  return rawItems.value.reduce((acc, curr) => acc + (curr.archived_chapters || 0), 0);
});
const totalOpenHooks = computed(() => {
  return rawItems.value.reduce((acc, curr) => acc + (curr.open_hooks || 0), 0);
});
const totalOverdueHooks = computed(() => {
  return rawItems.value.reduce((acc, curr) => acc + (curr.overdue_hooks || 0), 0);
});
const totalTokens = computed(() => {
  return rawItems.value.reduce((acc, curr) => acc + (curr.token_usage || 0), 0);
});
const totalCostUSD = computed(() => {
  return rawItems.value.reduce((acc, curr) => acc + (curr.estimated_cost_usd || 0), 0);
});
const avgReviewPassRate = computed(() => {
  const withChapters = rawItems.value.filter(item => (item.archived_chapters || 0) > 0);
  if (withChapters.length === 0) return '—';
  const sum = withChapters.reduce((acc, curr) => acc + (curr.review_pass_rate || 0), 0);
  return `${Math.round((sum / withChapters.length) * 100)}%`;
});

async function fetchLibraryData() {
  isLoading.value = true;
  errorMessage.value = '';

  try {
    const items = await api.listStudioLibrary();
    if (!Array.isArray(items)) {
      throw new Error('返回数据格式异常：预期为作品列表数组');
    }
    rawItems.value = items;
  } catch (err) {
    console.error('fetchLibraryData error:', err);
    rawItems.value = [];
    errorMessage.value = `获取多作品总览失败: ${err.message || '网络连接超时或后端服务未就绪'}`;
  } finally {
    isLoading.value = false;
  }
}

async function handleDeleteProject(item) {
  const ok = await actions.deleteProject(item.project_id);
  if (ok) {
    await fetchLibraryData();
  }
}

async function openProject(projectId) {
  try {
    await actions.switchToProject(projectId, 'workbench');
  } catch (e) {
    console.error('Failed to open project:', e);
  }
}

onMounted(() => {
  fetchLibraryData();
});
</script>
