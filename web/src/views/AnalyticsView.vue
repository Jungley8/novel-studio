<template>
  <div class="flex-1 p-6 overflow-y-auto space-y-6" v-if="state.currentProject">
    <!-- 头部工具栏 -->
    <div class="flex flex-col md:flex-row md:items-center justify-between gap-4 border-b border-atelier-750 pb-5">
      <div>
        <div class="flex items-center gap-2.5">
          <div class="w-8 h-8 rounded-lg bg-brand-amber/10 border border-brand-amber/25 flex items-center justify-center text-brand-amber shadow-amber-glow">
            <BarChart3 class="w-4 h-4" />
          </div>
          <div>
            <h2 class="text-base font-serif font-bold text-ink-50 tracking-wide">全书体检</h2>
            <p class="text-xs text-ink-400 mt-0.5">掌握人物登场频率与剧情起伏节奏。</p>
          </div>
        </div>
      </div>

      <div class="flex items-center gap-3">
        <div class="flex gap-1 bg-atelier-900 p-1 rounded-lg border border-atelier-750 text-xs">
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
          @click="actions.loadAnalytics" 
          class="flex items-center gap-1.5 px-3 py-1.5 bg-atelier-850 hover:bg-atelier-800 text-ink-300 text-xs rounded-md border border-atelier-750 transition cursor-pointer">
          <RefreshCw class="w-3.5 h-3.5 text-ink-400" />
          <span>刷新</span>
        </button>
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
import { ref } from 'vue';
import { state, actions } from '../stores/appState';
import { BarChart3, Flame, Activity, RefreshCw } from 'lucide-vue-next';

const subTab = ref('heatmap');

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
