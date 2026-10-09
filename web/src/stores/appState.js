import { reactive, computed } from 'vue';
import { createConfigActions } from './domain/configActions';
import { createProjectActions } from './domain/projectActions';
import { createWorkbenchActions } from './domain/workbenchActions';
import { createMatrixActions } from './domain/matrixActions';
import { createCodexActions } from './domain/codexActions';

function loadStoredMessages() {
  if (typeof window === 'undefined') return [];
  try {
    const raw = localStorage.getItem('novel_studio_notifications');
    if (raw) {
      const parsed = JSON.parse(raw);
      if (Array.isArray(parsed)) return parsed;
    }
  } catch (_) {}
  return [];
}

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
  editingChapterIndex: null,
  wordsTarget: 2000,
  narrativeStyle: 'hardboiled',
  rewriteLoopCount: 0,
  isGeneratingBeats: false,
  isRenderingScene: false,
  isReviewing: false,
  isRewriting: false,
  isSavingDraft: false,
  isCommitting: false,
  isSanitizing: false,
  isLinting: false,
  lastSavedAt: null,
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
    empirical_tells: [],
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
  showMessageCenterModal: false,

  // Message & Notification Center
  enableSystemNotifications: true,
  systemNotificationPermission: typeof window !== 'undefined' && 'Notification' in window ? Notification.permission : 'unsupported',
  messages: loadStoredMessages(),

  // Toasts
  toasts: [],

  // Global Dialog (Cross-Platform Confirm / Alert / Prompt)
  dialog: {
    isOpen: false,
    mode: 'confirm', // 'confirm' | 'alert' | 'prompt'
    title: '',
    message: '',
    details: '',
    input: '',
    placeholder: '',
    multiline: false,
    type: 'warning', // 'danger' | 'warning' | 'info' | 'success'
    confirmText: '确认',
    cancelText: '取消',
    resolve: null,
  },
});

export const computedState = {
  nextChapterIndex: computed(() => state.chapters.length + 1),
  currentWorkingChapterIndex: computed(() => state.editingChapterIndex || (state.chapters.length + 1)),
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
  unreadMessageCount: computed(() => {
    return state.messages.filter(m => !m.read).length;
  }),
  activeTasks: computed(() => {
    const tasks = [];
    if (state.pipelineState.active) {
      tasks.push({
        id: 'pipeline',
        type: 'pipeline',
        title: '全流程自主闭环推演',
        desc: state.pipelineState.message || '自主推演进行中...',
      });
    }
    if (state.isRenderingScene) {
      tasks.push({
        id: 'render',
        type: 'render',
        title: `第 ${state.editingChapterIndex || (state.chapters.length + 1)} 章正文起草`,
        desc: `目标约 ${state.wordsTarget || 2000} 字 · ${state.narrativeStyle}`,
      });
    }
    if (state.isGeneratingBeats) {
      tasks.push({
        id: 'beats',
        type: 'beats',
        title: `第 ${state.editingChapterIndex || (state.chapters.length + 1)} 章分段构思`,
        desc: '正在梳理情节与冲突安排...',
      });
    }
    if (state.isReviewing) {
      tasks.push({
        id: 'review',
        type: 'review',
        title: `第 ${state.editingChapterIndex || (state.chapters.length + 1)} 章文风体检`,
        desc: '正在检查文风与文字质量...',
      });
    }
    if (state.isRewriting) {
      tasks.push({
        id: 'rewrite',
        type: 'rewrite',
        title: `第 ${state.editingChapterIndex || (state.chapters.length + 1)} 章逐条精修`,
        desc: `第 ${state.rewriteLoopCount} 轮精修润色...`,
      });
    }
    if (state.isCommitting) {
      tasks.push({
        id: 'commit',
        type: 'commit',
        title: `第 ${state.editingChapterIndex || (state.chapters.length + 1)} 章保存入库`,
        desc: '正在保存本章手稿与状态...',
      });
    }
    if (state.isSanitizing) {
      tasks.push({
        id: 'sanitize',
        type: 'sanitize',
        title: '去除机械壳',
        desc: '正在去除机械套话与多余冒号...',
      });
    }
    return tasks;
  }),
};

// Request Native Desktop / Web Notification Permission
export async function requestNotificationPermission() {
  if (typeof window === 'undefined' || !('Notification' in window)) {
    notify('当前环境不支持桌面通知', '系统未开放通知接口', 'warning');
    return 'unsupported';
  }
  try {
    const perm = await Notification.requestPermission();
    state.systemNotificationPermission = perm;
    if (perm === 'granted') {
      notify('系统通知已开启', '章节生成完成时将弹出通知提醒', 'success');
    } else {
      notify('系统通知未开启', '可随时在系统设置中开启通知', 'info');
    }
    return perm;
  } catch (e) {
    console.warn('requestNotificationPermission error:', e);
    return 'denied';
  }
}

// Send Native Desktop / System Notification
export function sendSystemNotification(title, body = '') {
  if (typeof window === 'undefined' || !('Notification' in window)) {
    return;
  }
  if (Notification.permission === 'granted' && state.enableSystemNotifications) {
    try {
      const n = new Notification(title, {
        body: body || title,
        icon: '/favicon.ico',
        tag: 'novel-studio-' + Date.now(),
      });
      n.onclick = () => {
        window.focus?.();
        n.close();
      };
    } catch (e) {
      console.warn('sendSystemNotification failed:', e);
    }
  }
}

// Toast & Message Center Notifications Helper
export function notify(title, message = '', type = 'info', duration = 3500, meta = {}) {
  const id = 'msg_' + Date.now() + '_' + Math.random().toString(36).substring(2, 6);
  if (duration > 0) {
    state.toasts.push({ id, title, message, type });
    setTimeout(() => {
      state.toasts = state.toasts.filter(t => t.id !== id);
    }, duration);
  }

  // Record in persistent Message Center
  const newMsg = {
    id,
    title,
    message: typeof message === 'string' ? message : JSON.stringify(message),
    type,
    timestamp: new Date().toISOString(),
    read: false,
    ...meta,
  };
  state.messages.unshift(newMsg);
  if (state.messages.length > 100) {
    state.messages.pop();
  }
  try {
    localStorage.setItem('novel_studio_notifications', JSON.stringify(state.messages.slice(0, 50)));
  } catch (_) {}

  // Trigger Native Desktop Notification on task completion or failure
  if (type === 'success' || type === 'error' || type === 'warning') {
    sendSystemNotification(title, typeof message === 'string' ? message : '');
  }
}

// Cross-Platform Global Dialogs Helper (Web & Desktop Native Window Compatible)
export const dialogs = {
  confirm(options) {
    return new Promise((resolve) => {
      const opts = typeof options === 'string' ? { message: options } : (options || {});
      state.dialog = {
        isOpen: true,
        mode: 'confirm',
        title: opts.title || '操作确认',
        message: opts.message || '',
        details: opts.details || '',
        input: '',
        placeholder: '',
        multiline: false,
        type: opts.type || 'warning',
        confirmText: opts.confirmText || '确认',
        cancelText: opts.cancelText || '取消',
        resolve: (val) => {
          state.dialog.isOpen = false;
          resolve(Boolean(val));
        },
      };
    });
  },

  alert(options) {
    return new Promise((resolve) => {
      const opts = typeof options === 'string' ? { message: options } : (options || {});
      state.dialog = {
        isOpen: true,
        mode: 'alert',
        title: opts.title || '系统提示',
        message: opts.message || '',
        details: opts.details || '',
        input: '',
        placeholder: '',
        multiline: false,
        type: opts.type || 'info',
        confirmText: opts.confirmText || '我知道了',
        cancelText: '',
        resolve: () => {
          state.dialog.isOpen = false;
          resolve();
        },
      };
    });
  },

  prompt(options) {
    return new Promise((resolve) => {
      const opts = typeof options === 'string' ? { message: options } : (options || {});
      state.dialog = {
        isOpen: true,
        mode: 'prompt',
        title: opts.title || '请输入指令',
        message: opts.message || '',
        details: opts.details || '',
        input: opts.defaultValue || '',
        placeholder: opts.placeholder || '请输入内容...',
        multiline: Boolean(opts.multiline),
        type: opts.type || 'info',
        confirmText: opts.confirmText || '确定',
        cancelText: opts.cancelText || '取消',
        resolve: (val) => {
          state.dialog.isOpen = false;
          resolve(val);
        },
      };
    });
  },
};

// Assemble Domain Actions with Cross-Domain Coordination
const actionHelpers = {};

const configActs = createConfigActions(state, notify);
const matrixActs = createMatrixActions(state, notify, actionHelpers, dialogs);
const codexActs = createCodexActions(state, notify, dialogs);
const projectActs = createProjectActions(state, notify, actionHelpers, dialogs);
const workbenchActs = createWorkbenchActions(state, notify, actionHelpers, dialogs);

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
  confirm: dialogs.confirm,
  alert: dialogs.alert,
  prompt: dialogs.prompt,
  requestNotificationPermission,
  markAllMessagesRead() {
    state.messages.forEach(m => { m.read = true; });
    try {
      localStorage.setItem('novel_studio_notifications', JSON.stringify(state.messages.slice(0, 50)));
    } catch (_) {}
  },
  markMessageRead(id) {
    const msg = state.messages.find(m => m.id === id);
    if (msg) {
      msg.read = true;
      try {
        localStorage.setItem('novel_studio_notifications', JSON.stringify(state.messages.slice(0, 50)));
      } catch (_) {}
    }
  },
  clearAllMessages() {
    state.messages = [];
    try {
      localStorage.removeItem('novel_studio_notifications');
    } catch (_) {}
  },
  removeMessage(id) {
    state.messages = state.messages.filter(m => m.id !== id);
    try {
      localStorage.setItem('novel_studio_notifications', JSON.stringify(state.messages.slice(0, 50)));
    } catch (_) {}
  },
};

