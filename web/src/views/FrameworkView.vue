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
            rows="6" 
            class="w-full bg-atelier-950 border border-atelier-750 rounded-lg p-2.5 text-xs text-ink-100 font-serif focus:outline-none focus:border-brand-amber/60 leading-relaxed resize-none" 
            placeholder="输入全书核心设定与立意..."></textarea>
        </div>

        <!-- 主角档案 -->
        <div 
          v-if="state.currentProject.protagonist" 
          class="p-4 bg-atelier-900 border border-atelier-750 rounded-xl space-y-2.5 lg:col-span-2 shadow-atelier-sm">
          <div class="flex items-center gap-2 text-xs font-serif font-bold text-brand-amber">
            <User class="w-3.5 h-3.5" />
            <span>主角档案</span>
          </div>
          <p class="text-[10px] text-ink-400">主角姓名、当前境界、核心目标与健康状态。</p>
          
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-3 text-xs pt-1">
            <div>
              <label class="text-ink-400 text-[11px] font-medium">姓名与境界：</label>
              <input 
                v-model="state.currentProject.protagonist.name_and_level" 
                class="w-full mt-1 bg-atelier-950 border border-atelier-750 rounded-md px-2.5 py-1.5 text-xs text-brand-amber font-mono font-bold focus:outline-none focus:border-brand-amber/60">
            </div>
            <div>
              <label class="text-ink-400 text-[11px] font-medium">战备/健康状态：</label>
              <input 
                v-model="state.currentProject.protagonist.health_status" 
                class="w-full mt-1 bg-atelier-950 border border-atelier-750 rounded-md px-2.5 py-1.5 text-xs text-emerald-400 font-mono focus:outline-none focus:border-brand-amber/60">
            </div>
            <div class="sm:col-span-2">
              <label class="text-ink-400 text-[11px] font-medium">当前主线目标与行事底线：</label>
              <input 
                v-model="state.currentProject.protagonist.core_goal" 
                class="w-full mt-1 bg-atelier-950 border border-atelier-750 rounded-md px-2.5 py-1.5 text-xs text-ink-200 focus:outline-none focus:border-brand-amber/60">
            </div>
            <div class="sm:col-span-2">
              <label class="text-ink-400 text-[11px] font-medium">随身携带道具 (Inventory)：</label>
              <textarea 
                v-model="state.currentProject.protagonist.inventory" 
                rows="2"
                class="w-full mt-1 bg-atelier-950 border border-atelier-750 rounded-md p-2 text-xs text-ink-300 font-mono focus:outline-none focus:border-brand-amber/60 resize-none leading-relaxed"></textarea>
            </div>
          </div>
        </div>
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
              @click="syncFactionsToCodex" 
              class="text-xs text-brand-amber hover:text-brand-amber-hover font-medium flex items-center gap-1 cursor-pointer bg-brand-amber/10 px-2 py-1 rounded border border-brand-amber/30"
              title="将所有势力一键同步至设定集与关系图谱">
              <Share2 class="w-3 h-3" />
              <span>同步至设定集</span>
            </button>
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
                v-model="fac.alignment" 
                class="w-28 bg-atelier-900 border border-atelier-750 rounded px-2 py-1 text-xs text-brand-amber font-mono text-center focus:outline-none focus:border-brand-amber/60" 
                placeholder="阵营立场">
            </div>
            <div>
              <label class="text-[10px] text-ink-400">核心主张与独门手段：</label>
              <input 
                v-model="fac.doctrine" 
                class="w-full mt-0.5 bg-atelier-900 border border-atelier-750 rounded px-2 py-1 text-xs text-ink-200 focus:outline-none focus:border-brand-amber/60">
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
    </div>
  </div>
</template>

<script setup>
import { onMounted, watch } from 'vue';
import { state, actions, notify } from '../stores/appState';
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
  Share2
} from 'lucide-vue-next';

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
    fw.volume_arcs.forEach((v, i) => {
      if (!v.volume_index && v.volume) v.volume_index = v.volume;
      if (!v.volume_index) v.volume_index = i + 1;
      if (!v.climax && v.climax_event) v.climax = v.climax_event;
      if (!v.estimated_chapters && v.end_chapter && v.start_chapter) {
        v.estimated_chapters = v.end_chapter - v.start_chapter + 1;
      }
    });
  }
  // 势力阵营规范
  if (Array.isArray(fw.factions)) {
    fw.factions.forEach(f => {
      if (!f.alignment && f.stance) f.alignment = f.stance;
      if (!f.doctrine && f.power_base) f.doctrine = f.power_base;
      if (!f.threat_level && f.secret_agenda) f.threat_level = f.secret_agenda;
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
  const nextVol = state.currentProject.framework.volume_arcs.length + 1;
  state.currentProject.framework.volume_arcs.push({
    volume_index: nextVol,
    title: `第 ${nextVol} 卷`,
    theme: '',
    estimated_chapters: 30,
    core_goal: '',
    climax: '',
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
    alignment: '中立',
    doctrine: '',
    threat_level: '中等',
  });
}

function removeFrameworkFaction(idx) {
  state.currentProject.framework.factions.splice(idx, 1);
}

async function syncFactionsToCodex() {
  const fw = state.currentProject?.framework;
  if (!fw || !Array.isArray(fw.factions) || fw.factions.length === 0) {
    notify('暂无势力', '请先添加或推演势力阵营', 'info');
    return;
  }
  let addedCount = 0;
  for (const fac of fw.factions) {
    if (!fac.name.trim()) continue;
    const exists = (state.codexEntries || []).some(e => e.name === fac.name.trim());
    if (!exists) {
      await actions.createCodexEntry?.({
        name: fac.name.trim(),
        category: 'FACTION',
        summary: `立场: ${fac.alignment || '中立'} | 威胁: ${fac.threat_level || '中等'}`,
        details_markdown: fac.doctrine || '',
        tracking_mode: 'AUTO_MENTION',
      });
      addedCount++;
    }
  }
  await actions.loadCodexEntries?.();
  notify('同步完成', `已将 ${addedCount} 个新势力同步至设定集与图谱`, 'success');
}
</script>
