import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    host: '0.0.0.0',
    port: 8083,
    proxy: {
      '/api': {
        target: process.env.VITE_API_URL ?? 'http://localhost:4083',
        changeOrigin: true,
      },
      '/healthz': {
        target: process.env.VITE_API_URL ?? 'http://localhost:4083',
        changeOrigin: true,
      },
    },
  },
})
