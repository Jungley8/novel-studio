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
        <div v-if="form.genesis_mode" class="space-y-2.5">
          <!-- 爆款灵感模版一键套用 -->
          <div>
            <div class="flex items-center justify-between mb-1.5">
              <span class="text-xs text-ink-300 font-medium">🎲 爆款模版 (一键套用)：</span>
              <span class="text-[10px] text-ink-500">点击自动填充书名与梗概</span>
            </div>
            <div class="grid grid-cols-3 gap-1.5">
              <button 
                v-for="preset in bestsellerPresets" 
                :key="preset.tag"
                type="button"
                @click="applyPreset(preset)"
                class="px-2 py-1.5 text-left bg-atelier-850 hover:bg-atelier-800 hover:border-brand-amber/50 rounded-lg border border-atelier-750 transition cursor-pointer group">
                <div class="text-[11px] font-bold text-ink-100 group-hover:text-brand-amber transition-colors flex items-center justify-between">
                  <span>{{ preset.tag }}</span>
                  <span class="text-[9px] font-mono text-ink-400 group-hover:text-ink-200">{{ preset.platformBadge }}</span>
                </div>
                <div class="text-[10px] text-ink-400 truncate mt-0.5">{{ preset.shortTitle }}</div>
              </button>
            </div>
          </div>

          <label class="text-xs text-brand-amber font-semibold flex items-center justify-between pt-1">
            <span>核心灵感与创世梗概：</span>
            <span class="text-[10px] text-ink-500 font-normal">支持一两句话或详细梗概</span>
          </label>
          <textarea 
            v-model="form.concept" 
            rows="4" 
            class="w-full bg-atelier-950 border border-atelier-750 rounded-lg p-2.5 text-xs text-ink-100 focus:outline-none focus:border-brand-amber/60 leading-relaxed font-serif resize-none" 
            placeholder="例如：凡人无灵根，天道被不可名状的神明寄生；修士飞升实为充当神明祭品；主角偶然获得一枚能吞噬神明诅咒的凡骨铁印，在三千年宗门血腥秩序下隐忍弑神..."></textarea>
          <p class="text-[11px] text-ink-400 leading-relaxed">
            💡 AI 将全自动推演：世界底层公理、10 阶战力天梯、核心对抗势力、全书分卷大纲、开局伏笔簿与关系图谱。
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

const bestsellerPresets = [
  {
    tag: '仙侠克系',
    platformBadge: '起点',
    shortTitle: '凡人弑神录',
    title: '凡人弑神录：万古血祭大典',
    platform: '起点仙侠',
    concept: '凡人无灵根，天道被不可名状的神明寄生；修士飞升实为充当神明祭品；主角偶然获得一枚能吞噬神明诅咒的凡骨铁印，在三千年宗门血腥秩序下隐忍弑神，逆炼万古仙道。',
  },
  {
    tag: '赛博修真',
    platformBadge: '番茄',
    shortTitle: '神经元飞升',
    title: '神经元飞升：重构赛博天庭',
    platform: '番茄脑洞',
    concept: '高维算法入侵修真界，灵根被改造成生化芯片，金丹成为微型聚变堆，天道成为中央算力集群。主角从底层黑市破烂义体开局，破解宗门代码防火墙，以开源之躯重构仙界。',
  },
  {
    tag: '规则怪谈',
    platformBadge: '番茄',
    shortTitle: '死亡规则推演',
    title: '诡异降临：我能推演死亡规则',
    platform: '番茄脑洞',
    concept: '诡异迷雾笼罩全球，各类致命规则不断具现，稍有触犯即刻抹杀。主角觉醒死亡倒计时与因果回溯推演能力，洞悉规则破绽，反套路猎杀诡异收容物。',
  },
  {
    tag: '反派逆袭',
    platformBadge: '番茄',
    shortTitle: '截胡天命主角',
    title: '穿成短命反派，我截胡了天命主角',
    platform: '番茄脑洞',
    concept: '穿越成开篇被退婚打脸的炮灰大少爷，深知家族即将覆灭、主角即将崛起。主角果断反向布局，利用信息差提前截胡气运至宝、收买关键配角，反手将天命主角逼入绝境。',
  },
  {
    tag: '悬疑复仇',
    platformBadge: '知乎',
    shortTitle: '第十九嫌疑人',
    title: '深渊回响：第十九个嫌疑人',
    platform: '知乎盐言',
    concept: '连环迷案背后隐藏着十五年前的警队绝密档案。主角以法医顾问身份重返故地，在层层迷雾与假线索中抽丝剥茧，步步为营设下精妙杀局，向深不可测的幕后权贵完成绝地复仇。',
  },
  {
    tag: '海外异能',
    platformBadge: '海外',
    shortTitle: '永夜王座',
    title: '永夜王座：血族始祖觉醒',
    platform: '海外短剧',
    concept: '古老纯血始祖在现代都市贫民窟苏醒，宿命契约被撕毁，月影狼族与圣殿猎人四面围剿。主角逐步夺回被封印的神圣源血，以绝对力量横扫背叛者，重铸黑夜帝国。',
  },
];

function applyPreset(preset) {
  form.title = preset.title;
  form.target_platform = preset.platform;
  form.concept = preset.concept;
  form.genesis_mode = true;
}

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
