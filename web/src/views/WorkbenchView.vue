<template>
  <!-- 当尚未选择任何项目时的极简美学引导页 (Zero State) -->
  <div v-if="!state.currentProject" class="flex-1 flex flex-col items-center justify-center p-8 bg-atelier-950 text-center relative overflow-hidden select-none">
    <div class="max-w-md w-full p-8 rounded-2xl bg-atelier-900/60 border border-atelier-750 backdrop-blur-xl shadow-atelier-lg space-y-6">
      <div class="w-16 h-16 rounded-2xl bg-gradient-to-br from-brand-amber/20 to-amber-600/10 border border-brand-amber/30 mx-auto flex items-center justify-center text-brand-amber text-2xl font-serif font-bold shadow-amber-glow">
        墨
      </div>
      <div class="space-y-2">
        <h3 class="text-xl font-bold font-serif text-ink-50 tracking-wide">
          开始创作
        </h3>
        <p class="text-xs text-ink-300 leading-relaxed">
          构思情节与起草正文，开启你的小说长篇连载。
        </p>
      </div>
      <div class="pt-2 flex flex-col gap-2.5">
        <button 
          @click="state.showNewProjectModal = true"
          class="w-full py-2.5 px-4 bg-gradient-to-r from-brand-amber to-amber-600 hover:from-brand-amber-hover hover:to-amber-500 text-atelier-950 font-bold text-xs rounded-lg transition shadow-atelier-md flex items-center justify-center gap-2 cursor-pointer">
          <Plus class="w-4 h-4" />
          <span>新建作品</span>
        </button>
        <button 
          @click="state.activeTab = 'config'"
          class="w-full py-2 px-4 bg-atelier-850 hover:bg-atelier-800 text-ink-200 border border-atelier-700 rounded-lg text-xs transition flex items-center justify-center gap-1.5 cursor-pointer">
          <Settings class="w-3.5 h-3.5 text-ink-400" />
          <span>配置 AI</span>
        </button>
      </div>
    </div>
  </div>

  <!-- 主创作手稿工作台 (3 栏弹性自适应架构) -->
  <div v-else class="flex-1 flex min-h-0 overflow-hidden relative">
    <!-- 禅模式悬浮退出胶囊条 -->
    <div 
      v-if="state.isZenMode" 
      class="absolute top-4 right-6 z-30 flex items-center gap-2">
      <button 
        @click="state.isZenMode = false"
        class="flex items-center gap-1.5 px-3 py-1.5 rounded-full bg-atelier-900/80 hover:bg-atelier-800 text-ink-300 hover:text-ink-50 border border-atelier-700/80 backdrop-blur-md text-xs transition shadow-atelier-md cursor-pointer"
        title="退出专注模式 (快捷键 ESC)">
        <Minimize2 class="w-3.5 h-3.5 text-brand-amber" />
        <span>退出专注</span>
      </button>
    </div>

    <!-- 左栏：六步工序 (宽 320px，支持折叠与无障碍滚动) -->
    <WorkbenchStepsPanel
      :on-render-scene="handleRenderScene"
      :on-rewrite-draft="handleRewriteDraft"
      :on-human-touches="openHumanTouchesModal"
      @set-render-error="(msg) => renderError = msg"
    />

    <!-- 中栏：文学手稿画布 (弹性自适应排版空间) -->
    <WorkbenchCanvasPanel
      :render-error="renderError"
      :render-elapsed-sec="renderElapsedSec"
      :rewrite-elapsed-sec="rewriteElapsedSec"
      :on-render-scene="handleRenderScene"
      :on-rewrite-draft="handleRewriteDraft"
      :on-human-touches="openHumanTouchesModal"
      @clear-render-error="renderError = null"
    />

  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, watch } from 'vue';
import { state, computedState, actions, notify } from '../stores/appState';
import { api } from '../api/client';
import { Plus, Settings, Minimize2 } from 'lucide-vue-next';
import WorkbenchStepsPanel from '../components/workbench/WorkbenchStepsPanel.vue';
import WorkbenchCanvasPanel from '../components/workbench/WorkbenchCanvasPanel.vue';

const renderElapsedSec = ref(0);
const rewriteElapsedSec = ref(0);
const renderError = ref(null);
let renderTimer = null;
let rewriteTimer = null;

// 文学起草
async function handleRenderScene() {
  if (!state.currentProject) return;
  state.isRenderingScene = true;
  renderError.value = null;
  renderElapsedSec.value = 0;
  if (renderTimer) clearInterval(renderTimer);
  renderTimer = setInterval(() => {
    renderElapsedSec.value++;
  }, 1000);

  const targetIndex = computedState.currentWorkingChapterIndex.value;
  notify(
    '正文起草已启动',
    `第 ${targetIndex} 章起草中，预计需要 10~30 秒`,
    'info',
    3500,
    { taskType: 'render', chapterIndex: targetIndex, status: 'started' }
  );

  try {
    let streamedDraft = '';
    let isFirstDelta = true;
    const res = await api.renderSceneStream(
      state.currentProject.id,
      {
        chapter_index: targetIndex,
        beats: state.workbench.beats,
        words_target: state.wordsTarget || 2000,
        narrative_style: state.narrativeStyle || 'hardboiled',
      },
      {
        onDelta: (delta) => {
          if (isFirstDelta) {
            streamedDraft = delta;
            state.workbench.content = streamedDraft;
            isFirstDelta = false;
          } else {
            streamedDraft += delta;
            state.workbench.content = streamedDraft;
          }
        },
      }
    );
    if (res && res.content) {
      state.workbench.content = res.content;
    }
    state.activeStep = 3;
    await actions.runLinter();
    await actions.saveCheckpoint();
    notify(
      '正文起草完成',
      `第 ${targetIndex} 章已完成约 ${state.workbench.content.length} 字 (耗时 ${renderElapsedSec.value}s)`,
      'success',
      4500,
      { taskType: 'render', chapterIndex: targetIndex, status: 'completed' }
    );
  } catch (e) {
    console.error('handleRenderScene error:', e);
    renderError.value = e.message || '正文初稿起草异常';
    notify(
      '正文起草失败',
      e.message || '请检查网络或配置',
      'error',
      6000,
      { taskType: 'render', chapterIndex: targetIndex, status: 'failed' }
    );
  } finally {
    state.isRenderingScene = false;
    if (renderTimer) {
      clearInterval(renderTimer);
      renderTimer = null;
    }
  }
}

// 自动精修
async function handleRewriteDraft() {
  if (!state.currentProject || !state.workbench.content.trim()) return;
  state.isRewriting = true;
  renderError.value = null;
  rewriteElapsedSec.value = 0;
  if (rewriteTimer) clearInterval(rewriteTimer);
  rewriteTimer = setInterval(() => {
    rewriteElapsedSec.value++;
  }, 1000);

  const targetIndex = computedState.currentWorkingChapterIndex.value;
  notify(
    '精修已启动',
    `正在依照修改意见优化第 ${targetIndex} 章手稿`,
    'info',
    3500,
    { taskType: 'rewrite', chapterIndex: targetIndex, status: 'started' }
  );

  try {
    const issues = state.reviewResult?.issues?.length 
      ? state.reviewResult.issues 
      : ['需根据去AI味与语法健全原则进行深度精修'];
    const suggestions = state.reviewResult?.suggestions 
      || '请重塑叙事节奏，补齐主谓宾完整结构，消除机械断句与模式化废词。';

    let streamedContent = '';
    let isFirstDelta = true;
    const res = await api.rewriteDraftStream(
      state.currentProject.id,
      {
        chapter_index: targetIndex,
        original_draft: state.workbench.content,
        content: state.workbench.content,
        issues: issues,
        suggestions: suggestions,
        words_target: state.wordsTarget || 2000,
        narrative_style: state.narrativeStyle || 'hardboiled',
        review: {
          verdict: 'REVISION_NEEDED',
          score: state.reviewResult?.score || 70,
          issues: issues,
          suggestions: suggestions,
        },
      },
      {
        onDelta: (delta) => {
          if (isFirstDelta) {
            streamedContent = delta;
            state.workbench.content = streamedContent;
            isFirstDelta = false;
          } else {
            streamedContent += delta;
            state.workbench.content = streamedContent;
          }
        },
      }
    );
    if (res && res.content) {
      state.workbench.content = res.content;
    }
    state.rewriteLoopCount++;
    await actions.runLinter();
    state.activeStep = 4;
    await actions.saveCheckpoint();
    notify(
      '精修完成',
      `已依照意见优化正文 (耗时 ${rewriteElapsedSec.value}s)`,
      'success',
      4500,
      { taskType: 'rewrite', chapterIndex: targetIndex, status: 'completed' }
    );
  } catch (e) {
    console.error('handleRewriteDraft error:', e);
    renderError.value = e.message || '正文精修接口异常';
    notify('精修失败', e.message || '请检查网络或配置', 'error', 6000, { taskType: 'rewrite', chapterIndex: targetIndex, status: 'failed' });
  } finally {
    state.isRewriting = false;
    if (rewriteTimer) {
      clearInterval(rewriteTimer);
      rewriteTimer = null;
    }
  }
}

function openHumanTouchesModal() {
  state.showHumanTouchesModal = true;
}

onMounted(() => {
  if (actions.restoreCheckpoint) {
    actions.restoreCheckpoint(computedState.currentWorkingChapterIndex.value);
  }
});

// 切换作品时清空本视图的瞬态渲染状态，防止上一个作品的错误横幅/计时残留。
watch(() => state.currentProject?.id, () => {
  renderError.value = null;
  renderElapsedSec.value = 0;
  rewriteElapsedSec.value = 0;
  if (renderTimer) {
    clearInterval(renderTimer);
    renderTimer = null;
  }
  if (rewriteTimer) {
    clearInterval(rewriteTimer);
    rewriteTimer = null;
  }
});

onUnmounted(() => {
  if (renderTimer) clearInterval(renderTimer);
  if (rewriteTimer) clearInterval(rewriteTimer);
});
</script>
