import { defineConfig } from '@playwright/test';

// Smoke tests de la UI en un navegador, con el puente de Wails simulado
// (e2e/bridge.ts). No arrancan el backend Go ni tocan la red.
export default defineConfig({
  testDir: './e2e',
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? 'github' : 'list',
  use: {
    baseURL: 'http://localhost:5173',
    // En CI se usa el Chromium de Playwright; en local, el Chrome instalado
    // (Playwright no distribuye Chromium para macOS antiguos).
    channel: process.env.CI ? undefined : 'chrome',
    viewport: { width: 1180, height: 780 },
    trace: 'retain-on-failure',
  },
  webServer: {
    command: 'npm run dev',
    url: 'http://localhost:5173',
    reuseExistingServer: !process.env.CI,
  },
});
