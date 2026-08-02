import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import vueDevTools from 'vite-plugin-vue-devtools'

export default defineConfig({
  plugins: [
    vue(),
    vueDevTools(),
  ],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    host: '0.0.0.0', // Allows access from any device on your network
    port: 5173,      // Your specified port
    strictPort: true, // Optional: stops the server if port 5173 is in use
  
    proxy: {
      '/api': {
        target: 'http://localhost:7842',
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/api/, '') 
      },
    },
  },
  
})