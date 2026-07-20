import { test, expect } from '@playwright/test';

test.describe('Wishlist page', () => {
	test('loads and shows wishlist heading', async ({ page }) => {
		await page.goto('/wishlist');
		await expect(page.locator('text=Wishlist')).toBeVisible();
		await expect(page).toHaveTitle(/Wishlist.*AudioFile/);
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

	async function mockSharedWishlist(page: import('@playwright/test').Page) {
		await page.route('**/api/public/wishlist/shared-user-1', async (route) => {
			await route.fulfill({
				json: [{
					id: 'wish-1',
					title: 'Kind of Blue',
					artist: 'Miles Davis',
					priority: 2,
					targetPrice: 20,
					notes: 'first press',
					label: 'Columbia',
				}],
			});
		});
	}

	async function expectSharedWishlistView(page: import('@playwright/test').Page) {
		await expect(page.locator('text=Shared records to hunt for')).toBeVisible();
		await expect(page.locator('text=Kind of Blue')).toBeVisible();
		await expect(page.getByRole('button', { name: 'Share' })).toBeHidden();
		await expect(page.getByRole('button', { name: '+ Add to Wishlist' })).toBeHidden();
		await expect(page.getByRole('button', { name: 'Edit' })).toBeHidden();
		await expect(page.getByRole('button', { name: 'Purchased' })).toBeHidden();
		await expect(page.getByRole('button', { name: 'Remove' })).toBeHidden();
	}

	test('loads a shared wishlist without owner-only controls', async ({ page }) => {
		await mockSharedWishlist(page);
		await page.goto('/wishlist?share=shared-user-1');
		await expect(page).not.toHaveURL(/\/login/);
		await expectSharedWishlistView(page);
	});

	test('loads a shared wishlist when the path has a trailing slash', async ({ page }) => {
		await mockSharedWishlist(page);
		await page.goto('/wishlist/?share=shared-user-1');
		await expect(page).not.toHaveURL(/\/login/);
		await expectSharedWishlistView(page);
	});

	test('hides the claim button from logged-out visitors', async ({ page }) => {
		await mockSharedWishlist(page);
		await page.goto('/wishlist?share=shared-user-1');
		await expect(page.locator('text=Kind of Blue')).toBeVisible();
		await expect(page.getByRole('button', { name: 'Add to my shared wishlists' })).toBeHidden();
	});
});
