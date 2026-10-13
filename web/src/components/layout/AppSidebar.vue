<template>
  <aside class="w-64 bg-atelier-900 border-r border-atelier-750 flex flex-col justify-between shrink-0 select-none">
    <div class="flex flex-col min-h-0 flex-1">
      <!-- 品牌标识与 macOS 交通灯避让区 -->
      <div class="h-14 pl-20 pr-3 border-b border-atelier-750 flex items-center justify-between" style="-webkit-app-region: drag;">
        <div class="flex items-center gap-2" style="-webkit-app-region: no-drag;">
          <div class="w-6 h-6 rounded-md bg-brand-amber/15 border border-brand-amber/30 flex items-center justify-center text-brand-amber font-serif font-bold text-xs shadow-amber-glow">
            墨
          </div>
          <div>
            <span class="text-xs font-semibold tracking-wide text-ink-50">NovelStudio</span>
            <span class="block text-[8px] font-mono tracking-wider text-ink-400 uppercase -mt-0.5">
              {{ isGlobalMode ? 'Studio 全局中枢' : '作品创作工作区' }}
            </span>
          </div>
        </div>
        <span 
          :class="isGlobalMode ? 'bg-amber-500/10 text-brand-amber border-amber-500/25' : 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20'"
          class="text-[9px] font-mono font-medium px-1.5 py-0.5 rounded border" 
          style="-webkit-app-region: no-drag;">
          {{ isGlobalMode ? 'STUDIO' : 'PROJECT' }}
        </span>
      </div>

      <!-- ========================================== -->
      <!-- 模式 A：全局 Studio 侧边栏 (物理完全隔离)    -->
      <!-- ========================================== -->
      <div v-if="isGlobalMode" class="flex flex-col flex-1 min-h-0">
        <!-- 活跃作品快捷通道卡片 -->
        <div class="p-3 border-b border-atelier-750 bg-atelier-950/40 space-y-2">
          <div class="flex items-center justify-between">
            <span class="text-[10px] font-mono uppercase tracking-wider text-ink-400 flex items-center gap-1">
              <BookText class="w-3 h-3 text-brand-amber" />
              <span>活跃作品通道</span>
            </span>
            <button 
              @click="state.showNewProjectModal = true"
              class="text-[11px] text-brand-amber hover:text-brand-amber-hover flex items-center gap-0.5 font-medium transition cursor-pointer"
              title="新建一部小说作品">
              <Plus class="w-3 h-3" />
              <span>新建作品</span>
            </button>
          </div>

          <!-- 已有作品：展示当前聚焦作品与直达按钮 -->
          <div v-if="state.currentProject" class="space-y-1.5 bg-atelier-850/80 border border-atelier-750/80 rounded-lg p-2">
            <div class="flex items-center justify-between gap-1">
              <span class="font-serif font-bold text-xs text-ink-100 truncate">
                《{{ state.currentProject.title }}》
              </span>
              <span class="text-[9px] font-mono px-1.5 py-0.2 rounded bg-atelier-800 text-ink-300 border border-atelier-700/60 shrink-0">
                {{ state.currentProject.target_platform || '通用' }}
              </span>
            </div>

            <!-- 多部作品时的紧凑切换器 -->
            <div v-if="state.projects.length > 1" class="relative">
              <select 
                :value="state.selectedProjectId" 
                @change="handleProjectChange($event)"
                class="w-full bg-atelier-900 border border-atelier-700/60 rounded px-2 py-1 text-[11px] text-ink-300 font-mono appearance-none focus:outline-none focus:border-brand-amber/60 pr-5 cursor-pointer">
                <option v-for="p in state.projects" :key="p.id" :value="p.id">换书：{{ p.title }}</option>
              </select>
              <ChevronDown class="w-3 h-3 text-ink-400 absolute right-1.5 top-1.5 pointer-events-none" />
            </div>

            <button 
              @click="actions.switchToProject(state.selectedProjectId)"
              class="w-full py-1.5 px-2 bg-brand-amber/15 hover:bg-brand-amber/25 text-brand-amber border border-brand-amber/35 rounded text-xs font-semibold flex items-center justify-center gap-1 transition cursor-pointer group shadow-sm">
              <span>进入创作空间</span>
              <ArrowUpRight class="w-3.5 h-3.5 group-hover:translate-x-0.5 group-hover:-translate-y-0.5 transition-transform" />
            </button>
          </div>

          <!-- 无作品时的空态创建引导 -->
          <button 
            v-else
            @click="state.showNewProjectModal = true"
            class="w-full py-2 px-2.5 bg-brand-amber/10 hover:bg-brand-amber/20 border border-brand-amber/30 rounded-md text-xs text-brand-amber font-medium flex items-center justify-center gap-1.5 transition cursor-pointer">
            <Plus class="w-3.5 h-3.5" />
            <span>新建第一部作品</span>
          </button>
        </div>

        <!-- 全局专有导航菜单 (仅 Studio 与全局系统设置) -->
        <nav class="flex-1 p-2 space-y-3 overflow-y-auto min-h-0">
          <div v-for="group in globalNavGroups" :key="group.title" class="space-y-0.5">
            <div class="px-3 pt-1 pb-1 text-[10px] font-mono tracking-wider text-ink-400 uppercase">
              {{ group.title }}
            </div>
            <button 
              v-for="item in group.items" 
              :key="item.id"
              @click="actions.setActiveTab(item.id)"
              :title="item.desc"
              :class="[
                'w-full flex items-center justify-between px-3 py-1.5 rounded-md text-xs font-medium transition group cursor-pointer text-left',
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
                class="text-[10px] font-mono px-1.5 py-0.2 rounded bg-atelier-750 text-ink-300 shrink-0">
                {{ item.badge }}
              </span>
            </button>
          </div>
        </nav>
      </div>

      <!-- ========================================== -->
      <!-- 模式 B：作品级创作侧边栏 (物理完全隔离)    -->
      <!-- ========================================== -->
      <div v-else class="flex flex-col flex-1 min-h-0">
        <!-- 顶栏：醒目的返回 Studio 全局按钮 -->
        <div class="p-2 border-b border-atelier-750 bg-atelier-950/50">
          <button 
            @click="actions.switchToGlobal('studio-library')"
            class="w-full py-1.5 px-2.5 bg-atelier-850 hover:bg-atelier-800 border border-atelier-700/80 hover:border-brand-amber/40 text-ink-200 hover:text-brand-amber rounded-md text-xs font-medium flex items-center justify-between transition cursor-pointer group shadow-sm"
            title="退出单作品工作区，返回 Studio 全局总览">
            <div class="flex items-center gap-1.5">
              <ArrowLeft class="w-3.5 h-3.5 text-brand-amber group-hover:-translate-x-0.5 transition-transform" />
              <span>返回 Studio 全局</span>
            </div>
            <span class="text-[10px] font-mono text-ink-400 group-hover:text-ink-300">多作品总览</span>
          </button>
        </div>

        <!-- 作品选择器与管理条 -->
        <div class="p-3 border-b border-atelier-750 bg-atelier-950/30">
          <div class="flex items-center justify-between mb-1.5">
            <span class="text-[10px] font-mono uppercase tracking-wider text-ink-400">创作工程</span>
            <button 
              @click="state.showNewProjectModal = true"
              class="text-[11px] text-brand-amber hover:text-brand-amber-hover flex items-center gap-0.5 font-medium transition cursor-pointer"
              title="新建一部小说作品">
              <Plus class="w-3 h-3" />
              <span>新建作品</span>
            </button>
          </div>
          <div v-if="state.projects.length > 0" class="flex items-center gap-1.5">
            <div class="relative flex-1 min-w-0">
              <select 
                :value="state.selectedProjectId" 
                @change="handleProjectChange($event)"
                class="w-full bg-atelier-850 border border-atelier-700 rounded-md px-2.5 py-1.5 text-xs text-ink-100 font-medium appearance-none focus:outline-none focus:border-brand-amber/60 focus:ring-1 focus:ring-brand-amber/40 transition truncate pr-6 cursor-pointer">
                <option v-for="p in state.projects" :key="p.id" :value="p.id">{{ p.title }}</option>
              </select>
              <ChevronDown class="w-3.5 h-3.5 text-ink-400 absolute right-2 top-2.5 pointer-events-none" />
            </div>
            <button 
              v-if="state.selectedProjectId"
              @click="handleDeleteActiveProject"
              class="p-1.5 bg-atelier-850 hover:bg-rose-500/15 text-ink-400 hover:text-rose-400 border border-atelier-700 hover:border-rose-500/30 rounded-md transition cursor-pointer shrink-0"
              title="彻底删除当前作品">
              <Trash2 class="w-3.5 h-3.5" />
            </button>
          </div>
          <button 
            v-else
            @click="state.showNewProjectModal = true"
            class="w-full py-2 px-2.5 bg-brand-amber/10 hover:bg-brand-amber/20 border border-brand-amber/30 rounded-md text-xs text-brand-amber font-medium flex items-center justify-center gap-1.5 transition cursor-pointer">
            <Plus class="w-3.5 h-3.5" />
            <span>新建第一部作品</span>
          </button>
        </div>

        <!-- 作品专属导航列表 (按真实写作三部曲生命周期编排，完全无 Studio 混杂) -->
        <nav class="flex-1 p-2 space-y-3 overflow-y-auto min-h-0">
          <div v-for="group in projectNavGroups" :key="group.title" class="space-y-0.5">
            <div class="px-3 pt-1 pb-1 text-[10px] font-mono tracking-wider text-ink-400 uppercase">
              {{ group.title }}
            </div>
            <button 
              v-for="item in group.items" 
              :key="item.id"
              @click="actions.setActiveTab(item.id)"
              :title="item.desc"
              :class="[
                'w-full flex items-center justify-between px-3 py-1.5 rounded-md text-xs font-medium transition group cursor-pointer text-left',
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
                class="text-[10px] font-mono px-1.5 py-0.2 rounded bg-atelier-750 text-ink-300 shrink-0">
                {{ item.badge }}
              </span>
            </button>
          </div>
        </nav>
      </div>
    </div>

    <!-- ========================================== -->
    <!-- 底部区域：区分全局模式与作品模式              -->
    <!-- ========================================== -->
    <!-- 全局模式底部 -->
    <div v-if="isGlobalMode" class="p-3 border-t border-atelier-750 bg-atelier-950/60 space-y-1.5 shrink-0">
      <div class="flex items-center justify-between text-[11px] text-ink-300">
        <span class="flex items-center gap-1.5">
          <Layers class="w-3.5 h-3.5 text-brand-amber" />
          <span>全库多作品架构</span>
        </span>
        <span class="font-mono text-ink-400 text-[10px]">{{ state.projects.length }} 部在库</span>
      </div>
      <div class="text-[10px] font-mono text-ink-400 text-center">
        本地安全存储 · 离线全自动流水线
      </div>
    </div>

    <!-- 作品模式底部 (单作品专用导出备份) -->
    <div v-else class="p-3 border-t border-atelier-750 bg-atelier-950/60 space-y-2 shrink-0">
      <div class="grid grid-cols-2 gap-1.5">
        <a 
          :href="state.currentProject ? '/api/projects/' + state.currentProject.id + '/export/json' : '#'" 
          download 
          class="flex items-center justify-center gap-1.5 py-1.5 px-2 text-[11px] font-medium bg-atelier-850 hover:bg-atelier-800 text-ink-300 hover:text-ink-100 rounded border border-atelier-700/80 transition text-center cursor-pointer"
          title="导出 JSON 格式备份数据">
          <Download class="w-3 h-3 text-ink-400" />
          <span>导出备份</span>
        </a>
        <a 
          :href="state.currentProject ? '/api/projects/' + state.currentProject.id + '/export/markdown' : '#'" 
          download 
          class="flex items-center justify-center gap-1.5 py-1.5 px-2 text-[11px] font-medium bg-brand-amber/10 hover:bg-brand-amber/20 text-brand-amber rounded border border-brand-amber/25 transition text-center cursor-pointer"
          title="导出 Markdown 全本文件">
          <BookDown class="w-3 h-3 text-brand-amber" />
          <span>导出全书</span>
        </a>
      </div>
      <div class="text-[10px] font-mono text-ink-400 text-center pt-0.5">
        当前作品数据 · 本地随时离线写作
      </div>
    </div>
  </aside>
</template>

<script setup>
import { computed } from 'vue';
import { state, computedState, actions } from '../../stores/appState';
import {
  Library,
  Lightbulb,
  MessagesSquare,
  Compass,
  BookOpen,
  LayoutGrid,
  PenTool,
  Cpu,
  Anchor,
  Archive,
  BarChart3,
  History,
  Network,
  Settings,
  Plus,
  ChevronDown,
  Download,
  BookDown,
  GitBranch,
  ListChecks,
  Rocket,
  AlertTriangle,
  CalendarClock,
  Gauge,
  Trash2,
  ArrowLeft,
  ArrowUpRight,
  Layers,
  BookText
} from 'lucide-vue-next';

const isGlobalMode = computed(() => computedState.isGlobalScope.value);

// 全局专属菜单分组 (无任何作品级菜单)
const globalNavGroups = computed(() => [
  {
    title: 'Studio 中枢',
    items: [
      { id: 'studio-library', label: '多作品总览', desc: '全库作品进度与健康度总览', icon: Library, badge: state.projects.length ? `${state.projects.length}部` : '' },
      { id: 'studio-ideas', label: '灵感 Inbox', desc: '记录捕捉灵感与快速转创世', icon: Lightbulb },
      { id: 'studio-chat', label: '全局对话', desc: '跨项目闲聊与宏观设定风暴', icon: MessagesSquare },
      { id: 'pipeline', label: '全自动流水线', desc: '一键从创世跑到成章（含停线提示）', icon: Rocket },
      { id: 'studio-todos', label: '停线待办', desc: '需要人工介入的章节与原因', icon: AlertTriangle },
      { id: 'studio-automation', label: '自动化排程', desc: '定时续产与开书队列，无人值守', icon: CalendarClock },
    ]
  },
  {
    title: '系统设置',
    items: [
      { id: 'config', label: 'AI 系统设置', desc: '接入的大模型与密钥管理', icon: Settings },
    ]
  }
]);

// 作品级专属菜单分组 (按三部曲工序划分，无任何 Studio 全局菜单)
const projectNavGroups = computed(() => [
  {
    title: '准备构思',
    items: [
      { id: 'framework', label: '作品设定', desc: '核心设定、规则与文风基调', icon: Compass },
      { id: 'codex', label: '设定集', desc: '人物、势力与世界观档案', icon: BookOpen, badge: state.codexEntries.length ? `${state.codexEntries.length}条` : '' },
      { id: 'matrix', label: '全书大纲', desc: '全书章节结构与剧情路线', icon: LayoutGrid, badge: state.matrixOverview?.total_scenes ? `${state.matrixOverview.total_scenes}场` : '' },
      { id: 'intents', label: '章节意图', desc: '按章提炼核心冲突、视角与情绪张力', icon: ListChecks },
    ]
  },
  {
    title: '动笔写作',
    items: [
      { id: 'workbench', label: '写正文', desc: '构思、起草与打磨每章手稿', icon: PenTool, badge: state.chapters.length ? `第${state.chapters.length + 1}章` : '第1章' },
      { id: 'statemachine', label: '人物状态', desc: '主角境界、战力与物品变动', icon: Cpu },
      { id: 'hooks', label: '伏笔簿', desc: '记录剧情暗线与回收进度', icon: Anchor, badge: state.hooks.filter(h => h.status !== 'RESOLVED').length ? `${state.hooks.filter(h => h.status !== 'RESOLVED').length}线` : '' },
      { id: 'forecast', label: '剧情树', desc: '剧情前瞻分支推演与计划路线', icon: GitBranch },
    ]
  },
  {
    title: '成果阅览',
    items: [
      { id: 'chapters', label: '章节目录', desc: '已完稿章节的归档与浏览', icon: Archive, badge: `${state.chapters.length}章` },
      { id: 'activity', label: '动态时间线', desc: '内容生产与变更留痕，可看全文详情', icon: History },
      { id: 'graph', label: '关系图谱', desc: '实体与势力错综关联图', icon: Network, badge: state.codexRelations.length ? `${state.codexRelations.length}条` : '' },
      { id: 'analytics', label: '全书体检', desc: '篇幅节奏、字数与走势', icon: BarChart3 },
      { id: 'metrics', label: '质量指标', desc: '章节终态、达标率与返工走势', icon: Gauge },
    ]
  }
]);

// 切换作品；被拒绝（存在进行中的生成任务）时把下拉框恢复为当前作品。
async function handleProjectChange(event) {
  const ok = await actions.selectProject(event.target.value);
  if (!ok) {
    event.target.value = state.selectedProjectId;
  }
}

async function handleDeleteActiveProject() {
  if (!state.selectedProjectId) return;
  await actions.deleteProject(state.selectedProjectId);
}
</script>
