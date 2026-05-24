import { defineConfig } from '@playwright/test';

const baseUrl = process.env.BASE_URL || 'http://localhost:4321';
const apiBase = process.env.BASE_URL || 'http://localhost:8080';

export default defineConfig({
	testDir: './e2e',
	timeout: 30000,
	retries: 1,
	use: {
		baseURL: baseUrl,
		screenshot: 'only-on-failure',
		trace: 'retain-on-failure',
		extraHTTPHeaders: process.env.BASE_URL ? {} : undefined,
	},
	webServer: process.env.BASE_URL ? undefined : [
		{
			command: 'cd ../backend && DATABASE_URL=$DATABASE_URL go run ./cmd/server',
			port: 8080,
			reuseExistingServer: true,
			timeout: 30000,
		},
		{
			command: 'npx astro dev --port 4321 --host 0.0.0.0',
			port: 4321,
			reuseExistingServer: true,
			timeout: 30000,
		},
	],
	projects: [
		{
			name: 'Desktop Chrome',
			use: { viewport: { width: 1280, height: 720 } },
		},
		{
			name: 'Mobile iPhone',
			use: { viewport: { width: 375, height: 812 }, isMobile: true },
		},
	],
});
