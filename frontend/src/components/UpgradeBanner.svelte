<script lang="ts">
	type LimitInfo = {
		used: number;
		limit: number;
		isExceeded: boolean;
	};

	type Props = {
		limits?: {
			collection: LimitInfo;
			wishlist: LimitInfo;
			shares: LimitInfo;
		};
		tier?: string;
	};

	let { limits, tier = 'free' }: Props = $props();

	let exceededLabels = $derived.by(() => {
		if (!limits) return [];
		const labels: string[] = [];
		if (limits.collection.isExceeded) labels.push('collection');
		if (limits.wishlist.isExceeded) labels.push('wishlist');
		if (limits.shares.isExceeded) labels.push('shares');
		return labels;
	});

	let isVisible = $derived(exceededLabels.length > 0 && tier !== 'premium');
</script>

{#if isVisible}
	<div class="bg-gold/20 border border-gold-muted/40 rounded-lg p-4 mb-6">
		<div class="flex items-start gap-3">
			<svg width="20" height="20" viewBox="0 0 20 20" fill="none" class="shrink-0 mt-0.5" aria-hidden="true">
				<circle cx="10" cy="10" r="9" stroke="#854F0B" stroke-width="1.5"/>
				<path d="M10 6v5" stroke="#854F0B" stroke-width="1.5" stroke-linecap="round"/>
				<circle cx="10" cy="14" r="0.75" fill="#854F0B"/>
			</svg>
			<div class="flex-1">
				<p class="text-sm font-semibold text-espresso mb-1">You've reached your free tier limit{exceededLabels.length > 1 ? 's' : ''}</p>
				<p class="text-xs text-gold-dark">
					Your {exceededLabels.join(', ')} limit{exceededLabels.length > 1 ? 's are' : ' is'} full.
					Upgrade to Premium for unlimited access.
				</p>
				<a href="/account#billing" class="inline-block mt-2 text-xs bg-espresso text-gold px-3 py-1.5 rounded uppercase tracking-wider hover:bg-espresso/90 transition-colors">
					View Plans
				</a>
			</div>
		</div>
	</div>
{/if}
