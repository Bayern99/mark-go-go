import react from '@vitejs/plugin-react'
import { playwright } from '@vitest/browser-playwright'
import { defineConfig } from 'vitest/config'

export default defineConfig({
  base: './',
  plugins: [react()],
  define: {
    __VUE_OPTIONS_API__: 'true',
    __VUE_PROD_DEVTOOLS__: 'false',
    __VUE_PROD_HYDRATION_MISMATCH_DETAILS__: 'false',
  },
  server: {
    port: 5173,
    strictPort: true,
    // Dev-only: let the typeset bench (?raw imports under docs/) read files
    // outside the web root. Does not affect `vite build`.
    fs: { allow: ['..'] },
    watch: {
      ignored: ['**/.mygo/**', '**/build/**'],
    },
  },
  test: {
    include: ['test/**/*.test.{ts,tsx}'],
    setupFiles: ['test/setup.ts'],
    browser: {
      enabled: true,
      headless: true,
      provider: playwright({ launchOptions: { args: ['--no-sandbox'] } }),
      instances: [{ browser: 'chromium', viewport: { width: 1280, height: 900 } }],
    },
  },
})
