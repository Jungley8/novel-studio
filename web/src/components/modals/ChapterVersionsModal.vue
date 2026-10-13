<template>
  <div
    class="fixed inset-0 bg-black/80 backdrop-blur-sm flex items-center justify-center p-4 z-50"
    @click.self="$emit('close')">
    <div class="bg-atelier-900 border border-atelier-750 rounded-2xl max-w-3xl w-full max-h-[85vh] flex flex-col p-5 space-y-4 shadow-2xl animate-fade-in">
      <div class="flex items-center justify-between border-b border-atelier-750 pb-3 shrink-0">
        <h3 class="text-sm font-serif font-bold text-ink-50">
          第 {{ chapter.chapter_index }} 章 · 版本记录
          <span v-if="!listLoading && versions.length > 0" class="ml-2 text-[11px] font-mono font-normal text-ink-400">{{ versions.length }} 个版本</span>
        </h3>
        <button
          @click="$emit('close')"
          class="text-ink-400 hover:text-ink-100 p-1 rounded transition cursor-pointer">
          <X class="w-4 h-4" />
        </button>
      </div>

      <div v-if="listLoading" class="text-center py-10 text-xs text-ink-400">
        正在加载版本记录…
      </div>
      <div v-else-if="listError" class="text-center py-10 text-xs text-brand-rose">
        {{ listError }}
      </div>
      <div v-else-if="versions.length === 0" class="text-center py-10 space-y-1">
        <p class="text-xs text-ink-300">暂无历史版本（版本从本次升级后开始记录）</p>
      </div>

      <div v-else class="flex gap-4 min-h-0 flex-1 overflow-hidden">
        <!-- 左侧：版本列表 -->
        <div class="w-64 shrink-0 overflow-y-auto space-y-1.5 pr-1">
          <div
            v-for="v in versions"
            :key="v.id"
            @click="selectVersion(v)"
            class="w-full px-2.5 py-2 rounded-lg border text-left transition cursor-pointer"
            :class="selectedId === v.id
              ? 'bg-brand-amber/15 border-brand-amber/30'
              : 'bg-atelier-950/60 border-atelier-800 hover:border-brand-amber/30'">
            <div class="flex items-center gap-1.5">
              <span
                class="text-[10px] font-bold px-1.5 py-0.5 rounded"
                :class="v.source === 'edit'
                  ? 'bg-atelier-750 text-ink-200'
                  : 'bg-brand-amber/20 text-brand-amber'">
                {{ v.source === 'edit' ? '编辑保存' : '定稿' }}
              </span>
              <span class="text-[10px] font-mono text-ink-500">{{ (v.content_hash || '').slice(0, 8) }}</span>
            </div>
            <div class="mt-1 text-[11px] font-mono text-ink-300">
              {{ v.word_count || 0 }} 字 · {{ formatDate(v.created_at) }}
            </div>
          </div>
        </div>

        <!-- 右侧：版本详情 -->
        <div class="flex-1 flex flex-col min-w-0 min-h-0">
          <div v-if="detailLoading" class="text-center py-10 text-xs text-ink-400">
            正在加载版本全文…
          </div>
          <div v-else-if="detailError" class="text-center py-10 text-xs text-brand-rose">
            {{ detailError }}
          </div>
          <div v-else-if="!selectedVersion" class="flex-1 flex items-center justify-center text-xs text-ink-500">
            请在左侧选择一个版本查看
          </div>
          <div v-else class="flex flex-col min-h-0 flex-1">
            <div class="flex items-center gap-4 text-xs select-none border-b border-atelier-750 shrink-0">
              <button
                @click="detailTab = 'full'"
                class="py-2 border-b-2 font-medium transition cursor-pointer"
                :class="detailTab === 'full' ? 'border-brand-amber text-brand-amber font-bold' : 'border-transparent text-ink-400 hover:text-ink-200'">
                版本全文
              </button>
              <button
                @click="detailTab = 'diff'"
                class="py-2 border-b-2 font-medium transition cursor-pointer"
                :class="detailTab === 'diff' ? 'border-brand-amber text-brand-amber font-bold' : 'border-transparent text-ink-400 hover:text-ink-200'">
                与当前正文对比
              </button>
            </div>

            <div v-if="detailTab === 'full'" class="flex-1 overflow-y-auto py-3 text-xs font-serif leading-relaxed text-ink-100 whitespace-pre-wrap">
              {{ selectedVersion.content || '（该版本正文为空）' }}
            </div>
            <div v-else class="flex-1 overflow-auto py-2 text-xs min-h-[360px]">
              <CodeDiff
                :old-string="selectedVersion.content || ''"
                :new-string="chapter.content || ''"
                output-format="side-by-side"
                theme="dark"
                diff-style="word"
                language="markdown"
                filename="历史版本"
                new-filename="当前章节正文"
                class="h-full rounded-lg"
              />
            </div>
          </div>
        </div>
      </div>

      <div class="flex justify-end gap-2.5 pt-3 border-t border-atelier-750 shrink-0">
        <button
          @click="$emit('close')"
          class="px-3.5 py-1.5 bg-atelier-850 hover:bg-atelier-800 text-xs text-ink-300 rounded-lg transition cursor-pointer">
          关闭
        </button>
        <button
          @click="$emit('load', selectedVersion)"
          :disabled="!selectedVersion"
          class="px-4 py-1.5 bg-brand-amber hover:bg-brand-amber-hover text-atelier-950 font-bold text-xs rounded-lg shadow-amber-glow transition cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed">
          载入到工作台
        </button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, watch } from 'vue';
import { CodeDiff } from 'v-code-diff';
import { api } from '../../api/client';
import { X } from 'lucide-vue-next';

const props = defineProps({
  projectId: { type: String, required: true },
  chapter: { type: Object, required: true },
});
defineEmits(['close', 'load']);

const versions = ref([]);
const listLoading = ref(true);
const listError = ref('');
const selectedId = ref('');
const selectedVersion = ref(null);
const detailLoading = ref(false);
const detailError = ref('');
const detailTab = ref('full');

async function loadList() {
  listLoading.value = true;
  listError.value = '';
  try {
    const list = await api.listChapterVersions(props.projectId, props.chapter.chapter_index);
    versions.value = Array.isArray(list) ? list : [];
  } catch (e) {
    listError.value = e.message || '加载版本列表失败';
  } finally {
    listLoading.value = false;
  }
}

async function selectVersion(v) {
  if (!v) return;
  selectedId.value = v.id;
  detailTab.value = 'full';
  detailLoading.value = true;
  detailError.value = '';
  selectedVersion.value = null;
  try {
    selectedVersion.value = await api.getChapterVersion(props.projectId, props.chapter.chapter_index, v.id);
  } catch (e) {
    detailError.value = e.message || '加载版本全文失败';
  } finally {
    detailLoading.value = false;
  }
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

onMounted(loadList);
watch(() => props.chapter?.id, () => {
  selectedId.value = '';
  selectedVersion.value = null;
  loadList();
});
</script>
