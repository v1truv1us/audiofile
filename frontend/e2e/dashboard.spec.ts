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

test.describe('Dashboard (/dashboard)', () => {
	test('keeps protected dashboard content hidden while auth is unresolved', async ({ page }) => {
		await page.goto('/dashboard', { waitUntil: 'domcontentloaded' });
		await expect(page.locator('h1', { hasText: 'Your Collection' })).toBeHidden();
	});

	test('loads and shows the AudioFile branding', async ({ page }) => {
		await signIn(page);
		await page.goto('/dashboard');
		await expect(page.locator('text=AudioFile').first()).toBeVisible();
		await expect(page).toHaveTitle(/Dashboard.*AudioFile/);
	});

	test('shows stat cards after data loads', async ({ page }) => {
		await signIn(page);
		await page.goto('/dashboard');
		// Stat cards have espresso background
		const statCards = page.locator('.bg-espresso.rounded-lg');
		await expect(statCards).toHaveCount(3);
		// Wait for API data to populate
		await expect(page.locator('text=In the collection')).toBeVisible();
		await expect(page.locator('text=Estimated value')).toBeVisible();
		await expect(page.locator('text=On the wishlist')).toBeVisible();
	});

	test('shows recent additions with vinyl discs', async ({ page }) => {
		await signIn(page);
		await page.goto('/dashboard');
		await expect(page.locator('text=Recent additions')).toBeVisible();
		// Vinyl disc SVGs render in recent cards
		const svgs = page.locator('[aria-hidden="true"] svg');
		await expect(svgs.first()).toBeVisible();
	});

	test('has link to browse all records', async ({ page }) => {
		await signIn(page);
		await page.goto('/dashboard');
		const link = page.locator('a[href="/collection"]');
		await expect(link).toContainText('Browse all');
	});

	test('has add record button', async ({ page }) => {
		await signIn(page);
		await page.goto('/dashboard');
		await expect(page.locator('text=+ Add a Record')).toBeVisible();
	});
});

test.describe('Public landing page (/)', () => {
	test('renders marketing landing for signed-out visitors', async ({ page }) => {
		await page.goto('/', { waitUntil: 'domcontentloaded' });
		await expect(page).toHaveTitle(/Catalog Your Vinyl Collection.*AudioFile/);
		await expect(page.locator('h1', { hasText: 'vinyl collection' })).toBeVisible();
		await expect(page.locator('meta[name="robots"]')).toHaveCount(0);
	});

	test('landing CTAs link to signup and pricing', async ({ page }) => {
		await page.goto('/', { waitUntil: 'domcontentloaded' });
		await expect(page.locator('a[href="/signup"]')).toBeVisible();
		await expect(page.locator('a[href="/pricing"]').first()).toBeVisible();
	});
});
