import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    port: 35173,
    host: true, // 暴露局域网地址，手机扫码可访问
    // host:true 让 Vite 监听 0.0.0.0，但 HMR WebSocket 客户端脚本会被注入为局域网 IP，
    // 导致从 localhost 访问时热更 WebSocket 连不上。
    // 显式指定 hmr.host=localhost，强制浏览器始终连回本机。
    hmr: {
      host: 'localhost',
      clientPort: 35173,
    },
    proxy: {
      '/api': {
        target: 'http://localhost:8000',
        changeOrigin: true,
        // SSE 流式输出：http-proxy 不会主动 flush 响应头，导致浏览器等全量数据才收到内容。
        // 修法：proxyRes 事件里提前 flushHeaders()，让 http-proxy 继续正常 pipe body。
        // 注意：不要手动 pipe，否则与 http-proxy 内部 pipe 形成双写。
        configure: (proxy) => {
          proxy.on('proxyRes', (proxyRes, _req, res) => {
            const ct = proxyRes.headers['content-type'] ?? ''
            if (ct.includes('text/event-stream')) {
              res.setHeader('Cache-Control', 'no-cache')
              res.setHeader('Connection', 'keep-alive')
              res.setHeader('X-Accel-Buffering', 'no')
              res.flushHeaders()
            }
          })
        },
      },
    },
  },
})
