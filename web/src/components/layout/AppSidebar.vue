<template>
  <aside class="w-64 bg-atelier-900 border-r border-atelier-750 flex flex-col justify-between shrink-0 select-none">
    <div class="flex flex-col min-h-0">
      <!-- 品牌标识 -->
      <div class="h-14 px-4 border-b border-atelier-750 flex items-center justify-between">
        <div class="flex items-center gap-2.5">
          <div class="w-7 h-7 rounded-md bg-brand-amber/15 border border-brand-amber/30 flex items-center justify-center text-brand-amber font-serif font-bold text-sm shadow-amber-glow">
            墨
          </div>
          <div>
            <span class="text-sm font-semibold tracking-wide text-ink-50">NovelStudio</span>
            <span class="block text-[9px] font-mono tracking-wider text-ink-400 uppercase -mt-0.5">Atelier 2.0</span>
          </div>
        </div>
        <span class="text-[10px] font-mono font-medium px-1.5 py-0.5 rounded bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
          GO·SQLITE
        </span>
      </div>

      <!-- 项目选择器 -->
      <div class="p-3 border-b border-atelier-750 bg-atelier-950/40">
        <div class="flex items-center justify-between mb-1.5">
          <span class="text-[10px] font-mono uppercase tracking-wider text-ink-400">当前书卷 (Work)</span>
          <button 
            @click="state.showNewProjectModal = true"
            class="text-[11px] text-brand-amber hover:text-brand-amber-hover flex items-center gap-0.5 font-medium transition cursor-pointer">
            <Plus class="w-3 h-3" />
            <span>新建</span>
          </button>
        </div>
        <div class="relative">
          <select 
            :value="state.selectedProjectId" 
            @change="actions.selectProject($event.target.value)"
            class="w-full bg-atelier-850 border border-atelier-700 rounded-md px-2.5 py-1.5 text-xs text-ink-100 font-medium appearance-none focus:outline-none focus:border-brand-amber/60 focus:ring-1 focus:ring-brand-amber/40 transition truncate pr-6 cursor-pointer">
            <option v-for="p in state.projects" :key="p.id" :value="p.id">{{ p.title }}</option>
          </select>
          <ChevronDown class="w-3.5 h-3.5 text-ink-400 absolute right-2 top-2.5 pointer-events-none" />
        </div>
      </div>

      <!-- 主导航列表 -->
      <nav class="p-2 space-y-0.5 overflow-y-auto">
        <button 
          v-for="item in navItems" 
          :key="item.id"
          @click="selectTab(item.id)"
          :class="[
            'w-full flex items-center justify-between px-3 py-2 rounded-md text-xs font-medium transition group cursor-pointer text-left',
            state.activeTab === item.id 
              ? 'bg-atelier-800 text-brand-amber shadow-atelier-sm font-semibold' 
              : 'text-ink-300 hover:text-ink-100 hover:bg-atelier-850'
          ]">
          <div class="flex items-center gap-2.5 min-w-0">
            <component 
              :is="item.icon" 
              :class="[
                'w-4 h-4 shrink-0 transition-colors',
                state.activeTab === item.id ? 'text-brand-amber' : 'text-ink-400 group-hover:text-ink-200'
              ]" 
            />
            <span class="truncate">{{ item.label }}</span>
          </div>
          <span 
            v-if="item.badge" 
            class="text-[10px] font-mono px-1.5 py-0.2 rounded bg-atelier-750 text-ink-300">
            {{ item.badge }}
          </span>
        </button>
      </nav>
    </div>

    <!-- 底部导出与状态 -->
    <div class="p-3 border-t border-atelier-750 bg-atelier-950/60 space-y-2">
      <div class="grid grid-cols-2 gap-1.5">
        <a 
          :href="state.currentProject ? '/api/projects/' + state.currentProject.id + '/export/json' : '#'" 
          download 
          class="flex items-center justify-center gap-1.5 py-1.5 px-2 text-[11px] font-medium bg-atelier-850 hover:bg-atelier-800 text-ink-300 hover:text-ink-100 rounded border border-atelier-700/80 transition text-center cursor-pointer">
          <Download class="w-3 h-3 text-ink-400" />
          <span>快照 JSON</span>
        </a>
        <a 
          :href="state.currentProject ? '/api/projects/' + state.currentProject.id + '/export/markdown' : '#'" 
          download 
          class="flex items-center justify-center gap-1.5 py-1.5 px-2 text-[11px] font-medium bg-brand-amber/10 hover:bg-brand-amber/20 text-brand-amber rounded border border-brand-amber/25 transition text-center cursor-pointer">
          <BookDown class="w-3 h-3 text-brand-amber" />
          <span>全本 MD</span>
        </a>
      </div>
      <div class="text-[10px] font-mono text-ink-400 text-center pt-1">
        单二进制零 CGO · 工业级长篇防护
      </div>
    </div>
  </aside>
</template>

<script setup>
import { computed } from 'vue';
import { state, actions } from '../../stores/appState';
import {
  PenTool,
  LayoutGrid,
  BookOpen,
  Compass,
  Cpu,
  Anchor,
  Archive,
  BarChart3,
  Settings,
  Plus,
  ChevronDown,
  Download,
  BookDown
} from 'lucide-vue-next';

const navItems = computed(() => [
  { id: 'workbench', label: '生产手稿 (Workbench)', icon: PenTool, badge: state.chapters.length ? `第${state.chapters.length + 1}章` : '' },
  { id: 'matrix', label: '矩阵大纲 (The Matrix)', icon: LayoutGrid, badge: state.matrixOverview?.total_scenes ? `${state.matrixOverview.total_scenes}场` : '' },
  { id: 'codex', label: '全域百科 (The Codex)', icon: BookOpen, badge: state.codexEntries.length ? `${state.codexEntries.length}条` : '' },
  { id: 'framework', label: '创世总纲 (Framework)', icon: Compass },
  { id: 'statemachine', label: '实体状态机 (State Machine)', icon: Cpu },
  { id: 'hooks', label: '伏笔因果 (Plot Ledger)', icon: Anchor, badge: state.hooks.filter(h => h.status !== 'RESOLVED').length ? `${state.hooks.filter(h => h.status !== 'RESOLVED').length}线` : '' },
  { id: 'chapters', label: '全本正史 (History)', icon: Archive, badge: `${state.chapters.length}章` },
  { id: 'analytics', label: '态势分析 (Analytics)', icon: BarChart3 },
  { id: 'config', label: '系统与模型 (Engine)', icon: Settings },
]);

function selectTab(tabId) {
  state.activeTab = tabId;
  if (tabId === 'matrix') actions.loadMatrixOverview();
  if (tabId === 'codex') actions.loadCodexEntries();
  if (tabId === 'analytics') actions.loadAnalytics();
}
</script>
