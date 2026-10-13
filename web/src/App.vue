<template>
  <div class="flex h-screen bg-atelier-950 text-ink-100 font-sans overflow-hidden antialiased select-none">
    <!-- 侧边导航栏 (全局模式与作品空间物理隔离) -->
    <AppSidebar v-show="!state.isZenMode" />

    <!-- 主工作区容器 -->
    <main class="flex-1 flex flex-col min-w-0 bg-atelier-950 overflow-hidden relative">
      <!-- 顶部状态栏 (全局态与作品态严格区分) -->
      <AppHeader v-show="!state.isZenMode" @trigger-pipeline="actions.runAutonomousPipeline" />

      <!-- 视图挂载区 (根据 activeTab 响应式切换与隔离) -->
      <div class="flex-1 flex flex-col min-h-0 overflow-hidden relative">
        <!-- 作品空间无选中项目时的空态引导兜底 -->
        <div 
          v-if="computedState.isProjectScope.value && !state.currentProject"
          class="flex-1 flex flex-col items-center justify-center p-8 bg-atelier-950 text-center select-none">
          <div class="max-w-md w-full p-8 rounded-2xl bg-atelier-900/80 border border-atelier-750 shadow-atelier-lg space-y-5">
            <div class="w-14 h-14 rounded-2xl bg-brand-amber/10 border border-brand-amber/25 mx-auto flex items-center justify-center text-brand-amber shadow-amber-glow">
              <BookOpen class="w-6 h-6" />
            </div>
            <div class="space-y-1.5">
              <h3 class="text-base font-bold font-serif text-ink-50">尚未选定小说作品</h3>
              <p class="text-xs text-ink-400 leading-relaxed">
                当前处于作品创作空间。你可以从 Studio 多作品总览中挑选已有作品，或立即新建一部新书工程。
              </p>
            </div>
            <div class="flex flex-col sm:flex-row items-center justify-center gap-2.5 pt-2">
              <button 
                @click="actions.switchToGlobal('studio-library')"
                class="w-full sm:w-auto px-4 py-2 bg-atelier-850 hover:bg-atelier-800 text-ink-200 border border-atelier-700 rounded-lg text-xs font-medium transition cursor-pointer flex items-center justify-center gap-1.5">
                <ArrowLeft class="w-3.5 h-3.5 text-brand-amber" />
                <span>返回多作品总览</span>
              </button>
              <button 
                @click="state.showNewProjectModal = true"
                class="w-full sm:w-auto px-4 py-2 bg-brand-amber hover:bg-brand-amber-hover text-atelier-950 font-bold text-xs rounded-lg transition shadow-amber-glow cursor-pointer flex items-center justify-center gap-1.5">
                <Plus class="w-3.5 h-3.5" />
                <span>新建小说作品</span>
              </button>
            </div>
          </div>
        </div>

        <!-- 正常视图挂载 (作品级 13 页面 + 全局级 7 页面) -->
        <template v-else>
          <!-- 作品级：动笔写作与构思成果 -->
          <WorkbenchView v-show="state.activeTab === 'workbench'" />
          <MatrixView v-show="state.activeTab === 'matrix'" />
          <IntentsView v-show="state.activeTab === 'intents'" />
          <CodexView v-show="state.activeTab === 'codex'" />
          <FrameworkView v-show="state.activeTab === 'framework'" />
          <StateMachineView v-show="state.activeTab === 'statemachine'" />
          <HooksView v-show="state.activeTab === 'hooks'" />
          <ForecastView v-show="state.activeTab === 'forecast'" />
          <ChaptersView v-show="state.activeTab === 'chapters'" />
          <ActivityView v-show="state.activeTab === 'activity'" />
          <GraphView v-show="state.activeTab === 'graph'" />
          <AnalyticsView v-show="state.activeTab === 'analytics'" />
          <MetricsView v-show="state.activeTab === 'metrics'" />

          <!-- 全局级：Studio 中枢与系统配置 -->
          <StudioLibraryView v-show="state.activeTab === 'studio-library'" />
          <StudioIdeasView v-show="state.activeTab === 'studio-ideas'" />
          <StudioChatView v-show="state.activeTab === 'studio-chat'" />
          <PipelineView v-show="state.activeTab === 'pipeline'" />
          <StudioTodoView v-show="state.activeTab === 'studio-todos'" />
          <StudioAutomationView v-show="state.activeTab === 'studio-automation'" />
          <SettingsView v-show="state.activeTab === 'config'" />
        </template>
      </div>
    </main>

    <!-- 弹窗与抽屉集中挂载 -->
    <PromptInspectorModal />
    <WorkshopChatDrawer />
    <NewProjectModal />
    <SceneModal />
    <MarkersModal />
    <CodexModal />
    <ProgressionModal />
    <RelationModal />
    <HarmonizeModal />
    <HumanTouchesModal />
    <MessageCenterModal />

    <!-- 全局轻量浮层通知 (Toast) -->
    <ToastContainer />

    <!-- 全局跨平台交互模态对话框 (Confirm / Prompt / Alert) -->
    <GlobalDialogModal />
  </div>
</template>

<script setup>
import { onMounted, onErrorCaptured } from 'vue';
import { state, computedState, actions, startEditLeaseHeartbeat } from './stores/appState';
import { BookOpen, ArrowLeft, Plus } from 'lucide-vue-next';

onErrorCaptured((err, instance, info) => {
  console.error('[App onErrorCaptured]', err, info);
  actions.showToast?.(`界面渲染异常: ${err?.message || '未知错误'}`, 'error');
  return false;
});

// 基础布局与通用组件
import AppSidebar from './components/layout/AppSidebar.vue';
import AppHeader from './components/layout/AppHeader.vue';
import ToastContainer from './components/common/ToastContainer.vue';
import GlobalDialogModal from './components/common/GlobalDialogModal.vue';

// 作品级与全局业务功能视图
import WorkbenchView from './views/WorkbenchView.vue';
import MatrixView from './views/MatrixView.vue';
import IntentsView from './views/IntentsView.vue';
import CodexView from './views/CodexView.vue';
import FrameworkView from './views/FrameworkView.vue';
import StateMachineView from './views/StateMachineView.vue';
import HooksView from './views/HooksView.vue';
import ForecastView from './views/ForecastView.vue';
import ChaptersView from './views/ChaptersView.vue';
import ActivityView from './views/ActivityView.vue';
import GraphView from './views/GraphView.vue';
import AnalyticsView from './views/AnalyticsView.vue';
import MetricsView from './views/MetricsView.vue';
import SettingsView from './views/SettingsView.vue';
import StudioLibraryView from './views/StudioLibraryView.vue';
import StudioIdeasView from './views/StudioIdeasView.vue';
import StudioChatView from './views/StudioChatView.vue';
import PipelineView from './views/PipelineView.vue';
import StudioTodoView from './views/StudioTodoView.vue';
import StudioAutomationView from './views/StudioAutomationView.vue';

// 业务弹窗与抽屉
import PromptInspectorModal from './components/modals/PromptInspectorModal.vue';
import WorkshopChatDrawer from './components/modals/WorkshopChatDrawer.vue';
import NewProjectModal from './components/modals/NewProjectModal.vue';
import SceneModal from './components/modals/SceneModal.vue';
import MarkersModal from './components/modals/MarkersModal.vue';
import CodexModal from './components/modals/CodexModal.vue';
import ProgressionModal from './components/modals/ProgressionModal.vue';
import RelationModal from './components/modals/RelationModal.vue';
import HarmonizeModal from './components/modals/HarmonizeModal.vue';
import HumanTouchesModal from './components/modals/HumanTouchesModal.vue';
import MessageCenterModal from './components/modals/MessageCenterModal.vue';

onMounted(async () => {
  await actions.loadConfig();
  await actions.loadProjects();

  // 若作品库为空，默认进入全局多作品总览
  if (state.projects.length === 0) {
    actions.switchToGlobal('studio-library');
  }

  // 前台编辑租约心跳：在途编辑期间阻止调度器对同一作品并发续产
  startEditLeaseHeartbeat();

  window.addEventListener('keydown', (e) => {
    if (e.key === 'Escape' && state.isZenMode) {
      state.isZenMode = false;
    }
  });
});
</script>
