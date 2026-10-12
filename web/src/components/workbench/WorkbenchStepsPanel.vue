<template>
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

        <!-- 小白首选：全自动一键成章主入口 -->
        <div class="mb-2.5">
          <button 
            @click="actions.runAutonomousPipeline()"
            :disabled="state.pipelineState.active"
            class="w-full p-2.5 rounded-xl border transition flex flex-col gap-1 cursor-pointer disabled:cursor-not-allowed group text-left relative overflow-hidden"
            :class="state.pipelineState.active 
              ? 'bg-brand-amber/15 border-brand-amber/50 shadow-amber-glow animate-subtle-pulse' 
              : 'bg-gradient-to-br from-atelier-800 via-atelier-850 to-atelier-800 hover:from-atelier-750 hover:to-atelier-800 border-brand-amber/35 hover:border-brand-amber shadow-atelier-sm'">
            
            <div class="flex items-center justify-between">
              <div class="flex items-center gap-1.5">
                <span class="w-5 h-5 rounded-md bg-brand-amber text-atelier-950 flex items-center justify-center font-bold text-xs shadow-sm">
                  ⚡
                </span>
                <span class="text-xs font-bold text-ink-50 font-serif tracking-wide group-hover:text-brand-amber transition-colors">
                  {{ state.pipelineState.active ? (state.pipelineState.message || '全自动成章中...') : '全自动一键成章' }}
                </span>
              </div>
              <span v-if="!state.pipelineState.active" class="text-[10px] px-1.5 py-0.5 rounded bg-brand-amber/15 text-brand-amber border border-brand-amber/30 font-medium">
                小白推荐
              </span>
              <Loader2 v-else class="w-3.5 h-3.5 text-brand-amber animate-spin" />
            </div>

            <p class="text-[10px] text-ink-400 group-hover:text-ink-300 leading-tight">
              {{ state.pipelineState.active ? '构思大纲 ➔ 正文起草 ➔ 质量体检 ➔ 自动精修 ➔ 入库' : '自动构思冲突、推演节拍、起草高能正文并自愈精修' }}
            </p>
          </button>
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
            <div class="flex items-center justify-between mb-1">
              <label class="block text-[11px] text-ink-300 font-medium">剧情走向：</label>
              <button 
                type="button"
                @click="actions.suggestChapterConflict()"
                :disabled="state.isSuggestingConflict"
                class="text-[10px] text-brand-amber hover:text-brand-amber-hover font-semibold flex items-center gap-1 transition cursor-pointer disabled:opacity-50">
                <Loader2 v-if="state.isSuggestingConflict" class="w-3 h-3 animate-spin" />
                <Sparkles v-else class="w-3 h-3" />
                <span>{{ state.isSuggestingConflict ? '正在构思...' : '🎲 智能推演剧情' }}</span>
              </button>
            </div>
            <textarea 
              v-model="state.workbench.coreConflict"
              rows="3" 
              :disabled="state.pipelineState.active || state.isSuggestingConflict"
              class="w-full bg-atelier-950 border border-atelier-750 rounded-lg p-2.5 text-xs text-ink-100 placeholder-ink-500 resize-none focus-ring font-sans leading-relaxed"
              :class="{ 'opacity-60 cursor-not-allowed': state.pipelineState.active || state.isSuggestingConflict }"
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
              <input 
                v-model="b.reader_emotion" 
                class="w-full bg-atelier-850 border border-atelier-700 rounded px-2 py-1 text-[11px] text-ink-300 focus-ring" 
                placeholder="期望读者情绪（紧张/解气/心疼…）">
              <input 
                v-model="b.info_gap" 
                class="w-full bg-atelier-850 border border-atelier-700 rounded px-2 py-1 text-[11px] text-ink-300 focus-ring" 
                placeholder="信息差（读者知角色不知 / 反之）">
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
              <span>单章偏好</span>
            </span>
            <span class="text-[10px] font-mono text-ink-400">单章微调 · 继承全书</span>
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

      <!-- 创作参考坞 (吸底卡片，支持一键抽屉式展开) -->
      <div class="shrink-0 border-t border-atelier-750 bg-atelier-950/80 backdrop-blur-sm flex flex-col transition-all duration-200">
        <!-- 胶囊标题栏 / 折叠开关 -->
        <div 
          @click="showReferenceDock = !showReferenceDock" 
          class="p-2.5 flex items-center justify-between text-xs text-ink-300 hover:text-ink-100 transition cursor-pointer select-none group">
          <div class="flex items-center gap-2 min-w-0">
            <span class="w-2 h-2 rounded-full bg-brand-amber shrink-0 animate-pulse"></span>
            <span class="font-serif font-bold text-ink-100 text-[11px] shrink-0">创作参考</span>
            <span class="text-[10px] text-ink-400 truncate font-mono">
              {{ computedState.protagonistCharacter.value ? (computedState.protagonistCharacter.value.name + (computedState.protagonistCharacter.value.realm ? ' (' + computedState.protagonistCharacter.value.realm + ')' : '')) : '凡人' }} · {{ lastChapterTail ? '上章末尾' : '开局首章' }}
            </span>
          </div>
          <div class="flex items-center gap-1 text-[10px] text-ink-400 group-hover:text-brand-amber shrink-0">
            <span>{{ showReferenceDock ? '收起' : '展开' }}</span>
            <ChevronUp v-if="!showReferenceDock" class="w-3.5 h-3.5" />
            <ChevronDown v-else class="w-3.5 h-3.5" />
          </div>
        </div>

        <!-- 展开内容区 (最大高度 240px，带平滑滚动) -->
        <div v-show="showReferenceDock" class="p-3 pt-0 max-h-60 overflow-y-auto space-y-2.5 border-t border-atelier-800/60">
          <!-- 主角状态 -->
          <div class="p-2.5 bg-atelier-900/80 border border-atelier-750/80 rounded-lg space-y-1 text-xs">
            <div class="flex items-center justify-between text-[11px]">
              <span class="font-bold text-brand-emerald flex items-center gap-1.5">
                <Cpu class="w-3.5 h-3.5" />
                <span>主角现状</span>
              </span>
              <strong class="text-ink-100 font-semibold truncate max-w-[140px]">{{ computedState.protagonistCharacter.value ? (computedState.protagonistCharacter.value.name + (computedState.protagonistCharacter.value.realm ? ' (' + computedState.protagonistCharacter.value.realm + ')' : '')) : '凡人' }}</strong>
            </div>
            <div class="text-[10px] text-ink-300">
              随身底牌：<span class="text-ink-200">{{ computedState.protagonistCharacter.value?.inventory || '无' }}</span>
            </div>
          </div>

          <!-- 上章结尾 -->
          <div v-if="state.chapters.length > 0" class="p-2.5 bg-atelier-900/80 border border-atelier-750/80 rounded-lg space-y-1 text-xs">
            <span class="font-bold text-brand-amber flex items-center gap-1.5 text-[11px]">
              <Anchor class="w-3.5 h-3.5" />
              <span>上章结尾</span>
            </span>
            <p class="text-[10px] text-ink-300 font-serif leading-relaxed italic line-clamp-3">
              “{{ lastChapterTail }}”
            </p>
          </div>

          <!-- 涉及人物 -->
          <div class="p-2.5 bg-atelier-900/80 border border-atelier-750/80 rounded-lg space-y-1.5 text-xs">
            <div class="flex items-center justify-between text-[11px]">
              <span class="font-bold text-brand-cyan flex items-center gap-1.5">
                <BookOpen class="w-3.5 h-3.5" />
                <span>登场人物</span>
              </span>
              <span class="text-[10px] font-mono text-ink-500">{{ state.codexEntries.length }} 词条</span>
            </div>
            <div v-if="state.codexEntries.length > 0" class="flex flex-wrap gap-1">
              <span 
                v-for="e in state.codexEntries.slice(0, 8)" 
                :key="e.id"
                class="text-[9px] px-1.5 py-0.5 rounded bg-atelier-850 border border-atelier-700/80 text-ink-200">
                {{ e.name }}
              </span>
            </div>
            <div v-else class="text-[10px] text-ink-500 italic">
              暂无人物设定
            </div>
          </div>
        </div>
      </div>
    </div>
</template>

<script setup>
import { ref, computed } from 'vue';
import { state, computedState, actions, notify, dialogs } from '../../stores/appState';
import { createDefaultWorkbench } from '../../stores/domain/projectScope';
import { api } from '../../api/client';
import {
  Sparkles,
  Target,
  GitCommit,
  Cpu,
  PenTool,
  Loader2,
  ShieldAlert,
  RotateCcw,
  RefreshCw,
  CheckCircle2,
  Check,
  ShieldCheck,
  Anchor,
  BookOpen,
  HeartHandshake,
  ChevronUp,
  ChevronDown
} from 'lucide-vue-next';

const props = defineProps({
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

const emit = defineEmits(['set-render-error']);

const handleRenderScene = () => props.onRenderScene?.();
const handleRewriteDraft = () => props.onRewriteDraft?.();
const openHumanTouchesModal = () => props.onHumanTouches?.();

const showReferenceDock = ref(false);

const workflowSteps = [
  { id: 1, shortLabel: '构思', desc: '确定核心冲突与关键事件' },
  { id: 2, shortLabel: '大纲', desc: '规划四个阶段的剧情安排' },
  { id: 3, shortLabel: '起草', desc: '设定字数风格并起草正文' },
  { id: 4, shortLabel: '体检', desc: '检查 AI 腔调与文字质量' },
  { id: 5, shortLabel: '精修', desc: '依照修改建议精准修缮' },
  { id: 6, shortLabel: '定稿', desc: '保存至目录并开启新章' },
];

const inspirationPresets = [
  { title: '拍卖截胡', desc: '低买高坑，暗藏乾坤', conflict: '主角在黑市拍卖会发现被掩盖的远古残器，反派欲恶意抬价加害，主角顺水推舟设局反坑反派万两灵石。' },
  { title: '考核越级', desc: '以弱胜强，众目反转', conflict: '内门考核中反派暗中篡改抽签安排强敌，主角暗藏底牌在众目睽睽下以弱胜强，打破全场轻视。' },
  { title: '密室反杀', desc: '利用公理，反向栽赃', conflict: '宗门密室内遭死士围剿，主角利用对世界规则的独特理解触发禁制，反向猎杀并栽赃幕后黑手。' },
  { title: '伏笔收网', desc: '暗线引爆，逆转攻守', conflict: '前序埋伏的暗线被敌方误以为是破绽发起决战，主角瞬间翻开底牌彻底逆转攻守并回收悬念。' },
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
  const note = `
[伏笔回收目标: ${h.title} - ${h.details}]`;
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
  await props.onRenderScene?.();
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
      draft_text: state.workbench.content,
      beats: state.workbench.beats,
      reviewer_model: state.config?.reviewer_model || state.config?.reviewer_provider?.model || '',
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
    emit('set-render-error', e.message || '文风体检异常');
    notify('体检失败', e.message || '请检查网络或配置', 'error', 6000, { taskType: 'review', chapterIndex: targetIndex, status: 'failed' });
  } finally {
    state.isReviewing = false;
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
    state.pipelineState.halted = false;
    state.pipelineState.haltInfo = null;
    await actions.selectProject(state.currentProject.id);
    state.workbench = createDefaultWorkbench();
    state.reviewResult = null;
    state.activeStep = 1;
  } catch (e) {
    console.error('handleCommitChapter error:', e);
    notify('保存失败', e.message || '保存章节发生异常', 'error', 5000, { taskType: 'commit', chapterIndex: targetIndex, status: 'failed' });
  } finally {
    state.isCommitting = false;
  }
}

function openHarmonizeModal() {
  state.showHarmonizeModal = true;
}
</script>
