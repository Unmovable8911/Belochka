/// <reference types="vitest/config" />
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import path from 'path'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/__tests__/setup.ts'],
  },
  server: {
    port: 53137,
    proxy: {
      '/api': {
        target: 'http://localhost:53136',
        changeOrigin: true,
      },
      '/ws': {
        target: 'ws://localhost:53136',
        ws: true,
      },
    },
  },
})
