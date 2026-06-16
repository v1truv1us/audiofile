import { test, expect } from '@playwright/test';

test.describe('Navigation', () => {
	test('nav links work from dashboard', async ({ page }) => {
		await page.goto('/');

		await page.click('a[href="/collection"]');
		await expect(page).toHaveURL('/collection');
		await expect(page.locator('text=Collection')).toBeVisible();

		await page.click('a[href="/wishlist"]');
		await expect(page).toHaveURL('/wishlist');
		await expect(page.locator('text=Wishlist')).toBeVisible();

		await page.click('a[href="/"]');
		await expect(page).toHaveURL('/');
		await expect(page.locator('text=Your Collection')).toBeVisible();
	});

	test('logo links back to dashboard from any page', async ({ page }) => {
		await page.goto('/collection');
		await page.click('a[href="/"]');
		await expect(page).toHaveURL('/');
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
