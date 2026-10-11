import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';
import path from 'path';

// 开发模式与正式/生产端口彻底分离：
// - 前端开发 Web 端口统一为 5173（Vite 标准端口，支持 process.env.VITE_PORT 覆盖）
// - 后端开发接口端口默认为 28981（与正式常驻服务 28980 隔离，支持 NOVEL_STUDIO_PORT / NOVEL_STUDIO_BACKEND_URL 覆盖）
// - 开发模式禁止默认弹出浏览器窗口
const devWebPort = Number(process.env.VITE_PORT) || 5173;
const devApiPort = Number(process.env.NOVEL_STUDIO_PORT) || 28981;
const devApiTarget = process.env.NOVEL_STUDIO_BACKEND_URL || `http://127.0.0.1:${devApiPort}`;

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    port: devWebPort,
    strictPort: true,
    open: false,
    proxy: {
      '/api': {
        target: devApiTarget,
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
});
