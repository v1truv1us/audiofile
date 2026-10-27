import { test, expect } from '@playwright/test';

// Must match supabase-js's derived localStorage key: sb-<hostname-label>-auth-token
const SUPABASE_URL = process.env.PUBLIC_SUPABASE_URL || 'https://supabase.v1truv1us.dev';
const SUPABASE_AUTH_KEY = `sb-${new URL(SUPABASE_URL).hostname.split('.')[0]}-auth-token`;

async function signIn(page: import('@playwright/test').Page) {
	await page.addInitScript((key) => {
		window.localStorage.setItem(key, JSON.stringify({
			access_token: 'test-token',
			token_type: 'bearer',
			expires_in: 3600,
			expires_at: Math.floor(Date.now() / 1000) + 3600,
			refresh_token: 'test-refresh-token',
			user: {
				id: '00000000-0000-0000-0000-000000000001',
				aud: 'authenticated',
				role: 'authenticated',
				email: 'test@example.com',
			},
		}));
	}, SUPABASE_AUTH_KEY);
}

test.describe('Collection add form', () => {
	test.beforeEach(async ({ page }) => {
		await signIn(page);
		await page.route('**/api/collection?sort=', async (route) => {
			await route.fulfill({ json: [] });
		});
		await page.route('**/api/releases/search?q=*', async (route) => {
			await route.fulfill({ json: [{ title: 'Kind of Blue', artist: 'Miles Davis', year: 1959, label: 'Columbia', coverUrl: 'https://img.example/kind.jpg' }] });
		});
		await page.route('**/api/releases/scan', async (route) => {
			const body = route.request().postDataJSON() as { barcode?: string };
			await route.fulfill({ json: { barcode: body.barcode, results: [{ title: '40oz. To Freedom', artist: 'Sublime', year: 1992, label: 'Skunk Records', coverUrl: 'https://img.example/sublime.jpg' }] } });
		});
	});

	test('opens add form, searches releases, and fills fields from selected result', async ({ page }) => {
		await page.goto('/collection');
		await page.getByRole('button', { name: '+ Add Record' }).click();

		await expect(page.getByLabel('Search releases')).toBeVisible();
		await page.getByLabel('Search releases').fill('kind of blue miles davis');
		await page.getByRole('button', { name: 'Search releases' }).click();
		await page.getByRole('button', { name: /Kind of Blue/ }).click();

		await expect(page.getByLabel('Title')).toHaveValue('Kind of Blue');
		await expect(page.getByLabel('Artist')).toHaveValue('Miles Davis');
		await expect(page.getByLabel('Year')).toHaveValue('1959');
		await expect(page.getByLabel('Label')).toHaveValue('Columbia');
	});

	test('uses manual barcode input to find and fill a release', async ({ page }) => {
		await page.goto('/collection');
		await page.getByRole('button', { name: '+ Add Record' }).click();

		await page.getByLabel('Or enter barcode').fill('018771210510');
		await page.getByRole('button', { name: 'Scan barcode', exact: true }).click();
		await page.getByRole('button', { name: /40oz\. To Freedom/ }).click();

		await expect(page.getByLabel('Title')).toHaveValue('40oz. To Freedom');
		await expect(page.getByLabel('Artist')).toHaveValue('Sublime');
		await expect(page.getByLabel('Year')).toHaveValue('1992');
		await expect(page.getByLabel('Label')).toHaveValue('Skunk Records');
	});
});
