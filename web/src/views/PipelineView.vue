<template>
  <div class="flex-1 p-6 overflow-y-auto space-y-6 bg-atelier-950">
    <!-- 头部工具栏 -->
    <div class="flex flex-col md:flex-row md:items-center justify-between gap-4 border-b border-atelier-750 pb-5">
      <div>
        <div class="flex items-center gap-2.5">
          <div class="w-8 h-8 rounded-lg bg-brand-amber/10 border border-brand-amber/25 flex items-center justify-center text-brand-amber shadow-amber-glow">
            <Rocket class="w-4 h-4" />
          </div>
          <div>
            <div class="flex items-center gap-2">
              <h2 class="text-base font-serif font-bold text-ink-50 tracking-wide">全自动创作流水线</h2>
              <span 
                v-if="isRunning" 
                class="flex items-center gap-1.5 text-[11px] font-mono px-2.5 py-0.5 rounded-full bg-brand-amber/15 text-brand-amber border border-brand-amber/30">
                <span class="w-1.5 h-1.5 rounded-full bg-brand-amber animate-pulse"></span>
                <span>生产线运行中</span>
                <span class="text-ink-400">· 耗时 {{ formatElapsedTime(elapsedSeconds) }}</span>
              </span>
              <span 
                v-else-if="pipelineResult?.halted" 
                class="text-[11px] font-mono px-2 py-0.5 rounded-full bg-rose-500/15 text-rose-400 border border-rose-500/30">
                停线待人工
              </span>
              <span 
                v-else-if="pipelineResult" 
                class="text-[11px] font-mono px-2 py-0.5 rounded-full bg-emerald-500/15 text-emerald-400 border border-emerald-500/30">
                已完工
              </span>
            </div>
            <p class="text-xs text-ink-400 mt-0.5">从核心脑洞一键自动化串通创世、设定推演、章节意图与正文批量成章。</p>
          </div>
        </div>
      </div>

      <div class="flex items-center gap-2.5">
        <button 
          v-if="events.length > 0 && !isRunning" 
          @click="resetPipeline"
          class="flex items-center gap-1.5 px-3 py-1.5 bg-atelier-850 hover:bg-atelier-800 text-ink-300 hover:text-ink-100 text-xs rounded-md border border-atelier-750 transition cursor-pointer"
          title="重置流水线，开启新作品批产">
          <RotateCcw class="w-3.5 h-3.5 text-ink-400" />
          <span>重置 / 新建任务</span>
        </button>
      </div>
    </div>

    <!-- 主布局：左右分栏 -->
    <div class="grid grid-cols-1 lg:grid-cols-12 gap-6 items-start">
      
      <!-- 左栏：流水线参数配置表单 (4 cols / 5 cols) -->
      <div class="lg:col-span-5 space-y-4">
        <div class="p-5 bg-atelier-900 border border-atelier-750 rounded-xl shadow-atelier-sm space-y-4">
          <div class="flex items-center justify-between border-b border-atelier-800 pb-3">
            <span class="text-xs font-semibold text-ink-100 flex items-center gap-1.5">
              <Sliders class="w-3.5 h-3.5 text-brand-amber" />
              <span>流水线运行参数</span>
            </span>
            <span class="text-[10px] font-mono text-ink-500">
              CLI: novel-studio pipeline
            </span>
          </div>

          <!-- 书名 -->
          <div class="space-y-1.5">
            <label class="block text-xs font-medium text-ink-200">
              作品书名 <span class="text-rose-400">*</span>
            </label>
            <input 
              v-model="formData.title"
              :disabled="isRunning"
              type="text"
              placeholder="例如：太初剑尊、我在赛博修长生"
              class="w-full bg-atelier-950 border border-atelier-750 rounded-md px-3 py-2 text-xs text-ink-100 placeholder-ink-600 focus:outline-none focus:border-brand-amber/60 focus:ring-1 focus:ring-brand-amber/40 transition disabled:opacity-50"
            />
          </div>

          <!-- 目标平台 -->
          <div class="space-y-1.5">
            <label class="block text-xs font-medium text-ink-200">
              目标发布平台
            </label>
            <select 
              v-model="formData.target_platform"
              :disabled="isRunning"
              class="w-full bg-atelier-950 border border-atelier-750 rounded-md px-3 py-2 text-xs text-ink-100 focus:outline-none focus:border-brand-amber/60 focus:ring-1 focus:ring-brand-amber/40 transition disabled:opacity-50 cursor-pointer">
              <option value="通用网文">通用网文 (标准叙事与节奏)</option>
              <option value="起点仙侠">起点仙侠 (重世界观严谨与力量体系)</option>
              <option value="番茄脑洞">番茄脑洞 (强金手指与极速爽点推进)</option>
              <option value="知乎盐言">知乎盐言 (短篇第一人称、强反转)</option>
            </select>
          </div>

          <!-- 核心脑洞 (textarea) -->
          <div class="space-y-1.5">
            <label class="block text-xs font-medium text-ink-200">
              核心脑洞与设定契机 <span class="text-rose-400">*</span>
            </label>
            <textarea 
              v-model="formData.concept"
              :disabled="isRunning"
              rows="4"
              placeholder="详细描述故事起点、主角金手指、核心悬念冲突与世界观反差..."
              class="w-full bg-atelier-950 border border-atelier-750 rounded-md p-3 text-xs text-ink-100 placeholder-ink-600 focus:outline-none focus:border-brand-amber/60 focus:ring-1 focus:ring-brand-amber/40 transition disabled:opacity-50 leading-relaxed resize-y"
            ></textarea>
          </div>

          <!-- 卷数与起始章 -->
          <div class="grid grid-cols-2 gap-3">
            <div class="space-y-1.5">
              <label class="block text-[11px] font-medium text-ink-300">
                生成卷数 (0=全部)
              </label>
              <input 
                v-model.number="formData.volumes"
                :disabled="isRunning"
                type="number"
                min="0"
                class="w-full bg-atelier-950 border border-atelier-750 rounded-md px-3 py-1.5 text-xs text-ink-100 font-mono focus:outline-none focus:border-brand-amber/60 transition disabled:opacity-50"
              />
            </div>
            <div class="space-y-1.5">
              <label class="block text-[11px] font-medium text-ink-300">
                起始章数 (0=卷首)
              </label>
              <input 
                v-model.number="formData.chapter_from"
                :disabled="isRunning"
                type="number"
                min="0"
                class="w-full bg-atelier-950 border border-atelier-750 rounded-md px-3 py-1.5 text-xs text-ink-100 font-mono focus:outline-none focus:border-brand-amber/60 transition disabled:opacity-50"
              />
            </div>
          </div>

          <!-- 每章字数与返工轮次 -->
          <div class="grid grid-cols-2 gap-3">
            <div class="space-y-1.5">
              <label class="block text-[11px] font-medium text-ink-300">
                每章目标字数
              </label>
              <input 
                v-model.number="formData.words_target"
                :disabled="isRunning"
                type="number"
                min="500"
                step="100"
                class="w-full bg-atelier-950 border border-atelier-750 rounded-md px-3 py-1.5 text-xs text-ink-100 font-mono focus:outline-none focus:border-brand-amber/60 transition disabled:opacity-50"
              />
            </div>
            <div class="space-y-1.5">
              <label class="block text-[11px] font-medium text-ink-300">
                质检返工上限
              </label>
              <input 
                v-model.number="formData.max_rewrite_loops"
                :disabled="isRunning"
                type="number"
                min="1"
                max="10"
                class="w-full bg-atelier-950 border border-atelier-750 rounded-md px-3 py-1.5 text-xs text-ink-100 font-mono focus:outline-none focus:border-brand-amber/60 transition disabled:opacity-50"
              />
            </div>
          </div>

          <!-- 叙事风格 (可选) -->
          <div class="space-y-1.5">
            <label class="block text-xs font-medium text-ink-200">
              叙事文风笔触 (可选)
            </label>
            <input 
              v-model="formData.narrative_style"
              :disabled="isRunning"
              type="text"
              placeholder="例如：冷峻硬汉、快节奏高爽点、黑色幽默"
              class="w-full bg-atelier-950 border border-atelier-750 rounded-md px-3 py-2 text-xs text-ink-100 placeholder-ink-600 focus:outline-none focus:border-brand-amber/60 transition disabled:opacity-50"
            />
          </div>

          <!-- 自动锁定意图卡 Toggle -->
          <div class="pt-2 border-t border-atelier-800">
            <div class="flex items-start justify-between gap-3">
              <div class="space-y-0.5">
                <span class="text-xs font-medium text-ink-200 flex items-center gap-1.5">
                  <Lock class="w-3.5 h-3.5 text-brand-amber" />
                  <span>自动锁定意图卡</span>
                </span>
                <p class="text-[11px] text-ink-400 leading-relaxed">
                  {{ formData.auto_lock_intents ? '开启：意图生成后立即全速推进正文批产' : '关闭：生成意图后停线待人工锁定，确认无误再批产' }}
                </p>
              </div>
              <label class="relative inline-flex items-center cursor-pointer shrink-0 mt-0.5">
                <input 
                  v-model="formData.auto_lock_intents" 
                  :disabled="isRunning"
                  type="checkbox" 
                  class="sr-only peer" 
                />
                <div class="w-9 h-5 bg-atelier-800 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-4 after:w-4 after:transition-all peer-checked:bg-brand-amber disabled:opacity-50"></div>
              </label>
            </div>
          </div>

          <!-- 启动流水线按钮 -->
          <div class="pt-2">
            <button 
              @click="handleStartPipeline"
              :disabled="isRunning || !canSubmit"
              class="w-full py-2.5 px-4 bg-brand-amber hover:bg-brand-amber-hover text-atelier-950 font-bold text-xs rounded-lg shadow-amber-glow transition cursor-pointer flex items-center justify-center gap-2 disabled:opacity-40 disabled:cursor-not-allowed">
              <span v-if="isRunning" class="w-3.5 h-3.5 border-2 border-atelier-950 border-t-transparent rounded-full animate-spin"></span>
              <Rocket v-else class="w-4 h-4" />
              <span>{{ isRunning ? '流水线自动批产中...' : '启动全自动流水线' }}</span>
            </button>
          </div>
        </div>
      </div>

      <!-- 右栏：进度时间轴与终止汇总结果 (7 cols) -->
      <div class="lg:col-span-7 space-y-4">
        
        <!-- 终止状态：成功/停线汇总卡 (PIPELINE_RESULT) -->
        <div 
          v-if="pipelineResult"
          class="p-5 rounded-xl border transition shadow-atelier-sm space-y-4"
          :class="pipelineResult.halted 
            ? 'bg-amber-950/20 border-amber-500/40' 
            : 'bg-emerald-950/20 border-emerald-500/40'">
          
          <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 border-b border-atelier-800/80 pb-4">
            <div class="flex items-center gap-2.5">
              <div 
                class="w-8 h-8 rounded-lg flex items-center justify-center"
                :class="pipelineResult.halted 
                  ? 'bg-amber-500/15 text-amber-400 border border-amber-500/30' 
                  : 'bg-emerald-500/15 text-emerald-400 border border-emerald-500/30'">
                <AlertTriangle v-if="pipelineResult.halted" class="w-4 h-4" />
                <CheckCircle2 v-else class="w-4 h-4" />
              </div>
              <div>
                <div class="flex items-center gap-2">
                  <h3 class="text-sm font-serif font-bold text-ink-50">
                    《{{ pipelineResult.title || formData.title }}》
                  </h3>
                  <span 
                    v-if="pipelineResult.halted"
                    class="px-2 py-0.5 rounded text-[11px] font-mono font-semibold bg-rose-500/20 text-rose-300 border border-rose-500/40">
                    停线待人工
                  </span>
                  <span 
                    v-else
                    class="px-2 py-0.5 rounded text-[11px] font-mono font-semibold bg-emerald-500/20 text-emerald-300 border border-emerald-500/40">
                    流水线完工
                  </span>
                </div>
                <p v-if="pipelineResult.halted && pipelineResult.halt_reason" class="text-xs text-rose-300/90 mt-1">
                  停线原因：{{ pipelineResult.halt_reason }}
                </p>
                <p v-else class="text-xs text-ink-400 mt-0.5">
                  已完成全链路自动生产与质量审计。
                </p>
              </div>
            </div>

            <button 
              @click="goToWorkbench(pipelineResult.project_id)"
              class="px-3.5 py-2 bg-brand-amber hover:bg-brand-amber-hover text-atelier-950 font-bold text-xs rounded-lg shadow-amber-glow transition cursor-pointer flex items-center gap-1.5 shrink-0">
              <span>进入作品工作台</span>
              <ArrowUpRight class="w-3.5 h-3.5" />
            </button>
          </div>

          <!-- 汇总指标卡片网格 -->
          <div class="grid grid-cols-2 sm:grid-cols-3 gap-3">
            <div class="p-3 bg-atelier-900/80 border border-atelier-800 rounded-lg">
              <span class="text-[10px] font-mono text-ink-400 uppercase">工程项目 ID</span>
              <p class="text-xs font-mono font-medium text-ink-200 mt-1 truncate" :title="pipelineResult.project_id">
                {{ pipelineResult.project_id }}
              </p>
            </div>

            <div class="p-3 bg-atelier-900/80 border border-atelier-800 rounded-lg">
              <span class="text-[10px] font-mono text-ink-400 uppercase">总 Token 消耗</span>
              <p class="text-xs font-mono font-bold text-brand-amber mt-1">
                {{ (pipelineResult.total_usage?.total_tokens || 0).toLocaleString() }}
              </p>
            </div>

            <div class="p-3 bg-atelier-900/80 border border-atelier-800 rounded-lg">
              <span class="text-[10px] font-mono text-ink-400 uppercase">预估成本 (USD)</span>
              <p class="text-xs font-mono font-bold text-ink-100 mt-1">
                ${{ Number(pipelineResult.estimated_cost_usd || 0).toFixed(4) }}
              </p>
            </div>
          </div>

          <!-- 分卷生产状态 -->
          <div v-if="Array.isArray(pipelineResult.volumes) && pipelineResult.volumes.length > 0" class="space-y-1.5">
            <span class="text-[11px] font-mono text-ink-400">分卷生产汇总：</span>
            <div class="flex flex-wrap gap-2">
              <div 
                v-for="(vol, vIdx) in pipelineResult.volumes" 
                :key="vIdx"
                class="px-2.5 py-1.5 rounded bg-atelier-900/90 border border-atelier-800 text-xs font-mono flex items-center gap-2">
                <span class="text-ink-200">
                  {{ typeof vol === 'object' ? `第 ${vol.volume_index || vIdx + 1} 卷` : `卷 ${vIdx + 1}` }}
                </span>
                <span v-if="vol.chapters_count !== undefined" class="text-brand-amber">
                  {{ vol.chapters_count }} 章
                </span>
                <span 
                  v-if="vol.halted" 
                  class="text-[10px] px-1.5 py-0.2 rounded bg-rose-500/20 text-rose-300 border border-rose-500/40">
                  停线
                </span>
              </div>
            </div>
          </div>

          <!-- 备忘与说明 Notes -->
          <div v-if="Array.isArray(pipelineResult.notes) && pipelineResult.notes.length > 0" class="pt-2 border-t border-atelier-800/80 space-y-1">
            <span class="text-[11px] font-mono text-ink-400">流水线备忘留痕：</span>
            <ul class="space-y-1 pl-1">
              <li 
                v-for="(note, nIdx) in pipelineResult.notes" 
                :key="nIdx" 
                class="text-xs text-ink-300 flex items-start gap-1.5">
                <span class="text-ink-500">•</span>
                <span>{{ note }}</span>
              </li>
            </ul>
          </div>
        </div>

        <!-- 终止状态：异常失败卡片 (PIPELINE_ERROR) -->
        <div 
          v-if="pipelineError"
          class="p-4 bg-rose-500/10 border border-rose-500/30 rounded-xl space-y-3 text-xs text-rose-300">
          <div class="flex items-center gap-2.5 text-sm font-semibold text-rose-200">
            <AlertCircle class="w-4 h-4 text-rose-400 shrink-0" />
            <span>流水线执行异常中断</span>
          </div>
          <p class="leading-relaxed bg-rose-950/40 p-3 rounded border border-rose-500/20 font-mono text-[11px]">
            {{ pipelineError }}
          </p>
          <div class="flex items-center justify-end gap-2">
            <button 
              @click="pipelineError = null" 
              class="px-3 py-1 bg-atelier-850 hover:bg-atelier-800 text-ink-300 rounded text-xs transition cursor-pointer">
              清除提示
            </button>
            <button 
              @click="handleStartPipeline" 
              class="px-3 py-1 bg-rose-500/20 hover:bg-rose-500/30 text-rose-200 rounded text-xs font-medium transition cursor-pointer">
              重试执行
            </button>
          </div>
        </div>

        <!-- 初始空态说明卡片 (未启动且无事件流时显示) -->
        <div 
          v-if="events.length === 0 && !pipelineResult && !pipelineError"
          class="p-6 bg-atelier-900 border border-atelier-750 rounded-xl space-y-5">
          <div class="flex items-center gap-2.5 text-ink-50">
            <Layers class="w-5 h-5 text-brand-amber" />
            <h3 class="text-sm font-serif font-bold">全流程工业化批产流水线说明</h3>
          </div>
          <p class="text-xs text-ink-400 leading-relaxed">
            全自动流水线将按专业网文工序逐级驱动，一站式完成从概念到可交付成章的完整生产闭环：
          </p>

          <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div class="p-3 bg-atelier-950/60 border border-atelier-800 rounded-lg space-y-1">
              <span class="text-xs font-semibold text-ink-200 flex items-center gap-1.5">
                <span class="w-4 h-4 rounded-full bg-brand-amber/15 text-brand-amber text-[10px] font-mono flex items-center justify-center">1</span>
                创世设定 (Genesis)
              </span>
              <p class="text-[11px] text-ink-400 leading-relaxed">
                自动解析核心脑洞，生成平台文风基调、核心规则与力量体系梯队。
              </p>
            </div>

            <div class="p-3 bg-atelier-950/60 border border-atelier-800 rounded-lg space-y-1">
              <span class="text-xs font-semibold text-ink-200 flex items-center gap-1.5">
                <span class="w-4 h-4 rounded-full bg-brand-amber/15 text-brand-amber text-[10px] font-mono flex items-center justify-center">2</span>
                设定集与图谱 (Codex)
              </span>
              <p class="text-[11px] text-ink-400 leading-relaxed">
                建立主角、敌对势力、金手指实体并生成多角色关系网络。
              </p>
            </div>

            <div class="p-3 bg-atelier-950/60 border border-atelier-800 rounded-lg space-y-1">
              <span class="text-xs font-semibold text-ink-200 flex items-center gap-1.5">
                <span class="w-4 h-4 rounded-full bg-brand-amber/15 text-brand-amber text-[10px] font-mono flex items-center justify-center">3</span>
                章节意图规划 (Intents)
              </span>
              <p class="text-[11px] text-ink-400 leading-relaxed">
                规划每卷剧情弧线，提取核心冲突、视角与情绪张力卡。
              </p>
            </div>

            <div class="p-3 bg-atelier-950/60 border border-atelier-800 rounded-lg space-y-1">
              <span class="text-xs font-semibold text-ink-200 flex items-center gap-1.5">
                <span class="w-4 h-4 rounded-full bg-brand-amber/15 text-brand-amber text-[10px] font-mono flex items-center justify-center">4</span>
                自动批产与质检 (Batch)
              </span>
              <p class="text-[11px] text-ink-400 leading-relaxed">
                逐章生成正文手稿，触发反 AI 腔调调和与总编多维度严格评审。
              </p>
            </div>
          </div>

          <div class="p-3 rounded-lg bg-brand-amber/10 border border-brand-amber/25 text-xs text-brand-amber/90 leading-relaxed flex items-start gap-2">
            <Sparkles class="w-4 h-4 shrink-0 mt-0.5" />
            <span>
              <strong>安全停线机制：</strong> 若关闭「自动锁定意图卡」，系统将在意图规划完成后自动暂停，以便作者人工复核卡片后再解锁批产。
            </span>
          </div>
        </div>

        <!-- 进度时间轴 (Timeline) -->
        <div v-if="events.length > 0" class="p-5 bg-atelier-900 border border-atelier-750 rounded-xl space-y-3 shadow-atelier-sm">
          <div class="flex items-center justify-between border-b border-atelier-800 pb-2.5">
            <div class="flex items-center gap-2">
              <span class="text-xs font-semibold text-ink-100 flex items-center gap-1.5">
                <Activity class="w-3.5 h-3.5 text-brand-amber" />
                <span>事件日志时间轴</span>
              </span>
              <span class="text-[10px] font-mono px-2 py-0.2 rounded bg-atelier-800 text-ink-400 border border-atelier-750">
                {{ events.length }} 条事件
              </span>
              <button 
                @click="autoScroll = !autoScroll"
                class="text-[10px] font-mono px-2 py-0.5 rounded border transition cursor-pointer"
                :class="autoScroll 
                  ? 'bg-brand-amber/10 text-brand-amber border-brand-amber/30' 
                  : 'bg-atelier-800 text-ink-400 border-atelier-750'">
                自动滚屏: {{ autoScroll ? '开' : '关' }}
              </button>
            </div>
            <span v-if="isRunning" class="text-[11px] font-mono text-brand-amber flex items-center gap-1">
              <span class="w-1.5 h-1.5 rounded-full bg-brand-amber animate-ping"></span>
              流式接入中...
            </span>
          </div>

          <!-- 滚动列表容器 -->
          <div 
            ref="timelineContainer"
            class="max-h-[500px] overflow-y-auto space-y-2 pr-1 select-text">
            
            <div 
              v-for="(ev, idx) in events" 
              :key="idx"
              class="p-2.5 rounded-lg border text-xs transition"
              :class="getEventCardClass(ev)">
              
              <div class="flex items-center justify-between gap-2 flex-wrap mb-1">
                <div class="flex items-center gap-1.5 flex-wrap">
                  <!-- Phase 徽章 -->
                  <span 
                    class="px-2 py-0.5 rounded text-[10px] font-mono font-semibold"
                    :class="getPhaseBadgeClass(ev.phase)">
                    {{ formatPhaseName(ev.phase) }}
                  </span>

                  <!-- 卷/章标签 -->
                  <span 
                    v-if="ev.volume_index !== undefined && ev.volume_index !== null" 
                    class="px-1.5 py-0.5 rounded text-[10px] font-mono bg-atelier-800 text-ink-300 border border-atelier-750">
                    卷 {{ ev.volume_index }}
                  </span>
                  <span 
                    v-if="ev.chapter_index !== undefined && ev.chapter_index !== null" 
                    class="px-1.5 py-0.5 rounded text-[10px] font-mono bg-brand-amber/10 text-brand-amber border border-brand-amber/25">
                    第 {{ ev.chapter_index }} 章
                  </span>
                </div>

                <!-- 人工介入提示 -->
                <span 
                  v-if="ev.needs_human"
                  class="px-2 py-0.5 rounded text-[10px] font-bold bg-rose-500/20 text-rose-300 border border-rose-500/40 flex items-center gap-1">
                  <AlertTriangle class="w-3 h-3 text-rose-400" />
                  <span>待人工锁定/介入</span>
                </span>
              </div>

              <!-- Message 文本 -->
              <div class="text-xs text-ink-200 leading-relaxed font-mono">
                {{ ev.message }}
              </div>
            </div>
          </div>
        </div>

      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, computed, nextTick, onUnmounted } from 'vue';
import { state, actions, notify } from '../stores/appState';
import { api } from '../api/client';
import {
  Rocket,
  RotateCcw,
  Sliders,
  Lock,
  CheckCircle2,
  AlertTriangle,
  AlertCircle,
  ArrowUpRight,
  Layers,
  Sparkles,
  Activity
} from 'lucide-vue-next';

const isRunning = ref(false);
const pipelineResult = ref(null);
const pipelineError = ref(null);
const events = ref([]);
const timelineContainer = ref(null);
const autoScroll = ref(true);
const elapsedSeconds = ref(0);
let timerId = null;

function formatElapsedTime(sec) {
  const m = Math.floor(sec / 60);
  const s = sec % 60;
  return `${m}分${s < 10 ? '0' : ''}${s}秒`;
}

function stopTimer() {
  if (timerId) {
    clearInterval(timerId);
    timerId = null;
  }
}

onUnmounted(() => {
  stopTimer();
});

const formData = reactive({
  title: '',
  concept: '',
  target_platform: '通用网文',
  volumes: 0,
  words_target: 2000,
  max_rewrite_loops: 3,
  narrative_style: '',
  chapter_from: 0,
  auto_lock_intents: false,
});

const canSubmit = computed(() => {
  return formData.title.trim().length > 0 && formData.concept.trim().length > 0;
});

// 重置表单与运行状态
function resetPipeline() {
  if (isRunning.value) return;
  stopTimer();
  elapsedSeconds.value = 0;
  pipelineResult.value = null;
  pipelineError.value = null;
  events.value = [];
}

// 自动滚底
async function scrollToBottom() {
  await nextTick();
  if (autoScroll.value && timelineContainer.value) {
    timelineContainer.value.scrollTop = timelineContainer.value.scrollHeight;
  }
}

// 启动流水线
async function handleStartPipeline() {
  if (!formData.title.trim()) {
    notify('请输入作品书名', '作品书名为必填项', 'warning');
    return;
  }
  if (!formData.concept.trim()) {
    notify('请输入核心脑洞', '核心脑洞设定为必填项', 'warning');
    return;
  }

  isRunning.value = true;
  pipelineResult.value = null;
  pipelineError.value = null;
  events.value = [];

  stopTimer();
  elapsedSeconds.value = 0;
  timerId = setInterval(() => {
    elapsedSeconds.value++;
  }, 1000);

  const payload = {
    title: formData.title.trim(),
    concept: formData.concept.trim(),
    target_platform: formData.target_platform,
    reasoning_model: state.config?.reasoning_model || 'deepseek-reasoner',
    volumes: Number(formData.volumes) || 0,
    auto_lock_intents: Boolean(formData.auto_lock_intents),
    words_target: Number(formData.words_target) || 2000,
    max_rewrite_loops: Number(formData.max_rewrite_loops) || 3,
    narrative_style: formData.narrative_style.trim(),
    chapter_from: Number(formData.chapter_from) || 0,
  };

  try {
    await api.startPipeline(payload, {
      onEvent: async (ev) => {
        events.value.push(ev);
        await scrollToBottom();
      },
      onResult: async (res) => {
        stopTimer();
        pipelineResult.value = res;
        isRunning.value = false;
        if (res?.halted) {
          notify('流水线停线待人工', res.halt_reason || '意图已生成，待人工锁定', 'warning');
        } else {
          notify('流水线执行完成', `作品《${res?.title || formData.title}》已就绪`, 'success');
        }
        await actions.loadProjects();
        await scrollToBottom();
      },
      onError: (err) => {
        stopTimer();
        pipelineError.value = err.message || '流水线执行异常';
        isRunning.value = false;
        notify('流水线执行中断', err.message || '网络连接超时或服务异常', 'error');
      },
    });
  } catch (err) {
    stopTimer();
    if (!pipelineError.value) {
      pipelineError.value = err.message || '流水线执行失败';
      isRunning.value = false;
      notify('流水线启动失败', err.message || '网络连接异常', 'error');
    }
  } finally {
    stopTimer();
    isRunning.value = false;
  }
}

// 跳转至作品工作台
async function goToWorkbench(projectId) {
  await actions.switchToProject(projectId, 'workbench');
}

function formatPhaseName(phase) {
  const map = {
    PIPELINE_START: '流水线启动',
    FRAMEWORK_READY: '创世设定完成',
    RELATIONS_DONE: '实体图谱就绪',
    INTENTS_READY: '章节意图生成',
    INTENTS_LOCKED: '章节意图锁定',
    VOLUME_START: '分卷开启',
    CHAPTER_START: '章节起草',
    VOLUME_DONE: '分卷完成',
    HALTED: '停线待人工',
    PIPELINE_COMPLETE: '流水线完工',
  };
  return map[phase] || phase || '处理中';
}

function getPhaseBadgeClass(phase) {
  switch (phase) {
    case 'PIPELINE_START':
      return 'bg-blue-500/15 text-blue-400 border border-blue-500/30';
    case 'FRAMEWORK_READY':
    case 'RELATIONS_DONE':
      return 'bg-purple-500/15 text-purple-300 border border-purple-500/30';
    case 'INTENTS_READY':
      return 'bg-amber-500/15 text-amber-300 border border-amber-500/30';
    case 'INTENTS_LOCKED':
      return 'bg-emerald-500/15 text-emerald-300 border border-emerald-500/30';
    case 'VOLUME_START':
    case 'CHAPTER_START':
      return 'bg-cyan-500/15 text-cyan-300 border border-cyan-500/30';
    case 'VOLUME_DONE':
    case 'PIPELINE_COMPLETE':
      return 'bg-emerald-500/20 text-emerald-400 border border-emerald-500/40';
    case 'HALTED':
      return 'bg-rose-500/20 text-rose-300 border border-rose-500/40';
    default:
      return 'bg-atelier-800 text-ink-300 border border-atelier-700';
  }
}

function getEventCardClass(ev) {
  if (ev.needs_human || ev.phase === 'HALTED') {
    return 'bg-rose-950/20 border-rose-500/30 text-rose-200';
  }
  if (ev.phase === 'PIPELINE_COMPLETE' || ev.phase === 'VOLUME_DONE') {
    return 'bg-emerald-950/20 border-emerald-500/30 text-emerald-200';
  }
  return 'bg-atelier-950/80 border-atelier-800/80 text-ink-200';
}
</script>
