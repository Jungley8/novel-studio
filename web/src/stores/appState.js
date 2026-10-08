import { reactive, computed } from 'vue';
import { createConfigActions } from './domain/configActions';
import { createProjectActions } from './domain/projectActions';
import { createWorkbenchActions } from './domain/workbenchActions';
import { createMatrixActions } from './domain/matrixActions';
import { createCodexActions } from './domain/codexActions';

export const state = reactive({
  // Navigation & View
  activeTab: 'workbench',
  isLoading: false,
  globalLoadingMessage: '',
  showWorkflowPanel: true,
  showHorizonPanel: true,
  isZenMode: false,

  // System & Config
  config: {
    api_base: '',
    api_key: '',
    reasoning_model: 'deepseek-reasoner',
    writer_model: 'deepseek-chat',
    reviewer_model: 'deepseek-reasoner',
    reviewer_provider: {
      api_base: '',
      api_key: '',
      model: '',
    },
  },
  enableReviewerProvider: false,
  configTestStatus: {},

  // Projects
  projects: [],
  selectedProjectId: '',
  currentProject: null,
  chapters: [],
  hooks: [],

  // The Matrix
  matrixOverview: null,
  activeSceneForMarkers: null,
  sceneMarkersList: [],

  // The Codex
  codexEntries: [],
  codexCategoryFilter: 'ALL',
  activeCodexForSub: null,

  // Analytics
  analyticsHeatmap: null,
  analyticsTension: null,

  // Workbench Execution State
  activeStep: 1,
  wordsTarget: 2000,
  narrativeStyle: 'hardboiled',
  rewriteLoopCount: 0,
  isGeneratingBeats: false,
  isRenderingScene: false,
  isReviewing: false,
  isRewriting: false,
  workbench: {
    coreConflict: '',
    beats: [
      { phase: '蓄力压迫', tension: 4, action: '', expectation_broken: '' },
      { phase: '试探下套', tension: 6, action: '', expectation_broken: '' },
      { phase: '绝地反转', tension: 9, action: '', expectation_broken: '' },
      { phase: '章末留钩', tension: 8, action: '', expectation_broken: '' },
    ],
    stateMutation: {
      inventory_delta: '',
      power_delta: '',
    },
    content: '',
  },
  linterReport: {
    burstiness_score: 50,
    hit_banned_words: [],
    passed: true,
    message: '就绪',
  },
  reviewResult: null,

  // Pipeline State
  pipelineState: {
    active: false,
    lastFinished: false,
    phase: '',
    message: '',
    resumed: false,
    tokens: { prompt_tokens: 0, completion_tokens: 0, total_tokens: 0 },
  },

  // Modals & Panels Visibility
  showNewProjectModal: false,
  showPromptInspectorModal: false,
  showWorkshopChatDrawer: false,
  showSceneModal: false,
  editingScene: null,
  targetChapterIdForNewScene: '',
  showMarkersModal: false,
  showCodexModal: false,
  editingCodexEntry: null,
  showProgressionModal: false,
  showRelationModal: false,
  showHarmonizeModal: false,
  showHumanTouchesModal: false,

  // Toasts
  toasts: [],
});

export const computedState = {
  nextChapterIndex: computed(() => state.chapters.length + 1),
  activeHooksList: computed(() => state.hooks.filter(h => h.status !== 'RESOLVED')),
  characterCodexList: computed(() => state.codexEntries.filter(e => e.category === 'CHARACTER')),
  filteredCodexEntries: computed(() => {
    if (state.codexCategoryFilter === 'ALL') return state.codexEntries;
    return state.codexEntries.filter(e => e.category === state.codexCategoryFilter);
  }),
  currentVolume: computed(() => {
    if (!state.currentProject?.framework?.volume_arcs) return null;
    const ch = state.chapters.length + 1;
    return state.currentProject.framework.volume_arcs.find(v => ch >= v.start_chapter && ch <= v.end_chapter)
      || state.currentProject.framework.volume_arcs[0];
  }),
  currentPowerTier: computed(() => {
    if (!state.currentProject?.framework?.power_ladder) return null;
    const realm = state.currentProject.protagonist?.name_and_level || '';
    for (const tier of state.currentProject.framework.power_ladder) {
      if (realm.includes(tier.tier_name)) return tier;
    }
    return state.currentProject.framework.power_ladder[0];
  }),
  pipelineCostUSD: computed(() => {
    const p = state.pipelineState.tokens.prompt_tokens || 0;
    const c = state.pipelineState.tokens.completion_tokens || 0;
    return (p * 0.14 + c * 0.28) / 1000000;
  }),
};

// Toast Notifications Helper
export function notify(title, message = '', type = 'info', duration = 3500) {
  const id = Date.now() + Math.random().toString(36).substring(2, 6);
  state.toasts.push({ id, title, message, type });
  setTimeout(() => {
    state.toasts = state.toasts.filter(t => t.id !== id);
  }, duration);
}

// Assemble Domain Actions with Cross-Domain Coordination
const actionHelpers = {};

const configActs = createConfigActions(state, notify);
const matrixActs = createMatrixActions(state, notify, actionHelpers);
const codexActs = createCodexActions(state, notify);
const projectActs = createProjectActions(state, notify, actionHelpers);
const workbenchActs = createWorkbenchActions(state, notify, actionHelpers);

// Register cross-domain helper functions for inter-module workflows
actionHelpers.loadMatrixOverview = matrixActs.loadMatrixOverview;
actionHelpers.loadCodexEntries = codexActs.loadCodexEntries;
actionHelpers.loadAnalytics = matrixActs.loadAnalytics;
actionHelpers.selectProject = projectActs.selectProject;
actionHelpers.restoreCheckpoint = workbenchActs.restoreCheckpoint;

export const actions = {
  ...configActs,
  ...projectActs,
  ...workbenchActs,
  ...matrixActs,
  ...codexActs,
};
