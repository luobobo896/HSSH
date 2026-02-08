import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    port: 18080,
    host: '0.0.0.0',
    proxy: {
      '/api': {
        target: 'http://localhost:18081',
        changeOrigin: true,
      },
      '/ws': {
        target: 'ws://localhost:18081',
        ws: true,
      },
    },
  },
})
