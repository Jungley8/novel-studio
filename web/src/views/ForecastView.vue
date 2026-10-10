<template>
  <!-- 当尚未选择任何项目时的零态引导 -->
  <div v-if="!state.currentProject" class="flex-1 flex flex-col items-center justify-center p-8 bg-atelier-950 text-center select-none">
    <div class="max-w-md w-full p-8 rounded-2xl bg-atelier-900/60 border border-atelier-750 backdrop-blur-xl shadow-atelier-lg space-y-6">
      <div class="w-16 h-16 rounded-2xl bg-gradient-to-br from-brand-amber/20 to-amber-600/10 border border-brand-amber/30 mx-auto flex items-center justify-center text-brand-amber shadow-amber-glow">
        <GitBranch class="w-8 h-8" />
      </div>
      <div class="space-y-2">
        <h3 class="text-xl font-bold font-serif text-ink-50 tracking-wide">
          剧情树推演
        </h3>
        <p class="text-xs text-ink-300 leading-relaxed">
          请先在左侧选择或新建一部小说作品，即可开启多分支叙事推演与战略抉择。
        </p>
      </div>
      <div class="pt-2">
        <button 
          @click="state.showNewProjectModal = true"
          class="w-full py-2.5 px-4 bg-gradient-to-r from-brand-amber to-amber-600 hover:from-brand-amber-hover hover:to-amber-500 text-atelier-950 font-bold text-xs rounded-lg transition shadow-atelier-md flex items-center justify-center gap-2 cursor-pointer">
          <Plus class="w-4 h-4" />
          <span>新建作品</span>
        </button>
      </div>
    </div>
  </div>

  <!-- 主视图容器 -->
  <div v-else class="flex-1 p-6 overflow-y-auto space-y-6 bg-atelier-950">
    <!-- 头部工具栏 -->
    <div class="flex flex-col md:flex-row md:items-center justify-between gap-4 border-b border-atelier-750 pb-5">
      <div>
        <div class="flex items-center gap-2.5">
          <div class="w-8 h-8 rounded-lg bg-brand-amber/10 border border-brand-amber/25 flex items-center justify-center text-brand-amber shadow-amber-glow">
            <GitBranch class="w-4 h-4" />
          </div>
          <div>
            <div class="flex items-center gap-2">
              <h2 class="text-base font-serif font-bold text-ink-50 tracking-wide">剧情树</h2>
              <span class="text-[10px] font-mono text-ink-400 bg-atelier-850 px-2 py-0.5 rounded border border-atelier-750">
                {{ state.currentProject.title || '当前作品' }}
              </span>
            </div>
            <p class="text-xs text-ink-400 mt-0.5">前瞻叙事分支推演、多路径对比与执行计划选定。</p>
          </div>
        </div>
      </div>

      <div class="flex items-center gap-3">
        <div class="flex items-center gap-2 text-xs font-mono bg-atelier-850 px-3 py-1.5 rounded-md border border-atelier-750">
          <span class="text-ink-400">推演记录</span>
          <strong class="text-brand-amber">{{ forecasts.length }}</strong>
          <span class="text-atelier-700">|</span>
          <span class="text-ink-400">活跃计划</span>
          <span v-if="activePlan" :class="getPlanStatusClass(activePlan.status)" class="font-bold">
            {{ activePlan.status.toUpperCase() }}
          </span>
          <span v-else class="text-ink-500">未选定</span>
        </div>

        <button 
          @click="loadData" 
          :disabled="isLoading"
          class="flex items-center gap-1.5 px-3 py-1.5 bg-atelier-850 hover:bg-atelier-800 text-ink-300 text-xs rounded-md border border-atelier-750 transition cursor-pointer disabled:opacity-50">
          <RefreshCw class="w-3.5 h-3.5 text-ink-400" :class="{ 'animate-spin': isLoading }" />
          <span>刷新</span>
        </button>
      </div>
    </div>

    <!-- 活跃计划区 (Active Plan) -->
    <div class="bg-atelier-900 border border-atelier-750 rounded-xl p-5 shadow-atelier-sm space-y-3">
      <div class="flex items-center justify-between border-b border-atelier-800 pb-3">
        <div class="flex items-center gap-2">
          <Compass class="w-4 h-4 text-brand-amber" />
          <h3 class="text-xs font-bold font-serif text-ink-100 tracking-wide">活跃计划区</h3>
        </div>
        <div v-if="activePlan" class="flex items-center gap-2">
          <span class="text-[11px] font-mono text-ink-400">
            分支: <span class="text-ink-200 font-bold">{{ activePlan.branch_id }}</span>
          </span>
          <span 
            :class="getPlanStatusBadgeClass(activePlan.status)" 
            class="text-[10px] font-mono px-2 py-0.5 rounded border">
            {{ getPlanStatusLabel(activePlan.status) }}
          </span>
        </div>
      </div>

      <!-- 无活跃计划提示 -->
      <div v-if="!activePlan" class="py-3 px-4 bg-atelier-950/60 border border-dashed border-atelier-800 rounded-lg flex items-center gap-3">
        <Info class="w-4 h-4 text-ink-400 shrink-0" />
        <div class="text-xs text-ink-300">
          <span class="font-medium text-ink-200">未选定路线，生产将按正史自由推进</span>
          <span class="text-ink-500 ml-1.5">（可在下方推演分支卡片中选定心仪路线以指导生产）</span>
        </div>
      </div>

      <!-- 存在活跃计划详情 -->
      <div v-else class="space-y-3">
        <!-- 黄色警示提示 (若有 warning_message) -->
        <div 
          v-if="activePlan.warning_message" 
          class="p-3 bg-amber-500/10 border border-amber-500/30 rounded-lg flex items-start gap-2.5 text-amber-200 text-xs shadow-atelier-sm">
          <AlertTriangle class="w-4 h-4 text-amber-400 shrink-0 mt-0.5" />
          <div class="flex-1 leading-relaxed">
            <span class="font-bold text-amber-300 mr-1">警示提醒：</span>
            <span>{{ activePlan.warning_message }}</span>
          </div>
        </div>

        <!-- 计划内容 -->
        <div class="space-y-1">
          <div class="text-[10px] font-mono uppercase tracking-wider text-ink-400">计划内容 (plan_text)</div>
          <div class="text-xs text-ink-100 bg-atelier-950 p-3.5 rounded-lg border border-atelier-800 whitespace-pre-wrap leading-relaxed font-sans">
            {{ activePlan.plan_text }}
          </div>
        </div>

        <!-- 预期状态增量 (若有 expected_deltas) -->
        <div v-if="activePlan.expected_deltas" class="space-y-1">
          <div class="text-[10px] font-mono uppercase tracking-wider text-ink-400">预期增量 (expected_deltas)</div>
          <div class="text-xs text-ink-200 bg-atelier-950 p-2.5 rounded-lg border border-atelier-800 whitespace-pre-wrap leading-relaxed">
            {{ activePlan.expected_deltas }}
          </div>
        </div>

        <!-- 计划章节消费推进 (若有 plan_chapters) -->
        <div v-if="activePlan.plan_chapters && activePlan.plan_chapters.length > 0" class="space-y-1.5 pt-1">
          <div class="text-[10px] font-mono uppercase tracking-wider text-ink-400">规划章节推进 (plan_chapters)</div>
          <div class="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-2">
            <div 
              v-for="ch in activePlan.plan_chapters" 
              :key="ch.chapter_index" 
              class="p-2.5 bg-atelier-950 border border-atelier-800 rounded-md text-xs flex items-center justify-between">
              <div class="truncate mr-2">
                <span class="font-mono text-brand-amber font-bold mr-1.5">第 {{ ch.chapter_index }} 章</span>
                <span class="text-ink-300">{{ ch.title || ch.summary || '未设定小结' }}</span>
              </div>
              <span 
                :class="ch.status === 'consumed' ? 'text-emerald-400 bg-emerald-500/10 border-emerald-500/20' : 'text-amber-400 bg-amber-500/10 border-amber-500/20'" 
                class="text-[10px] font-mono px-1.5 py-0.5 rounded border shrink-0">
                {{ ch.status }}
              </span>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- 顶部「生成推演」表单 -->
    <div class="bg-atelier-900 border border-atelier-750 rounded-xl p-5 shadow-atelier-sm space-y-4">
      <div class="flex items-center justify-between border-b border-atelier-800 pb-3">
        <div class="flex items-center gap-2">
          <Sparkles class="w-4 h-4 text-brand-amber" />
          <h3 class="text-xs font-bold font-serif text-ink-100 tracking-wide">生成推演</h3>
        </div>
        <span class="text-[10px] font-mono text-ink-400">
          基于当前正史基准章向前推演候选分支
        </span>
      </div>

      <div class="space-y-3">
        <div>
          <label class="block text-xs font-medium text-ink-300 mb-1.5">
            剧情分歧点 (Divergence)：
          </label>
          <textarea 
            v-model="form.divergence" 
            rows="3"
            placeholder="设定本轮剧情的分歧点或假想事件，例如：主角在拍卖会上选择放弃筑基丹转而拍下残破剑柄，遭到黑市势力截杀；或者主角选择暗中联络魔宗卧底..."
            class="w-full bg-atelier-950 border border-atelier-750 rounded-lg p-3 text-xs text-ink-100 placeholder-ink-500 focus:outline-none focus:border-brand-amber/60 focus:ring-1 focus:ring-brand-amber/40 transition resize-y">
          </textarea>
        </div>

        <div class="grid grid-cols-1 sm:grid-cols-3 gap-3 items-end">
          <div>
            <label class="block text-[11px] font-mono text-ink-400 mb-1">
              分支数量 (2-5)：
            </label>
            <select 
              v-model.number="form.branch_count" 
              class="w-full bg-atelier-950 border border-atelier-750 rounded-md px-3 py-2 text-xs text-ink-100 focus:outline-none focus:border-brand-amber/60 cursor-pointer">
              <option :value="2">2 个候选分支</option>
              <option :value="3">3 个候选分支</option>
              <option :value="4">4 个候选分支</option>
              <option :value="5">5 个候选分支</option>
            </select>
          </div>

          <div>
            <label class="block text-[11px] font-mono text-ink-400 mb-1">
              推演章数 (3-5)：
            </label>
            <select 
              v-model.number="form.horizon" 
              class="w-full bg-atelier-950 border border-atelier-750 rounded-md px-3 py-2 text-xs text-ink-100 focus:outline-none focus:border-brand-amber/60 cursor-pointer">
              <option :value="3">前瞻 3 章</option>
              <option :value="4">前瞻 4 章</option>
              <option :value="5">前瞻 5 章</option>
            </select>
          </div>

          <div>
            <button 
              @click="handleCreateForecast" 
              :disabled="isGenerating || !form.divergence.trim()"
              class="w-full py-2 px-4 bg-brand-amber hover:bg-brand-amber-hover text-atelier-950 font-bold text-xs rounded-md shadow-amber-glow transition flex items-center justify-center gap-1.5 cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed">
              <Sparkles class="w-3.5 h-3.5" :class="{ 'animate-spin': isGenerating }" />
              <span>{{ isGenerating ? 'AI 推演中...' : '生成推演' }}</span>
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- 推演列表 -->
    <div class="space-y-4">
      <div class="flex items-center justify-between">
        <div class="flex items-center gap-2">
          <Layers class="w-4 h-4 text-ink-400" />
          <h3 class="text-xs font-bold font-serif text-ink-100 tracking-wide">推演历史列表</h3>
        </div>
        <span class="text-[10px] font-mono text-ink-400">点击单项展开分支卡片与详情</span>
      </div>

      <!-- 空白列表状态 -->
      <div 
        v-if="forecasts.length === 0" 
        class="text-center py-16 bg-atelier-900 border border-dashed border-atelier-750 rounded-xl space-y-2">
        <GitBranch class="w-8 h-8 text-ink-500 mx-auto" />
        <h4 class="text-xs font-semibold text-ink-200">暂无推演记录</h4>
        <p class="text-[11px] text-ink-400 max-w-sm mx-auto">
          在上方输入分歧点假想并点击「生成推演」，AI 将基于当前基准章节生成多条并列分支。
        </p>
      </div>

      <!-- 列表卡片 -->
      <div v-else class="space-y-3">
        <div 
          v-for="fc in forecasts" 
          :key="fc.id" 
          class="bg-atelier-900 border rounded-xl overflow-hidden transition shadow-atelier-sm"
          :class="expandedForecastId === fc.id ? 'border-brand-amber/50 bg-atelier-900' : 'border-atelier-750 hover:border-atelier-700'">
          
          <!-- 列表项摘要栏 (点击切换展开/折叠) -->
          <div 
            @click="toggleForecast(fc.id)" 
            class="p-4 flex flex-col md:flex-row md:items-center justify-between gap-3 cursor-pointer select-none">
            <div class="flex items-start md:items-center gap-3 min-w-0">
              <div 
                class="w-7 h-7 rounded-lg flex items-center justify-center shrink-0"
                :class="expandedForecastId === fc.id ? 'bg-brand-amber/20 text-brand-amber' : 'bg-atelier-850 text-ink-400'">
                <GitBranch class="w-3.5 h-3.5" />
              </div>
              <div class="min-w-0">
                <div class="flex items-center gap-2 flex-wrap">
                  <span class="font-mono text-xs font-bold text-ink-100">{{ fc.id }}</span>
                  <span 
                    :class="getForecastStatusBadgeClass(fc.status)" 
                    class="text-[10px] font-mono px-2 py-0.2 rounded border font-medium">
                    {{ getForecastStatusLabel(fc.status) }}
                  </span>
                  <span class="text-[11px] font-mono text-ink-400 bg-atelier-950 px-2 py-0.5 rounded border border-atelier-800">
                    基准章: 第 {{ fc.base_chapter }} 章
                  </span>
                  <span class="text-[11px] font-mono text-ink-400">
                    {{ fc.branch_count }} 分支 · 前瞻 {{ fc.horizon }} 章
                  </span>
                </div>
                <div class="text-xs text-ink-300 mt-1 line-clamp-1">
                  <span class="text-ink-400">分歧点：</span>{{ fc.divergence }}
                </div>
              </div>
            </div>

            <div class="flex items-center gap-3 shrink-0 self-end md:self-center">
              <span class="text-[11px] font-mono text-ink-400">
                {{ formatTime(fc.created_at) }}
              </span>
              <button 
                class="w-6 h-6 rounded flex items-center justify-center text-ink-400 hover:text-ink-100 bg-atelier-850 border border-atelier-750">
                <ChevronUp v-if="expandedForecastId === fc.id" class="w-3.5 h-3.5" />
                <ChevronDown v-else class="w-3.5 h-3.5" />
              </button>
            </div>
          </div>

          <!-- 展开详情面板 -->
          <div 
            v-if="expandedForecastId === fc.id" 
            class="border-t border-atelier-800 p-5 bg-atelier-950/40 space-y-5">
            <!-- 详情头部说明 -->
            <div class="bg-atelier-950 p-3.5 rounded-lg border border-atelier-800 space-y-1.5">
              <div class="text-[10px] font-mono uppercase tracking-wider text-ink-400 flex items-center justify-between">
                <span>分歧点全貌 (Divergence)</span>
                <span class="text-ink-500 font-mono">Fingerprint: {{ currentDetail(fc.id)?.fingerprint || fc.fingerprint }}</span>
              </div>
              <p class="text-xs text-ink-100 leading-relaxed whitespace-pre-wrap">
                {{ currentDetail(fc.id)?.divergence || fc.divergence }}
              </p>
            </div>

            <!-- 分支并排对比卡片 -->
            <div>
              <div class="text-xs font-mono text-ink-400 mb-3 flex items-center justify-between">
                <span class="font-bold text-ink-200">候选走向分支对比 ({{ (currentDetail(fc.id)?.branches || fc.branches || []).length }}条)</span>
                <span class="text-[11px] text-ink-400">选定分支将确立为唯一生效计划</span>
              </div>

              <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                <div 
                  v-for="branch in (currentDetail(fc.id)?.branches || fc.branches || [])" 
                  :key="branch.id" 
                  class="bg-atelier-900 border rounded-xl p-4 flex flex-col justify-between transition space-y-4"
                  :class="isBranchActive(fc.id, branch.id) ? 'border-emerald-500/50 ring-1 ring-emerald-500/30' : 'border-atelier-750 hover:border-brand-amber/30'">
                  
                  <!-- 分支头部与意图吻合度突出展示 -->
                  <div class="space-y-3">
                    <div class="flex items-center justify-between border-b border-atelier-800 pb-2">
                      <span class="font-mono text-xs font-bold text-brand-amber">
                        分支 ID: {{ branch.id }}
                      </span>
                      <span 
                        v-if="isBranchActive(fc.id, branch.id)" 
                        class="text-[10px] font-mono px-2 py-0.5 rounded bg-emerald-500/10 text-emerald-400 border border-emerald-500/30 font-bold">
                        当前活跃路线
                      </span>
                    </div>

                    <!-- 突出渲染 intent_match -->
                    <div class="p-3 bg-brand-amber/10 border border-brand-amber/30 rounded-lg space-y-1">
                      <div class="flex items-center gap-1.5 text-[11px] font-bold text-brand-amber uppercase tracking-wider font-mono">
                        <Target class="w-3.5 h-3.5" />
                        <span>意图吻合度 (intent_match)</span>
                      </div>
                      <div class="text-xs text-ink-100 font-medium leading-relaxed whitespace-pre-wrap">
                        {{ branch.intent_match || '未提供意图契合评估' }}
                      </div>
                    </div>

                    <!-- beats_summary -->
                    <div class="p-3 bg-atelier-950 rounded-lg border border-atelier-800 space-y-1">
                      <div class="flex items-center gap-1.5 text-[11px] font-mono text-ink-400 uppercase tracking-wider">
                        <Layers class="w-3.5 h-3.5 text-ink-400" />
                        <span>节拍大纲 (beats_summary)</span>
                      </div>
                      <div class="text-xs text-ink-200 leading-relaxed whitespace-pre-wrap font-sans">
                        {{ branch.beats_summary || '无大纲描述' }}
                      </div>
                    </div>

                    <!-- decisions -->
                    <div class="p-3 bg-atelier-950 rounded-lg border border-atelier-800 space-y-1">
                      <div class="flex items-center gap-1.5 text-[11px] font-mono text-ink-400 uppercase tracking-wider">
                        <Compass class="w-3.5 h-3.5 text-ink-400" />
                        <span>核心抉择 (decisions)</span>
                      </div>
                      <div class="text-xs text-ink-200 leading-relaxed whitespace-pre-wrap">
                        {{ branch.decisions || '无决策变动' }}
                      </div>
                    </div>

                    <!-- expected_deltas -->
                    <div class="p-3 bg-atelier-950 rounded-lg border border-atelier-800 space-y-1">
                      <div class="flex items-center gap-1.5 text-[11px] font-mono text-ink-400 uppercase tracking-wider">
                        <Flame class="w-3.5 h-3.5 text-ink-400" />
                        <span>预期增量 (expected_deltas)</span>
                      </div>
                      <div class="text-xs text-ink-200 leading-relaxed whitespace-pre-wrap">
                        {{ branch.expected_deltas || '无状态变化增量' }}
                      </div>
                    </div>

                    <!-- risks -->
                    <div class="p-3 bg-rose-500/5 rounded-lg border border-rose-500/20 space-y-1">
                      <div class="flex items-center gap-1.5 text-[11px] font-mono text-rose-400 uppercase tracking-wider">
                        <AlertTriangle class="w-3.5 h-3.5 text-rose-400" />
                        <span>叙事风险 (risks)</span>
                      </div>
                      <div class="text-xs text-rose-200/90 leading-relaxed whitespace-pre-wrap">
                        {{ branch.risks || '无明确叙事风险' }}
                      </div>
                    </div>
                  </div>

                  <!-- 选定此分支操作按钮 -->
                  <div class="pt-3 border-t border-atelier-800">
                    <button 
                      v-if="isBranchActive(fc.id, branch.id)"
                      disabled
                      class="w-full py-2 px-3 bg-emerald-500/15 border border-emerald-500/30 text-emerald-400 rounded-lg text-xs font-semibold flex items-center justify-center gap-1.5 cursor-default">
                      <Check class="w-3.5 h-3.5" />
                      <span>当前采用中 (已生效)</span>
                    </button>
                    <button 
                      v-else
                      @click="handleSelectBranch(fc.id, branch.id)"
                      :disabled="isSelecting"
                      class="w-full py-2 px-3 bg-brand-amber/15 hover:bg-brand-amber/25 border border-brand-amber/30 hover:border-brand-amber text-brand-amber rounded-lg text-xs font-semibold transition flex items-center justify-center gap-1.5 cursor-pointer disabled:opacity-50">
                      <CheckCircle class="w-3.5 h-3.5" />
                      <span>{{ isSelecting && selectingBranchId === branch.id ? '选定中...' : '选定此分支' }}</span>
                    </button>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, watch } from 'vue';
import { state, notify } from '../stores/appState';
import { api } from '../api/client';
import { 
  GitBranch, 
  Sparkles, 
  Compass, 
  Info, 
  AlertTriangle, 
  RefreshCw, 
  Layers, 
  Target, 
  Flame, 
  Check, 
  CheckCircle, 
  ChevronDown, 
  ChevronUp, 
  Plus 
} from 'lucide-vue-next';

// 页面推演状态
const forecasts = ref([]);
const forecastDetails = ref({});
const activePlan = ref(null);
const expandedForecastId = ref(null);

const isLoading = ref(false);
const isGenerating = ref(false);
const isSelecting = ref(false);
const selectingBranchId = ref(null);

// 生成推演表单
const form = reactive({
  divergence: '',
  branch_count: 3,
  horizon: 3,
});

// 获取已缓存或拉取的详情
function currentDetail(fid) {
  return forecastDetails.value[fid] || null;
}

// 格式化时间
function formatTime(isoString) {
  if (!isoString) return '';
  try {
    const d = new Date(isoString);
    if (isNaN(d.getTime())) return isoString;
    return d.toLocaleString('zh-CN', {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
    });
  } catch {
    return isoString;
  }
}

// 判断分支是否为当前活跃计划
function isBranchActive(forecastId, branchId) {
  if (!activePlan.value) return false;
  return activePlan.value.forecast_id === forecastId && activePlan.value.branch_id === branchId;
}

// 计划状态徽章样式与文本
function getPlanStatusClass(status) {
  switch (status) {
    case 'active': return 'text-emerald-400';
    case 'stale': return 'text-amber-400';
    case 'consumed': return 'text-blue-400';
    case 'diverged': return 'text-rose-400';
    default: return 'text-ink-400';
  }
}

function getPlanStatusBadgeClass(status) {
  switch (status) {
    case 'active': return 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20';
    case 'stale': return 'bg-amber-500/10 text-amber-400 border-amber-500/20';
    case 'consumed': return 'bg-blue-500/10 text-blue-400 border-blue-500/20';
    case 'diverged': return 'bg-rose-500/10 text-rose-400 border-rose-500/20';
    default: return 'bg-atelier-800 text-ink-400 border-atelier-700';
  }
}

function getPlanStatusLabel(status) {
  switch (status) {
    case 'active': return '🟢 活跃执行 (ACTIVE)';
    case 'stale': return '⚠️ 已过期 (STALE)';
    case 'consumed': return '🔵 已消耗 (CONSUMED)';
    case 'diverged': return '🔴 已偏离 (DIVERGED)';
    case 'dropped': return '⚪ 已放弃 (DROPPED)';
    default: return status || 'UNKNOWN';
  }
}

// 推演状态徽章样式与文本
function getForecastStatusBadgeClass(status) {
  switch (status) {
    case 'active': return 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20';
    case 'stale': return 'bg-amber-500/10 text-amber-400 border-amber-500/20';
    case 'archived': return 'bg-atelier-800 text-ink-400 border-atelier-700';
    default: return 'bg-atelier-800 text-ink-300 border-atelier-700';
  }
}

function getForecastStatusLabel(status) {
  switch (status) {
    case 'active': return '活跃 (ACTIVE)';
    case 'stale': return '已过期 (STALE)';
    case 'archived': return '已归档 (ARCHIVED)';
    default: return status || 'UNKNOWN';
  }
}

// 加载推演列表与活跃计划
async function loadData() {
  if (!state.currentProject?.id) return;
  isLoading.value = true;
  try {
    const [listRes, planRes] = await Promise.allSettled([
      api.listForecasts(state.currentProject.id),
      api.getActivePlan(state.currentProject.id),
    ]);

    if (listRes.status === 'fulfilled') {
      forecasts.value = listRes.value || [];
      // 默认展开逻辑：若尚无展开项且有推演记录，展开首个活跃推演或首项
      if (forecasts.value.length > 0 && !expandedForecastId.value) {
        const firstTarget = forecasts.value.find(f => f.status === 'active') || forecasts.value[0];
        expandForecast(firstTarget.id);
      }
    } else {
      notify('加载推演列表失败', listRes.reason?.message || '未知错误', 'error');
    }

    if (planRes.status === 'fulfilled') {
      activePlan.value = planRes.value;
    } else {
      activePlan.value = null;
    }
  } finally {
    isLoading.value = false;
  }
}

// 展开/收起推演项
async function toggleForecast(fid) {
  if (expandedForecastId.value === fid) {
    expandedForecastId.value = null;
    return;
  }
  await expandForecast(fid);
}

// 单独展开推演详情 (实时触发后端 LoadForecast 以刷新派生 stale 状态)
async function expandForecast(fid) {
  expandedForecastId.value = fid;
  if (!state.currentProject?.id) return;
  try {
    const detail = await api.getForecast(state.currentProject.id, fid);
    forecastDetails.value[fid] = detail;
    // 派生 stale 状态同步更新到列表中对应项
    const idx = forecasts.value.findIndex(f => f.id === fid);
    if (idx !== -1 && detail) {
      forecasts.value[idx].status = detail.status;
      forecasts.value[idx].branches = detail.branches;
    }
  } catch (err) {
    notify('获取推演详情失败', err.message || '网络异常', 'error');
  }
}

// 生成推演动作
async function handleCreateForecast() {
  if (!state.currentProject?.id) return;
  const divText = form.divergence.trim();
  if (!divText) {
    notify('提示', '请输入分歧点假想', 'warning');
    return;
  }
  if (form.branch_count < 2 || form.branch_count > 5) {
    notify('提示', '分支数必须在 2 到 5 之间', 'warning');
    return;
  }
  if (form.horizon < 3 || form.horizon > 5) {
    notify('提示', '推演章数必须在 3 到 5 之间', 'warning');
    return;
  }

  isGenerating.value = true;
  try {
    const newFc = await api.createForecast(state.currentProject.id, {
      divergence: divText,
      branch_count: Number(form.branch_count),
      horizon: Number(form.horizon),
    });
    notify('剧情推演生成成功', `已生成 ${newFc?.branches?.length || form.branch_count} 条候选推演路线`, 'success');
    form.divergence = '';
    await loadData();
    if (newFc?.id) {
      await expandForecast(newFc.id);
    }
  } catch (err) {
    notify('生成推演失败', err.message || '网络或模型异常', 'error');
  } finally {
    isGenerating.value = false;
  }
}

// 选定分支计划动作 (严格显示后端错误，绝不静默吞错)
async function handleSelectBranch(forecastId, branchId) {
  if (!state.currentProject?.id) return;
  isSelecting.value = true;
  selectingBranchId.value = branchId;
  try {
    const newPlan = await api.selectForecast(state.currentProject.id, forecastId, { branch_id: branchId });
    activePlan.value = newPlan;
    notify('分支计划选定成功', '已更新当前作品的活跃推演计划', 'success');
    await loadData();
    if (forecastId) {
      await expandForecast(forecastId);
    }
  } catch (err) {
    notify('选定分支失败', err.message || '操作失败', 'error');
  } finally {
    isSelecting.value = false;
    selectingBranchId.value = null;
  }
}

// 项目切换时清空本地推演状态并重拉，防止跨项目串数据
watch(
  () => state.currentProject?.id,
  (newId, oldId) => {
    if (newId !== oldId) {
      forecasts.value = [];
      forecastDetails.value = {};
      expandedForecastId.value = null;
      activePlan.value = null;
      form.divergence = '';
      if (newId) {
        loadData();
      }
    }
  },
  { immediate: true }
);

// Tab 切换响应：从其他 tab 切到 forecast 时重新拉取，确保派生 stale 状态最新
watch(
  () => state.activeTab,
  (newTab) => {
    if (newTab === 'forecast' && state.currentProject?.id) {
      loadData();
    }
  }
);
</script>
