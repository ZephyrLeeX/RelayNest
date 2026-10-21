import { defineConfig } from 'vite';
import { fileURLToPath } from 'node:url';

export default defineConfig({
  // 产物是 Go 编译输入；独占输出目录，避免清理其他服务端文件。
  build: {
    outDir: fileURLToPath(new URL('../server/internal/webui/dist', import.meta.url)),
    emptyOutDir: true,
  },
});
