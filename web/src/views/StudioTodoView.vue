<template>
  <div class="flex-1 p-6 overflow-y-auto space-y-6 bg-atelier-950">
    <!-- 头部工具栏 -->
    <div class="flex flex-col md:flex-row md:items-center justify-between gap-4 border-b border-atelier-750 pb-5">
      <div>
        <div class="flex items-center gap-2.5">
          <div class="w-8 h-8 rounded-lg bg-amber-500/10 border border-amber-500/25 flex items-center justify-center text-brand-amber shadow-amber-glow">
            <AlertTriangle class="w-4 h-4" />
          </div>
          <div>
            <div class="flex items-center gap-2">
              <h2 class="text-base font-serif font-bold text-ink-50 tracking-wide">停线待办中心</h2>
              <span 
                v-if="todos.length > 0" 
                class="text-[11px] font-mono px-2 py-0.5 rounded-full bg-rose-500/15 text-rose-400 border border-rose-500/30">
                {{ todos.length }} 项待介入
              </span>
            </div>
            <p class="text-xs text-ink-400 mt-0.5">汇总全自动生产线中因质检不达标、循环超限或意图冲突而挂起的章节。</p>
          </div>
        </div>
      </div>

      <div class="flex items-center gap-2.5">
        <button 
          @click="fetchTodos" 
          :disabled="isLoading"
          class="flex items-center gap-1.5 px-3 py-1.5 bg-atelier-850 hover:bg-atelier-800 text-ink-300 hover:text-ink-100 text-xs rounded-md border border-atelier-750 transition cursor-pointer disabled:opacity-50"
          title="刷新待办列表">
          <RefreshCw class="w-3.5 h-3.5 text-ink-400" :class="{ 'animate-spin': isLoading }" />
          <span>{{ isLoading ? '刷新中...' : '刷新待办' }}</span>
        </button>
      </div>
    </div>

    <!-- 异常错误提示栏 (防白屏，不使用假数据) -->
    <div 
      v-if="errorMessage" 
      class="p-4 bg-rose-500/10 border border-rose-500/30 rounded-xl flex items-center justify-between gap-3 text-xs text-rose-300">
      <div class="flex items-center gap-2">
        <AlertTriangle class="w-4 h-4 text-rose-400 shrink-0" />
        <span>{{ errorMessage }}</span>
      </div>
      <button 
        @click="fetchTodos" 
        class="px-2.5 py-1 bg-rose-500/20 hover:bg-rose-500/30 text-rose-200 rounded font-medium transition cursor-pointer">
        重试连接
      </button>
    </div>

    <!-- 加载中指示器 -->
    <div v-if="isLoading && todos.length === 0" class="py-16 text-center text-xs text-ink-400 space-y-2">
      <span class="inline-block w-5 h-5 border-2 border-brand-amber border-t-transparent rounded-full animate-spin"></span>
      <p class="text-[11px] font-mono text-ink-500">正在检查停线待办事项...</p>
    </div>

    <!-- 空态：无停线待办 -->
    <div 
      v-else-if="!errorMessage && todos.length === 0" 
      class="p-12 text-center bg-atelier-900 border border-atelier-750 rounded-xl max-w-xl mx-auto space-y-4">
      <div class="w-12 h-12 mx-auto rounded-full bg-emerald-500/10 border border-emerald-500/25 flex items-center justify-center text-emerald-400">
        <CheckCircle2 class="w-6 h-6" />
      </div>
      <div class="space-y-1">
        <h3 class="text-sm font-semibold text-ink-100 font-serif">暂无停线待办</h3>
        <p class="text-xs text-ink-400 leading-relaxed max-w-sm mx-auto">
          所有章节均在自动闭环内推进，生产流水线状态健康，无需人工介入。
        </p>
      </div>
      <div class="flex items-center justify-center gap-3 pt-2">
        <button 
          @click="actions.switchToProject(null, 'workbench')" 
          class="px-4 py-2 bg-brand-amber hover:bg-brand-amber-hover text-atelier-950 font-bold text-xs rounded-lg shadow-amber-glow transition cursor-pointer">
          前往工作台创作
        </button>
        <button 
          @click="actions.switchToGlobal('studio-automation')" 
          class="px-4 py-2 bg-atelier-850 hover:bg-atelier-800 text-ink-300 hover:text-ink-100 text-xs rounded-lg border border-atelier-700 transition cursor-pointer">
          查看自动化排程
        </button>
      </div>
    </div>

    <!-- 待办列表 -->
    <div v-else-if="todos.length > 0" class="space-y-3 max-w-4xl">
      <div 
        v-for="(todo, index) in todos" 
        :key="`${todo.project_id}_${todo.chapter_index}_${index}`"
        class="p-4 bg-atelier-900 border border-atelier-750 rounded-xl hover:border-brand-amber/40 transition shadow-atelier-sm space-y-3">
        
        <!-- 卡片头部信息 -->
        <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-2.5">
          <div class="flex items-center gap-2 flex-wrap">
            <span class="text-sm font-serif font-bold text-ink-50">
              {{ todo.project_title || '未命名作品' }}
            </span>
            <span class="px-2 py-0.5 rounded text-[11px] font-mono font-medium bg-brand-amber/15 text-brand-amber border border-brand-amber/30">
              第 {{ todo.chapter_index }} 章
            </span>
            <span 
              v-if="todo.verdict" 
              class="px-2 py-0.5 rounded text-[11px] font-mono font-semibold bg-rose-500/15 text-rose-400 border border-rose-500/30">
              {{ todo.verdict }}
            </span>
            <span 
              v-if="todo.score !== undefined && todo.score !== null" 
              class="px-2 py-0.5 rounded text-[11px] font-mono bg-atelier-800 text-ink-300 border border-atelier-700">
              评分: {{ todo.score }}
            </span>
          </div>

          <div class="flex items-center gap-3 self-end sm:self-auto">
            <span class="text-[11px] font-mono text-ink-400">
              {{ formatRelativeTime(todo.updated_at) }}
            </span>
            <button 
              @click="handleTodo(todo)"
              class="px-3 py-1.5 bg-brand-amber hover:bg-brand-amber-hover text-atelier-950 font-bold text-xs rounded-md shadow-amber-glow transition cursor-pointer flex items-center gap-1">
              <span>去处理</span>
              <ArrowUpRight class="w-3.5 h-3.5" />
            </button>
          </div>
        </div>

        <!-- 卡片主体：问题项列表 -->
        <div class="pt-2 border-t border-atelier-800">
          <div class="text-[11px] font-mono text-ink-400 mb-1.5 flex items-center gap-1">
            <AlertCircle class="w-3 h-3 text-rose-400" />
            <span>停线原因与质检问题清单：</span>
          </div>

          <ul v-if="Array.isArray(todo.issues) && todo.issues.length > 0" class="space-y-1 pl-1">
            <li 
              v-for="(issue, iIdx) in todo.issues" 
              :key="iIdx"
              class="text-xs text-rose-300/90 flex items-start gap-1.5 leading-relaxed">
              <span class="text-rose-400 select-none">•</span>
              <span>{{ issue }}</span>
            </li>
          </ul>
          <p v-else class="text-xs text-ink-500 italic pl-1">
            未列出具体问题明细（达到重写上限或意图未锁定）。
          </p>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue';
import { state, actions } from '../stores/appState';
import { api } from '../api/client';
import {
  AlertTriangle,
  RefreshCw,
  CheckCircle2,
  AlertCircle,
  ArrowUpRight
} from 'lucide-vue-next';

const isLoading = ref(false);
const errorMessage = ref('');
const todos = ref([]);

// 解析时间戳/字符串
function parseDate(dateInput) {
  if (!dateInput) return null;
  if (dateInput instanceof Date) return dateInput;
  if (typeof dateInput === 'number') {
    return new Date(dateInput > 1e11 ? dateInput : dateInput * 1000);
  }
  if (typeof dateInput === 'string' && /^\d+$/.test(dateInput)) {
    const num = Number(dateInput);
    return new Date(num > 1e11 ? num : num * 1000);
  }
  const parsed = new Date(dateInput);
  return isNaN(parsed.getTime()) ? null : parsed;
}

// 相对时间格式化
function formatRelativeTime(dateInput) {
  const date = parseDate(dateInput);
  if (!date) return dateInput ? String(dateInput) : '刚刚';

  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffSec = Math.floor(diffMs / 1000);

  if (diffSec < 60) return '刚刚';
  const diffMin = Math.floor(diffSec / 60);
  if (diffMin < 60) return `${diffMin} 分钟前`;
  const diffHour = Math.floor(diffMin / 60);
  if (diffHour < 24) return `${diffHour} 小时前`;
  const diffDay = Math.floor(diffHour / 24);
  if (diffDay < 7) return `${diffDay} 天前`;
  if (diffDay < 30) return `${Math.floor(diffDay / 7)} 周前`;

  const y = date.getFullYear();
  const m = String(date.getMonth() + 1).padStart(2, '0');
  const d = String(date.getDate()).padStart(2, '0');
  return `${y}-${m}-${d}`;
}

async function fetchTodos() {
  isLoading.value = true;
  errorMessage.value = '';

  try {
    const res = await api.listStudioTodos();
    todos.value = Array.isArray(res?.todos) ? res.todos : [];
  } catch (err) {
    console.error('StudioTodoView fetchTodos error:', err);
    errorMessage.value = `获取待办失败: ${err.message || '网络连接超时或后端未就绪'}`;
    todos.value = [];
  } finally {
    isLoading.value = false;
  }
}

// 去处理：切换当前选中项目并跳转到 workbench，载入停线章节的在途草稿
async function handleTodo(todo) {
  if (todo?.project_id) {
    // Fail-Closed：切换被拒绝（例如当前作品正在生成）时保持原地。
    const ok = await actions.selectProject(todo.project_id);
    if (!ok) return;
  }
  if (todo?.chapter_index) {
    // 先把当前在途内容落盘，再载入待办章节的停线草稿与体检建议
    if (state.workbench.content.trim() || state.workbench.coreConflict.trim()) {
      if (actions.saveCheckpoint) await actions.saveCheckpoint({}, true);
    }
    state.editingChapterIndex = todo.chapter_index;
    if (actions.restoreCheckpoint) {
      await actions.restoreCheckpoint(todo.chapter_index);
    }
    state.activeStep = 5;
  }
  actions.setActiveTab('workbench');
}

onMounted(() => {
  fetchTodos();
});
</script>
