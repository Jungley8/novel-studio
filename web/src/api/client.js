// API Client for NovelStudio Go backend

async function streamFetch(endpoint, data, callbacks = {}) {
  const { onDelta, onComplete, onError } = callbacks;
  const url = endpoint.includes('?') ? `${endpoint}&stream=true` : `${endpoint}?stream=true`;
  const response = await fetch(url, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Accept': 'text/event-stream',
    },
    body: JSON.stringify(data),
  });

  if (!response.ok) {
    let errorMsg = `HTTP Error ${response.status}`;
    try {
      const errJson = await response.json();
      errorMsg = errJson.error || errorMsg;
    } catch {
      const text = await response.text();
      if (text) errorMsg = text;
    }
    const err = new Error(errorMsg);
    if (onError) onError(err);
    throw err;
  }

  if (response.body && typeof response.body.getReader === 'function') {
    const reader = response.body.getReader();
    const decoder = new TextDecoder();
    let buffer = '';
    let completedData = null;

    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      buffer += decoder.decode(value, { stream: true });

      const lines = buffer.split('\n');
      buffer = lines.pop() || '';

      let currentEvent = '';
      for (const line of lines) {
        const trimmed = line.trim();
        if (trimmed.startsWith('event:')) {
          currentEvent = trimmed.replace('event:', '').trim();
        } else if (trimmed.startsWith('data:')) {
          const dataStr = trimmed.replace('data:', '').trim();
          try {
            const parsed = JSON.parse(dataStr);
            if (currentEvent === 'delta') {
              if (onDelta && parsed.delta) {
                onDelta(parsed.delta);
              }
            } else if (currentEvent === 'complete' || currentEvent === 'done') {
              completedData = parsed;
              if (onComplete) onComplete(parsed);
            } else if (currentEvent === 'error') {
              const err = new Error(parsed.error || '流式处理异常');
              if (onError) onError(err);
              throw err;
            }
          } catch (parseErr) {
            if (currentEvent === 'error') throw parseErr;
          }
        }
      }
    }
    return completedData;
  }

  return response.json();
}

async function apiFetch(endpoint, options = {}) {
  const headers = {
    'Content-Type': 'application/json',
    ...(options.headers || {}),
  };

  const response = await fetch(endpoint, {
    ...options,
    headers,
  });

  if (!response.ok) {
    let errorMsg = `HTTP Error ${response.status}`;
    try {
      const errJson = await response.json();
      errorMsg = errJson.error || errorMsg;
    } catch {
      const text = await response.text();
      if (text) errorMsg = text;
    }
    throw new Error(errorMsg);
  }

  if (response.status === 204) {
    return null;
  }

  // Handle empty or text responses
  const contentType = response.headers.get('content-type');
  if (contentType && contentType.includes('application/json')) {
    return response.json();
  }
  return response.text();
}

export const api = {
  // Config
  getConfig: () => apiFetch('/api/config'),
  saveConfig: (data) => apiFetch('/api/config', { method: 'POST', body: JSON.stringify(data) }),
  testConfig: (data) => apiFetch('/api/config/test', { method: 'POST', body: JSON.stringify(data) }),

  // Projects
  listProjects: () => apiFetch('/api/projects'),
  getProject: (id) => apiFetch(`/api/projects/${id}`),
  createProject: (data) => apiFetch('/api/projects', { method: 'POST', body: JSON.stringify(data) }),
  updateProject: (data) => apiFetch('/api/projects', { method: 'POST', body: JSON.stringify(data) }),
  bootstrapProject: (data) => apiFetch('/api/projects/bootstrap', { method: 'POST', body: JSON.stringify(data) }),
  deleteProject: (id) => apiFetch(`/api/projects/${id}`, { method: 'DELETE' }),

  // Chapters & Hooks
  listChapters: (projectId) => apiFetch(`/api/projects/${projectId}/chapters`),
  commitChapter: (projectId, data) => apiFetch(`/api/projects/${projectId}/chapters`, { method: 'POST', body: JSON.stringify(data) }),
  uncommitChapter: (projectId, chapterIndex) => apiFetch(`/api/projects/${projectId}/chapters/${chapterIndex}/uncommit`, { method: 'POST' }),
  deleteChapter: (projectId, chapterIndex) => apiFetch(`/api/projects/${projectId}/chapters/${chapterIndex}`, { method: 'DELETE' }),
  listHooks: (projectId) => apiFetch(`/api/projects/${projectId}/hooks`),
  createHook: (projectId, data) => apiFetch(`/api/projects/${projectId}/hooks`, { method: 'POST', body: JSON.stringify(data) }),
  updateHook: (projectId, data) => apiFetch(`/api/projects/${projectId}/hooks`, { method: 'POST', body: JSON.stringify(data) }),
  deleteHook: (id) => apiFetch(`/api/hooks/${id}`, { method: 'DELETE' }),
  extractPlotHooks: (projectId) => apiFetch(`/api/projects/${projectId}/hooks/ai-extract`, { method: 'POST' }),

  // The Matrix
  getMatrixOverview: (projectId) => apiFetch(`/api/projects/${projectId}/matrix`),
  generateMatrixScenes: (projectId, data) => apiFetch(`/api/projects/${projectId}/matrix/ai-generate`, { method: 'POST', body: JSON.stringify(data) }),
  listScenes: (projectId, chapterId) => {
    const q = chapterId ? `?chapter_id=${encodeURIComponent(chapterId)}` : '';
    return apiFetch(`/api/projects/${projectId}/scenes${q}`);
  },
  createScene: (projectId, data) => apiFetch(`/api/projects/${projectId}/scenes`, { method: 'POST', body: JSON.stringify(data) }),
  getScene: (id) => apiFetch(`/api/scenes/${id}`),
  updateScene: (id, data) => apiFetch(`/api/scenes/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteScene: (id) => apiFetch(`/api/scenes/${id}`, { method: 'DELETE' }),
  listSceneMarkers: (sceneId) => apiFetch(`/api/scenes/${sceneId}/markers`),
  createSceneMarker: (sceneId, data) => apiFetch(`/api/scenes/${sceneId}/markers`, { method: 'POST', body: JSON.stringify(data) }),
  deleteSceneMarker: (sceneId, markerId) => apiFetch(`/api/scenes/${sceneId}/markers/${markerId}`, { method: 'DELETE' }),

  // The Codex
  listCodexEntries: (projectId, category = '') => {
    const q = category && category !== 'ALL' ? `?category=${encodeURIComponent(category)}` : '';
    return apiFetch(`/api/projects/${projectId}/codex${q}`);
  },
  getCodexEntry: (projectId, id) => apiFetch(`/api/projects/${projectId}/codex/${id}`),
  createCodexEntry: (projectId, data) => apiFetch(`/api/projects/${projectId}/codex`, { method: 'POST', body: JSON.stringify(data) }),
  generateCodexEntry: (projectId, data) => apiFetch(`/api/projects/${projectId}/codex/ai-generate`, { method: 'POST', body: JSON.stringify(data) }),
  updateCodexEntry: (projectId, id, data) => apiFetch(`/api/projects/${projectId}/codex/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteCodexEntry: (projectId, id) => apiFetch(`/api/projects/${projectId}/codex/${id}`, { method: 'DELETE' }),
  createCodexProgression: (projectId, entryId, data) => apiFetch(`/api/projects/${projectId}/codex/${entryId}/progressions`, { method: 'POST', body: JSON.stringify(data) }),
  listCodexRelations: (projectId, entryId = '') => {
    const q = entryId ? `?entry_id=${encodeURIComponent(entryId)}` : '';
    return apiFetch(`/api/projects/${projectId}/codex/relations${q}`);
  },
  createCodexRelation: (projectId, data) => apiFetch(`/api/projects/${projectId}/codex/relations`, { method: 'POST', body: JSON.stringify(data) }),
  deleteCodexRelation: (projectId, id) => apiFetch(`/api/projects/${projectId}/codex/relations/${id}`, { method: 'DELETE' }),
  extractCodexRelations: (projectId) => apiFetch(`/api/projects/${projectId}/codex/relations/ai-extract`, { method: 'POST' }),
  scanCodex: (projectId, text) => apiFetch(`/api/projects/${projectId}/codex/scan`, { method: 'POST', body: JSON.stringify({ text }) }),

  // State Machine
  getStateMachine: (projectId) => apiFetch(`/api/projects/${projectId}/statemachine`),
  saveStateMachine: (projectId, data) => apiFetch(`/api/projects/${projectId}/statemachine`, { method: 'POST', body: JSON.stringify(data) }),
  analyzeStateMachine: (projectId) => apiFetch(`/api/projects/${projectId}/statemachine/ai-analyze`, { method: 'POST' }),

  // Manuscript & Workshop Actions
  suggestConflict: (projectId, data) => apiFetch(`/api/projects/${projectId}/suggest-conflict`, { method: 'POST', body: JSON.stringify(data) }),
  deriveBeats: (projectId, data) => apiFetch(`/api/projects/${projectId}/derive-beats`, { method: 'POST', body: JSON.stringify(data) }),
  renderScene: (projectId, data) => apiFetch(`/api/projects/${projectId}/render-scene`, { method: 'POST', body: JSON.stringify(data) }),
  renderSceneStream: (projectId, data, callbacks) => streamFetch(`/api/projects/${projectId}/render-scene`, data, callbacks),
  reviewDraft: (projectId, data) => apiFetch(`/api/projects/${projectId}/review-draft`, { method: 'POST', body: JSON.stringify(data) }),
  rewriteDraft: (projectId, data) => apiFetch(`/api/projects/${projectId}/rewrite-draft`, { method: 'POST', body: JSON.stringify(data) }),
  rewriteDraftStream: (projectId, data, callbacks) => streamFetch(`/api/projects/${projectId}/rewrite-draft`, data, callbacks),
  getCheckpoint: (projectId, chapterIndex) => apiFetch(`/api/projects/${projectId}/checkpoint?chapter_index=${chapterIndex}`),
  saveCheckpoint: (projectId, data) => apiFetch(`/api/projects/${projectId}/checkpoint`, { method: 'POST', body: JSON.stringify(data) }),
  clearCheckpoint: (projectId, chapterIndex) => apiFetch(`/api/projects/${projectId}/checkpoint?chapter_index=${chapterIndex}`, { method: 'DELETE' }),
  produceAutonomous: (projectId, data) => apiFetch(`/api/projects/${projectId}/workshop/produce`, { method: 'POST', body: JSON.stringify(data) }),
  inlineAction: (data) => apiFetch('/api/workshop/inline-action', { method: 'POST', body: JSON.stringify(data) }),
  promptPreview: (projectId, data) => apiFetch(`/api/projects/${projectId}/prompt-preview`, { method: 'POST', body: JSON.stringify(data) }),
  workshopChat: (projectId, data) => apiFetch(`/api/projects/${projectId}/chat`, { method: 'POST', body: JSON.stringify(data) }),

  // Genesis Framework
  getFramework: (projectId) => apiFetch(`/api/projects/${projectId}/framework`),
  updateFramework: (projectId, data) => apiFetch(`/api/projects/${projectId}/framework`, { method: 'PUT', body: JSON.stringify(data) }),
  bootstrapFramework: (projectId, concept) => apiFetch(`/api/projects/${projectId}/framework/bootstrap`, { method: 'POST', body: JSON.stringify({ concept }) }),

  // Analytics & Quality Bench
  getAnalyticsHeatmap: (projectId) => apiFetch(`/api/projects/${projectId}/analytics/heatmap`),
  getAnalyticsTension: (projectId) => apiFetch(`/api/projects/${projectId}/analytics/tension`),
  getBenchReport: (projectId) => apiFetch(`/api/projects/${projectId}/bench`),

  // Activity Ledger (内容生产与变更时间线)
  listActivity: (projectId, params = {}) => {
    const q = new URLSearchParams();
    for (const [k, v] of Object.entries(params)) {
      if (v !== undefined && v !== null && v !== '') q.set(k, v);
    }
    const qs = q.toString();
    return apiFetch(`/api/projects/${projectId}/activity${qs ? `?${qs}` : ''}`);
  },
  getActivityEvent: (projectId, eventId) => apiFetch(`/api/projects/${projectId}/activity/${encodeURIComponent(eventId)}`),

  // Linter & Anti-AI Harmonizer
  lintAnalyze: (text) => apiFetch('/api/linter/analyze', { method: 'POST', body: JSON.stringify({ text }) }),
  harmonize: (projectId, data) => apiFetch(`/api/projects/${projectId}/harmonize`, { method: 'POST', body: JSON.stringify(data) }),
  suggestHumanTouches: (projectId, data) => apiFetch(`/api/projects/${projectId}/suggest-human-touches`, { method: 'POST', body: JSON.stringify(data) }),
  sanitizeAI: (projectId, data) => apiFetch(`/api/projects/${projectId}/sanitize-ai`, { method: 'POST', body: JSON.stringify(data) }),

  // Studio
  listStudioLibrary: () => apiFetch('/api/studio/library'),
  listStudioThreads: () => apiFetch('/api/studio/threads'),
  getStudioThread: (id) => apiFetch(`/api/studio/threads/${id}`),
  deleteStudioThread: (id) => apiFetch(`/api/studio/threads/${id}`, { method: 'DELETE' }),
  studioChat: (data) => apiFetch('/api/studio/chat', { method: 'POST', body: JSON.stringify(data) }),

  // Forecast (剧情树)
  listForecasts: (projectId) => apiFetch(`/api/projects/${projectId}/forecast`),
  createForecast: (projectId, data) => apiFetch(`/api/projects/${projectId}/forecast`, { method: 'POST', body: JSON.stringify(data) }),
  getForecast: (projectId, fid) => apiFetch(`/api/projects/${projectId}/forecast/${fid}`),
  selectForecast: (projectId, fid, data) => apiFetch(`/api/projects/${projectId}/forecast/${fid}/select`, { method: 'POST', body: JSON.stringify(typeof data === 'string' ? { branch_id: data } : data) }),
  getActivePlan: (projectId) => apiFetch(`/api/projects/${projectId}/forecast/active-plan`),

  // Chapter Intents (章节意图工坊)
  listChapterIntents: (projectId, volumeIndex) => {
    const q = (volumeIndex !== undefined && volumeIndex !== null && volumeIndex > 0) ? `?volume=${encodeURIComponent(volumeIndex)}` : '';
    return apiFetch(`/api/projects/${projectId}/intents${q}`);
  },
  generateVolumeIntents: (projectId, data) => apiFetch(`/api/projects/${projectId}/intents/generate`, { method: 'POST', body: JSON.stringify(data) }),
  saveChapterIntent: (projectId, chapterIndex, data) => apiFetch(`/api/projects/${projectId}/intents/${chapterIndex}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteChapterIntent: (projectId, chapterIndex) => apiFetch(`/api/projects/${projectId}/intents/${chapterIndex}`, { method: 'DELETE' }),
  generateMatrixFromIntents: (projectId, data) => apiFetch(`/api/projects/${projectId}/matrix/generate-from-intents`, { method: 'POST', body: JSON.stringify(data) }),

  // Studio Todos & Pipeline
  listStudioTodos: () => apiFetch('/api/studio/todos'),
  startPipeline: (data, handlers) => startPipelineStream('/api/pipeline', data, handlers),

  // Studio Automation & Queue
  listAutomations: () => apiFetch('/api/studio/automation'),
  saveAutomation: (projectId, data) => apiFetch(`/api/studio/automation/${projectId}`, { method: 'PUT', body: JSON.stringify(data) }),
  resumeAutomation: (projectId) => apiFetch(`/api/studio/automation/${projectId}/resume`, { method: 'POST' }),
  listQueue: () => apiFetch('/api/studio/queue'),
  createQueueItem: (data) => apiFetch('/api/studio/queue', { method: 'POST', body: JSON.stringify(data) }),
  updateQueueItem: (id, data) => apiFetch(`/api/studio/queue/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteQueueItem: (id) => apiFetch(`/api/studio/queue/${id}`, { method: 'DELETE' }),
};

async function startPipelineStream(endpoint, data, handlers = {}) {
  const { onEvent, onResult, onError } = handlers || {};
  const url = endpoint.includes('?') ? `${endpoint}&stream=true` : `${endpoint}?stream=true`;
  const response = await fetch(url, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Accept': 'text/event-stream',
    },
    body: JSON.stringify(data),
  });

  if (!response.ok) {
    let errorMsg = `HTTP Error ${response.status}`;
    try {
      const errJson = await response.json();
      errorMsg = errJson.error || errorMsg;
    } catch {
      const text = await response.text();
      if (text) errorMsg = text;
    }
    const err = new Error(errorMsg);
    if (onError) onError(err);
    throw err;
  }

  if (response.body && typeof response.body.getReader === 'function') {
    const reader = response.body.getReader();
    const decoder = new TextDecoder();
    let buffer = '';
    let finished = false;
    let resultData = null;

    try {
      while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        buffer += decoder.decode(value, { stream: true });

        const lines = buffer.split('\n');
        buffer = lines.pop() || '';

        for (const line of lines) {
          const trimmed = line.trim();
          if (!trimmed || trimmed.startsWith(':')) {
            continue;
          }
          if (trimmed.startsWith('data:')) {
            const dataStr = trimmed.slice(5).trim();
            if (!dataStr) continue;
            try {
              const parsed = JSON.parse(dataStr);
              if (parsed.phase === 'PIPELINE_ERROR') {
                finished = true;
                const err = new Error(parsed.error || '流水线执行异常');
                if (onError) onError(err);
                try {
                  await reader.cancel();
                } catch (_) {}
                return;
              } else if (parsed.phase === 'PIPELINE_RESULT') {
                if (!finished) {
                  resultData = parsed.result;
                  if (onResult) onResult(parsed.result);
                }
              } else {
                if (!finished && onEvent) {
                  onEvent(parsed);
                }
              }
            } catch (_) {}
          }
        }
      }

      if (buffer.trim()) {
        const trimmed = buffer.trim();
        if (!trimmed.startsWith(':') && trimmed.startsWith('data:')) {
          const dataStr = trimmed.slice(5).trim();
          if (dataStr) {
            try {
              const parsed = JSON.parse(dataStr);
              if (parsed.phase === 'PIPELINE_ERROR') {
                if (!finished) {
                  finished = true;
                  const err = new Error(parsed.error || '流水线执行异常');
                  if (onError) onError(err);
                  return;
                }
              } else if (parsed.phase === 'PIPELINE_RESULT') {
                if (!finished) {
                  resultData = parsed.result;
                  if (onResult) onResult(parsed.result);
                }
              } else {
                if (!finished && onEvent) {
                  onEvent(parsed);
                }
              }
            } catch (_) {}
          }
        }
      }
    } catch (streamErr) {
      if (!finished) {
        finished = true;
        if (onError) onError(streamErr);
        throw streamErr;
      }
    }

    return resultData;
  }

  const result = await response.json();
  if (handlers?.onResult) handlers.onResult(result);
  return result;
}
