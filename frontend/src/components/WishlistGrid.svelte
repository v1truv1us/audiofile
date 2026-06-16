<script lang="ts">
	import WishlistRow from './WishlistRow.svelte';
	import ShareDialog from './ShareDialog.svelte';
	import { apiFetch } from '../lib/api';
	import { type ReleaseSearchResult } from '../lib/barcode';

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
	let showAddForm = $state(false);
	let editingId = $state<string | null>(null);
	let saving = $state(false);
	let error = $state('');
	let showShareDialog = $state(false);
	let isSharedView = $state(false);

	let form = $state({
		title: '',
		artist: '',
		priority: '5',
		targetPrice: '',
		notes: '',
		year: '',
		label: '',
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

	function resetForm() {
		form = { title: '', artist: '', priority: '5', targetPrice: '', notes: '', year: '', label: '', coverUrl: '' };
		releaseResults = [];
		releaseQuery = '';
		editingId = null;
	}

	function editWishlistItem(item: ApiItem) {
		form = {
			title: item.title,
			artist: item.artist,
			priority: String(item.priority),
			targetPrice: item.targetPrice ? String(item.targetPrice) : '',
			notes: item.notes ?? '',
			year: '',
			label: item.label ?? '',
			coverUrl: '',
		};
		releaseQuery = `${item.artist} ${item.title}`;
		releaseResults = [];
		editingId = item.id;
		showAddForm = true;
	}

	async function saveWishlistItem() {
		saving = true;
		error = '';
		try {
			if (!form.title.trim() || !form.artist.trim()) {
				error = 'Title and artist are required.';
				return;
			}
			const url = editingId ? `/api/wishlist/${editingId}` : '/api/wishlist';
			const res = await apiFetch(url, {
				method: editingId ? 'PUT' : 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({
					title: form.title.trim(),
					artist: form.artist.trim(),
					priority: Number(form.priority),
					targetPrice: form.targetPrice ? Number(form.targetPrice) : null,
					notes: form.notes.trim(),
					year: form.year ? Number(form.year) : null,
					label: form.label,
					coverUrl: form.coverUrl,
				}),
			});
			if (!res.ok) throw new Error(await res.text());
			resetForm();
			showAddForm = false;
			await fetchWishlist();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to save wishlist item';
		} finally {
			saving = false;
		}
	}

	async function removeWishlistItem(item: ApiItem) {
		if (!confirm(`Remove ${item.title} from your wishlist?`)) return;
		error = '';
		try {
			const res = await apiFetch(`/api/wishlist/${item.id}`, { method: 'DELETE' });
			if (!res.ok) throw new Error(await res.text());
			await fetchWishlist();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to remove wishlist item';
		}
	}

	async function markPurchased(item: ApiItem) {
		if (!confirm(`Move ${item.title} to your collection?`)) return;
		error = '';
		try {
			const res = await apiFetch(`/api/wishlist/${item.id}/purchase`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ mediaCondition: 'VG', sleeveCondition: 'VG' }),
			});
			if (!res.ok) throw new Error(await res.text());
			await fetchWishlist();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to move wishlist item to collection';
		}
	}

	async function fetchWishlist() {
		loading = true;
		try {
			const shareID = new URLSearchParams(window.location.search).get('share');
			isSharedView = Boolean(shareID);
			const res = shareID
				? await apiFetch(`/api/public/wishlist/${encodeURIComponent(shareID)}`, { public: true })
				: await apiFetch('/api/wishlist');
			if (!res.ok) throw new Error(await res.text());
			items = await res.json();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to fetch wishlist';
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
			<p class="text-gold-dark text-xs tracking-[0.12em] uppercase">{isSharedView ? 'Shared records to hunt for' : "Records you're hunting for"}</p>
		</div>
		{#if !isSharedView}
			<div class="flex gap-2">
				<button type="button" class="border border-espresso/30 text-espresso text-xs tracking-[0.1em] uppercase px-4 py-2 rounded" on:click={() => showShareDialog = !showShareDialog}>Share</button>
				<button type="button" class="bg-espresso text-gold text-xs tracking-[0.1em] uppercase px-4 py-2 rounded" on:click={() => { resetForm(); showAddForm = !showAddForm; }}>+ Add to Wishlist</button>
			</div>
		{/if}
	</div>
	{#if error && !showAddForm && !showShareDialog}<p class="text-xs text-red-700">{error}</p>{/if}

	{#if showShareDialog}
		<ShareDialog onclose={() => showShareDialog = false} />
	{/if}

	{#if showAddForm}
		<div class="bg-white border border-gold/50 rounded-lg p-4 grid grid-cols-1 gap-3 sm:grid-cols-2">
			<h2 class="sm:col-span-2 font-serif text-xl text-espresso">{editingId ? 'Edit wishlist item' : 'Add wishlist item'}</h2>
			<div class="sm:col-span-2">
				<label class="text-xs text-gold-dark uppercase tracking-wide">Search releases<input class="mt-1 w-full border border-gold/40 rounded px-3 py-2 text-espresso normal-case" placeholder="A Love Supreme John Coltrane" bind:value={releaseQuery} /></label>
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
			<label class="text-xs text-gold-dark uppercase tracking-wide">Priority<select class="mt-1 w-full border border-gold/40 rounded px-3 py-2 text-espresso normal-case" bind:value={form.priority}>{#each [1,2,3,4,5,6,7,8,9,10] as priority}<option value={String(priority)}>{priority}</option>{/each}</select></label>
			<label class="text-xs text-gold-dark uppercase tracking-wide">Target price<input class="mt-1 w-full border border-gold/40 rounded px-3 py-2 text-espresso normal-case" type="number" step="0.01" bind:value={form.targetPrice} /></label>
			<label class="text-xs text-gold-dark uppercase tracking-wide">Year<input class="mt-1 w-full border border-gold/40 rounded px-3 py-2 text-espresso normal-case" type="number" bind:value={form.year} /></label>
			<label class="text-xs text-gold-dark uppercase tracking-wide">Label<input class="mt-1 w-full border border-gold/40 rounded px-3 py-2 text-espresso normal-case" bind:value={form.label} /></label>
			<label class="text-xs text-gold-dark uppercase tracking-wide sm:col-span-2">Notes<textarea class="mt-1 w-full border border-gold/40 rounded px-3 py-2 text-espresso normal-case" bind:value={form.notes}></textarea></label>
			{#if error}<p class="text-xs text-red-700 sm:col-span-2">{error}</p>{/if}
			<div class="flex gap-2 sm:col-span-2">
				<button type="button" disabled={saving} class="bg-espresso text-gold text-xs tracking-[0.1em] uppercase px-4 py-2 rounded disabled:opacity-60" on:click={saveWishlistItem}>{saving ? 'Saving...' : 'Save Wishlist Item'}</button>
				<button type="button" class="text-xs text-gold-dark px-4 py-2" on:click={() => { resetForm(); showAddForm = false; }}>Cancel</button>
			</div>
		</div>
	{/if}

	{#if loading}
		<div class="text-center py-12 text-gold-muted text-xs tracking-widest uppercase">Loading...</div>
	{:else if items.length === 0}
		<div class="bg-espresso/5 border border-gold-muted/20 rounded-lg p-8 text-center">
			<div class="font-serif text-espresso text-lg mb-1">Still hunting?</div>
			<p class="text-gold-dark text-xs tracking-wide mb-4">Add records you're looking for and track your target price.</p>
			{#if !isSharedView}<button type="button" class="bg-espresso text-gold text-xs tracking-[0.1em] uppercase px-5 py-2.5 rounded" on:click={() => showAddForm = true}>+ Add a Record to Hunt</button>{/if}
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
					onEdit={isSharedView ? undefined : () => editWishlistItem(item)}
					onRemove={isSharedView ? undefined : () => removeWishlistItem(item)}
					onPurchased={isSharedView ? undefined : () => markPurchased(item)}
				/>
			{/each}
		</div>
	{/if}
</div>
