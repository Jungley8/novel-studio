import { createApp } from 'vue';
import App from './App.vue';
import './assets/main.css';
import { dialogs } from './stores/appState';

// 全局跨平台适配：兼容 Desktop 原生窗口 (Wails/WKWebView) 与 Web 浏览器环境
if (typeof window !== 'undefined') {
  window.$dialog = dialogs;
  window.dialog = dialogs;

  // 拦截全局 alert 调用，使用高保真应用内对话框替代系统原生弹窗
  window.alert = (msg) => {
    dialogs.alert({
      title: '系统提示',
      message: String(msg ?? ''),
      type: 'info',
    });
  };
}

const app = createApp(App);

app.config.errorHandler = (err, instance, info) => {
  console.error('[Global Vue Error]', err, info);
};

app.mount('#app');
