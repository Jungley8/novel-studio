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

      <div class="flex items-center gap-2.5">
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
    <div class="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 gap-3">
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

      <!-- TODO: 等待 GET /api/studio/analytics 后端 Studio 级聚合接口提供 Token 花费数据 -->
      <div class="p-3.5 bg-atelier-900 border border-atelier-750 rounded-xl shadow-atelier-sm" title="等待 GET /api/studio/analytics 聚合接口支持">
        <div class="text-[11px] font-mono text-ink-400 flex items-center justify-between">
          <span>Token 花费</span>
          <span class="text-[9px] text-ink-500 font-normal">待后端</span>
        </div>
        <div class="text-lg font-serif text-ink-400 mt-1">—</div>
      </div>

      <!-- TODO: 等待 GET /api/studio/analytics 后端 Studio 级聚合接口提供评审通过率数据 -->
      <div class="p-3.5 bg-atelier-900 border border-atelier-750 rounded-xl shadow-atelier-sm" title="等待 GET /api/studio/analytics 聚合接口支持">
        <div class="text-[11px] font-mono text-ink-400 flex items-center justify-between">
          <span>评审通过率</span>
          <span class="text-[9px] text-ink-500 font-normal">待后端</span>
        </div>
        <div class="text-lg font-serif text-ink-400 mt-1">—</div>
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
              <!-- TODO: 等待 GET /api/studio/analytics 后端 Studio 级聚合接口提供 Token 花费数据 -->
              <th class="py-3 px-4 font-semibold text-center" title="等待 GET /api/studio/analytics">Token 花费</th>
              <!-- TODO: 等待 GET /api/studio/analytics 后端 Studio 级聚合接口提供全书评审通过率 -->
              <th class="py-3 px-4 font-semibold text-center" title="等待 GET /api/studio/analytics">评审通过率</th>
              <th class="py-3 px-4 font-semibold">最近活动</th>
              <th class="py-3 px-4 font-semibold text-right">操作</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-atelier-750/60">
            <tr 
              v-for="item in libraryItems" 
              :key="item.project.id"
              class="hover:bg-atelier-850/60 transition group cursor-pointer"
              @click="openProject(item.project.id)">
              
              <!-- 书名与平台定位 -->
              <td class="py-3.5 px-4">
                <div class="flex items-center gap-2">
                  <span class="font-serif font-bold text-ink-50 group-hover:text-brand-amber transition-colors">
                    {{ item.project.title }}
                  </span>
                  <span class="text-[10px] font-mono px-1.5 py-0.5 rounded bg-atelier-800 text-ink-300 border border-atelier-700/80 shrink-0">
                    {{ item.project.target_platform || '通用网文' }}
                  </span>
                  <span 
                    v-if="state.selectedProjectId === item.project.id" 
                    class="text-[9px] font-mono px-1.5 py-0.2 rounded bg-brand-amber/15 text-brand-amber border border-brand-amber/30">
                    当前激活
                  </span>
                </div>
              </td>

              <!-- 当前章 -->
              <td class="py-3.5 px-4 font-mono text-ink-200">
                <span v-if="item.chaptersCount > 0" class="text-brand-amber font-semibold">
                  第 {{ item.chaptersCount }} 章
                </span>
                <span v-else class="text-ink-500">
                  尚未完稿 (草拟中)
                </span>
              </td>

              <!-- OPEN 钩数 -->
              <td class="py-3.5 px-4 text-center font-mono">
                <span 
                  :class="item.openHooksCount > 0 ? 'bg-atelier-800 text-ink-200' : 'text-ink-500'"
                  class="px-2 py-0.5 rounded text-[11px]">
                  {{ item.openHooksCount }}
                </span>
              </td>

              <!-- 超期钩数 (钩债务) -->
              <td class="py-3.5 px-4 text-center font-mono">
                <span 
                  v-if="item.overdueHooksCount > 0" 
                  class="px-2 py-0.5 rounded text-[11px] font-bold bg-rose-500/15 text-rose-400 border border-rose-500/30">
                  {{ item.overdueHooksCount }}
                </span>
                <span v-else class="text-emerald-400/80 text-[11px]">
                  0
                </span>
              </td>

              <!-- Token 花费 (未接入聚合接口，按规范留 TODO 并显示 —) -->
              <!-- TODO: 等待 GET /api/studio/analytics 后端 Studio 级聚合接口提供 Token 花费数据 -->
              <td class="py-3.5 px-4 text-center font-mono text-ink-500">
                —
              </td>

              <!-- 评审通过率 (未接入聚合接口，按规范留 TODO 并显示 —) -->
              <!-- TODO: 等待 GET /api/studio/analytics 后端 Studio 级聚合接口提供全书评审通过率 -->
              <td class="py-3.5 px-4 text-center font-mono text-ink-500">
                —
              </td>

              <!-- 最近活动时间 -->
              <td class="py-3.5 px-4 font-mono text-[11px] text-ink-400">
                {{ item.recentActivityText }}
              </td>

              <!-- 操作 -->
              <td class="py-3.5 px-4 text-right" @click.stop>
                <button 
                  @click="openProject(item.project.id)"
                  class="px-2.5 py-1 bg-atelier-800 hover:bg-brand-amber hover:text-atelier-950 text-ink-300 font-medium text-xs rounded border border-atelier-700 transition cursor-pointer flex items-center gap-1 ml-auto">
                  <span>进入工作台</span>
                  <ArrowUpRight class="w-3 h-3" />
                </button>
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
  ArrowUpRight 
} from 'lucide-vue-next';

const isLoading = ref(false);
const errorMessage = ref('');
const rawItems = ref([]);

const libraryItems = computed(() => rawItems.value);

const totalProjects = computed(() => libraryItems.value.length);
const totalChapters = computed(() => {
  return libraryItems.value.reduce((acc, curr) => acc + (curr.chaptersCount || 0), 0);
});
const totalOpenHooks = computed(() => {
  return libraryItems.value.reduce((acc, curr) => acc + (curr.openHooksCount || 0), 0);
});
const totalOverdueHooks = computed(() => {
  return libraryItems.value.reduce((acc, curr) => acc + (curr.overdueHooksCount || 0), 0);
});

function formatActivityDate(dateStr) {
  if (!dateStr) return '—';
  try {
    const d = new Date(dateStr);
    if (isNaN(d.getTime())) return '—';
    const now = new Date();
    const diffMs = now.getTime() - d.getTime();
    const diffMins = Math.floor(diffMs / (1000 * 60));
    const diffHours = Math.floor(diffMins / 60);
    const diffDays = Math.floor(diffHours / 24);

    if (diffMins < 5) return '刚刚';
    if (diffMins < 60) return `${diffMins} 分钟前`;
    if (diffHours < 24) return `${diffHours} 小时前`;
    if (diffDays < 7) return `${diffDays} 天前`;

    const year = d.getFullYear();
    const month = String(d.getMonth() + 1).padStart(2, '0');
    const day = String(d.getDate()).padStart(2, '0');
    return `${year}-${month}-${day}`;
  } catch (_) {
    return '—';
  }
}

async function fetchLibraryData() {
  isLoading.value = true;
  errorMessage.value = '';

  try {
    // 1. 获取全库项目列表 (GET /api/projects)
    const projects = await api.listProjects();
    state.projects = projects;

    if (!Array.isArray(projects) || projects.length === 0) {
      rawItems.value = [];
      return;
    }

    // 2. 并行拉取每个项目的真实关联数据 (hooks / chapters / analytics 如有)
    const items = await Promise.all(
      projects.map(async (project) => {
        let chapters = [];
        let hooks = [];

        try {
          chapters = await api.listChapters(project.id);
        } catch (e) {
          console.warn(`Failed to list chapters for ${project.id}:`, e);
        }

        try {
          hooks = await api.listHooks(project.id);
        } catch (e) {
          console.warn(`Failed to list hooks for ${project.id}:`, e);
        }

        // 计算当前章与伏笔指标
        const chaptersCount = Array.isArray(chapters) ? chapters.length : 0;
        const openHooks = Array.isArray(hooks) ? hooks.filter(h => h.status === 'OPEN') : [];
        const openHooksCount = openHooks.length;

        // 超期钩数定义：OPEN 且目标揭晓章已落后于当前已写章节数
        const overdueHooksCount = openHooks.filter(h => {
          const target = Number(h.target_chapter) || 0;
          return target > 0 && target <= chaptersCount;
        }).length;

        // 计算最近活动时间：取 project.updated_at、project.created_at 或最新章节 created_at
        let latestTimestamp = project.updated_at || project.created_at || '';
        if (Array.isArray(chapters) && chapters.length > 0) {
          const latestChapter = chapters[chapters.length - 1];
          if (latestChapter.created_at && (!latestTimestamp || new Date(latestChapter.created_at) > new Date(latestTimestamp))) {
            latestTimestamp = latestChapter.created_at;
          }
        }

        return {
          project,
          chaptersCount,
          openHooksCount,
          overdueHooksCount,
          recentActivityText: formatActivityDate(latestTimestamp),
          // TODO: 等待 GET /api/studio/analytics 提供 Token 花费与评审通过率
        };
      })
    );

    rawItems.value = items;
  } catch (err) {
    console.error('fetchLibraryData error:', err);
    errorMessage.value = `获取多作品总览失败: ${err.message || '网络连接超时或后端服务未就绪'}`;
  } finally {
    isLoading.value = false;
  }
}

async function openProject(projectId) {
  try {
    await actions.selectProject(projectId);
    state.activeTab = 'workbench';
  } catch (e) {
    console.error('Failed to open project:', e);
  }
}

onMounted(() => {
  fetchLibraryData();
});
</script>
