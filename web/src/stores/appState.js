import { reactive, computed } from 'vue';
import { api } from '../api/client';

export const state = reactive({
  // Navigation & View
  activeTab: 'workbench',
  isLoading: false,
  globalLoadingMessage: '',

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

// Actions
export const actions = {
  async loadConfig() {
    try {
      const data = await api.getConfig();
      state.config = {
        ...state.config,
        ...data,
        reviewer_provider: data.reviewer_provider || { api_base: '', api_key: '', model: '' },
      };
      if (data.reviewer_provider && (data.reviewer_provider.api_base || data.reviewer_provider.api_key || data.reviewer_provider.model)) {
        state.enableReviewerProvider = true;
      }
    } catch (e) {
      console.error('load config error:', e);
    }
  },

  async saveConfig() {
    try {
      const payload = { ...state.config };
      if (!state.enableReviewerProvider) {
        payload.reviewer_provider = null;
      }
      await api.saveConfig(payload);
      notify('系统配置已保存', '模型与路由参数已实时更新生效', 'success');
    } catch (e) {
      notify('保存配置失败', e.message, 'error');
    }
  },

  async loadProjects() {
    try {
      state.projects = await api.listProjects();
      if (state.projects.length > 0 && !state.selectedProjectId) {
        state.selectedProjectId = state.projects[0].id;
        await this.selectProject(state.projects[0].id);
      }
    } catch (e) {
      console.error('load projects error:', e);
    }
  },

  async selectProject(id) {
    if (!id) return;
    state.selectedProjectId = id;
    try {
      state.currentProject = await api.getProject(id);
      state.chapters = await api.listChapters(id);
      state.hooks = await api.listHooks(id);
      await this.loadMatrixOverview();
      await this.loadCodexEntries();
      await this.loadAnalytics();
    } catch (e) {
      console.error('select project error:', e);
      notify('加载项目失败', e.message, 'error');
    }
  },

  async loadMatrixOverview() {
    if (!state.currentProject) return;
    try {
      state.matrixOverview = await api.getMatrixOverview(state.currentProject.id);
    } catch (e) {
      console.error('load matrix error:', e);
    }
  },

  async loadCodexEntries() {
    if (!state.currentProject) return;
    try {
      state.codexEntries = await api.listCodexEntries(state.currentProject.id);
    } catch (e) {
      console.error('load codex error:', e);
    }
  },

  async loadAnalytics() {
    if (!state.currentProject) return;
    try {
      state.analyticsHeatmap = await api.getAnalyticsHeatmap(state.currentProject.id);
      state.analyticsTension = await api.getAnalyticsTension(state.currentProject.id);
    } catch (e) {
      console.error('load analytics error:', e);
    }
  },

  async runLinter() {
    if (!state.workbench.content) return;
    try {
      state.linterReport = await api.lintAnalyze(state.workbench.content);
    } catch (e) {
      console.error('linter error:', e);
    }
  },

  async saveCurrentProject() {
    if (!state.currentProject) return;
    try {
      await api.updateProject(state.currentProject);
      notify('项目设定已保存', '实体物理状态与规则已持久化', 'success');
    } catch (e) {
      notify('保存项目失败', e.message, 'error');
    }
  },

  async saveFramework() {
    if (!state.currentProject?.framework) return;
    try {
      await api.updateFramework(state.currentProject.id, state.currentProject.framework);
      notify('创世总纲已保存', '天道公理与战力阶梯已同步', 'success');
    } catch (e) {
      notify('保存创世总纲失败', e.message, 'error');
    }
  },

  async bootstrapCurrentFramework(concept) {
    if (!state.currentProject) return;
    state.isLoading = true;
    try {
      const c = concept || state.currentProject.framework?.core_concept || state.currentProject.title;
      state.currentProject.framework = await api.bootstrapFramework(state.currentProject.id, c);
      notify('创世推演完成', '天道法则与战力天平已自动构建', 'success');
    } catch (e) {
      notify('创世推演失败', e.message, 'error');
    } finally {
      state.isLoading = false;
    }
  },

  async createPlotHook(title, details, targetChapter) {
    if (!state.currentProject) return;
    try {
      const newHook = {
        title: title || '未命名伏笔',
        details: details || '',
        created_chapter: state.chapters.length + 1,
        target_chapter: targetChapter || state.chapters.length + 3,
        status: 'OPEN',
      };
      await api.createHook(state.currentProject.id, newHook);
      state.hooks = await api.listHooks(state.currentProject.id);
      notify('伏笔已记录', '因果账本已更新', 'success');
    } catch (e) {
      notify('新建伏笔失败', e.message, 'error');
    }
  },

  async updatePlotHook(hook) {
    if (!state.currentProject) return;
    try {
      await api.updateHook(state.currentProject.id, hook);
      notify('伏笔状态已更新', '', 'success', 2000);
    } catch (e) {
      notify('更新伏笔失败', e.message, 'error');
    }
  },

  async deletePlotHook(id) {
    if (!confirm('确定删除该伏笔记录吗？')) return;
    try {
      await api.deleteHook(id);
      state.hooks = state.hooks.filter(h => h.id !== id);
      notify('伏笔已删除', '', 'info', 2000);
    } catch (e) {
      notify('删除伏笔失败', e.message, 'error');
    }
  },

  async deleteScene(sceneId) {
    if (!confirm('确定删除该场次吗？')) return;
    try {
      await api.deleteScene(sceneId);
      await this.loadMatrixOverview();
      notify('场次已删除', '', 'info', 2000);
    } catch (e) {
      notify('删除场次失败', e.message, 'error');
    }
  },

  async deleteCodexEntry(entryId) {
    if (!confirm('确定删除该百科实体吗？')) return;
    try {
      await api.deleteCodexEntry(state.currentProject.id, entryId);
      state.codexEntries = state.codexEntries.filter(e => e.id !== entryId);
      notify('百科实体已删除', '', 'info', 2000);
    } catch (e) {
      notify('删除实体失败', e.message, 'error');
    }
  },
};

