import { marked } from 'marked';

// 配置 marked：支持 GFM 表格、列表、换行转为 <br>
marked.setOptions({
  gfm: true,
  breaks: true,
});

/**
 * 将 Markdown 字符串解析为带排版的 HTML
 * @param {string} content 
 * @returns {string} HTML 字符串
 */
export function renderMarkdown(content) {
  if (!content) return '';
  try {
    return marked.parse(content);
  } catch (err) {
    console.error('Failed to parse markdown:', err);
    return `<p class="whitespace-pre-wrap">${escapeHtml(content)}</p>`;
  }
}

/**
 * 基础 HTML 转义（防范解析异常时的裸文本注入）
 */
function escapeHtml(str) {
  return String(str)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#039;');
}

/**
 * 安全复制文本到剪贴板
 * @param {string} text 
 * @returns {Promise<boolean>}
 */
export async function copyToClipboard(text) {
  if (!text) return false;
  if (navigator?.clipboard?.writeText) {
    try {
      await navigator.clipboard.writeText(text);
      return true;
    } catch (_) {}
  }

  // 兜底方案
  try {
    const textarea = document.createElement('textarea');
    textarea.value = text;
    textarea.style.position = 'fixed';
    textarea.style.opacity = '0';
    document.body.appendChild(textarea);
    textarea.select();
    const success = document.execCommand('copy');
    document.body.removeChild(textarea);
    return success;
  } catch (_) {
    return false;
  }
}
