import { test, expect } from '@playwright/test';

test.describe('Wishlist page', () => {
	test('loads and shows wishlist heading', async ({ page }) => {
		await page.goto('/wishlist');
		await expect(page.locator('text=Wishlist')).toBeVisible();
		await expect(page).toHaveTitle(/Wishlist.*CrateKeeper/);
	});

	test('shows wishlist items from API', async ({ page }) => {
		await page.goto('/wishlist');
		await page.waitForTimeout(2000);
		// Wishlist rows have white background
		const rows = page.locator('.bg-white.border.border-gold\\/50');
		await expect(rows.first()).toBeVisible();
		// Each row has mini vinyl disc SVG
		await expect(rows.first().locator('svg')).toBeVisible();
		// Should show priority
		await expect(page.locator('text=High priority')).toBeVisible();
	});

	test('shows add to wishlist button', async ({ page }) => {
		await page.goto('/wishlist');
		await expect(page.locator('text=+ Add to Wishlist')).toBeVisible();
	});
});
