<template>
  <div class="flex-1 p-6 md:p-8 overflow-y-auto w-full max-w-6xl mx-auto space-y-6" v-if="state.currentProject">
    <!-- 头部工具栏 -->
    <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4 border-b border-atelier-750 pb-5">
      <div class="flex items-center gap-3">
        <div class="w-9 h-9 rounded-xl bg-brand-amber/10 border border-brand-amber/25 flex items-center justify-center text-brand-amber shadow-amber-glow">
          <Compass class="w-4 h-4" />
        </div>
        <div>
          <div class="flex items-center gap-2">
            <h2 class="text-base font-serif font-bold text-ink-50 tracking-wide">作品设定</h2>
            <span class="text-[10px] font-mono px-1.5 py-0.5 rounded bg-brand-amber/15 text-brand-amber border border-brand-amber/30">
              作品级 · 绑定当前作品
            </span>
          </div>
          <p class="text-xs text-ink-400 mt-0.5">全书核心规则、偏好基调与宏观走向。</p>
        </div>
      </div>

      <div class="flex items-center gap-2.5">
        <button 
          @click="actions.bootstrapCurrentFramework()" 
          :disabled="state.isLoading" 
          class="flex items-center gap-1.5 px-3 py-2 bg-atelier-800 hover:bg-atelier-700 text-brand-amber text-xs font-semibold rounded-md border border-atelier-700 transition cursor-pointer disabled:opacity-50">
          <Loader2 v-if="state.isLoading" class="w-3.5 h-3.5 animate-spin" />
          <Sparkles v-else class="w-3.5 h-3.5" />
          <span>{{ state.isLoading ? '推演中...' : 'AI 推演' }}</span>
        </button>
        <button 
          @click="handleSaveFramework" 
          class="flex items-center gap-1.5 px-4 py-2 bg-brand-amber hover:bg-brand-amber-hover text-atelier-950 font-bold text-xs rounded-md shadow-amber-glow transition cursor-pointer">
          <Save class="w-3.5 h-3.5" />
          <span>保存设定</span>
        </button>
      </div>
    </div>

    <!-- 1. 作品篇幅与风格偏好 (作品级设置) -->
    <div class="p-5 bg-atelier-900 border border-atelier-750 rounded-xl space-y-4 shadow-atelier-md">
      <div class="flex items-center justify-between">
        <div>
          <h3 class="text-xs font-serif font-bold text-ink-100 flex items-center gap-2">
            <Sliders class="w-3.5 h-3.5 text-brand-amber" />
            <span>作品偏好</span>
          </h3>
          <p class="text-[11px] text-ink-400 mt-0.5">全书默认单章目标字数与基调风格，各章节默认继承。</p>
        </div>
      </div>

      <div class="grid grid-cols-1 md:grid-cols-2 gap-5">
        <!-- 默认单章字数 -->
        <div class="p-3 bg-atelier-950 rounded-lg border border-atelier-800 space-y-2">
          <div class="flex items-center justify-between">
            <label class="text-xs font-medium text-ink-300">单章字数：</label>
            <span class="text-[11px] font-mono text-brand-amber font-bold">
              {{ state.currentProject.default_words_target || 2000 }} 字
            </span>
          </div>
          <div class="grid grid-cols-4 gap-1.5">
            <button 
              v-for="target in [1500, 2000, 2500, 3000]" 
              :key="target"
              @click="setProjectWordsTarget(target)"
              class="py-1 text-xs font-mono rounded border transition cursor-pointer text-center"
              :class="(state.currentProject.default_words_target || 2000) === target ? 'bg-brand-amber/15 text-brand-amber border-brand-amber/40 font-bold' : 'bg-atelier-900 text-ink-300 border-atelier-750 hover:bg-atelier-850'">
              {{ target }}
            </button>
          </div>
        </div>

        <!-- 默认叙事基调 -->
        <div class="p-3 bg-atelier-950 rounded-lg border border-atelier-800 space-y-2">
          <label class="block text-xs font-medium text-ink-300">叙事风格：</label>
          <div class="grid grid-cols-3 sm:grid-cols-5 gap-1.5">
            <button 
              v-for="tone in [
                { id: 'hardboiled', name: '冷峻凝练' },
                { id: 'high_tension', name: '热血张力' },
                { id: 'classical', name: '古典雅致' },
                { id: 'vernacular', name: '生动市井' },
                { id: 'cinematic', name: '全景镜头' }
              ]"
              :key="tone.id"
              @click="setProjectNarrativeStyle(tone.id)"
              class="py-1 px-1.5 text-xs rounded border transition cursor-pointer text-center truncate"
              :class="(state.currentProject.default_narrative_style || 'hardboiled') === tone.id ? 'bg-brand-amber/15 text-brand-amber border-brand-amber/40 font-bold' : 'bg-atelier-900 text-ink-300 border-atelier-750 hover:bg-atelier-850'">
              {{ tone.name }}
            </button>
          </div>
        </div>

        <!-- 视觉与氛围锚点 -->
        <div v-if="state.currentProject.framework" class="p-3 bg-atelier-950 rounded-lg border border-atelier-800 space-y-1.5 md:col-span-2">
          <div class="flex items-center justify-between">
            <label class="block text-xs font-medium text-ink-300">视觉与氛围锚点：</label>
            <span class="text-[10px] text-ink-400">全书恒定感官基调，渲染时强制约束</span>
          </div>
          <textarea 
            v-model="state.currentProject.framework.atmosphere_anchor" 
            rows="2" 
            class="w-full bg-atelier-900 border border-atelier-750 rounded-md p-2 text-xs text-ink-100 font-serif focus:outline-none focus:border-brand-amber/60 leading-relaxed resize-none" 
            placeholder="例如：高反差冷色调 / 破败废土 / 阴冷民俗..."></textarea>
        </div>
      </div>
    </div>

    <!-- 2. 世界观空白状态 -->
    <div 
      v-if="!state.currentProject.framework" 
      class="p-10 text-center bg-atelier-900 border border-dashed border-atelier-750 rounded-xl space-y-4 shadow-atelier-md">
      <div class="w-12 h-12 rounded-xl bg-brand-amber/10 border border-brand-amber/25 flex items-center justify-center mx-auto text-brand-amber">
        <Compass class="w-6 h-6" />
      </div>
      <div class="space-y-1">
        <h3 class="text-sm font-serif font-bold text-ink-100">尚未生成宏观世界观</h3>
        <p class="text-xs text-ink-400 max-w-md mx-auto leading-relaxed">
          先立规矩，小说才不容易卡文或战力崩溃。AI 将为你一键构建世界规则、战力阶梯、分卷规划与势力阵营。
        </p>
      </div>
      <button 
        @click="actions.bootstrapCurrentFramework()" 
        class="px-4 py-2 bg-brand-amber hover:bg-brand-amber-hover text-atelier-950 font-bold text-xs rounded-lg shadow-amber-glow transition cursor-pointer inline-flex items-center gap-2">
        <Sparkles class="w-3.5 h-3.5" />
        <span>一键推演世界观</span>
      </button>
    </div>

    <!-- 3. 世界观设定总览表单区 -->
    <div v-else class="space-y-6">
      <!-- 核心立意与主角档案 -->
      <div class="grid grid-cols-1 lg:grid-cols-3 gap-5">
        <!-- 核心立意 -->
        <div class="p-4 bg-atelier-900 border border-atelier-750 rounded-xl space-y-2 lg:col-span-1 shadow-atelier-sm">
          <div class="flex items-center gap-2 text-xs font-serif font-bold text-brand-amber">
            <Lightbulb class="w-3.5 h-3.5" />
            <span>核心立意</span>
          </div>
          <p class="text-[10px] text-ink-400">一句话概括全书主线与核心看点。</p>
          <textarea 
            v-model="state.currentProject.framework.theme_premise" 
            rows="4" 
            class="w-full bg-atelier-950 border border-atelier-750 rounded-lg p-2.5 text-xs text-ink-100 font-serif focus:outline-none focus:border-brand-amber/60 leading-relaxed resize-none" 
            placeholder="输入全书核心设定与立意..."></textarea>
          <div class="pt-2 border-t border-atelier-800 space-y-1">
            <label class="block text-[11px] font-medium text-ink-300">一句话梗概 (Logline)：</label>
            <textarea 
              v-model="state.currentProject.framework.logline" 
              rows="2" 
              class="w-full bg-atelier-950 border border-atelier-750 rounded-lg p-2 text-xs text-ink-100 font-serif focus:outline-none focus:border-brand-amber/60 leading-relaxed resize-none" 
              placeholder="一句话概括：主角+冲突+危机..."></textarea>
          </div>
        </div>

        <!-- 主角档案 -->
        <div 
          v-if="computedState.protagonistCharacter.value" 
          class="p-4 bg-atelier-900 border border-atelier-750 rounded-xl space-y-2.5 lg:col-span-2 shadow-atelier-sm">
          <div class="flex items-center gap-2 text-xs font-serif font-bold text-brand-amber">
            <User class="w-3.5 h-3.5" />
            <span>主角档案</span>
          </div>
          <p class="text-[10px] text-ink-400">主角姓名、当前境界、核心动机与健康状态。</p>
          
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-3 text-xs pt-1">
            <div>
              <label class="text-ink-400 text-[11px] font-medium">姓名：</label>
              <input 
                v-model="computedState.protagonistCharacter.value.name" 
                class="w-full mt-1 bg-atelier-950 border border-atelier-750 rounded-md px-2.5 py-1.5 text-xs text-brand-amber font-mono font-bold focus:outline-none focus:border-brand-amber/60">
            </div>
            <div>
              <label class="text-ink-400 text-[11px] font-medium">当前境界：</label>
              <input 
                v-model="computedState.protagonistCharacter.value.realm" 
                class="w-full mt-1 bg-atelier-950 border border-atelier-750 rounded-md px-2.5 py-1.5 text-xs text-brand-amber font-mono font-bold focus:outline-none focus:border-brand-amber/60"
                placeholder="例如：练气六层巅峰">
            </div>
            <div>
              <label class="text-ink-400 text-[11px] font-medium">战备/健康状态：</label>
              <input 
                v-model="computedState.protagonistCharacter.value.health_status" 
                class="w-full mt-1 bg-atelier-950 border border-atelier-750 rounded-md px-2.5 py-1.5 text-xs text-emerald-400 font-mono focus:outline-none focus:border-brand-amber/60">
            </div>
            <div>
              <label class="text-ink-400 text-[11px] font-medium">外在欲望 (Want)：</label>
              <input 
                v-model="computedState.protagonistCharacter.value.want" 
                class="w-full mt-1 bg-atelier-950 border border-atelier-750 rounded-md px-2.5 py-1.5 text-xs text-ink-200 focus:outline-none focus:border-brand-amber/60"
                placeholder="内在欲望：自以为追求的外在目标">
            </div>
            <div>
              <label class="text-ink-400 text-[11px] font-medium">真实需求 (Need)：</label>
              <input 
                v-model="computedState.protagonistCharacter.value.need" 
                class="w-full mt-1 bg-atelier-950 border border-atelier-750 rounded-md px-2.5 py-1.5 text-xs text-ink-200 focus:outline-none focus:border-brand-amber/60"
                placeholder="真实需求：真正需要面对的内心成长课题">
            </div>
            <div>
              <label class="text-ink-400 text-[11px] font-medium">核心创伤 (Ghost)：</label>
              <input 
                v-model="computedState.protagonistCharacter.value.ghost" 
                class="w-full mt-1 bg-atelier-950 border border-atelier-750 rounded-md px-2.5 py-1.5 text-xs text-ink-200 focus:outline-none focus:border-brand-amber/60"
                placeholder="核心鬼魂：过去的创伤执念与恐惧弱点">
            </div>
            <div class="sm:col-span-2">
              <label class="text-ink-400 text-[11px] font-medium">随身携带道具 (Inventory)：</label>
              <textarea 
                v-model="computedState.protagonistCharacter.value.inventory" 
                rows="2"
                class="w-full mt-1 bg-atelier-950 border border-atelier-750 rounded-md p-2 text-xs text-ink-300 font-mono focus:outline-none focus:border-brand-amber/60 resize-none leading-relaxed"></textarea>
            </div>
          </div>
        </div>
      </div>

      <!-- 有意母题与符号 -->
      <div class="p-4 bg-atelier-900 border border-atelier-750 rounded-xl space-y-2 shadow-atelier-sm">
        <div class="flex items-center justify-between">
          <div class="flex items-center gap-2 text-xs font-serif font-bold text-brand-amber">
            <Sparkles class="w-3.5 h-3.5" />
            <span>母题与符号</span>
          </div>
          <span class="text-[10px] font-mono text-ink-500">{{ (state.currentProject.framework.motifs || []).length }} / 8</span>
        </div>
        <p class="text-[10px] text-ink-400">每行一条有意反复的意象或符号；渲染时会鼓励自然复现以强化主题回响，但严禁机械复读。</p>
        <textarea
          v-model="motifsText"
          rows="3"
          class="w-full bg-atelier-950 border border-atelier-750 rounded-lg p-2.5 text-xs text-ink-100 font-serif focus:outline-none focus:border-brand-amber/60 leading-relaxed resize-none"
          placeholder="例：冰冷蓝色数据光效&#10;例：机械蜂鸣与心跳同频"></textarea>
      </div>

      <!-- 世界规则 -->
      <div class="p-4 bg-atelier-900 border border-atelier-750 rounded-xl space-y-3 shadow-atelier-sm">
        <div class="flex items-center justify-between">
          <div>
            <div class="flex items-center gap-2 text-xs font-serif font-bold text-brand-amber">
              <Scale class="w-3.5 h-3.5" />
              <span>世界规则</span>
            </div>
            <p class="text-[11px] text-ink-400 mt-0.5">全书不可违背的底层物理、运行铁律与因果约束。</p>
          </div>
          <button 
            @click="addFrameworkAxiom" 
            class="text-xs text-brand-amber hover:text-brand-amber-hover font-medium flex items-center gap-1 cursor-pointer">
            <Plus class="w-3 h-3" />
            <span>添加规则</span>
          </button>
        </div>

        <div class="space-y-2">
          <div 
            v-for="(axiom, aIdx) in state.currentProject.framework.world_axioms" 
            :key="aIdx" 
            class="flex items-center gap-2.5 bg-atelier-950 p-2 rounded-lg border border-atelier-800">
            <span class="text-brand-amber font-mono text-xs w-6 shrink-0 text-center">{{ aIdx + 1 }}.</span>
            <input 
              v-model="state.currentProject.framework.world_axioms[aIdx]" 
              class="flex-1 bg-transparent text-xs text-ink-100 font-serif focus:outline-none">
            <button 
              @click="removeFrameworkAxiom(aIdx)" 
              class="text-ink-500 hover:text-rose-400 text-xs px-1.5 transition cursor-pointer">
              ✕
            </button>
          </div>
        </div>
      </div>

      <!-- 战力阶梯 -->
      <div class="p-4 bg-atelier-900 border border-atelier-750 rounded-xl space-y-3 shadow-atelier-sm">
        <div class="flex items-center justify-between">
          <div>
            <div class="flex items-center gap-2 text-xs font-serif font-bold text-brand-amber">
              <Layers class="w-3.5 h-3.5" />
              <span>战力阶梯</span>
            </div>
            <p class="text-[11px] text-ink-400 mt-0.5">全书每层境界的破坏力、突破瓶颈与代价副作用。</p>
          </div>
          <button 
            @click="addFrameworkTier" 
            class="text-xs text-brand-amber hover:text-brand-amber-hover font-medium flex items-center gap-1 cursor-pointer">
            <Plus class="w-3 h-3" />
            <span>添加阶梯</span>
          </button>
        </div>

        <div class="overflow-x-auto">
          <table class="w-full text-left text-xs border-collapse">
            <thead>
              <tr class="border-b border-atelier-750 text-ink-400 text-[11px]">
                <th class="py-2.5 px-2 w-12 font-mono text-center">阶</th>
                <th class="py-2.5 px-3 w-36">境界名称</th>
                <th class="py-2.5 px-3">破坏力表征</th>
                <th class="py-2.5 px-3">突破瓶颈</th>
                <th class="py-2.5 px-3">代价与副作用</th>
                <th class="py-2.5 px-2 w-10 text-center">操作</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-atelier-800 font-sans">
              <tr 
                v-for="(tier, tIdx) in state.currentProject.framework.power_ladder" 
                :key="tIdx" 
                class="hover:bg-atelier-850/50">
                <td class="py-2 px-2 font-mono text-brand-amber font-bold text-center">
                  {{ tier.tier || (tIdx + 1) }}
                </td>
                <td class="py-2 px-3">
                  <input 
                    v-model="tier.realm" 
                    placeholder="境界/阶段名称"
                    class="w-full bg-atelier-950 border border-atelier-750 rounded px-2 py-1 text-xs text-purple-300 font-semibold focus:outline-none focus:border-brand-amber/60">
                </td>
                <td class="py-2 px-3">
                  <input 
                    v-model="tier.description" 
                    placeholder="破坏力与能力表征"
                    class="w-full bg-atelier-950 border border-atelier-750 rounded px-2 py-1 text-xs text-ink-200 focus:outline-none focus:border-brand-amber/60">
                </td>
                <td class="py-2 px-3">
                  <input 
                    v-model="tier.bottleneck" 
                    placeholder="突破门槛与关卡"
                    class="w-full bg-atelier-950 border border-atelier-750 rounded px-2 py-1 text-xs text-ink-300 focus:outline-none focus:border-brand-amber/60">
                </td>
                <td class="py-2 px-3">
                  <input 
                    v-model="tier.drawback" 
                    placeholder="突破代价或能力反噬"
                    class="w-full bg-atelier-950 border border-atelier-750 rounded px-2 py-1 text-xs text-rose-300 focus:outline-none focus:border-brand-amber/60">
                </td>
                <td class="py-2 px-2 text-center">
                  <button 
                    @click="removeFrameworkTier(tIdx)" 
                    class="text-ink-500 hover:text-rose-400 text-xs transition cursor-pointer">
                    ✕
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <!-- 全书节拍 -->
      <div class="p-4 bg-atelier-900 border border-atelier-750 rounded-xl space-y-3 shadow-atelier-sm">
        <div class="flex items-center justify-between">
          <div>
            <div class="flex items-center gap-2 text-xs font-serif font-bold text-brand-amber">
              <Compass class="w-3.5 h-3.5" />
              <span>全书节拍</span>
            </div>
            <p class="text-[11px] text-ink-400 mt-0.5">全书宏观三幕/救猫咪节拍锚点与章节跨度定位。</p>
          </div>
          <button 
            @click="addFrameworkMilestone" 
            class="text-xs text-brand-amber hover:text-brand-amber-hover font-medium flex items-center gap-1 cursor-pointer">
            <Plus class="w-3 h-3" />
            <span>添加里程碑</span>
          </button>
        </div>

        <div class="flex items-center gap-3 bg-atelier-950 p-2.5 rounded-lg border border-atelier-800 text-xs">
          <label class="text-ink-300 font-medium shrink-0">全书计划总章数：</label>
          <input 
            type="number" 
            min="1" 
            v-model.number="state.currentProject.framework.total_planned_chapters" 
            class="w-24 bg-atelier-900 border border-atelier-750 rounded px-2.5 py-1 text-xs text-brand-amber font-mono font-bold focus:outline-none focus:border-brand-amber/60"
            placeholder="如 120">
          <span class="text-[10px] text-ink-500">章（0 为未定）</span>
        </div>

        <!-- 空态提示 -->
        <div 
          v-if="!(state.currentProject.framework.story_milestones && state.currentProject.framework.story_milestones.length > 0)"
          class="p-6 text-center bg-atelier-950 rounded-lg border border-dashed border-atelier-800 text-xs text-ink-400">
          尚未规划全书节拍，可让 AI 创世推演或手动添加
        </div>

        <!-- 列表 -->
        <div v-else class="space-y-2">
          <div 
            v-for="(ms, mIdx) in (state.currentProject.framework.story_milestones || [])" 
            :key="ms.key || mIdx"
            class="flex items-center gap-2.5 p-2.5 bg-atelier-950 border border-atelier-800 rounded-lg text-xs">
            <div class="flex-1 min-w-0 flex items-center gap-2">
              <span class="text-[10px] font-mono text-ink-500 shrink-0">#{{ mIdx + 1 }}</span>
              <input 
                v-model="ms.name" 
                placeholder="里程碑名称（如：触发事件、中点）"
                class="flex-1 bg-atelier-900 border border-atelier-750 rounded px-2 py-1 text-xs text-ink-100 font-serif focus:outline-none focus:border-brand-amber/60">
            </div>

            <div class="flex items-center gap-1.5 shrink-0">
              <span class="text-ink-400 text-[11px]">第</span>
              <input 
                type="number" 
                min="1" 
                v-model.number="ms.target_chapter" 
                placeholder="目标章"
                class="w-16 bg-atelier-900 border border-atelier-750 rounded px-2 py-1 text-xs text-brand-amber font-mono font-bold text-center focus:outline-none focus:border-brand-amber/60">
              <span class="text-ink-400 text-[11px]">章</span>
            </div>

            <button 
              @click="toggleFrameworkMilestoneStatus(ms)"
              class="px-2 py-1 rounded text-[11px] font-medium border transition cursor-pointer shrink-0"
              :class="ms.status === 'DONE' ? 'bg-emerald-500/15 text-emerald-400 border-emerald-500/30' : 'bg-brand-amber/15 text-brand-amber border-brand-amber/30'">
              {{ ms.status === 'DONE' ? '已兑现' : '待兑现' }}
            </button>

            <button 
              @click="removeFrameworkMilestone(mIdx)" 
              class="text-ink-500 hover:text-rose-400 text-xs px-1 transition cursor-pointer shrink-0"
              title="删除">
              ✕
            </button>
          </div>
        </div>
      </div>

      <!-- 分卷规划 -->
      <div class="p-4 bg-atelier-900 border border-atelier-750 rounded-xl space-y-3 shadow-atelier-sm">
        <div class="flex items-center justify-between">
          <div>
            <div class="flex items-center gap-2 text-xs font-serif font-bold text-brand-amber">
              <ScrollText class="w-3.5 h-3.5" />
              <span>分卷规划</span>
            </div>
            <p class="text-[11px] text-ink-400 mt-0.5">每卷章节跨度、阶段目标与终局高潮爆发点。</p>
          </div>
          <button 
            @click="addFrameworkVolume" 
            class="text-xs text-brand-amber hover:text-brand-amber-hover font-medium flex items-center gap-1 cursor-pointer">
            <Plus class="w-3 h-3" />
            <span>添加分卷</span>
          </button>
        </div>

        <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div 
            v-for="(vol, vIdx) in state.currentProject.framework.volume_arcs" 
            :key="vIdx" 
            class="p-4 bg-atelier-950 border border-atelier-750 rounded-lg space-y-2.5 relative">
            <button 
              @click="removeFrameworkVolume(vIdx)" 
              class="absolute top-2.5 right-2.5 text-ink-500 hover:text-rose-400 text-xs transition cursor-pointer">
              ✕
            </button>
            <div class="flex items-center gap-2">
              <span class="text-xs font-mono font-bold text-brand-amber shrink-0">
                第 {{ vol.volume_index || (vIdx + 1) }} 卷
              </span>
              <input 
                v-model="vol.title" 
                class="flex-1 bg-atelier-900 border border-atelier-750 rounded px-2 py-1 text-xs text-ink-100 font-serif font-bold focus:outline-none focus:border-brand-amber/60" 
                placeholder="分卷标题">
              <div class="flex items-center gap-1 text-[11px] text-ink-400 font-mono shrink-0">
                <span>预估</span>
                <input 
                  v-model.number="vol.estimated_chapters" 
                  type="number" 
                  placeholder="30"
                  class="w-12 bg-atelier-900 border border-atelier-750 rounded px-1 py-0.5 text-center text-xs text-ink-200">
                <span>章</span>
              </div>
            </div>

            <!-- 起止章范围与状态锁 -->
            <div class="flex items-center justify-between gap-2 p-1.5 bg-atelier-900/60 border border-atelier-800 rounded text-[11px]">
              <div class="flex items-center gap-1 text-ink-300 font-mono">
                <span class="text-ink-400">起止章：</span>
                <input 
                  v-model.number="vol.start_chapter" 
                  type="number" 
                  min="1"
                  :disabled="vol.status === 'LOCKED'"
                  placeholder="1"
                  class="w-12 bg-atelier-950 border border-atelier-750 rounded px-1.5 py-0.5 text-center text-xs text-ink-100 font-mono focus:outline-none focus:border-brand-amber/60 disabled:opacity-50 disabled:cursor-not-allowed">
                <span class="text-ink-500">~</span>
                <input 
                  v-model.number="vol.end_chapter" 
                  type="number" 
                  min="1"
                  :disabled="vol.status === 'LOCKED'"
                  placeholder="40"
                  class="w-12 bg-atelier-950 border border-atelier-750 rounded px-1.5 py-0.5 text-center text-xs text-ink-100 font-mono focus:outline-none focus:border-brand-amber/60 disabled:opacity-50 disabled:cursor-not-allowed">
                <span class="text-ink-400">章</span>
              </div>

              <div class="flex items-center gap-1.5">
                <button
                  type="button"
                  @click="vol.status = (vol.status === 'LOCKED' ? 'DRAFT' : 'LOCKED')"
                  class="flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-mono transition cursor-pointer border"
                  :class="vol.status === 'LOCKED' ? 'bg-amber-500/15 text-brand-amber border-brand-amber/40 font-bold' : 'bg-atelier-950 text-ink-400 border-atelier-750 hover:text-ink-200'"
                  :title="vol.status === 'LOCKED' ? '已锁定起止章，点击解锁' : '未锁定，点击锁定起止章'">
                  <Lock v-if="vol.status === 'LOCKED'" class="w-3 h-3 text-brand-amber" />
                  <Unlock v-else class="w-3 h-3 text-ink-400" />
                  <span>{{ vol.status === 'LOCKED' ? '已锁定' : '未锁定' }}</span>
                </button>
                <select 
                  v-model="vol.status" 
                  class="bg-atelier-950 border border-atelier-750 rounded px-1 py-0.5 text-[10px] text-ink-300 font-mono focus:outline-none focus:border-brand-amber/60">
                  <option value="DRAFT">草稿</option>
                  <option value="LOCKED">锁定</option>
                  <option value="DONE">完卷</option>
                </select>
              </div>
            </div>

            <div>
              <label class="text-[10px] text-ink-400">本卷核心立意：</label>
              <input 
                v-model="vol.theme" 
                class="w-full mt-0.5 bg-atelier-900 border border-atelier-750 rounded px-2 py-1 text-xs text-ink-200 focus:outline-none focus:border-brand-amber/60" 
                placeholder="例如：微末崛起，潜龙勿用">
            </div>

            <div>
              <label class="text-[10px] text-ink-400">本卷核心目标：</label>
              <textarea 
                v-model="vol.core_goal" 
                rows="2" 
                class="w-full mt-0.5 bg-atelier-900 border border-atelier-750 rounded p-1.5 text-xs text-ink-200 focus:outline-none focus:border-brand-amber/60 resize-none font-serif"></textarea>
            </div>
            <div>
              <label class="text-[10px] text-brand-amber">卷终高潮爆发点：</label>
              <textarea 
                v-model="vol.climax" 
                rows="2" 
                class="w-full mt-0.5 bg-atelier-900 border border-brand-amber/30 rounded p-1.5 text-xs text-brand-amber focus:outline-none focus:border-brand-amber resize-none font-serif"></textarea>
            </div>
          </div>
        </div>
      </div>

      <!-- 势力阵营 (属于设定集实体) -->
      <div class="p-4 bg-atelier-900 border border-atelier-750 rounded-xl space-y-3 shadow-atelier-sm">
        <div class="flex items-center justify-between">
          <div>
            <div class="flex items-center gap-2 text-xs font-serif font-bold text-brand-amber">
              <Swords class="w-3.5 h-3.5" />
              <span>势力阵营</span>
            </div>
            <p class="text-[11px] text-ink-400 mt-0.5">各大组织派系、阵营立场与独门手段（同步收录于设定集）。</p>
          </div>
          <div class="flex items-center gap-2">
            <button 
              @click="addFrameworkFaction" 
              class="text-xs text-brand-amber hover:text-brand-amber-hover font-medium flex items-center gap-1 cursor-pointer">
              <Plus class="w-3 h-3" />
              <span>添加势力</span>
            </button>
          </div>
        </div>

        <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div 
            v-for="(fac, fIdx) in factionEntries" 
            :key="fac.id || fIdx" 
            class="p-4 bg-atelier-950 border border-atelier-750 rounded-lg space-y-2 relative">
            <button 
              @click="removeFrameworkFaction(fac)" 
              class="absolute top-2.5 right-2.5 text-ink-500 hover:text-rose-400 text-xs transition cursor-pointer">
              ✕
            </button>
            <div class="flex items-center gap-2">
              <input 
                v-model="fac.name" 
                class="flex-1 bg-atelier-900 border border-atelier-750 rounded px-2 py-1 text-xs text-ink-100 font-bold focus:outline-none focus:border-brand-amber/60" 
                placeholder="势力名称">
              <input 
                v-model="fac.alignment" 
                class="w-28 bg-atelier-900 border border-atelier-750 rounded px-2 py-1 text-xs text-brand-amber font-mono text-center focus:outline-none focus:border-brand-amber/60" 
                placeholder="阵营立场">
            </div>
            <div>
              <label class="text-[10px] text-ink-400">核心主张与独门手段：</label>
              <input 
                v-model="fac.details_markdown" 
                class="w-full mt-0.5 bg-atelier-900 border border-atelier-750 rounded px-2 py-1 text-xs text-ink-200 focus:outline-none focus:border-brand-amber/60"
                placeholder="宗门教义、修行功法或组织运作手段">
            </div>
            <div>
              <label class="text-[10px] text-rose-400">威胁级别与宿怨：</label>
              <textarea 
                v-model="fac.threat_level" 
                rows="2" 
                placeholder="例如：极高，与主角家族有血海深仇"
                class="w-full mt-0.5 bg-atelier-900 border border-rose-500/30 rounded p-1.5 text-xs text-rose-200 focus:outline-none focus:border-rose-400/60 resize-none font-serif"></textarea>
            </div>
          </div>
        </div>
      </div>

      <!-- 危险区域 (Danger Zone) -->
      <div class="p-5 bg-rose-500/5 border border-rose-500/25 rounded-xl space-y-3 shadow-atelier-sm">
        <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
          <div class="space-y-1">
            <div class="flex items-center gap-2 text-xs font-serif font-bold text-rose-400">
              <AlertTriangle class="w-4 h-4 text-rose-400" />
              <span>危险区域 · 删除当前作品</span>
            </div>
            <p class="text-[11px] text-ink-400">
              彻底删除作品《{{ state.currentProject.title }}》及其所有已定稿章节、在途手稿、大纲设定、伏笔簿与演进记录。此操作不可撤回。
            </p>
          </div>
          <button 
            @click="handleDeleteCurrentProject" 
            :disabled="state.isLoading"
            class="px-3.5 py-2 bg-rose-500/15 hover:bg-rose-500 text-rose-300 hover:text-white border border-rose-500/30 font-bold text-xs rounded-lg transition cursor-pointer flex items-center gap-1.5 self-start sm:self-auto shrink-0 disabled:opacity-50">
            <Trash2 class="w-3.5 h-3.5" />
            <span>彻底删除此作品</span>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, watch } from 'vue';
import { state, computedState, actions, notify } from '../stores/appState';
import { 
  Compass, 
  Sparkles, 
  Save, 
  Lightbulb, 
  User, 
  Scale, 
  Layers, 
  ScrollText, 
  Swords, 
  Plus,
  Sliders,
  Loader2,
  Lock,
  Unlock,
  AlertTriangle,
  Trash2
} from 'lucide-vue-next';

async function handleDeleteCurrentProject() {
  if (!state.currentProject) return;
  await actions.deleteProject(state.currentProject.id);
}

function normalizeFramework() {
  const fw = state.currentProject?.framework;
  if (!fw) return;
  // 核心立意规范
  if (!fw.theme_premise && fw.core_concept) {
    fw.theme_premise = fw.core_concept;
  }
  // 战力阶梯规范
  if (Array.isArray(fw.power_ladder)) {
    fw.power_ladder.forEach((t, i) => {
      if (!t.tier) t.tier = i + 1;
      if (!t.realm && t.tier_name) t.realm = t.tier_name;
      if (!t.description && t.features) t.description = t.features;
    });
  }
  // 分卷规划规范
  if (Array.isArray(fw.volume_arcs)) {
    let acc = 0;
    fw.volume_arcs.forEach((v, i) => {
      if (!v.volume_index && v.volume) v.volume_index = v.volume;
      if (!v.volume_index) v.volume_index = i + 1;
      if (!v.status) v.status = 'DRAFT';
      if (!v.climax && v.climax_event) v.climax = v.climax_event;
      if (!v.estimated_chapters) {
        if (v.end_chapter && v.start_chapter && v.end_chapter >= v.start_chapter) {
          v.estimated_chapters = v.end_chapter - v.start_chapter + 1;
        } else {
          v.estimated_chapters = 30;
        }
      }
      if (!v.start_chapter || !v.end_chapter || v.start_chapter > v.end_chapter) {
        v.start_chapter = acc + 1;
        v.end_chapter = acc + (v.estimated_chapters || 30);
      }
      acc = v.end_chapter;
    });
  }
}

watch(() => state.currentProject?.framework, () => {
  normalizeFramework();
}, { deep: true, immediate: true });

onMounted(() => {
  normalizeFramework();
});

function setProjectWordsTarget(target) {
  if (!state.currentProject) return;
  state.currentProject.default_words_target = target;
  state.wordsTarget = target;
}

function setProjectNarrativeStyle(style) {
  if (!state.currentProject) return;
  state.currentProject.default_narrative_style = style;
  state.narrativeStyle = style;
}

// 母题清单 ⇄ 多行文本（每行一条；保存时由后端 SanitizeMotifs 清洗并校验上限）
const motifsText = computed({
  get: () => (state.currentProject?.framework?.motifs || []).join('\n'),
  set: (value) => {
    if (!state.currentProject?.framework) return;
    state.currentProject.framework.motifs = String(value || '')
      .split('\n')
      .map((s) => s.trim())
      .filter(Boolean);
  },
});

function addFrameworkAxiom() {
  if (!state.currentProject?.framework) return;
  state.currentProject.framework.world_axioms = state.currentProject.framework.world_axioms || [];
  state.currentProject.framework.world_axioms.push('');
}

function removeFrameworkAxiom(idx) {
  state.currentProject.framework.world_axioms.splice(idx, 1);
}

function addFrameworkTier() {
  if (!state.currentProject?.framework) return;
  state.currentProject.framework.power_ladder = state.currentProject.framework.power_ladder || [];
  const nextTier = state.currentProject.framework.power_ladder.length + 1;
  state.currentProject.framework.power_ladder.push({
    tier: nextTier,
    realm: '',
    description: '',
    bottleneck: '',
    drawback: '',
  });
}

function removeFrameworkTier(idx) {
  state.currentProject.framework.power_ladder.splice(idx, 1);
}

function addFrameworkVolume() {
  if (!state.currentProject?.framework) return;
  state.currentProject.framework.volume_arcs = state.currentProject.framework.volume_arcs || [];
  const arcs = state.currentProject.framework.volume_arcs;
  const nextVol = arcs.length + 1;
  const lastVol = arcs.length > 0 ? arcs[arcs.length - 1] : null;
  const startCh = lastVol && lastVol.end_chapter ? lastVol.end_chapter + 1 : 1;
  const est = 30;
  arcs.push({
    volume_index: nextVol,
    title: `第 ${nextVol} 卷`,
    theme: '',
    start_chapter: startCh,
    end_chapter: startCh + est - 1,
    status: 'DRAFT',
    estimated_chapters: est,
    core_goal: '',
    climax: '',
  });
}

function removeFrameworkVolume(idx) {
  state.currentProject.framework.volume_arcs.splice(idx, 1);
}

function addFrameworkMilestone() {
  if (!state.currentProject?.framework) return;
  state.currentProject.framework.story_milestones = state.currentProject.framework.story_milestones || [];
  const milestones = state.currentProject.framework.story_milestones;
  const existingKeys = new Set(milestones.map(m => m.key));
  let counter = milestones.length + 1;
  let customKey = `CUSTOM_${counter}`;
  while (existingKeys.has(customKey)) {
    counter++;
    customKey = `CUSTOM_${counter}`;
  }
  milestones.push({
    key: customKey,
    name: '',
    target_chapter: 1,
    status: 'PENDING',
    note: '',
  });
}

function removeFrameworkMilestone(idx) {
  if (!state.currentProject?.framework?.story_milestones) return;
  state.currentProject.framework.story_milestones.splice(idx, 1);
}

function toggleFrameworkMilestoneStatus(ms) {
  if (!ms) return;
  ms.status = ms.status === 'DONE' ? 'PENDING' : 'DONE';
}

function handleSaveFramework() {
  if (state.currentProject?.framework) {
    state.currentProject.framework.story_milestones = state.currentProject.framework.story_milestones || [];
  }
  actions.saveFramework();
}

const factionEntries = computed(() => {
  return (state.codexEntries || []).filter(e => e.category === 'FACTION');
});

async function addFrameworkFaction() {
  if (!state.currentProject) return;
  await actions.createCodexEntry({
    project_id: state.currentProject.id,
    category: 'FACTION',
    name: '新建宗门/势力',
    alignment: '中立',
    threat_level: '中等',
    summary: '立场: 中立 | 威胁度: 中等',
    details_markdown: '',
    tracking_mode: 'AUTO_MENTION',
  });
}

async function removeFrameworkFaction(fac) {
  if (!fac?.id) return;
  await actions.deleteCodexEntry(fac.id);
}
</script>
