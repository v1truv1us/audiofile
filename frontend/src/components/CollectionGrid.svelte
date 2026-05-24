<script lang="ts">
	import RecordCard, { labelThemeFor } from './RecordCard.svelte';
	import { apiUrl } from '../lib/api';

	type ApiItem = {
		id: string;
		release: {
			id: string;
			title: string;
			artist: string;
			year: number;
			label: string;
		};
		mediaCondition: string;
		notes: string;
		purchasePrice: number | null;
		isForSale: boolean;
	};

	let items: ApiItem[] = $state([]);
	let sort = $state('recent');
	let loading = $state(true);

	async function fetchCollection() {
		loading = true;
		try {
			const sortParam = sort === 'artist' ? 'artist' : sort === 'year' ? 'year' : sort === 'condition' ? 'condition' : '';
			const res = await fetch(apiUrl(`/api/collection?sort=${sortParam}`));
			items = await res.json();
		} catch (e) {
			console.error('Failed to fetch collection', e);
		} finally {
			loading = false;
		}
	}

	$effect(() => {
		sort;
		fetchCollection();
	});
</script>

<div class="space-y-8">
	<div class="flex items-end justify-between border-b border-gold-muted/30 pb-6">
		<div>
			<h1 class="font-serif text-4xl text-espresso font-normal mb-1">Collection</h1>
			<p class="text-gold-dark text-xs tracking-[0.12em] uppercase">{items.length} records · sorted by {sort === 'recent' ? 'recently added' : sort}</p>
		</div>
		<div class="flex items-center gap-3">
			<select
				class="text-xs border border-gold/50 bg-cream text-espresso rounded px-3 py-2 tracking-wide"
				bind:value={sort}
			>
				<option value="recent">Recently added</option>
				<option value="artist">Artist A–Z</option>
				<option value="year">Year</option>
				<option value="condition">Condition</option>
			</select>
			<button class="bg-espresso text-gold text-xs tracking-[0.1em] uppercase px-4 py-2 rounded">+ Add Record</button>
		</div>
	</div>

	{#if loading}
		<div class="text-center py-12 text-gold-muted text-xs tracking-widest uppercase">Loading...</div>
	{:else if items.length === 0}
		<div class="text-center py-12 text-gold-muted text-xs tracking-widest uppercase">No records yet</div>
	{:else}
		<div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
			{#each items as item (item.id)}
				<RecordCard
					title={item.release.title}
					artist={item.release.artist}
					year={item.release.year}
					grade={item.mediaCondition}
					pressing={item.notes || 'Unknown'}
					label={item.release.label}
				/>
			{/each}
		</div>
	{/if}
</div>
