<script lang="ts">
	import WishlistRow from './WishlistRow.svelte';
	import { apiUrl } from '../lib/api';

	type ApiItem = {
		id: string;
		title: string;
		artist: string;
		priority: number;
		targetPrice: number | null;
		notes: string;
		label: string;
	};

	let items: ApiItem[] = $state([]);
	let loading = $state(true);

	async function fetchWishlist() {
		loading = true;
		try {
			const res = await fetch(apiUrl('/api/wishlist'));
			items = await res.json();
		} catch (e) {
			console.error('Failed to fetch wishlist', e);
		} finally {
			loading = false;
		}
	}

	$effect(() => {
		fetchWishlist();
	});
</script>

<div class="space-y-8">
	<div class="flex items-end justify-between border-b border-gold-muted/30 pb-6">
		<div>
			<h1 class="font-serif text-4xl text-espresso font-normal mb-1">Wishlist</h1>
			<p class="text-gold-dark text-xs tracking-[0.12em] uppercase">Records you're hunting for</p>
		</div>
		<button class="bg-espresso text-gold text-xs tracking-[0.1em] uppercase px-4 py-2 rounded">+ Add to Wishlist</button>
	</div>

	{#if loading}
		<div class="text-center py-12 text-gold-muted text-xs tracking-widest uppercase">Loading...</div>
	{:else if items.length === 0}
		<div class="bg-espresso/5 border border-gold-muted/20 rounded-lg p-8 text-center">
			<div class="font-serif text-espresso text-lg mb-1">Still hunting?</div>
			<p class="text-gold-dark text-xs tracking-wide mb-4">Add records you're looking for and track your target price.</p>
			<button class="bg-espresso text-gold text-xs tracking-[0.1em] uppercase px-5 py-2.5 rounded">+ Add a Record to Hunt</button>
		</div>
	{:else}
		<div class="space-y-2">
			{#each items as item (item.id)}
				<WishlistRow
					title={item.title}
					artist={item.artist}
					priority={item.priority}
					targetPrice={item.targetPrice}
					notes={item.notes}
					label={item.label}
				/>
			{/each}
		</div>
	{/if}
</div>
