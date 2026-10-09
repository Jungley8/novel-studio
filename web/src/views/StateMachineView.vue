<template>
  <div class="flex-1 p-6 overflow-y-auto space-y-6" v-if="state.currentProject">
    <!-- 头部工具栏 -->
    <div class="flex flex-col md:flex-row md:items-center justify-between gap-4 border-b border-atelier-750 pb-5">
      <div>
        <div class="flex items-center gap-2.5">
          <div class="w-8 h-8 rounded-lg bg-brand-amber/10 border border-brand-amber/25 flex items-center justify-center text-brand-amber shadow-amber-glow">
            <Cpu class="w-4 h-4" />
          </div>
          <div>
            <h2 class="text-base font-serif font-bold text-ink-50 tracking-wide">人物状态</h2>
            <p class="text-xs text-ink-400 mt-0.5">记录主角当前境界、战力与随身物品。</p>
          </div>
        </div>
      </div>

      <button 
        @click="actions.saveCurrentProject" 
        class="flex items-center gap-1.5 px-3.5 py-1.5 bg-brand-amber hover:bg-brand-amber-hover text-atelier-950 font-bold text-xs rounded-md shadow-amber-glow transition cursor-pointer">
        <Save class="w-3.5 h-3.5" />
        <span>保存状态</span>
      </button>
    </div>

    <!-- 状态机两栏核心网格 -->
    <div class="grid grid-cols-1 lg:grid-cols-2 gap-6" v-if="state.currentProject.protagonist">
      <!-- 左栏：主角当前状态 -->
      <div class="p-5 bg-atelier-900 border border-atelier-750 rounded-xl space-y-4 shadow-atelier-sm">
        <div class="flex items-center gap-2 text-xs font-serif font-bold text-brand-amber border-b border-atelier-800 pb-2.5">
          <User class="w-4 h-4" />
          <span>主角状态</span>
        </div>

        <div class="space-y-3 text-xs">
          <div>
            <label class="text-ink-400 font-medium">姓名与境界 / 等级：</label>
            <input 
              v-model="state.currentProject.protagonist.name_and_level" 
              class="w-full mt-1.5 bg-atelier-950 border border-atelier-750 rounded-md px-3 py-1.5 text-xs text-brand-amber font-mono font-bold focus:outline-none focus:border-brand-amber/60">
          </div>

          <div>
            <label class="text-ink-400 font-medium">随身物品与道具 (Inventory)：</label>
            <textarea 
              v-model="state.currentProject.protagonist.inventory" 
              rows="4" 
              class="w-full mt-1.5 bg-atelier-950 border border-atelier-750 rounded-md p-2.5 text-xs text-ink-100 font-mono focus:outline-none focus:border-brand-amber/60 resize-none leading-relaxed" 
              placeholder="例如：洗髓丹x2, 残骨铁印x1, 玄铁匕首x1"></textarea>
          </div>

          <div>
            <label class="text-ink-400 font-medium">当前目标与行事底线 (Core Goal)：</label>
            <input 
              v-model="state.currentProject.protagonist.core_goal" 
              class="w-full mt-1.5 bg-atelier-950 border border-atelier-750 rounded-md px-3 py-1.5 text-xs text-ink-200 focus:outline-none focus:border-brand-amber/60">
          </div>
        </div>

        <!-- 结构化突破演进记录 (Level History) -->
        <div 
          v-if="state.currentProject.protagonist.structured_level?.history?.length > 0" 
          class="pt-3 border-t border-atelier-800 space-y-2">
          <span class="text-xs font-mono font-bold text-brand-amber flex items-center gap-1.5">
            <Zap class="w-3.5 h-3.5" />
            <span>境界突破演进历史 (Evolution Trace)：</span>
          </span>
          <div class="space-y-1.5 max-h-40 overflow-y-auto">
            <div 
              v-for="(trans, idx) in state.currentProject.protagonist.structured_level.history" 
              :key="idx" 
              class="text-[11px] flex items-center justify-between text-ink-200 font-mono bg-atelier-950 px-3 py-1.5 rounded-md border border-atelier-800">
              <span class="flex items-center gap-1.5">
                <span class="text-ink-400">{{ trans.from_realm || '初始' }}</span>
                <span class="text-brand-amber">➔</span>
                <strong class="text-emerald-400">{{ trans.to_realm }}</strong>
              </span>
              <span class="text-ink-500 text-[10px]">
                第 {{ trans.chapter }} 章 {{ trans.reason ? '(' + trans.reason + ')' : '' }}
              </span>
            </div>
          </div>
        </div>
      </div>

      <!-- 右栏：世界公理与绝对法则 (Rule Engine) -->
      <div class="p-5 bg-atelier-900 border border-atelier-750 rounded-xl space-y-3.5 shadow-atelier-sm">
        <div class="flex items-center gap-2 text-xs font-serif font-bold text-brand-amber border-b border-atelier-800 pb-2.5">
          <Scale class="w-4 h-4" />
          <span>世界底层物理公理 (Rule Engine)</span>
        </div>
        <p class="text-[11px] text-ink-400 leading-relaxed">
          模型在推演剧情与渲染正文时绝对不可违背的底层物理铁律与世界观约束。
        </p>
        <textarea 
          v-model="state.currentProject.world_rules" 
          rows="14" 
          class="w-full bg-atelier-950 border border-atelier-750 rounded-lg p-3 text-xs text-ink-100 font-mono focus:outline-none focus:border-brand-amber/60 leading-relaxed resize-none"></textarea>
      </div>
    </div>

    <!-- 群像多人物动态状态看板 (Cast Dynamic Character States) -->
    <div class="p-5 bg-atelier-900 border border-atelier-750 rounded-xl space-y-4 shadow-atelier-sm">
      <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 border-b border-atelier-800 pb-3.5">
        <div class="flex items-center gap-2.5">
          <div class="w-7 h-7 rounded-lg bg-brand-crimson/10 border border-brand-crimson/25 flex items-center justify-center text-brand-crimson">
            <Users class="w-4 h-4" />
          </div>
          <div>
            <div class="flex items-center gap-2">
              <h3 class="text-sm font-serif font-bold text-ink-50">全书群像多人物动态状态看板 (Cast Dynamic States)</h3>
              <span class="text-[10px] font-mono px-2 py-0.5 rounded-full bg-atelier-800 border border-atelier-700 text-brand-amber">
                {{ computedState.characterCodexList.value.length }} 位角色
              </span>
            </div>
            <p class="text-[11px] text-ink-400 mt-0.5">
              破除单一主角偏见：收录各方势力角色的戏剧定位、立场变迁、独立台词声口与章节演进历史
            </p>
          </div>
        </div>

        <button 
          @click="openCreateCharacter"
          class="flex items-center gap-1.5 px-3 py-1.5 bg-atelier-850 hover:bg-atelier-800 border border-atelier-750 hover:border-brand-amber/40 text-xs font-semibold text-ink-200 hover:text-brand-amber rounded-lg transition cursor-pointer self-start sm:self-auto">
          <Plus class="w-3.5 h-3.5" />
          <span>添加群像角色</span>
        </button>
      </div>

      <!-- 空状态 -->
      <div 
        v-if="computedState.characterCodexList.value.length === 0" 
        class="py-10 text-center space-y-3 bg-atelier-950/50 rounded-xl border border-dashed border-atelier-800">
        <Users class="w-8 h-8 text-ink-500 mx-auto opacity-50" />
        <div class="space-y-1">
          <p class="text-xs font-medium text-ink-300">当前尚未收录群像配角</p>
          <p class="text-[11px] text-ink-500 max-w-sm mx-auto">
            点击上方按钮添加反派、导师或盟友。系统在正文推演中将自动按其独有声口刻画台词，并按章节提交记录立场演化。
          </p>
        </div>
      </div>

      <!-- 角色卡片列表 -->
      <div v-else class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        <div 
          v-for="char in computedState.characterCodexList.value" 
          :key="char.id"
          class="bg-atelier-950 border border-atelier-800 hover:border-atelier-700 rounded-xl p-4 space-y-3 transition flex flex-col justify-between group">
          
          <div class="space-y-2.5">
            <!-- 头部：姓名、标识色与双徽章 -->
            <div class="flex items-start justify-between gap-2">
              <div class="flex items-center gap-2">
                <div 
                  class="w-3.5 h-3.5 rounded-full border border-black/30 shrink-0" 
                  :style="{ backgroundColor: char.color_tag || '#e0a96d' }"></div>
                <h4 class="text-sm font-serif font-bold text-ink-100 group-hover:text-brand-amber transition">
                  {{ char.name }}
                </h4>
              </div>

              <div class="flex items-center gap-1.5 flex-wrap justify-end">
                <!-- 戏剧定位徽章 -->
                <span 
                  :class="getArchetypeBadgeClass(char.archetype)"
                  class="text-[10px] font-mono px-2 py-0.5 rounded border font-semibold">
                  {{ formatArchetype(char.archetype) }}
                </span>
                <!-- 对主立场徽章 -->
                <span 
                  :class="getDispositionBadgeClass(char.current_disposition)"
                  class="text-[10px] font-mono px-2 py-0.5 rounded border font-semibold">
                  {{ formatDisposition(char.current_disposition) }}
                </span>
              </div>
            </div>

            <!-- 核心一句话定位摘要 -->
            <p v-if="char.summary" class="text-xs text-ink-300 font-serif leading-relaxed line-clamp-2">
              {{ char.summary }}
            </p>

            <!-- 台词声口与语言特征 -->
            <div class="bg-atelier-900/90 rounded-lg p-2.5 border border-atelier-800 space-y-1.5 text-[11px]">
              <div class="flex items-start gap-1.5 text-ink-300">
                <span class="text-brand-amber shrink-0">🗣️ 声口：</span>
                <span class="font-mono text-ink-200">
                  {{ char.voice_tone || '沉稳冷静，言简意赅' }}
                </span>
              </div>
              <div class="flex items-start gap-1.5 text-ink-300">
                <span class="text-brand-crimson shrink-0">🎯 动机：</span>
                <span class="text-ink-200">
                  {{ char.core_motivation || '求生存与利益自保' }}
                </span>
              </div>
            </div>

            <!-- 阶段演化记录 / 激活状态 -->
            <div v-if="char.progressions && char.progressions.length > 0" class="space-y-1 text-[11px]">
              <span class="text-[10px] text-ink-400 font-mono flex items-center gap-1">
                <GitCommit class="w-3 h-3 text-brand-amber" />
                <span>动态演进快照 (第 {{ char.progressions[char.progressions.length - 1].active_from_chapter }} 章起)：</span>
              </span>
              <p class="text-ink-300 bg-atelier-900 px-2 py-1 rounded border border-atelier-800 text-[10px] font-serif truncate">
                {{ char.progressions[char.progressions.length - 1].label }}: 
                {{ char.progressions[char.progressions.length - 1].state_description }}
              </p>
            </div>

            <!-- 别名列表 -->
            <div v-if="char.aliases && char.aliases.length > 0" class="flex items-center gap-1 flex-wrap pt-1">
              <span class="text-[10px] text-ink-500 font-mono">别名:</span>
              <span 
                v-for="alias in char.aliases" 
                :key="alias" 
                class="text-[10px] font-mono bg-atelier-900 text-ink-400 px-1.5 py-0.5 rounded border border-atelier-800">
                {{ alias }}
              </span>
            </div>
          </div>

          <!-- 底部动作条 -->
          <div class="pt-2 border-t border-atelier-800/80 flex items-center justify-between text-[11px]">
            <span class="text-[10px] font-mono text-ink-500">
              追踪: {{ char.tracking_mode }}
            </span>
            <button 
              @click="editCharacter(char)"
              class="flex items-center gap-1 text-ink-400 hover:text-brand-amber transition cursor-pointer text-[11px]">
              <Edit3 class="w-3 h-3" />
              <span>编辑人设与状态</span>
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { state, computedState, actions } from '../stores/appState';
import { 
  Cpu, 
  Save, 
  User, 
  Scale, 
  Zap,
  Users,
  Plus,
  Edit3,
  GitCommit
} from 'lucide-vue-next';

function formatArchetype(arc) {
  const map = {
    PROTAGONIST: '主角/同位体',
    ANTAGONIST: '宿敌反派',
    DEUTERAGONIST: '关键搭档',
    MENTOR: '引路导师',
    SUPPORTING: '群像配角',
  };
  return map[arc] || arc || '群像配角';
}

function getArchetypeBadgeClass(arc) {
  switch (arc) {
    case 'ANTAGONIST':
      return 'bg-rose-500/10 text-rose-400 border-rose-500/20';
    case 'PROTAGONIST':
      return 'bg-amber-500/10 text-amber-400 border-amber-500/20';
    case 'DEUTERAGONIST':
      return 'bg-purple-500/10 text-purple-400 border-purple-500/20';
    case 'MENTOR':
      return 'bg-blue-500/10 text-blue-400 border-blue-500/20';
    default:
      return 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20';
  }
}

function formatDisposition(disp) {
  const map = {
    HOSTILE: '敌对仇视',
    WARY: '警惕忌惮',
    NEUTRAL: '中立唯利',
    FRIENDLY: '亲善互盟',
    DEVOTED: '忠诚归心',
  };
  return map[disp] || disp || '中立唯利';
}

function getDispositionBadgeClass(disp) {
  switch (disp) {
    case 'HOSTILE':
      return 'bg-rose-500/15 text-rose-300 border-rose-500/30';
    case 'WARY':
      return 'bg-amber-500/15 text-amber-300 border-amber-500/30';
    case 'FRIENDLY':
      return 'bg-emerald-500/15 text-emerald-300 border-emerald-500/30';
    case 'DEVOTED':
      return 'bg-cyan-500/15 text-cyan-300 border-cyan-500/30';
    default:
      return 'bg-zinc-500/15 text-zinc-300 border-zinc-500/30';
  }
}

function openCreateCharacter() {
  state.editingCodexEntry = {
    category: 'CHARACTER',
    archetype: 'SUPPORTING',
    current_disposition: 'NEUTRAL',
    tracking_mode: 'AUTO_MENTION',
  };
  state.showCodexModal = true;
}

function editCharacter(char) {
  state.editingCodexEntry = { ...char };
  state.showCodexModal = true;
}
</script>

