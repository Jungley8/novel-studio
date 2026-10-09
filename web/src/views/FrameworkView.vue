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
          @click="actions.saveFramework" 
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
          先立规矩，小说才不容易卡文或战力崩溃。AI 将为你一键构建天道法则、战力阶梯、分卷规划与势力阵营。
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
            v-model="state.currentProject.framework.core_concept" 
            rows="6" 
            class="w-full bg-atelier-950 border border-atelier-750 rounded-lg p-2.5 text-xs text-ink-100 font-serif focus:outline-none focus:border-brand-amber/60 leading-relaxed resize-none" 
            placeholder="输入全书核心设定与立意..."></textarea>
        </div>

        <!-- 主角档案 -->
        <div 
          v-if="state.currentProject.framework.protagonist" 
          class="p-4 bg-atelier-900 border border-atelier-750 rounded-xl space-y-2.5 lg:col-span-2 shadow-atelier-sm">
          <div class="flex items-center gap-2 text-xs font-serif font-bold text-brand-amber">
            <User class="w-3.5 h-3.5" />
            <span>主角档案</span>
          </div>
          <p class="text-[10px] text-ink-400">主角姓名、初始境界、内在动机与致命缺陷。</p>
          
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-3 text-xs pt-1">
            <div>
              <label class="text-ink-400 text-[11px] font-medium">主角姓名：</label>
              <input 
                v-model="state.currentProject.framework.protagonist.name" 
                class="w-full mt-1 bg-atelier-950 border border-atelier-750 rounded-md px-2.5 py-1.5 text-xs text-ink-50 font-bold focus:outline-none focus:border-brand-amber/60">
            </div>
            <div>
              <label class="text-ink-400 text-[11px] font-medium">初始境界：</label>
              <input 
                v-model="state.currentProject.framework.protagonist.initial_realm" 
                class="w-full mt-1 bg-atelier-950 border border-atelier-750 rounded-md px-2.5 py-1.5 text-xs text-brand-amber font-mono focus:outline-none focus:border-brand-amber/60">
            </div>
            <div>
              <label class="text-ink-400 text-[11px] font-medium">内在动机：</label>
              <input 
                v-model="state.currentProject.framework.protagonist.core_drive" 
                class="w-full mt-1 bg-atelier-950 border border-atelier-750 rounded-md px-2.5 py-1.5 text-xs text-ink-200 focus:outline-none focus:border-brand-amber/60">
            </div>
            <div>
              <label class="text-ink-400 text-[11px] font-medium">致命缺陷：</label>
              <input 
                v-model="state.currentProject.framework.protagonist.fatal_flaw" 
                class="w-full mt-1 bg-atelier-950 border border-atelier-750 rounded-md px-2.5 py-1.5 text-xs text-rose-300 focus:outline-none focus:border-brand-amber/60">
            </div>
            <div class="sm:col-span-2">
              <label class="text-ink-400 text-[11px] font-medium">核心金手指：</label>
              <input 
                v-model="state.currentProject.framework.protagonist.special_trait" 
                class="w-full mt-1 bg-atelier-950 border border-atelier-750 rounded-md px-2.5 py-1.5 text-xs text-brand-amber font-mono focus:outline-none focus:border-brand-amber/60">
            </div>
          </div>
        </div>
      </div>

      <!-- 天道法则 -->
      <div class="p-4 bg-atelier-900 border border-atelier-750 rounded-xl space-y-3 shadow-atelier-sm">
        <div class="flex items-center justify-between">
          <div>
            <div class="flex items-center gap-2 text-xs font-serif font-bold text-brand-amber">
              <Scale class="w-3.5 h-3.5" />
              <span>天道法则</span>
            </div>
            <p class="text-[11px] text-ink-400 mt-0.5">世界不可违背的底层物理与因果铁律，杜绝逻辑崩塌。</p>
          </div>
          <button 
            @click="addFrameworkAxiom" 
            class="text-xs text-brand-amber hover:text-brand-amber-hover font-medium flex items-center gap-1 cursor-pointer">
            <Plus class="w-3 h-3" />
            <span>添加法则</span>
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
            <p class="text-[11px] text-ink-400 mt-0.5">每层境界的破坏力、突破瓶颈与反噬代价。</p>
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
                <th class="py-2.5 px-3">反噬代价</th>
                <th class="py-2.5 px-2 w-10 text-center">操作</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-atelier-800 font-sans">
              <tr 
                v-for="(tier, tIdx) in state.currentProject.framework.power_ladder" 
                :key="tIdx" 
                class="hover:bg-atelier-850/50">
                <td class="py-2 px-2 font-mono text-brand-amber font-bold text-center">{{ tier.tier }}</td>
                <td class="py-2 px-3">
                  <input 
                    v-model="tier.tier_name" 
                    class="w-full bg-atelier-950 border border-atelier-750 rounded px-2 py-1 text-xs text-purple-300 font-semibold focus:outline-none focus:border-brand-amber/60">
                </td>
                <td class="py-2 px-3">
                  <input 
                    v-model="tier.features" 
                    class="w-full bg-atelier-950 border border-atelier-750 rounded px-2 py-1 text-xs text-ink-200 focus:outline-none focus:border-brand-amber/60">
                </td>
                <td class="py-2 px-3">
                  <input 
                    v-model="tier.bottleneck" 
                    class="w-full bg-atelier-950 border border-atelier-750 rounded px-2 py-1 text-xs text-ink-300 focus:outline-none focus:border-brand-amber/60">
                </td>
                <td class="py-2 px-3">
                  <input 
                    v-model="tier.drawback" 
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
              <span class="text-xs font-mono font-bold text-brand-amber">第 {{ vol.volume }} 卷</span>
              <input 
                v-model="vol.title" 
                class="flex-1 bg-atelier-900 border border-atelier-750 rounded px-2 py-1 text-xs text-ink-100 font-serif font-bold focus:outline-none focus:border-brand-amber/60" 
                placeholder="分卷标题">
              <div class="flex items-center gap-1 text-[11px] text-ink-400 font-mono">
                <span>第</span>
                <input 
                  v-model.number="vol.start_chapter" 
                  type="number" 
                  class="w-12 bg-atelier-900 border border-atelier-750 rounded px-1 py-0.5 text-center text-xs text-ink-200">
                <span>-</span>
                <input 
                  v-model.number="vol.end_chapter" 
                  type="number" 
                  class="w-12 bg-atelier-900 border border-atelier-750 rounded px-1 py-0.5 text-center text-xs text-ink-200">
                <span>章</span>
              </div>
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
                v-model="vol.climax_event" 
                rows="2" 
                class="w-full mt-0.5 bg-atelier-900 border border-brand-amber/30 rounded p-1.5 text-xs text-brand-amber focus:outline-none focus:border-brand-amber resize-none font-serif"></textarea>
            </div>
          </div>
        </div>
      </div>

      <!-- 势力阵营 -->
      <div class="p-4 bg-atelier-900 border border-atelier-750 rounded-xl space-y-3 shadow-atelier-sm">
        <div class="flex items-center justify-between">
          <div>
            <div class="flex items-center gap-2 text-xs font-serif font-bold text-brand-amber">
              <Swords class="w-3.5 h-3.5" />
              <span>势力阵营</span>
            </div>
            <p class="text-[11px] text-ink-400 mt-0.5">各大派系、立场冲突与暗线图谋。</p>
          </div>
          <button 
            @click="addFrameworkFaction" 
            class="text-xs text-brand-amber hover:text-brand-amber-hover font-medium flex items-center gap-1 cursor-pointer">
            <Plus class="w-3 h-3" />
            <span>添加势力</span>
          </button>
        </div>

        <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div 
            v-for="(fac, fIdx) in state.currentProject.framework.factions" 
            :key="fIdx" 
            class="p-4 bg-atelier-950 border border-atelier-750 rounded-lg space-y-2 relative">
            <button 
              @click="removeFrameworkFaction(fIdx)" 
              class="absolute top-2.5 right-2.5 text-ink-500 hover:text-rose-400 text-xs transition cursor-pointer">
              ✕
            </button>
            <div class="flex items-center gap-2">
              <input 
                v-model="fac.name" 
                class="flex-1 bg-atelier-900 border border-atelier-750 rounded px-2 py-1 text-xs text-ink-100 font-bold focus:outline-none focus:border-brand-amber/60" 
                placeholder="势力名称">
              <input 
                v-model="fac.stance" 
                class="w-24 bg-atelier-900 border border-atelier-750 rounded px-2 py-1 text-xs text-brand-amber font-mono text-center focus:outline-none focus:border-brand-amber/60" 
                placeholder="立场">
            </div>
            <div>
              <label class="text-[10px] text-ink-400">权力根基与底蕴：</label>
              <input 
                v-model="fac.power_base" 
                class="w-full mt-0.5 bg-atelier-900 border border-atelier-750 rounded px-2 py-1 text-xs text-ink-200 focus:outline-none focus:border-brand-amber/60">
            </div>
            <div>
              <label class="text-[10px] text-rose-400">暗线图谋：</label>
              <textarea 
                v-model="fac.secret_agenda" 
                rows="2" 
                class="w-full mt-0.5 bg-atelier-900 border border-atelier-750 rounded p-1.5 text-xs text-rose-200 focus:outline-none focus:border-rose-400/60 resize-none font-serif"></textarea>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { state, actions } from '../stores/appState';
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
  Loader2
} from 'lucide-vue-next';

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
    tier_name: '',
    features: '',
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
  const nextVol = state.currentProject.framework.volume_arcs.length + 1;
  state.currentProject.framework.volume_arcs.push({
    volume: nextVol,
    title: `第 ${nextVol} 卷`,
    start_chapter: (nextVol - 1) * 30 + 1,
    end_chapter: nextVol * 30,
    core_goal: '',
    climax_event: '',
  });
}

function removeFrameworkVolume(idx) {
  state.currentProject.framework.volume_arcs.splice(idx, 1);
}

function addFrameworkFaction() {
  if (!state.currentProject?.framework) return;
  state.currentProject.framework.factions = state.currentProject.framework.factions || [];
  state.currentProject.framework.factions.push({
    name: '',
    stance: '中立',
    power_base: '',
    secret_agenda: '',
  });
}

function removeFrameworkFaction(idx) {
  state.currentProject.framework.factions.splice(idx, 1);
}
</script>
