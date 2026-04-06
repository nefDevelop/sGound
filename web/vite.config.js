import { defineConfig, loadEnv } from 'vite'
import { svelte } from '@sveltejs/vite-plugin-svelte'

// https://vite.dev/config/
export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')
  const backendPort = env.SGOUND_PORT || '8080'

  return {
    plugins: [svelte()],
    server: {
      watch: {
        usePolling: true
      },
      proxy: {
        '/api': `http://localhost:${backendPort}`,
        '/rest': `http://localhost:${backendPort}`
      }
    }
  }
})
