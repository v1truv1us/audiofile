import { test, expect } from '@playwright/test';

test.describe('Login page', () => {
	test('loads and shows welcome message', async ({ page }) => {
		await page.goto('/login');
		await expect(page.locator('text=Welcome back')).toBeVisible();
		await expect(page).toHaveTitle(/Sign In.*CrateKeeper/);
	});

	test('shows passkey sign-in button', async ({ page }) => {
		await page.goto('/login');
		await expect(page.locator('text=Sign in with Passkey')).toBeVisible();
	});

	test('shows email/password fallback fields', async ({ page }) => {
		await page.goto('/login');
		await expect(page.locator('input[type="email"]')).toBeVisible();
		await expect(page.locator('input[type="password"]')).toBeVisible();
		await expect(page.locator('text=Sign in with email')).toBeVisible();
	});

	test('shows create account link', async ({ page }) => {
		await page.goto('/login');
		await expect(page.locator('text=Create an account')).toBeVisible();
	});

	test('renders vinyl logo on login', async ({ page }) => {
		await page.goto('/login');
		const logoSvg = page.locator('.text-center svg');
		await expect(logoSvg).toBeVisible();
	});
});
