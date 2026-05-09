import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    port: 35173,
    host: true, // 暴露局域网地址，手机扫码可访问
    proxy: {
      '/api': {
        target: 'http://localhost:38080',
        changeOrigin: true,
      },
    },
  },
})

