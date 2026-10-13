import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'
import wails from '@wailsio/runtime/plugins/vite'
import { fileURLToPath, URL } from 'node:url'

// https://vitejs.dev/config/
export default defineConfig(({ mode }) => {
  const isDev = mode === 'development' || process.env.DEV === 'true'

  return {
    resolve: {
      alias: {
        '@': fileURLToPath(new URL('./src', import.meta.url)),
      },
    },
    server: {
      host: '127.0.0.1',
      port: Number(process.env.WAILS_VITE_PORT) || 9245,
      strictPort: true,
      sourcemapIgnoreList: false,
    },
    build: {
      sourcemap: isDev ? true : 'hidden',
      // Vite 8 / Rolldown: do not force esbuild (not installed by default).
      // false = readable stacks in DEV builds; true = default Oxc minify in production.
      minify: isDev ? false : true,
    },
    optimizeDeps: {
      include: [
        '@uiw/react-codemirror',
        '@codemirror/view',
        '@codemirror/state',
        '@codemirror/lang-javascript',
        '@codemirror/lang-go',
      ],
    },
    plugins: [react(), wails('./bindings')],
    test: {
      environment: 'jsdom',
      include: ['src/**/*.test.ts', 'src/**/*.test.tsx'],
      setupFiles: ['./src/test/setup.ts'],
    },
  }
})
