import { defineConfig } from 'vite'

export default defineConfig({
  clearScreen: false,
  server: {
    strictPort: true,
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    target: 'esnext',
  },
})
