<template>
  <div class="flex-1 p-6 overflow-y-auto space-y-6" v-if="state.currentProject">
    <!-- 头部工具栏 -->
    <div class="flex flex-col md:flex-row md:items-center justify-between gap-4 border-b border-atelier-750 pb-5">
      <div>
        <div class="flex items-center gap-2.5">
          <div class="w-8 h-8 rounded-lg bg-brand-amber/10 border border-brand-amber/25 flex items-center justify-center text-brand-amber shadow-amber-glow">
            <Compass class="w-4 h-4" />
          </div>
          <div>
            <h2 class="text-base font-serif font-bold text-ink-50 tracking-wide">宏观创世总纲 (Genesis Framework Bible)</h2>
            <p class="text-xs text-ink-400 mt-0.5">万字长篇宏观骨架：天道法则、严谨战力阶梯、分卷任务链与暗线势力谱系</p>
          </div>
        </div>
      </div>

      <div class="flex items-center gap-2.5">
        <button 
          @click="actions.bootstrapCurrentFramework()" 
          :disabled="state.isLoading" 
          class="flex items-center gap-1.5 px-3 py-1.5 bg-atelier-850 hover:bg-atelier-800 text-brand-amber text-xs font-semibold rounded-md border border-atelier-750 transition cursor-pointer disabled:opacity-50">
          <Sparkles class="w-3.5 h-3.5 text-brand-amber" />
          <span>{{ state.isLoading ? 'AI 宏观推演中...' : '重新 AI 创世推演' }}</span>
        </button>
        <button 
          @click="actions.saveFramework" 
          class="flex items-center gap-1.5 px-3.5 py-1.5 bg-brand-amber hover:bg-brand-amber-hover text-atelier-950 font-bold text-xs rounded-md shadow-amber-glow transition cursor-pointer">
          <Save class="w-3.5 h-3.5" />
          <span>保存总纲修改</span>
        </button>
      </div>
    </div>

    <!-- 空白状态 -->
    <div 
      v-if="!state.currentProject.framework" 
      class="p-12 text-center bg-atelier-900 border border-dashed border-atelier-750 rounded-xl space-y-4 shadow-atelier-md">
      <div class="w-14 h-14 rounded-full bg-brand-amber/10 border border-brand-amber/25 flex items-center justify-center mx-auto text-brand-amber">
        <Compass class="w-7 h-7" />
      </div>
      <div class="space-y-1">
        <h3 class="text-base font-serif font-bold text-ink-100">本作品尚未构建宏观创世总纲</h3>
        <p class="text-xs text-ink-400 max-w-md mx-auto leading-relaxed">
          缺乏宏观总纲的长篇极易中途战力崩坏、主线迷失。启动 AI 宏观推演，自动建构天道物理公理、10 级代价晋升天梯、分卷大纲与对抗势力。
        </p>
      </div>
      <button 
        @click="actions.bootstrapCurrentFramework()" 
        class="px-5 py-2.5 bg-brand-amber hover:bg-brand-amber-hover text-atelier-950 font-bold text-xs rounded-lg shadow-amber-glow transition cursor-pointer inline-flex items-center gap-2">
        <Sparkles class="w-4 h-4" />
        <span>立即启动 AI 宏观创世推演</span>
      </button>
    </div>

    <!-- 总纲表单区 -->
    <div v-else class="space-y-6">
      <!-- 1. 核心高概念与主角创世档案 -->
      <div class="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <!-- 核心高概念 -->
        <div class="p-4 bg-atelier-900 border border-atelier-750 rounded-xl space-y-2 lg:col-span-1 shadow-atelier-sm">
          <div class="flex items-center gap-2 text-xs font-serif font-bold text-brand-amber">
            <Lightbulb class="w-3.5 h-3.5" />
            <span>核心高概念 (Core Concept)</span>
          </div>
          <textarea 
            v-model="state.currentProject.framework.core_concept" 
            rows="6" 
            class="w-full bg-atelier-950 border border-atelier-750 rounded-lg p-2.5 text-xs text-ink-100 font-serif focus:outline-none focus:border-brand-amber/60 leading-relaxed resize-none" 
            placeholder="输入全书核心设定与立意..."></textarea>
        </div>

        <!-- 主角创世建档 -->
        <div 
          v-if="state.currentProject.framework.protagonist" 
          class="p-4 bg-atelier-900 border border-atelier-750 rounded-xl space-y-3 lg:col-span-2 shadow-atelier-sm">
          <div class="flex items-center gap-2 text-xs font-serif font-bold text-brand-amber">
            <User class="w-3.5 h-3.5" />
            <span>主角创世档案 (Protagonist Genesis Profile)</span>
          </div>
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-3 text-xs">
            <div>
              <label class="text-ink-400 text-[11px] font-medium">主角姓名：</label>
              <input 
                v-model="state.currentProject.framework.protagonist.name" 
                class="w-full mt-1 bg-atelier-950 border border-atelier-750 rounded-md px-2.5 py-1.5 text-xs text-ink-50 font-bold focus:outline-none focus:border-brand-amber/60">
            </div>
            <div>
              <label class="text-ink-400 text-[11px] font-medium">初始境界阶梯：</label>
              <input 
                v-model="state.currentProject.framework.protagonist.initial_realm" 
                class="w-full mt-1 bg-atelier-950 border border-atelier-750 rounded-md px-2.5 py-1.5 text-xs text-brand-amber font-mono focus:outline-none focus:border-brand-amber/60">
            </div>
            <div>
              <label class="text-ink-400 text-[11px] font-medium">核心内在驱动力 (Core Drive)：</label>
              <input 
                v-model="state.currentProject.framework.protagonist.core_drive" 
                class="w-full mt-1 bg-atelier-950 border border-atelier-750 rounded-md px-2.5 py-1.5 text-xs text-ink-200 focus:outline-none focus:border-brand-amber/60">
            </div>
            <div>
              <label class="text-ink-400 text-[11px] font-medium">致命缺陷 / 逆鳞 (Fatal Flaw)：</label>
              <input 
                v-model="state.currentProject.framework.protagonist.fatal_flaw" 
                class="w-full mt-1 bg-atelier-950 border border-atelier-750 rounded-md px-2.5 py-1.5 text-xs text-rose-300 focus:outline-none focus:border-brand-amber/60">
            </div>
            <div class="sm:col-span-2">
              <label class="text-ink-400 text-[11px] font-medium">金手指 / 弑神契机 (Special Trait / Hook)：</label>
              <input 
                v-model="state.currentProject.framework.protagonist.special_trait" 
                class="w-full mt-1 bg-atelier-950 border border-atelier-750 rounded-md px-2.5 py-1.5 text-xs text-brand-amber font-mono focus:outline-none focus:border-brand-amber/60">
            </div>
          </div>
        </div>
      </div>

      <!-- 2. 天道法则与世界公理 (World Axioms) -->
      <div class="p-4 bg-atelier-900 border border-atelier-750 rounded-xl space-y-3 shadow-atelier-sm">
        <div class="flex items-center justify-between">
          <div>
            <div class="flex items-center gap-2 text-xs font-serif font-bold text-brand-amber">
              <Scale class="w-3.5 h-3.5" />
              <span>天道法则与世界公理 (World Axioms - 物理铁律，绝对不可违背)</span>
            </div>
            <p class="text-[11px] text-ink-400 mt-0.5">推演节拍和渲染章节时强制作为公理约束，杜绝战力崩塌与违背常识。</p>
          </div>
          <button 
            @click="addFrameworkAxiom" 
            class="text-xs text-brand-amber hover:text-brand-amber-hover font-medium flex items-center gap-1 cursor-pointer">
            <Plus class="w-3 h-3" />
            <span>添加天道公理</span>
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

      <!-- 3. 严谨战力阶梯与代价天平 (Power Ladder) -->
      <div class="p-4 bg-atelier-900 border border-atelier-750 rounded-xl space-y-3 shadow-atelier-sm">
        <div class="flex items-center justify-between">
          <div>
            <div class="flex items-center gap-2 text-xs font-serif font-bold text-brand-amber">
              <Layers class="w-3.5 h-3.5" />
              <span>战力晋升阶梯与代价天平 (Power Ladder - 杜绝战力膨胀与无脑越级)</span>
            </div>
            <p class="text-[11px] text-ink-400 mt-0.5">每阶具备破坏力表征、突破瓶颈与不可逆的代价反噬，形成严谨的力量闭环。</p>
          </div>
          <button 
            @click="addFrameworkTier" 
            class="text-xs text-brand-amber hover:text-brand-amber-hover font-medium flex items-center gap-1 cursor-pointer">
            <Plus class="w-3 h-3" />
            <span>添加境界阶梯</span>
          </button>
        </div>

        <div class="overflow-x-auto">
          <table class="w-full text-left text-xs border-collapse">
            <thead>
              <tr class="border-b border-atelier-750 text-ink-400 text-[11px]">
                <th class="py-2.5 px-2 w-12 font-mono text-center">阶</th>
                <th class="py-2.5 px-3 w-36">境界名称</th>
                <th class="py-2.5 px-3">破坏力与感知表征</th>
                <th class="py-2.5 px-3">突破瓶颈 (机缘/顿悟)</th>
                <th class="py-2.5 px-3">道途代价 / 命格反噬</th>
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

      <!-- 4. 分卷大纲与主线任务链 (Volume Arcs) -->
      <div class="p-4 bg-atelier-900 border border-atelier-750 rounded-xl space-y-3 shadow-atelier-sm">
        <div class="flex items-center justify-between">
          <div>
            <div class="flex items-center gap-2 text-xs font-serif font-bold text-brand-amber">
              <ScrollText class="w-3.5 h-3.5" />
              <span>分卷大纲与主线任务链 (Volume Arcs - 长篇宏观节奏控制器)</span>
            </div>
            <p class="text-[11px] text-ink-400 mt-0.5">每卷设定明确章节跨度、阶段使命与终局高潮，生成章节时自动映射本卷终局目标。</p>
          </div>
          <button 
            @click="addFrameworkVolume" 
            class="text-xs text-brand-amber hover:text-brand-amber-hover font-medium flex items-center gap-1 cursor-pointer">
            <Plus class="w-3 h-3" />
            <span>添加分卷规划</span>
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
              <label class="text-[10px] text-ink-400">本卷核心使命 / 主线破局点：</label>
              <textarea 
                v-model="vol.core_goal" 
                rows="2" 
                class="w-full mt-0.5 bg-atelier-900 border border-atelier-750 rounded p-1.5 text-xs text-ink-200 focus:outline-none focus:border-brand-amber/60 resize-none font-serif"></textarea>
            </div>
            <div>
              <label class="text-[10px] text-brand-amber">卷终高潮爆发点 (Climax Event)：</label>
              <textarea 
                v-model="vol.climax_event" 
                rows="2" 
                class="w-full mt-0.5 bg-atelier-900 border border-brand-amber/30 rounded p-1.5 text-xs text-brand-amber focus:outline-none focus:border-brand-amber resize-none font-serif"></textarea>
            </div>
          </div>
        </div>
      </div>

      <!-- 5. 势力谱系与利益冲突暗线 (Factions & Agendas) -->
      <div class="p-4 bg-atelier-900 border border-atelier-750 rounded-xl space-y-3 shadow-atelier-sm">
        <div class="flex items-center justify-between">
          <div>
            <div class="flex items-center gap-2 text-xs font-serif font-bold text-brand-amber">
              <Swords class="w-3.5 h-3.5" />
              <span>势力谱系与利益冲突暗线 (Factions & Secret Agendas)</span>
            </div>
            <p class="text-[11px] text-ink-400 mt-0.5">多方势力各自为战并具备隐秘诉求，杜绝脸谱化降智反派。</p>
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
              <label class="text-[10px] text-rose-400">核心诉求与暗线阴谋 (Secret Agenda)：</label>
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
  Plus 
} from 'lucide-vue-next';

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
