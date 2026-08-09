// @ts-check
import { defineConfig } from 'astro/config';

import svelte from '@astrojs/svelte';
import sitemap from '@astrojs/sitemap';
import tailwindcss from '@tailwindcss/vite';
import sentry from '@sentry/astro';
import basicSsl from '@vitejs/plugin-basic-ssl';

// https://astro.build/config
export default defineConfig({
  site: 'https://audiofile.app',
  integrations: [
    svelte(),
    sitemap({
      filter: (page) => {
        const publicPaths = ['/', '/pricing', '/login', '/signup', '/privacy', '/terms', '/refund'];
        const path = new URL(page).pathname.replace(/\/$/, '') || '/';
        return publicPaths.includes(path);
      },
    }),
    sentry({
      dsn: process.env.PUBLIC_SENTRY_DSN,
      environment: process.env.PUBLIC_ENVIRONMENT || 'development',
      release: 'audiofile@0.2.0',
      tracesSampleRate: 0.2,
      replaysSessionSampleRate: 0,
      replaysOnErrorSampleRate: 1.0,
    }),
  ],
  server: {
    host: '0.0.0.0',
    port: 4321,
    allowedHosts: true, // allow Tailscale funnel hostnames in dev
  },
  vite: {
    plugins: [tailwindcss(), basicSsl()],
    cacheDir: '/tmp/vite-cache-build',
    server: {
      proxy: {
        '/api': {
          target: 'http://localhost:8080',
          changeOrigin: true,
        },
      },
    },
  }
});