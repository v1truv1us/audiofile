<script lang="ts">
	import { fetchBillingConfig, initPaddle, openPaddleCheckout } from '../lib/paddle';
	import { supabase } from '../lib/supabase';

	type ActionType = 'collection' | 'wishlist' | 'share';

	type Props = {
		isOpen: boolean;
		actionType: ActionType;
		onClose: () => void;
		onComplete?: () => void;
	};

	let { isOpen, actionType, onClose, onComplete }: Props = $props();

	let config = $state<any>(null);
	let loading = $state(false);
	let processing = $state(false);

	const messages: Record<ActionType, { title: string; body: string }> = {
		collection: {
			title: 'Collection limit reached',
			body: 'You\'ve added 50 records to your collection — the free tier maximum. Upgrade to Premium for unlimited space.',
		},
		wishlist: {
			title: 'Wishlist limit reached',
			body: 'You\'ve added 25 items to your wishlist — the free tier maximum. Upgrade to Premium for unlimited wishlist items.',
		},
		share: {
			title: 'Sharing limit reached',
			body: 'You\'ve used your 1 free wishlist share link. Upgrade to Premium for unlimited sharing.',
		},
	};

	function handleBackdropClick(e: MouseEvent) {
		if (e.target === e.currentTarget) {
			onClose();
		}
	}

	async function loadConfig() {
		loading = true;
		processing = false;
		try {
			config = await fetchBillingConfig();
			if (config?.clientToken) {
				await initPaddle(config.clientToken, config.environment);
			}
		} catch (err) {
			console.error('Failed to load billing config', err);
		} finally {
			loading = false;
		}
	}

	async function handleCheckout() {
		if (!config?.premiumMonthlyPriceId || config.premiumMonthlyPriceId.includes('XXXX')) {
			alert('Premium subscription not yet configured.');
			return;
		}
		const { data: { session } } = await supabase.auth.getSession();
		const userId = session?.user?.id;
		if (!userId) {
			alert('Please sign in again to upgrade.');
			return;
		}
		processing = true;
		try {
			openPaddleCheckout({
				priceId: config.premiumMonthlyPriceId,
				userId,
				successUrl: window.location.origin + '/account?checkout=success#billing',
				onComplete: () => {
					processing = false;
					onComplete?.();
				}
			});
		} catch (err) {
			console.error('Paddle checkout failed', err);
			alert('Failed to open checkout. Please try again.');
			processing = false;
		}
	}

	$effect(() => {
		if (isOpen) {
			loadConfig();
		}
	});
</script>

{#if isOpen}
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div class="fixed inset-0 z-50 flex items-center justify-center bg-espresso/60 backdrop-blur-sm" onclick={handleBackdropClick}>
		<div class="bg-cream border border-gold-muted/40 rounded-lg p-6 max-w-md w-full mx-4 shadow-xl">
			<div class="flex items-start justify-between mb-4">
				<div class="flex items-center gap-2">
					<svg width="24" height="24" viewBox="0 0 24 24" fill="none" aria-hidden="true">
						<circle cx="12" cy="12" r="11" stroke="#BA7517" stroke-width="1"/>
						<circle cx="12" cy="12" r="3.5" stroke="#BA7517" stroke-width="0.6" opacity="0.5"/>
						<circle cx="12" cy="12" r="1.2" fill="#BA7517"/>
					</svg>
					<h2 class="font-serif text-xl text-espresso">{messages[actionType].title}</h2>
				</div>
				<button type="button" class="text-gold-dark hover:text-espresso transition-colors" onclick={onClose} aria-label="Close">
					<svg width="18" height="18" viewBox="0 0 18 18" fill="currentColor"><path d="M4.5 4.5l9 9m0-9l-9 9" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/></svg>
				</button>
			</div>

			<p class="text-sm text-gold-dark mb-6 leading-relaxed">{messages[actionType].body}</p>

			<div class="flex flex-col gap-3">
				{#if loading}
					<div class="text-center py-2 text-gold-muted text-xs">Loading...</div>
				{:else}
					<button
						type="button"
						disabled={processing}
						onclick={handleCheckout}
						class="w-full bg-espresso hover:bg-espresso/90 text-gold font-semibold py-3 px-4 rounded transition-colors text-xs uppercase tracking-wider disabled:opacity-50"
					>
						{processing ? 'Processing...' : 'Upgrade to Premium — $5/mo'}
					</button>
				{/if}
				<button
					type="button"
					onclick={onClose}
					class="w-full border border-espresso/30 text-espresso hover:bg-gold-muted/10 py-2.5 px-4 rounded transition-colors text-xs uppercase tracking-wider"
				>
					Maybe Later
				</button>
			</div>

			<p class="text-[10px] text-gold-muted text-center mt-4">
				Premium includes unlimited collection, wishlist, and sharing.
			</p>
		</div>
	</div>
{/if}
