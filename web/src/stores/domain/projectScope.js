// 作品作用域与全局作用域划分：单一事实源。
//
// 全局域（Studio 全局管理与统筹）：
// 多作品总览、灵感 Inbox、全局对话、全自动流水线、停线待办、自动化排程、系统设置。
//
// 作品域（单部小说的创作生命周期工序）：
// 准备构思（作品设定、设定集、全书大纲、章节意图）、
// 动笔写作（写正文、人物状态、伏笔簿、剧情树）、
// 成果阅览（章节目录、动态时间线、关系图谱、全书体检、质量指标）。

export const GLOBAL_TABS = [
  'studio-library',
  'studio-chat',
  'pipeline',
  'studio-todos',
  'studio-automation',
  'config',
];

export const PROJECT_TABS = [
  // 准备构思
  'framework',
  'codex',
  'matrix',
  'intents',
  // 动笔写作
  'workbench',
  'statemachine',
  'hooks',
  'forecast',
  // 成果阅览
  'chapters',
  'activity',
  'graph',
  'analytics',
  'metrics',
];

export function isGlobalTab(tabId) {
  return GLOBAL_TABS.includes(tabId) || tabId === 'studio-ideas';
}

export function isProjectTab(tabId) {
  return PROJECT_TABS.includes(tabId);
}

export function getTabScope(tabId) {
  return isGlobalTab(tabId) ? 'global' : 'project';
}

export const TAB_META = {
  'studio-library': { label: '多作品总览', scope: 'global', group: 'Studio 中枢', desc: '全库小说创作进度、伏笔债务与多作品健康度全景' },
  'studio-chat': { label: '对话记录', scope: 'global', group: 'Studio 中枢', desc: '灵感脑洞推演、Inbox 沉淀与历史会话记录' },
  'studio-ideas': { label: '对话记录', scope: 'global', group: 'Studio 中枢', desc: '灵感脑洞推演、Inbox 沉淀与历史会话记录' },
  'pipeline': { label: '全自动流水线', scope: 'global', group: 'Studio 中枢', desc: '从核心脑洞一键自动化串通创世、设定推演与批量成章' },
  'studio-todos': { label: '停线待办', scope: 'global', group: 'Studio 中枢', desc: '全自动流水线中因质检不达标或冲突而需人工介入的章节' },
  'studio-automation': { label: '自动化排程', scope: 'global', group: 'Studio 中枢', desc: '全书定时续产与开书队列，无人值守流水线排程' },
  'config': { label: 'AI 系统设置', scope: 'global', group: '系统设置', desc: '接入的大语言模型、API 密钥与模型路由管理' },

  'framework': { label: '作品设定', scope: 'project', group: '准备构思', desc: '作品世界观底层规则、偏好基调与宏观三幕走向' },
  'codex': { label: '设定集', scope: 'project', group: '准备构思', desc: '人物、势力与世界观档案' },
  'matrix': { label: '全书大纲', scope: 'project', group: '准备构思', desc: '全书章节结构、分卷分幕与剧情场景路线' },
  'intents': { label: '章节意图', scope: 'project', group: '准备构思', desc: '按章提炼核心冲突、视角与情绪张力' },

  'workbench': { label: '写正文', scope: 'project', group: '动笔写作', desc: '文学手稿双视界画布、六步闭环工序与四拍推演' },
  'statemachine': { label: '人物状态', scope: 'project', group: '动笔写作', desc: '主角境界、战力数值与物品变动轨迹' },
  'hooks': { label: '伏笔簿', scope: 'project', group: '动笔写作', desc: '暗线布设与回收进度追踪' },
  'forecast': { label: '剧情树', scope: 'project', group: '动笔写作', desc: '多分支剧情前瞻推演与计划路线' },

  'chapters': { label: '章节目录', scope: 'project', group: '成果阅览', desc: '已完稿章节的归档、浏览与版本对比' },
  'activity': { label: '动态时间线', scope: 'project', group: '成果阅览', desc: '内容生产与变更留痕，全文详情审计' },
  'graph': { label: '关系图谱', scope: 'project', group: '成果阅览', desc: '实体与势力错综关联图网络' },
  'analytics': { label: '全书体检', scope: 'project', group: '成果阅览', desc: '全书篇幅节奏、字数分布与情绪走势' },
  'metrics': { label: '质量指标', scope: 'project', group: '成果阅览', desc: '章节终态、达标率与返工走势看板' },
};


// 默认四拍分段（新章起始状态）。
export function defaultBeats() {
  return [
    { phase: '蓄力压迫', tension: 4, action: '', expectation_broken: '' },
    { phase: '试探下套', tension: 6, action: '', expectation_broken: '' },
    { phase: '绝地反转', tension: 9, action: '', expectation_broken: '' },
    { phase: '章末留钩', tension: 8, action: '', expectation_broken: '' },
  ];
}

export function createDefaultWorkbench() {
  return {
    coreConflict: '',
    beats: defaultBeats(),
    stateMutation: {
      inventory_delta: '',
      power_delta: '',
    },
    content: '',
    originalDraftBeforeRewrite: null,
  };
}

export function createDefaultLinterReport() {
  return {
    burstiness_score: 50,
    hit_banned_words: [],
    empirical_tells: [],
    passed: true,
    message: '就绪',
  };
}

export function createDefaultPipelineState() {
  return {
    active: false,
    lastFinished: false,
    justCommitted: false,
    lastCommittedChapter: null,
    phase: '',
    message: '',
    resumed: false,
    tokens: { prompt_tokens: 0, completion_tokens: 0, total_tokens: 0 },
    halted: false,
    haltInfo: null,
  };
}

// 当前作品正在进行的互斥生成任务；返回任务名或 ''。
// 这些任务全部把产物写回“当前作品”的全局工作台状态，
// 切换作品会使其结果串稿，因此切换必须先等待其完成。
export function activeGenerationTask(state) {
  if (state.pipelineState?.active) return '全自动成章';
  if (state.bootstrappingProjectId && state.bootstrappingProjectId === state.selectedProjectId) return '创世推演';
  if (state.isSuggestingConflict) return '剧情构思';
  if (state.isGeneratingBeats) return '分段推演';
  if (state.isRenderingScene) return '正文起草';
  if (state.isReviewing) return '文风体检';
  if (state.isRewriting) return '精修重写';
  if (state.isCommitting) return '章节定稿';
  if (state.isSanitizing) return '去机械壳';
  if (state.isLinting) return '句式体检';
  return '';
}

// 切换作品时整体重置作品级瞬态状态（在加载新作品数据之前调用）。
export function resetProjectScopedState(state) {
  // 工作台在途稿、工序与审校
  state.activeStep = 1;
  state.editingChapterIndex = null;
  state.rewriteLoopCount = 0;
  state.lastSavedAt = null;
  state.workbench = createDefaultWorkbench();
  state.linterReport = createDefaultLinterReport();
  state.reviewResult = null;
  state.pipelineState = createDefaultPipelineState();

  // 实体弹窗调制基质（防止旧作品的实体 id 被提交到新作品）
  state.activeSceneForMarkers = null;
  state.sceneMarkersList = [];
  state.editingScene = null;
  state.targetChapterIdForNewScene = '';
  state.editingCodexEntry = null;
  state.activeCodexForSub = null;
  state.codexCategoryFilter = 'ALL';

  // 关闭所有作品级弹窗与抽屉（组件在重新打开时会自行按新作品初始化）
  state.showSceneModal = false;
  state.showMarkersModal = false;
  state.showCodexModal = false;
  state.showProgressionModal = false;
  state.showRelationModal = false;
  state.showHarmonizeModal = false;
  state.showHumanTouchesModal = false;
  state.showWorkshopChatDrawer = false;
  state.showPromptInspectorModal = false;
}
