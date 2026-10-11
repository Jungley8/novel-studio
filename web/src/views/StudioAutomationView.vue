<template>
  <div class="flex-1 p-6 md:p-8 overflow-y-auto w-full max-w-6xl mx-auto space-y-8 bg-atelier-950">
    <!-- 头部工具栏 -->
    <div class="flex flex-col md:flex-row md:items-center justify-between gap-4 border-b border-atelier-750 pb-5">
      <div>
        <div class="flex items-center gap-2.5">
          <div class="w-8 h-8 rounded-lg bg-brand-amber/10 border border-brand-amber/25 flex items-center justify-center text-brand-amber shadow-amber-glow">
            <CalendarClock class="w-4 h-4" />
          </div>
          <div>
            <div class="flex items-center gap-2">
              <h2 class="text-base font-serif font-bold text-ink-50 tracking-wide">自动化排程中心</h2>
              <span class="text-[10px] font-mono px-2 py-0.5 rounded-full bg-brand-amber/15 text-brand-amber border border-brand-amber/30">
                无人值守 · 调度流水线
              </span>
            </div>
            <p class="text-xs text-ink-400 mt-0.5">管理已有作品的定时续产排程与新书自动开书批产队列。</p>
          </div>
        </div>
      </div>

      <div class="flex items-center gap-2.5">
        <button 
          @click="refreshAll" 
          :disabled="isLoading"
          class="flex items-center gap-1.5 px-3 py-1.5 bg-atelier-850 hover:bg-atelier-800 text-ink-300 hover:text-ink-100 text-xs rounded-md border border-atelier-750 transition cursor-pointer disabled:opacity-50"
          title="刷新全部排程与队列">
          <RefreshCw class="w-3.5 h-3.5 text-ink-400" :class="{ 'animate-spin': isLoading }" />
          <span>{{ isLoading ? '刷新中...' : '刷新数据' }}</span>
        </button>
      </div>
    </div>

    <!-- 全局异常告警栏 -->
    <div 
      v-if="errorMessage" 
      class="p-4 bg-rose-500/10 border border-rose-500/30 rounded-xl flex items-center justify-between gap-3 text-xs text-rose-300">
      <div class="flex items-center gap-2">
        <AlertTriangle class="w-4 h-4 text-rose-400 shrink-0" />
        <span>{{ errorMessage }}</span>
      </div>
      <button 
        @click="refreshAll" 
        class="px-2.5 py-1 bg-rose-500/20 hover:bg-rose-500/30 text-rose-200 rounded font-medium transition cursor-pointer">
        重试连接
      </button>
    </div>

    <!-- ==================== 区块一：已有作品定时续产排程 ==================== -->
    <section class="space-y-4">
      <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-2">
        <div class="flex items-center gap-2">
          <Clock class="w-4 h-4 text-brand-amber" />
          <h3 class="text-sm font-serif font-bold text-ink-100">作品定时续产排程</h3>
          <span 
            v-if="automations.length > 0" 
            class="text-[10px] font-mono px-1.5 py-0.2 rounded bg-atelier-800 text-ink-300 border border-atelier-700">
            {{ automations.length }} 部作品
          </span>
        </div>
        <p class="text-[11px] text-ink-400">设置每部作品自动触发的时刻与批产配额；遇质检或意图停线将自动暂停待人工介入。</p>
      </div>

      <!-- 作品排程加载中 -->
      <div v-if="isLoadingAutomations && automations.length === 0" class="py-12 text-center text-xs text-ink-400 space-y-2">
        <span class="inline-block w-5 h-5 border-2 border-brand-amber border-t-transparent rounded-full animate-spin"></span>
        <p class="text-[11px] font-mono text-ink-500">正在获取作品自动化排程配置...</p>
      </div>

      <!-- 作品排程空状态 -->
      <div 
        v-else-if="automations.length === 0" 
        class="p-8 text-center bg-atelier-900 border border-atelier-750 rounded-xl space-y-2">
        <CalendarClock class="w-8 h-8 text-ink-500 mx-auto" />
        <h4 class="text-xs font-semibold text-ink-200">暂无作品排程</h4>
        <p class="text-[11px] text-ink-400">尚未检测到可配置的长篇小说工程，请先在左侧新建一部作品。</p>
      </div>

      <!-- 作品排程列表卡片 -->
      <div v-else class="space-y-3">
        <div 
          v-for="item in automations" 
          :key="item.project_id"
          class="p-4 bg-atelier-900 border border-atelier-750 rounded-xl hover:border-atelier-700 transition shadow-atelier-sm space-y-3">
          
          <!-- 卡片顶栏：作品标题、状态徽章、最近运行 -->
          <div class="flex flex-col lg:flex-row lg:items-center justify-between gap-2.5 pb-2.5 border-b border-atelier-800">
            <div class="flex items-center gap-2.5 flex-wrap">
              <span class="text-sm font-serif font-bold text-ink-50">
                {{ item.project_title || '未命名作品' }}
              </span>
              <span class="text-[10px] font-mono text-ink-400 bg-atelier-950 px-1.5 py-0.5 rounded border border-atelier-800">
                {{ item.project_id }}
              </span>

              <!-- 停线暂停徽章 -->
              <span 
                v-if="item.paused" 
                class="px-2 py-0.5 rounded text-[11px] font-medium bg-rose-500/15 text-rose-400 border border-rose-500/30 flex items-center gap-1">
                <AlertCircle class="w-3 h-3" />
                <span>已暂停：{{ item.paused_reason || '质检停线' }}</span>
              </span>

              <!-- 启用状态徽章 -->
              <span 
                v-else-if="automationEditMap[item.project_id]?.enabled" 
                class="px-2 py-0.5 rounded text-[11px] font-medium bg-emerald-500/15 text-emerald-400 border border-emerald-500/30 flex items-center gap-1">
                <CheckCircle2 class="w-3 h-3" />
                <span>排程运行中</span>
              </span>
              <span 
                v-else 
                class="px-2 py-0.5 rounded text-[11px] font-medium bg-atelier-800 text-ink-400 border border-atelier-700">
                未启用
              </span>
            </div>

            <!-- 右侧最近执行信息 & 恢复排程按钮 -->
            <div class="flex items-center gap-2.5 flex-wrap self-end lg:self-auto">
              <!-- 最近执行态 -->
              <div v-if="item.last_run_at || item.last_status" class="text-[11px] font-mono text-ink-400 flex items-center gap-1.5">
                <span 
                  v-if="item.last_status"
                  :class="getAutomationStatusBadge(item.last_status).class"
                  class="px-1.5 py-0.2 rounded text-[10px] font-medium border">
                  {{ getAutomationStatusBadge(item.last_status).label }}
                </span>
                <span v-if="item.last_run_at">
                  最近执行: {{ formatRelativeTime(item.last_run_at) }}
                </span>
                <span v-if="item.last_message" class="text-ink-400 max-w-[200px] truncate" :title="item.last_message">
                  ({{ item.last_message }})
                </span>
              </div>

              <!-- 恢复排程按钮 (仅 paused 态展示) -->
              <button 
                v-if="item.paused"
                @click="handleResumeAutomation(item.project_id)"
                :disabled="automationResumingMap[item.project_id]"
                class="px-2.5 py-1 bg-amber-500/15 hover:bg-amber-500/25 text-brand-amber border border-amber-500/30 font-medium text-xs rounded transition cursor-pointer flex items-center gap-1 disabled:opacity-50"
                title="清除暂停标记并恢复自动排程">
                <Play class="w-3 h-3" />
                <span>{{ automationResumingMap[item.project_id] ? '恢复中...' : '恢复排程' }}</span>
              </button>
            </div>
          </div>

          <!-- 卡片配置行：启用开关、触发时刻、配额、保存 -->
          <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-12 gap-3 items-end pt-1">
            <!-- 启用开关 -->
            <div class="lg:col-span-2 flex items-center h-8">
              <label class="flex items-center gap-2 cursor-pointer select-none">
                <input 
                  type="checkbox" 
                  v-model="automationEditMap[item.project_id].enabled" 
                  @change="handleSaveAutomation(item.project_id)"
                  class="rounded border-atelier-700 bg-atelier-950 text-brand-amber accent-brand-amber">
                <span class="text-xs font-medium text-ink-200">启用自动续产</span>
              </label>
            </div>

            <!-- 触发时刻输入 -->
            <div class="lg:col-span-5">
              <label class="block text-[11px] text-ink-400 mb-1">
                触发时刻 (HH:MM，逗号分隔)：
              </label>
              <input 
                type="text" 
                v-model="automationEditMap[item.project_id].timesStr" 
                placeholder="例如 02:00, 14:00"
                class="w-full bg-atelier-950 border border-atelier-750 rounded-md px-2.5 py-1 text-xs text-ink-100 font-mono focus:outline-none focus:border-brand-amber/60">
            </div>

            <!-- 配额设置 (类型 + 数量) -->
            <div class="lg:col-span-3">
              <label class="block text-[11px] text-ink-400 mb-1">单次批产配额：</label>
              <div class="flex items-center gap-1.5">
                <select 
                  v-model="automationEditMap[item.project_id].quota_kind"
                  class="bg-atelier-950 border border-atelier-750 rounded-md px-2 py-1 text-xs text-ink-200 font-mono focus:outline-none focus:border-brand-amber/60">
                  <option value="chapters">章</option>
                  <option value="volumes">卷</option>
                </select>
                <input 
                  type="number" 
                  min="1"
                  v-model.number="automationEditMap[item.project_id].quota_count"
                  class="w-20 bg-atelier-950 border border-atelier-750 rounded-md px-2 py-1 text-xs text-ink-100 font-mono focus:outline-none focus:border-brand-amber/60">
                <span class="text-[11px] text-ink-400 shrink-0">
                  {{ automationEditMap[item.project_id]?.quota_kind === 'volumes' ? '卷' : '章' }}
                </span>
              </div>
            </div>

            <!-- 保存操作按钮 -->
            <div class="lg:col-span-2 flex justify-end">
              <button 
                @click="handleSaveAutomation(item.project_id)"
                :disabled="automationSavingMap[item.project_id]"
                class="w-full sm:w-auto px-3.5 py-1.5 bg-atelier-800 hover:bg-brand-amber hover:text-atelier-950 text-ink-200 font-medium text-xs rounded-md border border-atelier-700 transition cursor-pointer flex items-center justify-center gap-1.5 disabled:opacity-50">
                <Loader2 v-if="automationSavingMap[item.project_id]" class="w-3.5 h-3.5 animate-spin text-brand-amber" />
                <Save v-else class="w-3.5 h-3.5" />
                <span>{{ automationSavingMap[item.project_id] ? '保存中...' : '保存配置' }}</span>
              </button>
            </div>
          </div>

          <!-- 行级报错信息提示 -->
          <div 
            v-if="automationErrorMap[item.project_id]" 
            class="text-[11px] text-rose-400 bg-rose-500/10 border border-rose-500/20 px-3 py-1.5 rounded-md flex items-center gap-1.5">
            <AlertCircle class="w-3.5 h-3.5 shrink-0" />
            <span>{{ automationErrorMap[item.project_id] }}</span>
          </div>
        </div>
      </div>
    </section>

    <!-- ==================== 区块二：新书自动开书队列 ==================== -->
    <section class="space-y-4 pt-4 border-t border-atelier-800">
      <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-2">
        <div class="flex items-center gap-2">
          <Layers class="w-4 h-4 text-brand-amber" />
          <h3 class="text-sm font-serif font-bold text-ink-100">新书队列</h3>
          <span 
            v-if="queueItems.length > 0" 
            class="text-[10px] font-mono px-1.5 py-0.2 rounded bg-atelier-800 text-ink-300 border border-atelier-700">
            {{ queueItems.length }} 个任务
          </span>
        </div>
        <p class="text-[11px] text-ink-400">全自动无人值守开书队列，调度器将在全局设置时刻自动开启并批产。</p>
      </div>

      <!-- 顶部添加新书表单 -->
      <div class="p-4 bg-atelier-900 border border-atelier-750 rounded-xl space-y-3.5 shadow-atelier-sm">
        <div class="flex items-center justify-between border-b border-atelier-800 pb-2">
          <h4 class="text-xs font-serif font-bold text-ink-100 flex items-center gap-1.5">
            <Plus class="w-3.5 h-3.5 text-brand-amber" />
            <span>添加新书到队列</span>
          </h4>
          <span class="text-[10px] font-mono text-ink-400">自动完成创世、大纲与首卷批产</span>
        </div>

        <div class="grid grid-cols-1 md:grid-cols-3 gap-3">
          <div>
            <label class="block text-[11px] text-ink-300 mb-1">书名 <span class="text-rose-400">*</span>：</label>
            <input 
              type="text" 
              v-model="newQueueForm.title" 
              placeholder="例如《大乾镇魔司》"
              class="w-full bg-atelier-950 border border-atelier-750 rounded-md px-3 py-1.5 text-xs text-ink-100 focus:outline-none focus:border-brand-amber/60">
          </div>

          <div>
            <label class="block text-[11px] text-ink-300 mb-1">目标平台：</label>
            <input 
              type="text" 
              v-model="newQueueForm.platform" 
              placeholder="默认 通用网文"
              class="w-full bg-atelier-950 border border-atelier-750 rounded-md px-3 py-1.5 text-xs text-ink-100 focus:outline-none focus:border-brand-amber/60">
          </div>

          <div>
            <label class="block text-[11px] text-ink-300 mb-1">自动批产卷数：</label>
            <input 
              type="number" 
              min="1"
              v-model.number="newQueueForm.volumes" 
              placeholder="1"
              class="w-full bg-atelier-950 border border-atelier-750 rounded-md px-3 py-1.5 text-xs text-ink-100 font-mono focus:outline-none focus:border-brand-amber/60">
          </div>
        </div>

        <div>
          <label class="block text-[11px] text-ink-300 mb-1">核心脑洞与设定反差：</label>
          <textarea 
            v-model="newQueueForm.concept" 
            rows="2" 
            placeholder="输入题材类型、核心反差设定、金手指或主要剧情线索..."
            class="w-full bg-atelier-950 border border-atelier-750 rounded-md p-2.5 text-xs text-ink-100 placeholder:text-ink-500 focus:outline-none focus:border-brand-amber/60 resize-y"></textarea>
        </div>

        <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 pt-1">
          <label class="flex items-center gap-2 cursor-pointer select-none">
            <input 
              type="checkbox" 
              v-model="newQueueForm.auto_lock" 
              class="rounded border-atelier-700 bg-atelier-950 text-brand-amber accent-brand-amber">
            <span class="text-xs text-ink-200">自动锁定意图卡（开启后全自动推进，不因意图暂停）</span>
          </label>

          <button 
            @click="handleCreateQueueItem"
            :disabled="isAddingQueue"
            class="px-4 py-1.5 bg-brand-amber hover:bg-brand-amber-hover text-atelier-950 font-bold text-xs rounded-md shadow-amber-glow transition cursor-pointer flex items-center justify-center gap-1.5 disabled:opacity-50">
            <Loader2 v-if="isAddingQueue" class="w-3.5 h-3.5 animate-spin" />
            <Plus v-else class="w-3.5 h-3.5" />
            <span>{{ isAddingQueue ? '提交中...' : '加入队列' }}</span>
          </button>
        </div>

        <!-- 表单错误提示 -->
        <div 
          v-if="createQueueError" 
          class="text-[11px] text-rose-400 bg-rose-500/10 border border-rose-500/20 px-3 py-1.5 rounded-md flex items-center gap-1.5">
          <AlertCircle class="w-3.5 h-3.5 shrink-0" />
          <span>{{ createQueueError }}</span>
        </div>
      </div>

      <!-- 队列列表加载中 -->
      <div v-if="isLoadingQueue && queueItems.length === 0" class="py-12 text-center text-xs text-ink-400 space-y-2">
        <span class="inline-block w-5 h-5 border-2 border-brand-amber border-t-transparent rounded-full animate-spin"></span>
        <p class="text-[11px] font-mono text-ink-500">正在检查新书队列任务...</p>
      </div>

      <!-- 队列空状态 -->
      <div 
        v-else-if="queueItems.length === 0" 
        class="p-8 text-center bg-atelier-900 border border-atelier-750 rounded-xl space-y-2">
        <CheckCircle2 class="w-8 h-8 text-emerald-400 mx-auto" />
        <h4 class="text-xs font-semibold text-ink-200">队列为空</h4>
        <p class="text-[11px] text-ink-400">当前没有排队的新书任务。在上方填写创意设定即可加入无人值守开书队列。</p>
      </div>

      <!-- 队列任务列表 -->
      <div v-else class="space-y-3">
        <div 
          v-for="item in queueItems" 
          :key="item.id"
          class="p-4 bg-atelier-900 border border-atelier-750 rounded-xl hover:border-atelier-700 transition shadow-atelier-sm space-y-3">
          
          <!-- 常规展示模式 -->
          <template v-if="editingQueueId !== item.id">
            <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-2.5">
              <div class="flex items-center gap-2 flex-wrap">
                <span class="text-sm font-serif font-bold text-ink-50">
                  {{ item.title }}
                </span>
                <span 
                  :class="getQueueStatusBadge(item.status).class"
                  class="px-2 py-0.5 rounded text-[11px] font-mono font-medium border">
                  {{ getQueueStatusBadge(item.status).label }}
                </span>
                <span class="text-[10px] font-mono px-1.5 py-0.5 rounded bg-atelier-800 text-ink-300 border border-atelier-700">
                  {{ item.platform || '通用网文' }}
                </span>
                <span class="text-[10px] font-mono text-ink-400">
                  {{ item.volumes || 1 }} 卷
                </span>
                <span 
                  v-if="item.auto_lock"
                  class="text-[10px] font-mono px-1 py-0.2 rounded bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
                  自动锁定意图
                </span>
              </div>

              <!-- 右侧时间与操作按钮 -->
              <div class="flex items-center gap-2 self-end sm:self-auto">
                <span class="text-[11px] font-mono text-ink-400 mr-1">
                  {{ formatRelativeTime(item.created_at) }}
                </span>

                <!-- 仅 pending 状态允许编辑 -->
                <button 
                  v-if="item.status === 'pending'"
                  @click="startEditQueueItem(item)"
                  class="px-2.5 py-1 bg-atelier-800 hover:bg-atelier-750 text-ink-200 text-xs rounded border border-atelier-700 transition cursor-pointer flex items-center gap-1"
                  title="编辑待产任务">
                  <Edit3 class="w-3 h-3 text-ink-400" />
                  <span>编辑</span>
                </button>

                <!-- 删除按钮 (running 状态 disabled) -->
                <button 
                  @click="handleDeleteQueueItem(item)"
                  :disabled="item.status === 'running' || deletingQueueMap[item.id]"
                  :title="item.status === 'running' ? '执行中的任务不可删除' : '从队列中删除'"
                  class="px-2.5 py-1 bg-atelier-800 hover:bg-rose-500/20 text-rose-400 hover:text-rose-300 text-xs rounded border border-atelier-700 hover:border-rose-500/30 transition cursor-pointer flex items-center gap-1 disabled:opacity-40 disabled:cursor-not-allowed">
                  <Trash2 class="w-3 h-3" />
                  <span>{{ deletingQueueMap[item.id] ? '删除中...' : '删除' }}</span>
                </button>
              </div>
            </div>

            <!-- 核心脑洞与执行日志 -->
            <div v-if="item.concept || item.last_message || item.project_id" class="pt-2 border-t border-atelier-800 text-xs space-y-1.5">
              <p v-if="item.concept" class="text-ink-300 leading-relaxed text-[11px]">
                <span class="text-ink-400 font-medium">脑洞设定：</span>{{ item.concept }}
              </p>
              <div class="flex items-center gap-3 text-[11px] font-mono text-ink-400 flex-wrap">
                <span v-if="item.project_id" class="text-brand-amber">
                  已关联工程: {{ item.project_id }}
                </span>
                <span v-if="item.last_message" class="text-ink-400 italic">
                  状态反馈: {{ item.last_message }}
                </span>
              </div>
            </div>
          </template>

          <!-- 内联编辑模式 (仅 pending 行) -->
          <template v-else>
            <div class="space-y-3 p-1">
              <div class="flex items-center justify-between border-b border-atelier-800 pb-2">
                <span class="text-xs font-semibold text-brand-amber flex items-center gap-1.5">
                  <Edit3 class="w-3.5 h-3.5" />
                  <span>编辑队列任务「{{ item.title }}」</span>
                </span>
                <button 
                  @click="cancelEditQueueItem" 
                  class="text-ink-400 hover:text-ink-200 text-xs flex items-center gap-0.5 cursor-pointer">
                  <X class="w-3 h-3" />
                  <span>取消</span>
                </button>
              </div>

              <div class="grid grid-cols-1 md:grid-cols-3 gap-3">
                <div>
                  <label class="block text-[11px] text-ink-300 mb-1">书名：</label>
                  <input 
                    type="text" 
                    v-model="queueEditForm.title" 
                    class="w-full bg-atelier-950 border border-atelier-750 rounded-md px-2.5 py-1 text-xs text-ink-100 focus:outline-none focus:border-brand-amber/60">
                </div>

                <div>
                  <label class="block text-[11px] text-ink-300 mb-1">目标平台：</label>
                  <input 
                    type="text" 
                    v-model="queueEditForm.platform" 
                    class="w-full bg-atelier-950 border border-atelier-750 rounded-md px-2.5 py-1 text-xs text-ink-100 focus:outline-none focus:border-brand-amber/60">
                </div>

                <div>
                  <label class="block text-[11px] text-ink-300 mb-1">批产卷数：</label>
                  <input 
                    type="number" 
                    min="1"
                    v-model.number="queueEditForm.volumes" 
                    class="w-full bg-atelier-950 border border-atelier-750 rounded-md px-2.5 py-1 text-xs text-ink-100 font-mono focus:outline-none focus:border-brand-amber/60">
                </div>
              </div>

              <div>
                <label class="block text-[11px] text-ink-300 mb-1">核心脑洞：</label>
                <textarea 
                  v-model="queueEditForm.concept" 
                  rows="2" 
                  class="w-full bg-atelier-950 border border-atelier-750 rounded-md p-2 text-xs text-ink-100 focus:outline-none focus:border-brand-amber/60 resize-y"></textarea>
              </div>

              <div class="flex items-center justify-between gap-3 pt-1">
                <label class="flex items-center gap-2 cursor-pointer select-none">
                  <input 
                    type="checkbox" 
                    v-model="queueEditForm.auto_lock" 
                    class="rounded border-atelier-700 bg-atelier-950 text-brand-amber accent-brand-amber">
                  <span class="text-xs text-ink-200">自动锁定意图卡</span>
                </label>

                <div class="flex items-center gap-2">
                  <button 
                    @click="cancelEditQueueItem" 
                    class="px-3 py-1 bg-atelier-800 hover:bg-atelier-750 text-ink-300 text-xs rounded border border-atelier-700 transition cursor-pointer">
                    取消
                  </button>
                  <button 
                    @click="handleUpdateQueueItem(item.id)" 
                    :disabled="isUpdatingQueue"
                    class="px-3 py-1 bg-brand-amber hover:bg-brand-amber-hover text-atelier-950 font-bold text-xs rounded shadow-amber-glow transition cursor-pointer flex items-center gap-1 disabled:opacity-50">
                    <Loader2 v-if="isUpdatingQueue" class="w-3 h-3 animate-spin" />
                    <Save v-else class="w-3 h-3" />
                    <span>{{ isUpdatingQueue ? '保存中...' : '保存修改' }}</span>
                  </button>
                </div>
              </div>

              <!-- 编辑行报错提示 -->
              <div 
                v-if="queueRowErrorMap[item.id]" 
                class="text-[11px] text-rose-400 bg-rose-500/10 border border-rose-500/20 px-3 py-1 rounded-md flex items-center gap-1.5">
                <AlertCircle class="w-3.5 h-3.5 shrink-0" />
                <span>{{ queueRowErrorMap[item.id] }}</span>
              </div>
            </div>
          </template>
        </div>
      </div>
    </section>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted, watch } from 'vue';
import { state, actions, notify } from '../stores/appState';
import { api } from '../api/client';
import {
  CalendarClock,
  Clock,
  Layers,
  RefreshCw,
  Plus,
  Play,
  Save,
  Trash2,
  Edit3,
  CheckCircle2,
  AlertTriangle,
  AlertCircle,
  X,
  Loader2
} from 'lucide-vue-next';

// 加载态与错误态
const isLoadingAutomations = ref(false);
const isLoadingQueue = ref(false);
const errorMessage = ref('');

const isLoading = computed(() => isLoadingAutomations.value || isLoadingQueue.value);

// 区块一：作品排程数据
const automations = ref([]);
const automationEditMap = ref({});
const automationSavingMap = reactive({});
const automationResumingMap = reactive({});
const automationErrorMap = reactive({});

// 区块二：新书队列数据
const queueItems = ref([]);
const newQueueForm = ref({
  title: '',
  concept: '',
  platform: '通用网文',
  volumes: 1,
  auto_lock: true,
});
const createQueueError = ref('');
const isAddingQueue = ref(false);

const editingQueueId = ref(null);
const queueEditForm = ref({
  title: '',
  concept: '',
  platform: '通用网文',
  volumes: 1,
  auto_lock: true,
});
const isUpdatingQueue = ref(false);
const queueRowErrorMap = reactive({});
const deletingQueueMap = reactive({});

// 日期解析工具函数
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

// 相对时间展示
function formatRelativeTime(dateInput) {
  const date = parseDate(dateInput);
  if (!date) return dateInput ? String(dateInput) : '暂无记录';

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

// 队列状态徽章解析
function getQueueStatusBadge(status) {
  switch (status) {
    case 'pending':
      return { label: '排队中', class: 'bg-atelier-800 text-ink-300 border-atelier-700' };
    case 'running':
      return { label: '执行中', class: 'bg-sky-500/15 text-sky-400 border-sky-500/30' };
    case 'done':
      return { label: '已完成', class: 'bg-emerald-500/15 text-emerald-400 border-emerald-500/30' };
    case 'halted':
      return { label: '已停线', class: 'bg-rose-500/15 text-rose-400 border-rose-500/30' };
    case 'error':
      return { label: '异常', class: 'bg-rose-500/15 text-rose-400 border-rose-500/30' };
    default:
      return { label: status || '未知', class: 'bg-atelier-800 text-ink-400 border-atelier-700' };
  }
}

// 排程执行状态徽章解析
function getAutomationStatusBadge(status) {
  switch (status) {
    case 'success':
    case 'ok':
    case 'done':
      return { label: '成功', class: 'bg-emerald-500/15 text-emerald-400 border-emerald-500/30' };
    case 'running':
      return { label: '执行中', class: 'bg-sky-500/15 text-sky-400 border-sky-500/30' };
    case 'halted':
      return { label: '停线', class: 'bg-amber-500/15 text-amber-400 border-amber-500/30' };
    case 'error':
      return { label: '异常', class: 'bg-rose-500/15 text-rose-400 border-rose-500/30' };
    default:
      return { label: status || '', class: 'bg-atelier-800 text-ink-400 border-atelier-700' };
  }
}

// 获取作品排程列表
async function fetchAutomations() {
  isLoadingAutomations.value = true;
  try {
    const res = await api.listAutomations();
    const items = Array.isArray(res?.items) ? res.items : [];
    automations.value = items;

    // 同步到编辑字典
    const newEditMap = { ...automationEditMap.value };
    items.forEach(item => {
      newEditMap[item.project_id] = {
        enabled: Boolean(item.enabled),
        timesStr: Array.isArray(item.times) ? item.times.join(', ') : '',
        quota_kind: item.quota_kind || 'chapters',
        quota_count: item.quota_count ?? 1,
      };
    });
    automationEditMap.value = newEditMap;
  } catch (err) {
    console.error('fetchAutomations error:', err);
    errorMessage.value = `获取作品排程失败: ${err.message || '网络或后端服务未就绪'}`;
  } finally {
    isLoadingAutomations.value = false;
  }
}

// 保存单个作品排程配置
async function handleSaveAutomation(projectId) {
  const edit = automationEditMap.value[projectId];
  if (!edit) return;

  const rawTimes = edit.timesStr
    ? edit.timesStr.split(',').map(s => s.trim()).filter(Boolean)
    : [];
  const quotaCount = parseInt(edit.quota_count, 10) || 1;
  const payload = {
    enabled: Boolean(edit.enabled),
    times: rawTimes,
    quota_kind: edit.quota_kind || 'chapters',
    quota_count: quotaCount,
  };

  automationSavingMap[projectId] = true;
  automationErrorMap[projectId] = '';

  try {
    await api.saveAutomation(projectId, payload);
    notify('排程已保存', '配置已更新生效', 'success');
    await fetchAutomations();
  } catch (err) {
    console.error('saveAutomation error:', err);
    automationErrorMap[projectId] = err.message || '保存排程失败';
    notify('保存排程失败', err.message || '请求参数错误或服务端异常', 'error');
  } finally {
    automationSavingMap[projectId] = false;
  }
}

// 恢复作品排程 (停线后恢复)
async function handleResumeAutomation(projectId) {
  automationResumingMap[projectId] = true;
  try {
    await api.resumeAutomation(projectId);
    notify('已恢复排程', '作品已解除暂停，将在下次定时时刻继续推演', 'success');
    await fetchAutomations();
  } catch (err) {
    console.error('resumeAutomation error:', err);
    notify('恢复排程失败', err.message || '网络异常或作品不存在', 'error');
  } finally {
    automationResumingMap[projectId] = false;
  }
}

// 获取新书队列列表
async function fetchQueue() {
  isLoadingQueue.value = true;
  try {
    const res = await api.listQueue();
    queueItems.value = Array.isArray(res?.items) ? res.items : [];
  } catch (err) {
    console.error('fetchQueue error:', err);
    errorMessage.value = `获取新书队列失败: ${err.message || '网络或后端服务未就绪'}`;
  } finally {
    isLoadingQueue.value = false;
  }
}

// 添加新书到队列
async function handleCreateQueueItem() {
  const title = (newQueueForm.value.title || '').trim();
  if (!title) {
    createQueueError.value = '请填写作品书名';
    return;
  }

  createQueueError.value = '';
  isAddingQueue.value = true;

  try {
    const payload = {
      title,
      concept: (newQueueForm.value.concept || '').trim(),
      platform: (newQueueForm.value.platform || '').trim() || '通用网文',
      volumes: parseInt(newQueueForm.value.volumes, 10) || 1,
      auto_lock: Boolean(newQueueForm.value.auto_lock),
    };
    await api.createQueueItem(payload);
    notify('已加入队列', `新书《${payload.title}》已进入排产队列`, 'success');
    
    // 重置表单
    newQueueForm.value = {
      title: '',
      concept: '',
      platform: '通用网文',
      volumes: 1,
      auto_lock: true,
    };
    await fetchQueue();
  } catch (err) {
    console.error('createQueueItem error:', err);
    createQueueError.value = err.message || '添加新书任务失败';
    notify('添加新书失败', err.message || '请求参数校验不通过', 'error');
  } finally {
    isAddingQueue.value = false;
  }
}

// 开启行内编辑 (仅 pending 状态)
function startEditQueueItem(item) {
  editingQueueId.value = item.id;
  queueRowErrorMap[item.id] = '';
  queueEditForm.value = {
    title: item.title || '',
    concept: item.concept || '',
    platform: item.platform || '通用网文',
    volumes: item.volumes ?? 1,
    auto_lock: item.auto_lock ?? true,
  };
}

// 取消行内编辑
function cancelEditQueueItem() {
  editingQueueId.value = null;
}

// 保存行内编辑
async function handleUpdateQueueItem(id) {
  const title = (queueEditForm.value.title || '').trim();
  if (!title) {
    queueRowErrorMap[id] = '书名不能为空';
    return;
  }

  isUpdatingQueue.value = true;
  queueRowErrorMap[id] = '';

  try {
    const payload = {
      title,
      concept: (queueEditForm.value.concept || '').trim(),
      platform: (queueEditForm.value.platform || '').trim() || '通用网文',
      volumes: parseInt(queueEditForm.value.volumes, 10) || 1,
      auto_lock: Boolean(queueEditForm.value.auto_lock),
    };
    await api.updateQueueItem(id, payload);
    notify('修改成功', `新书《${payload.title}》任务信息已更新`, 'success');
    editingQueueId.value = null;
    await fetchQueue();
  } catch (err) {
    console.error('updateQueueItem error:', err);
    queueRowErrorMap[id] = err.message || '更新队列任务失败';
    notify('更新失败', err.message || '该任务可能已在运行中，不可修改', 'error');
  } finally {
    isUpdatingQueue.value = false;
  }
}

// 删除队列任务
async function handleDeleteQueueItem(item) {
  if (item.status === 'running') {
    notify('无法删除', '正在执行的新书任务不可删除', 'warning');
    return;
  }

  const confirmed = await actions.confirm({
    title: '确认删除队列任务',
    message: `确定要将新书《${item.title}》从排队任务中移除吗？`,
    type: 'danger',
    confirmText: '确认删除',
  });
  if (!confirmed) return;

  deletingQueueMap[item.id] = true;
  try {
    await api.deleteQueueItem(item.id);
    notify('已删除', `新书《${item.title}》已从队列移除`, 'success');
    await fetchQueue();
  } catch (err) {
    console.error('deleteQueueItem error:', err);
    notify('删除失败', err.message || '删除失败，任务可能已在执行中', 'error');
  } finally {
    deletingQueueMap[item.id] = false;
  }
}

// 刷新全量数据
async function refreshAll() {
  errorMessage.value = '';
  await Promise.allSettled([
    fetchAutomations(),
    fetchQueue(),
  ]);
}

// 监听活动选项卡切换，切回时自适应刷新
watch(
  () => state.activeTab,
  (newTab) => {
    if (newTab === 'studio-automation') {
      refreshAll();
    }
  }
);

onMounted(() => {
  refreshAll();
});
</script>
