<script lang="ts">
	import { apiFetch } from '../lib/api';
	import { supabase } from '../lib/supabase';

	let status = $state<any>(null);
	let config = $state<any>(null);
	let loading = $state(true);
	let processing = $state(false);
	let paddleReady = $state(false);

	async function loadBillingStatus() {
		try {
			const res = await apiFetch('/api/billing/status');
			status = await res.json();
		} catch (err) {
			console.error('Failed to load billing details', err);
		} finally {
			loading = false;
		}
	}

	async function loadBillingConfig() {
		try {
			const res = await apiFetch('/api/billing/config');
			config = await res.json();
			// Load + initialize Paddle.js once we have the client token
			if (config?.clientToken) {
				await initPaddle(config.clientToken, config.environment);
			}
		} catch (err) {
			console.error('Failed to load billing config', err);
		}
	}

	declare global {
		interface Window { Paddle: any }
	}

	function loadPaddleScript(): Promise<void> {
		return new Promise((resolve, reject) => {
			if (window.Paddle) { resolve(); return; }
			const script = document.createElement('script');
			script.src = 'https://cdn.paddle.com/paddle/v2/paddle.js';
			script.async = true;
			script.onload = () => resolve();
			script.onerror = () => reject(new Error('Failed to load Paddle.js'));
			document.head.appendChild(script);
		});
	}

	async function initPaddle(token: string, environment: string) {
		try {
			await loadPaddleScript();
			// Paddle.js v2: set environment BEFORE Initialize.
			// Sandbox accounts MUST call Paddle.Environment.set('sandbox')
			// so Paddle.js routes to sandbox-checkout-service.paddle.com
			// instead of the production checkout-service.paddle.com.
			if (environment !== 'production' && window.Paddle.Environment) {
				window.Paddle.Environment.set('sandbox');
			}
			window.Paddle.Initialize({
				token,
				eventCallback: (event: any) => {
					console.log('[PADDLE DEBUG] Global event:', event);
				}
			});
			paddleReady = true;
		} catch (err) {
			console.error('Paddle.js init failed', err);
		}
	}

	async function handleCheckout() {
		if (!config?.premiumMonthlyPriceId || config.premiumMonthlyPriceId.includes('XXXX')) {
			alert('Premium subscription not yet configured.');
			return;
		}
		if (!paddleReady || !window.Paddle) {
			alert('Payment system is still loading. Please try again in a moment.');
			return;
		}

		// Get the authenticated user's ID so the webhook can attribute the subscription
		const { data: { session } } = await supabase.auth.getSession();
		const userId = session?.user?.id;
		if (!userId) {
			alert('Please sign in again to upgrade.');
			return;
		}

		processing = true;
		try {
			window.Paddle.Checkout.open({
				items: [{ priceId: config.premiumMonthlyPriceId, quantity: 1 }],
				customData: { user_id: userId },
				settings: {
					successUrl: window.location.origin + '/account?checkout=success',
					theme: 'light'
				},
				eventCallback: (event: any) => {
					if (event?.name === 'checkout.completed') {
						processing = false;
						loadBillingStatus();
					}
				}
			});
		} catch (err) {
			console.error('Paddle checkout failed', err);
			alert('Failed to open checkout. Please try again.');
		}
		processing = false;
	}

	async function handlePortal() {
		processing = true;
		try {
			const res = await apiFetch('/api/billing/portal', { method: 'POST' });
			const data = await res.json();
			if (data.portalUrl) {
				window.location.href = data.portalUrl;
			}
		} catch (err) {
			alert('Failed to open billing portal.');
		} finally {
			processing = false;
		}
	}

	$effect(() => {
		loadBillingStatus();
		loadBillingConfig();
	});
</script>

<div class="border border-gold-muted/30 bg-white p-6 rounded-lg max-w-lg">
	{#if config?.environment === 'sandbox'}
		<div class="mb-3 px-3 py-2 bg-amber-50 border border-amber-200 rounded text-xs text-amber-800">
			⚠️ Sandbox Mode - Test transactions only
		</div>
	{/if}

	<h3 class="font-serif text-2xl text-espresso mb-4">Membership Plan</h3>

	{#if loading}
		<div class="text-gold-dark text-xs animate-pulse">Loading membership details...</div>
	{:else if status}
		<div class="space-y-4">
			<div class="flex justify-between items-center pb-3 border-b border-gold-muted/10">
				<div>
					<span class="text-xs text-gold-dark uppercase tracking-wider block">Current Tier</span>
					<span class="text-lg font-bold text-espresso uppercase">
						{status.tier}
						{#if status.isVip} <span class="text-gold-muted font-serif text-sm font-normal lowercase">(vip exemption)</span>{/if}
					</span>
				</div>
				<span class="px-2.5 py-1 text-xs rounded-full uppercase tracking-wider {status.status === 'active' || status.isVip ? 'bg-emerald-100 text-emerald-800' : 'bg-gold-muted/20 text-gold-dark'}">
					{status.isVip ? 'Lifetime Exemption' : status.status}
				</span>
			</div>

			<!-- Limits Visual Meter -->
			<div class="space-y-3 py-2">
				<h4 class="text-xs uppercase text-gold-dark tracking-wider font-semibold">Usage Limits</h4>

				<!-- Collection Limit -->
				<div>
					<div class="flex justify-between text-xs mb-1">
						<span class="text-espresso">Collection Space</span>
						<span class="font-semibold">{status.limits.collection.limit < 0 ? `${status.limits.collection.used} / ∞ releases` : `${status.limits.collection.used} / ${status.limits.collection.limit} releases`}</span>
					</div>
					<div class="w-full bg-gold-muted/20 h-2.5 rounded-full overflow-hidden">
						<div class="bg-gold h-full transition-all duration-300" style="width: {status.limits.collection.limit < 0 ? '100' : Math.min((status.limits.collection.used / status.limits.collection.limit) * 100, 100)}%"></div>
					</div>
				</div>

				<!-- Wishlist Limit -->
				<div>
					<div class="flex justify-between text-xs mb-1">
						<span class="text-espresso">Wishlist Space</span>
						<span class="font-semibold">{status.limits.wishlist.limit < 0 ? `${status.limits.wishlist.used} / ∞ items` : `${status.limits.wishlist.used} / ${status.limits.wishlist.limit} items`}</span>
					</div>
					<div class="w-full bg-gold-muted/20 h-2.5 rounded-full overflow-hidden">
						<div class="bg-gold h-full transition-all duration-300" style="width: {status.limits.wishlist.limit < 0 ? '100' : Math.min((status.limits.wishlist.used / status.limits.wishlist.limit) * 100, 100)}%"></div>
					</div>
				</div>

				<!-- Wishlist Share Limit -->
				<div>
					<div class="flex justify-between text-xs mb-1">
						<span class="text-espresso">Wishlist Shares</span>
						<span class="font-semibold">{status.limits.shares.limit < 0 ? `${status.limits.shares.used} / ∞ active links` : `${status.limits.shares.used} / ${status.limits.shares.limit} active links`}</span>
					</div>
					<div class="w-full bg-gold-muted/20 h-2.5 rounded-full overflow-hidden">
						<div class="bg-espresso h-full transition-all duration-300" style="width: {status.limits.shares.limit < 0 ? '100' : Math.min((status.limits.shares.used / status.limits.shares.limit) * 100, 100)}%"></div>
					</div>
				</div>
			</div>

			<div class="pt-4 flex gap-4">
				{#if status.tier === 'free' && !status.isVip}
					<button
						disabled={processing}
						onclick={handleCheckout}
						class="w-full bg-espresso hover:bg-espresso/90 text-gold font-semibold py-3 px-4 rounded transition-colors text-xs uppercase tracking-wider disabled:opacity-50">
						{processing ? 'Loading...' : 'Upgrade to Premium — $5/mo'}
					</button>
				{:else if !status.isVip}
					<button
						disabled={processing}
						onclick={handlePortal}
						class="w-full border border-espresso text-espresso hover:bg-gold-muted/10 font-semibold py-3 px-4 rounded transition-colors text-xs uppercase tracking-wider disabled:opacity-50">
						{processing ? 'Loading...' : 'Manage Subscription'}
					</button>
				{/if}
			</div>
		</div>
	{/if}
</div>
