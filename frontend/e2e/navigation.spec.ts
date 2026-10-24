import { test, expect } from '@playwright/test';

const SUPABASE_AUTH_KEY = 'sb-bwzldaesynlruqukbnej-auth-token';
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

test.describe('Navigation', () => {
	test('nav links work from dashboard', async ({ page }) => {
		await signIn(page);
		await page.goto('/dashboard');

		await page.click('a[href="/collection"]');
		await expect(page).toHaveURL('/collection');
		await expect(page.locator('text=Collection')).toBeVisible();

		await page.click('a[href="/wishlist"]');
		await expect(page).toHaveURL('/wishlist');
		await expect(page.locator('text=Wishlist')).toBeVisible();

		await page.click('a[href="/"]');
		await expect(page).toHaveURL('/dashboard');
		await expect(page.locator('text=Your Collection')).toBeVisible();
	});

	test('logo links back to dashboard from any page', async ({ page }) => {
		await signIn(page);
		await page.goto('/collection');
		await page.click('a[href="/"]');
		await expect(page).toHaveURL('/dashboard');
	});

	test('signed-in visitors on / are redirected to /dashboard', async ({ page }) => {
		await signIn(page);
		await page.goto('/');
		await expect(page).toHaveURL('/dashboard');
	});

	test('sign in link goes to login page', async ({ page }) => {
		await page.goto('/');
		await page.click('a[href="/login"]');
		await expect(page).toHaveURL('/login');
	});
});

test.describe('Mobile navigation', () => {
	test.use({ viewport: { width: 375, height: 812 }, isMobile: true });

	test('shows hamburger menu on mobile', async ({ page }) => {
		await page.goto('/');
		// Desktop nav links hidden on mobile
		const desktopNav = page.locator('.hidden.sm\\:flex');
		await expect(desktopNav).toBeHidden();
		// Hamburger button visible
		const hamburger = page.locator('#menu-toggle');
		await expect(hamburger).toBeVisible();
	});

	test('hamburger opens mobile menu', async ({ page }) => {
		await page.goto('/');
		await page.click('#menu-toggle');
		const menu = page.locator('#mobile-menu');
		await expect(menu).toBeVisible();
		await expect(menu.locator('text=Collection')).toBeVisible();
		await expect(menu.locator('text=Wishlist')).toBeVisible();
		await expect(menu.locator('text=Sign in')).toBeVisible();
	});

	test('mobile menu links navigate correctly', async ({ page }) => {
		await page.goto('/');
		await page.click('#menu-toggle');
		await page.locator('#mobile-menu a[href="/collection"]').click();
		await expect(page).toHaveURL('/collection');
	});
});
