<script lang="ts">
	type ActionType = 'collection' | 'wishlist' | 'share';

	type Props = {
		isOpen: boolean;
		actionType: ActionType;
		onClose: () => void;
	};

	let { isOpen, actionType, onClose }: Props = $props();

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

	async function handleCheckout() {
		try {
			const { apiFetch } = await import('../lib/api');
			const res = await apiFetch('/api/billing/checkout', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ priceId: 'price_premium_monthly' }),
			});
			const data = await res.json();
			if (data.checkoutUrl) {
				window.location.href = data.checkoutUrl;
			}
		} catch {
			alert('Failed to initiate checkout. Please try again.');
		}
	}
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
				<button
					type="button"
					onclick={handleCheckout}
					class="w-full bg-espresso hover:bg-espresso/90 text-gold font-semibold py-3 px-4 rounded transition-colors text-xs uppercase tracking-wider"
				>
					Upgrade to Premium — $5/mo
				</button>
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
