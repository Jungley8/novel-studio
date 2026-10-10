<template>
  <div class="flex-1 p-6 overflow-y-auto space-y-6">
    <!-- 头部工具栏 -->
    <div class="flex flex-col md:flex-row md:items-center justify-between gap-4 border-b border-atelier-750 pb-5">
      <div>
        <div class="flex items-center gap-2.5">
          <div class="w-8 h-8 rounded-lg bg-brand-amber/10 border border-brand-amber/25 flex items-center justify-center text-brand-amber shadow-amber-glow">
            <ListChecks class="w-4 h-4" />
          </div>
          <div>
            <div class="flex items-center gap-2">
              <h2 class="text-base font-serif font-bold text-ink-50 tracking-wide">章节意图工坊</h2>
              <span class="text-[10px] font-mono px-1.5 py-0.5 rounded bg-brand-amber/15 text-brand-amber border border-brand-amber/30">
                INTENTS
              </span>
            </div>
            <p class="text-xs text-ink-400 mt-0.5">按章提炼核心戏剧冲突、POV视角与情绪张力，衔接分卷总纲与细分场次。</p>
          </div>
        </div>
      </div>

      <!-- 右侧控制区：卷选择器 + 刷新 + AI 推演按钮 -->
      <div v-if="state.currentProject && volumeArcs.length > 0" class="flex flex-wrap items-center gap-2.5">
        <!-- 卷选择器 -->
        <div class="flex items-center gap-1.5 bg-atelier-900 border border-atelier-750 rounded-lg px-2.5 py-1.5 text-xs">
          <span class="text-ink-400 shrink-0">选择卷:</span>
          <select 
            v-model.number="selectedVolumeIndex" 
            @change="loadIntents"
            class="bg-transparent text-ink-100 font-medium focus:outline-none cursor-pointer pr-1">
            <option 
              v-for="(arc, idx) in volumeArcs" 
              :key="idx" 
              :value="arc.volume_index || (idx + 1)"
              class="bg-atelier-900 text-ink-100">
              第 {{ arc.volume_index || (idx + 1) }} 卷 · {{ arc.title || arc.name || '未命名卷' }}
            </option>
          </select>
        </div>

        <!-- 刷新按钮 -->
        <button 
          @click="loadIntents" 
          :disabled="isLoading"
          class="flex items-center gap-1.5 px-3 py-1.5 bg-atelier-850 hover:bg-atelier-800 text-ink-200 text-xs font-medium rounded-lg border border-atelier-750 transition cursor-pointer disabled:opacity-50"
          title="刷新章节意图卡列表">
          <RefreshCw class="w-3.5 h-3.5 text-ink-400" :class="{ 'animate-spin': isLoading }" />
          <span>刷新</span>
        </button>

        <!-- AI 推演整卷意图卡按钮 -->
        <button 
          @click="generateIntents" 
          :disabled="isGenerating || isLoading"
          class="flex items-center gap-1.5 px-4 py-1.5 bg-brand-amber hover:bg-brand-amber-hover text-atelier-950 font-bold text-xs rounded-lg shadow-amber-glow transition cursor-pointer disabled:opacity-50">
          <Sparkles class="w-3.5 h-3.5" :class="{ 'animate-spin': isGenerating }" />
          <span>{{ isGenerating ? '推演中...' : 'AI 推演整卷意图卡' }}</span>
        </button>
      </div>
    </div>

    <!-- 空态 1：未选项目 -->
    <div 
      v-if="!state.currentProject" 
      class="text-center py-20 bg-atelier-900 border border-dashed border-atelier-750 rounded-xl space-y-3">
      <div class="w-12 h-12 rounded-full bg-atelier-800 flex items-center justify-center mx-auto text-ink-400">
        <ListChecks class="w-6 h-6" />
      </div>
      <h3 class="text-sm font-semibold text-ink-200">未选择小说作品</h3>
      <p class="text-xs text-ink-400 max-w-md mx-auto">
        请在左侧侧边栏选择已有作品，或新建一部作品开始推演章节意图。
      </p>
    </div>

    <!-- 空态 2：项目无 framework 或无 volume_arcs -->
    <div 
      v-else-if="volumeArcs.length === 0" 
      class="text-center py-20 bg-atelier-900 border border-dashed border-atelier-750 rounded-xl space-y-3">
      <div class="w-12 h-12 rounded-full bg-atelier-800 flex items-center justify-center mx-auto text-brand-amber">
        <Layers class="w-6 h-6" />
      </div>
      <h3 class="text-sm font-semibold text-ink-200">尚未规划分卷大纲</h3>
      <p class="text-xs text-ink-400 max-w-md mx-auto">
        章节意图卡基于分卷设定生成。请先前往「作品设定」中配置分卷弧光规划。
      </p>
      <div class="pt-2">
        <button 
          @click="state.activeTab = 'framework'" 
          class="px-4 py-2 bg-brand-amber/15 hover:bg-brand-amber/25 text-brand-amber border border-brand-amber/30 text-xs font-semibold rounded-lg transition cursor-pointer">
          前往作品设定规划分卷
        </button>
      </div>
    </div>

    <!-- 空态 3：当前卷零意图卡 -->
    <div 
      v-else-if="intents.length === 0 && !isLoading" 
      class="text-center py-20 bg-atelier-900 border border-dashed border-atelier-750 rounded-xl space-y-3">
      <div class="w-12 h-12 rounded-full bg-atelier-800 flex items-center justify-center mx-auto text-ink-400">
        <Sparkles class="w-6 h-6 text-brand-amber" />
      </div>
      <h3 class="text-sm font-semibold text-ink-200">当前卷暂无章节意图卡</h3>
      <p class="text-xs text-ink-400 max-w-md mx-auto">
        第 {{ selectedVolumeIndex }} 卷尚未生成任何章节意图卡。点击下方按钮，由 AI 自动推演本卷各章的戏剧冲突、视角、张力与伏笔。
      </p>
      <div class="pt-2">
        <button 
          @click="generateIntents" 
          :disabled="isGenerating"
          class="px-4 py-2 bg-brand-amber hover:bg-brand-amber-hover text-atelier-950 font-bold text-xs rounded-lg shadow-amber-glow transition cursor-pointer disabled:opacity-50 inline-flex items-center gap-1.5">
          <Sparkles class="w-3.5 h-3.5" :class="{ 'animate-spin': isGenerating }" />
          <span>{{ isGenerating ? 'AI 推演中...' : '立即 AI 推演整卷意图卡' }}</span>
        </button>
      </div>
    </div>

    <!-- 列表内容区：章节意图卡 -->
    <div v-else class="space-y-4">
      <!-- 统计指标小栏 -->
      <div class="flex items-center justify-between text-xs font-mono text-ink-400 px-1">
        <div>
          <span>第 {{ selectedVolumeIndex }} 卷 · </span>
          <span class="text-ink-200 font-bold">{{ sortedIntents.length }}</span>
          <span> 章意图卡</span>
        </div>
        <div class="flex items-center gap-3">
          <span class="flex items-center gap-1">
            <span class="w-2 h-2 rounded-full bg-atelier-600"></span>
            <span>草稿: {{ countByStatus('DRAFT') }}</span>
          </span>
          <span class="flex items-center gap-1">
            <span class="w-2 h-2 rounded-full bg-brand-amber"></span>
            <span>已锁定: {{ countByStatus('LOCKED') }}</span>
          </span>
          <span class="flex items-center gap-1">
            <span class="w-2 h-2 rounded-full bg-rose-500"></span>
            <span>已过期: {{ countByStatus('STALE') }}</span>
          </span>
        </div>
      </div>

      <!-- 意图卡网格列表 -->
      <div class="space-y-4">
        <div 
          v-for="intent in sortedIntents" 
          :key="intent.chapter_index"
          class="bg-atelier-900 border border-atelier-750 rounded-xl p-5 space-y-4 shadow-atelier-md transition hover:border-atelier-700">
          
          <!-- 卡片头部：章号、状态徽章、操作按钮 -->
          <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 border-b border-atelier-800 pb-3">
            <div class="flex items-center gap-2.5">
              <span class="px-2.5 py-1 bg-brand-amber/15 text-brand-amber text-xs font-mono font-bold rounded border border-brand-amber/30 shadow-amber-glow">
                第 {{ intent.chapter_index }} 章
              </span>

              <!-- 状态徽章：DRAFT 灰 / LOCKED 琥珀 / STALE 红 + title -->
              <span 
                v-if="intent.status === 'LOCKED'"
                class="px-2 py-0.5 rounded text-[11px] font-mono font-semibold bg-brand-amber/15 text-brand-amber border border-brand-amber/40 shadow-amber-glow flex items-center gap-1"
                title="已锁定状态：受保护卡片">
                <Lock class="w-3 h-3" />
                <span>已锁定</span>
              </span>
              <span 
                v-else-if="intent.status === 'STALE'"
                class="px-2 py-0.5 rounded text-[11px] font-mono font-semibold bg-rose-500/15 text-rose-400 border border-rose-500/30 flex items-center gap-1"
                title="上游卷纲已变更，建议重推">
                <AlertCircle class="w-3 h-3" />
                <span>已过期 (STALE)</span>
              </span>
              <span 
                v-else
                class="px-2 py-0.5 rounded text-[11px] font-mono font-medium bg-atelier-800 text-ink-300 border border-atelier-700 flex items-center gap-1"
                title="草稿状态：可自由编辑或重推">
                <span>草稿</span>
              </span>

              <span v-if="intent.pov" class="text-xs text-ink-400 font-serif">
                · POV: <strong class="text-ink-200">{{ intent.pov }}</strong>
              </span>
            </div>

            <!-- 卡片右侧动作按钮 -->
            <div class="flex items-center gap-2">
              <!-- 锁定 / 解锁切换 -->
              <button 
                @click="toggleLockIntent(intent)"
                :title="intent.status === 'LOCKED' ? '解锁为草稿' : '锁定意图卡'"
                class="px-2.5 py-1 text-xs rounded border transition flex items-center gap-1 cursor-pointer"
                :class="intent.status === 'LOCKED' 
                  ? 'bg-atelier-800 hover:bg-atelier-750 text-ink-300 border-atelier-700' 
                  : 'bg-brand-amber/10 hover:bg-brand-amber/20 text-brand-amber border-brand-amber/30'">
                <component :is="intent.status === 'LOCKED' ? Unlock : Lock" class="w-3 h-3" />
                <span>{{ intent.status === 'LOCKED' ? '解锁' : '锁定' }}</span>
              </button>

              <!-- 编辑 / 收起 -->
              <button 
                @click="toggleEdit(intent)"
                class="px-2.5 py-1 text-xs bg-atelier-850 hover:bg-atelier-800 text-ink-200 rounded border border-atelier-750 transition flex items-center gap-1 cursor-pointer">
                <component :is="editingId === intent.chapter_index ? X : Edit3" class="w-3 h-3" />
                <span>{{ editingId === intent.chapter_index ? '收起' : '编辑' }}</span>
              </button>

              <!-- 删除 -->
              <button 
                @click="deleteIntent(intent)"
                class="px-2.5 py-1 text-xs bg-rose-500/10 hover:bg-rose-500/20 text-rose-400 rounded border border-rose-500/30 transition flex items-center gap-1 cursor-pointer"
                title="删除本章意图卡">
                <Trash2 class="w-3 h-3" />
                <span>删除</span>
              </button>
            </div>
          </div>

          <!-- 常规查看模式 -->
          <div v-if="editingId !== intent.chapter_index" class="space-y-3.5">
            <!-- 核心冲突 -->
            <div>
              <span class="text-[11px] font-mono text-ink-400 block mb-1">核心戏剧冲突:</span>
              <p class="text-xs font-serif text-ink-100 leading-relaxed bg-atelier-950/80 p-3 rounded-lg border border-atelier-800">
                {{ intent.conflict || '暂未填写戏剧冲突' }}
              </p>
            </div>

            <!-- 张力目标 + 情绪曲线 -->
            <div class="grid grid-cols-1 md:grid-cols-12 gap-4 items-start">
              <!-- 张力 (tension_target/10 + 进度条) -->
              <div class="md:col-span-5 p-3 bg-atelier-950/60 rounded-lg border border-atelier-800 space-y-1.5">
                <div class="flex items-center justify-between text-xs">
                  <span class="text-ink-400">张力目标</span>
                  <span class="font-mono font-bold" :class="tensionColorClass(intent.tension_target)">
                    {{ intent.tension_target || 5 }} / 10
                  </span>
                </div>
                <div class="w-full bg-atelier-800 rounded-full h-1.5 overflow-hidden">
                  <div 
                    class="h-full rounded-full transition-all duration-300"
                    :class="tensionBarClass(intent.tension_target)"
                    :style="{ width: ((intent.tension_target || 5) * 10) + '%' }">
                  </div>
                </div>
              </div>

              <!-- 情绪曲线 (箭头串联) -->
              <div class="md:col-span-7 p-3 bg-atelier-950/60 rounded-lg border border-atelier-800 space-y-1.5">
                <span class="text-[11px] text-ink-400 block">情绪起伏曲线:</span>
                <div v-if="intent.emotion_curve && intent.emotion_curve.length > 0" class="flex flex-wrap items-center gap-1.5 text-xs">
                  <template v-for="(em, eIdx) in intent.emotion_curve" :key="eIdx">
                    <span class="px-2 py-0.5 rounded bg-atelier-800 text-ink-200 border border-atelier-750 font-serif">
                      {{ em }}
                    </span>
                    <ArrowRight v-if="eIdx < intent.emotion_curve.length - 1" class="w-3 h-3 text-brand-amber/70 shrink-0" />
                  </template>
                </div>
                <div v-else class="text-xs text-ink-500 italic">
                  未配置情绪起伏曲线
                </div>
              </div>
            </div>

            <!-- 伏笔埋钩与收钩 chips -->
            <div class="grid grid-cols-1 md:grid-cols-2 gap-3 pt-1">
              <!-- 埋钩 chips -->
              <div class="p-2.5 bg-atelier-950/40 rounded-lg border border-atelier-850 space-y-1.5">
                <div class="flex items-center gap-1.5 text-[11px] text-emerald-400 font-medium">
                  <Anchor class="w-3 h-3" />
                  <span>埋设伏笔 (Hook Plants)</span>
                </div>
                <div v-if="intent.hook_plants && intent.hook_plants.length > 0" class="flex flex-wrap gap-1.5">
                  <span 
                    v-for="(p, pIdx) in intent.hook_plants" 
                    :key="pIdx"
                    class="px-2 py-0.5 rounded bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 text-xs font-serif">
                    {{ p }}
                  </span>
                </div>
                <div v-else class="text-[11px] text-ink-500 italic">无新埋伏笔</div>
              </div>

              <!-- 收钩 chips -->
              <div class="p-2.5 bg-atelier-950/40 rounded-lg border border-atelier-850 space-y-1.5">
                <div class="flex items-center gap-1.5 text-[11px] text-brand-amber font-medium">
                  <CheckCircle2 class="w-3 h-3" />
                  <span>回收伏笔 (Hook Payoffs)</span>
                </div>
                <div v-if="intent.hook_payoffs && intent.hook_payoffs.length > 0" class="flex flex-wrap gap-1.5">
                  <span 
                    v-for="(po, poIdx) in intent.hook_payoffs" 
                    :key="poIdx"
                    class="px-2 py-0.5 rounded bg-brand-amber/10 text-brand-amber border border-brand-amber/25 text-xs font-serif">
                    {{ po }}
                  </span>
                </div>
                <div v-else class="text-[11px] text-ink-500 italic">无伏笔回收</div>
              </div>
            </div>
          </div>

          <!-- 卡内展开式编辑模式 -->
          <div v-else class="space-y-4 pt-2 border-t border-atelier-800">
            <!-- 核心戏剧冲突编辑 -->
            <div>
              <label class="text-xs font-medium text-ink-300 block mb-1">
                核心戏剧冲突 <span class="text-rose-400">*</span>
              </label>
              <textarea 
                v-model="editForm.conflict"
                rows="3"
                placeholder="明确本章核心戏剧冲突、受阻阻碍与对峙焦点..."
                class="w-full bg-atelier-950 border border-atelier-750 rounded-lg p-2.5 text-xs text-ink-100 font-serif focus:outline-none focus:border-brand-amber/60 resize-none leading-relaxed"></textarea>
            </div>

            <!-- POV 视角 & 张力目标 -->
            <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <div>
                <label class="text-xs font-medium text-ink-300 block mb-1">POV 叙事视角:</label>
                <input 
                  v-model="editForm.pov"
                  type="text"
                  placeholder="如: 主角视角、反派暗线视角..."
                  class="w-full bg-atelier-950 border border-atelier-750 rounded-lg px-3 py-2 text-xs text-ink-100 font-serif focus:outline-none focus:border-brand-amber/60" />
              </div>

              <div>
                <div class="flex items-center justify-between mb-1">
                  <label class="text-xs font-medium text-ink-300">张力目标 (1-10):</label>
                  <span class="font-mono text-xs font-bold" :class="tensionColorClass(editForm.tension_target)">
                    {{ editForm.tension_target }} / 10
                  </span>
                </div>
                <div class="flex items-center gap-3">
                  <input 
                    v-model.number="editForm.tension_target"
                    type="range"
                    min="1"
                    max="10"
                    step="1"
                    class="flex-1 accent-brand-amber cursor-pointer" />
                  <input 
                    v-model.number="editForm.tension_target"
                    type="number"
                    min="1"
                    max="10"
                    class="w-14 bg-atelier-950 border border-atelier-750 rounded-lg px-2 py-1 text-xs text-ink-100 font-mono text-center focus:outline-none focus:border-brand-amber/60" />
                </div>
              </div>
            </div>

            <!-- 情绪起伏曲线 (逗号分隔) -->
            <div>
              <label class="text-xs font-medium text-ink-300 block mb-1">
                情绪起伏曲线 (逗号分隔):
              </label>
              <input 
                v-model="editForm.emotion_curve"
                type="text"
                placeholder="例如: 期待, 意外打压, 蓄力咬牙, 爆发反击"
                class="w-full bg-atelier-950 border border-atelier-750 rounded-lg px-3 py-2 text-xs text-ink-100 font-serif focus:outline-none focus:border-brand-amber/60" />
            </div>

            <!-- 埋设伏笔 & 回收伏笔 (逗号分隔) -->
            <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <div>
                <label class="text-xs font-medium text-emerald-400 block mb-1">
                  埋设伏笔 (逗号分隔):
                </label>
                <input 
                  v-model="editForm.hook_plants"
                  type="text"
                  placeholder="例如: 破碎的黑石戒指, 师尊诡异的冷笑"
                  class="w-full bg-atelier-950 border border-atelier-750 rounded-lg px-3 py-2 text-xs text-ink-100 font-serif focus:outline-none focus:border-emerald-500/60" />
              </div>

              <div>
                <label class="text-xs font-medium text-brand-amber block mb-1">
                  回收伏笔 (逗号分隔):
                </label>
                <input 
                  v-model="editForm.hook_payoffs"
                  type="text"
                  placeholder="例如: 解开藏宝阁密图暗语, 揭穿二叔内鬼身份"
                  class="w-full bg-atelier-950 border border-atelier-750 rounded-lg px-3 py-2 text-xs text-ink-100 font-serif focus:outline-none focus:border-brand-amber/60" />
              </div>
            </div>

            <!-- 保存与取消操作 -->
            <div class="flex items-center justify-end gap-2.5 pt-3 border-t border-atelier-800">
              <button 
                @click="editingId = null"
                class="px-3.5 py-1.5 bg-atelier-850 hover:bg-atelier-800 text-ink-300 text-xs rounded-lg transition cursor-pointer">
                取消
              </button>
              <button 
                @click="saveIntent(intent)"
                :disabled="isSaving"
                class="px-4 py-1.5 bg-brand-amber hover:bg-brand-amber-hover text-atelier-950 font-bold text-xs rounded-lg shadow-amber-glow transition disabled:opacity-50 cursor-pointer flex items-center gap-1.5">
                <Save class="w-3.5 h-3.5" :class="{ 'animate-spin': isSaving }" />
                <span>{{ isSaving ? '保存中...' : '保存意图卡' }}</span>
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, computed, watch, onMounted } from 'vue';
import { state, actions, notify } from '../stores/appState';
import { api } from '../api/client';
import { 
  ListChecks, 
  RefreshCw, 
  Sparkles, 
  Layers, 
  Lock, 
  Unlock, 
  AlertCircle, 
  Edit3, 
  Trash2, 
  X, 
  Save, 
  ArrowRight, 
  Anchor, 
  CheckCircle2 
} from 'lucide-vue-next';

const intents = ref([]);
const isLoading = ref(false);
const isGenerating = ref(false);
const isSaving = ref(false);
const editingId = ref(null);

const editForm = reactive({
  conflict: '',
  pov: '',
  tension_target: 5,
  emotion_curve: '',
  hook_plants: '',
  hook_payoffs: '',
});

const volumeArcs = computed(() => {
  return state.currentProject?.framework?.volume_arcs || [];
});

const selectedVolumeIndex = ref(1);

const sortedIntents = computed(() => {
  return [...intents.value].sort((a, b) => (a.chapter_index || 0) - (b.chapter_index || 0));
});

function countByStatus(status) {
  return intents.value.filter(i => i.status === status).length;
}

function tensionColorClass(level) {
  if (level >= 8) return 'text-rose-400';
  if (level >= 5) return 'text-brand-amber';
  return 'text-emerald-400';
}

function tensionBarClass(level) {
  if (level >= 8) return 'bg-rose-500';
  if (level >= 5) return 'bg-brand-amber';
  return 'bg-emerald-500';
}

function parseCommaList(input) {
  if (Array.isArray(input)) return input;
  if (!input || typeof input !== 'string') return [];
  return input.split(/[,，]/).map(s => s.trim()).filter(Boolean);
}

async function loadIntents() {
  if (!state.currentProject) {
    intents.value = [];
    return;
  }
  isLoading.value = true;
  try {
    const vol = selectedVolumeIndex.value || 1;
    const res = await api.listChapterIntents(state.currentProject.id, vol);
    intents.value = Array.isArray(res) ? res : [];
  } catch (err) {
    notify('加载章节意图卡失败', err.message || '网络或服务端异常', 'error');
    intents.value = [];
  } finally {
    isLoading.value = false;
  }
}

async function generateIntents() {
  if (!state.currentProject) return;
  if (!selectedVolumeIndex.value) {
    notify('提示', '请先选择需要推演的卷', 'warning');
    return;
  }

  isGenerating.value = true;
  try {
    const res = await api.generateVolumeIntents(state.currentProject.id, {
      volume_index: Number(selectedVolumeIndex.value)
    });
    const count = res?.created ?? (res?.intents?.length || 0);
    notify('AI 推演完成', `新增 ${count} 张意图卡`, 'success');
    await loadIntents();
  } catch (err) {
    notify('AI 推演失败', err.message || '服务端异常，请稍后重试', 'error');
  } finally {
    isGenerating.value = false;
  }
}

function toggleEdit(intent) {
  if (editingId.value === intent.chapter_index) {
    editingId.value = null;
    return;
  }
  editingId.value = intent.chapter_index;
  editForm.conflict = intent.conflict || '';
  editForm.pov = intent.pov || '';
  editForm.tension_target = intent.tension_target || 5;
  editForm.emotion_curve = (intent.emotion_curve || []).join(', ');
  editForm.hook_plants = (intent.hook_plants || []).join(', ');
  editForm.hook_payoffs = (intent.hook_payoffs || []).join(', ');
}

async function saveIntent(intent) {
  if (!state.currentProject) return;
  if (!editForm.conflict.trim()) {
    notify('校验失败', '核心戏剧冲突不能为空', 'warning');
    return;
  }

  isSaving.value = true;
  try {
    const payload = {
      project_id: state.currentProject.id,
      chapter_index: intent.chapter_index,
      volume_index: intent.volume_index || selectedVolumeIndex.value,
      conflict: editForm.conflict.trim(),
      pov: editForm.pov.trim(),
      tension_target: Number(editForm.tension_target) || 5,
      emotion_curve: parseCommaList(editForm.emotion_curve),
      hook_plants: parseCommaList(editForm.hook_plants),
      hook_payoffs: parseCommaList(editForm.hook_payoffs),
      status: intent.status || 'DRAFT', // 显式带上当前卡 status，防止编辑后意外降级 LOCKED 卡
    };
    await api.saveChapterIntent(state.currentProject.id, intent.chapter_index, payload);
    notify('保存成功', `第 ${intent.chapter_index} 章意图卡已保存`, 'success');
    editingId.value = null;
    await loadIntents();
  } catch (err) {
    notify('保存失败', err.message || '网络或服务端异常', 'error');
  } finally {
    isSaving.value = false;
  }
}

async function toggleLockIntent(intent) {
  if (!state.currentProject) return;
  const targetStatus = intent.status === 'LOCKED' ? 'DRAFT' : 'LOCKED';
  const actionName = targetStatus === 'LOCKED' ? '锁定' : '解锁';

  try {
    const payload = {
      project_id: state.currentProject.id,
      chapter_index: intent.chapter_index,
      volume_index: intent.volume_index || selectedVolumeIndex.value,
      conflict: intent.conflict || '',
      pov: intent.pov || '',
      tension_target: intent.tension_target || 5,
      emotion_curve: intent.emotion_curve || [],
      hook_plants: intent.hook_plants || [],
      hook_payoffs: intent.hook_payoffs || [],
      status: targetStatus,
    };
    await api.saveChapterIntent(state.currentProject.id, intent.chapter_index, payload);
    notify(`${actionName}成功`, `第 ${intent.chapter_index} 章意图卡已${actionName}`, 'success');
    await loadIntents();
  } catch (err) {
    // 提交 LOCKED 失败时展示后端 400 文案
    notify(`${actionName}失败`, err.message || '服务端拒绝了状态变更', 'error');
  }
}

async function deleteIntent(intent) {
  if (!state.currentProject) return;
  const ok = await actions.confirm({
    title: '删除意图卡',
    message: `确定要删除第 ${intent.chapter_index} 章的章节意图卡吗？此操作无法撤销。`,
    type: 'danger'
  });
  if (!ok) return;

  try {
    await api.deleteChapterIntent(state.currentProject.id, intent.chapter_index);
    notify('已删除', `第 ${intent.chapter_index} 章意图卡已删除`, 'info');
    await loadIntents();
  } catch (err) {
    notify('删除失败', err.message || '未知错误', 'error');
  }
}

// 监听项目切换
watch(
  () => state.selectedProjectId,
  () => {
    if (volumeArcs.value.length > 0) {
      selectedVolumeIndex.value = volumeArcs.value[0].volume_index || 1;
    } else {
      selectedVolumeIndex.value = 1;
    }
    loadIntents();
  },
  { immediate: true }
);

// 监听分卷规划变化
watch(
  () => volumeArcs.value,
  (newArcs) => {
    if (newArcs && newArcs.length > 0) {
      const exists = newArcs.some(a => (a.volume_index || 1) === selectedVolumeIndex.value);
      if (!exists) {
        selectedVolumeIndex.value = newArcs[0].volume_index || 1;
        loadIntents();
      }
    }
  }
);

onMounted(() => {
  loadIntents();
});
</script>
