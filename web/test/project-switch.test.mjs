// 回归测试：切换作品必须整体重置工作台与作品级瞬态状态，
// 且生成任务进行中必须拒绝切换（Fail-Closed）。
//
// 运行：pnpm test（node --import ./test/register-loader.mjs --test test/project-switch.test.mjs）

import test from 'node:test';
import assert from 'node:assert/strict';

import { state, actions } from '../src/stores/appState.js';

const projectA = { id: 'pA', title: '作品A', default_words_target: 3000, default_narrative_style: 'cinematic' };
const projectB = { id: 'pB', title: '作品B', world_rules: '', framework: null };

const calls = [];

// “最近在途断点”的可变配置：默认无断点，测试可按需注入。
const latestCheckpoints = { pA: null, pB: null };

function respondJson(data, status = 200) {
  return new Response(JSON.stringify(data), {
    status,
    headers: { 'content-type': 'application/json' },
  });
}

globalThis.fetch = async (url, options = {}) => {
  const method = (options.method || 'GET').toUpperCase();
  const u = String(url);
  calls.push({ url: u, method, body: options.body ? JSON.parse(options.body) : null });

  if (u === '/api/projects/pB' && method === 'GET') return respondJson(projectB);
  if (u === '/api/projects/pB/chapters') return respondJson([]);
  if (u === '/api/projects/pB/hooks') return respondJson([]);
  if (u === '/api/projects/pB/matrix') return respondJson({ total_scenes: 0, total_words: 0, volumes: [] });
  if (u === '/api/projects/pB/codex') return respondJson([]);
  if (u === '/api/projects/pB/codex/relations') return respondJson([]);
  if (u === '/api/projects/pB/analytics/heatmap') return respondJson({});
  if (u === '/api/projects/pB/analytics/tension') return respondJson({});

  if (u === '/api/projects/pA' && method === 'GET') return respondJson(projectA);
  if (u === '/api/projects/pA/chapters') {
    return respondJson([
      { id: 'ch-a1', chapter_index: 1, title: '第一章' },
      { id: 'ch-a2', chapter_index: 2, title: '第二章' },
    ]);
  }
  if (u === '/api/projects/pA/hooks') return respondJson([]);
  if (u === '/api/projects/pA/matrix') return respondJson({ total_scenes: 0, total_words: 0, volumes: [] });
  if (u === '/api/projects/pA/codex') return respondJson([]);
  if (u === '/api/projects/pA/codex/relations') return respondJson([]);
  if (u === '/api/projects/pA/analytics/heatmap') return respondJson({});
  if (u === '/api/projects/pA/analytics/tension') return respondJson({});

  // 断点：latest 查询返回配置的“最近在途断点”；按章节 GET 一律 404；POST 落盘成功。
  if (u === '/api/projects/pA/checkpoint?latest=true') return respondJson(latestCheckpoints.pA);
  if (u === '/api/projects/pB/checkpoint?latest=true') return respondJson(latestCheckpoints.pB);
  if (u.startsWith('/api/projects/pA/checkpoint') || u.startsWith('/api/projects/pB/checkpoint')) {
    if (method === 'GET') return respondJson({ error: 'checkpoint not found' }, 404);
    if (method === 'POST') return respondJson({ status: 'ok' });
  }

  if (u === '/api/linter/analyze') return respondJson({ burstiness_score: 60, passed: true, hit_banned_words: [] });

  return respondJson({ error: `unrouted: ${method} ${u}` }, 500);
};

test('切换作品整体重置工作台与作品级状态，并把旧作品在途草稿落盘到旧作品', async () => {
  // —— 预置：作品A 处于“正在精修第 7 章”的中间态，含正文/审校/停线/弹窗基质 ——
  state.selectedProjectId = 'pA';
  state.currentProject = projectA;
  state.chapters = [{ id: 'ch-a1', chapter_index: 1 }, { id: 'ch-a2', chapter_index: 2 }];
  state.workbench.content = 'A的正文草稿';
  state.workbench.coreConflict = 'A的核心冲突';
  state.workbench.beats = [{ phase: '自定义', tension: 5, action: 'A的动作', expectation_broken: '' }];
  state.workbench.stateMutation = { inventory_delta: '+宝剑', power_delta: '+1' };
  state.reviewResult = { verdict: 'REVISION_NEEDED', score: 60, issues: ['问题1'] };
  state.linterReport = { burstiness_score: 12, hit_banned_words: ['仿佛'], empirical_tells: [], passed: false, message: '不达标' };
  state.editingChapterIndex = 7;
  state.activeStep = 5;
  state.rewriteLoopCount = 2;
  state.lastSavedAt = new Date();
  state.pipelineState.active = false;
  state.pipelineState.halted = true;
  state.pipelineState.haltInfo = { chapterIndex: 7, verdict: 'REVISION_NEEDED', score: 60 };
  state.pipelineState.tokens = { prompt_tokens: 100, completion_tokens: 200, total_tokens: 300 };
  state.pipelineState.message = '第 7 章已停线待人工';
  state.activeSceneForMarkers = { id: 'sc-a' };
  state.sceneMarkersList = [{ id: 'mk-a' }];
  state.editingScene = { id: 'sc-a' };
  state.targetChapterIdForNewScene = 'ch-a9';
  state.editingCodexEntry = { id: 'codex-a' };
  state.activeCodexForSub = { id: 'codex-a' };
  state.codexCategoryFilter = 'ITEM';
  state.showCodexModal = true;
  state.showSceneModal = true;
  state.showWorkshopChatDrawer = true;

  const ok = await actions.selectProject('pB');
  assert.equal(ok, true);

  // 旧作品在途草稿先落盘到旧作品（pA / 第 7 章），不得丢弃、不得写错项目。
  const flushCall = calls.find(c => c.method === 'POST' && c.url === '/api/projects/pA/checkpoint');
  assert.ok(flushCall, '切换前应把旧作品草稿落盘到旧作品');
  assert.equal(flushCall.body.chapter_index, 7);
  assert.equal(flushCall.body.draft_text, 'A的正文草稿');

  // 新作品上下文
  assert.equal(state.selectedProjectId, 'pB');
  assert.equal(state.currentProject.title, '作品B');

  // 工作台整体重置（本缺陷的核心回归点）
  assert.equal(state.workbench.content, '');
  assert.equal(state.workbench.coreConflict, '');
  assert.equal(state.workbench.beats.length, 4);
  assert.equal(state.workbench.beats[0].phase, '蓄力压迫');
  assert.deepEqual(state.workbench.stateMutation, { inventory_delta: '', power_delta: '' });
  assert.equal(state.reviewResult, null);
  assert.equal(state.linterReport.message, '就绪');
  assert.equal(state.linterReport.burstiness_score, 50);
  assert.equal(state.editingChapterIndex, null);
  assert.equal(state.activeStep, 1);
  assert.equal(state.rewriteLoopCount, 0);
  assert.equal(state.lastSavedAt, null);

  // 流水线/停线瞬态重置
  assert.equal(state.pipelineState.active, false);
  assert.equal(state.pipelineState.halted, false);
  assert.equal(state.pipelineState.haltInfo, null);
  assert.equal(state.pipelineState.tokens.total_tokens, 0);
  assert.equal(state.pipelineState.message, '');

  // 弹窗基质重置
  assert.equal(state.activeSceneForMarkers, null);
  assert.deepEqual(state.sceneMarkersList, []);
  assert.equal(state.editingScene, null);
  assert.equal(state.targetChapterIdForNewScene, '');
  assert.equal(state.editingCodexEntry, null);
  assert.equal(state.activeCodexForSub, null);
  assert.equal(state.codexCategoryFilter, 'ALL');
  assert.equal(state.showCodexModal, false);
  assert.equal(state.showSceneModal, false);
  assert.equal(state.showWorkshopChatDrawer, false);

  // 新作品的单章默认参数与断点恢复目标（恢复该作品最近在途断点）
  assert.equal(state.wordsTarget, 2000);
  assert.equal(state.narrativeStyle, 'hardboiled');
  const restoreCall = calls.find(c => c.method === 'GET' && c.url === '/api/projects/pB/checkpoint?latest=true');
  assert.ok(restoreCall, '切换后应恢复新作品最近在用的一章断点');
});

test('生成任务进行中拒绝切换作品（Fail-Closed）', async () => {
  const before = calls.length;
  const workbenchBefore = state.workbench.content;

  state.pipelineState.active = true;
  const ok = await actions.selectProject('pA');
  state.pipelineState.active = false;

  assert.equal(ok, false, '生成任务进行中必须拒绝切换');
  assert.equal(state.selectedProjectId, 'pB', '被拒绝后仍停留在当前作品');
  assert.equal(state.workbench.content, workbenchBefore, '被拒绝时不得触碰工作台内容');
  const touchedPA = calls.slice(before).some(c => c.url.includes('/api/projects/pA/'));
  assert.equal(touchedPA, false, '被拒绝时不得发起旧作品/目标作品的任何加载请求');
});

test('工作台为空时切换不产生多余落盘，按 latest 断点恢复', async () => {
  const before = calls.length;
  const ok = await actions.selectProject('pA');
  assert.equal(ok, true);

  const flushCalls = calls.slice(before).filter(c => c.method === 'POST' && c.url.includes('/checkpoint'));
  assert.equal(flushCalls.length, 0, '空工作台切换不应产生落盘写入');

  const restoreCall = calls.slice(before).find(c => c.method === 'GET' && c.url === '/api/projects/pA/checkpoint?latest=true');
  assert.ok(restoreCall, '切换后应查询新作品最近在途断点');

  // 项目默认值覆盖到当前作品
  assert.equal(state.wordsTarget, 3000);
  assert.equal(state.narrativeStyle, 'cinematic');
  assert.equal(state.workbench.content, '');
});

test('切换作品恢复“最近推进的一章”在途断点（含已归档章节重修稿）', async () => {
  // pB 的最近断点是第 4 章的重修稿（而非“下一章”）
  latestCheckpoints.pB = {
    project_id: 'pB',
    chapter_index: 4,
    phase: 'DRAFTED',
    core_conflict: 'B的在途冲突',
    draft_text: 'B的在途重修稿',
    beats: null,
    state_mutation: null,
    audit_report: null,
    rewrite_loops: 0,
  };

  const ok = await actions.selectProject('pB');
  latestCheckpoints.pB = null;
  assert.equal(ok, true);

  assert.equal(state.workbench.content, 'B的在途重修稿', '应恢复最近在途断点的正文');
  assert.equal(state.workbench.coreConflict, 'B的在途冲突');
  assert.equal(state.editingChapterIndex, 4, '编辑章节应指向断点章节');
  assert.equal(state.activeStep, 3, 'DRAFTED 断点应定位到起草工序');

  // 恢复后在途断点可继续保存（不产生跨作品写入）
  const before = calls.length;
  const postSwitch = calls.slice(before);
  assert.equal(postSwitch.some(c => c.method === 'POST' && c.url.includes('/checkpoint')), false);
});
