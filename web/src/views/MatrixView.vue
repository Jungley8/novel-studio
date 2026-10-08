<template>
  <div class="flex-1 p-6 overflow-y-auto space-y-6" v-if="state.currentProject">
    <!-- 头部指标栏 -->
    <div class="flex flex-col md:flex-row md:items-center justify-between gap-4 border-b border-atelier-750 pb-5">
      <div>
        <div class="flex items-center gap-2.5">
          <div class="w-8 h-8 rounded-lg bg-brand-amber/10 border border-brand-amber/25 flex items-center justify-center text-brand-amber shadow-amber-glow">
            <LayoutGrid class="w-4 h-4" />
          </div>
          <div>
            <h2 class="text-base font-serif font-bold text-ink-50 tracking-wide">矩阵大纲 (The Matrix)</h2>
            <p class="text-xs text-ink-400 mt-0.5">分卷 / 幕次 / 章节 / 场次 4 层戏剧张力透视与结构网格</p>
          </div>
        </div>
      </div>

      <div class="flex items-center gap-3">
        <div v-if="state.matrixOverview" class="flex items-center gap-2 text-xs font-mono bg-atelier-850 px-3 py-1.5 rounded-md border border-atelier-750">
          <span class="text-ink-400">场次</span>
          <strong class="text-brand-amber">{{ state.matrixOverview.total_scenes }}</strong>
          <span class="text-atelier-700">|</span>
          <span class="text-ink-400">总字数</span>
          <strong class="text-brand-amber">{{ state.matrixOverview.total_words?.toLocaleString() || 0 }}</strong>
        </div>
        <button 
          @click="actions.loadMatrixOverview" 
          class="flex items-center gap-1.5 px-3 py-1.5 bg-atelier-850 hover:bg-atelier-800 text-ink-200 text-xs font-medium rounded-md border border-atelier-750 transition cursor-pointer">
          <RefreshCw class="w-3.5 h-3.5 text-ink-400" />
          <span>刷新大纲</span>
        </button>
      </div>
    </div>

    <!-- 卷与场次主容器 -->
    <div v-if="state.matrixOverview && state.matrixOverview.volumes && state.matrixOverview.volumes.length > 0" class="space-y-6">
      <div 
        v-for="vol in state.matrixOverview.volumes" 
        :key="vol.volume_index" 
        class="bg-atelier-900 border border-atelier-750 rounded-xl p-5 space-y-4 shadow-atelier-md">
        
        <!-- 分卷头部 -->
        <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 border-b border-atelier-750/80 pb-3.5">
          <div class="flex items-center gap-3">
            <span class="px-2.5 py-1 bg-brand-amber/15 text-brand-amber text-xs font-mono font-bold rounded border border-brand-amber/30 shadow-amber-glow">
              第 {{ vol.volume_index }} 卷
            </span>
            <h3 class="text-sm font-serif font-bold text-ink-100 tracking-wide">{{ vol.title }}</h3>
            <span v-if="vol.theme" class="text-xs text-ink-400 truncate max-w-md">· 核心立意: {{ vol.theme }}</span>
          </div>

          <div class="flex items-center gap-4 text-xs font-mono text-ink-400">
            <div class="flex items-center gap-1.5">
              <span>平均张力</span>
              <span :class="tensionColorClass(Math.round(vol.avg_tension || 5))" class="font-bold">
                {{ (vol.avg_tension || 0).toFixed(1) }}/10
              </span>
            </div>
            <span>{{ vol.total_words?.toLocaleString() || 0 }} 字</span>
          </div>
        </div>

        <!-- 章节行与场次卡片列表 -->
        <div class="space-y-3.5">
          <div 
            v-for="row in vol.chapters" 
            :key="row.chapter.id" 
            class="p-4 bg-atelier-950/70 rounded-lg border border-atelier-800 space-y-3">
            
            <div class="flex items-center justify-between">
              <div class="flex items-center gap-2.5">
                <span class="text-xs font-mono font-bold text-brand-amber">第 {{ row.chapter.chapter_index }} 章</span>
                <span class="text-xs font-serif font-semibold text-ink-100">{{ row.chapter.title }}</span>
                <span class="text-[11px] text-ink-400 font-mono">({{ row.chapter.word_count }} 字)</span>
              </div>
              <button 
                @click="openCreateScene(row.chapter.id)" 
                class="px-2.5 py-1 text-xs bg-brand-amber/10 hover:bg-brand-amber/20 text-brand-amber rounded border border-brand-amber/30 transition flex items-center gap-1 cursor-pointer">
                <Plus class="w-3 h-3" />
                <span>添加场次</span>
              </button>
            </div>

            <!-- 场景卡片横向网格 -->
            <div v-if="row.scenes && row.scenes.length > 0" class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3 pt-1">
              <div 
                v-for="sc in row.scenes" 
                :key="sc.id" 
                class="p-3.5 bg-atelier-900 border border-atelier-750/90 rounded-lg hover:border-brand-amber/40 hover:shadow-amber-glow transition space-y-2.5 group">
                
                <div class="flex items-center justify-between gap-2">
                  <div class="flex items-center gap-1.5 min-w-0">
                    <span class="text-xs font-mono font-bold text-ink-400 shrink-0">场 {{ sc.scene_index }}:</span>
                    <span class="text-xs font-semibold text-ink-100 truncate">{{ sc.title || '未命名场景' }}</span>
                  </div>
                  <span 
                    class="text-[10px] px-2 py-0.5 rounded font-mono font-bold shrink-0" 
                    :class="sc.tension_level >= 8 
                      ? 'bg-rose-500/15 text-rose-400 border border-rose-500/30' 
                      : (sc.tension_level >= 5 
                        ? 'bg-brand-amber/15 text-brand-amber border border-brand-amber/30' 
                        : 'bg-emerald-500/15 text-emerald-400 border border-emerald-500/30')">
                    张力 {{ sc.tension_level }}
                  </span>
                </div>

                <div class="text-[11px] text-ink-300 space-y-1">
                  <div v-if="sc.dramatic_goal" class="line-clamp-1">
                    <span class="text-ink-400 font-medium">目标:</span> {{ sc.dramatic_goal }}
                  </div>
                  <div v-if="sc.conflict_barrier" class="line-clamp-1">
                    <span class="text-ink-400 font-medium">冲突:</span> {{ sc.conflict_barrier }}
                  </div>
                </div>

                <div class="flex items-center justify-between text-[11px] text-ink-400 pt-2 border-t border-atelier-800">
                  <span class="font-mono text-[10px]">{{ sc.word_count }} 字</span>
                  <div class="flex items-center gap-2">
                    <button 
                      @click="openSceneMarkers(sc)" 
                      class="hover:text-brand-amber text-[11px] flex items-center gap-1 transition cursor-pointer">
                      <Bookmark class="w-3 h-3" />
                      <span>批注 ({{ sc.markers ? sc.markers.length : 0 }})</span>
                    </button>
                    <button 
                      @click="editScene(sc)" 
                      class="hover:text-ink-100 transition cursor-pointer">
                      编辑
                    </button>
                    <button 
                      @click="actions.deleteScene(sc.id)" 
                      class="hover:text-rose-400 transition cursor-pointer">
                      删除
                    </button>
                  </div>
                </div>
              </div>
            </div>

            <div v-else class="text-[11px] text-ink-400 italic py-1.5 px-1 flex items-center gap-2">
              <Sparkles class="w-3.5 h-3.5 text-atelier-600" />
              <span>暂无细分场次，点击右上角“+ 添加场次”进行精细化戏剧拆解与张力控制</span>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- 空白状态 -->
    <div v-else class="text-center py-20 bg-atelier-900 border border-dashed border-atelier-750 rounded-xl space-y-3">
      <div class="w-12 h-12 rounded-full bg-atelier-800 flex items-center justify-center mx-auto text-ink-400">
        <LayoutGrid class="w-6 h-6" />
      </div>
      <h3 class="text-sm font-semibold text-ink-200">暂无大纲数据</h3>
      <p class="text-xs text-ink-400 max-w-md mx-auto">
        请先在生产手稿中推演章节，或在创世总纲中规划分卷任务链。
      </p>
    </div>
  </div>
</template>

<script setup>
import { state, actions } from '../stores/appState';
import { 
  LayoutGrid, 
  RefreshCw, 
  Plus, 
  Bookmark, 
  Sparkles 
} from 'lucide-vue-next';

function tensionColorClass(level) {
  if (level >= 8) return 'text-rose-400';
  if (level >= 5) return 'text-brand-amber';
  return 'text-emerald-400';
}

function openCreateScene(chapterId) {
  state.editingScene = null;
  state.targetChapterIdForNewScene = chapterId;
  state.showSceneModal = true;
}

function editScene(scene) {
  state.editingScene = { ...scene };
  state.showSceneModal = true;
}

function openSceneMarkers(scene) {
  state.activeSceneForMarkers = scene;
  state.sceneMarkersList = scene.markers || [];
  state.showMarkersModal = true;
}
</script>
