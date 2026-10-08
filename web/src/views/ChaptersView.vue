<template>
  <div class="flex-1 flex flex-col h-full overflow-hidden bg-atelier-950">
    <!-- 头部工具栏 -->
    <div class="h-14 px-6 border-b border-atelier-750 flex items-center justify-between bg-atelier-900/40 shrink-0 select-none">
      <div class="flex items-center gap-3">
        <div class="w-8 h-8 rounded-lg bg-brand-amber/10 border border-brand-amber/25 flex items-center justify-center text-brand-amber shadow-amber-glow">
          <Archive class="w-4 h-4" />
        </div>
        <div>
          <div class="flex items-center gap-2">
            <h2 class="text-sm font-serif font-bold text-ink-50 tracking-wide">全本已归档正史 (Canon History)</h2>
            <span class="text-[10px] font-mono text-ink-400 bg-atelier-850 px-2 py-0.5 rounded border border-atelier-750">
              {{ state.currentProject?.title || '未命名作品' }}
            </span>
          </div>
          <p class="text-[11px] text-ink-400">历次因果原子封存章节、质检裁决报告与成书正文阅览室</p>
        </div>
      </div>

      <div class="flex items-center gap-3">
        <!-- 统计指标 -->
        <div class="hidden sm:flex items-center gap-2 text-xs font-mono bg-atelier-850 px-3 py-1.5 rounded-md border border-atelier-750">
          <span class="text-ink-400">已归档</span>
          <strong class="text-brand-amber">{{ state.chapters.length }}</strong>
          <span class="text-ink-400">章</span>
          <span class="text-atelier-700">|</span>
          <span class="text-ink-400">总字数</span>
          <strong class="text-brand-amber">{{ totalWords.toLocaleString() }}</strong>
        </div>

        <!-- 视图模式切换：树结构 vs 瀑布卡片 -->
        <div class="flex items-center bg-atelier-850 rounded-lg p-0.5 border border-atelier-750">
          <button 
            @click="viewMode = 'tree'" 
            class="px-2.5 py-1 text-xs rounded-md transition flex items-center gap-1.5 cursor-pointer"
            :class="viewMode === 'tree' ? 'bg-brand-amber text-atelier-950 font-bold shadow-atelier-sm' : 'text-ink-300 hover:text-ink-100'">
            <ListTree class="w-3.5 h-3.5" />
            <span>树形目录</span>
          </button>
          <button 
            @click="viewMode = 'cards'" 
            class="px-2.5 py-1 text-xs rounded-md transition flex items-center gap-1.5 cursor-pointer"
            :class="viewMode === 'cards' ? 'bg-brand-amber text-atelier-950 font-bold shadow-atelier-sm' : 'text-ink-300 hover:text-ink-100'">
            <LayoutGrid class="w-3.5 h-3.5" />
            <span>瀑布卡片</span>
          </button>
        </div>

        <button 
          v-if="viewMode === 'cards'"
          @click="toggleExpandAll" 
          class="px-3 py-1.5 bg-atelier-850 hover:bg-atelier-800 text-ink-300 text-xs rounded-md border border-atelier-750 transition cursor-pointer">
          {{ isAllExpanded ? '收起全部正文' : '展开全部正文' }}
        </button>
      </div>
    </div>

    <!-- 空白状态 -->
    <div 
      v-if="state.chapters.length === 0" 
      class="flex-1 flex flex-col items-center justify-center p-8 text-center bg-atelier-950">
      <div class="w-14 h-14 rounded-2xl bg-atelier-900 border border-atelier-750 flex items-center justify-center text-ink-400 mb-4 shadow-inner">
        <Archive class="w-7 h-7 text-ink-500" />
      </div>
      <h3 class="text-sm font-semibold font-serif text-ink-200">暂无已封存归档章节</h3>
      <p class="text-xs text-ink-400 max-w-md mx-auto mt-1 leading-relaxed">
        在“故事工坊”中完成节拍推演、正文渲染与反 AI 质检后，点击“因果封存归档”即可将章节正式收录入全本正史。
      </p>
      <button 
        @click="state.activeTab = 'workbench'"
        class="mt-4 px-4 py-2 bg-brand-amber hover:bg-brand-amber-hover text-atelier-950 text-xs font-bold rounded-lg transition shadow-atelier-sm flex items-center gap-1.5 cursor-pointer">
        <PenTool class="w-3.5 h-3.5" />
        <span>前往故事工坊创作第一章</span>
      </button>
    </div>

    <!-- 模式 1：树结构全景阅览器 (Tree + Reader Split View) -->
    <div v-else-if="viewMode === 'tree'" class="flex-1 flex overflow-hidden min-h-0">
      <!-- 左侧：卷-章-节拍层级树 (可折叠自适应宽度) -->
      <div class="w-80 2xl:w-96 border-r border-atelier-750 bg-atelier-900/30 flex flex-col shrink-0 overflow-hidden select-none">
        <!-- 树目录顶栏过滤搜索 -->
        <div class="p-3 border-b border-atelier-750 bg-atelier-900/50">
          <div class="relative">
            <Search class="w-3.5 h-3.5 text-ink-400 absolute left-2.5 top-2.5" />
            <input 
              v-model="searchQuery"
              type="text"
              placeholder="搜索章节、冲突关键词..."
              class="w-full bg-atelier-950 border border-atelier-750 rounded-lg pl-8 pr-3 py-1.5 text-xs text-ink-100 placeholder-ink-500 focus:outline-none focus:border-brand-amber/50 transition font-sans" />
          </div>
        </div>

        <!-- 目录树滚动区 -->
        <div class="flex-1 overflow-y-auto p-2 space-y-1">
          <div v-for="vol in filteredVolumeGroups" :key="vol.id" class="space-y-0.5">
            <!-- 卷节点 (Volume Node) -->
            <div 
              @click="toggleVolume(vol.id)"
              class="w-full px-2.5 py-1.5 rounded-lg flex items-center justify-between text-xs font-bold text-ink-200 hover:bg-atelier-850/60 transition cursor-pointer group">
              <div class="flex items-center gap-2 truncate">
                <ChevronRight 
                  class="w-3.5 h-3.5 text-ink-400 transition-transform duration-150 shrink-0" 
                  :class="volExpandedMap[vol.id] !== false ? 'rotate-90' : ''" />
                <BookOpen class="w-3.5 h-3.5 text-brand-amber shrink-0" />
                <span class="font-serif tracking-wide truncate">{{ vol.title }}</span>
              </div>
              <span class="text-[10px] font-mono text-ink-400 bg-atelier-950 px-1.5 py-0.5 rounded border border-atelier-800 shrink-0">
                {{ vol.chapters.length }}章
              </span>
            </div>

            <!-- 卷下章节列表 (Chapters under Volume) -->
            <div v-show="volExpandedMap[vol.id] !== false" class="pl-4 space-y-0.5 border-l border-atelier-800 ml-3.5">
              <div v-for="c in vol.chapters" :key="c.id" class="space-y-0.5">
                <!-- 章节节点 (Chapter Item) -->
                <div 
                  @click="selectChapter(c.id)"
                  class="w-full px-2 py-1.5 rounded-lg flex items-center justify-between text-xs transition cursor-pointer group"
                  :class="selectedChapterId === c.id 
                    ? 'bg-brand-amber/15 text-brand-amber font-semibold border border-brand-amber/30' 
                    : 'text-ink-300 hover:text-ink-100 hover:bg-atelier-850/50 border border-transparent'">
                  
                  <div class="flex items-center gap-1.5 min-w-0">
                    <!-- 章节节拍展开小箭头 -->
                    <button 
                      @click.stop="toggleChapterBeats(c.id)"
                      class="p-0.5 rounded hover:bg-atelier-800 text-ink-400 hover:text-ink-200 transition"
                      title="展开节拍事实">
                      <ChevronRight 
                        class="w-3 h-3 transition-transform duration-150" 
                        :class="chapterBeatsExpandedMap[c.id] ? 'rotate-90' : ''" />
                    </button>
                    <span class="font-mono text-[11px] shrink-0">第{{ c.chapter_index }}章</span>
                    <span class="truncate font-serif text-[12px]">{{ c.title }}</span>
                  </div>

                  <div class="flex items-center gap-1.5 shrink-0">
                    <span 
                      v-if="c.review" 
                      class="text-[9px] font-mono px-1 rounded"
                      :class="c.review.verdict === 'ACCEPTED' ? 'text-brand-emerald bg-brand-emerald/10' : 'text-brand-rose bg-brand-rose/10'">
                      {{ c.review.score }}分
                    </span>
                    <span class="text-[10px] font-mono text-ink-500">{{ c.word_count }}字</span>
                  </div>
                </div>

                <!-- 章节子节点：4段因果节拍事实 (Beats Leaf Nodes) -->
                <div 
                  v-show="chapterBeatsExpandedMap[c.id]" 
                  class="pl-6 pr-1 py-1 space-y-1 text-[11px] bg-atelier-950/60 rounded-md border border-atelier-850/60 my-0.5">
                  <div 
                    v-for="(b, bIdx) in c.beats" 
                    :key="bIdx"
                    class="flex items-start gap-1.5 text-ink-400 leading-snug">
                    <span class="text-[9px] font-mono font-bold text-brand-amber/80 bg-brand-amber/10 px-1 rounded shrink-0">
                      拍{{ bIdx + 1 }}
                    </span>
                    <span class="text-ink-300 truncate" :title="b.action || b.expectation_broken">
                      {{ b.phase }} · {{ b.action || b.expectation_broken || '因果推进' }}
                    </span>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- 右侧：全章正文沉浸阅读主视窗 (宽幅典雅排版) -->
      <div v-if="currentSelectedChapter" class="flex-1 flex flex-col overflow-hidden bg-atelier-950">
        <!-- 章节阅读器顶栏信息与快捷操作 -->
        <div class="px-6 py-3.5 border-b border-atelier-750 bg-atelier-900/30 flex items-center justify-between shrink-0 select-none">
          <div class="flex items-center gap-3 min-w-0">
            <span class="px-2.5 py-0.5 bg-brand-amber/15 text-brand-amber text-xs font-mono font-bold rounded border border-brand-amber/30 shrink-0">
              第 {{ currentSelectedChapter.chapter_index }} 章
            </span>
            <h3 class="text-base font-serif font-bold text-ink-50 tracking-wide truncate">
              {{ currentSelectedChapter.title }}
            </h3>
            <span class="hidden md:inline text-xs font-mono text-ink-400">
              · {{ currentSelectedChapter.word_count }} 字 · 约 {{ Math.max(1, Math.ceil(currentSelectedChapter.word_count / 400)) }} 分钟读完
            </span>
          </div>

          <div class="flex items-center gap-2 shrink-0">
            <!-- 质检结果角标 -->
            <span 
              v-if="currentSelectedChapter.review" 
              class="px-2.5 py-1 rounded font-mono text-xs font-medium flex items-center gap-1" 
              :class="currentSelectedChapter.review.verdict === 'ACCEPTED' 
                ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20' 
                : 'bg-rose-500/10 text-rose-400 border border-rose-500/20'">
              <CheckCircle2 v-if="currentSelectedChapter.review.verdict === 'ACCEPTED'" class="w-3.5 h-3.5" />
              <AlertTriangle v-else class="w-3.5 h-3.5" />
              <span>主审: {{ currentSelectedChapter.review.score }}分 ({{ currentSelectedChapter.review.verdict }})</span>
            </span>

            <!-- 复制正文 -->
            <button 
              @click="copyChapterContent"
              class="px-2.5 py-1 text-xs bg-atelier-850 hover:bg-atelier-800 text-ink-200 border border-atelier-750 rounded-md transition flex items-center gap-1 cursor-pointer"
              title="复制全章正文到剪贴板">
              <Check v-if="copied" class="w-3.5 h-3.5 text-brand-emerald" />
              <Copy v-else class="w-3.5 h-3.5 text-ink-400" />
              <span>{{ copied ? '已复制' : '复制正文' }}</span>
            </button>

            <!-- 载入工坊精修 -->
            <button 
              @click="handleLoadToWorkbench(currentSelectedChapter)"
              class="px-2.5 py-1 text-xs bg-atelier-850 hover:bg-atelier-800 text-ink-200 border border-atelier-750 rounded-md transition flex items-center gap-1 cursor-pointer"
              title="载入故事工坊画布进行精修">
              <FileEdit class="w-3.5 h-3.5 text-brand-amber" />
              <span>载入工坊</span>
            </button>

            <!-- 撤回归档为草稿 -->
            <button 
              @click="handleRevertToDraft(currentSelectedChapter.chapter_index)"
              class="px-2.5 py-1 text-xs bg-brand-rose/10 hover:bg-brand-rose/20 text-brand-rose border border-brand-rose/30 rounded-md transition flex items-center gap-1 cursor-pointer"
              title="将本章从正史移出，回滚主角账本，恢复为在途草稿">
              <RotateCcw class="w-3.5 h-3.5" />
              <span>撤回归档为草稿</span>
            </button>

            <!-- 字体大小缩放 -->
            <div class="flex items-center bg-atelier-850 rounded border border-atelier-750 text-xs">
              <button 
                @click="readerFontSize = 'sm'"
                class="px-2 py-0.5 text-[11px] rounded transition"
                :class="readerFontSize === 'sm' ? 'bg-atelier-750 text-ink-100 font-bold' : 'text-ink-400 hover:text-ink-200'">
                小
              </button>
              <button 
                @click="readerFontSize = 'base'"
                class="px-2 py-0.5 text-[11px] rounded transition"
                :class="readerFontSize === 'base' ? 'bg-atelier-750 text-ink-100 font-bold' : 'text-ink-400 hover:text-ink-200'">
                中
              </button>
              <button 
                @click="readerFontSize = 'lg'"
                class="px-2 py-0.5 text-[11px] rounded transition"
                :class="readerFontSize === 'lg' ? 'bg-atelier-750 text-ink-100 font-bold' : 'text-ink-400 hover:text-ink-200'">
                大
              </button>
            </div>
          </div>
        </div>

        <!-- 标签页切换：沉浸正文 / 节拍事实 / 质检报告 -->
        <div class="px-6 border-b border-atelier-750 bg-atelier-900/10 flex items-center gap-4 text-xs select-none">
          <button 
            @click="activeReaderTab = 'content'"
            class="py-2.5 border-b-2 font-medium transition cursor-pointer flex items-center gap-1.5"
            :class="activeReaderTab === 'content' ? 'border-brand-amber text-brand-amber font-bold' : 'border-transparent text-ink-400 hover:text-ink-200'">
            <FileText class="w-3.5 h-3.5" />
            <span>沉浸正文成稿</span>
          </button>
          <button 
            @click="activeReaderTab = 'beats'"
            class="py-2.5 border-b-2 font-medium transition cursor-pointer flex items-center gap-1.5"
            :class="activeReaderTab === 'beats' ? 'border-brand-amber text-brand-amber font-bold' : 'border-transparent text-ink-400 hover:text-ink-200'">
            <Layers class="w-3.5 h-3.5" />
            <span>4段因果节拍与状态变迁</span>
          </button>
          <button 
            @click="activeReaderTab = 'review'"
            class="py-2.5 border-b-2 font-medium transition cursor-pointer flex items-center gap-1.5"
            :class="activeReaderTab === 'review' ? 'border-brand-amber text-brand-amber font-bold' : 'border-transparent text-ink-400 hover:text-ink-200'">
            <ShieldCheck class="w-3.5 h-3.5" />
            <span>总编审质检报告</span>
          </button>
        </div>

        <!-- 阅读器正文主内容区 (宽幅自适应滚动) -->
        <div class="flex-1 overflow-y-auto p-6 md:p-10 flex justify-center">
          <div class="w-full max-w-4xl 2xl:max-w-5xl space-y-6">
            
            <!-- 核心冲突卡 -->
            <div v-if="currentSelectedChapter.core_conflict" class="p-3.5 bg-atelier-900/60 rounded-xl border border-atelier-800 text-xs text-ink-300 flex items-start gap-2.5">
              <span class="text-brand-amber font-semibold shrink-0">本章核心冲突:</span>
              <span class="leading-relaxed">{{ currentSelectedChapter.core_conflict }}</span>
            </div>

            <!-- TAB 1: 沉浸正文 -->
            <div v-if="activeReaderTab === 'content'" class="space-y-4">
              <div 
                class="font-serif leading-[2.2] tracking-wide text-ink-100 whitespace-pre-wrap selection:bg-brand-amber/30 selection:text-ink-50 transition-all"
                :class="{
                  'text-sm': readerFontSize === 'sm',
                  'text-base md:text-[17px]': readerFontSize === 'base',
                  'text-lg md:text-[19px]': readerFontSize === 'lg'
                }">
                {{ currentSelectedChapter.content }}
              </div>

              <!-- 章节收尾信息 -->
              <div class="pt-8 border-t border-atelier-800/80 flex items-center justify-between text-xs text-ink-500 font-mono">
                <span>封存时间: {{ formatDate(currentSelectedChapter.created_at) }}</span>
                <span>突发度得分: {{ currentSelectedChapter.burstiness_score || 50 }} 分</span>
              </div>
            </div>

            <!-- TAB 2: 因果节拍事实 -->
            <div v-else-if="activeReaderTab === 'beats'" class="space-y-4">
              <div class="grid grid-cols-1 md:grid-cols-2 gap-3.5">
                <div 
                  v-for="(b, idx) in currentSelectedChapter.beats" 
                  :key="idx"
                  class="p-4 bg-atelier-900/80 border border-atelier-750 rounded-xl space-y-2">
                  <div class="flex items-center justify-between text-xs">
                    <span class="font-bold text-brand-amber flex items-center gap-1.5">
                      <span class="w-5 h-5 rounded-full bg-brand-amber/20 flex items-center justify-center font-mono text-[10px]">
                        {{ idx + 1 }}
                      </span>
                      <span>{{ b.phase }}</span>
                    </span>
                    <span class="font-mono text-[11px] px-2 py-0.5 rounded bg-atelier-850 text-ink-300 border border-atelier-750">
                      张力: {{ b.tension }}/10
                    </span>
                  </div>
                  <div class="text-xs text-ink-200">
                    <span class="text-ink-400 font-medium">执行动作：</span>
                    <span>{{ b.action || '无特定动作' }}</span>
                  </div>
                  <div class="text-xs text-ink-300">
                    <span class="text-brand-amber/80 font-medium">预期打破：</span>
                    <span>{{ b.expectation_broken || '无反转' }}</span>
                  </div>
                </div>
              </div>

              <!-- 状态变迁账本 -->
              <div v-if="currentSelectedChapter.state_mutation" class="p-4 bg-atelier-900/80 border border-atelier-750 rounded-xl space-y-2 text-xs">
                <h4 class="font-bold text-brand-emerald flex items-center gap-1.5">
                  <Sparkles class="w-3.5 h-3.5" />
                  <span>主角状态机变迁 (State Mutation)</span>
                </h4>
                <div class="grid grid-cols-1 sm:grid-cols-2 gap-2 text-ink-300">
                  <div>战力增减：<span class="text-ink-100">{{ currentSelectedChapter.state_mutation.power_delta || '无' }}</span></div>
                  <div>物品变动：<span class="text-ink-100">{{ currentSelectedChapter.state_mutation.inventory_delta || '无' }}</span></div>
                </div>
              </div>
            </div>

            <!-- TAB 3: 责编质检报告 -->
            <div v-else-if="activeReaderTab === 'review'" class="space-y-4">
              <div v-if="currentSelectedChapter.review" class="p-5 bg-atelier-900/80 border border-atelier-750 rounded-xl space-y-4 text-xs">
                <div class="flex items-center justify-between pb-3 border-b border-atelier-800">
                  <div class="flex items-center gap-2">
                    <span class="font-bold text-sm text-ink-100 font-serif">主编审综合判定</span>
                    <span 
                      class="px-2 py-0.5 rounded font-mono font-bold"
                      :class="currentSelectedChapter.review.verdict === 'ACCEPTED' ? 'bg-brand-emerald/15 text-brand-emerald' : 'bg-brand-rose/15 text-brand-rose'">
                      {{ currentSelectedChapter.review.verdict }}
                    </span>
                  </div>
                  <span class="text-sm font-mono font-bold text-brand-amber">
                    {{ currentSelectedChapter.review.score }} / 100 分
                  </span>
                </div>

                <div v-if="currentSelectedChapter.review.issues?.length > 0" class="space-y-2">
                  <span class="text-brand-rose font-semibold block">审查指出的问题：</span>
                  <ul class="list-disc list-inside space-y-1 text-ink-300 pl-1">
                    <li v-for="(iss, iIdx) in currentSelectedChapter.review.issues" :key="iIdx">
                      {{ iss }}
                    </li>
                  </ul>
                </div>

                <div v-if="currentSelectedChapter.review.suggestions" class="space-y-1.5">
                  <span class="text-brand-amber font-semibold block">修改整改建议：</span>
                  <p class="text-ink-200 leading-relaxed bg-atelier-950 p-3 rounded-lg border border-atelier-800">
                    {{ currentSelectedChapter.review.suggestions }}
                  </p>
                </div>
              </div>
              <div v-else class="text-center py-10 text-xs text-ink-500">
                本章无独立审查记录。
              </div>
            </div>

          </div>
        </div>
      </div>

      <div v-else class="flex-1 flex items-center justify-center text-xs text-ink-400">
        请在左侧目录树选择章节查看正史。
      </div>
    </div>

    <!-- 模式 2：瀑布卡片视图 (Card Waterfall Layout) -->
    <div v-else class="flex-1 p-6 overflow-y-auto space-y-4">
      <div 
        v-for="c in state.chapters" 
        :key="c.id" 
        class="p-5 bg-atelier-900 border border-atelier-750 rounded-xl space-y-3.5 shadow-atelier-sm hover:border-brand-amber/30 transition">
        
        <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-2.5 border-b border-atelier-800 pb-3">
          <div class="flex items-center gap-3">
            <span class="px-2.5 py-0.5 bg-brand-amber/15 text-brand-amber text-xs font-mono font-bold rounded border border-brand-amber/30">
              第 {{ c.chapter_index }} 章
            </span>
            <h3 class="text-sm font-serif font-bold text-ink-50 tracking-wide">{{ c.title }}</h3>
          </div>

          <div class="flex items-center gap-3 text-xs">
            <span 
              v-if="c.review" 
              class="px-2 py-0.5 rounded font-mono text-[11px] font-medium" 
              :class="c.review.verdict === 'ACCEPTED' 
                ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20' 
                : 'bg-rose-500/10 text-rose-400 border border-rose-500/20'">
              主审: {{ c.review.score }}分 ({{ c.review.verdict }})
            </span>
            <span class="text-ink-400 font-mono text-[11px]">{{ c.word_count }} 字</span>
            <span class="text-ink-400 font-mono text-[11px]">突发度 {{ c.burstiness_score || 50 }} 分</span>
          </div>
        </div>

        <div v-if="c.core_conflict" class="text-xs text-ink-300 flex items-start gap-2 bg-atelier-950/50 p-2.5 rounded-lg border border-atelier-800/80">
          <span class="text-brand-amber font-medium shrink-0">核心冲突:</span>
          <span>{{ c.core_conflict }}</span>
        </div>

        <!-- 正文展示区域 -->
        <div 
          :class="[
            'text-xs text-ink-100 bg-atelier-950 p-4 rounded-lg border border-atelier-800 font-serif leading-relaxed whitespace-pre-wrap transition-all',
            expandedMap[c.id] ? '' : 'line-clamp-4 max-h-28 overflow-hidden'
          ]">
          {{ c.content }}
        </div>

        <div class="flex flex-col sm:flex-row justify-between sm:items-center text-xs text-ink-400 pt-2 border-t border-atelier-800/60 gap-2">
          <span class="text-[10px] font-mono">归档时间: {{ formatDate(c.created_at) }}</span>
          <div class="flex items-center gap-3">
            <button 
              @click="handleLoadToWorkbench(c)" 
              class="text-ink-300 hover:text-brand-amber flex items-center gap-1 font-medium transition cursor-pointer"
              title="载入故事工坊画布进行精修">
              <FileEdit class="w-3.5 h-3.5 text-brand-amber" />
              <span>载入工坊</span>
            </button>
            <button 
              @click="handleRevertToDraft(c.chapter_index)" 
              class="text-ink-400 hover:text-brand-rose flex items-center gap-1 font-medium transition cursor-pointer"
              title="将本章从正史移出，回滚主角账本，恢复为在途草稿">
              <RotateCcw class="w-3.5 h-3.5" />
              <span>撤回为草稿</span>
            </button>
            <button 
              @click="toggleExpand(c.id)" 
              class="text-brand-amber hover:text-brand-amber-hover flex items-center gap-1 font-medium transition cursor-pointer">
              <span>{{ expandedMap[c.id] ? '收起正文' : '展开阅读完整章节' }}</span>
              <ChevronDown class="w-3.5 h-3.5 transition-transform" :class="expandedMap[c.id] ? 'rotate-180' : ''" />
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { reactive, computed, ref, watch } from 'vue';
import { state, actions } from '../stores/appState';
import { 
  Archive, 
  ChevronDown, 
  ChevronRight, 
  BookOpen, 
  FileText, 
  Layers, 
  Sparkles, 
  ShieldCheck, 
  CheckCircle2, 
  AlertTriangle, 
  Clock, 
  Copy, 
  Check, 
  ListTree, 
  LayoutGrid, 
  Search, 
  PenTool,
  RotateCcw,
  FileEdit
} from 'lucide-vue-next';

// 视图模式: 'tree' (树形目录) 或 'cards' (瀑布卡片)
const viewMode = ref('tree');
const searchQuery = ref('');
const selectedChapterId = ref('');
const activeReaderTab = ref('content'); // 'content', 'beats', 'review'
const readerFontSize = ref('base'); // 'sm', 'base', 'lg'
const copied = ref(false);

const expandedMap = reactive({});
const isAllExpanded = ref(false);
const volExpandedMap = reactive({});
const chapterBeatsExpandedMap = reactive({});

const totalWords = computed(() => {
  return state.chapters.reduce((sum, c) => sum + (c.word_count || 0), 0);
});

// 计算按卷分层结构 (Volume -> Chapters)
const volumeGroups = computed(() => {
  const chapters = [...state.chapters].sort((a, b) => a.chapter_index - b.chapter_index);
  if (chapters.length === 0) return [];

  const frameworkArcs = state.currentProject?.framework?.volume_arcs || [];
  
  if (frameworkArcs.length > 0) {
    let chapterOffset = 1;
    const groups = [];
    frameworkArcs.forEach((arc, idx) => {
      const est = arc.estimated_chapters || 20;
      const startCh = chapterOffset;
      const endCh = chapterOffset + est - 1;
      const chsInVol = chapters.filter(c => c.chapter_index >= startCh && c.chapter_index <= endCh);
      groups.push({
        id: `vol_${arc.volume_index || idx + 1}`,
        title: arc.title ? `第 ${arc.volume_index || idx + 1} 卷 · ${arc.title}` : `第 ${idx + 1} 卷`,
        theme: arc.theme || '',
        rangeText: `第 ${startCh} - ${endCh} 章`,
        chapters: chsInVol,
      });
      chapterOffset += est;
    });

    const maxArcCh = chapterOffset - 1;
    const remaining = chapters.filter(c => c.chapter_index > maxArcCh);
    if (remaining.length > 0) {
      groups.push({
        id: 'vol_extra',
        title: `后续卷`,
        theme: '',
        rangeText: `第 ${maxArcCh + 1} 章及以后`,
        chapters: remaining,
      });
    }
    return groups;
  }

  // 默认每 20 章聚合为一卷
  const CHS_PER_VOL = 20;
  const groups = [];
  const maxCh = chapters[chapters.length - 1].chapter_index;
  const numVols = Math.max(1, Math.ceil(maxCh / CHS_PER_VOL));

  for (let v = 1; v <= numVols; v++) {
    const startCh = (v - 1) * CHS_PER_VOL + 1;
    const endCh = v * CHS_PER_VOL;
    const chsInVol = chapters.filter(c => c.chapter_index >= startCh && c.chapter_index <= endCh);
    groups.push({
      id: `vol_auto_${v}`,
      title: `第 ${v} 卷`,
      theme: '因果演进与主线推进',
      rangeText: `第 ${startCh} - ${endCh} 章`,
      chapters: chsInVol,
    });
  }
  return groups;
});

// 根据搜索词过滤后的卷章树
const filteredVolumeGroups = computed(() => {
  const q = searchQuery.value.trim().toLowerCase();
  if (!q) return volumeGroups.value;

  return volumeGroups.value.map(vol => {
    const matchedChapters = vol.chapters.filter(c => 
      c.title?.toLowerCase().includes(q) ||
      String(c.chapter_index).includes(q) ||
      c.core_conflict?.toLowerCase().includes(q) ||
      c.content?.toLowerCase().includes(q)
    );
    return {
      ...vol,
      chapters: matchedChapters,
    };
  }).filter(vol => vol.chapters.length > 0);
});

// 当前选中的章节对象
const currentSelectedChapter = computed(() => {
  if (!selectedChapterId.value && state.chapters.length > 0) {
    return state.chapters[0];
  }
  return state.chapters.find(c => c.id === selectedChapterId.value) || state.chapters[0] || null;
});

// 监听并在有章节时默认选中第一个
watch(() => state.chapters, (newChs) => {
  if (newChs.length > 0 && !selectedChapterId.value) {
    selectedChapterId.value = newChs[0].id;
  }
}, { immediate: true });

function selectChapter(id) {
  selectedChapterId.value = id;
}

function toggleVolume(volId) {
  volExpandedMap[volId] = volExpandedMap[volId] === false ? true : false;
}

function toggleChapterBeats(chapterId) {
  chapterBeatsExpandedMap[chapterId] = !chapterBeatsExpandedMap[chapterId];
}

async function copyChapterContent() {
  if (!currentSelectedChapter.value?.content) return;
  try {
    await navigator.clipboard.writeText(currentSelectedChapter.value.content);
    copied.value = true;
    setTimeout(() => {
      copied.value = false;
    }, 2000);
  } catch (e) {
    console.error('copy failed', e);
  }
}

function toggleExpand(id) {
  expandedMap[id] = !expandedMap[id];
}

function toggleExpandAll() {
  isAllExpanded.value = !isAllExpanded.value;
  state.chapters.forEach(c => {
    expandedMap[c.id] = isAllExpanded.value;
  });
}

function formatDate(isoStr) {
  if (!isoStr) return '未知时间';
  try {
    const d = new Date(isoStr);
    return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')} ${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`;
  } catch {
    return isoStr;
  }
}

function handleLoadToWorkbench(chapter) {
  if (!chapter) return;
  actions.loadChapterToWorkbench(chapter);
}

async function handleRevertToDraft(chapterIndex) {
  if (!chapterIndex) return;
  await actions.revertChapterToDraft(chapterIndex);
}
</script>
