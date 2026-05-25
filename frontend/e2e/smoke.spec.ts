import { test, expect } from '@playwright/test';

const BASE = process.env.BASE_URL || 'https://audiofile.app';

test.describe('Smoke — production site', () => {
	test('login page loads correctly', async ({ page }) => {
		await page.goto(`${BASE}/login/`);
		await expect(page.locator('h1')).toContainText('Welcome back');
		await expect(page.locator('input[type="email"]')).toBeVisible();
		await expect(page.locator('input[type="password"]')).toBeVisible();
		await expect(page.locator('#email-signin-btn')).toBeVisible();
		await expect(page.locator('text=Passkey')).not.toBeVisible();
	});

	test('signup page loads correctly', async ({ page }) => {
		await page.goto(`${BASE}/signup/`);
		await expect(page.locator('h1')).toContainText('Start your collection');
		await expect(page.locator('input[type="email"]')).toBeVisible();
		await expect(page.locator('#signup-btn')).toBeVisible();
	});

	test('signup link works from login', async ({ page }) => {
		await page.goto(`${BASE}/login/`);
		await page.click('text=Create an account');
		await expect(page.locator('h1')).toContainText('Start your collection');
	});

	test('login link works from signup', async ({ page }) => {
		await page.goto(`${BASE}/signup/`);
		await page.click('text=Sign in');
		await expect(page.locator('h1')).toContainText('Welcome back');
	});

	test('protected pages redirect to login', async ({ page }) => {
		await page.goto(`${BASE}/collection/`);
		await expect(page).toHaveURL(/\/login/);
	});

	test('branding is AudioFile', async ({ page }) => {
		await page.goto(`${BASE}/login/`);
		const navBrand = page.locator('.font-serif.text-gold').first();
		await expect(navBrand).toContainText('AudioFile');
		await expect(page.locator('text=CrateKeeper')).not.toBeVisible();
	});

	test('health endpoint', async ({ request }) => {
		const resp = await request.get(`${BASE}/api/health`);
		expect(resp.status()).toBe(200);
		const body = await resp.json();
		expect(body.status).toBe('ok');
	});

	test('release search endpoint is available', async ({ request }) => {
		const resp = await request.get(`${BASE}/api/releases/search?q=kind%20of%20blue%20miles%20davis`);
		expect(resp.status()).toBe(200);
		const body = await resp.json();
		expect(Array.isArray(body)).toBe(true);
	});

	test('barcode scan endpoint is available', async ({ request }) => {
		const resp = await request.post(`${BASE}/api/releases/scan`, {
			data: { barcode: '018771210510' },
		});
		expect(resp.status()).toBe(200);
		const body = await resp.json();
		expect(body.barcode).toBe('018771210510');
		expect(Array.isArray(body.results)).toBe(true);
	});
});
