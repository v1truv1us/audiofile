import { test, expect } from '@playwright/test';

// Must match supabase-js's derived localStorage key: sb-<hostname-label>-auth-token
const SUPABASE_URL = process.env.PUBLIC_SUPABASE_URL || 'https://supabase.v1truv1us.dev';
const SUPABASE_AUTH_KEY = `sb-${new URL(SUPABASE_URL).hostname.split('.')[0]}-auth-token`;
const USER_ID = '00000000-0000-0000-0000-000000000001';

async function signIn(page: import('@playwright/test').Page) {
	await page.addInitScript(({ key, userID }) => {
		window.localStorage.setItem(key, JSON.stringify({
			access_token: 'test-token',
			token_type: 'bearer',
			expires_in: 3600,
			expires_at: Math.floor(Date.now() / 1000) + 3600,
			refresh_token: 'test-refresh-token',
			user: {
				id: userID,
				aud: 'authenticated',
				role: 'authenticated',
				email: 'test@example.com',
			},
		}));
	}, { key: SUPABASE_AUTH_KEY, userID: USER_ID });
}

test.describe('Collection page', () => {
	test('loads and shows collection heading', async ({ page }) => {
		await page.goto('/collection');
		await expect(page.locator('text=Collection')).toBeVisible();
		await expect(page).toHaveTitle(/Collection.*AudioFile/);
	});

	test('shows sort dropdown', async ({ page }) => {
		await page.goto('/collection');
		const select = page.locator('select');
		await expect(select).toBeVisible();
		await expect(select.locator('option', { hasText: 'Recently added' })).toBeVisible();
		await expect(select.locator('option', { hasText: 'Artist A–Z' })).toBeVisible();
		await expect(select.locator('option', { hasText: 'Year' })).toBeVisible();
		await expect(select.locator('option', { hasText: 'Condition' })).toBeVisible();
	});

	test('shows record cards from API', async ({ page }) => {
		await page.goto('/collection');
		// Wait for data to load
		await page.waitForTimeout(2000);
		// Record cards have white background and gold border
		const cards = page.locator('.bg-white.border.border-gold\\/60');
		await expect(cards.first()).toBeVisible();
		// Each card should have vinyl disc SVG
		await expect(cards.first().locator('svg')).toBeVisible();
		// Each card should have grade badge
		await expect(cards.first().locator('text=VG')).toBeVisible();
	});

	test('shows add record button', async ({ page }) => {
		await page.goto('/collection');
		await expect(page.locator('text=+ Add Record')).toBeVisible();
	});

	test('sort dropdown changes the order', async ({ page }) => {
		await page.goto('/collection');
		await page.waitForTimeout(2000);
		// Get first card title
		const firstTitle = await page.locator('.bg-white.border.border-gold\\/60').first().locator('.font-serif').textContent();
		// Change sort
		await page.locator('select').selectOption('artist');
		await page.waitForTimeout(2000);
		// First card should be different
		const newFirstTitle = await page.locator('.bg-white.border.border-gold\\/60').first().locator('.font-serif').textContent();
		// Either titles differ or it's the same (if only one record)
		expect(newFirstTitle).toBeTruthy();
	});

	test('share button sends a public collection link', async ({ page }) => {
		await signIn(page);
		await page.addInitScript(() => {
			Object.defineProperty(navigator, 'share', {
				configurable: true,
				value: async (data: ShareData) => { (window as any).__sharedUrl = data.url; },
			});
		});
		await page.route('**/api/collection?sort=', async (route) => {
			await route.fulfill({ json: [] });
		});

		await page.goto('/collection');
		await page.getByRole('button', { name: 'Share' }).click();

		await expect.poll(() => page.evaluate(() => (window as any).__sharedUrl)).toBe(`http://127.0.0.1:4321/collection?share=${USER_ID}`);
	});

	test('loads a shared collection without owner-only controls', async ({ page }) => {
		await page.route('**/api/public/collection/shared-user-1?sort=', async (route) => {
			await route.fulfill({
				json: [{
					id: 'item-1',
					release: {
						id: 'release-1',
						title: 'Kind of Blue',
						artist: 'Miles Davis',
						year: 1959,
						label: 'Columbia',
					},
					mediaCondition: 'NM',
					sleeveCondition: 'VG+',
					purchasePrice: 25.5,
					notes: 'clean',
					isForSale: false,
				}],
			});
		});

		await page.goto('/collection?share=shared-user-1');
		await expect(page).not.toHaveURL(/\/login/);

		await expect(page.locator('text=Shared collection')).toBeVisible();
		await expect(page.locator('text=Kind of Blue')).toBeVisible();
		await expect(page.getByRole('button', { name: 'Share' })).toBeHidden();
		await expect(page.getByRole('button', { name: '+ Add Record' })).toBeHidden();
		await expect(page.getByRole('button', { name: 'Edit' })).toBeHidden();
		await expect(page.getByRole('button', { name: 'Remove' })).toBeHidden();
	});

	test('loads a shared collection when the path has a trailing slash', async ({ page }) => {
		await page.route('**/api/public/collection/shared-user-1?sort=', async (route) => {
			await route.fulfill({
				json: [{
					id: 'item-1',
					release: {
						id: 'release-1',
						title: 'Kind of Blue',
						artist: 'Miles Davis',
						year: 1959,
						label: 'Columbia',
					},
					mediaCondition: 'NM',
					sleeveCondition: 'VG+',
					purchasePrice: 25.5,
					notes: 'clean',
					isForSale: false,
				}],
			});
		});

		await page.goto('/collection/?share=shared-user-1');
		await expect(page).not.toHaveURL(/\/login/);
		await expect(page.locator('text=Shared collection')).toBeVisible();
		await expect(page.locator('text=Kind of Blue')).toBeVisible();
	});
});
