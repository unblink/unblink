import tailwindcss from '@tailwindcss/vite';
import devtools from 'solid-devtools/vite';
import { defineConfig } from 'vite';
import solidPlugin from 'vite-plugin-solid';

export default defineConfig(({ mode }) => {
  return {
    plugins: [devtools(), solidPlugin(), tailwindcss()],
    server: {
      port: 3000,
    },
    build: {
      target: 'esnext',
    },
    appType: 'spa',
  };
});