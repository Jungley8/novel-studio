<template>
    <!-- 中栏：文学手稿画布 (弹性自适应排版空间) -->
    <div class="flex-1 flex flex-col min-h-0 min-w-0 overflow-hidden bg-atelier-950">
      
      <!-- 停线待人工红/琥珀色横幅 (质检超轮次停线，需人工介入定稿) -->
      <div 
        v-if="state.pipelineState?.halted && state.pipelineState?.haltInfo" 
        class="px-5 py-3.5 bg-gradient-to-r from-red-950/95 via-atelier-900/95 to-amber-950/90 border-b border-red-500/40 text-xs select-none shrink-0 animate-fade-in shadow-md">
        <div class="flex flex-col md:flex-row md:items-center justify-between gap-3">
          <div class="flex items-start gap-3">
            <div class="w-7 h-7 rounded-lg bg-red-500/20 text-red-400 border border-red-500/30 flex items-center justify-center font-bold shrink-0 mt-0.5">
              <AlertTriangle class="w-4 h-4 text-red-400" />
            </div>
            <div>
              <div class="font-bold text-red-200 flex flex-wrap items-center gap-2">
                <span class="text-sm font-serif">第 {{ state.pipelineState.haltInfo.chapterIndex }} 章已停线待人工</span>
                <span class="text-[11px] font-mono px-2 py-0.5 rounded bg-red-900/60 text-red-300 border border-red-500/30 font-normal">
                  终审 {{ state.pipelineState.haltInfo.verdict }} · {{ state.pipelineState.haltInfo.score }} 分 · 返工 {{ state.pipelineState.haltInfo.rewriteLoops }} 轮
                </span>
              </div>
              <div class="text-[11px] text-ink-300 mt-1">
                已达最大重写轮次或触发严重缺陷，自动流水线已安全阻断，需人工介入审阅定稿。
              </div>

              <!-- Top 3 Issues -->
              <div 
                v-if="state.pipelineState.haltInfo.issues && state.pipelineState.haltInfo.issues.length > 0" 
                class="mt-2 space-y-1 bg-atelier-950/70 p-2.5 rounded-lg border border-red-500/25 max-w-2xl">
                <div class="text-[10px] text-red-400 font-semibold mb-1 flex items-center gap-1">
                  <span>主要阻断问题（Top 3 Issues）：</span>
                </div>
                <div 
                  v-for="(issue, idx) in state.pipelineState.haltInfo.issues.slice(0, 3)" 
                  :key="idx" 
                  class="text-[11px] text-red-200/90 flex items-start gap-1.5 leading-snug">
                  <span class="text-red-400 font-bold shrink-0">•</span>
                  <span class="truncate">{{ issue }}</span>
                </div>
              </div>
            </div>
          </div>

          <!-- 双按钮操作区 -->
          <div class="flex items-center gap-2.5 shrink-0 self-end md:self-center">
            <button 
              @click="actions.loadHaltDraftForManualEdit ? actions.loadHaltDraftForManualEdit() : loadHaltDraft()" 
              class="px-3.5 py-1.5 rounded-lg bg-brand-amber/20 hover:bg-brand-amber/30 text-brand-amber border border-brand-amber/40 font-bold text-xs transition flex items-center gap-1.5 cursor-pointer shadow-sm">
              <PenTool class="w-3.5 h-3.5" />
              <span>载入返工稿进手工精修</span>
            </button>
            <button 
              @click="actions.retryAutonomousPipeline ? actions.retryAutonomousPipeline() : actions.runAutonomousPipeline()" 
              :disabled="state.pipelineState.active"
              class="px-3.5 py-1.5 rounded-lg bg-gradient-to-r from-red-600 to-amber-600 hover:from-red-500 hover:to-amber-500 text-ink-50 font-bold text-xs shadow-md transition flex items-center gap-1.5 cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed">
              <RotateCcw class="w-3.5 h-3.5" />
              <span>重试全自动</span>
            </button>
          </div>
        </div>
      </div>

      <!-- 宏观创世推演进行中提示条 -->
      <div 
        v-if="state.bootstrappingProjectId && state.bootstrappingProjectId === state.currentProject?.id" 
        class="px-5 py-2.5 bg-gradient-to-r from-brand-amber/15 via-atelier-900/90 to-brand-amber/10 border-b border-brand-amber/30 text-xs select-none shrink-0 animate-fade-in shadow-sm">
        <div class="flex items-center justify-between gap-3">
          <div class="flex items-center gap-2.5 min-w-0">
            <Loader2 class="w-4 h-4 text-brand-amber animate-spin shrink-0" />
            <div class="truncate">
              <span class="font-bold text-ink-100 font-serif">《{{ state.currentProject.title }}》宏观创世推演进行中</span>
              <span class="text-ink-400 ml-2 hidden sm:inline">{{ state.bootstrapProgressMessage || 'AI 正在梳理世界公理、战力阶梯、势力阵营与首章黄金冲突...' }}</span>
            </div>
          </div>
          <button 
            @click="actions.setActiveTab('framework')" 
            class="text-[11px] text-brand-amber hover:underline cursor-pointer shrink-0">
            查看推演设定 →
          </button>
        </div>
      </div>

      <!-- 连载接龙卡片 (本章已成功归档，一键衔接下一章) -->
      <div 
        v-if="state.pipelineState.justCommitted" 
        class="px-5 py-3 bg-gradient-to-r from-emerald-950/80 via-atelier-900/90 to-atelier-900 border-b border-brand-emerald/30 flex items-center justify-between text-xs select-none shrink-0 animate-fade-in">
        <div class="flex items-center gap-2.5">
          <div class="w-6 h-6 rounded-full bg-brand-emerald/20 text-brand-emerald flex items-center justify-center font-bold">
            ✓
          </div>
          <div>
            <div class="font-bold text-ink-50">
              第 {{ state.pipelineState.lastCommittedChapter }} 章已成功定稿入库！
            </div>
            <div class="text-[11px] text-ink-300">
              正史视界与状态机已结算，准备好开启下一章了吗？
            </div>
          </div>
        </div>
        <div class="flex items-center gap-2">
          <button 
            @click="state.pipelineState.justCommitted = false" 
            class="px-2.5 py-1 rounded bg-atelier-800 hover:bg-atelier-750 text-[11px] text-ink-400 hover:text-ink-200 transition cursor-pointer">
            稍后
          </button>
          <button 
            @click="actions.startNextChapter()" 
            :disabled="state.pipelineState?.halted"
            :title="state.pipelineState?.halted ? '本章停线，需人工定稿后才能继续' : ''"
            class="px-3.5 py-1.5 rounded-lg bg-gradient-to-r from-brand-amber to-amber-600 hover:from-brand-amber-hover hover:to-amber-500 text-atelier-950 font-bold text-xs shadow-amber-glow transition flex items-center gap-1.5 cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed disabled:grayscale disabled:shadow-none">
            <Zap class="w-3.5 h-3.5 fill-current" />
            <span>⚡ 一键生成第 {{ computedState.nextChapterIndex.value }} 章</span>
          </button>
        </div>
      </div>

      <!-- 撤回草稿 / 历史章节精修提示条 -->
      <div 
        v-if="state.editingChapterIndex" 
        class="px-5 py-2 bg-brand-amber/10 border-b border-brand-amber/25 flex items-center justify-between text-xs text-brand-amber select-none shrink-0">
        <div class="flex items-center gap-2">
          <RotateCcw class="w-3.5 h-3.5 shrink-0" />
          <span>正在精修 <strong>第 {{ state.editingChapterIndex }} 章</strong> 草稿（保存将覆盖该章节）</span>
        </div>
        <button 
          @click="resetToNewChapter" 
          class="px-2 py-0.5 rounded bg-brand-amber/20 hover:bg-brand-amber/30 text-[11px] font-medium transition cursor-pointer">
          放弃精修
        </button>
      </div>

      <!-- 手稿顶栏状态与字数计量 (去除按钮堆叠，主次分明) -->
      <div class="h-11 px-4 border-b border-atelier-750 flex items-center justify-between bg-atelier-900/30 shrink-0 select-none gap-2">
        <div class="flex items-center gap-2.5 min-w-0">
          <!-- 重新展开工步快捷按钮 (当左侧折叠时呈现) -->
          <button 
            v-if="!state.showWorkflowPanel && !state.isZenMode"
            @click="state.showWorkflowPanel = true"
            class="px-2 py-1 rounded bg-atelier-850 hover:bg-atelier-800 border border-atelier-750 text-xs text-brand-amber flex items-center gap-1 transition cursor-pointer"
            title="展开左侧工序">
            <PanelLeft class="w-3.5 h-3.5" />
            <span class="text-[11px] font-medium hidden sm:inline">工序</span>
          </button>

          <span class="text-xs font-bold font-serif text-ink-100 truncate">
            第 {{ computedState.currentWorkingChapterIndex.value }} 章
          </span>
          <div class="flex items-center gap-1.5 shrink-0">
            <span class="text-[11px] font-mono text-ink-400">
              <strong class="text-brand-amber font-semibold">{{ state.workbench.content.length }}</strong> / {{ state.wordsTarget || 2000 }} 字
            </span>
            <span 
              class="text-[9px] font-mono px-1.5 py-0.2 rounded border transition"
              :class="state.workbench.content.length >= (state.wordsTarget || 2000)
                ? 'bg-emerald-500/15 text-emerald-400 border-emerald-500/30'
                : 'bg-brand-amber/10 text-brand-amber border-brand-amber/25'">
              {{ Math.min(100, Math.round((state.workbench.content.length / (state.wordsTarget || 2000)) * 100)) }}% 达标
            </span>
          </div>

          <button 
            @click="actions.runAutonomousPipeline()"
            :disabled="state.pipelineState.active"
            class="ml-1.5 px-2.5 py-1 rounded-md text-[11px] font-bold transition flex items-center gap-1 cursor-pointer disabled:opacity-50"
            :class="state.pipelineState.active 
              ? 'bg-amber-500/20 text-brand-amber border border-brand-amber/40 animate-subtle-pulse' 
              : 'bg-gradient-to-r from-brand-amber to-amber-600 hover:from-brand-amber-hover hover:to-amber-500 text-atelier-950 shadow-atelier-sm'"
            title="AI 全自动构思冲突、起草正文与精修归档">
            <Loader2 v-if="state.pipelineState.active" class="w-3 h-3 animate-spin" />
            <Zap v-else class="w-3 h-3 fill-current" />
            <span>{{ state.pipelineState.active ? (state.pipelineState.message || '正在成章...') : '一键成章' }}</span>
          </button>

          <!-- VSCode 风格 Diff 对比模式开关 -->
          <button 
            v-if="hasRewriteDiff"
            @click="showDiffViewer = !showDiffViewer"
            class="ml-1.5 px-2.5 py-1 rounded-md text-[11px] font-bold transition flex items-center gap-1.5 cursor-pointer shadow-sm border"
            :class="showDiffViewer 
              ? 'bg-brand-cyan text-atelier-950 border-brand-cyan shadow-cyan-glow' 
              : 'bg-brand-cyan/15 hover:bg-brand-cyan/25 text-brand-cyan border-brand-cyan/40'"
            title="对比改写前后差异 (VSCode 风格行与词级 Diff)">
            <GitCompare class="w-3.5 h-3.5" />
            <span>{{ showDiffViewer ? '返回编辑' : '改写对比' }}</span>
            <span class="text-[10px] font-mono px-1 py-0.2 rounded" :class="showDiffViewer ? 'bg-atelier-950/20 text-atelier-950' : 'bg-brand-cyan/25 text-brand-cyan'">
              {{ diffWordDeltaStr }}
            </span>
          </button>
        </div>

        <!-- 风格与字数预算快速选择 (单章微调，继承作品级设定) -->
        <div class="hidden xl:flex items-center gap-2 bg-atelier-850 px-2.5 py-1 rounded-md border border-atelier-750 text-xs" title="单章特调（未修改则继承全书设定）">
          <span class="text-[10px] text-brand-amber font-mono font-medium">单章特调:</span>
          <span class="text-[10px] text-ink-400">风格</span>
          <select 
            v-model="state.narrativeStyle" 
            class="bg-transparent text-[11px] text-brand-amber font-medium focus:outline-none cursor-pointer">
            <option value="hardboiled" class="bg-atelier-900 text-ink-100">冷峻凝练</option>
            <option value="high_tension" class="bg-atelier-900 text-ink-100">热血张力</option>
            <option value="classical" class="bg-atelier-900 text-ink-100">古典雅致</option>
            <option value="vernacular" class="bg-atelier-900 text-ink-100">生动市井</option>
            <option value="cinematic" class="bg-atelier-900 text-ink-100">全景镜头</option>
          </select>

          <span class="text-atelier-700">|</span>

          <span class="text-[10px] text-ink-400">字数</span>
          <select 
            v-model="state.wordsTarget" 
            class="bg-transparent text-[11px] text-brand-amber font-medium focus:outline-none cursor-pointer">
            <option :value="1500" class="bg-atelier-900 text-ink-100">1500 字</option>
            <option :value="2000" class="bg-atelier-900 text-ink-100">2000 字</option>
            <option :value="2500" class="bg-atelier-900 text-ink-100">2500 字</option>
            <option :value="3000" class="bg-atelier-900 text-ink-100">3000 字</option>
          </select>
        </div>

        <!-- 右侧核心功能组 (亲密性整合，清晰克制) -->
        <div class="flex items-center gap-1.5 shrink-0">
          <div 
            v-if="state.isSavingDraft" 
            class="hidden lg:flex items-center gap-1 text-[10px] text-brand-amber font-mono mr-1">
            <Loader2 class="w-3 h-3 animate-spin text-brand-amber" />
            <span>正在保存...</span>
          </div>
          <div 
            v-else-if="state.lastSavedAt" 
            class="hidden lg:flex items-center gap-1.5 text-[10px] text-ink-400 font-mono mr-1"
            title="快捷键 Cmd/Ctrl + S 手动存盘">
            <span class="w-1.5 h-1.5 rounded-full bg-emerald-400"></span>
            <span>已保存 {{ formatSaveTime(state.lastSavedAt) }}</span>
          </div>

          <button 
            @click="handleManualSaveDraft" 
            :disabled="state.isSavingDraft"
            class="px-2.5 py-1 text-[11px] font-medium bg-brand-amber/15 hover:bg-brand-amber/25 text-brand-amber rounded-md border border-brand-amber/30 transition flex items-center gap-1 cursor-pointer disabled:opacity-50"
            title="手动保存草稿">
            <Loader2 v-if="state.isSavingDraft" class="w-3 h-3 animate-spin text-brand-amber" />
            <Save v-else class="w-3 h-3" />
            <span class="hidden sm:inline">{{ state.isSavingDraft ? '存盘中...' : '保存' }}</span>
          </button>

          <button 
            v-if="state.workbench?.content || (state.workbench?.beats || []).some(b => b.action) || state.workbench?.coreConflict"
            @click="handleDiscardDraft" 
            class="px-2 py-1 text-[11px] font-medium bg-atelier-850 hover:bg-brand-rose/20 text-ink-400 hover:text-brand-rose rounded-md border border-atelier-750 hover:border-brand-rose/30 transition flex items-center gap-1 cursor-pointer"
            title="清空当前草稿">
            <Trash2 class="w-3 h-3" />
            <span class="hidden md:inline">重置</span>
          </button>

          <button 
            @click="actions.runLinter" 
            :disabled="state.isLinting"
            class="px-2.5 py-1 text-[11px] font-medium bg-atelier-850 hover:bg-atelier-800 text-ink-300 hover:text-ink-100 rounded-md border border-atelier-750 transition flex items-center gap-1 cursor-pointer disabled:opacity-50"
            title="检查 AI 腔调与文字质量">
            <Loader2 v-if="state.isLinting" class="w-3 h-3 animate-spin text-brand-amber" />
            <Sparkles v-else class="w-3 h-3 text-brand-amber" />
            <span class="hidden sm:inline">{{ state.isLinting ? '体检中...' : '体检' }}</span>
          </button>

          <button 
            @click="actions.runDeterministicSanitize" 
            :disabled="state.isSanitizing"
            class="hidden lg:flex px-2.5 py-1 text-[11px] font-medium bg-atelier-850 hover:bg-atelier-800 text-ink-300 hover:text-ink-100 rounded-md border border-atelier-750 transition items-center gap-1 cursor-pointer disabled:opacity-50"
            title="去掉冒号与冗余连词">
            <Loader2 v-if="state.isSanitizing" class="w-3 h-3 animate-spin text-brand-cyan" />
            <Sparkles v-else class="w-3 h-3 text-brand-cyan" />
            <span>{{ state.isSanitizing ? '去壳中...' : '去壳' }}</span>
          </button>

          <button 
            @click="openHumanTouchesModal" 
            class="hidden lg:flex px-2.5 py-1 text-[11px] font-medium bg-atelier-850 hover:bg-atelier-800 text-ink-300 hover:text-ink-100 rounded-md border border-atelier-750 transition items-center gap-1 cursor-pointer"
            title="增添生活气息与细节描摹">
            <HeartHandshake class="w-3 h-3 text-brand-amber" />
            <span>润色</span>
          </button>

          <span 
            v-if="state.reviewResult"
            class="text-[10px] font-mono font-bold px-2 py-0.5 rounded border"
            :class="state.reviewResult.score >= 80 ? 'bg-brand-emerald/15 text-brand-emerald border-brand-emerald/30' : 'bg-brand-rose/15 text-brand-rose border-brand-rose/30'">
            {{ state.reviewResult.score }}分
          </span>
        </div>
      </div>

      <!-- 小白新手创作引导横幅 (无章节且设定不完善时温馨提示) -->
      <div 
        v-if="showBeginnerBanner" 
        class="mx-4 my-2.5 p-2.5 bg-brand-amber/10 border border-brand-amber/25 rounded-lg flex items-center justify-between gap-3 text-xs shrink-0 select-none">
        <div class="flex items-center gap-2 min-w-0">
          <span class="text-sm">💡</span>
          <p class="text-ink-200 text-[11px] truncate">
            <strong class="text-brand-amber">新手建议：</strong> 先立规矩更顺手！建议先去【作品设定】推演规则或在【设定集】添加人物，再回来写正文更连贯。
          </p>
        </div>
        <div class="flex items-center gap-1.5 shrink-0">
          <button 
            @click="actions.setActiveTab('framework')" 
            class="px-2 py-0.5 bg-brand-amber/20 hover:bg-brand-amber/30 text-brand-amber rounded text-[11px] font-medium transition cursor-pointer">
            去设定作品
          </button>
          <button 
            @click="actions.setActiveTab('codex')" 
            class="px-2 py-0.5 bg-atelier-850 hover:bg-atelier-800 text-ink-300 rounded text-[11px] transition cursor-pointer">
            去建人物
          </button>
          <button 
            @click="dismissBeginnerBanner" 
            class="px-1.5 py-0.5 text-ink-500 hover:text-ink-300 text-xs transition cursor-pointer" 
            title="关闭提示">
            ✕
          </button>
        </div>
      </div>

      <!-- 划词改写浮动条 -->
      <div class="px-4 py-1.5 bg-atelier-900/60 border-b border-atelier-750 flex items-center justify-between text-xs shrink-0 select-none overflow-x-auto">
        <div class="flex items-center gap-1 flex-nowrap shrink-0">
          <span class="text-ink-400 text-[10px] font-mono uppercase tracking-wider mr-1 shrink-0">划词改写:</span>
          <button 
            v-for="act in inlineActionButtons" 
            :key="act.action"
            @click="runInlineAction(act.action)"
            :disabled="inlineActionLoading || !state.workbench.content"
            :title="act.desc"
            class="px-2.5 py-1 rounded-md bg-atelier-850 hover:bg-atelier-800 text-ink-200 hover:text-ink-50 border border-atelier-700 text-[11px] transition flex items-center gap-1 cursor-pointer disabled:opacity-40 shrink-0">
            <component :is="act.icon" class="w-3 h-3 text-brand-amber" />
            <span>{{ act.label }}</span>
          </button>
          <button 
            @click="openCustomInlinePrompt"
            :disabled="inlineActionLoading || !state.workbench.content"
            title="按自己的要求让 AI 改写"
            class="px-2.5 py-1 rounded-md bg-atelier-850 hover:bg-atelier-800 text-brand-amber border border-atelier-700 text-[11px] transition flex items-center gap-1 cursor-pointer disabled:opacity-40 shrink-0">
            <Wand2 class="w-3 h-3 text-brand-amber" />
            <span>自定义</span>
          </button>
        </div>

        <div v-if="inlineActionLoading" class="flex items-center gap-1 text-[11px] text-brand-amber shrink-0 ml-3">
          <Loader2 class="w-3 h-3 animate-spin" />
          <span>正在改写...</span>
        </div>
      </div>

      <!-- 划词 AI 结果差分对比横幅 (VSCode 词级 Diff 增强) -->
      <!-- 划词 AI 结果差分对比横幅 (专业 VSCode Git Diff 对比) -->
      <div v-if="showInlineDiff && inlineActionResult" class="p-3.5 bg-atelier-900/95 border-b border-brand-amber/30 space-y-2.5 shrink-0 animate-fade-in shadow-md">
        <div class="flex items-center justify-between text-xs">
          <div class="flex items-center gap-2">
            <span class="font-bold text-brand-amber flex items-center gap-1.5">
              <Sparkles class="w-3.5 h-3.5" />
              <span>伴写改写结果 ({{ inlineActionResult.action }})</span>
            </span>
            <span v-if="inlineDiffStats" class="text-[10px] font-mono px-1.5 py-0.5 rounded bg-brand-amber/15 text-brand-amber border border-brand-amber/30">
              原 {{ inlineDiffStats.oldWordCount }} 字 ➔ 新 {{ inlineDiffStats.newWordCount }} 字 ({{ inlineDiffStats.netDiff >= 0 ? '+' + inlineDiffStats.netDiff : inlineDiffStats.netDiff }}字)
            </span>
          </div>
          <div class="flex items-center gap-2">
            <!-- 模式切换：双栏 vs 单列 vs 纯文 -->
            <div class="flex items-center bg-atelier-950 rounded border border-atelier-750 p-0.5 text-[10px]">
              <button 
                type="button" 
                @click="inlineDiffViewMode = 'split'" 
                class="px-2 py-0.5 rounded transition cursor-pointer"
                :class="inlineDiffViewMode === 'split' ? 'bg-brand-amber text-atelier-950 font-bold' : 'text-ink-400 hover:text-ink-200'">
                左右对比
              </button>
              <button 
                type="button" 
                @click="inlineDiffViewMode = 'unified'" 
                class="px-2 py-0.5 rounded transition cursor-pointer"
                :class="inlineDiffViewMode === 'unified' ? 'bg-brand-amber text-atelier-950 font-bold' : 'text-ink-400 hover:text-ink-200'">
                单列流
              </button>
              <button 
                type="button" 
                @click="inlineDiffViewMode = 'result'" 
                class="px-2 py-0.5 rounded transition cursor-pointer"
                :class="inlineDiffViewMode === 'result' ? 'bg-brand-amber text-atelier-950 font-bold' : 'text-ink-400 hover:text-ink-200'">
                仅改写稿
              </button>
            </div>
            <button @click="applyInlineResult" class="px-2.5 py-1 bg-brand-emerald hover:bg-emerald-500 text-atelier-950 font-bold rounded text-xs transition cursor-pointer flex items-center gap-1">
              <Check class="w-3 h-3" />
              <span>采用改写</span>
            </button>
            <button @click="discardInlineResult" class="px-2.5 py-1 bg-atelier-800 hover:bg-atelier-750 text-ink-300 rounded text-xs transition cursor-pointer">
              ✕ 放弃
            </button>
          </div>
        </div>

        <!-- 专业 Git Diff 视图 (左右双栏 / 单列统一) -->
        <div v-if="inlineDiffViewMode === 'split' || inlineDiffViewMode === 'unified'" class="rounded-lg border border-atelier-750 overflow-hidden bg-atelier-950 min-h-[140px] max-h-72 overflow-y-auto">
          <CodeDiff
            :old-string="lastSelectedText || ''"
            :new-string="inlineActionResult.result || ''"
            :output-format="inlineDiffViewMode === 'split' ? 'side-by-side' : 'line-by-line'"
            theme="dark"
            diff-style="word"
            language="markdown"
            filename="原句 / 原段落"
            new-filename="伴写打磨稿"
            class="h-full"
          />
        </div>
        <!-- 纯文本视图 -->
        <div v-else class="p-3 bg-atelier-950 rounded-lg border border-atelier-750 text-xs text-ink-100 leading-relaxed font-serif max-h-56 overflow-y-auto whitespace-pre-wrap select-text">
          {{ inlineActionResult.result }}
        </div>
      </div>

      <!-- 全篇改写 VSCode Diff 对比面板 -->
      <div v-if="showDiffViewer && hasRewriteDiff" class="flex-1 min-h-0 flex flex-col p-2 md:p-4 overflow-hidden">
        <RewriteDiffViewer 
          :old-text="state.workbench.originalDraftBeforeRewrite" 
          :new-text="state.workbench.content" 
          @apply="handleApplyWholeRewrite" 
          @revert="handleRevertWholeRewrite" 
          @close="showDiffViewer = false" 
        />
      </div>

      <!-- 纯净排版正文文本框 (呼吸感居中稿纸) -->
      <div v-else class="flex-1 min-h-0 flex overflow-y-auto p-4 md:p-8 justify-center">
        <div class="w-full max-w-4xl xl:max-w-5xl 2xl:max-w-6xl flex flex-col h-full bg-atelier-900/20 rounded-xl p-4 md:p-8 border border-atelier-800/40 shadow-inner transition-all relative">
          
          <!-- 改写差异未确认醒目横幅 (提示查看 Git Diff 对比) -->
          <div 
            v-if="hasRewriteDiff && !showDiffViewer"
            class="mb-4 p-3.5 rounded-xl bg-cyan-950/40 border border-brand-cyan/40 text-xs flex flex-col sm:flex-row sm:items-center justify-between gap-3 shrink-0 animate-fade-in shadow-cyan-glow/10">
            <div class="flex items-center gap-2.5 min-w-0">
              <div class="w-7 h-7 rounded-lg bg-brand-cyan/20 border border-brand-cyan/30 flex items-center justify-center text-brand-cyan shrink-0 shadow-cyan-glow/20">
                <GitCompare class="w-4 h-4" />
              </div>
              <div class="min-w-0">
                <div class="font-bold text-ink-50 flex items-center gap-2">
                  <span>正文已完成 AI 精修改写</span>
                  <span class="text-[10px] font-mono px-1.5 py-0.2 rounded bg-brand-cyan/20 text-brand-cyan border border-brand-cyan/30 font-bold">
                    相比原稿: {{ diffWordDeltaStr }}
                  </span>
                </div>
                <div class="text-[11px] text-ink-300 truncate">
                  当前处于修改稿编辑态，点击可进入 Git 双栏左右逐句比对。
                </div>
              </div>
            </div>

            <div class="flex items-center gap-2 shrink-0 self-end sm:self-center">
              <button 
                @click="showDiffViewer = true" 
                class="px-3.5 py-1.5 bg-brand-cyan hover:bg-cyan-400 text-atelier-950 font-bold rounded-lg text-xs shadow-cyan-glow transition flex items-center gap-1.5 cursor-pointer">
                <GitCompare class="w-3.5 h-3.5" />
                <span>查看 Git 对比 (Diff)</span>
              </button>
              <button 
                @click="handleApplyWholeRewrite" 
                class="px-2.5 py-1.5 bg-atelier-850 hover:bg-atelier-800 text-brand-emerald border border-brand-emerald/30 font-medium rounded-lg text-xs transition cursor-pointer">
                采用
              </button>
              <button 
                @click="handleRevertWholeRewrite" 
                class="px-2.5 py-1.5 bg-atelier-850 hover:bg-atelier-800 text-ink-400 hover:text-brand-rose border border-atelier-750 font-medium rounded-lg text-xs transition cursor-pointer">
                还原原稿
              </button>
            </div>
          </div>

          <!-- 异常报错通知横幅 -->
          <div 
            v-if="renderError"
            class="mb-4 p-3.5 rounded-xl bg-brand-rose/15 border border-brand-rose/40 text-xs flex items-start justify-between gap-3 shrink-0 animate-fade-in">
            <div class="space-y-1.5 flex-1 min-w-0">
              <div class="font-bold text-brand-rose flex items-center gap-1.5">
                <AlertCircle class="w-4 h-4 shrink-0" />
                <span>{{ isTimeoutErrorText(renderError) ? '生成超时 / 算力拥堵' : '起草 / 精修异常' }}</span>
              </div>
              <p class="text-[11px] text-ink-200 leading-relaxed font-sans select-text break-words">{{ renderError }}</p>
              <div v-if="isTimeoutErrorText(renderError)" class="mt-2 text-[11px] text-amber-300 bg-amber-500/10 border border-amber-500/20 px-2.5 py-1.5 rounded-lg flex items-center justify-between gap-2">
                <span>💡 建议：免费/预览通道长文本算力有限，建议前往设置切换为主力写作模型（如 DeepSeek-V3 / Qwen-2.5）</span>
                <button 
                  @click="actions.switchToGlobal('config')" 
                  class="underline font-bold hover:text-white cursor-pointer shrink-0 text-amber-200">
                  前往设置 →
                </button>
              </div>
            </div>
            <div class="flex items-center gap-2 shrink-0">
              <button 
                @click="state.activeStep === 5 ? handleRewriteDraft() : handleRenderScene()" 
                class="px-2.5 py-1 bg-brand-rose/20 hover:bg-brand-rose/30 text-brand-rose border border-brand-rose/40 rounded text-xs transition cursor-pointer font-bold">
                重试
              </button>
              <button 
                @click="emit('clear-render-error')" 
                class="p-1 text-ink-400 hover:text-ink-200 cursor-pointer">
                <X class="w-3.5 h-3.5" />
              </button>
            </div>
          </div>

          <!-- 全自动流水线运行中 沉浸式动态看板 -->
          <div 
            v-if="state.pipelineState.active"
            class="mb-4 p-4 rounded-xl bg-atelier-950/95 border border-brand-amber/60 shadow-amber-glow/20 space-y-2.5 shrink-0 animate-subtle-pulse">
            <div class="flex items-center justify-between">
              <div class="flex items-center gap-2">
                <Loader2 class="w-4 h-4 text-brand-amber animate-spin" />
                <span class="text-xs font-bold text-brand-amber tracking-wide">
                  全自动流水线执行中 · 阶段: {{ state.pipelineState.phase || 'RUNNING' }}
                </span>
              </div>
              <span v-if="state.pipelineState.tokens?.total_tokens" class="text-xs font-mono font-bold px-2 py-0.5 rounded bg-brand-amber/15 text-brand-amber border border-brand-amber/30">
                已消耗 {{ state.pipelineState.tokens.total_tokens }} Tokens
              </span>
            </div>
            <div class="w-full bg-atelier-800 h-1.5 rounded-full overflow-hidden relative">
              <div class="h-full bg-gradient-to-r from-brand-amber to-amber-400 animate-pulse rounded-full w-full"></div>
            </div>
            <div class="flex items-center justify-between text-[11px] text-ink-300">
              <span class="font-medium text-ink-100">{{ state.pipelineState.message || '正在全力推演正文流水线，请稍候...' }}</span>
              <span class="text-ink-400 font-mono">画布已锁定输入</span>
            </div>
          </div>

          <!-- 正文初稿渲染中 动态动效横幅 (Generating Overlay Banner) -->
          <div 
            v-if="state.isRenderingScene && !state.pipelineState.active"
            class="mb-4 p-4 rounded-xl bg-atelier-950/95 border border-brand-amber/50 shadow-amber-glow/20 space-y-2.5 shrink-0 animate-subtle-pulse">
            <div class="flex items-center justify-between">
              <div class="flex items-center gap-2">
                <Loader2 class="w-4 h-4 text-brand-amber animate-spin" />
                <span class="text-xs font-bold text-brand-amber tracking-wide">
                  大模型正在起草第 {{ computedState.currentWorkingChapterIndex.value }} 章正文初稿...
                </span>
              </div>
              <span class="text-xs font-mono font-bold px-2 py-0.5 rounded bg-brand-amber/15 text-brand-amber border border-brand-amber/30">
                已耗时 {{ renderElapsedSec }}s
              </span>
            </div>
            <div class="w-full bg-atelier-800 h-1.5 rounded-full overflow-hidden relative">
              <div class="h-full bg-gradient-to-r from-brand-amber to-amber-400 animate-pulse rounded-full w-full"></div>
            </div>
            <div class="flex items-center justify-between text-[11px] text-ink-300">
              <span>目标字数: <strong class="text-ink-100">{{ state.wordsTarget || 2000 }}</strong> 字 · 叙事风格: <strong class="text-ink-100">{{ currentToneName }}</strong></span>
              <span class="text-ink-400">大模型生成约需 10 ~ 30 秒，请稍候</span>
            </div>
          </div>

          <!-- 定向返工精修中 动态动效横幅 -->
          <div 
            v-if="state.isRewriting && !state.pipelineState.active"
            class="mb-4 p-4 rounded-xl bg-atelier-950/95 border border-brand-amber/50 space-y-2.5 shrink-0 animate-subtle-pulse">
            <div class="flex items-center justify-between">
              <div class="flex items-center gap-2">
                <Loader2 class="w-4 h-4 text-brand-amber animate-spin" />
                <span class="text-xs font-bold text-brand-amber tracking-wide">
                  正在依照修改建议执行第 {{ state.rewriteLoopCount }} 轮精修...
                </span>
              </div>
              <span class="text-xs font-mono font-bold px-2 py-0.5 rounded bg-brand-amber/15 text-brand-amber border border-brand-amber/30">
                已耗时 {{ rewriteElapsedSec }}s
              </span>
            </div>
            <div class="w-full bg-atelier-800 h-1.5 rounded-full overflow-hidden relative">
              <div class="h-full bg-gradient-to-r from-brand-amber to-amber-400 animate-pulse rounded-full w-full"></div>
            </div>
            <p class="text-[11px] text-ink-300">正在逐句修缮，去除机械套路与生硬腔调...</p>
          </div>

          <textarea 
            id="prose-textarea"
            v-model="state.workbench.content"
            @input="handleProseInput"
            :disabled="state.isRenderingScene || state.isRewriting || state.pipelineState.active"
            class="flex-1 w-full bg-transparent text-ink-100 text-base md:text-[17px] leading-[2.1] font-serif resize-none focus:outline-none placeholder-ink-500 selection:bg-brand-amber/30 selection:text-ink-50 tracking-wide prose-canvas"
            :class="{ 'opacity-60 cursor-wait': state.isRenderingScene || state.isRewriting || state.pipelineState.active }"
            :placeholder="state.pipelineState.active ? '全自动流水线执行中，正文输入已锁定以防止竞态覆盖……' : (state.isRenderingScene ? '正文起草中，预计需要 10 ~ 30 秒，请稍候……' : '在此起草正文，可直接写作或由 AI 起草。')"></textarea>
        </div>
      </div>
    </div>
</template>

<script setup>
import { ref, computed, watch, onUnmounted } from 'vue';
import { state, computedState, actions, notify, dialogs } from '../../stores/appState';
import { createDefaultWorkbench } from '../../stores/domain/projectScope';
import { api } from '../../api/client';
import RewriteDiffViewer from './RewriteDiffViewer.vue';
import { CodeDiff } from 'v-code-diff';
import { computeWordDiff, computeDiffStats } from '../../utils/diff.js';
import {
  AlertCircle,
  AlertTriangle,
  RotateCcw,
  Sparkles,
  Zap,
  PanelLeft,
  PenTool,
  Save,
  Trash2,
  Wand2,
  HeartHandshake,
  Loader2,
  X,
  Scissors,
  Maximize2,
  Minimize2,
  Eye,
  MessageSquare,
  GitCompare,
  Check
} from 'lucide-vue-next';

const props = defineProps({
  renderError: {
    type: String,
    default: null
  },
  renderElapsedSec: {
    type: Number,
    default: 0
  },
  rewriteElapsedSec: {
    type: Number,
    default: 0
  },
  onRenderScene: {
    type: Function,
    default: () => {}
  },
  onRewriteDraft: {
    type: Function,
    default: () => {}
  },
  onHumanTouches: {
    type: Function,
    default: () => {}
  }
});

const emit = defineEmits(['clear-render-error']);

const handleRenderScene = () => props.onRenderScene?.();
const handleRewriteDraft = () => props.onRewriteDraft?.();
const openHumanTouchesModal = () => props.onHumanTouches?.();

function isTimeoutErrorText(msg) {
  if (!msg) return false;
  const lower = String(msg).toLowerCase();
  return lower.includes('超时') || lower.includes('timeout') || lower.includes('deadline') || lower.includes('算力') || lower.includes('headers');
}

const dismissedBannerProjectId = ref(null);
const showBeginnerBanner = computed(() => {
  if (!state.currentProject) return false;
  if (dismissedBannerProjectId.value === state.currentProject.id) return false;
  const hasChapters = (state.chapters || []).length > 0;
  const hasFramework = Boolean(state.currentProject.framework?.core_concept);
  const hasCodex = (state.codexEntries || []).length > 0;
  return !hasChapters && (!hasFramework || !hasCodex);
});

function dismissBeginnerBanner() {
  if (state.currentProject) {
    dismissedBannerProjectId.value = state.currentProject.id;
  }
}

const toneMap = {
  hardboiled: '冷峻凝练',
  high_tension: '热血张力',
  classical: '古典雅致',
  vernacular: '生动市井',
  cinematic: '全景镜头'
};
const currentToneName = computed(() => {
  return toneMap[state.narrativeStyle] || '冷峻凝练';
});

const inlineActionButtons = [
  { action: 'rewrite', label: '润色', desc: '重新打磨语句，提升文采', icon: Scissors },
  { action: 'expand', label: '扩写', desc: '丰富细节，补充场面描写', icon: Maximize2 },
  { action: 'shorten', label: '精简', desc: '删减赘字，使节奏更明快', icon: Minimize2 },
  { action: 'sensory', label: '生动', desc: '强化五感，增强现场画面感', icon: Eye },
  { action: 'dialogue', label: '改对话', desc: '使人物台词更符合性格', icon: MessageSquare },
];

function loadHaltDraft() {
  if (actions.loadHaltDraftForManualEdit) {
    actions.loadHaltDraftForManualEdit();
  } else {
    const idx = state.pipelineState?.haltInfo?.chapterIndex || state.editingChapterIndex || (state.chapters?.length ? state.chapters.length + 1 : 1);
    state.editingChapterIndex = idx;
    state.activeStep = 5;
    actions.setActiveTab('workbench');
  }
}

async function resetToNewChapter() {
  if (state.workbench.content.trim() || state.workbench.coreConflict.trim()) {
    const ok = await dialogs.confirm({
      title: '放弃精修',
      message: '未保存的修改将会丢失，是否切换至新章节？',
      type: 'warning',
      confirmText: '确认切换',
    });
    if (!ok) return;
  }
  state.editingChapterIndex = null;
  state.pipelineState.halted = false;
  state.pipelineState.haltInfo = null;
  state.workbench = createDefaultWorkbench();
  state.reviewResult = null;
  state.activeStep = 1;
  notify('已切换', '已重置画布为创作最新章节', 'info');
}

// 全篇改写 VSCode Diff 状态（双向同步单一事实源 state.workbench.showDiffViewer）
const showDiffViewer = computed({
  get: () => Boolean(state.workbench?.showDiffViewer),
  set: (val) => {
    if (state.workbench) {
      state.workbench.showDiffViewer = Boolean(val);
    }
  }
});
const inlineDiffViewMode = ref('split'); // 'split' (左右对比), 'unified' (单列流), 'result' (纯改后文)

const hasRewriteDiff = computed(() => {
  return Boolean(
    state.workbench?.originalDraftBeforeRewrite &&
    state.workbench.originalDraftBeforeRewrite.trim() !== state.workbench.content.trim()
  );
});

// 当精修任务执行完毕且产生实质正文改写差异时，自动切入左右双栏 Git Diff 审阅视图
watch(() => state.isRewriting, (isRewriting, prevIsRewriting) => {
  if (prevIsRewriting && !isRewriting && hasRewriteDiff.value) {
    if (state.workbench) {
      state.workbench.showDiffViewer = true;
    }
  }
});

const diffStats = computed(() => {
  if (!hasRewriteDiff.value) return null;
  return computeDiffStats(state.workbench.originalDraftBeforeRewrite, state.workbench.content);
});

const diffWordDeltaStr = computed(() => {
  if (!diffStats.value) return '';
  const d = diffStats.value.netDiff;
  return d >= 0 ? `+${d}字` : `${d}字`;
});

function handleApplyWholeRewrite() {
  if (state.workbench) {
    state.workbench.originalDraftBeforeRewrite = null;
    state.workbench.showDiffViewer = false;
  }
  actions.runLinter();
  actions.saveCheckpoint();
  notify('已采用改写', '已保留改写内容为当前正文', 'success');
}

function handleRevertWholeRewrite() {
  if (state.workbench) {
    if (state.workbench.originalDraftBeforeRewrite) {
      state.workbench.content = state.workbench.originalDraftBeforeRewrite;
    }
    state.workbench.originalDraftBeforeRewrite = null;
    state.workbench.showDiffViewer = false;
  }
  actions.runLinter();
  actions.saveCheckpoint();
  notify('已还原原文', '已回退至改写前手稿', 'info');
}

// 划词 Inline Actions
const inlineActionLoading = ref(false);
const inlineActionResult = ref(null);
const showInlineDiff = ref(false);
let lastSelectedText = '';

const inlineWordDiff = computed(() => {
  if (!lastSelectedText || !inlineActionResult.value?.result) return [];
  return computeWordDiff(lastSelectedText, inlineActionResult.value.result);
});

const inlineDiffStats = computed(() => {
  if (!lastSelectedText || !inlineActionResult.value?.result) return null;
  return computeDiffStats(lastSelectedText, inlineActionResult.value.result);
});

function getSelectedOrFullText() {
  const textarea = document.getElementById('prose-textarea');
  if (textarea && textarea.selectionStart !== textarea.selectionEnd) {
    const sel = textarea.value.substring(textarea.selectionStart, textarea.selectionEnd);
    if (sel.trim()) return sel;
  }
  return state.workbench.content;
}

async function runInlineAction(actionType, instruction = '') {
  const text = getSelectedOrFullText();
  if (!text || !text.trim()) return notify('请先选中文本', '可在正文中选定段落后运行伴写动作', 'info');
  lastSelectedText = text;
  inlineActionLoading.value = true;
  try {
    const res = await api.inlineAction({
      action: actionType,
      selection: text,
      instruction: instruction,
      surrounding_context: `当前正文字数: ${state.workbench.content.length}`,
    });
    inlineActionResult.value = res;
    showInlineDiff.value = true;
  } catch (e) {
    notify('AI 动作执行失败', e.message, 'error');
  } finally {
    inlineActionLoading.value = false;
  }
}

async function openCustomInlinePrompt() {
  const instr = await dialogs.prompt({
    title: '自定义指令',
    message: '输入你对选中文本的修改要求：',
    placeholder: '例如：增加环境描写、用动作代替心理描写、强化对抗氛围...',
    multiline: true,
    confirmText: '开始改写',
  });
  if (instr && instr.trim()) {
    runInlineAction('custom', instr.trim());
  }
}

async function applyInlineResult() {
  if (!inlineActionResult.value) return;
  if (lastSelectedText && lastSelectedText !== state.workbench.content && state.workbench.content.includes(lastSelectedText)) {
    state.workbench.content = state.workbench.content.replace(lastSelectedText, inlineActionResult.value.result);
  } else {
    state.workbench.content = inlineActionResult.value.result;
  }
  showInlineDiff.value = false;
  inlineActionResult.value = null;
  actions.runLinter();
  await actions.saveCheckpoint();
  notify('已采用改写', '正文已更新', 'success');
}

function discardInlineResult() {
  showInlineDiff.value = false;
  inlineActionResult.value = null;
}

async function handleManualSaveDraft() {
  const ok = await actions.saveCheckpoint({}, false);
  if (ok) {
    notify('草稿已保存', '本章进度与正文已同步保存', 'success');
  }
}

async function handleDiscardDraft() {
  await actions.discardCheckpoint(computedState.currentWorkingChapterIndex.value);
}

let proseDebounceTimer = null;
function handleProseInput() {
  actions.runLinter();
  if (state.reviewResult) {
    state.reviewResult = null;
  }
  if (proseDebounceTimer) clearTimeout(proseDebounceTimer);
  proseDebounceTimer = setTimeout(async () => {
    if (state.currentProject && (state.workbench.content.trim() || state.workbench.coreConflict.trim())) {
      await actions.saveCheckpoint({}, true);
    }
  }, 1500);
}

// 切换作品时取消在途的防抖自动保存与划词改写缓冲，
// 防止旧作品的正文在延迟触发时被写入新作品的断点。
watch(() => state.currentProject?.id, () => {
  if (proseDebounceTimer) {
    clearTimeout(proseDebounceTimer);
    proseDebounceTimer = null;
  }
  inlineActionResult.value = null;
  showInlineDiff.value = false;
  lastSelectedText = '';
});

function formatSaveTime(ts) {
  if (!ts) return '';
  const d = new Date(ts);
  return `${d.getHours().toString().padStart(2, '0')}:${d.getMinutes().toString().padStart(2, '0')}:${d.getSeconds().toString().padStart(2, '0')}`;
}

onUnmounted(() => {
  if (proseDebounceTimer) clearTimeout(proseDebounceTimer);
});
</script>
