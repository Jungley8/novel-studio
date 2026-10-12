import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';
import path from 'path';

// 开发模式与正式/生产端口彻底分离：
// - 前端开发 Web 端口默认 5183（避开工作区内其他项目占用的标准 5173，支持 process.env.VITE_PORT 覆盖）
// - 后端开发接口端口默认为 28981（与正式常驻服务 28980 隔离，支持 NOVEL_STUDIO_PORT / NOVEL_STUDIO_BACKEND_URL 覆盖）
// - 不自动弹出浏览器窗口 (open: false)
const devWebPort = Number(process.env.VITE_PORT) || 5183;
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
