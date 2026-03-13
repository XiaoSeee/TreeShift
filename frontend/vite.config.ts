import react from "@vitejs/plugin-react";
import { defineConfig } from "vitest/config";

/**
 * viteConfig 定义前端构建、开发服务器和测试环境配置。
 */
export default defineConfig({
  plugins: [react()],
  server: {
    host: "127.0.0.1",
    port: 34115,
  },
  test: {
    environment: "jsdom",
    globals: true,
    setupFiles: [],
  },
});
