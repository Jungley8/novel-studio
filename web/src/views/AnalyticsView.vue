<template>
  <!-- 当尚未选择任何项目时的零态引导 (照 ForecastView.vue 写法) -->
  <div v-if="!state.currentProject" class="flex-1 flex flex-col items-center justify-center p-8 bg-atelier-950 text-center select-none">
    <div class="max-w-md w-full p-8 rounded-2xl bg-atelier-900/60 border border-atelier-750 backdrop-blur-xl shadow-atelier-lg space-y-6">
      <div class="w-16 h-16 rounded-2xl bg-gradient-to-br from-brand-amber/20 to-amber-600/10 border border-brand-amber/30 mx-auto flex items-center justify-center text-brand-amber shadow-amber-glow">
        <BarChart3 class="w-8 h-8" />
      </div>
      <div class="space-y-2">
        <h3 class="text-xl font-bold font-serif text-ink-50 tracking-wide">
          全书体检
        </h3>
        <p class="text-xs text-ink-300 leading-relaxed">
          请先在左侧选择或新建一部小说作品，即可开启质量体检、人物出场与情节起伏节奏分析。
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
  <div v-else class="flex-1 p-6 overflow-y-auto space-y-6">
    <!-- 头部工具栏 -->
    <div class="flex flex-col md:flex-row md:items-center justify-between gap-4 border-b border-atelier-750 pb-5">
      <div>
        <div class="flex items-center gap-2.5">
          <div class="w-8 h-8 rounded-lg bg-brand-amber/10 border border-brand-amber/25 flex items-center justify-center text-brand-amber shadow-amber-glow">
            <BarChart3 class="w-4 h-4" />
          </div>
          <div>
            <h2 class="text-base font-serif font-bold text-ink-50 tracking-wide">全书体检</h2>
            <p class="text-xs text-ink-400 mt-0.5">质量探针诊断、人物登场频率与剧情起伏节奏。</p>
          </div>
        </div>
      </div>

      <div class="flex items-center gap-3">
        <div class="flex gap-1 bg-atelier-900 p-1 rounded-lg border border-atelier-750 text-xs">
          <button 
            @click="subTab = 'bench'" 
            :class="subTab === 'bench' 
              ? 'bg-atelier-800 text-brand-amber font-semibold shadow-atelier-sm' 
              : 'text-ink-400 hover:text-ink-200'" 
            class="px-3 py-1.5 rounded-md transition cursor-pointer flex items-center gap-1.5">
            <ShieldCheck class="w-3.5 h-3.5" />
            <span>质量体检</span>
          </button>
          <button 
            @click="subTab = 'heatmap'" 
            :class="subTab === 'heatmap' 
              ? 'bg-atelier-800 text-brand-amber font-semibold shadow-atelier-sm' 
              : 'text-ink-400 hover:text-ink-200'" 
            class="px-3 py-1.5 rounded-md transition cursor-pointer flex items-center gap-1.5">
            <Flame class="w-3.5 h-3.5" />
            <span>人物登场</span>
          </button>
          <button 
            @click="subTab = 'tension'" 
            :class="subTab === 'tension' 
              ? 'bg-atelier-800 text-brand-amber font-semibold shadow-atelier-sm' 
              : 'text-ink-400 hover:text-ink-200'" 
            class="px-3 py-1.5 rounded-md transition cursor-pointer flex items-center gap-1.5">
            <Activity class="w-3.5 h-3.5" />
            <span>情节起伏</span>
          </button>
        </div>

        <button 
          @click="handleRefresh" 
          :disabled="isLoadingBench"
          class="flex items-center gap-1.5 px-3 py-1.5 bg-atelier-850 hover:bg-atelier-800 text-ink-300 text-xs rounded-md border border-atelier-750 transition cursor-pointer disabled:opacity-60">
          <RefreshCw class="w-3.5 h-3.5 text-ink-400" :class="{ 'animate-spin': isLoadingBench }" />
          <span>刷新</span>
        </button>
      </div>
    </div>

    <!-- 子视图 0: 质量体检 (Bench Probes) -->
    <div v-if="subTab === 'bench'" class="space-y-4">
      <!-- Loading 态 -->
      <div v-if="isLoadingBench && !benchReport" class="py-20 text-center bg-atelier-900 border border-atelier-750 rounded-xl space-y-3 shadow-atelier-md">
        <RefreshCw class="w-6 h-6 animate-spin text-brand-amber mx-auto" />
        <h4 class="text-xs font-semibold text-ink-200">正在运行质量探针诊断...</h4>
        <p class="text-[11px] text-ink-400">遍历全书章节字数、章型连续性、伏笔闭环与状态机重放。</p>
      </div>

      <!-- Error 态 -->
      <div v-else-if="benchError" class="p-6 bg-rose-500/10 border border-rose-500/30 rounded-xl text-center space-y-3">
        <AlertCircle class="w-6 h-6 text-rose-400 mx-auto" />
        <div class="text-xs text-rose-200 font-semibold">{{ benchError }}</div>
        <button 
          @click="loadBench" 
          class="px-3 py-1.5 bg-atelier-850 hover:bg-atelier-800 text-ink-200 text-xs rounded-md border border-atelier-750 transition cursor-pointer">
          重新加载
        </button>
      </div>

      <!-- 报告正常渲染 (含 0 章空态) -->
      <div v-else-if="benchReport" class="space-y-4">
        <!-- 0 章空态提示横幅 -->
        <div 
          v-if="benchReport.chapters === 0" 
          class="bg-atelier-900 border border-dashed border-atelier-750 rounded-xl p-4 flex items-center gap-3 text-xs shadow-atelier-sm">
          <div class="w-8 h-8 rounded-lg bg-atelier-800 flex items-center justify-center text-brand-amber shrink-0">
            <AlertCircle class="w-4 h-4" />
          </div>
          <div>
            <div class="font-bold text-ink-100 font-serif">尚无已归档章节</div>
            <div class="text-[11px] text-ink-400 mt-0.5">当前作品尚未归档章节，下方展示全景指标基线零值。归档章节后将自动计算字数体检、章型节奏与跨章回声。</div>
          </div>
        </div>

        <!-- 六块指标卡网格 -->
        <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          <!-- 卡片 1: 字数体检 -->
          <div class="bg-atelier-900 border border-atelier-750 rounded-xl p-5 shadow-atelier-md flex flex-col justify-between space-y-4 hover:border-brand-amber/30 transition">
            <div>
              <div class="flex items-center justify-between pb-3 border-b border-atelier-800">
                <div class="flex items-center gap-2">
                  <div class="w-7 h-7 rounded-lg bg-brand-amber/10 border border-brand-amber/25 flex items-center justify-center text-brand-amber">
                    <FileText class="w-3.5 h-3.5" />
                  </div>
                  <div>
                    <h4 class="text-xs font-serif font-bold text-ink-100">字数体检</h4>
                    <span class="text-[10px] text-ink-400">章节篇幅与配额达标度</span>
                  </div>
                </div>
                <span class="text-[10px] font-mono px-2 py-0.5 rounded bg-atelier-850 text-ink-300 border border-atelier-750">
                  目标 {{ benchReport.word_count.target }} 字
                </span>
              </div>

              <div class="grid grid-cols-2 gap-2 pt-3">
                <div class="bg-atelier-950/60 p-2.5 rounded-lg border border-atelier-800/80">
                  <span class="text-[10px] text-ink-400 block mb-0.5">平均字数 (avg)</span>
                  <span class="text-sm font-mono font-bold text-brand-amber">{{ benchReport.word_count.avg }}</span>
                  <span class="text-[10px] text-ink-400 ml-1">字</span>
                </div>
                <div class="bg-atelier-950/60 p-2.5 rounded-lg border border-atelier-800/80">
                  <span class="text-[10px] text-ink-400 block mb-0.5">极值区间 (min / max)</span>
                  <span class="text-xs font-mono font-bold text-ink-200">{{ benchReport.word_count.min }} ~ {{ benchReport.word_count.max }}</span>
                  <span class="text-[10px] text-ink-400 ml-1">字</span>
                </div>
              </div>
            </div>

            <div class="pt-2 border-t border-atelier-800/60 flex items-center justify-between text-xs font-mono">
              <span class="text-ink-400 text-[11px]">低于及格线 (under_target):</span>
              <span v-if="benchReport.word_count.under_target === 0" class="inline-flex items-center gap-1 px-2 py-0.5 rounded text-[11px] font-bold bg-emerald-500/15 text-emerald-400 border border-emerald-500/30">
                <CheckCircle2 class="w-3 h-3" />
                <span>全部及格</span>
              </span>
              <span v-else class="inline-flex items-center gap-1 px-2 py-0.5 rounded text-[11px] font-bold bg-rose-500/15 text-rose-400 border border-rose-500/30">
                <AlertTriangle class="w-3 h-3" />
                <span>{{ benchReport.word_count.under_target }} 章未及格 (&lt;80%)</span>
              </span>
            </div>
          </div>

          <!-- 卡片 2: 章型节奏 -->
          <div class="bg-atelier-900 border border-atelier-750 rounded-xl p-5 shadow-atelier-md flex flex-col justify-between space-y-4 hover:border-brand-amber/30 transition">
            <div>
              <div class="flex items-center justify-between pb-3 border-b border-atelier-800">
                <div class="flex items-center gap-2">
                  <div class="w-7 h-7 rounded-lg bg-brand-amber/10 border border-brand-amber/25 flex items-center justify-center text-brand-amber">
                    <Layers class="w-3.5 h-3.5" />
                  </div>
                  <div>
                    <h4 class="text-xs font-serif font-bold text-ink-100">章型节奏</h4>
                    <span class="text-[10px] text-ink-400">叙事类型分布与连续同型</span>
                  </div>
                </div>
                <span class="text-[10px] font-mono px-2 py-0.5 rounded bg-atelier-850 text-ink-300 border border-atelier-750">
                  最长同型: {{ benchReport.chapter_types.max_consecutive }} 章
                </span>
              </div>

              <div class="space-y-2 pt-3">
                <div class="bg-atelier-950/60 p-2.5 rounded-lg border border-atelier-800/80 text-xs">
                  <div class="flex items-center justify-between text-[11px] mb-1">
                    <span class="text-ink-400">最长连续章序号 (max_run_chapters):</span>
                    <span v-if="benchReport.chapter_types.max_run_chapters && benchReport.chapter_types.max_run_chapters.length > 0" class="font-mono text-brand-amber font-semibold">
                      第 {{ benchReport.chapter_types.max_run_chapters.join('、') }} 章
                    </span>
                    <span v-else class="text-ink-500 font-mono">无连续同型</span>
                  </div>
                </div>

                <div class="bg-atelier-950/60 p-2.5 rounded-lg border border-atelier-800/80">
                  <span class="text-[10px] text-ink-400 block mb-1.5">章型分布 (distribution):</span>
                  <div v-if="Object.keys(benchReport.chapter_types.distribution || {}).length > 0" class="flex flex-wrap gap-1.5">
                    <span 
                      v-for="(cnt, cType) in benchReport.chapter_types.distribution" 
                      :key="cType"
                      class="text-[11px] font-mono px-2 py-0.5 rounded bg-atelier-900 border border-atelier-750 text-ink-200 flex items-center gap-1">
                      <span class="text-brand-amber font-bold">{{ cType }}:</span>
                      <span>{{ cnt }} 章</span>
                    </span>
                  </div>
                  <span v-else class="text-[11px] text-ink-500">尚无章型分布</span>
                </div>
              </div>
            </div>

            <div class="pt-2 border-t border-atelier-800/60 text-[11px] text-ink-400 font-mono">
              <span>最长连续: {{ benchReport.chapter_types.max_consecutive }} 章连同型</span>
            </div>
          </div>

          <!-- 卡片 3: 伏笔账本 -->
          <div class="bg-atelier-900 border border-atelier-750 rounded-xl p-5 shadow-atelier-md flex flex-col justify-between space-y-4 hover:border-brand-amber/30 transition">
            <div>
              <div class="flex items-center justify-between pb-3 border-b border-atelier-800">
                <div class="flex items-center gap-2">
                  <div class="w-7 h-7 rounded-lg bg-brand-amber/10 border border-brand-amber/25 flex items-center justify-center text-brand-amber">
                    <Anchor class="w-3.5 h-3.5" />
                  </div>
                  <div>
                    <h4 class="text-xs font-serif font-bold text-ink-100">伏笔账本</h4>
                    <span class="text-[10px] text-ink-400">生命周期状态与逾期审计</span>
                  </div>
                </div>
                <span 
                  :class="benchReport.hooks.overdue && benchReport.hooks.overdue.length > 0 ? 'bg-rose-500/15 text-rose-400 border-rose-500/30' : 'bg-emerald-500/15 text-emerald-400 border-emerald-500/30'" 
                  class="text-[10px] font-mono px-2 py-0.5 rounded border font-bold">
                  逾期: {{ benchReport.hooks.overdue ? benchReport.hooks.overdue.length : 0 }}
                </span>
              </div>

              <!-- 四类计数 -->
              <div class="grid grid-cols-4 gap-1.5 pt-3 text-center">
                <div class="bg-atelier-950/60 p-2 rounded-lg border border-atelier-800/80">
                  <span class="text-[10px] text-ink-400 block mb-0.5">开启中</span>
                  <span class="text-xs font-mono font-bold text-ink-200">{{ benchReport.hooks.open }}</span>
                </div>
                <div class="bg-atelier-950/60 p-2 rounded-lg border border-atelier-800/80">
                  <span class="text-[10px] text-ink-400 block mb-0.5">发酵中</span>
                  <span class="text-xs font-mono font-bold text-brand-amber">{{ benchReport.hooks.fermenting }}</span>
                </div>
                <div class="bg-atelier-950/60 p-2 rounded-lg border border-atelier-800/80">
                  <span class="text-[10px] text-ink-400 block mb-0.5">已揭晓</span>
                  <span class="text-xs font-mono font-bold text-emerald-400">{{ benchReport.hooks.resolved }}</span>
                </div>
                <div class="bg-atelier-950/60 p-2 rounded-lg border border-atelier-800/80">
                  <span class="text-[10px] text-ink-400 block mb-0.5">已废弃</span>
                  <span class="text-xs font-mono font-bold text-ink-500">{{ benchReport.hooks.abandoned }}</span>
                </div>
              </div>

              <!-- 逾期列表 -->
              <div class="mt-2.5">
                <span class="text-[10px] text-ink-400 block mb-1">逾期伏笔列表 (overdue):</span>
                <div v-if="benchReport.hooks.overdue && benchReport.hooks.overdue.length > 0" class="max-h-24 overflow-y-auto space-y-1">
                  <div 
                    v-for="h in benchReport.hooks.overdue" 
                    :key="h.id"
                    class="flex items-center justify-between text-[11px] p-1.5 rounded bg-rose-500/10 border border-rose-500/20 text-rose-300">
                    <span class="truncate max-w-[140px] font-medium">{{ h.title }}</span>
                    <span class="font-mono text-[10px] shrink-0 text-rose-400">应收第{{ h.target_chapter }}章</span>
                  </div>
                </div>
                <div v-else class="text-[11px] text-emerald-400/80 flex items-center gap-1.5 bg-emerald-500/10 p-1.5 rounded border border-emerald-500/20">
                  <CheckCircle2 class="w-3 h-3 shrink-0" />
                  <span>暂无逾期未回收伏笔</span>
                </div>
              </div>
            </div>

            <div class="pt-2 border-t border-atelier-800/60 text-[11px] text-ink-400 font-mono">
              <span>总计伏笔: {{ benchReport.hooks.open + benchReport.hooks.fermenting + benchReport.hooks.resolved + benchReport.hooks.abandoned }} 条</span>
            </div>
          </div>

          <!-- 卡片 4: 账本重放 -->
          <div class="bg-atelier-900 border border-atelier-750 rounded-xl p-5 shadow-atelier-md flex flex-col justify-between space-y-4 hover:border-brand-amber/30 transition">
            <div>
              <div class="flex items-center justify-between pb-3 border-b border-atelier-800">
                <div class="flex items-center gap-2">
                  <div class="w-7 h-7 rounded-lg bg-brand-amber/10 border border-brand-amber/25 flex items-center justify-center text-brand-amber">
                    <RotateCcw class="w-3.5 h-3.5" />
                  </div>
                  <div>
                    <h4 class="text-xs font-serif font-bold text-ink-100">账本重放</h4>
                    <span class="text-[10px] text-ink-400">状态机逐章增量基线审计</span>
                  </div>
                </div>
                <span 
                  :class="benchReport.ledger_replay.clean ? 'bg-emerald-500/15 text-emerald-400 border-emerald-500/30' : 'bg-rose-500/15 text-rose-400 border-rose-500/30'" 
                  class="text-[11px] font-mono px-2.5 py-0.5 rounded border font-bold">
                  {{ benchReport.ledger_replay.clean ? 'CLEAN' : 'DRIFT' }}
                </span>
              </div>

              <div class="space-y-2 pt-3">
                <div class="bg-atelier-950/60 p-2.5 rounded-lg border border-atelier-800/80">
                  <span class="text-[10px] text-ink-400 block mb-1">审计声明 (caveat):</span>
                  <p class="text-[11px] text-ink-300 leading-relaxed font-sans">{{ benchReport.ledger_replay.caveat }}</p>
                </div>

                <!-- 漂移明细可折叠 -->
                <div class="bg-atelier-950/60 rounded-lg border border-atelier-800/80 overflow-hidden">
                  <button 
                    @click="showDrifts = !showDrifts" 
                    class="w-full px-2.5 py-2 flex items-center justify-between text-[11px] text-ink-300 hover:text-ink-100 transition cursor-pointer">
                    <span class="flex items-center gap-1.5 font-medium">
                      <span>漂移明细 (drifts)</span>
                      <span class="font-mono text-[10px] text-ink-400">({{ benchReport.ledger_replay.drifts ? benchReport.ledger_replay.drifts.length : 0 }})</span>
                    </span>
                    <ChevronUp v-if="showDrifts" class="w-3.5 h-3.5 text-ink-400" />
                    <ChevronDown v-else class="w-3.5 h-3.5 text-ink-400" />
                  </button>

                  <div v-if="showDrifts" class="px-2.5 pb-2.5 pt-1 border-t border-atelier-800 max-h-28 overflow-y-auto space-y-1">
                    <div v-if="benchReport.ledger_replay.drifts && benchReport.ledger_replay.drifts.length > 0">
                      <div 
                        v-for="(drift, i) in benchReport.ledger_replay.drifts" 
                        :key="i"
                        class="text-[10px] font-mono p-1 rounded bg-rose-500/10 text-rose-300 border border-rose-500/20 mb-1">
                        {{ drift }}
                      </div>
                    </div>
                    <div v-else class="text-[11px] text-emerald-400/80 flex items-center gap-1 py-1">
                      <CheckCircle2 class="w-3 h-3" />
                      <span>完全吻合，无漂移项</span>
                    </div>
                  </div>
                </div>
              </div>
            </div>

            <div class="pt-2 border-t border-atelier-800/60 text-[11px] text-ink-400 font-mono">
              <span>状态审计: {{ benchReport.ledger_replay.clean ? '一致性验证通过' : '检测到差异' }}</span>
            </div>
          </div>

          <!-- 卡片 5: 跨章回声 -->
          <div class="bg-atelier-900 border border-atelier-750 rounded-xl p-5 shadow-atelier-md flex flex-col justify-between space-y-4 hover:border-brand-amber/30 transition">
            <div>
              <div class="flex items-center justify-between pb-3 border-b border-atelier-800">
                <div class="flex items-center gap-2">
                  <div class="w-7 h-7 rounded-lg bg-brand-amber/10 border border-brand-amber/25 flex items-center justify-center text-brand-amber">
                    <Repeat class="w-3.5 h-3.5" />
                  </div>
                  <div>
                    <h4 class="text-xs font-serif font-bold text-ink-100">跨章回声</h4>
                    <span class="text-[10px] text-ink-400">词汇重叠与语癖检测 (20章窗口)</span>
                  </div>
                </div>
                <span class="text-[10px] font-mono px-2 py-0.5 rounded bg-atelier-850 text-ink-300 border border-atelier-750">
                  回声探针
                </span>
              </div>

              <div class="grid grid-cols-2 gap-2 pt-3">
                <div class="bg-atelier-950/60 p-2.5 rounded-lg border border-atelier-800/80">
                  <span class="text-[10px] text-ink-400 block mb-0.5">总短语数 (total_phrases)</span>
                  <span class="text-sm font-mono font-bold text-brand-amber">{{ benchReport.echo.total_phrases }}</span>
                  <span class="text-[10px] text-ink-400 ml-1">处</span>
                </div>
                <div class="bg-atelier-950/60 p-2.5 rounded-lg border border-atelier-800/80">
                  <span class="text-[10px] text-ink-400 block mb-0.5">命中章数 (chapters_hit)</span>
                  <span class="text-sm font-mono font-bold text-ink-200">{{ benchReport.echo.chapters_hit }}</span>
                  <span class="text-[10px] text-ink-400 ml-1">章</span>
                </div>
              </div>

              <div class="mt-2 bg-atelier-950/60 p-2.5 rounded-lg border border-atelier-800/80 text-[11px] text-ink-400">
                以每章前序 20 章文本为对照样本，自动标记高频跨章回声，辅助精修词汇多样性。
              </div>
            </div>

            <div class="pt-2 border-t border-atelier-800/60 text-[11px] text-ink-400 font-mono">
              <span>回声率: {{ benchReport.chapters > 0 ? ((benchReport.echo.chapters_hit / benchReport.chapters) * 100).toFixed(0) : 0 }}% 章节受影响</span>
            </div>
          </div>

          <!-- 卡片 6: 文风契约 -->
          <div class="bg-atelier-900 border border-atelier-750 rounded-xl p-5 shadow-atelier-md flex flex-col justify-between space-y-4 hover:border-brand-amber/30 transition">
            <div>
              <div class="flex items-center justify-between pb-3 border-b border-atelier-800">
                <div class="flex items-center gap-2">
                  <div class="w-7 h-7 rounded-lg bg-brand-amber/10 border border-brand-amber/25 flex items-center justify-center text-brand-amber">
                    <Sparkles class="w-3.5 h-3.5" />
                  </div>
                  <div>
                    <h4 class="text-xs font-serif font-bold text-ink-100">文风契约</h4>
                    <span class="text-[10px] text-ink-400">黄金样本与语气常态锚定</span>
                  </div>
                </div>
                <span 
                  :class="benchReport.style_vault_seeded ? 'bg-emerald-500/15 text-emerald-400 border-emerald-500/30' : 'bg-brand-amber/15 text-brand-amber border-brand-amber/30'" 
                  class="text-[10px] font-mono px-2 py-0.5 rounded border font-bold">
                  {{ benchReport.style_vault_seeded ? '已种子化' : '未种子化' }}
                </span>
              </div>

              <div class="space-y-2 pt-3">
                <div class="bg-atelier-950/60 p-2.5 rounded-lg border border-atelier-800/80">
                  <span class="text-[10px] text-ink-400 block mb-1">契约锚点状态 (style_vault_seeded):</span>
                  <div v-if="benchReport.style_vault_seeded" class="text-[11px] text-emerald-400 leading-relaxed flex items-start gap-1.5">
                    <CheckCircle2 class="w-3.5 h-3.5 shrink-0 mt-0.5" />
                    <span>黄金文风片段已固化，已建立风格锚点，后续章节生成将自动遵循文风契约。</span>
                  </div>
                  <div v-else class="text-[11px] text-brand-amber/90 leading-relaxed flex items-start gap-1.5">
                    <AlertTriangle class="w-3.5 h-3.5 shrink-0 mt-0.5" />
                    <span>尚未种子化（需前序归档章节提炼），当前使用默认预设叙事基线。</span>
                  </div>
                </div>
              </div>
            </div>

            <div class="pt-2 border-t border-atelier-800/60 text-[11px] text-ink-400 font-mono">
              <span>锚点库: {{ benchReport.style_vault_seeded ? '就绪 (Active)' : '待固化' }}</span>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- 子视图 A: 人物出场热力图 -->
    <div v-if="subTab === 'heatmap'" class="space-y-4">
      <div 
        v-if="state.analyticsHeatmap?.chapters?.length > 0 && state.analyticsHeatmap?.entities?.length > 0" 
        class="bg-atelier-900 border border-atelier-750 rounded-xl p-5 overflow-x-auto shadow-atelier-md">
        
        <table class="w-full text-left text-xs border-collapse">
          <thead>
            <tr class="border-b border-atelier-750/80 text-ink-400">
              <th class="p-3 font-bold sticky left-0 bg-atelier-900 z-10 w-48 font-serif">人物 / 词条</th>
              <th class="p-3 font-bold text-center w-24 font-mono">总频次</th>
              <th 
                v-for="ch in state.analyticsHeatmap.chapters" 
                :key="ch.chapter_index" 
                class="p-3 font-mono text-center min-w-20">
                第 {{ ch.chapter_index }} 章
              </th>
            </tr>
          </thead>
          <tbody class="divide-y divide-atelier-800">
            <tr 
              v-for="ent in state.analyticsHeatmap.entities" 
              :key="ent.entry_id" 
              class="hover:bg-atelier-850/40 transition">
              <td class="p-3 font-medium text-ink-100 sticky left-0 bg-atelier-900/95 flex items-center gap-2">
                <span 
                  class="w-2.5 h-2.5 rounded-full shrink-0 ring-1 ring-atelier-750" 
                  :style="{ backgroundColor: ent.color_tag || '#e5a93c' }"></span>
                <span class="truncate max-w-40 font-serif font-semibold">{{ ent.name }}</span>
              </td>
              <td class="p-3 text-center font-mono font-bold text-brand-amber">
                {{ ent.total_mentions }}
              </td>
              <td 
                v-for="ch in state.analyticsHeatmap.chapters" 
                :key="ch.chapter_index" 
                class="p-3 text-center font-mono">
                <span 
                  v-if="ent.chapter_counts && ent.chapter_counts[ch.chapter_index]" 
                  class="inline-block px-2 py-0.5 rounded text-[11px] font-bold bg-brand-amber/15 text-brand-amber border border-brand-amber/30 shadow-amber-glow">
                  {{ ent.chapter_counts[ch.chapter_index] }}
                </span>
                <span v-else class="text-atelier-700">·</span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <div 
        v-else 
        class="text-center py-20 bg-atelier-900 border border-dashed border-atelier-750 rounded-xl space-y-3">
        <div class="w-12 h-12 rounded-full bg-atelier-800 flex items-center justify-center mx-auto text-ink-400">
          <Flame class="w-6 h-6" />
        </div>
        <h3 class="text-sm font-semibold text-ink-200">暂无可供热力分析的数据</h3>
        <p class="text-xs text-ink-400 max-w-md mx-auto">
          请在全域百科中创建条目并在生产手稿中撰写章节，系统将自动汇总全书时空出场频次。
        </p>
      </div>
    </div>

    <!-- 子视图 B: 全书张力心流曲线 -->
    <div v-if="subTab === 'tension'" class="space-y-4">
      <div 
        v-if="state.analyticsTension?.points?.length > 0" 
        class="bg-atelier-900 border border-atelier-750 rounded-xl p-5 space-y-5 shadow-atelier-md">
        
        <div class="flex items-center justify-between text-xs border-b border-atelier-800 pb-3">
          <span class="font-serif font-bold text-ink-100">全书场景戏剧张力分布阶梯 (1 - 10 梯度)：</span>
          <div class="flex items-center gap-2 font-mono">
            <span class="text-ink-400">平均张力:</span>
            <strong class="text-brand-amber text-sm">{{ (state.analyticsTension.avg_tension || 0).toFixed(1) }}</strong>
            <span class="text-ink-400">/ 10</span>
          </div>
        </div>

        <!-- 场景张力直方图条目 -->
        <div class="space-y-2.5 pt-1">
          <div 
            v-for="(pt, idx) in state.analyticsTension.points" 
            :key="idx" 
            class="flex items-center gap-3.5 text-xs p-2 rounded-lg bg-atelier-950/60 border border-atelier-800/80 hover:border-brand-amber/30 transition">
            <span class="w-32 font-mono text-ink-400 truncate shrink-0">
              第 {{ pt.chapter_index }} 章 · 场 {{ pt.scene_index }}
            </span>
            <span class="w-44 text-ink-200 font-serif font-medium truncate shrink-0">
              {{ pt.scene_title || pt.chapter_title }}
            </span>
            <div class="flex-1 bg-atelier-850 h-3 rounded-full overflow-hidden flex items-center p-0.5 border border-atelier-750">
              <div 
                :style="{ width: (pt.tension_level * 10) + '%' }" 
                :class="tensionBarClass(pt.tension_level)" 
                class="h-full rounded-full transition-all duration-500 shadow-sm"></div>
            </div>
            <span class="w-16 font-mono font-bold text-right shrink-0" :class="tensionTextColor(pt.tension_level)">
              {{ pt.tension_level }} / 10
            </span>
          </div>
        </div>
      </div>

      <div 
        v-else 
        class="text-center py-20 bg-atelier-900 border border-dashed border-atelier-750 rounded-xl space-y-3">
        <div class="w-12 h-12 rounded-full bg-atelier-800 flex items-center justify-center mx-auto text-ink-400">
          <Activity class="w-6 h-6" />
        </div>
        <h3 class="text-sm font-semibold text-ink-200">暂无场景张力数据</h3>
        <p class="text-xs text-ink-400 max-w-md mx-auto">
          请在矩阵大纲 (The Matrix) 中规划场景并设定戏剧张力等级。
        </p>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, watch, onMounted } from 'vue';
import { state, actions } from '../stores/appState';
import { api } from '../api/client';
import { 
  BarChart3, 
  Flame, 
  Activity, 
  RefreshCw, 
  ShieldCheck, 
  FileText, 
  Layers, 
  Anchor, 
  RotateCcw, 
  Repeat, 
  Sparkles, 
  Plus, 
  AlertCircle, 
  ChevronDown, 
  ChevronUp, 
  CheckCircle2, 
  AlertTriangle 
} from 'lucide-vue-next';

const subTab = ref('bench');
const benchReport = ref(null);
const isLoadingBench = ref(false);
const benchError = ref(null);
const showDrifts = ref(false);

async function loadBench() {
  if (!state.currentProject) {
    benchReport.value = null;
    return;
  }
  isLoadingBench.value = true;
  benchError.value = null;
  try {
    const report = await api.getBenchReport(state.currentProject.id);
    benchReport.value = report;
  } catch (err) {
    benchError.value = err.message || '获取质量体检报告失败';
    console.error('load bench report error:', err);
  } finally {
    isLoadingBench.value = false;
  }
}

async function handleRefresh() {
  await Promise.allSettled([
    actions.loadAnalytics ? actions.loadAnalytics() : Promise.resolve(),
    loadBench(),
  ]);
}

watch(
  () => [state.currentProject?.id, state.activeTab],
  ([newProjId, newTab], [oldProjId, oldTab]) => {
    if (newProjId !== oldProjId) {
      // 作品切换：先清空旧报告，避免加载期间展示上一个作品的数据
      benchReport.value = null;
    }
    if (!newProjId) {
      benchReport.value = null;
      return;
    }
    if (newTab === 'analytics' && (newTab !== oldTab || newProjId !== oldProjId)) {
      loadBench();
    }
  },
  { immediate: true }
);

onMounted(() => {
  if (state.currentProject && state.activeTab === 'analytics') {
    loadBench();
  }
});

function tensionBarClass(level) {
  if (level >= 8) return 'bg-rose-500 shadow-rose-500/50';
  if (level >= 5) return 'bg-brand-amber shadow-amber-500/50';
  return 'bg-emerald-500 shadow-emerald-500/50';
}

function tensionTextColor(level) {
  if (level >= 8) return 'text-rose-400';
  if (level >= 5) return 'text-brand-amber';
  return 'text-emerald-400';
}
</script>
