import { test, expect } from '@playwright/test';

test.describe('API health check', () => {
	test('returns ok status', async ({ request }) => {
		const res = await request.get('http://localhost:8080/api/health');
		expect(res.ok()).toBeTruthy();
		const body = await res.json();
		expect(body.status).toBe('ok');
		expect(body.version).toBeTruthy();
	});
});

test.describe('Collection API', () => {
	test('returns collection items', async ({ request }) => {
		const res = await request.get('http://localhost:8080/api/collection');
		expect(res.ok()).toBeTruthy();
		const items = await res.json();
		expect(Array.isArray(items)).toBeTruthy();
		if (items.length > 0) {
			const item = items[0];
			expect(item.id).toBeTruthy();
			expect(item.release).toBeTruthy();
			expect(item.release.title).toBeTruthy();
			expect(item.release.artist).toBeTruthy();
			expect(item.mediaCondition).toBeTruthy();
		}
	});

	test('returns collection stats', async ({ request }) => {
		const res = await request.get('http://localhost:8080/api/collection/stats');
		expect(res.ok()).toBeTruthy();
		const stats = await res.json();
		expect(typeof stats.collectionCount).toBe('number');
		expect(typeof stats.wishlistCount).toBe('number');
		expect(typeof stats.totalValue).toBe('number');
		expect(stats.collectionCount).toBeGreaterThanOrEqual(0);
	});

	test('supports sort parameter', async ({ request }) => {
		const [recent, artist, year] = await Promise.all([
			request.get('http://localhost:8080/api/collection?sort='),
			request.get('http://localhost:8080/api/collection?sort=artist'),
			request.get('http://localhost:8080/api/collection?sort=year'),
		]);
		expect(recent.ok()).toBeTruthy();
		expect(artist.ok()).toBeTruthy();
		expect(year.ok()).toBeTruthy();
	});
});

test.describe('Wishlist API', () => {
	test('returns wishlist items', async ({ request }) => {
		const res = await request.get('http://localhost:8080/api/wishlist');
		expect(res.ok()).toBeTruthy();
		const items = await res.json();
		expect(Array.isArray(items)).toBeTruthy();
		if (items.length > 0) {
			const item = items[0];
			expect(item.id).toBeTruthy();
			expect(item.title).toBeTruthy();
			expect(item.artist).toBeTruthy();
			expect(typeof item.priority).toBe('number');
		}
	});
});
