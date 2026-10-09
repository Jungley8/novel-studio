<template>
  <!-- 当尚未选择任何项目时的极简美学引导页 (Zero State) -->
  <div v-if="!state.currentProject" class="flex-1 flex flex-col items-center justify-center p-8 bg-atelier-950 text-center relative overflow-hidden select-none">
    <div class="max-w-md w-full p-8 rounded-2xl bg-atelier-900/60 border border-atelier-750 backdrop-blur-xl shadow-atelier-lg space-y-6">
      <div class="w-16 h-16 rounded-2xl bg-gradient-to-br from-brand-amber/20 to-amber-600/10 border border-brand-amber/30 mx-auto flex items-center justify-center text-brand-amber text-2xl font-serif font-bold shadow-amber-glow">
        墨
      </div>
      <div class="space-y-2">
        <h3 class="text-xl font-bold font-serif text-ink-50 tracking-wide">
          开始创作
        </h3>
        <p class="text-xs text-ink-300 leading-relaxed">
          构思情节与起草正文，开启你的小说长篇连载。
        </p>
      </div>
      <div class="pt-2 flex flex-col gap-2.5">
        <button 
          @click="state.showNewProjectModal = true"
          class="w-full py-2.5 px-4 bg-gradient-to-r from-brand-amber to-amber-600 hover:from-brand-amber-hover hover:to-amber-500 text-atelier-950 font-bold text-xs rounded-lg transition shadow-atelier-md flex items-center justify-center gap-2 cursor-pointer">
          <Plus class="w-4 h-4" />
          <span>新建作品</span>
        </button>
        <button 
          @click="state.activeTab = 'config'"
          class="w-full py-2 px-4 bg-atelier-850 hover:bg-atelier-800 text-ink-200 border border-atelier-700 rounded-lg text-xs transition flex items-center justify-center gap-1.5 cursor-pointer">
          <Settings class="w-3.5 h-3.5 text-ink-400" />
          <span>配置 AI</span>
        </button>
      </div>
    </div>
  </div>

  <!-- 主创作手稿工作台 (3 栏弹性自适应架构) -->
  <div v-else class="flex-1 flex min-h-0 overflow-hidden relative">
    <!-- 禅模式悬浮退出胶囊条 -->
    <div 
      v-if="state.isZenMode" 
      class="absolute top-4 right-6 z-30 flex items-center gap-2">
      <button 
        @click="state.isZenMode = false"
        class="flex items-center gap-1.5 px-3 py-1.5 rounded-full bg-atelier-900/80 hover:bg-atelier-800 text-ink-300 hover:text-ink-50 border border-atelier-700/80 backdrop-blur-md text-xs transition shadow-atelier-md cursor-pointer"
        title="退出专注模式 (快捷键 ESC)">
        <Minimize2 class="w-3.5 h-3.5 text-brand-amber" />
        <span>退出专注</span>
      </button>
    </div>

    <!-- 左栏：六步工序 (宽 320px，支持折叠与无障碍滚动) -->
    <div 
      v-show="state.showWorkflowPanel && !state.isZenMode" 
      class="w-80 border-r border-atelier-750 bg-atelier-900/40 flex flex-col min-h-0 shrink-0 overflow-hidden transition-all duration-200 select-none">
      
      <!-- 工步导航指示条 -->
      <div class="p-3 border-b border-atelier-750 bg-atelier-900/70 shrink-0">
        <div class="flex items-center justify-between mb-2">
          <div class="flex items-center gap-1.5">
            <Sparkles class="w-3.5 h-3.5 text-brand-amber" />
            <span class="text-xs font-bold text-ink-100 font-serif">创作工序</span>
          </div>
          <span class="text-[10px] font-mono px-1.5 py-0.5 rounded bg-atelier-800 text-brand-amber border border-atelier-700">
            第 {{ computedState.currentWorkingChapterIndex.value }} 章
          </span>
        </div>

        <!-- 6 步流线型步进器 -->
        <div class="grid grid-cols-6 gap-1 bg-atelier-950/60 p-1 rounded-lg border border-atelier-800">
          <button 
            v-for="step in workflowSteps" 
            :key="step.id"
            @click="goToStep(step.id)"
            class="flex flex-col items-center py-1.5 rounded transition relative group cursor-pointer"
            :title="step.desc"
            :class="state.activeStep === step.id 
              ? 'bg-atelier-800 text-brand-amber shadow-atelier-sm font-bold' 
              : 'text-ink-400 hover:text-ink-200 hover:bg-atelier-850/80'">
            <span class="text-[11px] font-mono leading-none">{{ step.id }}</span>
            <span class="text-[9px] mt-1 leading-none">{{ step.shortLabel }}</span>
            <span 
              v-if="stepBadge(step.id)" 
              class="w-1.5 h-1.5 rounded-full absolute top-1 right-1"
              :class="stepBadgeClass(step.id)"></span>
          </button>
        </div>
      </div>

      <!-- 工步面板内容区 (添加 min-h-0 与平滑纵向滚动) -->
      <div class="flex-1 min-h-0 p-3.5 overflow-y-auto space-y-4">
        <!-- STEP 1: 构思 -->
        <div v-show="state.activeStep === 1" class="space-y-3.5">
          <div>
            <div class="flex items-center justify-between">
              <span class="text-xs font-bold text-brand-amber flex items-center gap-1.5">
                <Target class="w-3.5 h-3.5" />
                <span>构思</span>
              </span>
              <span class="text-[10px] font-mono text-ink-400">第 {{ computedState.currentWorkingChapterIndex.value }} 章</span>
            </div>
            <p class="text-[11px] text-ink-400 mt-1">确定本章核心冲突与关键事件。</p>
          </div>

          <!-- 灵感参考标签 -->
          <div>
            <span class="text-[10px] uppercase font-mono tracking-wider text-ink-400 block mb-1.5">灵感参考：</span>
            <div class="grid grid-cols-2 gap-1.5">
              <button 
                v-for="preset in inspirationPresets" 
                :key="preset.title"
                @click="state.workbench.coreConflict = preset.conflict"
                class="px-2 py-1.5 text-left bg-atelier-850/90 hover:bg-atelier-800 text-ink-200 rounded border border-atelier-750 transition cursor-pointer group">
                <div class="font-semibold text-[11px] text-ink-100 group-hover:text-brand-amber transition-colors">{{ preset.title }}</div>
                <div class="text-[10px] text-ink-400 truncate">{{ preset.desc }}</div>
              </button>
            </div>
          </div>

          <!-- 待收伏笔 -->
          <div v-if="(computedState.activeHooksList.value || []).length > 0">
            <span class="text-[10px] uppercase font-mono tracking-wider text-ink-400 block mb-1.5">待收伏笔：</span>
            <div class="flex flex-wrap gap-1">
              <button 
                v-for="h in computedState.activeHooksList.value" 
                :key="h.id"
                @click="appendHookToConflict(h)"
                class="text-[10px] px-2 py-0.5 rounded bg-brand-amber/10 hover:bg-brand-amber/20 text-brand-amber border border-brand-amber/25 transition flex items-center gap-1 cursor-pointer"
                :title="h.details">
                <Anchor class="w-2.5 h-2.5" />
                <span>{{ h.title }} (第{{ h.target_chapter }}章)</span>
              </button>
            </div>
          </div>

          <div>
            <label class="block text-[11px] text-ink-300 mb-1 font-medium">剧情走向：</label>
            <textarea 
              v-model="state.workbench.coreConflict"
              rows="3" 
              class="w-full bg-atelier-950 border border-atelier-750 rounded-lg p-2.5 text-xs text-ink-100 placeholder-ink-500 resize-none focus-ring font-sans leading-relaxed"
              placeholder="例如：主角在拍卖会上洞悉残破丹炉藏有神念，引诱宿敌抬价反遭反噬..."></textarea>
          </div>

          <button 
            @click="handleDeriveBeats"
            :disabled="state.isGeneratingBeats || !state.workbench.coreConflict.trim()"
            class="w-full py-2.5 bg-gradient-to-r from-brand-amber to-amber-600 hover:from-brand-amber-hover hover:to-amber-500 text-atelier-950 font-bold text-xs rounded-lg shadow-atelier-sm transition flex items-center justify-center gap-1.5 cursor-pointer disabled:opacity-50">
            <Loader2 v-if="state.isGeneratingBeats" class="w-3.5 h-3.5 animate-spin" />
            <Sparkles v-else class="w-3.5 h-3.5" />
            <span>{{ state.isGeneratingBeats ? '正在构思大纲...' : '生成大纲' }}</span>
          </button>

          <button 
            v-if="state.workbench.beats.some(b => b.action)"
            @click="goToStep(2)"
            class="w-full py-2 bg-atelier-850 hover:bg-atelier-800 text-ink-300 hover:text-ink-100 text-xs rounded-lg border border-atelier-750 transition flex items-center justify-center gap-1 cursor-pointer">
            <span>已有分段大纲，前往查看 ➔</span>
          </button>
        </div>

        <!-- STEP 2: 大纲 -->
        <div v-show="state.activeStep === 2" class="space-y-3.5">
          <div class="flex items-center justify-between">
            <span class="text-xs font-bold text-brand-amber flex items-center gap-1.5">
              <GitCommit class="w-3.5 h-3.5" />
              <span>本章分段</span>
            </span>
            <button @click="handleDeriveBeats" class="text-[11px] text-ink-400 hover:text-brand-amber transition cursor-pointer">
              重新生成
            </button>
          </div>

          <div class="space-y-2">
            <div 
              v-for="(b, i) in state.workbench.beats" 
              :key="i"
              class="p-2.5 bg-atelier-950/80 border border-atelier-750 rounded-lg space-y-1.5">
              <div class="flex items-center justify-between">
                <span class="text-[11px] font-bold text-ink-100">{{ b.phase || `阶段 ${i + 1}` }}</span>
                <span class="text-[10px] font-mono px-1.5 py-0.5 rounded font-bold" :class="tensionBadgeClass(b.tension)">
                  张力 {{ b.tension }}/10
                </span>
              </div>
              <input 
                v-model="b.action" 
                class="w-full bg-atelier-850 border border-atelier-700 rounded px-2 py-1 text-xs text-ink-100 focus-ring" 
                placeholder="剧情发生的事情...">
              <input 
                v-model="b.expectation_broken" 
                class="w-full bg-atelier-850 border border-atelier-700 rounded px-2 py-1 text-[11px] text-ink-300 focus-ring" 
                placeholder="读者的心理预期与反转...">
            </div>
          </div>

          <!-- 角色变化预览 -->
          <div class="p-2.5 bg-atelier-950 rounded-lg border border-atelier-750 text-[11px] space-y-1">
            <span class="font-bold text-brand-emerald flex items-center gap-1">
              <ShieldCheck class="w-3 h-3" />
              <span>主角本章变化：</span>
            </span>
            <div class="text-ink-300">战力：<span class="text-ink-100">{{ state.workbench?.stateMutation?.power_delta || '无' }}</span></div>
            <div class="text-ink-300">物品：<span class="text-ink-100">{{ state.workbench?.stateMutation?.inventory_delta || '无' }}</span></div>
          </div>

          <button 
            @click="handleRenderScene"
            :disabled="state.isRenderingScene"
            class="w-full py-2.5 bg-gradient-to-r from-brand-amber to-amber-600 hover:from-brand-amber-hover hover:to-amber-500 text-atelier-950 font-bold text-xs rounded-lg shadow-atelier-sm transition flex items-center justify-center gap-1.5 cursor-pointer disabled:opacity-50">
            <Loader2 v-if="state.isRenderingScene" class="w-3.5 h-3.5 animate-spin" />
            <PenTool v-else class="w-3.5 h-3.5" />
            <span>{{ state.isRenderingScene ? '正在起草正文...' : '生成初稿' }}</span>
          </button>

          <button 
            v-if="state.workbench.content.trim()"
            @click="goToStep(3)"
            class="w-full py-2 bg-atelier-850 hover:bg-atelier-800 text-ink-300 hover:text-ink-100 text-xs rounded-lg border border-atelier-750 transition flex items-center justify-center gap-1 cursor-pointer">
            <span>正文初稿已就绪，调整文风 ➔</span>
          </button>
        </div>

        <!-- STEP 3: 起草 -->
        <div v-show="state.activeStep === 3" class="space-y-3.5">
          <div class="flex items-center justify-between">
            <span class="text-xs font-bold text-brand-amber flex items-center gap-1.5">
              <PenTool class="w-3.5 h-3.5" />
              <span>写作设置</span>
            </span>
            <span class="text-[10px] font-mono text-ink-400">{{ state.workbench.content.length }} 字已写</span>
          </div>

          <!-- 目标字数设定 -->
          <div class="p-3 bg-atelier-950 rounded-lg border border-atelier-750 space-y-2">
            <label class="block text-[11px] text-ink-300 font-medium">目标字数：</label>
            <div class="grid grid-cols-4 gap-1.5">
              <button 
                v-for="target in [1500, 2000, 2500, 3000]" 
                :key="target"
                @click="state.wordsTarget = target"
                class="py-1 text-[11px] font-mono rounded border transition cursor-pointer text-center"
                :class="state.wordsTarget === target ? 'bg-brand-amber/15 text-brand-amber border-brand-amber/40 font-bold' : 'bg-atelier-850 text-ink-300 border-atelier-750 hover:bg-atelier-800'">
                {{ target }}
              </button>
            </div>
          </div>

          <!-- 叙事风格设定 -->
          <div class="p-3 bg-atelier-950 rounded-lg border border-atelier-750 space-y-2">
            <label class="block text-[11px] text-ink-300 font-medium">叙事风格：</label>
            <div class="space-y-1.5">
              <button 
                v-for="tone in [
                  { id: 'hardboiled', name: '冷峻凝练', desc: '硬派简练，重动作与物态' },
                  { id: 'high_tension', name: '热血张力', desc: '节奏明快，冲突对抗强烈' },
                  { id: 'classical', name: '古典雅致', desc: '文笔沉郁，富有传统韵味' },
                  { id: 'vernacular', name: '生动市井', desc: '烟火气足，语言接地气' },
                  { id: 'cinematic', name: '全景镜头', desc: '画面感强，兼顾远景特写' }
                ]"
                :key="tone.id"
                @click="state.narrativeStyle = tone.id"
                class="w-full py-1.5 px-2.5 text-[11px] rounded border transition cursor-pointer text-left flex items-center justify-between"
                :class="state.narrativeStyle === tone.id ? 'bg-brand-amber/15 text-brand-amber border-brand-amber/40 font-bold' : 'bg-atelier-850 text-ink-300 border-atelier-750 hover:bg-atelier-800'">
                <span>{{ tone.name }}</span>
                <span class="text-[10px] opacity-75 font-sans">{{ tone.desc }}</span>
              </button>
            </div>
          </div>

          <button 
            @click="confirmAndRenderScene"
            :disabled="state.isRenderingScene"
            class="w-full py-2.5 bg-atelier-850 hover:bg-atelier-800 text-ink-100 font-medium text-xs rounded-lg border border-atelier-700 transition flex items-center justify-center gap-1.5 cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed">
            <Loader2 v-if="state.isRenderingScene" class="w-3.5 h-3.5 text-brand-amber animate-spin" />
            <RotateCcw v-else class="w-3.5 h-3.5 text-brand-amber" />
            <span>{{ state.isRenderingScene ? '正文起草中 (约需10~30秒)...' : '重新起草' }}</span>
          </button>

          <button 
            @click="goToStep(4)"
            :disabled="!state.workbench.content.trim()"
            class="w-full py-2.5 bg-gradient-to-r from-brand-amber to-amber-600 hover:from-brand-amber-hover hover:to-amber-500 text-atelier-950 font-bold text-xs rounded-lg shadow-atelier-sm transition flex items-center justify-center gap-1.5 cursor-pointer disabled:opacity-50">
            <span>检查文风 ➔</span>
          </button>
        </div>

        <!-- STEP 4: 体检 -->
        <div v-show="state.activeStep === 4" class="space-y-3.5">
          <div class="flex items-center justify-between">
            <span class="text-xs font-bold text-brand-amber flex items-center gap-1.5">
              <ShieldAlert class="w-3.5 h-3.5" />
              <span>文风体检</span>
            </span>
            <button @click="handleReviewDraft" :disabled="state.isReviewing" class="text-[11px] text-brand-amber hover:underline cursor-pointer flex items-center gap-1 disabled:opacity-50 font-medium">
              <Loader2 v-if="state.isReviewing" class="w-3 h-3 animate-spin" />
              <span>{{ state.isReviewing ? '正在体检...' : '开始体检' }}</span>
            </button>
          </div>

          <!-- 句式节奏指标卡 -->
          <div class="p-3 bg-atelier-950 rounded-lg border border-atelier-750 space-y-1.5">
            <div class="flex justify-between items-center text-xs">
              <span class="text-ink-300">句式节奏</span>
              <span class="font-mono font-bold" :class="(state.linterReport?.burstiness_score || 0) >= 45 ? 'text-brand-emerald' : 'text-brand-rose'">
                {{ state.linterReport?.burstiness_score ?? 0 }} 分
              </span>
            </div>
            <div class="w-full bg-atelier-800 h-1.5 rounded-full overflow-hidden">
              <div 
                :style="{ width: Math.min(100, state.linterReport?.burstiness_score || 0) + '%' }" 
                :class="(state.linterReport?.burstiness_score || 0) >= 45 ? 'bg-brand-emerald' : 'bg-brand-rose'" 
                class="h-full transition-all"></div>
            </div>
            <p class="text-[10px] text-ink-400">长短句交替，读来自然生动 (≥ 45 达标)。</p>
          </div>

          <!-- 套路用词 -->
          <div class="p-3 bg-atelier-950 rounded-lg border border-atelier-750 space-y-1">
            <div class="flex justify-between items-center text-xs">
              <span class="text-ink-300">套路用词：</span>
              <span class="font-mono text-xs font-bold" :class="(state.linterReport?.hit_banned_words || []).length === 0 ? 'text-brand-emerald' : 'text-brand-rose'">
                {{ (state.linterReport?.hit_banned_words || []).length }} 处发现
              </span>
            </div>
            <div v-if="(state.linterReport?.hit_banned_words || []).length > 0" class="flex flex-wrap gap-1 mt-1">
              <span v-for="w in (state.linterReport?.hit_banned_words || [])" :key="w" class="text-[10px] bg-brand-rose/10 text-brand-rose border border-brand-rose/25 px-1.5 py-0.5 rounded font-mono">
                {{ w }}
              </span>
            </div>
            <div v-else class="text-[10px] text-brand-emerald">未发现常见套路废词。</div>
          </div>

          <!-- 机械腔调 -->
          <div class="p-3 bg-atelier-950 rounded-lg border border-atelier-750 space-y-1">
            <div class="flex justify-between items-center text-xs">
              <span class="text-ink-300">机械腔调：</span>
              <span class="font-mono text-xs font-bold" :class="(state.linterReport?.empirical_tells || []).length === 0 ? 'text-brand-emerald' : 'text-brand-amber'">
                {{ (state.linterReport?.empirical_tells || []).length }} 处发现
              </span>
            </div>
            <div v-if="(state.linterReport?.empirical_tells || []).length > 0" class="flex flex-wrap gap-1 mt-1">
              <span v-for="tell in (state.linterReport?.empirical_tells || [])" :key="tell" class="text-[10px] bg-brand-amber/10 text-brand-amber border border-brand-amber/30 px-1.5 py-0.5 rounded font-mono">
                {{ tell }}
              </span>
            </div>
            <div v-else class="text-[10px] text-brand-emerald">文风自然，未发现刻意说教与机械连词。</div>
          </div>

          <!-- 快速润色工具 -->
          <div class="p-3 bg-atelier-950 rounded-lg border border-atelier-750 space-y-2">
            <div class="flex justify-between items-center text-xs">
              <span class="text-ink-200 font-bold flex items-center gap-1.5">
                <ShieldCheck class="w-3.5 h-3.5 text-brand-emerald" />
                <span>快速润色</span>
              </span>
              <button @click="openHarmonizeModal" class="text-[10px] text-brand-amber hover:underline cursor-pointer">
                设置
              </button>
            </div>
            <div class="grid grid-cols-3 gap-1.5">
              <button 
                @click="actions.runDeterministicSanitize" 
                :disabled="state.isSanitizing"
                class="py-1.5 px-1 bg-atelier-850 hover:bg-atelier-800 text-ink-200 border border-atelier-700 rounded text-[11px] transition flex items-center justify-center gap-1 cursor-pointer disabled:opacity-50"
                title="去掉冒号与冗余连词">
                <Loader2 v-if="state.isSanitizing" class="w-3 h-3 text-brand-amber animate-spin" />
                <Sparkles v-else class="w-3 h-3 text-brand-amber" />
                <span>{{ state.isSanitizing ? '去壳中...' : '去除壳子' }}</span>
              </button>
              <button 
                @click="openHarmonizeModal" 
                class="py-1.5 px-1 bg-atelier-850 hover:bg-atelier-800 text-ink-200 border border-atelier-700 rounded text-[11px] transition flex items-center justify-center gap-1 cursor-pointer"
                title="安全规避敏感词汇">
                <ShieldCheck class="w-3 h-3 text-brand-emerald" />
                <span>改敏感词</span>
              </button>
              <button 
                @click="openHumanTouchesModal" 
                class="py-1.5 px-1 bg-atelier-850 hover:bg-atelier-800 text-ink-200 border border-atelier-700 rounded text-[11px] transition flex items-center justify-center gap-1 cursor-pointer"
                title="增添生活气息与细节描摹">
                <HeartHandshake class="w-3 h-3 text-brand-amber" />
                <span>增加细节</span>
              </button>
            </div>
          </div>

          <!-- 体检报告 -->
          <div v-if="state.reviewResult" class="p-3 bg-atelier-950 rounded-lg border space-y-2" :class="state.reviewResult.verdict === 'ACCEPTED' ? 'border-brand-emerald/40' : 'border-brand-rose/40'">
            <div class="flex justify-between items-center text-xs">
              <span class="font-bold flex items-center gap-1" :class="state.reviewResult.verdict === 'ACCEPTED' ? 'text-brand-emerald' : 'text-brand-rose'">
                <span v-if="state.reviewResult.verdict === 'ACCEPTED'">✓ 体检合格</span>
                <span v-else>⚠️ 建议精修</span>
              </span>
              <span class="px-2 py-0.5 rounded text-xs font-mono font-bold" :class="state.reviewResult.score >= 80 ? 'bg-brand-emerald/15 text-brand-emerald border border-brand-emerald/30' : 'bg-brand-rose/15 text-brand-rose border border-brand-rose/30'">
                {{ state.reviewResult.score }} 分
              </span>
            </div>
            <div v-if="state.reviewResult.issues?.length" class="space-y-1">
              <div class="text-[10px] text-ink-400 font-semibold">修改建议 ({{ state.reviewResult.issues.length }}条)：</div>
              <ul class="text-[11px] text-brand-rose/90 space-y-0.5 list-disc list-inside bg-atelier-900 p-2 rounded">
                <li v-for="(iss, i) in state.reviewResult.issues" :key="i">{{ iss }}</li>
              </ul>
            </div>
            <div v-if="state.reviewResult.suggestions" class="text-[11px] text-ink-300 leading-relaxed bg-atelier-900 p-2 rounded">
              <strong class="text-ink-100">主审点评：</strong>{{ state.reviewResult.suggestions }}
            </div>

            <!-- 阶段流转按钮 -->
            <div class="pt-2 space-y-2">
              <button 
                v-if="state.reviewResult.verdict === 'ACCEPTED'"
                @click="goToStep(6)"
                class="w-full py-2.5 bg-brand-emerald hover:bg-emerald-500 text-atelier-950 font-bold text-xs rounded-lg shadow-atelier-sm transition flex items-center justify-center gap-1.5 cursor-pointer">
                <CheckCircle2 class="w-3.5 h-3.5" />
                <span>前往定稿 ➔</span>
              </button>
              <div v-else class="space-y-1.5">
                <button 
                  @click="goToStep(5)"
                  class="w-full py-2.5 bg-gradient-to-r from-brand-amber to-amber-600 hover:from-brand-amber-hover hover:to-amber-500 text-atelier-950 font-bold text-xs rounded-lg shadow-atelier-sm transition flex items-center justify-center gap-1.5 cursor-pointer">
                  <RefreshCw class="w-3.5 h-3.5" />
                  <span>前往精修 ➔</span>
                </button>
                <button 
                  @click="goToStep(6)"
                  class="w-full py-1.5 bg-atelier-850 hover:bg-atelier-800 text-ink-400 hover:text-ink-200 text-[11px] rounded border border-atelier-750 transition flex items-center justify-center cursor-pointer">
                  <span>直接定稿 ➔</span>
                </button>
              </div>
            </div>
          </div>
        </div>

        <!-- STEP 5: 精修 -->
        <div v-show="state.activeStep === 5" class="space-y-3.5">
          <div class="flex items-center justify-between">
            <span class="text-xs font-bold text-brand-amber flex items-center gap-1.5">
              <RefreshCw class="w-3.5 h-3.5" />
              <span>逐条精修</span>
            </span>
            <span class="text-[10px] font-mono px-1.5 py-0.5 rounded bg-atelier-800 text-brand-amber">
              第 {{ state.rewriteLoopCount }} / 3 轮
            </span>
          </div>

          <div v-if="state.reviewResult?.suggestions" class="p-3 bg-atelier-950 rounded-lg border border-atelier-750 text-xs space-y-1.5">
            <span class="font-bold text-ink-200">修改建议：</span>
            <p class="text-[11px] text-ink-300 leading-relaxed whitespace-pre-wrap">{{ state.reviewResult.suggestions }}</p>
          </div>

          <button 
            @click="handleRewriteDraft"
            :disabled="state.isRewriting"
            class="w-full py-2.5 bg-gradient-to-r from-brand-amber to-amber-600 hover:from-brand-amber-hover hover:to-amber-500 text-atelier-950 font-bold text-xs rounded-lg shadow-atelier-sm transition flex items-center justify-center gap-1.5 cursor-pointer disabled:opacity-50">
            <Loader2 v-if="state.isRewriting" class="w-3.5 h-3.5 animate-spin" />
            <RotateCcw v-else class="w-3.5 h-3.5" />
            <span>{{ state.isRewriting ? '正在精修正文...' : '自动精修' }}</span>
          </button>

          <div class="grid grid-cols-2 gap-2 pt-1">
            <button 
              @click="goToStep(4)"
              class="py-2 bg-atelier-850 hover:bg-atelier-800 text-ink-300 hover:text-ink-100 text-xs rounded-lg border border-atelier-750 transition flex items-center justify-center gap-1 cursor-pointer">
              <span>← 重新体检</span>
            </button>
            <button 
              @click="goToStep(6)"
              class="py-2 bg-atelier-850 hover:bg-atelier-800 text-ink-300 hover:text-ink-100 text-xs rounded-lg border border-atelier-750 transition flex items-center justify-center gap-1 cursor-pointer">
              <span>前往定稿 ➔</span>
            </button>
          </div>
        </div>

        <!-- STEP 6: 定稿 -->
        <div v-show="state.activeStep === 6" class="space-y-3.5">
          <div class="flex items-center justify-between">
            <span class="text-xs font-bold text-brand-amber flex items-center gap-1.5">
              <CheckCircle2 class="w-3.5 h-3.5" />
              <span>完成定稿</span>
            </span>
            <span class="text-[10px] font-mono text-ink-300">{{ state.workbench.content.length }} 字</span>
          </div>

          <p class="text-[11px] text-ink-400 leading-relaxed">
            保存至章节目录，记录剧情变化，开启下一章。
          </p>

          <button 
            @click="handleCommitChapter"
            :disabled="state.isCommitting || !state.workbench.content.trim()"
            class="w-full py-2.5 bg-brand-emerald hover:bg-emerald-500 text-atelier-950 font-bold text-xs rounded-lg shadow-atelier-sm transition flex items-center justify-center gap-1.5 cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed">
            <Loader2 v-if="state.isCommitting" class="w-4 h-4 animate-spin" />
            <Check v-else class="w-4 h-4" />
            <span>{{ state.isCommitting ? '正在保存...' : ('保存第 ' + computedState.currentWorkingChapterIndex.value + ' 章') }}</span>
          </button>
        </div>
      </div>
    </div>

    <!-- 中栏：文学手稿画布 (弹性自适应排版空间) -->
    <div class="flex-1 flex flex-col min-h-0 min-w-0 overflow-hidden bg-atelier-950">
      
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
          <span class="text-[11px] font-mono text-ink-400 shrink-0">
            <strong class="text-brand-amber font-semibold">{{ state.workbench.content.length }}</strong> 字
          </span>
        </div>

        <!-- 风格与字数预算快速选择 -->
        <div class="hidden xl:flex items-center gap-2 bg-atelier-850 px-2.5 py-1 rounded-md border border-atelier-750 text-xs">
          <span class="text-[10px] text-ink-400">风格:</span>
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

          <span class="text-[10px] text-ink-400">字数:</span>
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
          <span v-if="state.lastSavedAt" class="hidden 2xl:inline text-[10px] text-ink-400 font-mono">
            已存 {{ formatSaveTime(state.lastSavedAt) }}
          </span>

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

          <!-- 重新展开参考快捷按钮 (当右侧折叠时呈现) -->
          <button 
            v-if="!state.showHorizonPanel && !state.isZenMode"
            @click="state.showHorizonPanel = true"
            class="px-2 py-1 rounded bg-atelier-850 hover:bg-atelier-800 border border-atelier-750 text-xs text-brand-cyan flex items-center gap-1 transition cursor-pointer"
            title="展开右侧参考">
            <PanelRight class="w-3.5 h-3.5" />
            <span class="text-[11px] font-medium hidden sm:inline">参考</span>
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

      <!-- 划词 AI 结果差分对比横幅 -->
      <div v-if="showInlineDiff && inlineActionResult" class="p-3.5 bg-atelier-900/90 border-b border-brand-amber/30 space-y-2 shrink-0">
        <div class="flex items-center justify-between text-xs">
          <span class="font-bold text-brand-amber flex items-center gap-1.5">
            <Sparkles class="w-3.5 h-3.5" />
            <span>改写结果 ({{ inlineActionResult.action }})</span>
          </span>
          <div class="flex gap-2">
            <button @click="applyInlineResult" class="px-2.5 py-1 bg-brand-emerald hover:bg-emerald-500 text-atelier-950 font-bold rounded text-xs transition cursor-pointer">
              ✓ 采用
            </button>
            <button @click="discardInlineResult" class="px-2.5 py-1 bg-atelier-800 hover:bg-atelier-750 text-ink-300 rounded text-xs transition cursor-pointer">
              ✕ 放弃
            </button>
          </div>
        </div>
        <div class="p-3 bg-atelier-950 rounded-lg border border-atelier-750 text-xs text-ink-100 leading-relaxed font-serif max-h-36 overflow-y-auto whitespace-pre-wrap">
          {{ inlineActionResult.result }}
        </div>
      </div>

      <!-- 纯净排版正文文本框 (呼吸感居中稿纸) -->
      <div class="flex-1 min-h-0 flex overflow-y-auto p-4 md:p-8 justify-center">
        <div class="w-full max-w-4xl xl:max-w-5xl 2xl:max-w-6xl flex flex-col h-full bg-atelier-900/20 rounded-xl p-4 md:p-8 border border-atelier-800/40 shadow-inner transition-all relative">
          
          <!-- 异常报错通知横幅 -->
          <div 
            v-if="renderError"
            class="mb-4 p-3.5 rounded-xl bg-brand-rose/15 border border-brand-rose/40 text-xs flex items-start justify-between gap-3 shrink-0 animate-fade-in">
            <div class="space-y-1">
              <div class="font-bold text-brand-rose flex items-center gap-1.5">
                <AlertCircle class="w-4 h-4" />
                <span>起草异常</span>
              </div>
              <p class="text-[11px] text-ink-200 leading-relaxed font-sans select-text">{{ renderError }}</p>
            </div>
            <div class="flex items-center gap-2 shrink-0">
              <button 
                @click="handleRenderScene" 
                class="px-2.5 py-1 bg-brand-rose/20 hover:bg-brand-rose/30 text-brand-rose border border-brand-rose/40 rounded text-xs transition cursor-pointer font-bold">
                重试
              </button>
              <button 
                @click="renderError = null" 
                class="p-1 text-ink-400 hover:text-ink-200 cursor-pointer">
                <X class="w-3.5 h-3.5" />
              </button>
            </div>
          </div>

          <!-- 正文初稿渲染中 动态动效横幅 (Generating Overlay Banner) -->
          <div 
            v-if="state.isRenderingScene"
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
            v-if="state.isRewriting"
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
            :disabled="state.isRenderingScene || state.isRewriting"
            class="flex-1 w-full bg-transparent text-ink-100 text-base md:text-[17px] leading-[2.1] font-serif resize-none focus:outline-none placeholder-ink-500 selection:bg-brand-amber/30 selection:text-ink-50 tracking-wide prose-canvas"
            :class="{ 'opacity-60 cursor-wait': state.isRenderingScene || state.isRewriting }"
            :placeholder="state.isRenderingScene ? '正文起草中，预计需要 10 ~ 30 秒，请稍候……' : '在此起草正文，可直接写作或由 AI 起草。'"></textarea>
        </div>
      </div>
    </div>

    <!-- 右栏：创作参考 (宽 280px，支持一键折叠) -->
    <div 
      v-show="state.showHorizonPanel && !state.isZenMode" 
      class="w-72 border-l border-atelier-750 bg-atelier-900/30 flex flex-col min-h-0 shrink-0 overflow-y-auto p-3.5 space-y-3.5 transition-all duration-200 select-none">
      
      <div>
        <span class="text-[10px] font-mono uppercase tracking-wider text-ink-400 block mb-0.5">
          创作参考
        </span>
        <h4 class="text-xs font-bold text-ink-200 font-serif">主角状态与上章文风</h4>
      </div>

      <!-- 主角状态 -->
      <div class="p-3 bg-atelier-950/70 border border-atelier-750 rounded-lg space-y-1.5 text-xs">
        <span class="font-bold text-brand-emerald flex items-center gap-1.5">
          <Cpu class="w-3.5 h-3.5" />
          <span>主角现状</span>
        </span>
        <div class="text-[11px] text-ink-300">
          境界：<strong class="text-ink-100 font-semibold">{{ state.currentProject?.protagonist?.name_and_level || '凡人' }}</strong>
        </div>
        <div class="text-[11px] text-ink-300">
          随身物品：<strong class="text-ink-100">{{ state.currentProject?.protagonist?.inventory || '无' }}</strong>
        </div>
      </div>

      <!-- 上章结尾 -->
      <div v-if="state.chapters.length > 0" class="p-3 bg-atelier-950/70 border border-atelier-750 rounded-lg space-y-1.5 text-xs">
        <span class="font-bold text-brand-amber flex items-center gap-1.5">
          <Anchor class="w-3.5 h-3.5" />
          <span>上章结尾</span>
        </span>
        <p class="text-[11px] text-ink-300 font-serif leading-relaxed italic line-clamp-4">
          “{{ lastChapterTail }}”
        </p>
      </div>

      <!-- 涉及人物 -->
      <div class="p-3 bg-atelier-950/70 border border-atelier-750 rounded-lg space-y-2 text-xs">
        <div class="flex items-center justify-between">
          <span class="font-bold text-brand-cyan flex items-center gap-1.5">
            <BookOpen class="w-3.5 h-3.5" />
            <span>涉及人物</span>
          </span>
          <span class="text-[10px] font-mono text-ink-400">{{ state.codexEntries.length }} 词条</span>
        </div>
        <div v-if="state.codexEntries.length > 0" class="flex flex-wrap gap-1">
          <span 
            v-for="e in state.codexEntries.slice(0, 8)" 
            :key="e.id"
            class="text-[10px] px-2 py-0.5 rounded bg-atelier-850 border border-atelier-700/80 text-ink-200">
            {{ e.name }}
          </span>
        </div>
        <div v-else class="text-[11px] text-ink-500 italic">
          暂无人物，可在设定集中添加。
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue';
import { state, computedState, actions, notify, dialogs } from '../stores/appState';
import { api } from '../api/client';
import {
  Target,
  Sparkles,
  GitCommit,
  PenTool,
  ShieldAlert,
  RefreshCw,
  CheckCircle2,
  Check,
  ShieldCheck,
  Anchor,
  Loader2,
  Cpu,
  BookOpen,
  Wand2,
  Scissors,
  Maximize2,
  Minimize2,
  Eye,
  MessageSquare,
  RotateCcw,
  HeartHandshake,
  PanelLeft,
  PanelRight,
  Plus,
  Settings,
  Save,
  Trash2,
  AlertCircle,
  AlertTriangle,
  X
} from 'lucide-vue-next';

const workflowSteps = [
  { id: 1, shortLabel: '构思', desc: '确定核心冲突与关键事件' },
  { id: 2, shortLabel: '大纲', desc: '规划四个阶段的剧情安排' },
  { id: 3, shortLabel: '起草', desc: '设定字数风格并起草正文' },
  { id: 4, shortLabel: '体检', desc: '检查 AI 腔调与文字质量' },
  { id: 5, shortLabel: '精修', desc: '依照修改建议精准修缮' },
  { id: 6, shortLabel: '定稿', desc: '保存至目录并开启新章' },
];

const renderElapsedSec = ref(0);
const rewriteElapsedSec = ref(0);
const renderError = ref(null);
let renderTimer = null;
let rewriteTimer = null;

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

const inspirationPresets = [
  { title: '拍卖截胡', desc: '低买高坑，暗藏乾坤', conflict: '主角在黑市拍卖会发现被掩盖的远古残器，反派欲恶意抬价加害，主角顺水推舟设局反坑反派万两灵石。' },
  { title: '考核越级', desc: '以弱胜强，众目反转', conflict: '内门考核中反派暗中篡改抽签安排强敌，主角暗藏底牌在众目睽睽下以弱胜强，打破全场轻视。' },
  { title: '密室反杀', desc: '利用公理，反向栽赃', conflict: '宗门密室内遭死士围剿，主角利用对世界规则的独特理解触发禁制，反向猎杀并栽赃幕后黑手。' },
  { title: '伏笔收网', desc: '暗线引爆，逆转攻守', conflict: '前序埋伏的暗线被敌方误以为是破绽发起决战，主角瞬间翻开底牌彻底逆转攻守并回收悬念。' },
];

const inlineActionButtons = [
  { action: 'rewrite', label: '润色', desc: '重新打磨语句，提升文采', icon: Scissors },
  { action: 'expand', label: '扩写', desc: '丰富细节，补充场面描写', icon: Maximize2 },
  { action: 'shorten', label: '精简', desc: '删减赘字，使节奏更明快', icon: Minimize2 },
  { action: 'sensory', label: '生动', desc: '强化五感，增强现场画面感', icon: Eye },
  { action: 'dialogue', label: '改对话', desc: '使人物台词更符合性格', icon: MessageSquare },
];

const lastChapterTail = computed(() => {
  if (!state.chapters || !state.chapters.length) return '';
  const last = state.chapters[state.chapters.length - 1];
  if (!last || !last.content) return '';
  const runes = Array.from(last.content);
  return runes.slice(Math.max(0, runes.length - 120)).join('');
});

function tensionBadgeClass(tension) {
  if (tension >= 8) return 'bg-brand-rose/15 text-brand-rose border border-brand-rose/30';
  if (tension >= 5) return 'bg-brand-amber/15 text-brand-amber border border-brand-amber/30';
  return 'bg-brand-emerald/15 text-brand-emerald border border-brand-emerald/30';
}

function stepBadge(id) {
  if (id === 1) return Boolean(state.workbench?.coreConflict);
  if (id === 2) return (state.workbench?.beats || []).some(b => b.action);
  if (id === 3) return Boolean(state.workbench?.content);
  if (id === 4) return Boolean(state.reviewResult);
  if (id === 5) return state.reviewResult?.verdict === 'REVISION_NEEDED';
  return false;
}

function stepBadgeClass(id) {
  if (id === 4 && state.reviewResult) {
    return state.reviewResult.score >= 80 ? 'bg-brand-emerald' : 'bg-brand-rose';
  }
  if (id === 5 && state.reviewResult?.verdict === 'REVISION_NEEDED') {
    return 'bg-brand-rose animate-pulse';
  }
  return 'bg-brand-amber';
}

function goToStep(id) {
  state.activeStep = id;
}

function appendHookToConflict(h) {
  const note = `\n[伏笔回收目标: ${h.title} - ${h.details}]`;
  if (!state.workbench.coreConflict.includes(h.title)) {
    state.workbench.coreConflict += note;
  }
}

// 分段构思
async function handleDeriveBeats() {
  if (!state.currentProject || !state.workbench.coreConflict.trim()) return;
  state.isGeneratingBeats = true;
  const targetIndex = computedState.currentWorkingChapterIndex.value;
  notify(
    '开始构思分段',
    `正在梳理第 ${targetIndex} 章剧情与情节分段...`,
    'info',
    3000,
    { taskType: 'beats', chapterIndex: targetIndex, status: 'started' }
  );
  try {
    const res = await api.deriveBeats(state.currentProject.id, {
      chapter_index: targetIndex,
      core_conflict: state.workbench.coreConflict,
    });
    if (res.beats && res.beats.length) {
      state.workbench.beats = res.beats;
    }
    if (res.state_mutation) {
      state.workbench.stateMutation = res.state_mutation;
    }
    state.activeStep = 2;
    await actions.saveCheckpoint();
    notify(
      '大纲已生成',
      `第 ${targetIndex} 章已完成情节分段`,
      'success',
      3500,
      { taskType: 'beats', chapterIndex: targetIndex, status: 'completed' }
    );
  } catch (e) {
    console.error('handleDeriveBeats error:', e);
    notify('大纲生成失败', e.message || '请检查网络或配置', 'error', 5000, { taskType: 'beats', chapterIndex: targetIndex, status: 'failed' });
  } finally {
    state.isGeneratingBeats = false;
  }
}

// 文学起草
async function handleRenderScene() {
  if (!state.currentProject) return;
  state.isRenderingScene = true;
  renderError.value = null;
  renderElapsedSec.value = 0;
  if (renderTimer) clearInterval(renderTimer);
  renderTimer = setInterval(() => {
    renderElapsedSec.value++;
  }, 1000);

  const targetIndex = computedState.currentWorkingChapterIndex.value;
  notify(
    '正文起草已启动',
    `第 ${targetIndex} 章起草中，预计需要 10~30 秒`,
    'info',
    3500,
    { taskType: 'render', chapterIndex: targetIndex, status: 'started' }
  );

  try {
    const res = await api.renderScene(state.currentProject.id, {
      chapter_index: targetIndex,
      beats: state.workbench.beats,
      words_target: state.wordsTarget || 2000,
      narrative_style: state.narrativeStyle || 'hardboiled',
    });
    state.workbench.content = res.content || '';
    state.activeStep = 3;
    await actions.runLinter();
    await actions.saveCheckpoint();
    notify(
      '正文起草完成',
      `第 ${targetIndex} 章已完成约 ${state.workbench.content.length} 字 (耗时 ${renderElapsedSec.value}s)`,
      'success',
      4500,
      { taskType: 'render', chapterIndex: targetIndex, status: 'completed' }
    );
  } catch (e) {
    console.error('handleRenderScene error:', e);
    renderError.value = e.message || '正文初稿起草异常';
    notify(
      '正文起草失败',
      e.message || '请检查网络或配置',
      'error',
      6000,
      { taskType: 'render', chapterIndex: targetIndex, status: 'failed' }
    );
  } finally {
    state.isRenderingScene = false;
    if (renderTimer) {
      clearInterval(renderTimer);
      renderTimer = null;
    }
  }
}

async function confirmAndRenderScene() {
  if (state.workbench.content.trim()) {
    const ok = await dialogs.confirm({
      title: '重新起草正文',
      message: '重新生成将覆盖现有手稿，是否确认？',
      type: 'warning',
      confirmText: '确认重写',
    });
    if (!ok) return;
  }
  await handleRenderScene();
}

// 文风体检
async function handleReviewDraft() {
  if (!state.currentProject || !state.workbench.content.trim()) return;
  state.isReviewing = true;
  const targetIndex = computedState.currentWorkingChapterIndex.value;
  notify(
    '文风体检已启动',
    `正在检查第 ${targetIndex} 章手稿文字质量`,
    'info',
    3000,
    { taskType: 'review', chapterIndex: targetIndex, status: 'started' }
  );

  try {
    const res = await api.reviewDraft(state.currentProject.id, {
      chapter_index: targetIndex,
      content: state.workbench.content,
      beats: state.workbench.beats,
    });
    state.reviewResult = res;
    state.activeStep = res.verdict === 'ACCEPTED' ? 6 : 5;
    await actions.saveCheckpoint();
    if (res.verdict === 'ACCEPTED') {
      notify(
        '文风体检合格',
        `第 ${targetIndex} 章评定合格 (${res.score}分)，可直接定稿`,
        'success',
        4000,
        { taskType: 'review', chapterIndex: targetIndex, status: 'accepted' }
      );
    } else {
      notify(
        '建议继续精修',
        `第 ${targetIndex} 章得分 ${res.score}分，建议进行针对性修改`,
        'warning',
        4500,
        { taskType: 'review', chapterIndex: targetIndex, status: 'rejected' }
      );
    }
  } catch (e) {
    console.error('handleReviewDraft error:', e);
    notify('体检失败', e.message || '请检查网络或配置', 'error', 5000, { taskType: 'review', chapterIndex: targetIndex, status: 'failed' });
  } finally {
    state.isReviewing = false;
  }
}

// 自动精修
async function handleRewriteDraft() {
  if (!state.currentProject || !state.workbench.content.trim()) return;
  state.isRewriting = true;
  renderError.value = null;
  rewriteElapsedSec.value = 0;
  if (rewriteTimer) clearInterval(rewriteTimer);
  rewriteTimer = setInterval(() => {
    rewriteElapsedSec.value++;
  }, 1000);

  const targetIndex = computedState.currentWorkingChapterIndex.value;
  notify(
    '精修已启动',
    `正在依照修改意见优化第 ${targetIndex} 章手稿`,
    'info',
    3500,
    { taskType: 'rewrite', chapterIndex: targetIndex, status: 'started' }
  );

  try {
    const issues = state.reviewResult?.issues?.length 
      ? state.reviewResult.issues 
      : ['需根据去AI味与语法健全原则进行深度精修'];
    const suggestions = state.reviewResult?.suggestions 
      || '请重塑叙事节奏，补齐主谓宾完整结构，消除机械断句与模式化废词。';

    const res = await api.rewriteDraft(state.currentProject.id, {
      chapter_index: targetIndex,
      original_draft: state.workbench.content,
      content: state.workbench.content,
      issues: issues,
      suggestions: suggestions,
      words_target: state.wordsTarget || 2000,
      narrative_style: state.narrativeStyle || 'hardboiled',
      review: {
        verdict: 'REVISION_NEEDED',
        score: state.reviewResult?.score || 70,
        issues: issues,
        suggestions: suggestions,
      },
    });
    state.workbench.content = res.content || '';
    state.rewriteLoopCount++;
    await actions.runLinter();
    state.activeStep = 4;
    await actions.saveCheckpoint();
    notify(
      '精修完成',
      `已依照意见优化正文 (耗时 ${rewriteElapsedSec.value}s)`,
      'success',
      4500,
      { taskType: 'rewrite', chapterIndex: targetIndex, status: 'completed' }
    );
  } catch (e) {
    console.error('handleRewriteDraft error:', e);
    renderError.value = e.message || '正文精修接口异常';
    notify('精修失败', e.message || '请检查网络或配置', 'error', 6000, { taskType: 'rewrite', chapterIndex: targetIndex, status: 'failed' });
  } finally {
    state.isRewriting = false;
    if (rewriteTimer) {
      clearInterval(rewriteTimer);
      rewriteTimer = null;
    }
  }
}

// 保存定稿
async function handleCommitChapter() {
  if (!state.currentProject || !state.workbench.content.trim()) return;
  state.isCommitting = true;
  const targetIndex = computedState.currentWorkingChapterIndex.value;
  try {
    const titleMatch = state.workbench.coreConflict.match(/^[^\n，。！？]+/);
    const title = titleMatch ? titleMatch[0].trim() : `第 ${targetIndex} 章`;
    await api.commitChapter(state.currentProject.id, {
      chapter_index: targetIndex,
      title: title,
      content: state.workbench.content,
      beats: state.workbench.beats,
      state_mutation: state.workbench.stateMutation,
      audit_report: state.reviewResult,
      word_count: state.workbench.content.length,
    });
    try {
      await api.clearCheckpoint(state.currentProject.id, targetIndex);
    } catch (_) {}
    notify(
      '本章已定稿保存',
      `第 ${targetIndex} 章已保存至章节目录 (${state.workbench.content.length}字)`,
      'success',
      4500,
      { taskType: 'commit', chapterIndex: targetIndex, status: 'completed' }
    );
    state.editingChapterIndex = null;
    await actions.selectProject(state.currentProject.id);
    state.workbench.content = '';
    state.workbench.coreConflict = '';
    state.reviewResult = null;
    state.activeStep = 1;
  } catch (e) {
    console.error('handleCommitChapter error:', e);
    notify('保存失败', e.message || '保存章节发生异常', 'error', 5000, { taskType: 'commit', chapterIndex: targetIndex, status: 'failed' });
  } finally {
    state.isCommitting = false;
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
  state.workbench.content = '';
  state.workbench.coreConflict = '';
  state.workbench.beats = [
    { phase: '蓄力压迫', tension: 4, action: '', expectation_broken: '' },
    { phase: '试探下套', tension: 6, action: '', expectation_broken: '' },
    { phase: '绝地反转', tension: 9, action: '', expectation_broken: '' },
    { phase: '章末留钩', tension: 8, action: '', expectation_broken: '' },
  ];
  state.reviewResult = null;
  state.activeStep = 1;
  notify('已切换', '已重置画布为创作最新章节', 'info');
}

// 划词 Inline Actions
const inlineActionLoading = ref(false);
const inlineActionResult = ref(null);
const showInlineDiff = ref(false);
let lastSelectedText = '';

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

function formatSaveTime(ts) {
  if (!ts) return '';
  const d = new Date(ts);
  return `${d.getHours().toString().padStart(2, '0')}:${d.getMinutes().toString().padStart(2, '0')}:${d.getSeconds().toString().padStart(2, '0')}`;
}

function discardInlineResult() {
  showInlineDiff.value = false;
  inlineActionResult.value = null;
}

function openHarmonizeModal() {
  state.showHarmonizeModal = true;
}

function openHumanTouchesModal() {
  state.showHumanTouchesModal = true;
}

onMounted(() => {
  if (actions.restoreCheckpoint) {
    actions.restoreCheckpoint(computedState.currentWorkingChapterIndex.value);
  }
});

onUnmounted(() => {
  if (renderTimer) clearInterval(renderTimer);
  if (rewriteTimer) clearInterval(rewriteTimer);
  if (proseDebounceTimer) clearTimeout(proseDebounceTimer);
});
</script>
