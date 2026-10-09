<template>
  <div 
    v-if="state.showMessageCenterModal" 
    class="fixed inset-0 z-50 bg-black/60 backdrop-blur-sm flex justify-end transition-opacity duration-200 select-none animate-fade-in"
    @click.self="state.showMessageCenterModal = false">
    
    <div 
      class="w-full max-w-lg md:max-w-xl h-full bg-atelier-900 border-l border-atelier-750 shadow-atelier-xl flex flex-col justify-between overflow-hidden animate-slide-left">
      
      <!-- 抽屉顶栏 -->
      <div class="p-4 border-b border-atelier-750 bg-atelier-950/70 flex items-center justify-between shrink-0">
        <div class="flex items-center gap-2.5">
          <div class="w-7 h-7 rounded-lg bg-brand-amber/15 border border-brand-amber/30 flex items-center justify-center text-brand-amber">
            <Bell class="w-4 h-4" />
          </div>
          <div>
            <div class="flex items-center gap-2">
              <h3 class="text-sm font-bold font-serif text-ink-100">消息与任务中心</h3>
              <span 
                v-if="computedState.unreadMessageCount.value > 0"
                class="px-1.5 py-0.2 rounded-full text-[10px] font-mono font-bold bg-brand-rose text-white">
                {{ computedState.unreadMessageCount.value }} 未读
              </span>
            </div>
            <p class="text-[10px] text-ink-400">实时追踪后台推演进度、质检结果与系统通知</p>
          </div>
        </div>

        <div class="flex items-center gap-1.5">
          <!-- 桌面系统通知授权开关 -->
          <button 
            @click="handleToggleNotification" 
            class="px-2 py-1 text-[11px] rounded border transition flex items-center gap-1 cursor-pointer"
            :class="isNotificationGranted 
              ? (state.enableSystemNotifications ? 'bg-brand-emerald/15 text-brand-emerald border-brand-emerald/40' : 'bg-atelier-850 text-ink-400 border-atelier-700') 
              : 'bg-brand-amber/15 text-brand-amber border-brand-amber/40 hover:bg-brand-amber/25'"
            :title="isNotificationGranted ? '点击切换桌面弹窗通知' : '点击授权浏览器/操作系统桌面原生弹窗通知'">
            <BellRing v-if="isNotificationGranted && state.enableSystemNotifications" class="w-3 h-3 text-brand-emerald" />
            <BellOff v-else-if="isNotificationGranted" class="w-3 h-3 text-ink-400" />
            <Sparkles v-else class="w-3 h-3 text-brand-amber" />
            <span>{{ isNotificationGranted ? (state.enableSystemNotifications ? '桌面通知: 开' : '桌面通知: 关') : '开启桌面通知' }}</span>
          </button>

          <!-- 一键已读 -->
          <button 
            v-if="computedState.unreadMessageCount.value > 0"
            @click="actions.markAllMessagesRead" 
            class="p-1.5 text-ink-400 hover:text-ink-100 hover:bg-atelier-800 rounded transition cursor-pointer"
            title="全部标为已读">
            <CheckCheck class="w-4 h-4" />
          </button>

          <!-- 清空全部消息 -->
          <button 
            v-if="state.messages.length > 0"
            @click="confirmClearAll" 
            class="p-1.5 text-ink-400 hover:text-brand-rose hover:bg-atelier-800 rounded transition cursor-pointer"
            title="清空消息记录">
            <Trash2 class="w-4 h-4" />
          </button>

          <!-- 关闭抽屉 -->
          <button 
            @click="state.showMessageCenterModal = false" 
            class="p-1.5 text-ink-400 hover:text-ink-100 hover:bg-atelier-800 rounded transition cursor-pointer ml-1">
            <X class="w-4 h-4" />
          </button>
        </div>
      </div>

      <!-- 选项卡过滤条 -->
      <div class="px-4 py-2 border-b border-atelier-750 bg-atelier-950/40 flex items-center justify-between text-xs shrink-0">
        <div class="flex items-center gap-1">
          <button 
            v-for="f in filterTabs" 
            :key="f.id"
            @click="currentFilter = f.id"
            class="px-2.5 py-1 rounded-md text-[11px] font-medium transition cursor-pointer flex items-center gap-1.5"
            :class="currentFilter === f.id 
              ? 'bg-atelier-800 text-brand-amber shadow-atelier-sm font-semibold border border-atelier-700' 
              : 'text-ink-400 hover:text-ink-200 hover:bg-atelier-850'">
            <span>{{ f.label }}</span>
            <span 
              v-if="f.count > 0" 
              class="px-1.5 py-0.2 rounded-full text-[9px] font-mono"
              :class="currentFilter === f.id ? 'bg-brand-amber/20 text-brand-amber' : 'bg-atelier-800 text-ink-400'">
              {{ f.count }}
            </span>
          </button>
        </div>
        
        <span class="text-[10px] font-mono text-ink-500">共 {{ state.messages.length }} 条记录</span>
      </div>

      <!-- 主体消息列表与异步运行任务 -->
      <div class="flex-1 overflow-y-auto p-4 space-y-4">
        
        <!-- 正在执行的异步任务实时卡片区 -->
        <div v-if="computedState.activeTasks.value.length > 0" class="space-y-2">
          <div class="flex items-center justify-between text-xs">
            <span class="font-bold text-brand-amber flex items-center gap-1.5">
              <span class="relative flex h-2 w-2">
                <span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-brand-amber opacity-75"></span>
                <span class="relative inline-flex rounded-full h-2 w-2 bg-brand-amber"></span>
              </span>
              <span>正在进行的异步推演任务 ({{ computedState.activeTasks.value.length }})</span>
            </span>
            <span class="text-[10px] text-ink-400 font-mono">实时计算中</span>
          </div>

          <div 
            v-for="task in computedState.activeTasks.value" 
            :key="task.id"
            class="p-3.5 rounded-xl bg-atelier-950 border border-brand-amber/40 shadow-amber-glow/20 space-y-2 animate-subtle-pulse">
            <div class="flex items-center justify-between">
              <div class="flex items-center gap-2">
                <Loader2 class="w-4 h-4 text-brand-amber animate-spin" />
                <span class="text-xs font-bold text-ink-100">{{ task.title }}</span>
              </div>
              <span class="text-[10px] font-mono text-brand-amber bg-brand-amber/15 px-2 py-0.5 rounded border border-brand-amber/30">
                RUNNING
              </span>
            </div>
            
            <p class="text-[11px] text-ink-300 leading-relaxed font-sans">{{ task.desc }}</p>
            
            <!-- 运行进度动效条 -->
            <div class="w-full bg-atelier-800 h-1.5 rounded-full overflow-hidden relative">
              <div class="h-full bg-gradient-to-r from-brand-amber to-amber-400 animate-pulse rounded-full w-full"></div>
            </div>
          </div>
        </div>

        <!-- 历史消息记录列表 -->
        <div v-if="filteredMessages.length > 0" class="space-y-2.5">
          <div 
            v-for="msg in filteredMessages" 
            :key="msg.id"
            @click="actions.markMessageRead(msg.id)"
            class="p-3.5 rounded-xl border transition group cursor-pointer relative"
            :class="[
              msg.read ? 'bg-atelier-950/60 border-atelier-800 hover:border-atelier-750' : 'bg-atelier-950 border-atelier-700 hover:border-brand-amber/40 shadow-atelier-sm',
              msg.type === 'error' ? 'border-l-4 border-l-brand-rose' : '',
              msg.type === 'warning' ? 'border-l-4 border-l-brand-amber' : '',
              msg.type === 'success' ? 'border-l-4 border-l-brand-emerald' : '',
              msg.type === 'info' ? 'border-l-4 border-l-brand-cyan' : '',
            ]">
            
            <div class="flex items-start justify-between gap-3">
              <div class="flex items-start gap-2.5 min-w-0">
                <!-- 状态类型徽标 -->
                <div class="mt-0.5 shrink-0">
                  <CheckCircle2 v-if="msg.type === 'success'" class="w-4 h-4 text-brand-emerald" />
                  <AlertCircle v-else-if="msg.type === 'error'" class="w-4 h-4 text-brand-rose" />
                  <AlertTriangle v-else-if="msg.type === 'warning'" class="w-4 h-4 text-brand-amber" />
                  <Info v-else class="w-4 h-4 text-brand-cyan" />
                </div>

                <div class="min-w-0">
                  <div class="flex items-center gap-2">
                    <h4 class="text-xs font-semibold text-ink-100 truncate" :class="{ 'font-bold': !msg.read }">
                      {{ msg.title }}
                    </h4>
                    <span v-if="!msg.read" class="w-1.5 h-1.5 rounded-full bg-brand-amber shrink-0"></span>
                  </div>
                  <p class="text-[11px] text-ink-300 mt-1 leading-relaxed whitespace-pre-wrap select-text font-sans">
                    {{ msg.message }}
                  </p>
                </div>
              </div>

              <!-- 右侧时间与快捷删除 -->
              <div class="flex flex-col items-end shrink-0 gap-1.5">
                <span class="text-[10px] font-mono text-ink-500 whitespace-nowrap">
                  {{ formatTime(msg.timestamp) }}
                </span>
                <div class="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition">
                  <button 
                    @click.stop="copyMessageText(msg)" 
                    class="p-1 text-ink-400 hover:text-ink-200 hover:bg-atelier-850 rounded"
                    title="复制内容">
                    <Copy class="w-3 h-3" />
                  </button>
                  <button 
                    @click.stop="actions.removeMessage(msg.id)" 
                    class="p-1 text-ink-400 hover:text-brand-rose hover:bg-atelier-850 rounded"
                    title="删除">
                    <X class="w-3 h-3" />
                  </button>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- 空状态 -->
        <div 
          v-else-if="computedState.activeTasks.value.length === 0" 
          class="h-64 flex flex-col items-center justify-center text-center p-6 space-y-3">
          <div class="w-12 h-12 rounded-full bg-atelier-850 border border-atelier-750 flex items-center justify-center text-ink-500">
            <Inbox class="w-6 h-6" />
          </div>
          <div class="space-y-1">
            <p class="text-xs font-medium text-ink-300">暂无该分类通知记录</p>
            <p class="text-[11px] text-ink-500 max-w-xs">
              系统将自动记录所有正文渲染、因果推演、反AI味质检与自动闭环任务。
            </p>
          </div>
        </div>
      </div>

      <!-- 抽屉底栏状态信息 -->
      <div class="p-3 border-t border-atelier-750 bg-atelier-950/70 flex items-center justify-between text-[11px] text-ink-400 shrink-0">
        <div class="flex items-center gap-1.5 font-mono text-[10px]">
          <span class="w-2 h-2 rounded-full" :class="isNotificationGranted ? 'bg-brand-emerald' : 'bg-brand-amber'"></span>
          <span>系统通知状态: {{ notificationStatusText }}</span>
        </div>
        <button 
          @click="state.showMessageCenterModal = false" 
          class="px-3 py-1 bg-atelier-850 hover:bg-atelier-800 text-ink-300 rounded text-xs transition cursor-pointer">
          关闭窗口
        </button>
      </div>

    </div>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue';
import { state, computedState, actions, notify, dialogs, requestNotificationPermission } from '../../stores/appState';
import { 
  Bell, 
  BellRing, 
  BellOff, 
  CheckCheck, 
  Trash2, 
  X, 
  Sparkles, 
  Loader2, 
  CheckCircle2, 
  AlertCircle, 
  AlertTriangle, 
  Info, 
  Inbox,
  Copy
} from 'lucide-vue-next';

const currentFilter = ref('all');

const isNotificationGranted = computed(() => {
  return state.systemNotificationPermission === 'granted';
});

const notificationStatusText = computed(() => {
  if (state.systemNotificationPermission === 'granted') {
    return state.enableSystemNotifications ? '已开启并生效' : '已静音暂不提醒';
  } else if (state.systemNotificationPermission === 'denied') {
    return '系统已禁止权限';
  }
  return '未申请桌面权限';
});

const filterTabs = computed(() => [
  { id: 'all', label: '全部', count: state.messages.length },
  { id: 'tasks', label: '运行中', count: computedState.activeTasks.value.length },
  { id: 'unread', label: '未读', count: computedState.unreadMessageCount.value },
  { id: 'errors', label: '报错与告警', count: state.messages.filter(m => m.type === 'error' || m.type === 'warning').length },
]);

const filteredMessages = computed(() => {
  if (currentFilter.value === 'unread') {
    return state.messages.filter(m => !m.read);
  } else if (currentFilter.value === 'errors') {
    return state.messages.filter(m => m.type === 'error' || m.type === 'warning');
  } else if (currentFilter.value === 'tasks') {
    return state.messages.filter(m => m.taskType);
  }
  return state.messages;
});

async function handleToggleNotification() {
  if (!isNotificationGranted.value) {
    await requestNotificationPermission();
  } else {
    state.enableSystemNotifications = !state.enableSystemNotifications;
    notify(
      state.enableSystemNotifications ? '桌面系统通知已恢复' : '桌面系统通知已暂停',
      state.enableSystemNotifications ? '后台推演完成时将弹出系统通知' : '已转为应用内静音记录',
      'info',
      2500
    );
  }
}

async function confirmClearAll() {
  const ok = await dialogs.confirm({
    title: '清空全部消息',
    message: '确定清空所有历史通知记录吗？此操作无法撤销。',
    type: 'warning',
    confirmText: '确认清空',
  });
  if (ok) {
    actions.clearAllMessages();
    notify('消息记录已清空', '', 'info', 2000);
  }
}

function copyMessageText(msg) {
  const text = `${msg.title}\n${msg.message || ''}`;
  navigator.clipboard.writeText(text);
  notify('已复制到剪贴板', text.slice(0, 40) + '...', 'info', 1500);
}

function formatTime(isoStr) {
  if (!isoStr) return '';
  const d = new Date(isoStr);
  const now = new Date();
  const isToday = d.toDateString() === now.toDateString();
  const time = `${d.getHours().toString().padStart(2, '0')}:${d.getMinutes().toString().padStart(2, '0')}:${d.getSeconds().toString().padStart(2, '0')}`;
  if (isToday) return time;
  return `${d.getMonth() + 1}/${d.getDate()} ${time}`;
}
</script>
