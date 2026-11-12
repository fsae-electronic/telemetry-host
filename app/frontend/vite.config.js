import { defineConfig } from 'vite'
import { svelte } from '@sveltejs/vite-plugin-svelte'

// https://vite.dev/config/
export default defineConfig({
  plugins: [svelte()],
  build: {
    outDir: '../cmd/app/frontend',
    emptyOutDir: true,
  },
  server: {
    // index.html uses ../cmd/app/logo.png (the .exe icon) as favicon
    fs: { allow: ['..'] },
    proxy: {
      '/api': 'http://localhost:8080',
      '/ws': { target: 'ws://localhost:8080', ws: true },
    },
  },
})
