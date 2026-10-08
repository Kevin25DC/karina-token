import { expect, test, type Page } from '@playwright/test';
import { anthropicMeta, anthropicState, defaults, installBridge } from './bridge';

/** Falla el test si la página lanza una excepción no capturada. */
function trackErrors(page: Page): string[] {
  const errors: string[] = [];
  page.on('pageerror', (e) => errors.push(e.message));
  return errors;
}

test('arranca en el panel sin proveedores', async ({ page }) => {
  const errors = trackErrors(page);
  await installBridge(page);
  await page.goto('/');

  await expect(page.getByRole('heading', { name: 'Uso de IA' })).toBeVisible();
  await expect(page.getByText('No hay proveedores conectados')).toBeVisible();
  await expect(page.getByText('v9.9.9')).toBeVisible();
  expect(errors).toEqual([]);
});

test('muestra el onboarding en el primer arranque', async ({ page }) => {
  const errors = trackErrors(page);
  await installBridge(page, { config: { ...defaults.config, onboarding_done: false } });
  await page.goto('/');

  await expect(page.getByRole('heading', { name: /Bienvenido a Karina/ })).toBeVisible();
  expect(errors).toEqual([]);
});

test('navega por todas las secciones', async ({ page }) => {
  const errors = trackErrors(page);
  await installBridge(page, { providers: [anthropicMeta], states: [anthropicState] });
  await page.goto('/');

  await expect(page.getByRole('heading', { name: 'Uso de IA' })).toBeVisible();
  await expect(page.getByText('Anthropic Claude').first()).toBeVisible();

  const nav = page.getByRole('navigation');
  await nav.getByRole('button', { name: /Historial de uso/ }).click();
  await expect(page.getByRole('heading', { name: 'Historial de uso', exact: true })).toBeVisible();

  await nav.getByRole('button', { name: /Claude Code/ }).click();
  await expect(page.getByRole('heading', { name: 'Claude Code', exact: true })).toBeVisible();

  await nav.getByRole('button', { name: /Ajustes/ }).click();
  await expect(page.getByRole('heading', { name: 'Ajustes', exact: true })).toBeVisible();

  // Los atajos de teclado también cambian de sección.
  await page.keyboard.press('1');
  await expect(page.getByRole('heading', { name: 'Uso de IA' })).toBeVisible();
  expect(errors).toEqual([]);
});
