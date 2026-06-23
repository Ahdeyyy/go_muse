import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';

// The build is emitted directly into the Go binary's embed directory so a
// subsequent `go build ./cmd/web` ships the latest UI. During development,
// `vite` proxies /api to the Go server (run `go run ./cmd/web` alongside).
export default defineConfig({
  plugins: [svelte()],
  base: './',
  build: {
    outDir: '../internal/webapi/dist',
    emptyOutDir: true,
  },
  server: {
    port: 5173,
    proxy: {
      '/api': 'http://127.0.0.1:8765',
    },
  },
});
