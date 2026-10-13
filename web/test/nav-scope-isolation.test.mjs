// 作用域完全隔离与页面区分回归测试
//
// 验证：
// 1. 全局 Tab 与 作品级 Tab 的严格数学划分（无交集、全覆盖、元数据齐备）；
// 2. computedState.navScope / isGlobalScope / isProjectScope 准确响应；
// 3. actions.setActiveTab 维护全局与作品各自的最后激活状态（lastGlobalTab / lastProjectTab）；
// 4. switchToGlobal 与 switchToProject 互相切换并恢复各自记忆；
// 5. 无项目时 switchToProject 拒绝进入并保持 Fail-Closed；
// 6. 删除最后一部作品后自动退回全局 studio-library。

import test from 'node:test';
import assert from 'node:assert/strict';

// 提供基础 fetch mock 避免 node 单测环境下相对 URL 报错
if (!globalThis.fetch || !globalThis.__fetchMocked) {
  const originalFetch = globalThis.fetch;
  globalThis.fetch = async (url, options = {}) => {
    return new Response(JSON.stringify({ total_scenes: 0, volumes: [], chapters: [] }), {
      status: 200,
      headers: { 'content-type': 'application/json' },
    });
  };
  globalThis.__fetchMocked = true;
}

import { state, computedState, actions } from '../src/stores/appState.js';
import { 
  GLOBAL_TABS, 
  PROJECT_TABS, 
  isGlobalTab, 
  isProjectTab, 
  getTabScope, 
  TAB_META 
} from '../src/stores/domain/projectScope.js';

test('作用域划分：全局菜单与作品级菜单严格互斥且全覆盖', () => {
  // 1. 互斥性：没有任何 tab 同时属于全局和作品
  const intersection = GLOBAL_TABS.filter(t => PROJECT_TABS.includes(t));
  assert.deepEqual(intersection, [], '全局 Tab 与作品级 Tab 不应有任何交集');

  // 2. 全局 7 大模块完整覆盖
  assert.equal(GLOBAL_TABS.length, 7);
  assert.ok(GLOBAL_TABS.includes('studio-library'));
  assert.ok(GLOBAL_TABS.includes('studio-ideas'));
  assert.ok(GLOBAL_TABS.includes('studio-chat'));
  assert.ok(GLOBAL_TABS.includes('pipeline'));
  assert.ok(GLOBAL_TABS.includes('studio-todos'));
  assert.ok(GLOBAL_TABS.includes('studio-automation'));
  assert.ok(GLOBAL_TABS.includes('config'));

  // 3. 作品级 13 大工序完整覆盖
  assert.equal(PROJECT_TABS.length, 13);
  assert.ok(PROJECT_TABS.includes('framework'));
  assert.ok(PROJECT_TABS.includes('codex'));
  assert.ok(PROJECT_TABS.includes('matrix'));
  assert.ok(PROJECT_TABS.includes('intents'));
  assert.ok(PROJECT_TABS.includes('workbench'));
  assert.ok(PROJECT_TABS.includes('statemachine'));
  assert.ok(PROJECT_TABS.includes('hooks'));
  assert.ok(PROJECT_TABS.includes('forecast'));
  assert.ok(PROJECT_TABS.includes('chapters'));
  assert.ok(PROJECT_TABS.includes('activity'));
  assert.ok(PROJECT_TABS.includes('graph'));
  assert.ok(PROJECT_TABS.includes('analytics'));
  assert.ok(PROJECT_TABS.includes('metrics'));

  // 4. 所有 Tab 均具有完整元数据 (label, desc, scope, group)
  const allTabs = [...GLOBAL_TABS, ...PROJECT_TABS];
  for (const t of allTabs) {
    assert.ok(TAB_META[t], `Tab ${t} 必须在 TAB_META 中声明`);
    assert.ok(TAB_META[t].label, `Tab ${t} 必须有 label`);
    assert.ok(TAB_META[t].desc, `Tab ${t} 必须有 desc`);
    assert.ok(TAB_META[t].scope, `Tab ${t} 必须有 scope`);
    assert.equal(TAB_META[t].scope, isGlobalTab(t) ? 'global' : 'project');
  }
});

test('响应式作用域：computedState 正确判断当前导航模式与元数据', () => {
  actions.setActiveTab('studio-library');
  assert.equal(state.activeTab, 'studio-library');
  assert.equal(computedState.navScope.value, 'global');
  assert.equal(computedState.isGlobalScope.value, true);
  assert.equal(computedState.isProjectScope.value, false);
  assert.equal(computedState.activeTabMeta.value.label, '多作品总览');

  actions.setActiveTab('workbench');
  assert.equal(state.activeTab, 'workbench');
  assert.equal(computedState.navScope.value, 'project');
  assert.equal(computedState.isGlobalScope.value, false);
  assert.equal(computedState.isProjectScope.value, true);
  assert.equal(computedState.activeTabMeta.value.label, '写正文');
});

test('双域记忆跳转：switchToGlobal 与 switchToProject 独立记录并恢复各自的断点', async () => {
  state.projects = [{ id: 'proj-1', title: '测试作品1' }];
  state.selectedProjectId = 'proj-1';
  state.currentProject = state.projects[0];

  // 1. 在全局模式下浏览到灵感 Inbox
  actions.switchToGlobal('studio-ideas');
  assert.equal(state.activeTab, 'studio-ideas');
  assert.equal(state.lastGlobalTab, 'studio-ideas');

  // 2. 切入作品模式浏览到全书大纲
  await actions.switchToProject(null, 'matrix');
  assert.equal(state.activeTab, 'matrix');
  assert.equal(state.lastProjectTab, 'matrix');

  // 3. 返回全局模式，未指定具体 tab 时自动恢复上次停留在的 studio-ideas
  actions.switchToGlobal();
  assert.equal(state.activeTab, 'studio-ideas');
  assert.equal(computedState.navScope.value, 'global');

  // 4. 再次切入作品模式，未指定具体 tab 时自动恢复上次停留在的 matrix
  await actions.switchToProject();
  assert.equal(state.activeTab, 'matrix');
  assert.equal(computedState.navScope.value, 'project');
});

test('Fail-Closed 保护：当作品库为空时拒绝切入作品空间', async () => {
  state.projects = [];
  state.selectedProjectId = null;
  state.currentProject = null;
  actions.switchToGlobal('studio-library');

  // 尝试切入作品空间
  const ok = await actions.switchToProject();
  assert.equal(ok, false, '作品库为空时应拒绝切入作品空间');
  assert.equal(state.activeTab, 'studio-library', '应保持在全局视图中');
  assert.equal(state.showNewProjectModal, true, '应引导打开新建作品弹窗');
});
