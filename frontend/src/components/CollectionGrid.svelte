<script lang="ts">
	import RecordCard from './RecordCard.svelte';
	import { apiFetch } from '../lib/api';

	type ApiItem = {
		id: string;
		release: {
			id: string;
			title: string;
			artist: string;
			year: number;
			label: string;
			coverUrl?: string;
		};
		mediaCondition: string;
		notes: string;
		purchasePrice: number | null;
		isForSale: boolean;
	};

	let items: ApiItem[] = $state([]);
	let sort = $state('recent');
	let loading = $state(true);
	let showAddForm = $state(false);
	let saving = $state(false);
	let error = $state('');
	let shareMessage = $state('');
	type ReleaseSearchResult = {
		title: string;
		artist: string;
		year?: number;
		label?: string;
		coverUrl?: string;
	};

	let form = $state({
		title: '',
		artist: '',
		year: '',
		label: '',
		mediaCondition: 'VG',
		sleeveCondition: 'VG',
		purchasePrice: '',
		notes: '',
		coverUrl: '',
	});
	let releaseQuery = $state('');
	let releaseResults: ReleaseSearchResult[] = $state([]);
	let searching = $state(false);

	async function searchReleases() {
		const q = releaseQuery.trim() || [form.title, form.artist].filter(Boolean).join(' ');
		if (!q) {
			error = 'Enter a title or artist to search.';
			return;
		}
		searching = true;
		error = '';
		try {
			const res = await apiFetch(`/api/releases/search?q=${encodeURIComponent(q)}`);
			if (!res.ok) throw new Error(await res.text());
			releaseResults = await res.json();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to search releases';
		} finally {
			searching = false;
		}
	}

	function useRelease(result: ReleaseSearchResult) {
		form.title = result.title;
		form.artist = result.artist;
		form.year = result.year ? String(result.year) : '';
		form.label = result.label ?? '';
		form.coverUrl = result.coverUrl ?? '';
		releaseResults = [];
		releaseQuery = `${result.artist} ${result.title}`;
	}

	async function addRecord() {
		saving = true;
		error = '';
		try {
			if (!form.title.trim() || !form.artist.trim()) {
				error = 'Title and artist are required.';
				return;
			}
			const res = await apiFetch('/api/collection', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({
					title: form.title.trim(),
					artist: form.artist.trim(),
					year: form.year ? Number(form.year) : null,
					label: form.label.trim(),
					mediaCondition: form.mediaCondition,
					sleeveCondition: form.sleeveCondition,
					purchasePrice: form.purchasePrice ? Number(form.purchasePrice) : null,
					notes: form.notes.trim(),
					coverUrl: form.coverUrl,
				}),
			});
			if (!res.ok) throw new Error(await res.text());
			form = { title: '', artist: '', year: '', label: '', mediaCondition: 'VG', sleeveCondition: 'VG', purchasePrice: '', notes: '', coverUrl: '' };
			showAddForm = false;
			await fetchCollection();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to add record';
		} finally {
			saving = false;
		}
	}

	async function shareCollection() {
		const url = `${window.location.origin}/collection`;
		try {
			if (navigator.share) {
				await navigator.share({ title: 'AudioFile Collection', text: 'My vinyl collection on AudioFile', url });
				return;
			}
			await navigator.clipboard.writeText(url);
			shareMessage = 'Collection link copied.';
		} catch (e) {
			shareMessage = e instanceof Error ? e.message : 'Failed to share collection';
		}
	}

	async function fetchCollection() {
		loading = true;
		try {
			const sortParam = sort === 'artist' ? 'artist' : sort === 'year' ? 'year' : sort === 'condition' ? 'condition' : '';
			const res = await apiFetch(`/api/collection?sort=${sortParam}`);
			items = await res.json();
		} catch (e) {
			console.error('Failed to fetch collection', e);
		} finally {
			loading = false;
		}
	}

	$effect(() => {
		if (new URLSearchParams(window.location.search).get('add') === '1') {
			showAddForm = true;
		}
	});

	$effect(() => {
		sort;
		fetchCollection();
	});
</script>

<div class="space-y-8">
	<div class="flex flex-col gap-4 border-b border-gold-muted/30 pb-6 sm:flex-row sm:items-end sm:justify-between">
		<div>
			<h1 class="font-serif text-4xl text-espresso font-normal mb-1">Collection</h1>
			<p class="text-gold-dark text-xs tracking-[0.12em] uppercase">{items.length} records · sorted by {sort === 'recent' ? 'recently added' : sort}</p>
		</div>
		<div class="flex flex-col gap-3 sm:flex-row sm:items-center">
			<select
				class="text-xs border border-gold/50 bg-cream text-espresso rounded px-3 py-2 tracking-wide"
				bind:value={sort}
			>
				<option value="recent">Recently added</option>
				<option value="artist">Artist A–Z</option>
				<option value="year">Year</option>
				<option value="condition">Condition</option>
			</select>
			<button type="button" class="border border-espresso/30 text-espresso text-xs tracking-[0.1em] uppercase px-4 py-2 rounded" on:click={shareCollection}>Share</button>
			<button type="button" class="bg-espresso text-gold text-xs tracking-[0.1em] uppercase px-4 py-2 rounded" on:click={() => showAddForm = !showAddForm}>+ Add Record</button>
		</div>
	</div>
	{#if shareMessage}<p class="text-xs text-gold-dark">{shareMessage}</p>{/if}

	{#if showAddForm}
		<div class="bg-white border border-gold/50 rounded-lg p-4 grid grid-cols-1 gap-3 sm:grid-cols-2">
			<div class="sm:col-span-2">
				<label class="text-xs text-gold-dark uppercase tracking-wide">Search MusicBrainz<input class="mt-1 w-full border border-gold/40 rounded px-3 py-2 text-espresso normal-case" placeholder="Kind of Blue Miles Davis" bind:value={releaseQuery} /></label>
				<button type="button" disabled={searching} class="mt-2 text-xs border border-espresso/30 text-espresso px-3 py-1.5 rounded disabled:opacity-60" on:click={searchReleases}>{searching ? 'Searching...' : 'Search releases'}</button>
				{#if releaseResults.length > 0}
					<div class="mt-3 space-y-2">
						{#each releaseResults as result}
							<button type="button" class="w-full text-left border border-gold/30 rounded p-2 hover:border-gold flex gap-3 items-center" on:click={() => useRelease(result)}>
								{#if result.coverUrl}<img src={result.coverUrl} alt="" class="h-10 w-10 object-cover bg-espresso" />{/if}
								<span><span class="block text-sm text-espresso">{result.title}</span><span class="block text-[11px] text-gold-dark">{result.artist}{result.year ? ` · ${result.year}` : ''}{result.label ? ` · ${result.label}` : ''}</span></span>
							</button>
						{/each}
					</div>
				{/if}
			</div>
			<label class="text-xs text-gold-dark uppercase tracking-wide">Title<input class="mt-1 w-full border border-gold/40 rounded px-3 py-2 text-espresso normal-case" bind:value={form.title} /></label>
			<label class="text-xs text-gold-dark uppercase tracking-wide">Artist<input class="mt-1 w-full border border-gold/40 rounded px-3 py-2 text-espresso normal-case" bind:value={form.artist} /></label>
			<label class="text-xs text-gold-dark uppercase tracking-wide">Year<input class="mt-1 w-full border border-gold/40 rounded px-3 py-2 text-espresso normal-case" type="number" bind:value={form.year} /></label>
			<label class="text-xs text-gold-dark uppercase tracking-wide">Label<input class="mt-1 w-full border border-gold/40 rounded px-3 py-2 text-espresso normal-case" bind:value={form.label} /></label>
			<label class="text-xs text-gold-dark uppercase tracking-wide">Media condition<select class="mt-1 w-full border border-gold/40 rounded px-3 py-2 text-espresso normal-case" bind:value={form.mediaCondition}>{#each ['M','NM','VG+','VG','G+','G','F','P'] as condition}<option value={condition}>{condition}</option>{/each}</select></label>
			<label class="text-xs text-gold-dark uppercase tracking-wide">Sleeve condition<select class="mt-1 w-full border border-gold/40 rounded px-3 py-2 text-espresso normal-case" bind:value={form.sleeveCondition}>{#each ['M','NM','VG+','VG','G+','G','F','P'] as condition}<option value={condition}>{condition}</option>{/each}</select></label>
			<label class="text-xs text-gold-dark uppercase tracking-wide">Price<input class="mt-1 w-full border border-gold/40 rounded px-3 py-2 text-espresso normal-case" type="number" step="0.01" bind:value={form.purchasePrice} /></label>
			<label class="text-xs text-gold-dark uppercase tracking-wide sm:col-span-2">Notes<textarea class="mt-1 w-full border border-gold/40 rounded px-3 py-2 text-espresso normal-case" bind:value={form.notes}></textarea></label>
			{#if error}<p class="text-xs text-red-700 sm:col-span-2">{error}</p>{/if}
			<div class="flex gap-2 sm:col-span-2">
				<button type="button" disabled={saving} class="bg-espresso text-gold text-xs tracking-[0.1em] uppercase px-4 py-2 rounded disabled:opacity-60" on:click={addRecord}>{saving ? 'Saving...' : 'Save Record'}</button>
				<button type="button" class="text-xs text-gold-dark px-4 py-2" on:click={() => showAddForm = false}>Cancel</button>
			</div>
		</div>
	{/if}

	{#if loading}
		<div class="text-center py-12 text-gold-muted text-xs tracking-widest uppercase">Loading...</div>
	{:else if items.length === 0}
		<div class="text-center py-12 text-gold-muted text-xs tracking-widest uppercase">No records yet</div>
	{:else}
		<div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
			{#each items as item (item.id)}
				<RecordCard
					id={item.id}
					title={item.release.title}
					artist={item.release.artist}
					year={item.release.year}
					grade={item.mediaCondition}
					sleeveGrade={item.sleeveCondition}
					pressing={item.notes || 'Unknown'}
					label={item.release.label}
					coverUrl={item.release.coverUrl}
					purchasePrice={item.purchasePrice}
					notes={item.notes}
					onChanged={fetchCollection}
				/>
			{/each}
		</div>
	{/if}
</div>
