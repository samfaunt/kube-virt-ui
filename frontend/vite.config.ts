import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

export default defineConfig({
  plugins: [react()],
  server: {
    // ws: consoles are websockets under /api.
    proxy: { '/api': { target: 'http://localhost:8080', ws: true } },
  },
})
