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

test('Agentes de código solo aparece si se usan en el equipo o hay un proveedor Claude', async ({
  page,
}) => {
  await installBridge(page);
  await page.goto('/');
  const nav = page.getByRole('navigation');
  await expect(nav.getByRole('button', { name: /Ajustes/ })).toBeVisible();
  await expect(nav.getByRole('button', { name: /Agentes de código/ })).toHaveCount(0);

  await installBridge(page, { claudeCodeDetected: true });
  await page.goto('/');
  await expect(nav.getByRole('button', { name: /Agentes de código/ })).toBeVisible();
});

test('la mascota refleja el consumo', async ({ page }) => {
  const errors = trackErrors(page);
  await installBridge(page);
  await page.goto('/');
  await expect(page.getByRole('img', { name: /Kari.*sleeping/ })).toBeVisible();
  await expect(page.getByText('Conecta un proveedor y despierto')).toBeVisible();

  await installBridge(page, {
    providers: [anthropicMeta],
    states: [{ ...anthropicState, used_tokens: 950_000 }],
  });
  await page.goto('/');
  await expect(page.getByRole('img', { name: /Kari.*worried/ })).toBeVisible();
  await expect(page.getByText(/Ojo, 95% en Anthropic Claude/)).toBeVisible();
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

  await nav.getByRole('button', { name: /Agentes de código/ }).click();
  await expect(page.getByRole('heading', { name: 'Agentes de código', exact: true })).toBeVisible();

  await nav.getByRole('button', { name: /Ajustes/ }).click();
  await expect(page.getByRole('heading', { name: 'Ajustes', exact: true })).toBeVisible();

  // Los atajos de teclado también cambian de sección.
  await page.keyboard.press('1');
  await expect(page.getByRole('heading', { name: 'Uso de IA' })).toBeVisible();
  expect(errors).toEqual([]);
});
