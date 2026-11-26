import { test, expect } from '@playwright/test';

test.describe('Login page', () => {
	test('loads and shows welcome message', async ({ page }) => {
		await page.goto('/login');
		await expect(page.locator('text=Welcome back')).toBeVisible();
		await expect(page).toHaveTitle(/Sign In.*AudioFile/);
	});

	test('is excluded from search indexing', async ({ page }) => {
		await page.goto('/login');
		await expect(page.locator('meta[name="robots"]')).toHaveAttribute('content', 'noindex, nofollow');
	});

	test('does not show passkey sign-in', async ({ page }) => {
		await page.goto('/login');
		await expect(page.locator('text=Passkey')).not.toBeVisible();
	});

	test('shows email/password fields', async ({ page }) => {
		await page.goto('/login');
		await expect(page.locator('input[type="email"]')).toBeVisible();
		await expect(page.locator('input[type="password"]')).toBeVisible();
		await expect(page.locator('#email-signin-btn')).toBeVisible();
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
