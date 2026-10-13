// 回归测试：创世推演异步化、前台即时响应跳转、后台任务跟踪与状态机更新
//
// 运行：node --import ./test/register-loader.mjs --test test/async-bootstrap.test.mjs

import test from 'node:test';
import assert from 'node:assert/strict';

import { state, actions, computedState } from '../src/stores/appState.js';
import { activeGenerationTask } from '../src/stores/domain/projectScope.js';

function respondJson(data, status = 200) {
  return new Response(JSON.stringify(data), {
    status,
    headers: { 'content-type': 'application/json' },
  });
}

test('异步创世推演：创建后即时响应，任务栏显示创世推演，轮询完成后自动刷新作品设定', async () => {
  const projBoot = {
    id: 'proj_async_1',
    title: '万界独尊',
    target_platform: '番茄脑洞',
    world_rules: '【核心立意与构想】\n凡人逆天',
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
  };

  const projCompleted = {
    ...projBoot,
    framework: {
      theme_premise: '凡人逆天弑神',
      world_axioms: ['神明寄生'],
      volume_arcs: [{ volume_index: 1, title: '初入江湖', start_chapter: 1, end_chapter: 30 }],
    },
  };

  let pollCount = 0;
  globalThis.fetch = async (url, options = {}) => {
    const method = (options.method || 'GET').toUpperCase();
    const u = String(url);

    if (u === '/api/projects/bootstrap' && method === 'POST') {
      return respondJson(projBoot, 201);
    }
    if (u === '/api/projects' && method === 'GET') {
      return respondJson([pollCount >= 1 ? projCompleted : projBoot]);
    }
    if (u === '/api/projects/proj_async_1' && method === 'GET') {
      return respondJson(pollCount >= 1 ? projCompleted : projBoot);
    }
    if (u === '/api/projects/proj_async_1/chapters') return respondJson([]);
    if (u === '/api/projects/proj_async_1/hooks') return respondJson([]);
    if (u === '/api/projects/proj_async_1/matrix') return respondJson({ total_scenes: 0, total_words: 0, volumes: [] });
    if (u === '/api/projects/proj_async_1/codex') return respondJson([]);
    if (u === '/api/projects/proj_async_1/codex/relations') return respondJson([]);
    if (u === '/api/projects/proj_async_1/analytics/heatmap') return respondJson({});
    if (u === '/api/projects/proj_async_1/analytics/tension') return respondJson({});
    if (u === '/api/projects/proj_async_1/checkpoint?latest=true') return respondJson(null);

    if (u === '/api/projects/proj_async_1/bootstrap-status') {
      pollCount++;
      if (pollCount === 1) {
        return respondJson({
          project_id: 'proj_async_1',
          status: 'running',
          stage: '正在梳理世界公理与战力阶梯...',
        });
      }
      return respondJson({
        project_id: 'proj_async_1',
        status: 'completed',
        stage: '创世总纲推演完成',
      });
    }

    return respondJson({ error: `unhandled ${method} ${u}` }, 404);
  };

  // 1. 初始化作品列表并切换
  state.projects = [projBoot];
  state.selectedProjectId = projBoot.id;
  state.currentProject = projBoot;

  // 2. 启动异步监视
  actions.startBootstrapMonitor('proj_async_1', '万界独尊');
  assert.equal(state.bootstrappingProjectId, 'proj_async_1');

  // 3. activeTasks 包含创世推演任务
  const tasks = computedState.activeTasks.value;
  assert.equal(tasks.some(t => t.id === 'bootstrap' && t.title.includes('万界独尊')), true);

  // 4. activeGenerationTask 返回创世推演（保护在途状态，防止中途切换）
  assert.equal(activeGenerationTask(state), '创世推演');

  // 5. 等待轮询定时器触发（第 1 次 running，第 2 次 completed）
  await new Promise(r => setTimeout(r, 4500));

  // 6. 验证推演完成后状态自动恢复
  assert.equal(state.bootstrappingProjectId, null);
  assert.equal(state.currentProject.framework?.theme_premise, '凡人逆天弑神');
});
