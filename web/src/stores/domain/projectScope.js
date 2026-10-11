// 作品作用域状态：单一事实源。
//
// 所有“作品级”瞬态状态（在途手稿、审校结论、停线信息、弹窗调制基质…）
// 都属于一个作品的上下文。切换作品时必须整体清空，严禁把上一个作品
// 的内容/断点/实体 id 串入新作品（Fail-Closed：宁可清空重来，不可串稿）。

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
