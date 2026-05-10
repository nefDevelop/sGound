import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig, loadEnv } from 'vite';

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '');
  const backendPort = env.SGOUND_PORT || '8088';

  return {
    plugins: [sveltekit()],
    server: {
      watch: {
        usePolling: true
      },
      proxy: {
        '/api': `http://localhost:${backendPort}`,
        '/rest': `http://localhost:${backendPort}`
      }
    }
  };
});
