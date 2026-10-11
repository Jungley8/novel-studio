<template>
  <!-- 当尚未选择任何项目时的零态引导 -->
  <div v-if="!state.currentProject" class="flex-1 flex flex-col items-center justify-center p-8 bg-atelier-950 text-center select-none">
    <div class="max-w-md w-full p-8 rounded-2xl bg-atelier-900/60 border border-atelier-750 backdrop-blur-xl shadow-atelier-lg space-y-6">
      <div class="w-16 h-16 rounded-2xl bg-gradient-to-br from-brand-amber/20 to-amber-600/10 border border-brand-amber/30 mx-auto flex items-center justify-center text-brand-amber shadow-amber-glow">
        <Gauge class="w-8 h-8" />
      </div>
      <div class="space-y-2">
        <h3 class="text-xl font-bold font-serif text-ink-50 tracking-wide">
          质量指标
        </h3>
        <p class="text-xs text-ink-300 leading-relaxed">
          请先在左侧选择或新建一部小说作品，即可查看章节终态分布、达标率与返工走势。
        </p>
      </div>
    </div>
  </div>

  <!-- 主视图容器 -->
  <div v-else class="flex-1 p-6 overflow-y-auto space-y-6">
    <!-- 头部工具栏 -->
    <div class="flex flex-col md:flex-row md:items-center justify-between gap-4 border-b border-atelier-750 pb-5">
      <div>
        <div class="flex items-center gap-2.5">
          <div class="w-8 h-8 rounded-lg bg-brand-amber/10 border border-brand-amber/25 flex items-center justify-center text-brand-amber shadow-amber-glow">
            <Gauge class="w-4 h-4" />
          </div>
          <div>
            <h2 class="text-base font-serif font-bold text-ink-50 tracking-wide">质量指标</h2>
            <p class="text-xs text-ink-400 mt-0.5">章节终态、达标率与返工走势</p>
          </div>
        </div>
      </div>

      <div class="flex items-center gap-3">
        <button 
          @click="load" 
          :disabled="loading"
          class="flex items-center gap-1.5 px-3 py-1.5 bg-atelier-850 hover:bg-atelier-800 text-ink-300 text-xs rounded-md border border-atelier-750 transition cursor-pointer disabled:opacity-60">
          <RefreshCw class="w-3.5 h-3.5 text-ink-400" :class="{ 'animate-spin': loading }" />
          <span>刷新</span>
        </button>
      </div>
    </div>

    <!-- Loading 态 -->
    <div v-if="loading && !metricsData" class="py-20 text-center bg-atelier-900 border border-atelier-750 rounded-xl space-y-3 shadow-atelier-md">
      <RefreshCw class="w-6 h-6 animate-spin text-brand-amber mx-auto" />
      <h4 class="text-xs font-semibold text-ink-200">正在拉取质量指标数据...</h4>
      <p class="text-[11px] text-ink-400">读取章节质量评测、字数达成率与终态统计。</p>
    </div>

    <!-- Error 态 -->
    <div v-else-if="error" class="p-6 bg-rose-500/10 border border-rose-500/30 rounded-xl text-center space-y-3">
      <AlertCircle class="w-6 h-6 text-rose-400 mx-auto" />
      <div class="text-xs text-rose-200 font-semibold">{{ error }}</div>
      <button 
        @click="load" 
        class="px-3 py-1.5 bg-atelier-850 hover:bg-atelier-800 text-ink-200 text-xs rounded-md border border-atelier-750 transition cursor-pointer">
        重新加载
      </button>
    </div>

    <!-- 空态 -->
    <div 
      v-else-if="!metricsData || !metricsData.chapters || metricsData.chapters.length === 0" 
      class="text-center py-20 bg-atelier-900 border border-dashed border-atelier-750 rounded-xl space-y-3">
      <div class="w-12 h-12 rounded-full bg-atelier-800 flex items-center justify-center mx-auto text-ink-400">
        <Gauge class="w-6 h-6" />
      </div>
      <h3 class="text-sm font-semibold text-ink-200">暂无质量指标记录</h3>
      <p class="text-xs text-ink-400 max-w-md mx-auto">
        暂无质量指标记录——新生产的章节将自动落库
      </p>
    </div>

    <!-- 指标展示区 -->
    <div v-else class="space-y-6">
      <!-- 汇总卡片行 -->
      <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3.5">
        <!-- 终态分布 -->
        <div class="p-4 rounded-xl bg-atelier-900 border border-atelier-750 shadow-atelier-sm space-y-2">
          <div class="flex items-center justify-between text-xs text-ink-400">
            <span>终态分布</span>
            <span class="font-mono text-[11px] text-ink-500">共 {{ summary.chapters }} 章</span>
          </div>
          <div class="grid grid-cols-2 gap-1.5 text-xs font-mono pt-1">
            <div class="flex items-center justify-between px-2 py-1 rounded bg-emerald-500/10 border border-emerald-500/20 text-emerald-400">
              <span class="text-[11px]">已入库</span>
              <strong class="font-semibold">{{ summary.committed_chapters }}</strong>
            </div>
            <div class="flex items-center justify-between px-2 py-1 rounded bg-amber-500/10 border border-amber-500/20 text-amber-400">
              <span class="text-[11px]">停线待人工</span>
              <strong class="font-semibold">{{ summary.needs_human_chapters }}</strong>
            </div>
            <div class="flex items-center justify-between px-2 py-1 rounded bg-rose-500/10 border border-rose-500/20 text-rose-400">
              <span class="text-[11px]">技术阻断</span>
              <strong class="font-semibold">{{ summary.technical_blocked_chapters }}</strong>
            </div>
            <div class="flex items-center justify-between px-2 py-1 rounded bg-atelier-800/80 border border-atelier-700 text-ink-300">
              <span class="text-[11px]">未入库</span>
              <strong class="font-semibold">{{ summary.produced_chapters }}</strong>
            </div>
          </div>
        </div>

        <!-- 字数达标率 -->
        <div class="p-4 rounded-xl bg-atelier-900 border border-atelier-750 shadow-atelier-sm space-y-1.5">
          <div class="text-xs text-ink-400">字数达标率</div>
          <div class="flex items-baseline gap-2">
            <span class="text-2xl font-bold font-mono tracking-tight" :class="summary.word_pass_rate >= 0.8 ? 'text-emerald-400' : 'text-amber-400'">
              {{ (summary.word_pass_rate * 100).toFixed(1) }}%
            </span>
          </div>
          <div class="text-[11px] text-ink-400 flex items-center justify-between">
            <span>未达标章节</span>
            <span class="font-mono font-medium text-ink-200">{{ summary.under_target_chapters }} 章</span>
          </div>
        </div>

        <!-- 平均评审分 -->
        <div class="p-4 rounded-xl bg-atelier-900 border border-atelier-750 shadow-atelier-sm space-y-1.5">
          <div class="text-xs text-ink-400">平均评审分</div>
          <div class="flex items-baseline gap-2">
            <span class="text-2xl font-bold font-mono tracking-tight" :class="summary.avg_score >= 80 ? 'text-brand-amber' : 'text-ink-100'">
              {{ summary.avg_score.toFixed(1) }}
            </span>
            <span class="text-xs text-ink-500 font-mono">/ 100</span>
          </div>
          <div class="text-[11px] text-ink-400 flex items-center justify-between">
            <span>总评审章数</span>
            <span class="font-mono font-medium text-ink-200">{{ summary.chapters }} 章</span>
          </div>
        </div>

        <!-- 重写返工率 -->
        <div class="p-4 rounded-xl bg-atelier-900 border border-atelier-750 shadow-atelier-sm space-y-1.5">
          <div class="text-xs text-ink-400">重写返工率</div>
          <div class="flex items-baseline gap-2">
            <span class="text-2xl font-bold font-mono tracking-tight" :class="summary.rewrite_rate > 0.3 ? 'text-rose-400' : 'text-ink-100'">
              {{ (summary.rewrite_rate * 100).toFixed(1) }}%
            </span>
          </div>
          <div class="text-[11px] text-ink-400 flex items-center justify-between">
            <span>发生重写章节</span>
            <span class="font-mono font-medium text-ink-200">{{ summary.rewrite_chapters }} 章</span>
          </div>
        </div>

        <!-- 对账未闭合 -->
        <div class="p-4 rounded-xl bg-atelier-900 border border-atelier-750 shadow-atelier-sm space-y-1.5">
          <div class="text-xs text-ink-400">对账未闭合</div>
          <div class="flex items-baseline gap-2">
            <span class="text-2xl font-bold font-mono tracking-tight" :class="summary.unclosed_reconciliation_chapters > 0 ? 'text-amber-400' : 'text-emerald-400'">
              {{ summary.unclosed_reconciliation_chapters }}
            </span>
            <span class="text-xs text-ink-500">章</span>
          </div>
          <div class="text-[11px] text-ink-400">
            <span>存在未闭合事实观察</span>
          </div>
        </div>

        <!-- 伏笔通过率 -->
        <div class="p-4 rounded-xl bg-atelier-900 border border-atelier-750 shadow-atelier-sm space-y-1.5">
          <div class="text-xs text-ink-400">伏笔通过率</div>
          <div class="flex items-baseline gap-2">
            <span class="text-2xl font-bold font-mono tracking-tight text-ink-100">
              {{ summary.hook_claims_total > 0 ? (summary.hook_pass_rate * 100).toFixed(1) + '%' : '-' }}
            </span>
          </div>
          <div class="text-[11px] text-ink-400 flex items-center justify-between">
            <span>通过 / 声明</span>
            <span class="font-mono font-medium text-ink-200">{{ summary.hook_claims_validated }} / {{ summary.hook_claims_total }}</span>
          </div>
        </div>

        <!-- 压缩失败 -->
        <div class="p-4 rounded-xl bg-atelier-900 border border-atelier-750 shadow-atelier-sm space-y-1.5">
          <div class="text-xs text-ink-400">压缩失败</div>
          <div class="flex items-baseline gap-2">
            <span class="text-2xl font-bold font-mono tracking-tight" :class="summary.compaction_failed_chapters > 0 ? 'text-rose-400' : 'text-emerald-400'">
              {{ summary.compaction_failed_chapters }}
            </span>
            <span class="text-xs text-ink-500">章</span>
          </div>
          <div class="text-[11px] text-ink-400">
            <span>长上下文压制异常</span>
          </div>
        </div>
      </div>

      <!-- 章节表格 -->
      <div class="rounded-xl border border-atelier-750 bg-atelier-900 overflow-hidden shadow-atelier-md">
        <div class="px-5 py-3.5 border-b border-atelier-750 flex items-center justify-between">
          <h3 class="text-xs font-semibold text-ink-200">章节明细</h3>
          <span class="text-[11px] font-mono text-ink-500">共 {{ chapters.length }} 章</span>
        </div>

        <div class="overflow-x-auto">
          <table class="w-full text-left text-xs">
            <thead class="bg-atelier-850/60 border-b border-atelier-750 text-[11px] font-mono text-ink-400 uppercase">
              <tr>
                <th scope="col" class="py-3 px-4">章号</th>
                <th scope="col" class="py-3 px-4">终态</th>
                <th scope="col" class="py-3 px-4">字数 (实际/目标)</th>
                <th scope="col" class="py-3 px-4 text-center">评分</th>
                <th scope="col" class="py-3 px-4 text-center">重写轮</th>
                <th scope="col" class="py-3 px-4 text-center">对账未闭合</th>
                <th scope="col" class="py-3 px-4 text-center">伏笔 (通过/声明)</th>
                <th scope="col" class="py-3 px-4 text-center">压缩</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-atelier-750/50">
              <tr 
                v-for="ch in chapters" 
                :key="ch.chapter_index"
                class="hover:bg-atelier-850/40 transition">
                <!-- 章号 -->
                <td class="py-3 px-4 font-mono font-medium text-ink-100 whitespace-nowrap">
                  第 {{ ch.chapter_index }} 章
                </td>

                <!-- 终态 badge -->
                <td class="py-3 px-4 whitespace-nowrap">
                  <span 
                    :class="outcomeBadgeClass(ch.outcome)" 
                    class="inline-flex items-center px-2 py-0.5 rounded text-[11px] font-mono font-medium">
                    {{ outcomeLabel(ch.outcome) }}
                  </span>
                </td>

                <!-- 字数（实际/目标 + 比率） -->
                <td class="py-3 px-4 font-mono whitespace-nowrap">
                  <div class="flex items-center gap-2">
                    <span class="text-ink-100">{{ ch.actual_words }}</span>
                    <span class="text-ink-500">/</span>
                    <span class="text-ink-400">{{ ch.words_target }}</span>
                    <span 
                      :class="ch.under_target ? 'text-amber-400' : 'text-emerald-400'"
                      class="text-[10px] px-1.5 py-0.2 rounded bg-atelier-800">
                      {{ ((ch.word_quota_ratio || 0) * 100).toFixed(0) }}%
                    </span>
                  </div>
                </td>

                <!-- 评分 -->
                <td class="py-3 px-4 text-center font-mono whitespace-nowrap">
                  <span 
                    :class="scoreClass(ch.score)" 
                    class="font-semibold">
                    {{ ch.score }}
                  </span>
                </td>

                <!-- 重写轮 -->
                <td class="py-3 px-4 text-center font-mono whitespace-nowrap">
                  <span :class="ch.rewrite_loops > 0 ? 'text-amber-400 font-semibold' : 'text-ink-400'">
                    {{ ch.rewrite_loops }}
                  </span>
                </td>

                <!-- 对账未闭合 -->
                <td class="py-3 px-4 text-center font-mono whitespace-nowrap">
                  <span :class="ch.unresolved_observations > 0 ? 'text-amber-400 font-semibold' : 'text-ink-400'">
                    {{ ch.unresolved_observations }}
                  </span>
                </td>

                <!-- 伏笔（通过/声明） -->
                <td class="py-3 px-4 text-center font-mono whitespace-nowrap">
                  <span v-if="ch.hook_claims_total > 0" class="text-ink-200">
                    {{ ch.hook_claims_validated }} / {{ ch.hook_claims_total }}
                    <span v-if="ch.hook_claims_rejected > 0" class="text-[10px] text-rose-400 ml-1">
                      (拒{{ ch.hook_claims_rejected }})
                    </span>
                  </span>
                  <span v-else class="text-ink-500 text-[11px]">-</span>
                </td>

                <!-- 压缩 -->
                <td class="py-3 px-4 text-center font-mono whitespace-nowrap">
                  <span 
                    v-if="ch.compaction_failed" 
                    class="inline-flex items-center px-1.5 py-0.5 rounded text-[10px] bg-rose-500/10 text-rose-400 border border-rose-500/25">
                    失败
                  </span>
                  <span v-else class="text-ink-500 text-[11px]">正常</span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted } from 'vue';
import { state } from '../stores/appState';
import { api } from '../api/client';
import { Gauge, RefreshCw, AlertCircle } from 'lucide-vue-next';

const loading = ref(false);
const error = ref(null);
const metricsData = ref(null);

const summary = computed(() => metricsData.value?.summary || {
  chapters: 0,
  committed_chapters: 0,
  needs_human_chapters: 0,
  technical_blocked_chapters: 0,
  produced_chapters: 0,
  under_target_chapters: 0,
  rewrite_chapters: 0,
  unclosed_reconciliation_chapters: 0,
  compaction_failed_chapters: 0,
  word_pass_rate: 0,
  avg_score: 0,
  rewrite_rate: 0,
  hook_claims_total: 0,
  hook_claims_validated: 0,
  hook_claims_rejected: 0,
  hook_pass_rate: 0
});

const chapters = computed(() => metricsData.value?.chapters || []);

function outcomeLabel(outcome) {
  switch (outcome) {
  case 'committed':
    return '已入库';
  case 'needs_human':
    return '停线待人工';
  case 'technical_blocked':
    return '技术阻断';
  case 'produced':
    return '已产出未入库';
  default:
    return outcome || '未知';
  }
}

function outcomeBadgeClass(outcome) {
  switch (outcome) {
  case 'committed':
    return 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/30';
  case 'needs_human':
    return 'bg-amber-500/10 text-amber-400 border border-amber-500/30';
  case 'technical_blocked':
    return 'bg-rose-500/10 text-rose-400 border border-rose-500/30';
  case 'produced':
    return 'bg-ink-700/20 text-ink-300 border border-atelier-700';
  default:
    return 'bg-atelier-800 text-ink-400 border border-atelier-700';
  }
}

function scoreClass(score) {
  if (score >= 80) return 'text-emerald-400';
  if (score >= 60) return 'text-amber-400';
  return 'text-rose-400';
}

async function load() {
  if (!state.currentProject?.id) {
    metricsData.value = null;
    return;
  }
  loading.value = true;
  error.value = null;
  try {
    const res = await api.getProjectMetrics(state.currentProject.id);
    metricsData.value = res;
  } catch (err) {
    error.value = err.message || '获取质量指标失败';
    console.error('[MetricsView] load failed', err);
  } finally {
    loading.value = false;
  }
}

watch(
  () => state.currentProject?.id,
  (newId) => {
    if (newId) {
      load();
    } else {
      metricsData.value = null;
    }
  }
);

onMounted(() => {
  if (state.currentProject?.id) {
    load();
  }
});
</script>
