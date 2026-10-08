<template>
  <div 
    v-if="state.showNewProjectModal" 
    class="fixed inset-0 bg-black/80 backdrop-blur-sm flex items-center justify-center p-4 z-50">
    
    <div class="bg-atelier-900 border border-atelier-750 rounded-2xl max-w-lg w-full p-6 space-y-4 shadow-2xl animate-fade-in">
      <!-- 头部与模式切换 -->
      <div class="flex items-center justify-between border-b border-atelier-750 pb-3.5">
        <div class="flex items-center gap-2">
          <div class="w-6 h-6 rounded bg-brand-amber/15 text-brand-amber flex items-center justify-center">
            <BookPlus class="w-3.5 h-3.5" />
          </div>
          <h3 class="text-sm font-serif font-bold text-ink-50 tracking-wide">新建长篇作品工程</h3>
        </div>

        <div class="flex bg-atelier-950 p-0.5 rounded-lg border border-atelier-800 text-xs">
          <button 
            @click="form.genesis_mode = true" 
            :class="form.genesis_mode ? 'bg-brand-amber text-atelier-950 font-bold shadow-amber-glow' : 'text-ink-400 hover:text-ink-200'"
            class="px-2.5 py-1 rounded-md transition cursor-pointer">
            🌌 AI 宏观创世
          </button>
          <button 
            @click="form.genesis_mode = false" 
            :class="!form.genesis_mode ? 'bg-atelier-800 text-ink-100 font-bold' : 'text-ink-400 hover:text-ink-200'"
            class="px-2.5 py-1 rounded-md transition cursor-pointer">
            📝 快速建书
          </button>
        </div>
      </div>

      <!-- 表单字段 -->
      <div class="space-y-3.5">
        <div>
          <label class="text-xs font-medium text-ink-300">小说题名：</label>
          <input 
            v-model="form.title" 
            class="w-full mt-1.5 bg-atelier-950 border border-atelier-750 rounded-lg p-2.5 text-xs text-ink-50 font-serif font-bold focus:outline-none focus:border-brand-amber/60" 
            placeholder="例如：凡人弑神录：万古三千年修真界沉浮长卷">
        </div>

        <div>
          <label class="text-xs font-medium text-ink-300">目标平台定位与叙事调性：</label>
          <select 
            v-model="form.target_platform" 
            class="w-full mt-1.5 bg-atelier-950 border border-atelier-750 rounded-lg p-2.5 text-xs text-ink-100 focus:outline-none focus:border-brand-amber/60 cursor-pointer">
            <option value="起点仙侠">起点仙侠 (严谨战力 / 厚重世界观 / 克系修仙)</option>
            <option value="番茄脑洞">番茄脑洞 (快节奏爽文 / 密集反转 / 极致逆袭)</option>
            <option value="知乎盐言">知乎盐言 (第一人称 / 强悬疑复仇 / 情绪高压)</option>
            <option value="海外短剧">海外短剧 (高概念冲突 / 强钩子钩锁 / 豪门狼人)</option>
          </select>
        </div>

        <!-- 模式 A：AI 宏观创世推演 -->
        <div v-if="form.genesis_mode" class="space-y-2">
          <label class="text-xs text-brand-amber font-semibold flex items-center justify-between">
            <span>核心灵感与创世梗概 (Core Concept)：</span>
            <span class="text-[10px] text-ink-500 font-normal">支持一两句话或详细梗概</span>
          </label>
          <textarea 
            v-model="form.concept" 
            rows="5" 
            class="w-full bg-atelier-950 border border-atelier-750 rounded-lg p-2.5 text-xs text-ink-100 focus:outline-none focus:border-brand-amber/60 leading-relaxed font-serif resize-none" 
            placeholder="例如：凡人无灵根，天道被不可名状的神明寄生；修士飞升实为充当神明祭品；主角偶然获得一枚能吞噬神明诅咒的凡骨铁印，在三千年宗门血腥秩序下隐忍弑神..."></textarea>
          <p class="text-[11px] text-ink-400 leading-relaxed">
            💡 AI 将自动深度推演：天道绝对公理、10 级代价晋升天梯、四大对抗势力暗线、1~4 卷分卷主线任务链，并自动播种开局伏笔池。
          </p>
        </div>

        <!-- 模式 B：基础快速创建 -->
        <div v-else>
          <label class="text-xs font-medium text-ink-300">主角初始姓名与等级：</label>
          <input 
            v-model="form.protagonist_name" 
            class="w-full mt-1.5 bg-atelier-950 border border-atelier-750 rounded-lg p-2.5 text-xs text-ink-100 focus:outline-none focus:border-brand-amber/60" 
            placeholder="例如：顾渊 (凡人胎骨 / 练气一层)">
        </div>
      </div>

      <!-- 底部操作按钮 -->
      <div class="flex justify-end gap-2.5 pt-3 border-t border-atelier-750">
        <button 
          @click="state.showNewProjectModal = false" 
          :disabled="isSubmitting" 
          class="px-3.5 py-1.5 bg-atelier-850 hover:bg-atelier-800 text-xs text-ink-300 rounded-lg transition cursor-pointer">
          取消
        </button>
        <button 
          @click="submitCreate" 
          :disabled="isSubmitting || !form.title.trim()" 
          class="px-4 py-1.5 bg-brand-amber hover:bg-brand-amber-hover text-atelier-950 font-bold text-xs rounded-lg shadow-amber-glow transition flex items-center gap-1.5 cursor-pointer disabled:opacity-50">
          <span v-if="isSubmitting" class="w-3 h-3 border-2 border-atelier-950 border-t-transparent rounded-full animate-spin"></span>
          <span>{{ isSubmitting ? '正在创世推演...' : (form.genesis_mode ? '🌌 立即启动 AI 创世' : '创建作品') }}</span>
        </button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { reactive, ref } from 'vue';
import { state, actions, notify } from '../../stores/appState';
import { api } from '../../api/client';
import { BookPlus } from 'lucide-vue-next';

const isSubmitting = ref(false);

const form = reactive({
  title: '',
  target_platform: '起点仙侠',
  genesis_mode: true,
  concept: '',
  protagonist_name: '',
});

async function submitCreate() {
  if (!form.title.trim()) return;
  isSubmitting.value = true;
  try {
    let created;
    if (form.genesis_mode) {
      created = await api.bootstrapProject({
        title: form.title,
        target_platform: form.target_platform,
        concept: form.concept || form.title,
      });
    } else {
      created = await api.createProject({
        title: form.title,
        target_platform: form.target_platform,
        protagonist: {
          name_and_level: form.protagonist_name || '主角 (初始)',
          inventory: '',
          core_goal: '探查真相',
        },
      });
    }
    await actions.loadProjects();
    await actions.selectProject(created.id);
    state.showNewProjectModal = false;
    notify('作品工程创建成功', `${created.title} 已就绪`, 'success');
  } catch (e) {
    notify('创建工程失败', e.message, 'error');
  } finally {
    isSubmitting.value = false;
  }
}
</script>
