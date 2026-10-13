<template>
  <div class="flex-1 flex flex-col h-full overflow-hidden bg-atelier-950">
    <!-- 头部工具栏 -->
    <div class="px-6 py-3 border-b border-atelier-750 bg-atelier-900/40 shrink-0 select-none">
      <div class="flex items-center gap-3">
        <div class="w-8 h-8 rounded-lg bg-brand-amber/10 border border-brand-amber/25 flex items-center justify-center text-brand-amber shadow-amber-glow">
          <History class="w-4 h-4" />
        </div>
        <div class="min-w-0">
          <div class="flex items-center gap-2">
            <h2 class="text-sm font-serif font-bold text-ink-50 tracking-wide">动态时间线</h2>
            <span class="text-[10px] font-mono text-ink-400 bg-atelier-850 px-2 py-0.5 rounded border border-atelier-750">
              {{ state.currentProject?.title || '未命名作品' }}
            </span>
          </div>
          <p class="text-[11px] text-ink-400">内容生产与变更全留痕：正文、伏笔、百科、断点皆可回看全文详情。</p>
        </div>
      </div>

      <!-- 筛选条 -->
      <div class="flex flex-wrap items-center gap-2 mt-3">
        <select v-model="filters.entity_type" @change="reload"
          class="bg-atelier-850 border border-atelier-700 rounded-md px-2 py-1.5 text-xs text-ink-100 focus:outline-none focus:border-brand-amber/60 cursor-pointer">
          <option value="">全部实体</option>
          <option value="chapter">章节</option>
          <option value="hook">伏笔</option>
          <option value="codex_entry">百科条目</option>
          <option value="codex_progression">百科演进</option>
          <option value="codex_relation">百科关系</option>
          <option value="scene">场次</option>
          <option value="scene_marker">场次批注</option>
          <option value="checkpoint">断点</option>
          <option value="workshop">流水线</option>
          <option value="project">作品</option>
          <option value="framework">总纲</option>
        </select>
        <input v-model.number="filters.chapter_index" @change="reload" type="number" min="1" placeholder="章节号"
          class="w-24 bg-atelier-850 border border-atelier-700 rounded-md px-2 py-1.5 text-xs text-ink-100 placeholder:text-ink-500 focus:outline-none focus:border-brand-amber/60" />
        <label class="flex items-center gap-1.5 text-xs text-ink-300 cursor-pointer select-none">
          <input type="checkbox" v-model="filters.include_checkpoint" @change="reload" class="accent-amber-500" />
          <span>显示中间断点</span>
        </label>
        <button @click="reload" :disabled="loading"
          class="px-3 py-1.5 bg-atelier-850 hover:bg-atelier-800 text-ink-200 text-xs rounded-md border border-atelier-750 transition cursor-pointer disabled:opacity-50">
          {{ loading ? '加载中…' : '刷新' }}
        </button>
        <span class="text-[11px] font-mono text-ink-500 ml-auto">{{ groups.length }} 组 · {{ events.length }} 条</span>
      </div>
    </div>

    <!-- 时间线主体 -->
    <div class="flex-1 overflow-y-auto p-6 min-h-0">
      <div v-if="!state.currentProject" class="text-center text-xs text-ink-400 py-16">请先选择或新建一部作品。</div>
      <div v-else-if="loading && events.length === 0" class="text-center text-xs text-ink-400 py-16">正在拉取时间线…</div>
      <div v-else-if="events.length === 0" class="text-center text-xs text-ink-400 py-16">暂无动态。在工作台生产章节或编辑内容后，这里会出现完整留痕。</div>

      <div v-else class="max-w-3xl mx-auto space-y-3">
        <div v-for="g in groups" :key="g.key" class="rounded-lg border border-atelier-750 bg-atelier-900/40 overflow-hidden">
          <!-- run 折叠头（单条直接渲染） -->
          <button v-if="g.run" @click="toggleRun(g.key)"
            class="w-full flex items-center gap-2 px-4 py-2.5 text-left hover:bg-atelier-850/60 transition cursor-pointer">
            <ChevronDown :class="['w-3.5 h-3.5 text-ink-400 transition-transform', openRuns.has(g.key) ? '' : '-rotate-90']" />
            <span class="text-[10px] font-mono px-1.5 py-0.5 rounded bg-brand-amber/10 text-brand-amber border border-brand-amber/25">RUN</span>
            <span class="text-xs font-medium text-ink-100 truncate">{{ g.title }}</span>
            <span class="text-[10px] font-mono text-ink-500 ml-auto shrink-0">{{ g.items.length }} 步</span>
          </button>

          <div v-show="!g.run || openRuns.has(g.key)" class="divide-y divide-atelier-750/60">
            <div v-for="ev in g.items" :key="ev.id" class="px-4 py-3">
              <div class="flex items-start gap-3">
                <span :class="['mt-1 w-2 h-2 rounded-full shrink-0', dotClass(ev.entity_type)]" />
                <div class="min-w-0 flex-1">
                  <div class="flex items-baseline gap-2 flex-wrap">
                    <span class="text-xs font-semibold text-ink-100">{{ ev.title }}</span>
                    <span class="text-[10px] font-mono text-ink-500">{{ formatTime(ev.created_at) }}</span>
                  </div>
                  <p v-if="ev.summary" class="text-[11px] text-ink-400 mt-0.5 leading-relaxed">{{ ev.summary }}</p>
                  <div class="flex items-center gap-1.5 mt-1.5 flex-wrap">
                    <span class="text-[10px] font-mono px-1.5 py-0.5 rounded bg-atelier-850 text-ink-300 border border-atelier-700">{{ actorLabel(ev.actor) }}</span>
                    <span class="text-[10px] font-mono px-1.5 py-0.5 rounded bg-atelier-850 text-ink-300 border border-atelier-700">{{ sourceLabel(ev.source) }}</span>
                    <span v-if="ev.chapter_index" class="text-[10px] font-mono px-1.5 py-0.5 rounded bg-atelier-850 text-ink-300 border border-atelier-700">第{{ ev.chapter_index }}章</span>
                    <span v-if="ev.chars_added || ev.chars_removed" class="text-[10px] font-mono text-emerald-400/90">+{{ ev.chars_added }}/-{{ ev.chars_removed }}字</span>
                    <button @click="toggleDetail(ev)"
                      class="text-[11px] text-brand-amber hover:text-brand-amber-hover ml-auto cursor-pointer">
                      {{ expandedId === ev.id ? '收起详情' : '查看全文详情' }}
                    </button>
                  </div>
                  <p v-if="ev.excerpt_after && expandedId !== ev.id" class="text-[11px] text-ink-500 mt-1.5 line-clamp-2 leading-relaxed">{{ ev.excerpt_after }}</p>

                  <!-- 详情展开区 -->
                  <div v-if="expandedId === ev.id" class="mt-2 rounded-md border border-atelier-700 bg-atelier-950/70 p-3">
                    <div v-if="detailLoading" class="text-[11px] text-ink-400">正在解码全文…</div>
                    <div v-else-if="detail">
                      <div v-if="detail.fields_changed?.length" class="flex gap-1.5 flex-wrap mb-2">
                        <span v-for="f in detail.fields_changed" :key="f"
                          class="text-[10px] font-mono px-1.5 py-0.5 rounded bg-brand-amber/10 text-brand-amber border border-brand-amber/25">{{ f }}</span>
                      </div>
                      <div class="flex gap-1.5 mb-2">
                        <button @click="detailTab = 'after'"
                          :class="['px-2 py-1 text-[11px] rounded transition cursor-pointer', detailTab === 'after' ? 'bg-brand-amber text-atelier-950 font-bold' : 'bg-atelier-850 text-ink-300']">变更后全文</button>
                        <button v-if="detail.before" @click="detailTab = 'before'"
                          :class="['px-2 py-1 text-[11px] rounded transition cursor-pointer', detailTab === 'before' ? 'bg-brand-amber text-atelier-950 font-bold' : 'bg-atelier-850 text-ink-300']">变更前</button>
                        <span v-if="detail.token_usage" class="text-[10px] font-mono text-ink-500 self-center ml-auto">{{ tokenLabel(detail.token_usage) }}</span>
                      </div>
                      <pre class="text-[11px] text-ink-200 whitespace-pre-wrap leading-relaxed max-h-96 overflow-y-auto font-sans">{{ detailTab === 'after' ? (detail.after || '(空)') : detail.before }}</pre>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>

        <button v-if="nextCursor" @click="loadMore" :disabled="loading"
          class="w-full py-2 text-xs text-ink-300 bg-atelier-850 hover:bg-atelier-800 rounded-md border border-atelier-750 transition cursor-pointer disabled:opacity-50">
          {{ loading ? '加载中…' : '加载更早动态' }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, computed, watch, onMounted } from 'vue';
import { state } from '../stores/appState';
import { api } from '../api/client';
import { History, ChevronDown } from 'lucide-vue-next';

const loading = ref(false);
const events = ref([]);
const nextCursor = ref('');
const expandedId = ref(null);
const detail = ref(null);
const detailLoading = ref(false);
const detailTab = ref('after');

const filters = reactive({ entity_type: '', chapter_index: '', include_checkpoint: true });
const openRuns = ref(new Set());

function toggleRun(key) {
  if (openRuns.value.has(key)) openRuns.value.delete(key);
  else openRuns.value.add(key);
}

function hasDiffInfo(ev) {
  if (!ev) return false;
  return Boolean(
    (ev.fields_changed && ev.fields_changed.length > 0) ||
    ev.excerpt_after ||
    ev.excerpt_before ||
    ev.chars_added ||
    ev.chars_removed
  );
}

function areCompanionEvents(a, b) {
  if (!a || !b || a.id === b.id) return false;
  
  // 必须作用于同一作品、同实体/同章节
  const sameChapter = a.chapter_index === b.chapter_index;
  const sameEntity = (a.entity_type === b.entity_type && a.entity_id === b.entity_id) ||
    (a.entity_type === 'chapter' && b.entity_type === 'workshop') ||
    (a.entity_type === 'workshop' && b.entity_type === 'chapter') ||
    (a.entity_type === 'framework' && b.entity_type === 'framework') ||
    (a.entity_type === 'project' && b.entity_type === 'project');

  if (!sameChapter || !sameEntity) return false;

  // 如果同属一个 run_id，或者是 10 秒以内的相邻操作
  const sameRun = Boolean(a.run_id && b.run_id && a.run_id === b.run_id);
  const timeDiff = Math.abs(new Date(a.created_at).getTime() - new Date(b.created_at).getTime());
  const closeInTime = !isNaN(timeDiff) && timeDiff < 10000;

  if (!sameRun && !closeInTime) return false;

  // 必须一个是带 diff 的底层事件，另一个是无 diff 的聚合/伴随事件
  const aDiff = hasDiffInfo(a);
  const bDiff = hasDiffInfo(b);
  return (aDiff && !bDiff) || (!aDiff && bDiff);
}

function mergeCompanion(primary, companion) {
  const merged = { ...primary };
  if (!merged.token_usage && companion.token_usage) {
    merged.token_usage = companion.token_usage;
  }
  if (!merged.summary && companion.summary) {
    merged.summary = companion.summary;
  } else if (companion.summary && !merged.summary.includes(companion.summary)) {
    // 聚合补充更具业务语义的摘要
    merged.summary = `${companion.summary} · ${merged.summary}`;
  }
  return merged;
}

function deduplicateEventList(list) {
  if (!list || list.length <= 1) return list || [];
  const result = [];
  const mergedIds = new Set();

  for (let i = 0; i < list.length; i++) {
    const cur = list[i];
    if (mergedIds.has(cur.id)) continue;

    let mergedItem = cur;
    for (let j = i + 1; j < list.length; j++) {
      const candidate = list[j];
      if (mergedIds.has(candidate.id)) continue;

      if (areCompanionEvents(cur, candidate)) {
        mergedIds.add(candidate.id);
        const curHasDiff = hasDiffInfo(cur);
        const primary = curHasDiff ? cur : candidate;
        const comp = curHasDiff ? candidate : cur;
        mergedItem = mergeCompanion(primary, comp);
        break;
      }
    }
    result.push(mergedItem);
  }
  return result;
}

// 按 run_id 折叠：同 run 的多步事件收成一组，散事件保持平铺，自动去重伴随双写事件。
const groups = computed(() => {
  const runs = new Map();
  const rawOut = [];
  for (const ev of events.value) {
    if (ev.run_id) {
      if (!runs.has(ev.run_id)) {
        const g = { key: ev.run_id, run: true, title: runTitle(ev), items: [] };
        runs.set(ev.run_id, g);
        rawOut.push(g);
      }
      runs.get(ev.run_id).items.push(ev);
    } else {
      rawOut.push({ key: ev.id, run: false, items: [ev] });
    }
  }

  // 1. 先对每个 run 组内进行智能去重合并
  for (const g of rawOut) {
    g.items = deduplicateEventList(g.items);
    if (g.run && g.items.length) {
      const first = g.items[0];
      g.title = first.chapter_index ? `第${first.chapter_index}章 · 一次生产` : '一次批量操作';
    }
  }

  // 2. 对散事件进行跨条去重（如创世推演后保存作品等散事件）
  const finalOut = [];
  const mergedScatterIds = new Set();
  for (let i = 0; i < rawOut.length; i++) {
    const g = rawOut[i];
    if (g.run) {
      if (g.items.length > 0) finalOut.push(g);
      continue;
    }
    const curEv = g.items[0];
    if (mergedScatterIds.has(curEv.id)) continue;

    for (let j = i + 1; j < rawOut.length; j++) {
      const nextG = rawOut[j];
      if (nextG.run) continue;
      const nextEv = nextG.items[0];
      if (mergedScatterIds.has(nextEv.id)) continue;

      if (areCompanionEvents(curEv, nextEv)) {
        mergedScatterIds.add(nextEv.id);
        const curHasDiff = hasDiffInfo(curEv);
        const primary = curHasDiff ? curEv : nextEv;
        const comp = curHasDiff ? nextEv : curEv;
        g.items = [mergeCompanion(primary, comp)];
        break;
      }
    }
    finalOut.push(g);
  }

  return finalOut;
});

function runTitle(ev) {
  if (!ev) return '一次批量操作';
  return ev.chapter_index ? `第${ev.chapter_index}章 · 一次生产` : '一次批量操作';
}

function dotClass(entityType) {
  switch (entityType) {
    case 'chapter': return 'bg-brand-amber';
    case 'hook': return 'bg-violet-400';
    case 'codex_entry':
    case 'codex_progression':
    case 'codex_relation': return 'bg-emerald-400';
    case 'scene':
    case 'scene_marker': return 'bg-sky-400';
    case 'checkpoint': return 'bg-ink-500';
    case 'workshop': return 'bg-orange-400';
    case 'framework': return 'bg-fuchsia-400';
    default: return 'bg-ink-400';
  }
}

function actorLabel(a) {
  return { user: '人工', ai_writer: 'AI执笔', ai_reviewer: 'AI主编', ai_rewriter: 'AI返工', ai_reasoner: 'AI构思', system: '系统' }[a] || a || '未知';
}

function sourceLabel(s) {
  return {
    ui_manual: '手动', auto_pipeline: '自动流水线', cli_produce: '命令行生产', cli_genesis: '命令行创世',
    bootstrap: '创世推演', inline_action: '划词改写', ai_extract: 'AI提取', ai_analyze: 'AI分析',
    ai_generate: 'AI生成', ai_chat: '情境对话', direct: '直接写入', import: '导入',
  }[s] || s || '';
}

function formatTime(iso) {  if (!iso) return '';
  const d = new Date(iso);
  return `${d.getMonth() + 1}/${d.getDate()} ${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`;
}

function tokenLabel(raw) {
  try {
    const u = JSON.parse(raw);
    if (u && typeof u.total_tokens === 'number') {
      return `tokens P${u.prompt_tokens || 0}/C${u.completion_tokens || 0}/T${u.total_tokens}`;
    }
  } catch { /* 非 JSON 直接原文展示 */ }
  return `tokens: ${raw}`;
}

function queryParams(cursor) {
  const p = {};
  if (filters.entity_type) p.entity_type = filters.entity_type;
  if (filters.chapter_index) p.chapter_index = filters.chapter_index;
  if (!filters.include_checkpoint) p.include_checkpoint = 'false';
  if (cursor) p.cursor = cursor;
  p.limit = 50;
  return p;
}

async function reload() {
  if (!state.currentProject) return;
  loading.value = true;
  expandedId.value = null;
  detail.value = null;
  try {
    const res = await api.listActivity(state.currentProject.id, queryParams(''));
    events.value = res.events || [];
    nextCursor.value = res.next_cursor || '';
  } catch (e) {
    console.error('[Activity] reload failed', e);
  } finally {
    loading.value = false;
  }
}

async function loadMore() {
  if (!state.currentProject || !nextCursor.value) return;
  loading.value = true;
  try {
    const res = await api.listActivity(state.currentProject.id, queryParams(nextCursor.value));
    events.value.push(...(res.events || []));
    nextCursor.value = res.next_cursor || '';
  } catch (e) {
    console.error('[Activity] loadMore failed', e);
  } finally {
    loading.value = false;
  }
}

async function toggleDetail(ev) {
  if (expandedId.value === ev.id) {
    expandedId.value = null;
    detail.value = null;
    return;
  }
  expandedId.value = ev.id;
  detailTab.value = 'after';
  detail.value = null;
  detailLoading.value = true;
  try {
    detail.value = await api.getActivityEvent(state.currentProject.id, ev.id);
  } catch (e) {
    console.error('[Activity] detail failed', e);
  } finally {
    detailLoading.value = false;
  }
}

watch(() => state.selectedProjectId, () => {
  // 清空作品级过滤与展开态，防止旧作品的筛选条件过滤新作品时间线
  filters.entity_type = '';
  filters.chapter_index = '';
  openRuns.value = new Set();
  if (state.activeTab === 'activity') reload();
});
watch(() => state.activeTab, (tab) => { if (tab === 'activity') reload(); });
onMounted(() => { if (state.activeTab === 'activity' && state.currentProject) reload(); });
</script>
