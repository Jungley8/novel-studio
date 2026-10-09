<template>
  <div 
    v-if="state.showCodexModal" 
    class="fixed inset-0 bg-black/80 backdrop-blur-sm flex items-center justify-center p-4 z-50">
    
    <div class="bg-atelier-900 border border-atelier-750 rounded-2xl max-w-lg w-full p-6 space-y-4 shadow-2xl animate-fade-in">
      <div class="flex items-center justify-between border-b border-atelier-750 pb-3">
        <h3 class="text-sm font-serif font-bold text-ink-50">
          {{ state.editingCodexEntry ? '编辑设定词条' : '新建设定词条' }}
        </h3>
        <button 
          @click="state.showCodexModal = false" 
          class="text-ink-400 hover:text-ink-100 p-1 rounded transition cursor-pointer">
          <X class="w-4 h-4" />
        </button>
      </div>

      <div class="space-y-3 text-xs">
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
          <div>
            <label class="text-ink-300 font-medium">词条名称：</label>
            <input 
              v-model="form.name" 
              class="w-full mt-1 bg-atelier-950 border border-atelier-750 rounded-lg p-2.5 text-xs text-ink-50 font-serif font-bold focus:outline-none focus:border-brand-amber/60" 
              placeholder="例如：顾渊 / 聚仙楼 / 残骨铁印">
          </div>
          <div>
            <label class="text-ink-300 font-medium">设定分类：</label>
            <select 
              v-model="form.category" 
              class="w-full mt-1 bg-atelier-950 border border-atelier-750 rounded-lg p-2.5 text-xs text-ink-100 focus:outline-none focus:border-brand-amber/60 cursor-pointer">
              <option value="CHARACTER">👤 角色 (CHARACTER)</option>
              <option value="LOCATION">🗺️ 地理场景 (LOCATION)</option>
              <option value="LORE">📜 设定公理 (LORE)</option>
              <option value="ITEM">💎 道具灵宝 (ITEM)</option>
            </select>
          </div>
        </div>

        <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
          <div>
            <label class="text-ink-300 font-medium">标识色系：</label>
            <div class="flex items-center gap-2 mt-1">
              <input 
                type="color" 
                v-model="form.color_tag" 
                class="w-9 h-9 rounded border border-atelier-750 bg-atelier-950 cursor-pointer p-0.5">
              <input 
                v-model="form.color_tag" 
                class="flex-1 bg-atelier-950 border border-atelier-750 rounded-lg px-2.5 py-2 text-xs font-mono text-ink-200">
            </div>
          </div>
          <div>
            <label class="text-ink-300 font-medium">注入追踪模式：</label>
            <select 
              v-model="form.tracking_mode" 
              class="w-full mt-1 bg-atelier-950 border border-atelier-750 rounded-lg p-2.5 text-xs text-ink-100 focus:outline-none focus:border-brand-amber/60 cursor-pointer">
              <option value="AUTO_MENTION">AUTO_MENTION (命中别名时动态注入)</option>
              <option value="ALWAYS_INJECT">ALWAYS_INJECT (每章强制注入)</option>
              <option value="MANUAL">MANUAL (仅手动调用)</option>
            </select>
          </div>
        </div>

        <div>
          <label class="text-ink-300 font-medium">别名索引列表 (以中文或英文逗号分隔)：</label>
          <input 
            v-model="form.aliases_text" 
            class="w-full mt-1 bg-atelier-950 border border-atelier-750 rounded-lg p-2.5 text-xs text-ink-100 font-mono focus:outline-none focus:border-brand-amber/60" 
            placeholder="例如：白衣修罗, 疯子楚, 楚掌柜">
        </div>

        <!-- 角色独有人设与声口刻画字段 (当类别为 CHARACTER 时展示) -->
        <div v-if="form.category === 'CHARACTER'" class="p-3 bg-atelier-950/80 rounded-xl border border-brand-amber/30 space-y-2.5">
          <div class="text-[11px] font-serif font-bold text-brand-amber flex items-center gap-1.5">
            <span>🎭</span>
            <span>多人物刻画与声口人设 (Character Persona)</span>
          </div>

          <div class="grid grid-cols-1 sm:grid-cols-2 gap-2.5">
            <div>
              <label class="text-[10px] text-ink-400">角色戏剧定位：</label>
              <select 
                v-model="form.archetype" 
                class="w-full mt-1 bg-atelier-900 border border-atelier-750 rounded-md p-1.5 text-xs text-ink-100 focus:outline-none focus:border-brand-amber/60 cursor-pointer">
                <option value="SUPPORTING">SUPPORTING (配角群像/暗桩)</option>
                <option value="ANTAGONIST">ANTAGONIST (核心反派/宿敌)</option>
                <option value="DEUTERAGONIST">DEUTERAGONIST (关键搭档/女配)</option>
                <option value="MENTOR">MENTOR (引路导师/宗门长辈)</option>
                <option value="PROTAGONIST">PROTAGONIST (主角同位体)</option>
              </select>
            </div>
            <div>
              <label class="text-[10px] text-ink-400">对主角当前立场：</label>
              <select 
                v-model="form.current_disposition" 
                class="w-full mt-1 bg-atelier-900 border border-atelier-750 rounded-md p-1.5 text-xs text-ink-100 focus:outline-none focus:border-brand-amber/60 cursor-pointer">
                <option value="HOSTILE">HOSTILE (敌对仇视/暗杀)</option>
                <option value="WARY">WARY (警惕忌惮/猜疑)</option>
                <option value="NEUTRAL">NEUTRAL (中立观望/唯利)</option>
                <option value="FRIENDLY">FRIENDLY (亲善互利/同盟)</option>
                <option value="DEVOTED">DEVOTED (忠诚追随/归心)</option>
              </select>
            </div>
          </div>

          <div>
            <label class="text-[10px] text-ink-400">台词声口与语言习惯 (Voice Tone)：</label>
            <input 
              v-model="form.voice_tone" 
              class="w-full mt-0.5 bg-atelier-900 border border-atelier-750 rounded-md p-1.5 text-xs text-ink-100 focus:outline-none focus:border-brand-amber/60" 
              placeholder="例如：惜字如金、冷嘲热讽、文雅阴柔、粗粝豪爽">
          </div>

          <div>
            <label class="text-[10px] text-ink-400">核心底层动机与软肋 (Core Motivation)：</label>
            <input 
              v-model="form.core_motivation" 
              class="w-full mt-0.5 bg-atelier-900 border border-atelier-750 rounded-md p-1.5 text-xs text-ink-100 focus:outline-none focus:border-brand-amber/60" 
              placeholder="例如：搜罗凡人精血向魔尊邀功；极度自负，经不起激将法">
          </div>
        </div>

        <div>
          <label class="text-ink-300 font-medium">一句话核心特征摘要 (注入 Prompt 核心)：</label>
          <input 
            v-model="form.summary" 
            class="w-full mt-1 bg-atelier-950 border border-atelier-750 rounded-lg p-2.5 text-xs text-ink-100 font-serif focus:outline-none focus:border-brand-amber/60" 
            placeholder="简短凝练的人物身份、性格或地理关键特征">
        </div>

        <div>
          <label class="text-ink-300 font-medium">详细百科设定档案 (Markdown)：</label>
          <textarea 
            v-model="form.details_markdown" 
            rows="3" 
            class="w-full mt-1 bg-atelier-950 border border-atelier-750 rounded-lg p-2.5 text-xs text-ink-100 font-serif resize-none focus:outline-none focus:border-brand-amber/60 leading-relaxed" 
            placeholder="生平履历、功法招式、性格暗伤等详细背景..."></textarea>
        </div>
      </div>

      <div class="flex justify-end gap-2.5 pt-3 border-t border-atelier-750">
        <button 
          @click="state.showCodexModal = false" 
          class="px-3.5 py-1.5 bg-atelier-850 hover:bg-atelier-800 text-xs text-ink-300 rounded-lg transition cursor-pointer">
          取消
        </button>
        <button 
          @click="saveCodexEntry" 
          :disabled="!form.name.trim()"
          class="px-4 py-1.5 bg-brand-amber hover:bg-brand-amber-hover text-atelier-950 font-bold text-xs rounded-lg shadow-amber-glow transition cursor-pointer disabled:opacity-50">
          保存词条
        </button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { reactive, watch } from 'vue';
import { state, actions, notify } from '../../stores/appState';
import { api } from '../../api/client';
import { X } from 'lucide-vue-next';

const form = reactive({
  name: '',
  category: 'CHARACTER',
  color_tag: '#e5a93c',
  tracking_mode: 'AUTO_MENTION',
  archetype: 'SUPPORTING',
  voice_tone: '',
  core_motivation: '',
  current_disposition: 'WARY',
  aliases_text: '',
  summary: '',
  details_markdown: '',
});

watch(() => state.showCodexModal, (open) => {
  if (open) {
    if (state.editingCodexEntry) {
      const e = state.editingCodexEntry;
      Object.assign(form, {
        name: e.name || '',
        category: e.category || 'CHARACTER',
        color_tag: e.color_tag || '#e5a93c',
        tracking_mode: e.tracking_mode || 'AUTO_MENTION',
        archetype: e.archetype || 'SUPPORTING',
        voice_tone: e.voice_tone || '',
        core_motivation: e.core_motivation || '',
        current_disposition: e.current_disposition || 'WARY',
        aliases_text: (e.aliases || []).join(', '),
        summary: e.summary || '',
        details_markdown: e.details_markdown || '',
      });
    } else {
      Object.assign(form, {
        name: '',
        category: 'CHARACTER',
        color_tag: '#e5a93c',
        tracking_mode: 'AUTO_MENTION',
        archetype: 'SUPPORTING',
        voice_tone: '',
        core_motivation: '',
        current_disposition: 'WARY',
        aliases_text: '',
        summary: '',
        details_markdown: '',
      });
    }
  }
});

async function saveCodexEntry() {
  if (!form.name.trim() || !state.currentProject) return;
  const aliases = form.aliases_text
    .split(/[,，]/)
    .map(s => s.trim())
    .filter(Boolean);

  const payload = {
    name: form.name.trim(),
    category: form.category,
    color_tag: form.color_tag,
    tracking_mode: form.tracking_mode,
    archetype: form.archetype,
    voice_tone: form.voice_tone,
    core_motivation: form.core_motivation,
    current_disposition: form.current_disposition,
    aliases,
    summary: form.summary,
    details_markdown: form.details_markdown,
  };

  try {
    if (state.editingCodexEntry?.id) {
      await api.updateCodexEntry(state.currentProject.id, state.editingCodexEntry.id, payload);
      notify('词条已更新', payload.name, 'success');
    } else {
      await api.createCodexEntry(state.currentProject.id, payload);
      notify('词条已创建', payload.name, 'success');
    }
    await actions.loadCodexEntries();
    state.showCodexModal = false;
  } catch (e) {
    notify('保存词条失败', e.message, 'error');
  }
}
</script>
