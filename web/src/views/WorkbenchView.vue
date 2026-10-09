<template>
  <!-- 当尚未选择任何项目时的极简美学引导页 (Zero State) -->
  <div v-if="!state.currentProject" class="flex-1 flex flex-col items-center justify-center p-8 bg-atelier-950 text-center relative overflow-hidden select-none">
    <div class="max-w-md w-full p-8 rounded-2xl bg-atelier-900/60 border border-atelier-750 backdrop-blur-xl shadow-atelier-lg space-y-6">
      <div class="w-16 h-16 rounded-2xl bg-gradient-to-br from-brand-amber/20 to-amber-600/10 border border-brand-amber/30 mx-auto flex items-center justify-center text-brand-amber text-2xl font-serif font-bold shadow-amber-glow">
        墨
      </div>
      <div class="space-y-2">
        <h3 class="text-xl font-bold font-serif text-ink-50 tracking-wide">
          故事工厂 · 创作起点
        </h3>
        <p class="text-xs text-ink-300 leading-relaxed">
          基于状态机约束、实体因果账本与三权分立审校机制，为工业级长篇连载保驾护航。
        </p>
      </div>
      <div class="pt-2 flex flex-col gap-2.5">
        <button 
          @click="state.showNewProjectModal = true"
          class="w-full py-2.5 px-4 bg-gradient-to-r from-brand-amber to-amber-600 hover:from-brand-amber-hover hover:to-amber-500 text-atelier-950 font-bold text-xs rounded-lg transition shadow-atelier-md flex items-center justify-center gap-2 cursor-pointer">
          <Plus class="w-4 h-4" />
          <span>创建第一部小说作品</span>
        </button>
        <button 
          @click="state.activeTab = 'config'"
          class="w-full py-2 px-4 bg-atelier-850 hover:bg-atelier-800 text-ink-200 border border-atelier-700 rounded-lg text-xs transition flex items-center justify-center gap-1.5 cursor-pointer">
          <Settings class="w-3.5 h-3.5 text-ink-400" />
          <span>配置推理与写作模型 API</span>
        </button>
      </div>
    </div>
  </div>

  <!-- 主创作手稿工作台 (3 栏弹性自适应架构) -->
  <div v-else class="flex-1 flex overflow-hidden relative">
    <!-- 禅模式悬浮退出胶囊条 -->
    <div 
      v-if="state.isZenMode" 
      class="absolute top-4 right-6 z-30 flex items-center gap-2">
      <button 
        @click="state.isZenMode = false"
        class="flex items-center gap-1.5 px-3 py-1.5 rounded-full bg-atelier-900/80 hover:bg-atelier-800 text-ink-300 hover:text-ink-50 border border-atelier-700/80 backdrop-blur-md text-xs transition shadow-atelier-md cursor-pointer"
        title="退出专注模式 (快捷键 ESC)">
        <Minimize2 class="w-3.5 h-3.5 text-brand-amber" />
        <span>退出专注 (ESC)</span>
      </button>
    </div>

    <!-- 左栏：六步工序决策与节拍推演 (宽 330px，支持一键折叠) -->
    <div 
      v-show="state.showWorkflowPanel && !state.isZenMode" 
      class="w-80 border-r border-atelier-750 bg-atelier-900/40 flex flex-col shrink-0 overflow-hidden transition-all duration-200 select-none">
      
      <!-- 工步导航指示条 -->
      <div class="p-3 border-b border-atelier-750 bg-atelier-900/70">
        <div class="flex items-center justify-between mb-2">
          <div class="flex items-center gap-1.5">
            <Sparkles class="w-3.5 h-3.5 text-brand-amber" />
            <span class="text-xs font-bold text-ink-100 font-serif">生产流水线</span>
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

      <!-- 工步面板内容区 (纵向滚动) -->
      <div class="flex-1 p-3.5 overflow-y-auto space-y-4">
        <!-- STEP 1: 核心冲突与动机 -->
        <div v-show="state.activeStep === 1" class="space-y-3.5">
          <div>
            <div class="flex items-center justify-between">
              <span class="text-xs font-bold text-brand-amber flex items-center gap-1.5">
                <Target class="w-3.5 h-3.5" />
                <span>Step 1: 核心冲突与动机</span>
              </span>
              <span class="text-[10px] font-mono text-ink-400">第 {{ computedState.currentWorkingChapterIndex.value }} 章</span>
            </div>
            <p class="text-[11px] text-ink-400 mt-1">确立本章的核心戏剧钩子、物理阻碍与高光反转目标。</p>
          </div>

          <!-- 灵感预设标签胶囊 -->
          <div>
            <span class="text-[10px] uppercase font-mono tracking-wider text-ink-400 block mb-1.5">高能戏剧母题：</span>
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

          <!-- 伏笔回收药丸 -->
          <div v-if="(computedState.activeHooksList.value || []).length > 0">
            <span class="text-[10px] uppercase font-mono tracking-wider text-ink-400 block mb-1.5">待回收因果伏笔：</span>
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
            <label class="block text-[11px] text-ink-300 mb-1 font-medium">核心剧情冲突描述：</label>
            <textarea 
              v-model="state.workbench.coreConflict"
              rows="4" 
              class="w-full bg-atelier-950 border border-atelier-750 rounded-lg p-2.5 text-xs text-ink-100 placeholder-ink-500 resize-none focus-ring font-sans leading-relaxed"
              placeholder="例如：主角在拍卖会上洞悉残破丹炉藏有上古神念，引诱宿敌恶意抬价反遭反噬..."></textarea>
          </div>

          <button 
            @click="handleDeriveBeats"
            :disabled="state.isGeneratingBeats || !state.workbench.coreConflict.trim()"
            class="w-full py-2.5 bg-gradient-to-r from-brand-amber to-amber-600 hover:from-brand-amber-hover hover:to-amber-500 text-atelier-950 font-bold text-xs rounded-lg shadow-atelier-sm transition flex items-center justify-center gap-1.5 cursor-pointer disabled:opacity-50">
            <Loader2 v-if="state.isGeneratingBeats" class="w-3.5 h-3.5 animate-spin" />
            <Sparkles v-else class="w-3.5 h-3.5" />
            <span>{{ state.isGeneratingBeats ? '大模型正在推演节拍...' : '推演四段论节拍 (Derive Beats)' }}</span>
          </button>

          <button 
            v-if="state.workbench.beats.some(b => b.action)"
            @click="goToStep(2)"
            class="w-full py-2 bg-atelier-850 hover:bg-atelier-800 text-ink-300 hover:text-ink-100 text-xs rounded-lg border border-atelier-750 transition flex items-center justify-center gap-1 cursor-pointer">
            <span>已有节拍矩阵，前往查看/编辑 ➔</span>
          </button>
        </div>

        <!-- STEP 2: 因果节拍矩阵 -->
        <div v-show="state.activeStep === 2" class="space-y-3.5">
          <div class="flex items-center justify-between">
            <span class="text-xs font-bold text-brand-amber flex items-center gap-1.5">
              <GitCommit class="w-3.5 h-3.5" />
              <span>Step 2: 因果节拍矩阵</span>
            </span>
            <button @click="handleDeriveBeats" class="text-[11px] text-ink-400 hover:text-brand-amber transition cursor-pointer">
              重新推演
            </button>
          </div>

          <div class="space-y-2">
            <div 
              v-for="(b, i) in state.workbench.beats" 
              :key="i"
              class="p-2.5 bg-atelier-950/80 border border-atelier-750 rounded-lg space-y-1.5">
              <div class="flex items-center justify-between">
                <span class="text-[11px] font-bold text-ink-100">{{ b.phase || `节拍 ${i + 1}` }}</span>
                <span class="text-[10px] font-mono px-1.5 py-0.5 rounded font-bold" :class="tensionBadgeClass(b.tension)">
                  张力 {{ b.tension }}/10
                </span>
              </div>
              <input 
                v-model="b.action" 
                class="w-full bg-atelier-850 border border-atelier-700 rounded px-2 py-1 text-xs text-ink-100 focus-ring" 
                placeholder="客观物理动作与剧情事实...">
              <input 
                v-model="b.expectation_broken" 
                class="w-full bg-atelier-850 border border-atelier-700 rounded px-2 py-1 text-[11px] text-ink-300 focus-ring" 
                placeholder="打破谁的预期 / 读者心流反应...">
            </div>
          </div>

          <!-- 状态机变动预览 -->
          <div class="p-2.5 bg-atelier-950 rounded-lg border border-atelier-750 text-[11px] space-y-1">
            <span class="font-bold text-brand-emerald flex items-center gap-1">
              <ShieldCheck class="w-3 h-3" />
              <span>状态机预判结算：</span>
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
            <span>{{ state.isRenderingScene ? '作家模型文学渲染中...' : '渲染正文初稿 (Render Prose)' }}</span>
          </button>

          <button 
            v-if="state.workbench.content.trim()"
            @click="goToStep(3)"
            class="w-full py-2 bg-atelier-850 hover:bg-atelier-800 text-ink-300 hover:text-ink-100 text-xs rounded-lg border border-atelier-750 transition flex items-center justify-center gap-1 cursor-pointer">
            <span>正文初稿已生成，前往工序配置 ➔</span>
          </button>
        </div>

        <!-- STEP 3: 文学渲染与目标 -->
        <div v-show="state.activeStep === 3" class="space-y-3.5">
          <div class="flex items-center justify-between">
            <span class="text-xs font-bold text-brand-amber flex items-center gap-1.5">
              <PenTool class="w-3.5 h-3.5" />
              <span>Step 3: 正文生成配置</span>
            </span>
            <span class="text-[10px] font-mono text-ink-400">{{ state.workbench.content.length }} 字已渲染</span>
          </div>

          <!-- 目标字数设定 -->
          <div class="p-3 bg-atelier-950 rounded-lg border border-atelier-750 space-y-2">
            <label class="block text-[11px] text-ink-300 font-medium">目标章节字数期望：</label>
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

          <!-- 叙事口吻腔调设定 -->
          <div class="p-3 bg-atelier-950 rounded-lg border border-atelier-750 space-y-2">
            <label class="block text-[11px] text-ink-300 font-medium">叙事口吻与声口风格：</label>
            <div class="space-y-1.5">
              <button 
                v-for="tone in [
                  { id: 'hardboiled', name: '冷峻白描', desc: '硬派克制 · 物理物态' },
                  { id: 'high_tension', name: '热血张力', desc: '暴风骤雨 · 极强冲突' },
                  { id: 'classical', name: '古典志怪', desc: '青灯夜话 · 诡谲苍凉' },
                  { id: 'vernacular', name: '市井烟火', desc: '粗粝鲜活 · 地气充盈' },
                  { id: 'cinematic', name: '电影全景', desc: '蒙太奇景深 · 恢弘画卷' }
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
            class="w-full py-2.5 bg-atelier-850 hover:bg-atelier-800 text-ink-100 font-medium text-xs rounded-lg border border-atelier-700 transition flex items-center justify-center gap-1.5 cursor-pointer disabled:opacity-50">
            <RotateCcw class="w-3.5 h-3.5 text-brand-amber" />
            <span>{{ state.isRenderingScene ? '重新渲染中...' : '重新生成正文初稿' }}</span>
          </button>

          <button 
            @click="goToStep(4)"
            :disabled="!state.workbench.content.trim()"
            class="w-full py-2.5 bg-gradient-to-r from-brand-amber to-amber-600 hover:from-brand-amber-hover hover:to-amber-500 text-atelier-950 font-bold text-xs rounded-lg shadow-atelier-sm transition flex items-center justify-center gap-1.5 cursor-pointer disabled:opacity-50">
            <span>初稿就绪，前往质检门禁 ➔</span>
          </button>
        </div>

        <!-- STEP 4: 质检门禁与主编终审 -->
        <div v-show="state.activeStep === 4" class="space-y-3.5">
          <div class="flex items-center justify-between">
            <span class="text-xs font-bold text-brand-amber flex items-center gap-1.5">
              <ShieldAlert class="w-3.5 h-3.5" />
              <span>Step 4: 质检门禁终审</span>
            </span>
            <button @click="handleReviewDraft" :disabled="state.isReviewing" class="text-[11px] text-brand-amber hover:underline cursor-pointer">
              {{ state.isReviewing ? '终审中...' : '发起终审' }}
            </button>
          </div>

          <!-- 突发度指标卡 -->
          <div class="p-3 bg-atelier-950 rounded-lg border border-atelier-750 space-y-1.5">
            <div class="flex justify-between items-center text-xs">
              <span class="text-ink-300">句长突发度 (Burstiness)</span>
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
            <p class="text-[10px] text-ink-400">长短句剧烈交替可瓦解 AI 机械感 (≥ 45 达标)。</p>
          </div>

          <!-- 模式化套词命中小结 -->
          <div class="p-3 bg-atelier-950 rounded-lg border border-atelier-750 space-y-1">
            <div class="flex justify-between items-center text-xs">
              <span class="text-ink-300">模式化套词：</span>
              <span class="font-mono text-xs font-bold" :class="(state.linterReport?.hit_banned_words || []).length === 0 ? 'text-brand-emerald' : 'text-brand-rose'">
                {{ (state.linterReport?.hit_banned_words || []).length }} 处命中
              </span>
            </div>
            <div v-if="(state.linterReport?.hit_banned_words || []).length > 0" class="flex flex-wrap gap-1 mt-1">
              <span v-for="w in (state.linterReport?.hit_banned_words || [])" :key="w" class="text-[10px] bg-brand-rose/10 text-brand-rose border border-brand-rose/25 px-1.5 py-0.5 rounded font-mono">
                {{ w }}
              </span>
            </div>
            <div v-else class="text-[10px] text-brand-emerald">未检出高频模式化废词。</div>
          </div>

          <!-- 国内合规与扰动 -->
          <div class="p-3 bg-atelier-950 rounded-lg border border-atelier-750 space-y-2">
            <div class="flex justify-between items-center text-xs">
              <span class="text-ink-200 font-bold flex items-center gap-1.5">
                <ShieldCheck class="w-3.5 h-3.5 text-brand-emerald" />
                <span>国内合规与对抗扰动</span>
              </span>
              <button @click="openHarmonizeModal" class="text-[10px] text-brand-amber hover:underline cursor-pointer">
                参数设置
              </button>
            </div>
            <div class="flex gap-1.5">
              <button 
                @click="openHarmonizeModal" 
                class="flex-1 py-1.5 px-2 bg-atelier-850 hover:bg-atelier-800 text-ink-200 border border-atelier-700 rounded text-[11px] transition flex items-center justify-center gap-1 cursor-pointer">
                <ShieldCheck class="w-3 h-3 text-brand-emerald" />
                <span>合规扰动</span>
              </button>
              <button 
                @click="openHumanTouchesModal" 
                class="flex-1 py-1.5 px-2 bg-atelier-850 hover:bg-atelier-800 text-ink-200 border border-atelier-700 rounded text-[11px] transition flex items-center justify-center gap-1 cursor-pointer">
                <HeartHandshake class="w-3 h-3 text-brand-amber" />
                <span>人味注入</span>
              </button>
            </div>
          </div>

          <!-- 主编终审报告 -->
          <div v-if="state.reviewResult" class="p-3 bg-atelier-950 rounded-lg border space-y-2" :class="state.reviewResult.verdict === 'ACCEPTED' ? 'border-brand-emerald/40' : 'border-brand-rose/40'">
            <div class="flex justify-between items-center text-xs">
              <span class="font-bold flex items-center gap-1" :class="state.reviewResult.verdict === 'ACCEPTED' ? 'text-brand-emerald' : 'text-brand-rose'">
                <span v-if="state.reviewResult.verdict === 'ACCEPTED'">✓ 终审裁决: 验收通过</span>
                <span v-else>⚠️ 终审裁决: 驳回返工</span>
              </span>
              <span class="px-2 py-0.5 rounded text-xs font-mono font-bold" :class="state.reviewResult.score >= 80 ? 'bg-brand-emerald/15 text-brand-emerald border border-brand-emerald/30' : 'bg-brand-rose/15 text-brand-rose border border-brand-rose/30'">
                {{ state.reviewResult.score }} 分
              </span>
            </div>
            <div v-if="state.reviewResult.issues?.length" class="space-y-1">
              <div class="text-[10px] text-ink-400 font-semibold">检出问题点 ({{ state.reviewResult.issues.length }})：</div>
              <ul class="text-[11px] text-brand-rose/90 space-y-0.5 list-disc list-inside bg-atelier-900 p-2 rounded">
                <li v-for="(iss, i) in state.reviewResult.issues" :key="i">{{ iss }}</li>
              </ul>
            </div>
            <div v-if="state.reviewResult.suggestions" class="text-[11px] text-ink-300 leading-relaxed bg-atelier-900 p-2 rounded">
              <strong class="text-ink-100">主编建议：</strong>{{ state.reviewResult.suggestions }}
            </div>

            <!-- 终审后流转按钮 -->
            <div class="pt-2 space-y-2">
              <button 
                v-if="state.reviewResult.verdict === 'ACCEPTED'"
                @click="goToStep(6)"
                class="w-full py-2.5 bg-brand-emerald hover:bg-emerald-500 text-atelier-950 font-bold text-xs rounded-lg shadow-atelier-sm transition flex items-center justify-center gap-1.5 cursor-pointer">
                <CheckCircle2 class="w-3.5 h-3.5" />
                <span>质检合格，前往封存归档 (Step 6) ➔</span>
              </button>
              <div v-else class="space-y-1.5">
                <button 
                  @click="goToStep(5)"
                  class="w-full py-2.5 bg-gradient-to-r from-brand-amber to-amber-600 hover:from-brand-amber-hover hover:to-amber-500 text-atelier-950 font-bold text-xs rounded-lg shadow-atelier-sm transition flex items-center justify-center gap-1.5 cursor-pointer">
                  <RefreshCw class="w-3.5 h-3.5" />
                  <span>存在瑕疵，前往定向返工 (Step 5) ➔</span>
                </button>
                <button 
                  @click="goToStep(6)"
                  class="w-full py-1.5 bg-atelier-850 hover:bg-atelier-800 text-ink-400 hover:text-ink-200 text-[11px] rounded border border-atelier-750 transition flex items-center justify-center cursor-pointer">
                  <span>忽略警告，强制前往归档 ➔</span>
                </button>
              </div>
            </div>
          </div>
        </div>

        <!-- STEP 5: 定向返工修缮 -->
        <div v-show="state.activeStep === 5" class="space-y-3.5">
          <div class="flex items-center justify-between">
            <span class="text-xs font-bold text-brand-amber flex items-center gap-1.5">
              <RefreshCw class="w-3.5 h-3.5" />
              <span>Step 5: 缺陷定向返工</span>
            </span>
            <span class="text-[10px] font-mono px-1.5 py-0.5 rounded bg-atelier-800 text-brand-amber">
              第 {{ state.rewriteLoopCount }} / 3 轮
            </span>
          </div>

          <div v-if="state.reviewResult?.suggestions" class="p-3 bg-atelier-950 rounded-lg border border-atelier-750 text-xs space-y-1.5">
            <span class="font-bold text-ink-200">主编修改建议：</span>
            <p class="text-[11px] text-ink-300 leading-relaxed whitespace-pre-wrap">{{ state.reviewResult.suggestions }}</p>
          </div>

          <button 
            @click="handleRewriteDraft"
            :disabled="state.isRewriting"
            class="w-full py-2.5 bg-gradient-to-r from-brand-amber to-amber-600 hover:from-brand-amber-hover hover:to-amber-500 text-atelier-950 font-bold text-xs rounded-lg shadow-atelier-sm transition flex items-center justify-center gap-1.5 cursor-pointer disabled:opacity-50">
            <Loader2 v-if="state.isRewriting" class="w-3.5 h-3.5 animate-spin" />
            <RotateCcw v-else class="w-3.5 h-3.5" />
            <span>{{ state.isRewriting ? '执行局部差分返工中...' : '执行针对性精修返工' }}</span>
          </button>

          <div class="grid grid-cols-2 gap-2 pt-1">
            <button 
              @click="goToStep(4)"
              class="py-2 bg-atelier-850 hover:bg-atelier-800 text-ink-300 hover:text-ink-100 text-xs rounded-lg border border-atelier-750 transition flex items-center justify-center gap-1 cursor-pointer">
              <span>← 返回重新终审</span>
            </button>
            <button 
              @click="goToStep(6)"
              class="py-2 bg-atelier-850 hover:bg-atelier-800 text-ink-300 hover:text-ink-100 text-xs rounded-lg border border-atelier-750 transition flex items-center justify-center gap-1 cursor-pointer">
              <span>前往封存归档 →</span>
            </button>
          </div>
        </div>

        <!-- STEP 6: 结算归档 -->
        <div v-show="state.activeStep === 6" class="space-y-3.5">
          <div class="flex items-center justify-between">
            <span class="text-xs font-bold text-brand-amber flex items-center gap-1.5">
              <CheckCircle2 class="w-3.5 h-3.5" />
              <span>Step 6: 原子归档结算</span>
            </span>
            <span class="text-[10px] font-mono text-ink-300">{{ state.workbench.content.length }} 字</span>
          </div>

          <p class="text-[11px] text-ink-400 leading-relaxed">
            将本章正文、状态机转移与伏笔回收一次性原子提交至 SQLite 数据库，成为封存正史。
          </p>

          <button 
            @click="handleCommitChapter"
            :disabled="!state.workbench.content.trim()"
            class="w-full py-2.5 bg-brand-emerald hover:bg-emerald-500 text-atelier-950 font-bold text-xs rounded-lg shadow-atelier-sm transition flex items-center justify-center gap-1.5 cursor-pointer disabled:opacity-50">
            <Check class="w-4 h-4" />
            <span>提交并原子封存第 {{ computedState.currentWorkingChapterIndex.value }} 章</span>
          </button>
        </div>
      </div>
    </div>

    <!-- 中栏：文学手稿画布 (弹性自适应排版空间) -->
    <div class="flex-1 flex flex-col min-w-0 overflow-hidden bg-atelier-950">
      
      <!-- 撤回草稿 / 历史章节精修提示条 -->
      <div 
        v-if="state.editingChapterIndex" 
        class="px-5 py-2 bg-brand-amber/10 border-b border-brand-amber/25 flex items-center justify-between text-xs text-brand-amber select-none shrink-0">
        <div class="flex items-center gap-2">
          <RotateCcw class="w-3.5 h-3.5 shrink-0" />
          <span>正在精修 <strong>第 {{ state.editingChapterIndex }} 章</strong> 草稿（提交封存将覆盖该章节）</span>
        </div>
        <button 
          @click="resetToNewChapter" 
          class="px-2 py-0.5 rounded bg-brand-amber/20 hover:bg-brand-amber/30 text-[11px] font-medium transition cursor-pointer">
          放弃精修，创作新章节
        </button>
      </div>

      <!-- 手稿顶栏状态与字数计量 -->
      <div class="h-11 px-5 border-b border-atelier-750 flex items-center justify-between bg-atelier-900/30 shrink-0 select-none">
        <div class="flex items-center gap-3 min-w-0">
          <!-- 重新展开工序面板快捷按钮 (当左侧折叠时呈现) -->
          <button 
            v-if="!state.showWorkflowPanel && !state.isZenMode"
            @click="state.showWorkflowPanel = true"
            class="px-2 py-1 rounded bg-atelier-850 hover:bg-atelier-800 border border-atelier-750 text-xs text-brand-amber flex items-center gap-1 transition cursor-pointer"
            title="展开左侧工序流水线">
            <PanelLeft class="w-3.5 h-3.5" />
            <span class="text-[11px] font-medium">展开工步</span>
          </button>

          <span class="text-xs font-bold font-serif text-ink-100 truncate">
            第 {{ computedState.currentWorkingChapterIndex.value }} 章手稿
          </span>
          <span class="text-[11px] font-mono text-ink-400 shrink-0">
            <strong class="text-brand-amber font-semibold">{{ state.workbench.content.length }}</strong> 字 · 约 {{ Math.max(1, Math.ceil(state.workbench.content.length / 400)) }} 分钟读完
          </span>
        </div>

        <!-- 口吻与字数预算快捷切换器 -->
        <div class="hidden lg:flex items-center gap-2 bg-atelier-850 px-2.5 py-1 rounded-md border border-atelier-750 text-xs">
          <span class="text-[10px] font-mono text-ink-400">口吻:</span>
          <select 
            v-model="state.narrativeStyle" 
            class="bg-transparent text-[11px] text-brand-amber font-medium focus:outline-none cursor-pointer">
            <option value="hardboiled" class="bg-atelier-900 text-ink-100">冷峻白描 (硬派克制)</option>
            <option value="high_tension" class="bg-atelier-900 text-ink-100">热血张力 (高压对抗)</option>
            <option value="classical" class="bg-atelier-900 text-ink-100">古典志怪 (诡谲苍凉)</option>
            <option value="vernacular" class="bg-atelier-900 text-ink-100">市井烟火 (粗粝鲜活)</option>
            <option value="cinematic" class="bg-atelier-900 text-ink-100">电影全景 (景深画卷)</option>
          </select>

          <span class="text-atelier-700">|</span>

          <span class="text-[10px] font-mono text-ink-400">预算:</span>
          <select 
            v-model="state.wordsTarget" 
            class="bg-transparent text-[11px] text-brand-amber font-medium focus:outline-none cursor-pointer">
            <option :value="1500" class="bg-atelier-900 text-ink-100">1500 字</option>
            <option :value="2000" class="bg-atelier-900 text-ink-100">2000 字 (标准)</option>
            <option :value="2500" class="bg-atelier-900 text-ink-100">2500 字 (丰满)</option>
            <option :value="3000" class="bg-atelier-900 text-ink-100">3000 字 (大章)</option>
          </select>
        </div>

        <div class="flex items-center gap-1.5 shrink-0">
          <!-- 暂存状态提示 -->
          <div v-if="state.isSavingDraft" class="hidden sm:flex items-center gap-1 text-[11px] text-brand-amber">
            <Loader2 class="w-3 h-3 animate-spin" />
            <span>存档中...</span>
          </div>
          <span v-else-if="state.lastSavedAt" class="hidden sm:inline text-[10px] text-ink-400 font-mono" :title="`上次存档: ${formatSaveTime(state.lastSavedAt)}`">
            已存 {{ formatSaveTime(state.lastSavedAt) }}
          </span>

          <button 
            @click="handleManualSaveDraft" 
            :disabled="state.isSavingDraft"
            class="px-2.5 py-1 text-[11px] font-medium bg-brand-amber/15 hover:bg-brand-amber/25 text-brand-amber rounded-md border border-brand-amber/30 transition flex items-center gap-1 cursor-pointer"
            title="手动将当前草稿及工步进度保存至本地数据库">
            <Save class="w-3 h-3" />
            <span class="hidden sm:inline">保存草稿</span>
          </button>

          <button 
            v-if="state.workbench?.content || (state.workbench?.beats || []).some(b => b.action) || state.workbench?.coreConflict"
            @click="handleDiscardDraft" 
            class="px-2 py-1 text-[11px] font-medium bg-atelier-850 hover:bg-brand-rose/20 text-ink-400 hover:text-brand-rose rounded-md border border-atelier-750 hover:border-brand-rose/30 transition flex items-center gap-1 cursor-pointer"
            title="清除当前草稿并重置当前章节工作区">
            <Trash2 class="w-3 h-3" />
            <span class="hidden md:inline">放弃草稿</span>
          </button>

          <button 
            @click="actions.runLinter" 
            class="px-2.5 py-1 text-[11px] font-medium bg-atelier-850 hover:bg-atelier-800 text-ink-300 hover:text-ink-100 rounded-md border border-atelier-750 transition flex items-center gap-1 cursor-pointer">
            <Sparkles class="w-3 h-3 text-brand-amber" />
            <span class="hidden sm:inline">反AI味质检</span>
          </button>

          <button 
            @click="openHarmonizeModal" 
            class="hidden md:flex px-2.5 py-1 text-[11px] font-medium bg-atelier-850 hover:bg-atelier-800 text-ink-300 hover:text-ink-100 rounded-md border border-atelier-750 transition items-center gap-1 cursor-pointer"
            title="平滑替换敏感词与对抗扰动">
            <ShieldCheck class="w-3 h-3 text-brand-emerald" />
            <span>合规扰动</span>
          </button>

          <button 
            @click="openHumanTouchesModal" 
            class="hidden md:flex px-2.5 py-1 text-[11px] font-medium bg-atelier-850 hover:bg-atelier-800 text-ink-300 hover:text-ink-100 rounded-md border border-atelier-750 transition items-center gap-1 cursor-pointer"
            title="生成生理不适、世俗闲笔等真实人味">
            <HeartHandshake class="w-3 h-3 text-brand-amber" />
            <span>人味注入</span>
          </button>

          <span 
            v-if="state.reviewResult"
            class="text-[10px] font-mono font-bold px-2 py-0.5 rounded border"
            :class="state.reviewResult.score >= 80 ? 'bg-brand-emerald/15 text-brand-emerald border-brand-emerald/30' : 'bg-brand-rose/15 text-brand-rose border-brand-rose/30'">
            终审: {{ state.reviewResult.score }}分
          </span>

          <!-- 重新展开视界面板快捷按钮 (当右侧折叠时呈现) -->
          <button 
            v-if="!state.showHorizonPanel && !state.isZenMode"
            @click="state.showHorizonPanel = true"
            class="px-2 py-1 rounded bg-atelier-850 hover:bg-atelier-800 border border-atelier-750 text-xs text-brand-cyan flex items-center gap-1 transition cursor-pointer"
            title="展开右侧因果视界">
            <PanelRight class="w-3.5 h-3.5" />
            <span class="text-[11px] font-medium">展开视界</span>
          </button>
        </div>
      </div>

      <!-- 划词 / 段落 Inline AI 快速伴写浮动条 -->
      <div class="px-5 py-2 bg-atelier-900/60 border-b border-atelier-750 flex items-center justify-between text-xs shrink-0 select-none overflow-x-auto">
        <div class="flex items-center gap-1.5 flex-nowrap shrink-0">
          <span class="text-ink-400 text-[10px] font-mono uppercase tracking-wider mr-1 shrink-0">划词伴写:</span>
          <button 
            v-for="act in inlineActionButtons" 
            :key="act.action"
            @click="runInlineAction(act.action)"
            :disabled="inlineActionLoading || !state.workbench.content"
            class="px-2.5 py-1 rounded-md bg-atelier-850 hover:bg-atelier-800 text-ink-200 hover:text-ink-50 border border-atelier-700 text-[11px] transition flex items-center gap-1 cursor-pointer disabled:opacity-40 shrink-0">
            <component :is="act.icon" class="w-3 h-3 text-brand-amber" />
            <span>{{ act.label }}</span>
          </button>
          <button 
            @click="openCustomInlinePrompt"
            :disabled="inlineActionLoading || !state.workbench.content"
            class="px-2.5 py-1 rounded-md bg-atelier-850 hover:bg-atelier-800 text-brand-amber border border-atelier-700 text-[11px] transition flex items-center gap-1 cursor-pointer disabled:opacity-40 shrink-0">
            <Wand2 class="w-3 h-3 text-brand-amber" />
            <span>自定义指令</span>
          </button>
        </div>

        <div v-if="inlineActionLoading" class="flex items-center gap-1 text-[11px] text-brand-amber shrink-0 ml-3">
          <Loader2 class="w-3 h-3 animate-spin" />
          <span>AI 运算中...</span>
        </div>
      </div>

      <!-- 划词 AI 结果差分对比横幅 -->
      <div v-if="showInlineDiff && inlineActionResult" class="p-3.5 bg-atelier-900/90 border-b border-brand-amber/30 space-y-2 shrink-0">
        <div class="flex items-center justify-between text-xs">
          <span class="font-bold text-brand-amber flex items-center gap-1.5">
            <Sparkles class="w-3.5 h-3.5" />
            <span>AI 改写结果 ({{ inlineActionResult.action }})</span>
          </span>
          <div class="flex gap-2">
            <button @click="applyInlineResult" class="px-2.5 py-1 bg-brand-emerald hover:bg-emerald-500 text-atelier-950 font-bold rounded text-xs transition cursor-pointer">
              ✓ 采纳替换
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
      <div class="flex-1 flex overflow-y-auto p-4 md:p-8 justify-center">
        <div class="w-full max-w-4xl xl:max-w-5xl 2xl:max-w-6xl flex flex-col h-full bg-atelier-900/20 rounded-xl p-4 md:p-8 border border-atelier-800/40 shadow-inner transition-all">
          <textarea 
            id="prose-textarea"
            v-model="state.workbench.content"
            @input="handleProseInput"
            class="flex-1 w-full bg-transparent text-ink-100 text-base md:text-[17px] leading-[2.1] font-serif resize-none focus:outline-none placeholder-ink-500 selection:bg-brand-amber/30 selection:text-ink-50 tracking-wide prose-canvas"
            placeholder="正文手稿由此展开……支持手动写作或依据左侧流水线进行文学渲染。"></textarea>
        </div>
      </div>
    </div>

    <!-- 右栏：因果锁链视界 (宽 280px，支持一键折叠) -->
    <div 
      v-show="state.showHorizonPanel && !state.isZenMode" 
      class="w-72 border-l border-atelier-750 bg-atelier-900/30 flex flex-col shrink-0 overflow-y-auto p-3.5 space-y-3.5 transition-all duration-200 select-none">
      
      <div>
        <span class="text-[10px] font-mono uppercase tracking-wider text-ink-400 block mb-0.5">
          因果锁链视界 (Horizon)
        </span>
        <h4 class="text-xs font-bold text-ink-200 font-serif">全域时空与物理状态</h4>
      </div>

      <!-- 主角状态机快照 -->
      <div class="p-3 bg-atelier-950/70 border border-atelier-750 rounded-lg space-y-1.5 text-xs">
        <span class="font-bold text-brand-emerald flex items-center gap-1.5">
          <Cpu class="w-3.5 h-3.5" />
          <span>主角因果账本</span>
        </span>
        <div class="text-[11px] text-ink-300">
          境界：<strong class="text-ink-100 font-semibold">{{ state.currentProject?.protagonist?.name_and_level || '凡人' }}</strong>
        </div>
        <div class="text-[11px] text-ink-300">
          行囊道具：<strong class="text-ink-100">{{ state.currentProject?.protagonist?.inventory || '无' }}</strong>
        </div>
      </div>

      <!-- 上章收尾文风锚点 (Tail Anchor) -->
      <div v-if="state.chapters.length > 0" class="p-3 bg-atelier-950/70 border border-atelier-750 rounded-lg space-y-1.5 text-xs">
        <span class="font-bold text-brand-amber flex items-center gap-1.5">
          <Anchor class="w-3.5 h-3.5" />
          <span>上章尾部腔调锚点</span>
        </span>
        <p class="text-[11px] text-ink-300 font-serif leading-relaxed italic line-clamp-4">
          “{{ lastChapterTail }}”
        </p>
      </div>

      <!-- 本场共现百科实体 -->
      <div class="p-3 bg-atelier-950/70 border border-atelier-750 rounded-lg space-y-2 text-xs">
        <div class="flex items-center justify-between">
          <span class="font-bold text-brand-cyan flex items-center gap-1.5">
            <BookOpen class="w-3.5 h-3.5" />
            <span>全域百科活跃度</span>
          </span>
          <span class="text-[10px] font-mono text-ink-400">{{ state.codexEntries.length }} 实体</span>
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
          暂无百科实体，可在 The Codex 建立世界观。
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue';
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
  Trash2
} from 'lucide-vue-next';

const workflowSteps = [
  { id: 1, shortLabel: '冲突' },
  { id: 2, shortLabel: '节拍' },
  { id: 3, shortLabel: '渲染' },
  { id: 4, shortLabel: '质检' },
  { id: 5, shortLabel: '返工' },
  { id: 6, shortLabel: '归档' },
];

const inspirationPresets = [
  { title: '拍卖截胡', desc: '低买高坑，暗藏乾坤', conflict: '主角在黑市拍卖会发现被掩盖的远古残器，反派欲恶意抬价加害，主角顺水推舟设局反坑反派万两灵石。' },
  { title: '考核越级', desc: '以弱胜强，众目反转', conflict: '内门考核中反派暗中篡改抽签安排强敌，主角暗藏底牌在众目睽睽下以弱胜强，打破全场轻视。' },
  { title: '密室反杀', desc: '利用公理，反向栽赃', conflict: '宗门密室内遭死士围剿，主角利用对世界规则的独特理解触发禁制，反向猎杀并栽赃幕后黑手。' },
  { title: '伏笔收网', desc: '暗线引爆，逆转攻守', conflict: '前序埋伏的暗线被敌方误以为是破绽发起决战，主角瞬间翻开底牌彻底逆转攻守并回收悬念。' },
];

const inlineActionButtons = [
  { action: 'rewrite', label: '重写润色', icon: Scissors },
  { action: 'expand', label: '细节扩写', icon: Maximize2 },
  { action: 'shorten', label: '精简提炼', icon: Minimize2 },
  { action: 'sensory', label: '五感具象', icon: Eye },
  { action: 'dialogue', label: '打磨台词', icon: MessageSquare },
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

// 节拍推演
async function handleDeriveBeats() {
  if (!state.currentProject || !state.workbench.coreConflict.trim()) return;
  state.isGeneratingBeats = true;
  try {
    const res = await api.deriveBeats(state.currentProject.id, {
      chapter_index: computedState.currentWorkingChapterIndex.value,
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
    notify('节拍推演完成', '已根据前序正史生成因果四段论节拍', 'success');
  } catch (e) {
    notify('推演节拍失败', e.message, 'error');
  } finally {
    state.isGeneratingBeats = false;
  }
}

// 文学渲染
async function handleRenderScene() {
  if (!state.currentProject) return;
  state.isRenderingScene = true;
  try {
    const res = await api.renderScene(state.currentProject.id, {
      chapter_index: computedState.currentWorkingChapterIndex.value,
      beats: state.workbench.beats,
      words_target: state.wordsTarget || 2000,
      narrative_style: state.narrativeStyle || 'hardboiled',
    });
    state.workbench.content = res.content || '';
    state.activeStep = 3;
    await actions.runLinter();
    await actions.saveCheckpoint();
    notify('正文初稿生成完毕', `完成约 ${state.workbench.content.length} 字文学渲染`, 'success');
  } catch (e) {
    notify('渲染正文失败', e.message, 'error');
  } finally {
    state.isRenderingScene = false;
  }
}

async function confirmAndRenderScene() {
  if (state.workbench.content.trim()) {
    const ok = await dialogs.confirm({
      title: '重新渲染正文',
      message: '重新生成将基于当前设定的因果节拍覆盖现有正文手稿，是否确认重新渲染？',
      type: 'warning',
      confirmText: '确认重新生成',
    });
    if (!ok) return;
  }
  await handleRenderScene();
}

// 终审质检
async function handleReviewDraft() {
  if (!state.currentProject || !state.workbench.content.trim()) return;
  state.isReviewing = true;
  try {
    const res = await api.reviewDraft(state.currentProject.id, {
      chapter_index: computedState.currentWorkingChapterIndex.value,
      content: state.workbench.content,
      beats: state.workbench.beats,
    });
    state.reviewResult = res;
    state.activeStep = res.verdict === 'ACCEPTED' ? 6 : 5;
    await actions.saveCheckpoint();
    notify('主编终审完成', `得分: ${res.score} · 裁决: ${res.verdict}`, res.verdict === 'ACCEPTED' ? 'success' : 'info');
  } catch (e) {
    notify('审查失败', e.message, 'error');
  } finally {
    state.isReviewing = false;
  }
}

// 定向返工
async function handleRewriteDraft() {
  if (!state.currentProject || !state.workbench.content.trim()) return;
  state.isRewriting = true;
  try {
    const issues = state.reviewResult?.issues?.length 
      ? state.reviewResult.issues 
      : ['需根据去AI味与语法健全原则进行深度精修'];
    const suggestions = state.reviewResult?.suggestions 
      || '请重塑叙事节奏，补齐主谓宾完整结构，消除机械断句与模式化废词。';

    const res = await api.rewriteDraft(state.currentProject.id, {
      chapter_index: computedState.currentWorkingChapterIndex.value,
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
    notify('针对性返工完成', '已采纳修改意见，正文已替换更新', 'success');
  } catch (e) {
    notify('返工失败', e.message, 'error');
  } finally {
    state.isRewriting = false;
  }
}

// 提交归档
async function handleCommitChapter() {
  if (!state.currentProject || !state.workbench.content.trim()) return;
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
    notify('章节已成功封存归档', `第 ${targetIndex} 章已记录入正史与状态账本`, 'success');
    state.editingChapterIndex = null;
    await actions.selectProject(state.currentProject.id);
    state.workbench.content = '';
    state.workbench.coreConflict = '';
    state.reviewResult = null;
    state.activeStep = 1;
  } catch (e) {
    notify('归档失败', e.message, 'error');
  }
}

async function resetToNewChapter() {
  if (state.workbench.content.trim() || state.workbench.coreConflict.trim()) {
    const ok = await dialogs.confirm({
      title: '切换创作新章节',
      message: '当前手稿或冲突设定尚未归档，放弃精修并切换至新章节将重置工作台画布，是否继续？',
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
    title: '自定义 AI 伴写指令',
    message: '请输入你对选中文本的具体打磨、扩写或风格润色要求：',
    placeholder: '例如：增加周遭环境的阴冷与铁锈气味、用冷峻动作代替心理白描、强化暴风雨降临前的心理压迫...',
    multiline: true,
    confirmText: '执行 AI 指令',
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
  notify('已采纳改写结果', '正文已更新', 'success');
}

async function handleManualSaveDraft() {
  const ok = await actions.saveCheckpoint({}, false);
  if (ok) {
    notify('草稿已保存', '当前章节全部工步与正文已同步存档至数据库', 'success');
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
</script>
