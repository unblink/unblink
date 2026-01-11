import tailwindcss from '@tailwindcss/vite';
import { defineConfig, loadEnv } from 'vite';
import solidPlugin from 'vite-plugin-solid';
import devtools from 'solid-devtools/vite';

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, '.');
  const target = env.VITE_RELAY_API_URL;
  if (!target) throw new Error("VITE_RELAY_API_URL is not configured in .env");

  return {
    plugins: [devtools(), solidPlugin(), tailwindcss()],
    server: {
      port: 3000,
      proxy: {
        '/relay': {
          target,
          changeOrigin: true,
          secure: false,
          rewrite: (path: string) => path.replace(/^\/relay/, ''),
        },
      },
    },
    build: {
      target: 'esnext',
    },
    // SPA fallback - serve index.html for /node/* routes
    appType: 'spa',
  };
});
