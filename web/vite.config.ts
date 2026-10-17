import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

// Builds straight into the Go server's embed dir.
export default defineConfig({
  plugins: [react()],
  build: { outDir: '../server/internal/httpapi/static', emptyOutDir: true, chunkSizeWarningLimit: 900 },
  server: { proxy: { '/api': process.env.HUB_DEV_API ?? 'http://127.0.0.1:8088' } },
});
