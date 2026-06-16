import { test, expect } from '@playwright/test';

test.describe('Dashboard', () => {
	test('keeps protected dashboard content hidden while auth is unresolved', async ({ page }) => {
		await page.goto('/', { waitUntil: 'domcontentloaded' });
		await expect(page.locator('h1', { hasText: 'Your Collection' })).toBeHidden();
	});

	test('loads and shows the AudioFile branding', async ({ page }) => {
		await page.goto('/');
		await expect(page.locator('text=AudioFile').first()).toBeVisible();
		await expect(page).toHaveTitle(/Dashboard.*AudioFile/);
	});

	test('shows stat cards after data loads', async ({ page }) => {
		await page.goto('/');
		// Stat cards have espresso background
		const statCards = page.locator('.bg-espresso.rounded-lg');
		await expect(statCards).toHaveCount(3);
		// Wait for API data to populate
		await expect(page.locator('text=In the collection')).toBeVisible();
		await expect(page.locator('text=Estimated value')).toBeVisible();
		await expect(page.locator('text=On the wishlist')).toBeVisible();
	});

	test('shows recent additions with vinyl discs', async ({ page }) => {
		await page.goto('/');
		await expect(page.locator('text=Recent additions')).toBeVisible();
		// Vinyl disc SVGs render in recent cards
		const svgs = page.locator('[aria-hidden="true"] svg');
		await expect(svgs.first()).toBeVisible();
	});

	test('has link to browse all records', async ({ page }) => {
		await page.goto('/');
		const link = page.locator('a[href="/collection"]');
		await expect(link).toContainText('Browse all');
	});

	test('has add record button', async ({ page }) => {
		await page.goto('/');
		await expect(page.locator('text=+ Add a Record')).toBeVisible();
	});
});
