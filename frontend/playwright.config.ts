import { defineConfig } from '@playwright/test';

export default defineConfig({
	testDir: './e2e',
	timeout: 30000,
	retries: 1,
	use: {
		baseURL: 'http://localhost:4321',
		screenshot: 'only-on-failure',
		trace: 'retain-on-failure',
	},
	webServer: [
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
