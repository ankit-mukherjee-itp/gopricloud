import path from 'node:path'
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

const BACKEND_URL = 'http://localhost:8080'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    proxy: {
      // '/signup' is also a client-side route (the signup page), so a
      // plain browser navigation (GET) must fall through to the SPA
      // instead of hitting the backend's POST-only /signup endpoint.
      '/signup': {
        target: BACKEND_URL,
        bypass(req) {
          if (req.method !== 'POST') return req.url
        },
      },
      '/signin': BACKEND_URL,
      '/refresh': BACKEND_URL,
      '/test': BACKEND_URL,
      '/instances': BACKEND_URL,
    },
  },
})
