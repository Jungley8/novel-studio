<template>
  <div class="flex h-screen bg-atelier-950 text-ink-100 font-sans overflow-hidden antialiased select-none">
    <!-- 侧边导航栏 -->
    <AppSidebar v-show="!state.isZenMode" />

    <!-- 主工作区容器 -->
    <main class="flex-1 flex flex-col min-w-0 bg-atelier-950 overflow-hidden relative">
      <!-- 顶部状态栏 -->
      <AppHeader v-show="!state.isZenMode" @trigger-pipeline="actions.runAutonomousPipeline" />

      <!-- 视图挂载区 (根据 activeTab 响应式切换) -->
      <div class="flex-1 flex flex-col min-h-0 overflow-hidden relative">
        <WorkbenchView v-show="state.activeTab === 'workbench'" />
        <MatrixView v-show="state.activeTab === 'matrix'" />
        <CodexView v-show="state.activeTab === 'codex'" />
        <FrameworkView v-show="state.activeTab === 'framework'" />
        <StateMachineView v-show="state.activeTab === 'statemachine'" />
        <HooksView v-show="state.activeTab === 'hooks'" />
        <ChaptersView v-show="state.activeTab === 'chapters'" />
        <AnalyticsView v-show="state.activeTab === 'analytics'" />
        <SettingsView v-show="state.activeTab === 'config'" />
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

    <!-- 全局轻量浮层通知 (Toast) -->
    <ToastContainer />
  </div>
</template>

<script setup>
import { onMounted } from 'vue';
import { state, actions } from './stores/appState';

// 基础布局与通用组件
import AppSidebar from './components/layout/AppSidebar.vue';
import AppHeader from './components/layout/AppHeader.vue';
import ToastContainer from './components/common/ToastContainer.vue';

// 9 大业务功能视图
import WorkbenchView from './views/WorkbenchView.vue';
import MatrixView from './views/MatrixView.vue';
import CodexView from './views/CodexView.vue';
import FrameworkView from './views/FrameworkView.vue';
import StateMachineView from './views/StateMachineView.vue';
import HooksView from './views/HooksView.vue';
import ChaptersView from './views/ChaptersView.vue';
import AnalyticsView from './views/AnalyticsView.vue';
import SettingsView from './views/SettingsView.vue';

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

onMounted(async () => {
  await actions.loadConfig();
  await actions.loadProjects();

  window.addEventListener('keydown', (e) => {
    if (e.key === 'Escape' && state.isZenMode) {
      state.isZenMode = false;
    }
  });
});
</script>
