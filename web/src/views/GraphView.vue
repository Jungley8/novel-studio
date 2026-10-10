<template>
  <div class="flex-1 flex flex-col h-full overflow-hidden bg-atelier-950 select-none" v-if="state.currentProject">
    <!-- 头部工具栏 -->
    <div class="h-14 px-6 border-b border-atelier-750 flex items-center justify-between shrink-0 bg-atelier-900/60 backdrop-blur-sm z-10">
      <div class="flex items-center gap-3">
        <div class="w-8 h-8 rounded-lg bg-brand-amber/10 border border-brand-amber/25 flex items-center justify-center text-brand-amber shadow-amber-glow">
          <Network class="w-4 h-4" />
        </div>
        <div>
          <div class="flex items-center gap-2">
            <h2 class="text-sm font-serif font-bold text-ink-50 tracking-wide">关系图谱</h2>
            <span class="text-[10px] font-mono px-1.5 py-0.2 rounded bg-atelier-800 text-ink-300 border border-atelier-750">
              {{ validNodes.length }} 实体 · {{ validLinks.length }} 关联
            </span>
          </div>
          <p class="text-[11px] text-ink-400">实体与势力错综关联图谱。</p>
        </div>
      </div>

      <!-- 顶部操作区 -->
      <div class="flex items-center gap-2.5">
        <!-- 搜索框 -->
        <div class="relative w-44">
          <Search class="w-3.5 h-3.5 text-ink-400 absolute left-2.5 top-2.5 pointer-events-none" />
          <input 
            v-model="searchQuery" 
            type="text" 
            placeholder="搜索实体..." 
            class="w-full bg-atelier-950 border border-atelier-750 rounded-lg pl-8 pr-2.5 py-1.5 text-xs text-ink-100 placeholder:text-ink-500 focus:outline-none focus:border-brand-amber/60">
        </div>

        <!-- 居中复位 -->
        <button 
          @click="resetView" 
          class="p-1.5 bg-atelier-850 hover:bg-atelier-800 text-ink-300 hover:text-ink-100 border border-atelier-750 rounded-lg transition cursor-pointer"
          title="复位画布视角">
          <RotateCcw class="w-3.5 h-3.5" />
        </button>

        <!-- 新增关联 -->
        <button 
          @click="openCreateRelationModal()" 
          class="flex items-center gap-1.5 px-3 py-1.5 bg-atelier-850 hover:bg-atelier-800 text-ink-200 hover:text-brand-amber border border-atelier-750 hover:border-brand-amber/40 text-xs font-medium rounded-lg transition cursor-pointer"
          title="手动为两个实体添加关系连线">
          <Plus class="w-3.5 h-3.5" />
          <span>新增关联</span>
        </button>

        <!-- AI 智能推演 -->
        <button 
          @click="runAIExtractRelations" 
          :disabled="isExtracting || validNodes.length < 2"
          class="flex items-center gap-1.5 px-3.5 py-1.5 bg-brand-amber hover:bg-brand-amber-hover text-atelier-950 text-xs font-bold rounded-lg shadow-amber-glow transition cursor-pointer disabled:opacity-50"
          title="根据设定集与最新剧情，AI 自动梳理并更新实体拓扑关系">
          <Sparkles class="w-3.5 h-3.5" :class="{ 'animate-spin': isExtracting }" />
          <span>{{ isExtracting ? '推演中...' : '智能推演' }}</span>
        </button>
      </div>
    </div>

    <!-- 分类过滤横条 -->
    <div class="px-6 py-2 border-b border-atelier-800 bg-atelier-900/40 flex items-center justify-between text-xs shrink-0">
      <div class="flex items-center gap-1.5 overflow-x-auto">
        <button 
          v-for="cat in categoryFilters" 
          :key="cat.key" 
          @click="selectedCategory = cat.key"
          :class="selectedCategory === cat.key 
            ? 'bg-atelier-800 text-brand-amber font-semibold shadow-atelier-sm border-brand-amber/30' 
            : 'text-ink-400 hover:text-ink-200 border-transparent'"
          class="px-2.5 py-1 rounded-md border text-[11px] transition cursor-pointer flex items-center gap-1.5 shrink-0">
          <span class="w-2 h-2 rounded-full" :style="{ backgroundColor: cat.color }"></span>
          <span>{{ cat.label }}</span>
          <span class="text-[9px] font-mono text-ink-500">({{ cat.count }})</span>
        </button>
      </div>

      <div class="text-[11px] text-ink-500 font-mono hidden sm:block">
        支持拖拽节点 · 滚轮缩放 · 点击卡片查看关系细节
      </div>
    </div>

    <!-- 图谱交互主画布 -->
    <div 
      ref="containerRef"
      class="flex-1 relative overflow-hidden bg-atelier-950 cursor-grab active:cursor-grabbing"
      @mousedown="handleCanvasMouseDown"
      @wheel.prevent="handleWheel">

      <!-- 背景网格微点 -->
      <div 
        class="absolute inset-0 pointer-events-none opacity-20"
        :style="{
          backgroundImage: 'radial-gradient(circle, #888 1px, transparent 1px)',
          backgroundSize: `${24 * zoom}px ${24 * zoom}px`,
          backgroundPosition: `${pan.x}px ${pan.y}px`
        }">
      </div>

      <!-- SVG 渲染层 -->
      <svg 
        class="w-full h-full absolute inset-0 overflow-visible"
        :style="{
          transform: `translate(${pan.x}px, ${pan.y}px) scale(${zoom})`,
          transformOrigin: '0 0'
        }">
        <defs>
          <!-- 箭头标记 -->
          <marker 
            id="graph-arrow" 
            viewBox="0 0 10 10" 
            refX="22" 
            refY="5" 
            markerWidth="6" 
            markerHeight="6" 
            orient="auto-start-reverse">
            <path d="M 0 1.5 L 8 5 L 0 8.5 z" fill="#d97706" opacity="0.8" />
          </marker>
          <marker 
            id="graph-arrow-active" 
            viewBox="0 0 10 10" 
            refX="22" 
            refY="5" 
            markerWidth="7" 
            markerHeight="7" 
            orient="auto-start-reverse">
            <path d="M 0 1 L 9 5 L 0 9 z" fill="#f59e0b" />
          </marker>
        </defs>

        <!-- 连线组 (Edges) -->
        <g class="edges">
          <g 
            v-for="link in validLinks" 
            :key="link.id" 
            class="group cursor-pointer"
            @click.stop="selectLink(link)">
            <!-- 连线透明热区 (便于点击) -->
            <path 
              :d="computeLinkPath(link)" 
              stroke="transparent" 
              stroke-width="14" 
              fill="none" />
            
            <!-- 实际展示连线 -->
            <path 
              :d="computeLinkPath(link)" 
              :stroke="isLinkHighlighted(link) ? '#f59e0b' : '#3f3f46'" 
              :stroke-width="isLinkHighlighted(link) ? 2.5 : 1.5" 
              :stroke-opacity="isLinkDimmed(link) ? 0.15 : (isLinkHighlighted(link) ? 1 : 0.6)"
              :marker-end="isLinkHighlighted(link) ? 'url(#graph-arrow-active)' : 'url(#graph-arrow)'"
              stroke-dasharray="none"
              fill="none"
              class="transition-all duration-200" />

            <!-- 关系文字标签 -->
            <g :transform="`translate(${computeLinkMidpoint(link).x}, ${computeLinkMidpoint(link).y})`">
              <rect 
                x="-24" 
                y="-10" 
                width="48" 
                height="20" 
                rx="4" 
                :fill="isLinkHighlighted(link) ? '#451a03' : '#18181b'" 
                :stroke="isLinkHighlighted(link) ? '#f59e0b' : '#27272a'" 
                stroke-width="1"
                :opacity="isLinkDimmed(link) ? 0.2 : 0.95" />
              <text 
                x="0" 
                y="3" 
                text-anchor="middle" 
                font-size="9" 
                :fill="isLinkHighlighted(link) ? '#fef3c7' : '#a1a1aa'"
                font-family="monospace"
                :opacity="isLinkDimmed(link) ? 0.2 : 1"
                class="select-none pointer-events-none">
                {{ formatRelationLabel(link.relation_type) }}
              </text>
            </g>
          </g>
        </g>

        <!-- 节点组 (Nodes) -->
        <g class="nodes">
          <g 
            v-for="node in validNodes" 
            :key="node.id"
            :transform="`translate(${node.x}, ${node.y})`"
            class="cursor-pointer group"
            @mousedown.stop="startDragNode($event, node)"
            @click.stop="selectNode(node)">
            
            <!-- 主角特有尊贵金色旋转光环 (Radial Anchor) -->
            <circle 
              v-if="node.isProtagonist"
              r="30" 
              fill="none" 
              stroke="#fbbf24" 
              stroke-width="1.5" 
              stroke-dasharray="3 3"
              class="animate-spin"
              style="animation-duration: 25s;" />

            <!-- 选中或高亮光环 -->
            <circle 
              v-if="isNodeHighlighted(node)"
              r="34" 
              fill="none" 
              :stroke="node.color || '#f59e0b'" 
              stroke-width="2" 
              stroke-opacity="0.4"
              class="animate-pulse" />

            <!-- 节点主体背景圆 -->
            <circle 
              :r="node.isProtagonist ? 26 : 24" 
              :fill="isNodeSelected(node) ? '#27272a' : '#18181b'" 
              :stroke="node.color || '#f59e0b'" 
              :stroke-width="node.isProtagonist ? 2.5 : (isNodeSelected(node) ? 2.5 : 1.5)" 
              :opacity="isNodeDimmed(node) ? 0.2 : 1"
              class="transition-all duration-200 shadow-lg" />

            <!-- 节点分类简写字母或图标 -->
            <text 
              x="0" 
              y="5" 
              text-anchor="middle" 
              font-size="12" 
              font-family="serif" 
              font-weight="bold"
              :fill="node.color || '#f59e0b'"
              :opacity="isNodeDimmed(node) ? 0.2 : 1"
              class="select-none pointer-events-none">
              {{ node.name.slice(0, 1) }}
            </text>

            <!-- 节点下方名称 -->
            <text 
              x="0" 
              y="40" 
              text-anchor="middle" 
              font-size="11" 
              font-family="serif" 
              font-weight="600"
              :fill="isNodeHighlighted(node) ? '#ffffff' : '#e4e4e7'"
              :opacity="isNodeDimmed(node) ? 0.2 : 1"
              class="select-none pointer-events-none">
              {{ node.name }}
            </text>

            <!-- 节点下方小类目标识 -->
            <text 
              x="0" 
              y="52" 
              text-anchor="middle" 
              font-size="8" 
              font-family="monospace" 
              :fill="node.color || '#a1a1aa'"
              :opacity="isNodeDimmed(node) ? 0.15 : 0.75"
              class="select-none pointer-events-none">
              {{ formatCategoryName(node.category) }}
            </text>
          </g>
        </g>
      </svg>

      <!-- 悬浮缩放控制按钮 -->
      <div class="absolute bottom-5 left-6 flex items-center gap-1.5 bg-atelier-900/90 border border-atelier-750 p-1 rounded-lg shadow-xl backdrop-blur-sm z-10">
        <button 
          @click="zoomIn" 
          class="p-1.5 text-ink-300 hover:text-ink-100 hover:bg-atelier-800 rounded transition cursor-pointer"
          title="放大画布">
          <ZoomIn class="w-3.5 h-3.5" />
        </button>
        <span class="text-[10px] font-mono text-ink-400 px-1 w-10 text-center">
          {{ Math.round(zoom * 100) }}%
        </span>
        <button 
          @click="zoomOut" 
          class="p-1.5 text-ink-300 hover:text-ink-100 hover:bg-atelier-800 rounded transition cursor-pointer"
          title="缩小画布">
          <ZoomOut class="w-3.5 h-3.5" />
        </button>
        <div class="w-px h-3 bg-atelier-750 mx-0.5"></div>
        <button 
          @click="resetView" 
          class="p-1.5 text-ink-300 hover:text-ink-100 hover:bg-atelier-800 rounded transition cursor-pointer"
          title="复位视角">
          <RotateCcw class="w-3.5 h-3.5" />
        </button>
      </div>

      <!-- 空白状态浮层 -->
      <div 
        v-if="validNodes.length === 0" 
        class="absolute inset-0 flex items-center justify-center p-6 pointer-events-none">
        <div class="p-8 bg-atelier-900/95 border border-atelier-750 rounded-2xl max-w-md text-center space-y-4 shadow-2xl pointer-events-auto backdrop-blur-md">
          <div class="w-12 h-12 rounded-full bg-brand-amber/10 border border-brand-amber/20 flex items-center justify-center text-brand-amber mx-auto shadow-amber-glow">
            <Network class="w-6 h-6" />
          </div>
          <div>
            <h3 class="text-sm font-serif font-bold text-ink-100">暂无图谱实体</h3>
            <p class="text-xs text-ink-400 mt-1 leading-relaxed">
              尚未在设定集中录入人物、势力、灵宝或地理设定。请先在设定集中创建词条，或让 AI 智能生成。
            </p>
          </div>
          <button 
            @click="state.activeTab = 'codex'" 
            class="px-4 py-2 bg-brand-amber hover:bg-brand-amber-hover text-atelier-950 font-bold text-xs rounded-lg shadow-amber-glow transition cursor-pointer inline-flex items-center gap-1.5">
            <BookOpen class="w-3.5 h-3.5" />
            <span>前往设定集录入</span>
          </button>
        </div>
      </div>

      <!-- 当有实体但未产生关联线时的引导浮层 -->
      <div 
        v-else-if="validLinks.length === 0" 
        class="absolute top-4 left-1/2 -translate-x-1/2 bg-atelier-900/95 border border-brand-amber/30 rounded-xl px-4 py-2.5 flex items-center gap-3 shadow-amber-glow backdrop-blur-md z-10">
        <Sparkles class="w-4 h-4 text-brand-amber shrink-0 animate-pulse" />
        <span class="text-xs text-ink-200">
          已发现 {{ validNodes.length }} 个实体，点击右侧按钮让 AI 智能推演它们之间的爱恨纠葛与阵营纽带：
        </span>
        <button 
          @click="runAIExtractRelations" 
          :disabled="isExtracting"
          class="px-3 py-1 bg-brand-amber hover:bg-brand-amber-hover text-atelier-950 text-xs font-bold rounded-md shadow-amber-glow transition cursor-pointer disabled:opacity-50 shrink-0">
          {{ isExtracting ? '推演中...' : '立即推演' }}
        </button>
      </div>

      <!-- 右侧实体 / 关系详情抽屉 -->
      <transition 
        enter-active-class="transition duration-200 ease-out" 
        enter-from-class="translate-x-full opacity-0" 
        enter-to-class="translate-x-0 opacity-100" 
        leave-active-class="transition duration-150 ease-in" 
        leave-from-class="translate-x-0 opacity-100" 
        leave-to-class="translate-x-full opacity-0">
        <div 
          v-if="selectedNode || selectedLink" 
          class="absolute top-0 right-0 bottom-0 w-80 bg-atelier-900/95 border-l border-atelier-750 p-5 overflow-y-auto space-y-4 shadow-2xl backdrop-blur-md z-20">
          
          <!-- 实体详情展示 -->
          <div v-if="selectedNode" class="space-y-4">
            <div class="flex items-start justify-between border-b border-atelier-750 pb-3">
              <div class="flex items-center gap-2.5">
                <span 
                  class="w-3.5 h-3.5 rounded-full ring-2 ring-atelier-800 shrink-0" 
                  :style="{ backgroundColor: selectedNode.color }"></span>
                <div>
                  <h3 class="text-sm font-serif font-bold text-ink-50">{{ selectedNode.name }}</h3>
                  <span class="text-[10px] font-mono px-1.5 py-0.2 rounded bg-atelier-800 text-ink-300 border border-atelier-750">
                    {{ formatCategoryName(selectedNode.category) }}
                  </span>
                </div>
              </div>
              <button 
                @click="selectedNode = null" 
                class="p-1 text-ink-400 hover:text-ink-100 rounded transition cursor-pointer">
                <X class="w-4 h-4" />
              </button>
            </div>

            <!-- 摘要与别名 -->
            <div class="space-y-2 text-xs">
              <div v-if="selectedNode.raw?.summary" class="p-2.5 bg-atelier-950 rounded-lg border border-atelier-800 text-ink-300 leading-relaxed font-serif">
                {{ selectedNode.raw.summary }}
              </div>

              <div v-if="selectedNode.raw?.aliases && selectedNode.raw.aliases.length > 0" class="flex flex-wrap gap-1">
                <span 
                  v-for="al in selectedNode.raw.aliases" 
                  :key="al" 
                  class="text-[10px] font-mono bg-atelier-850 text-ink-400 px-1.5 py-0.5 rounded border border-atelier-800">
                  {{ al }}
                </span>
              </div>
            </div>

            <!-- 当前实体关联线列表 -->
            <div class="space-y-2.5 pt-2 border-t border-atelier-750">
              <div class="flex items-center justify-between">
                <span class="text-xs font-serif font-bold text-brand-amber">关联网络 ({{ nodeRelations.length }})</span>
                <button 
                  @click="openCreateRelationModal(selectedNode.id)" 
                  class="text-[11px] text-brand-amber hover:text-brand-amber-hover flex items-center gap-1 cursor-pointer">
                  <Plus class="w-3 h-3" />
                  <span>添加关系</span>
                </button>
              </div>

              <div class="space-y-2 max-h-60 overflow-y-auto">
                <div 
                  v-for="rel in nodeRelations" 
                  :key="rel.id" 
                  class="p-2.5 bg-atelier-950 rounded-lg border border-atelier-800 text-xs space-y-1">
                  <div class="flex items-center justify-between">
                    <span class="font-medium text-ink-200">
                      {{ rel.source_entry_id === selectedNode.id ? '向' : '来自' }}
                      <strong class="text-brand-amber">【{{ getEntityName(rel.source_entry_id === selectedNode.id ? rel.target_entry_id : rel.source_entry_id) }}】</strong>
                    </span>
                    <button 
                      @click="deleteRelation(rel.id)" 
                      class="text-ink-500 hover:text-rose-400 p-0.5 transition cursor-pointer"
                      title="删除此关系">
                      <Trash2 class="w-3 h-3" />
                    </button>
                  </div>
                  <div class="flex items-center gap-1.5 text-[11px]">
                    <span class="font-mono text-emerald-400 px-1 rounded bg-emerald-500/10 border border-emerald-500/20 text-[10px]">
                      {{ formatRelationLabel(rel.relation_type) }}
                    </span>
                    <span class="text-ink-400 truncate">{{ rel.description || '无备注说明' }}</span>
                  </div>
                </div>

                <div v-if="nodeRelations.length === 0" class="text-center py-4 text-ink-500 text-xs">
                  暂无已建立的关联关系
                </div>
              </div>
            </div>

            <!-- 跳转设定集编辑 -->
            <div class="pt-3 border-t border-atelier-750">
              <button 
                @click="jumpToCodexEdit(selectedNode.raw)" 
                class="w-full py-1.5 px-3 bg-atelier-850 hover:bg-atelier-800 text-ink-200 rounded-lg text-xs font-medium border border-atelier-750 transition flex items-center justify-center gap-1.5 cursor-pointer">
                <BookOpen class="w-3.5 h-3.5" />
                <span>在设定集中查看/编辑完整设定</span>
              </button>
            </div>
          </div>

          <!-- 连线详情展示 -->
          <div v-else-if="selectedLink" class="space-y-4">
            <div class="flex items-start justify-between border-b border-atelier-750 pb-3">
              <div>
                <h3 class="text-sm font-serif font-bold text-ink-50">关系脉络</h3>
                <span class="text-[10px] font-mono text-brand-amber">定向实体关联</span>
              </div>
              <button 
                @click="selectedLink = null" 
                class="p-1 text-ink-400 hover:text-ink-100 rounded transition cursor-pointer">
                <X class="w-4 h-4" />
              </button>
            </div>

            <div class="p-3 bg-atelier-950 rounded-xl border border-atelier-800 space-y-3 text-xs">
              <div class="flex items-center justify-between text-ink-100 font-serif">
                <span class="font-bold text-brand-amber">【{{ getEntityName(selectedLink.source_entry_id) }}】</span>
                <ArrowRight class="w-3.5 h-3.5 text-ink-400" />
                <span class="font-bold text-emerald-400">【{{ getEntityName(selectedLink.target_entry_id) }}】</span>
              </div>

              <div>
                <span class="text-[10px] text-ink-400 block mb-1">关系类别：</span>
                <span class="px-2 py-0.5 rounded bg-brand-amber/15 text-brand-amber border border-brand-amber/30 text-xs font-mono font-bold">
                  {{ formatRelationLabel(selectedLink.relation_type) }}
                </span>
              </div>

              <div>
                <span class="text-[10px] text-ink-400 block mb-1">背景说明：</span>
                <p class="text-ink-300 font-serif leading-relaxed">
                  {{ selectedLink.description || '暂无详细背景说明' }}
                </p>
              </div>
            </div>

            <button 
              @click="deleteRelation(selectedLink.id)" 
              class="w-full py-1.5 px-3 bg-rose-500/10 hover:bg-rose-500/20 text-rose-400 border border-rose-500/30 rounded-lg text-xs font-medium transition flex items-center justify-center gap-1.5 cursor-pointer">
              <Trash2 class="w-3.5 h-3.5" />
              <span>删除此关联记录</span>
            </button>
          </div>
        </div>
      </transition>
    </div>

    <!-- 手动新增关联模态弹窗 -->
    <div 
      v-if="showCreateRelationModal" 
      class="fixed inset-0 bg-black/80 backdrop-blur-sm flex items-center justify-center p-4 z-50">
      <div class="bg-atelier-900 border border-atelier-750 rounded-2xl max-w-md w-full p-5 space-y-4 shadow-2xl animate-fade-in">
        <div class="flex items-center justify-between border-b border-atelier-750 pb-3">
          <div class="flex items-center gap-2">
            <Share2 class="w-4 h-4 text-brand-amber" />
            <h3 class="text-sm font-serif font-bold text-ink-50">新增实体关联</h3>
          </div>
          <button 
            @click="showCreateRelationModal = false" 
            class="text-ink-400 hover:text-ink-100 p-1 rounded transition cursor-pointer">
            <X class="w-4 h-4" />
          </button>
        </div>

        <div class="space-y-3 text-xs">
          <div>
            <label class="text-[11px] text-ink-400 block mb-1">源实体 (主体)：</label>
            <select 
              v-model="createRelationForm.source_entry_id"
              class="w-full bg-atelier-950 border border-atelier-750 rounded-lg px-2.5 py-1.5 text-xs text-ink-100 focus:outline-none focus:border-brand-amber/60">
              <option value="">-- 选择源实体 --</option>
              <option v-for="e in state.codexEntries" :key="e.id" :value="e.id">
                {{ e.name }} ({{ formatCategoryName(e.category) }})
              </option>
            </select>
          </div>

          <div>
            <label class="text-[11px] text-ink-400 block mb-1">目标实体 (客体)：</label>
            <select 
              v-model="createRelationForm.target_entry_id"
              class="w-full bg-atelier-950 border border-atelier-750 rounded-lg px-2.5 py-1.5 text-xs text-ink-100 focus:outline-none focus:border-brand-amber/60">
              <option value="">-- 选择目标实体 --</option>
              <option 
                v-for="e in targetCandidateEntries" 
                :key="e.id" 
                :value="e.id">
                {{ e.name }} ({{ formatCategoryName(e.category) }})
              </option>
            </select>
          </div>

          <div>
            <label class="text-[11px] text-ink-400 block mb-1">关系类别：</label>
            <div class="grid grid-cols-3 gap-1.5 mb-1.5">
              <button 
                v-for="relPreset in relationPresets" 
                :key="relPreset"
                type="button"
                @click="createRelationForm.relation_type = relPreset"
                :class="createRelationForm.relation_type === relPreset ? 'bg-brand-amber/20 text-brand-amber border-brand-amber/40' : 'bg-atelier-850 text-ink-400 border-atelier-750'"
                class="px-2 py-1 rounded border text-[11px] font-mono transition cursor-pointer">
                {{ relPreset }}
              </button>
            </div>
            <input 
              v-model="createRelationForm.relation_type" 
              placeholder="自定义关系标签 (如: 执掌 / 誓约守护)"
              class="w-full bg-atelier-950 border border-atelier-750 rounded-lg px-2.5 py-1.5 text-xs text-ink-100 focus:outline-none focus:border-brand-amber/60 font-mono">
          </div>

          <div>
            <label class="text-[11px] text-ink-400 block mb-1">关系背景说明：</label>
            <textarea 
              v-model="createRelationForm.description" 
              rows="3" 
              placeholder="例如：生死大仇，三年前师门被灭之主谋..."
              class="w-full bg-atelier-950 border border-atelier-750 rounded-lg p-2.5 text-xs text-ink-100 font-serif focus:outline-none focus:border-brand-amber/60 resize-none"></textarea>
          </div>
        </div>

        <div class="flex items-center justify-end gap-2 pt-2 border-t border-atelier-750">
          <button 
            @click="showCreateRelationModal = false" 
            class="px-3.5 py-1.5 bg-atelier-850 hover:bg-atelier-800 text-ink-300 text-xs rounded-lg transition cursor-pointer">
            取消
          </button>
          <button 
            @click="submitCreateRelation" 
            :disabled="!createRelationForm.source_entry_id || !createRelationForm.target_entry_id || !createRelationForm.relation_type.trim()"
            class="px-4 py-1.5 bg-brand-amber hover:bg-brand-amber-hover text-atelier-950 font-bold text-xs rounded-lg shadow-amber-glow transition disabled:opacity-50 cursor-pointer">
            建立关联
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, watch } from 'vue';
import { state, actions } from '../stores/appState';
import { 
  Network, 
  Share2, 
  Sparkles, 
  Plus, 
  RotateCcw, 
  Search, 
  ZoomIn, 
  ZoomOut, 
  X, 
  Trash2, 
  ArrowRight, 
  BookOpen 
} from 'lucide-vue-next';

// 画布视图状态
const containerRef = ref(null);
const zoom = ref(1.0);
const pan = ref({ x: 300, y: 220 });
const isPanning = ref(false);
const startPanPos = ref({ x: 0, y: 0 });

// 交互过滤与选中状态
const searchQuery = ref('');
const selectedCategory = ref('ALL');
const selectedNode = ref(null);
const selectedLink = ref(null);
const hoveredNode = ref(null);
const isExtracting = ref(false);

// 拖拽节点状态
const draggingNode = ref(null);
const dragStartMousePos = ref({ x: 0, y: 0 });
const dragStartNodePos = ref({ x: 0, y: 0 });

// 手动创建关系弹窗状态
const showCreateRelationModal = ref(false);
const createRelationForm = ref({
  source_entry_id: '',
  target_entry_id: '',
  relation_type: 'ALLY',
  description: '',
});

const relationPresets = ['宿敌', '盟友', '师徒', '从属', '执掌', '亲族'];

// 分类配置与配色
const categoryColors = {
  CHARACTER: '#e0a96d', // Amber
  FACTION: '#e06c75',   // Crimson
  ITEM: '#98c379',      // Emerald
  LOCATION: '#61afef',  // Sky Blue
  LORE: '#c678dd',      // Purple
};

function formatCategoryName(cat) {
  const map = {
    CHARACTER: '角色',
    FACTION: '势力',
    ITEM: '灵宝',
    LOCATION: '场景',
    LORE: '公理',
  };
  return map[cat] || cat;
}

function formatRelationLabel(type) {
  const map = {
    NEMESIS: '宿敌',
    ALLY: '盟友',
    MASTER_DISCIPLE: '师徒',
    KINSHIP: '亲族',
    CRUSH: '爱慕',
    SERVANT: '从属',
    OWNER: '执掌',
  };
  return map[type] || type;
}

const categoryFilters = computed(() => {
  const entries = state.codexEntries || [];
  return [
    { key: 'ALL', label: '全部', color: '#f59e0b', count: entries.length },
    { key: 'CHARACTER', label: '角色', color: categoryColors.CHARACTER, count: entries.filter(e => e.category === 'CHARACTER').length },
    { key: 'FACTION', label: '势力', color: categoryColors.FACTION, count: entries.filter(e => e.category === 'FACTION').length },
    { key: 'ITEM', label: '灵宝', color: categoryColors.ITEM, count: entries.filter(e => e.category === 'ITEM').length },
    { key: 'LOCATION', label: '场景', color: categoryColors.LOCATION, count: entries.filter(e => e.category === 'LOCATION').length },
    { key: 'LORE', label: '公理', color: categoryColors.LORE, count: entries.filter(e => e.category === 'LORE').length },
  ];
});

// 图节点内部数据字典 (记录 x, y 坐标与力导向物理速度)
const nodePositions = ref(new Map());

// 初始化或更新节点坐标分布 (以主角为中心径向排布)
function syncNodePositions() {
  const entries = state.codexEntries || [];
  const cx = 450;
  const cy = 320;
  const radius = Math.min(320, Math.max(160, entries.length * 36));

  entries.forEach((e, idx) => {
    if (!nodePositions.value.has(e.id)) {
      const isProtagonist = e.name === '主角' || e.id.includes('_pro');
      if (isProtagonist) {
        // 主角直接锚定在核心正中央
        nodePositions.value.set(e.id, {
          x: cx,
          y: cy,
          vx: 0,
          vy: 0,
        });
      } else {
        // 其余配角、宗门与势力沿外层环形放射排列
        const angle = (idx / (entries.length || 1)) * 2 * Math.PI;
        nodePositions.value.set(e.id, {
          x: cx + Math.cos(angle) * radius + (Math.random() - 0.5) * 40,
          y: cy + Math.sin(angle) * radius + (Math.random() - 0.5) * 40,
          vx: 0,
          vy: 0,
        });
      }
    }
  });

  // 运行微物理模拟让节点松散分离并强制避让重叠
  runForceSimulation(60);
}

// 轻量级工业级高防重叠力导向模拟 (满足奥卡姆剃刀原则，零沉重依赖)
function runForceSimulation(iterations = 60) {
  const entries = state.codexEntries || [];
  const relations = state.codexRelations || [];
  if (entries.length === 0) return;

  const kRepel = 7500; // 提升排斥常数
  const kAttract = 0.035; // 连线引力常数
  const damp = 0.82;
  const minSafeDistance = 115; // 节点硬碰撞避让距离 (圆半径 24 + 外光环 + 文字标签)

  for (let it = 0; it < iterations; it++) {
    // 1. 节点间排斥与刚体碰撞分离 (Hard Collision Avoidance)
    for (let i = 0; i < entries.length; i++) {
      const p1 = nodePositions.value.get(entries[i].id);
      if (!p1) continue;
      for (let j = i + 1; j < entries.length; j++) {
        const p2 = nodePositions.value.get(entries[j].id);
        if (!p2) continue;
        const dx = p1.x - p2.x;
        const dy = p1.y - p2.y;
        const distSq = dx * dx + dy * dy || 0.01;
        const dist = Math.sqrt(distSq);

        // A. 弹性排斥场
        if (dist < 450) {
          const force = kRepel / distSq;
          const fx = (dx / (dist || 1)) * force;
          const fy = (dy / (dist || 1)) * force;
          p1.vx += fx;
          p1.vy += fy;
          p2.vx -= fx;
          p2.vy -= fy;
        }

        // B. 刚体硬碰撞隔离 (保证任意两节点绝对不重叠)
        if (dist < minSafeDistance) {
          const overlap = (minSafeDistance - dist) * 0.5;
          const nx = dx / (dist || 0.001);
          const ny = dy / (dist || 0.001);
          p1.x += nx * overlap;
          p1.y += ny * overlap;
          p2.x -= nx * overlap;
          p2.y -= ny * overlap;
        }
      }
    }

    // 2. 连线引力
    for (const rel of relations) {
      const p1 = nodePositions.value.get(rel.source_entry_id);
      const p2 = nodePositions.value.get(rel.target_entry_id);
      if (p1 && p2) {
        const dx = p2.x - p1.x;
        const dy = p2.y - p1.y;
        p1.vx += dx * kAttract;
        p1.vy += dy * kAttract;
        p2.vx -= dx * kAttract;
        p2.vy -= dy * kAttract;
      }
    }

    // 3. 向心力与位置步进
    for (const e of entries) {
      const p = nodePositions.value.get(e.id);
      if (!p) continue;
      const isProtagonist = e.name === '主角' || e.id.includes('_pro');
      const targetCenterX = 450;
      const targetCenterY = 320;

      if (isProtagonist) {
        // 主角作为全书核心锚点，向心力更强
        p.vx += (targetCenterX - p.x) * 0.04;
        p.vy += (targetCenterY - p.y) * 0.04;
      } else {
        p.vx += (targetCenterX - p.x) * 0.008;
        p.vy += (targetCenterY - p.y) * 0.008;
      }

      p.vx *= damp;
      p.vy *= damp;
      p.x += p.vx;
      p.y += p.vy;
    }
  }
}

// 计算过滤后的有效节点
const validNodes = computed(() => {
  const q = searchQuery.value.trim().toLowerCase();
  const cat = selectedCategory.value;

  return (state.codexEntries || [])
    .filter(e => {
      if (cat !== 'ALL' && e.category !== cat) return false;
      if (q && !e.name.toLowerCase().includes(q) && !(e.summary || '').toLowerCase().includes(q)) {
        return false;
      }
      return true;
    })
    .map(e => {
      const pos = nodePositions.value.get(e.id) || { x: 450, y: 320 };
      const isProtagonist = e.name === '主角' || e.id.includes('_pro');
      return {
        id: e.id,
        name: e.name,
        category: e.category,
        isProtagonist,
        color: isProtagonist ? '#fbbf24' : (e.color_tag || categoryColors[e.category] || '#f59e0b'),
        x: pos.x,
        y: pos.y,
        raw: e,
      };
    });
});

// 计算关联线
const validLinks = computed(() => {
  const nodeMap = new Map(validNodes.value.map(n => [n.id, n]));
  return (state.codexRelations || [])
    .filter(r => nodeMap.has(r.source_entry_id) && nodeMap.has(r.target_entry_id))
    .map(r => {
      const source = nodeMap.get(r.source_entry_id);
      const target = nodeMap.get(r.target_entry_id);
      return {
        ...r,
        source,
        target,
      };
    });
});

// 计算选定实体的所有关系
const nodeRelations = computed(() => {
  if (!selectedNode.value) return [];
  const id = selectedNode.value.id;
  return (state.codexRelations || []).filter(r => r.source_entry_id === id || r.target_entry_id === id);
});

// 动态计算连线曲率 (当存在双向关联时大幅弯曲，避免重合)
function getLinkCurvature(link) {
  const relations = state.codexRelations || [];
  const hasReverse = relations.some(r => 
    r.id !== link.id && 
    r.source_entry_id === link.target_entry_id && 
    r.target_entry_id === link.source_entry_id
  );
  return hasReverse ? 0.22 : 0.08;
}

// 辅助连线几何计算
function computeLinkPath(link) {
  const x1 = link.source.x;
  const y1 = link.source.y;
  const x2 = link.target.x;
  const y2 = link.target.y;

  const dx = x2 - x1;
  const dy = y2 - y1;
  const curve = getLinkCurvature(link);
  const cx = (x1 + x2) / 2 - dy * curve;
  const cy = (y1 + y2) / 2 + dx * curve;

  return `M ${x1} ${y1} Q ${cx} ${cy} ${x2} ${y2}`;
}

function computeLinkMidpoint(link) {
  const x1 = link.source.x;
  const y1 = link.source.y;
  const x2 = link.target.x;
  const y2 = link.target.y;
  const dx = x2 - x1;
  const dy = y2 - y1;
  const curve = getLinkCurvature(link);
  return {
    x: (x1 + x2) / 2 - dy * (curve * 0.5),
    y: (y1 + y2) / 2 + dx * (curve * 0.5),
  };
}

function getEntityName(id) {
  const e = (state.codexEntries || []).find(x => x.id === id);
  return e ? e.name : '未知实体';
}

// 高亮与暗淡判断
function isNodeSelected(node) {
  return selectedNode.value?.id === node.id;
}

function isNodeHighlighted(node) {
  if (selectedNode.value?.id === node.id) return true;
  if (selectedLink.value) {
    return selectedLink.value.source_entry_id === node.id || selectedLink.value.target_entry_id === node.id;
  }
  return false;
}

function isNodeDimmed(node) {
  if (selectedNode.value) {
    if (selectedNode.value.id === node.id) return false;
    const isNeighbor = (state.codexRelations || []).some(r => 
      (r.source_entry_id === selectedNode.value.id && r.target_entry_id === node.id) ||
      (r.target_entry_id === selectedNode.value.id && r.source_entry_id === node.id)
    );
    return !isNeighbor;
  }
  if (selectedLink.value) {
    return selectedLink.value.source_entry_id !== node.id && selectedLink.value.target_entry_id !== node.id;
  }
  return false;
}

function isLinkHighlighted(link) {
  if (selectedLink.value?.id === link.id) return true;
  if (selectedNode.value) {
    return link.source_entry_id === selectedNode.value.id || link.target_entry_id === selectedNode.value.id;
  }
  return false;
}

function isLinkDimmed(link) {
  if (selectedNode.value) {
    return link.source_entry_id !== selectedNode.value.id && link.target_entry_id !== selectedNode.value.id;
  }
  if (selectedLink.value) {
    return selectedLink.value.id !== link.id;
  }
  return false;
}

// 选中事件
function selectNode(node) {
  selectedLink.value = null;
  selectedNode.value = node;
}

function selectLink(link) {
  selectedNode.value = null;
  selectedLink.value = link;
}

// 画布平移与缩放
function handleWheel(e) {
  const zoomFactor = e.deltaY < 0 ? 1.08 : 0.92;
  const newZoom = Math.min(2.5, Math.max(0.3, zoom.value * zoomFactor));
  zoom.value = newZoom;
}

function zoomIn() {
  zoom.value = Math.min(2.5, zoom.value * 1.2);
}

function zoomOut() {
  zoom.value = Math.max(0.3, zoom.value * 0.8);
}

function resetView() {
  zoom.value = 1.0;
  pan.value = { x: 300, y: 220 };
  syncNodePositions();
}

function handleCanvasMouseDown(e) {
  if (e.target !== containerRef.value && !e.target.closest('svg')) return;
  // 背景拖动画布
  isPanning.value = true;
  startPanPos.value = { x: e.clientX - pan.value.x, y: e.clientY - pan.value.y };

  window.addEventListener('mousemove', onCanvasMouseMove);
  window.addEventListener('mouseup', onCanvasMouseUp);
}

function onCanvasMouseMove(e) {
  if (isPanning.value) {
    pan.value = {
      x: e.clientX - startPanPos.value.x,
      y: e.clientY - startPanPos.value.y,
    };
  }
}

function onCanvasMouseUp() {
  isPanning.value = false;
  window.removeEventListener('mousemove', onCanvasMouseMove);
  window.removeEventListener('mouseup', onCanvasMouseUp);
}

// 节点拖拽
function startDragNode(e, node) {
  draggingNode.value = node;
  dragStartMousePos.value = { x: e.clientX, y: e.clientY };
  dragStartNodePos.value = { x: node.x, y: node.y };

  window.addEventListener('mousemove', onNodeMouseMove);
  window.addEventListener('mouseup', onNodeMouseUp);
}

function onNodeMouseMove(e) {
  if (!draggingNode.value) return;
  const dx = (e.clientX - dragStartMousePos.value.x) / zoom.value;
  const dy = (e.clientY - dragStartMousePos.value.y) / zoom.value;
  const pos = nodePositions.value.get(draggingNode.value.id);
  if (pos) {
    pos.x = dragStartNodePos.value.x + dx;
    pos.y = dragStartNodePos.value.y + dy;
    pos.vx = 0;
    pos.vy = 0;
  }
}

function onNodeMouseUp() {
  draggingNode.value = null;
  window.removeEventListener('mousemove', onNodeMouseMove);
  window.removeEventListener('mouseup', onNodeMouseUp);
}

// 手动创建关系
const targetCandidateEntries = computed(() => {
  return (state.codexEntries || []).filter(e => e.id !== createRelationForm.value.source_entry_id);
});

function openCreateRelationModal(sourceId = '') {
  createRelationForm.value = {
    source_entry_id: sourceId || (state.codexEntries[0]?.id || ''),
    target_entry_id: '',
    relation_type: 'ALLY',
    description: '',
  };
  showCreateRelationModal.value = true;
}

async function submitCreateRelation() {
  if (!createRelationForm.value.source_entry_id || !createRelationForm.value.target_entry_id) return;
  await actions.createCodexRelation({
    source_entry_id: createRelationForm.value.source_entry_id,
    target_entry_id: createRelationForm.value.target_entry_id,
    relation_type: createRelationForm.value.relation_type.trim(),
    description: createRelationForm.value.description.trim(),
  });
  showCreateRelationModal.value = false;
}

async function deleteRelation(relId) {
  const confirmed = await actions.confirm({
    title: '删除实体关联',
    message: '确定要删除这条实体关联线吗？',
    type: 'warning',
    confirmText: '删除',
  });
  if (confirmed) {
    await actions.deleteCodexRelation(relId);
    if (selectedLink.value?.id === relId) {
      selectedLink.value = null;
    }
  }
}

async function runAIExtractRelations() {
  isExtracting.value = true;
  try {
    await actions.extractCodexRelations();
    runForceSimulation(30);
  } finally {
    isExtracting.value = false;
  }
}

function jumpToCodexEdit(rawEntry) {
  state.editingCodexEntry = { ...rawEntry };
  state.activeTab = 'codex';
  state.showCodexModal = true;
}

watch(() => state.codexEntries, () => {
  syncNodePositions();
}, { deep: true });

onMounted(async () => {
  if (state.currentProject) {
    await actions.loadCodexEntries();
    await actions.loadCodexRelations();
  }
  syncNodePositions();
});

onUnmounted(() => {
  window.removeEventListener('mousemove', onCanvasMouseMove);
  window.removeEventListener('mouseup', onCanvasMouseUp);
  window.removeEventListener('mousemove', onNodeMouseMove);
  window.removeEventListener('mouseup', onNodeMouseUp);
});
</script>
