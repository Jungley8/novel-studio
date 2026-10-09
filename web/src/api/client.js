// API Client for NovelStudio Go backend

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
  analyzeStateMachine: (projectId) => apiFetch(`/api/projects/${projectId}/statemachine/ai-analyze`, { method: 'POST' }),

  // Manuscript & Workshop Actions
  suggestConflict: (projectId, data) => apiFetch(`/api/projects/${projectId}/suggest-conflict`, { method: 'POST', body: JSON.stringify(data) }),
  deriveBeats: (projectId, data) => apiFetch(`/api/projects/${projectId}/derive-beats`, { method: 'POST', body: JSON.stringify(data) }),
  renderScene: (projectId, data) => apiFetch(`/api/projects/${projectId}/render-scene`, { method: 'POST', body: JSON.stringify(data) }),
  reviewDraft: (projectId, data) => apiFetch(`/api/projects/${projectId}/review-draft`, { method: 'POST', body: JSON.stringify(data) }),
  rewriteDraft: (projectId, data) => apiFetch(`/api/projects/${projectId}/rewrite-draft`, { method: 'POST', body: JSON.stringify(data) }),
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

  // Analytics
  getAnalyticsHeatmap: (projectId) => apiFetch(`/api/projects/${projectId}/analytics/heatmap`),
  getAnalyticsTension: (projectId) => apiFetch(`/api/projects/${projectId}/analytics/tension`),

  // Linter & Anti-AI Harmonizer
  lintAnalyze: (text) => apiFetch('/api/linter/analyze', { method: 'POST', body: JSON.stringify({ text }) }),
  harmonize: (projectId, data) => apiFetch(`/api/projects/${projectId}/harmonize`, { method: 'POST', body: JSON.stringify(data) }),
  suggestHumanTouches: (projectId, data) => apiFetch(`/api/projects/${projectId}/suggest-human-touches`, { method: 'POST', body: JSON.stringify(data) }),
  sanitizeAI: (projectId, data) => apiFetch(`/api/projects/${projectId}/sanitize-ai`, { method: 'POST', body: JSON.stringify(data) }),
};
