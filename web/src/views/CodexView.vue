<template>
  <div class="flex-1 p-6 overflow-y-auto space-y-6" v-if="state.currentProject">
    <!-- 头部工具栏 -->
    <div class="flex flex-col md:flex-row md:items-center justify-between gap-4 border-b border-atelier-750 pb-5">
      <div>
        <div class="flex items-center gap-2.5">
          <div class="w-8 h-8 rounded-lg bg-brand-amber/10 border border-brand-amber/25 flex items-center justify-center text-brand-amber shadow-amber-glow">
            <BookOpen class="w-4 h-4" />
          </div>
          <div>
            <h2 class="text-base font-serif font-bold text-ink-50 tracking-wide">设定集</h2>
            <p class="text-xs text-ink-400 mt-0.5">管理人物、势力、地点与宝物档案。</p>
          </div>
        </div>
      </div>

      <div class="flex items-center gap-2.5">
        <button 
          @click="showMentionScanner = !showMentionScanner"
          :class="showMentionScanner ? 'bg-brand-amber/15 text-brand-amber border-brand-amber/30' : 'bg-atelier-850 hover:bg-atelier-800 text-ink-300 border-atelier-750'"
          class="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-md border transition cursor-pointer"
          title="扫描正文中的人物引用">
          <ScanSearch class="w-3.5 h-3.5" />
          <span>引用检测</span>
        </button>

        <button 
          @click="openCreateCodexEntry"
          class="flex items-center gap-1.5 px-3.5 py-1.5 bg-brand-amber hover:bg-brand-amber-hover text-atelier-950 text-xs font-bold rounded-md shadow-amber-glow transition cursor-pointer">
          <Plus class="w-3.5 h-3.5" />
          <span>新建词条</span>
        </button>
      </div>
    </div>

    <!-- 词条分类过滤标签 -->
    <div class="flex items-center justify-between gap-4">
      <div class="flex gap-1 bg-atelier-900 p-1 rounded-lg border border-atelier-750 text-xs">
        <button 
          v-for="cat in categories" 
          :key="cat.key" 
          @click="state.codexCategoryFilter = cat.key"
          :class="state.codexCategoryFilter === cat.key 
            ? 'bg-atelier-800 text-brand-amber font-semibold shadow-atelier-sm' 
            : 'text-ink-400 hover:text-ink-200'"
          class="px-3 py-1.5 rounded-md transition cursor-pointer flex items-center gap-1.5">
          <component :is="cat.icon" class="w-3.5 h-3.5" />
          <span>{{ cat.label }}</span>
        </button>
      </div>

      <div class="text-xs font-mono text-ink-400">
        共 <strong class="text-brand-amber">{{ computedState.filteredCodexEntries.value.length }}</strong> 条设定
      </div>
    </div>

    <!-- 引用扫描试验台抽屉/展开区 -->
    <div 
      v-if="showMentionScanner" 
      class="p-4 bg-atelier-900 border border-brand-amber/30 rounded-xl space-y-3.5 shadow-amber-glow">
      <div class="flex items-center justify-between">
        <div class="flex items-center gap-2">
          <ScanSearch class="w-4 h-4 text-brand-amber" />
          <span class="text-xs font-semibold text-brand-amber">词条检测 (测试正文中角色与设定的自动识别)</span>
        </div>
        <button 
          @click="runMentionScan" 
          :disabled="isScanning || !mentionScanText.trim()"
          class="px-3 py-1 text-xs bg-brand-amber hover:bg-brand-amber-hover text-atelier-950 font-bold rounded transition cursor-pointer disabled:opacity-50 flex items-center gap-1">
          <Sparkles class="w-3 h-3" />
          <span>{{ isScanning ? '扫描中...' : '运行扫描' }}</span>
        </button>
      </div>

      <textarea 
        v-model="mentionScanText" 
        rows="2" 
        class="w-full bg-atelier-950 border border-atelier-750 rounded-lg p-2.5 text-xs text-ink-100 font-serif resize-none focus:outline-none focus:border-brand-amber/50" 
        placeholder="输入一段正文进行模拟检测（例如：风雪中，白衣修罗顾渊悄然拔出残骨铁印，望向那巍峨的聚仙楼...）"></textarea>

      <!-- 扫描结果展示 -->
      <div v-if="mentionScanResult" class="p-3 bg-atelier-950 rounded-lg border border-atelier-800 text-xs space-y-2">
        <div class="text-ink-400 flex items-center gap-2">
          <span>匹配到 <strong class="text-emerald-400">{{ mentionScanResult.matched_entries?.length || 0 }}</strong> 个词条</span>
          <span>·</span>
          <span>命中别名引用 <strong class="text-brand-amber">{{ mentionScanResult.mentions?.length || 0 }}</strong> 次</span>
        </div>
        <div class="flex flex-wrap gap-2 pt-1">
          <span 
            v-for="m in mentionScanResult.mentions" 
            :key="m.alias_used + m.position_start" 
            class="text-[11px] px-2.5 py-1 rounded font-mono bg-atelier-850 border border-atelier-750 text-ink-200 flex items-center gap-1.5">
            <span class="w-2 h-2 rounded-full bg-brand-amber"></span>
            <strong>{{ m.entry.name }}</strong>
            <span class="text-ink-400 text-[10px]">(命中: "{{ m.alias_used }}")</span>
          </span>
        </div>
      </div>
    </div>

    <!-- 设定词条卡片网格 -->
    <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
      <div 
        v-for="entry in computedState.filteredCodexEntries.value" 
        :key="entry.id" 
        class="p-4 bg-atelier-900 border border-atelier-750 rounded-xl hover:border-brand-amber/40 hover:shadow-atelier-md transition space-y-3 group">
        
        <div class="flex items-center justify-between gap-2">
          <div class="flex items-center gap-2 min-w-0">
            <span 
              class="w-3 h-3 rounded-full shrink-0 ring-2 ring-atelier-800" 
              :style="{ backgroundColor: entry.color_tag || '#e5a93c' }"></span>
            <h3 class="text-sm font-serif font-bold text-ink-50 truncate tracking-wide">{{ entry.name }}</h3>
            <span class="text-[10px] font-mono px-1.5 py-0.2 rounded bg-atelier-800 text-ink-400 border border-atelier-750 shrink-0">
              {{ formatCategoryName(entry.category) }}
            </span>
          </div>

          <span 
            class="text-[10px] px-2 py-0.5 rounded font-mono shrink-0" 
            :class="entry.tracking_mode === 'ALWAYS_INJECT' 
              ? 'bg-brand-amber/15 text-brand-amber border border-brand-amber/30' 
              : 'bg-atelier-800 text-ink-400 border border-atelier-750'">
            {{ entry.tracking_mode }}
          </span>
        </div>

        <p class="text-xs text-ink-300 leading-relaxed font-sans line-clamp-2 min-h-8">
          {{ entry.summary || '暂无一句话摘要' }}
        </p>

        <!-- 别名列表 -->
        <div v-if="entry.aliases && entry.aliases.length > 0" class="flex flex-wrap gap-1">
          <span 
            v-for="al in entry.aliases" 
            :key="al" 
            class="text-[10px] font-mono bg-atelier-850 text-ink-400 px-1.5 py-0.5 rounded border border-atelier-800">
            {{ al }}
          </span>
        </div>

        <!-- 演进阶段与人际网络操作条 -->
        <div class="pt-2.5 border-t border-atelier-800 flex items-center justify-between text-xs text-ink-400">
          <div class="flex items-center gap-3">
            <button 
              @click="openProgressionManager(entry)" 
              class="hover:text-brand-amber transition flex items-center gap-1 cursor-pointer text-[11px]">
              <GitCommit class="w-3 h-3 text-ink-400" />
              <span>阶段 ({{ entry.progressions ? entry.progressions.length : 0 }})</span>
            </button>
            <button 
              @click="openRelationManager(entry)" 
              class="hover:text-brand-amber transition flex items-center gap-1 cursor-pointer text-[11px]">
              <Share2 class="w-3 h-3 text-ink-400" />
              <span>拓扑 ({{ entry.relations ? entry.relations.length : 0 }})</span>
            </button>
          </div>

          <div class="flex items-center gap-2">
            <button 
              @click="editCodexEntry(entry)" 
              class="hover:text-ink-100 transition cursor-pointer text-[11px]">
              编辑
            </button>
            <button 
              @click="actions.deleteCodexEntry(entry.id)" 
              class="hover:text-rose-400 transition cursor-pointer text-[11px]">
              删除
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- 空白状态 -->
    <div 
      v-if="computedState.filteredCodexEntries.value.length === 0" 
      class="text-center py-20 bg-atelier-900 border border-dashed border-atelier-750 rounded-xl space-y-3">
      <div class="w-12 h-12 rounded-full bg-atelier-800 flex items-center justify-center mx-auto text-ink-400">
        <BookOpen class="w-6 h-6" />
      </div>
      <h3 class="text-sm font-semibold text-ink-200">当前分类暂无词条</h3>
      <p class="text-xs text-ink-400 max-w-md mx-auto">
        点击右上角“+ 新建词条”录入角色、势力、宝物或地理设定，写作时将自动作为参考。
      </p>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue';
import { state, computedState, actions } from '../stores/appState';
import { api } from '../api/client';
import { 
  BookOpen, 
  Plus, 
  ScanSearch, 
  User, 
  Compass, 
  Scroll, 
  Gem, 
  Sparkles,
  GitCommit,
  Share2
} from 'lucide-vue-next';

const categories = [
  { key: 'ALL', label: '全部', icon: BookOpen },
  { key: 'CHARACTER', label: '角色', icon: User },
  { key: 'LOCATION', label: '地理场景', icon: Compass },
  { key: 'LORE', label: '设定公理', icon: Scroll },
  { key: 'ITEM', label: '道具灵宝', icon: Gem },
];

const showMentionScanner = ref(false);
const mentionScanText = ref('');
const isScanning = ref(false);
const mentionScanResult = ref(null);

function formatCategoryName(cat) {
  const map = {
    CHARACTER: '角色',
    LOCATION: '场景',
    LORE: '公理',
    ITEM: '灵宝',
  };
  return map[cat] || cat;
}

async function runMentionScan() {
  if (!mentionScanText.value.trim() || !state.currentProject) return;
  isScanning.value = true;
  try {
    mentionScanResult.value = await api.scanCodex(state.currentProject.id, mentionScanText.value);
  } catch (e) {
    console.error('mention scan error:', e);
  } finally {
    isScanning.value = false;
  }
}

function openCreateCodexEntry() {
  state.editingCodexEntry = null;
  state.showCodexModal = true;
}

function editCodexEntry(entry) {
  state.editingCodexEntry = { ...entry };
  state.showCodexModal = true;
}

function openProgressionManager(entry) {
  state.activeCodexForSub = entry;
  state.showProgressionModal = true;
}

function openRelationManager(entry) {
  state.activeCodexForSub = entry;
  state.showRelationModal = true;
}
</script>
