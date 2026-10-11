<template>
  <div class="flex-1 flex flex-col h-full overflow-hidden bg-atelier-950 select-none">
    <!-- 未选项目空提示 -->
    <div v-if="!state.currentProject" class="flex-1 flex items-center justify-center p-8 text-center text-ink-400">
      <div class="space-y-3">
        <NetworkIcon class="w-10 h-10 mx-auto text-ink-500 opacity-50" />
        <p class="text-xs">请先在侧边栏选择或创建一部小说作品</p>
      </div>
    </div>

    <!-- 正常图谱工作区 -->
    <div v-else class="flex-1 flex flex-col h-full overflow-hidden">
      <!-- 头部工具栏 -->
      <div class="h-14 px-6 border-b border-atelier-750 flex items-center justify-between shrink-0 bg-atelier-900/60 backdrop-blur-sm z-10">
        <div class="flex items-center gap-3 shrink-0">
          <div class="w-8 h-8 rounded-lg bg-brand-amber/10 border border-brand-amber/25 flex items-center justify-center text-brand-amber shadow-amber-glow shrink-0">
            <NetworkIcon class="w-4 h-4" />
          </div>
          <div class="min-w-0">
            <div class="flex items-center gap-2">
              <h2 class="text-sm font-serif font-bold text-ink-50 tracking-wide whitespace-nowrap">关系图谱</h2>
              <span class="text-[10px] font-mono px-1.5 py-0.2 rounded bg-atelier-800 text-ink-300 border border-atelier-750 whitespace-nowrap">
                {{ validNodesCount }} 实体 · {{ validLinksCount }} 关联
              </span>
            </div>
            <p class="text-[11px] text-ink-400 hidden lg:block whitespace-nowrap">基于工业级力导向拓扑引擎呈现的实体错综关联图谱。</p>
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
            title="自适应居中画布视角">
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
            :disabled="isExtracting || validNodesCount < 2"
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
          支持拖拽节点 · 滚轮缩放 · 双击节点聚光聚焦 · 点击查看详情
        </div>
      </div>

      <!-- 图谱交互主画布容器 -->
      <div class="flex-1 relative overflow-hidden bg-atelier-950">
        <!-- 背景网格微点 -->
        <div 
          class="absolute inset-0 pointer-events-none opacity-20"
          :style="{
            backgroundImage: 'radial-gradient(circle, #888 1px, transparent 1px)',
            backgroundSize: '24px 24px',
          }">
        </div>

        <!-- vis-network 挂载节点 (绝对定位满格铺满，杜绝任何 0 高度折叠) -->
        <div 
          ref="networkContainer" 
          class="absolute inset-0 w-full h-full cursor-grab active:cursor-grabbing outline-none">
        </div>

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

        <!-- 真正暂无数据时的空白引导浮层 (仅当全库实体数为 0 时呈现) -->
        <div 
          v-if="!allGraphEntries || allGraphEntries.length === 0" 
          class="absolute inset-0 flex items-center justify-center p-6 pointer-events-none z-10">
          <div class="p-8 bg-atelier-900/95 border border-atelier-750 rounded-2xl max-w-md text-center space-y-4 shadow-2xl pointer-events-auto backdrop-blur-md">
            <div class="w-12 h-12 rounded-full bg-brand-amber/10 border border-brand-amber/20 flex items-center justify-center text-brand-amber mx-auto shadow-amber-glow">
              <NetworkIcon class="w-6 h-6" />
            </div>
            <div>
              <h3 class="text-sm font-serif font-bold text-ink-100">暂无图谱实体</h3>
              <p class="text-xs text-ink-400 mt-1 leading-relaxed">
                尚未在设定集中录入人物、势力、灵宝或地理设定。请先在设定集中创建词条，或让 AI 智能构思。
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
          v-else-if="validLinksCount === 0" 
          class="absolute top-4 left-1/2 -translate-x-1/2 bg-atelier-900/95 border border-brand-amber/30 rounded-xl px-4 py-2.5 flex items-center gap-3 shadow-amber-glow backdrop-blur-md z-10">
          <Sparkles class="w-4 h-4 text-brand-amber shrink-0 animate-pulse" />
          <span class="text-xs text-ink-200">
            已发现 {{ validNodesCount }} 个实体，点击右侧按钮让 AI 智能推演它们之间的爱恨纠葛与阵营纽带：
          </span>
          <button 
            @click="runAIExtractRelations" 
            :disabled="isExtracting"
            class="px-3 py-1 bg-brand-amber hover:bg-brand-amber-hover text-atelier-950 text-xs font-bold rounded-md shadow-amber-glow transition cursor-pointer disabled:opacity-50 shrink-0">
            {{ isExtracting ? '推演中...' : '立即推演' }}
          </button>
        </div>

        <!-- 右侧实体 / 关系详情抽屉 -->
        <GraphDetailDrawer
          :selected-node="selectedNode"
          :selected-link="selectedLink"
          :node-relations="nodeRelations"
          :close-detail-drawer="closeDetailDrawer"
          :delete-relation="deleteRelation"
          :format-relation-label="formatRelationLabel"
          :get-entity-name="getEntityName"
          :open-create-relation-modal="openCreateRelationModal"
          :format-category-name="formatCategoryName"
        />
      </div>
    </div>

    <!-- 手动新增关联模态弹窗 -->
    <CreateRelationModal
      :show-create-relation-modal="showCreateRelationModal"
      :source-id="createRelationSourceId"
      :all-graph-entries="allGraphEntries"
      :format-category-name="formatCategoryName"
      @close="showCreateRelationModal = false"
      @submitted="syncGraphData()"
    />
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, watch, nextTick } from 'vue';
import { state, actions } from '../stores/appState';
import { 
  Network as NetworkIcon, 
  Sparkles, 
  Plus, 
  RotateCcw, 
  Search, 
  ZoomIn, 
  ZoomOut, 
  BookOpen 
} from 'lucide-vue-next';
import { Network } from 'vis-network/esnext';
import { DataSet } from 'vis-data/esnext';
import 'vis-network/styles/vis-network.css';
import GraphDetailDrawer from '../components/graph/GraphDetailDrawer.vue';
import CreateRelationModal from '../components/graph/CreateRelationModal.vue';

// DOM 容器与状态
const networkContainer = ref(null);
const zoom = ref(1.0);
const searchQuery = ref('');
const selectedCategory = ref('ALL');
const selectedNode = ref(null);
const selectedLink = ref(null);
const isExtracting = ref(false);

// 手动创建关系弹窗状态
const showCreateRelationModal = ref(false);
const createRelationSourceId = ref('');

// 分类色彩体系 (Atelier 暗黑调色盘)
const categoryColors = {
  CHARACTER: '#e0a96d', // 金琥珀
  FACTION: '#e06c75',   // 赤朱红
  ITEM: '#98c379',      // 翡翠青
  LOCATION: '#61afef',  // 苍穹蓝
  LORE: '#c678dd',      // 紫虚玄
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

// 主角已统一在设定集 (Codex) 中持久化存储，直接取自 codexEntries
const allGraphEntries = computed(() => {
  return (state.codexEntries || []).map(e => ({
    ...e,
    isProtagonistNode: Boolean(
      e.archetype === 'PROTAGONIST' ||
      e.id?.endsWith('_pro') ||
      e.name === '主角'
    ),
  }));
});

const categoryFilters = computed(() => {
  const entries = allGraphEntries.value;
  return [
    { key: 'ALL', label: '全部', color: '#f59e0b', count: entries.length },
    { key: 'CHARACTER', label: '角色', color: categoryColors.CHARACTER, count: entries.filter(e => e.category === 'CHARACTER').length },
    { key: 'FACTION', label: '势力', color: categoryColors.FACTION, count: entries.filter(e => e.category === 'FACTION').length },
    { key: 'ITEM', label: '灵宝', color: categoryColors.ITEM, count: entries.filter(e => e.category === 'ITEM').length },
    { key: 'LOCATION', label: '场景', color: categoryColors.LOCATION, count: entries.filter(e => e.category === 'LOCATION').length },
    { key: 'LORE', label: '公理', color: categoryColors.LORE, count: entries.filter(e => e.category === 'LORE').length },
  ];
});

// 计算过滤后显示的实体与关联（保证纯响应式，不依赖原生库生命周期错位）
const filteredNodes = computed(() => {
  const q = searchQuery.value.trim().toLowerCase();
  const cat = selectedCategory.value;
  return allGraphEntries.value.filter(e => {
    if (cat !== 'ALL' && e.category !== cat) return false;
    if (q && !(e.name || '').toLowerCase().includes(q) && !(e.summary || '').toLowerCase().includes(q)) {
      return false;
    }
    return true;
  });
});

const filteredLinks = computed(() => {
  const nodeMap = new Set(filteredNodes.value.map(n => n.id));
  return (state.codexRelations || []).filter(r => 
    nodeMap.has(r.source_entry_id) && nodeMap.has(r.target_entry_id)
  );
});

const validNodesCount = computed(() => filteredNodes.value.length);
const validLinksCount = computed(() => filteredLinks.value.length);

// vis-network 原生实例与数据集 (脱离 Vue 深度 Proxy 封装以保障极致渲染性能)
let networkInstance = null;
let nodesDataSet = null;
let edgesDataSet = null;

// 初始化 vis-network 画布
function initNetwork() {
  if (!networkContainer.value) return;
  if (networkInstance) return;

  nodesDataSet = new DataSet([]);
  edgesDataSet = new DataSet([]);

  const options = {
    autoResize: true,
    height: '100%',
    width: '100%',
    nodes: {
      shape: 'box',
      shapeProperties: {
        borderRadius: 6,
      },
      margin: { top: 8, bottom: 8, left: 14, right: 14 },
      borderWidth: 1.5,
      borderWidthSelected: 2.5,
      font: {
        color: '#f4f4f5',
        size: 12,
        face: 'Noto Serif SC, PingFang SC, sans-serif',
      },
      shadow: {
        enabled: true,
        color: 'rgba(0, 0, 0, 0.45)',
        size: 8,
        x: 0,
        y: 2,
      },
    },
    edges: {
      arrows: {
        to: { enabled: true, scaleFactor: 0.65 },
      },
      color: {
        color: '#52525b',
        highlight: '#f59e0b',
        hover: '#a1a1aa',
        inherit: false,
      },
      font: {
        color: '#a1a1aa',
        size: 10,
        face: 'ui-monospace, SFMono-Regular, monospace',
        background: '#18181b',
        strokeWidth: 0,
        align: 'middle',
      },
      smooth: {
        type: 'curvedCW',
        roundness: 0.15,
      },
      width: 1.5,
      selectionWidth: 2.5,
      hoverWidth: 2,
    },
    physics: {
      solver: 'forceAtlas2Based',
      forceAtlas2Based: {
        gravitationalConstant: -60,
        centralGravity: 0.012,
        springLength: 130,
        springConstant: 0.08,
        damping: 0.45,
        avoidOverlap: 1.0, // 工业级防节点重叠约束 (杜绝任何重叠)
      },
      stabilization: {
        enabled: true,
        iterations: 120,
        updateInterval: 25,
      },
    },
    interaction: {
      hover: true,
      hoverConnectedEdges: true,
      selectConnectedEdges: true,
      tooltipDelay: 200,
      zoomView: true,
      dragView: true,
      navigationButtons: false,
      keyboard: false,
    },
  };

  networkInstance = new Network(
    networkContainer.value, 
    { nodes: nodesDataSet, edges: edgesDataSet }, 
    options
  );

  // 事件监听：单击节点或边
  networkInstance.on('click', (params) => {
    if (params.nodes.length > 0) {
      const nodeId = params.nodes[0];
      const nodeData = nodesDataSet.get(nodeId);
      selectedNode.value = nodeData || null;
      selectedLink.value = null;
    } else if (params.edges.length > 0) {
      const edgeId = params.edges[0];
      const edgeData = edgesDataSet.get(edgeId);
      selectedLink.value = edgeData?.rawRelation || null;
      selectedNode.value = null;
    } else {
      closeDetailDrawer();
    }
  });

  // 双击节点：平滑聚光聚焦并适度放大
  networkInstance.on('doubleClick', (params) => {
    if (params.nodes.length > 0) {
      const nodeId = params.nodes[0];
      networkInstance.focus(nodeId, {
        scale: 1.4,
        animation: { duration: 400, easingFunction: 'easeInOutQuad' },
      });
    }
  });

  // 缩放事件监听
  networkInstance.on('zoom', (params) => {
    zoom.value = params.scale;
  });
}

// 增量同步设定集实体与关联数据至 vis-network
function syncGraphData() {
  if (!nodesDataSet || !edgesDataSet) return;

  const currentNodes = filteredNodes.value;
  const currentNodeIds = new Set(currentNodes.map(n => n.id));
  const existingNodeIds = new Set(nodesDataSet.getIds());

  // 1. 清理被过滤或删除的节点
  for (const id of existingNodeIds) {
    if (!currentNodeIds.has(id)) {
      nodesDataSet.remove(id);
    }
  }

  // 2. 增量更新或追加节点
  const nodesToUpsert = [];
  for (const e of currentNodes) {
    const isProtagonist = Boolean(
      e.isProtagonistNode || 
      e.archetype === 'PROTAGONIST' || 
      e.name === '主角' || 
      e.id?.includes('_pro')
    );

    const catColor = isProtagonist ? '#fbbf24' : (e.color_tag || categoryColors[e.category] || '#f59e0b');

    const nodeItem = {
      id: e.id,
      label: e.name,
      name: e.name,
      category: e.category,
      isProtagonist,
      color: {
        background: isProtagonist ? '#272015' : '#18181b',
        border: catColor,
        highlight: {
          background: isProtagonist ? '#362a19' : '#27272a',
          border: '#f59e0b',
        },
        hover: {
          background: isProtagonist ? '#362a19' : '#27272a',
          border: catColor,
        },
      },
      borderWidth: isProtagonist ? 2.5 : 1.5,
      borderWidthSelected: isProtagonist ? 3.5 : 2.5,
      font: {
        color: isProtagonist ? '#fef08a' : '#f4f4f5',
        size: isProtagonist ? 13 : 12,
        face: 'Noto Serif SC, PingFang SC, sans-serif',
        bold: isProtagonist,
      },
      shape: 'box',
      shapeProperties: {
        borderRadius: isProtagonist ? 8 : 6,
      },
      margin: {
        top: isProtagonist ? 9 : 7,
        bottom: isProtagonist ? 9 : 7,
        left: isProtagonist ? 16 : 12,
        right: isProtagonist ? 16 : 12,
      },
      shadow: {
        enabled: true,
        color: isProtagonist ? 'rgba(251, 191, 36, 0.35)' : 'rgba(0, 0, 0, 0.45)',
        size: isProtagonist ? 12 : 6,
        x: 0,
        y: 2,
      },
      title: `${e.name} [${formatCategoryName(e.category)}]\n${e.summary || ''}`,
      raw: e,
      summary: e.summary,
    };
    nodesToUpsert.push(nodeItem);
  }
  nodesDataSet.update(nodesToUpsert);

  // 3. 同步关联连线
  const currentEdges = filteredLinks.value;
  const currentEdgeIds = new Set(currentEdges.map(r => r.id));
  const existingEdgeIds = new Set(edgesDataSet.getIds());

  for (const id of existingEdgeIds) {
    if (!currentEdgeIds.has(id)) {
      edgesDataSet.remove(id);
    }
  }

  const edgesToUpsert = [];
  for (const r of currentEdges) {
    edgesToUpsert.push({
      id: r.id,
      from: r.source_entry_id,
      to: r.target_entry_id,
      label: formatRelationLabel(r.relation_type),
      relation_type: r.relation_type,
      description: r.description,
      source_entry_id: r.source_entry_id,
      target_entry_id: r.target_entry_id,
      rawRelation: r,
      arrows: {
        to: { enabled: true, scaleFactor: 0.65 },
      },
      font: {
        color: '#a1a1aa',
        size: 10,
        face: 'ui-monospace, monospace',
        background: '#18181b',
        strokeWidth: 0,
      },
      color: {
        color: '#52525b',
        highlight: '#f59e0b',
        hover: '#d4d4d8',
      },
      smooth: {
        type: 'curvedCW',
        roundness: 0.15,
      },
    });
  }
  edgesDataSet.update(edgesToUpsert);
}

// 确保画布正确初始化并同步数据
function ensureNetwork() {
  if (!networkContainer.value) return;
  if (!networkInstance) {
    initNetwork();
  }
  syncGraphData();
}

// 搜索定位
watch(searchQuery, (query) => {
  const q = (query || '').trim().toLowerCase();
  if (!q) {
    if (selectedNode.value) {
      closeDetailDrawer();
    }
    return;
  }

  if (!nodesDataSet || !networkInstance) return;

  const matches = nodesDataSet.get({
    filter: (n) => n.name.toLowerCase().includes(q) || (n.summary || '').toLowerCase().includes(q)
  });

  if (matches.length > 0) {
    const target = matches[0];
    selectedNode.value = target;
    selectedLink.value = null;
    networkInstance.selectNodes([target.id]);
    networkInstance.focus(target.id, {
      scale: 1.3,
      animation: { duration: 400, easingFunction: 'easeInOutQuad' },
    });
  }
});

function closeDetailDrawer() {
  selectedNode.value = null;
  selectedLink.value = null;
  if (networkInstance) {
    networkInstance.unselectAll();
  }
}

// 关联辅助与详情抽屉
const nodeRelations = computed(() => {
  if (!selectedNode.value) return [];
  const id = selectedNode.value.id;
  return (state.codexRelations || []).filter(r => r.source_entry_id === id || r.target_entry_id === id);
});

function getEntityName(id) {
  const e = allGraphEntries.value.find(x => x.id === id);
  return e ? e.name : '未知实体';
}

// 画布视角控制
function zoomIn() {
  if (!networkInstance) return;
  const currentScale = networkInstance.getScale();
  const targetScale = Math.min(3.0, currentScale * 1.25);
  networkInstance.moveTo({
    scale: targetScale,
    animation: { duration: 200, easingFunction: 'easeInOutQuad' },
  });
  zoom.value = targetScale;
}

function zoomOut() {
  if (!networkInstance) return;
  const currentScale = networkInstance.getScale();
  const targetScale = Math.max(0.25, currentScale * 0.8);
  networkInstance.moveTo({
    scale: targetScale,
    animation: { duration: 200, easingFunction: 'easeInOutQuad' },
  });
  zoom.value = targetScale;
}

function resetView() {
  if (!networkInstance) return;
  networkInstance.setSize('100%', '100%');
  networkInstance.redraw();
  networkInstance.fit({
    animation: { duration: 500, easingFunction: 'easeInOutQuad' },
  });
  setTimeout(() => {
    if (networkInstance) {
      zoom.value = networkInstance.getScale();
    }
  }, 520);
}

// 手动创建关系
function openCreateRelationModal(sourceId = '') {
  createRelationSourceId.value = sourceId || '';
  showCreateRelationModal.value = true;
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
    await actions.loadCodexRelations();
    if (selectedLink.value?.id === relId) {
      selectedLink.value = null;
    }
    syncGraphData();
  }
}

async function runAIExtractRelations() {
  isExtracting.value = true;
  try {
    await actions.extractCodexRelations();
    await actions.loadCodexRelations();
    syncGraphData();
    if (networkInstance) {
      networkInstance.stabilize();
      networkInstance.fit({ animation: true });
    }
  } finally {
    isExtracting.value = false;
  }
}

// 核心生命周期监听：Tab 切换与项目切换保障 100% 唤醒
watch(() => state.activeTab, (tab) => {
  if (tab === 'graph') {
    nextTick(async () => {
      if (state.currentProject) {
        if (!state.codexEntries || state.codexEntries.length === 0) {
          await actions.loadCodexEntries();
        }
        if (!state.codexRelations || state.codexRelations.length === 0) {
          await actions.loadCodexRelations();
        }
      }
      ensureNetwork();
      if (networkInstance) {
        networkInstance.setSize('100%', '100%');
        networkInstance.redraw();
        networkInstance.fit();
      }
    });
  }
}, { immediate: true });

watch(() => state.currentProject?.id, async (projId) => {
  if (projId) {
    await actions.loadCodexEntries();
    await actions.loadCodexRelations();
    nextTick(() => {
      ensureNetwork();
      if (networkInstance) {
        networkInstance.setSize('100%', '100%');
        networkInstance.redraw();
        networkInstance.fit();
      }
    });
  }
}, { immediate: true });

watch([filteredNodes, filteredLinks], () => {
  syncGraphData();
  if (selectedNode.value && !filteredNodes.value.some(n => n.id === selectedNode.value.id)) {
    closeDetailDrawer();
  }
}, { deep: true });

function handleWindowResize() {
  if (networkInstance && state.activeTab === 'graph') {
    networkInstance.setSize('100%', '100%');
    networkInstance.redraw();
  }
}

onMounted(async () => {
  window.addEventListener('resize', handleWindowResize);
  if (state.currentProject) {
    await actions.loadCodexEntries();
    await actions.loadCodexRelations();
  }
  nextTick(() => {
    ensureNetwork();
    if (networkInstance) {
      networkInstance.fit();
    }
  });
});

onUnmounted(() => {
  window.removeEventListener('resize', handleWindowResize);
  if (networkInstance) {
    networkInstance.destroy();
    networkInstance = null;
  }
  nodesDataSet = null;
  edgesDataSet = null;
});
</script>

<style scoped>
:deep(.vis-tooltip) {
  background-color: #18181b !important;
  color: #f4f4f5 !important;
  border: 1px solid #3f3f46 !important;
  border-radius: 8px !important;
  padding: 8px 12px !important;
  font-family: inherit !important;
  font-size: 11px !important;
  box-shadow: 0 10px 25px -5px rgba(0, 0, 0, 0.5) !important;
  max-width: 260px !important;
  line-height: 1.4 !important;
  pointer-events: none !important;
  z-index: 100 !important;
  white-space: pre-wrap !important;
}
</style>
