import { apiFetch } from './api';

declare global {
	interface Window { Paddle: any }
}

let scriptPromise: Promise<void> | null = null;

export async function fetchBillingConfig(): Promise<{ premiumMonthlyPriceId: string; environment: string; clientToken: string }> {
	const res = await apiFetch('/api/billing/config');
	return res.json();
}

export function loadPaddleScript(): Promise<void> {
	if (window.Paddle) return Promise.resolve();
	if (scriptPromise) return scriptPromise;
	scriptPromise = new Promise((resolve, reject) => {
		const script = document.createElement('script');
		script.src = 'https://cdn.paddle.com/paddle/v2/paddle.js';
		script.async = true;
		script.onload = () => resolve();
		script.onerror = () => reject(new Error('Failed to load Paddle.js'));
		document.head.appendChild(script);
	});
	return scriptPromise;
}

export async function initPaddle(clientToken: string, environment: string): Promise<void> {
	await loadPaddleScript();
	if (environment !== 'production' && window.Paddle.Environment) {
		window.Paddle.Environment.set('sandbox');
	}
	window.Paddle.Initialize({
		token: clientToken,
		eventCallback: (event: any) => {
			console.log('[PADDLE DEBUG] Global event:', event);
		}
	});
}

export function openPaddleCheckout(options: { priceId: string; userId: string; successUrl: string; onComplete?: () => void }): void {
	window.Paddle.Checkout.open({
		items: [{ priceId: options.priceId, quantity: 1 }],
		customData: { user_id: options.userId },
		settings: { successUrl: options.successUrl, theme: 'light' },
		eventCallback: (event: any) => {
			if (event?.name === 'checkout.completed') {
				options.onComplete?.();
			}
		}
	});
}
