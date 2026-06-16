<script lang="ts">
	import { apiFetch } from '../lib/api';
	import { supabase } from '../lib/supabase';

	type Props = {
		onclose?: () => void;
	};

	let { onclose }: Props = $props();

	type SearchResult = { id: string; username: string; displayName: string };
	type OutgoingShare = {
		viewerId: string;
		viewer: { username: string; displayName: string };
		message: string | null;
		createdAt: string;
	};

	let linkMessage = $state('');
	let searchQuery = $state('');
	let searchResults: SearchResult[] = $state([]);
	let searching = $state(false);
	let shareStatus = $state('');
	let shareError = $state('');
	let revokingId = $state<string | null>(null);
	let outgoingShares: OutgoingShare[] = $state([]);
	let sharesError = $state('');
	let sharesLoading = $state(true);

	let searchTimer: ReturnType<typeof setTimeout> | null = null;

	async function copyPublicLink() {
		linkMessage = '';
		try {
			const { data: { session } } = await supabase.auth.getSession();
			const userID = session?.user?.id;
			if (!userID) throw new Error('Sign in to share your wishlist.');

			const url = `${window.location.origin}/wishlist?share=${encodeURIComponent(userID)}`;
			if (navigator.share) {
				await navigator.share({ title: 'AudioFile Wishlist', text: 'Records I am hunting for on AudioFile', url });
				return;
			}
			await navigator.clipboard.writeText(url);
			linkMessage = 'Wishlist link copied.';
		} catch (e) {
			linkMessage = e instanceof Error ? e.message : 'Failed to share wishlist';
		}
	}

	function onSearchInput() {
		if (searchTimer) clearTimeout(searchTimer);
		shareStatus = '';
		shareError = '';
		const q = searchQuery.trim();
		if (q.length < 2) {
			searchResults = [];
			return;
		}
		searchTimer = setTimeout(() => doSearch(q), 250);
	}

	async function doSearch(q: string) {
		searching = true;
		try {
			const res = await apiFetch(`/api/profiles/search?q=${encodeURIComponent(q)}`);
			if (res.status === 400) {
				searchResults = [];
				return;
			}
			if (!res.ok) throw new Error(await res.text());
			searchResults = await res.json();
		} catch (e) {
			shareError = e instanceof Error ? e.message : 'Search failed';
		} finally {
			searching = false;
		}
	}

	async function shareToUser(result: SearchResult) {
		shareStatus = '';
		shareError = '';
		try {
			const res = await apiFetch('/api/wishlist/shares', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ username: result.username }),
			});
			if (res.status === 201) {
				shareStatus = `Shared with ${result.username}`;
				searchQuery = '';
				searchResults = [];
				await fetchOutgoingShares();
				return;
			}
			if (res.status === 404) {
				shareError = 'User not found.';
				return;
			}
			if (res.status === 400) {
				shareError = "You can't share with yourself.";
				return;
			}
			if (res.status === 409) {
				shareError = 'Already shared with this user.';
				return;
			}
			throw new Error(await res.text());
		} catch (e) {
			shareError = e instanceof Error ? e.message : 'Failed to share';
		}
	}

	async function fetchOutgoingShares() {
		sharesLoading = true;
		sharesError = '';
		try {
			const res = await apiFetch('/api/wishlist/shares');
			if (!res.ok) throw new Error(await res.text());
			outgoingShares = await res.json();
		} catch (e) {
			sharesError = e instanceof Error ? e.message : 'Failed to load shares';
		} finally {
			sharesLoading = false;
		}
	}

	async function revokeShare(share: OutgoingShare) {
		revokingId = share.viewerId;
		sharesError = '';
		try {
			const res = await apiFetch(`/api/wishlist/shares/${encodeURIComponent(share.viewerId)}`, { method: 'DELETE' });
			if (!res.ok) throw new Error(await res.text());
			await fetchOutgoingShares();
		} catch (e) {
			sharesError = e instanceof Error ? e.message : 'Failed to revoke share';
		} finally {
			revokingId = null;
		}
	}

	$effect(() => {
		fetchOutgoingShares();
	});
</script>

<div class="bg-white border border-gold/50 rounded-lg p-4 space-y-5">
	<div class="flex items-center justify-between">
		<h2 class="font-serif text-xl text-espresso">Share wishlist</h2>
		<button type="button" class="text-xs text-gold-dark" on:click={() => onclose?.()}>Close</button>
	</div>

	<!-- Copy public link -->
	<section class="space-y-2">
		<h3 class="text-xs text-gold-dark uppercase tracking-wide">Public link</h3>
		<p class="text-[11px] text-gold-muted">Anyone with this link can view your wishlist.</p>
		<button type="button" class="border border-espresso/30 text-espresso text-xs tracking-[0.1em] uppercase px-4 py-2 rounded" on:click={copyPublicLink}>Copy public link</button>
		{#if linkMessage}<p class="text-xs text-gold-dark">{linkMessage}</p>{/if}
	</section>

	<hr class="border-gold/20" />

	<!-- Share to a user -->
	<section class="space-y-2">
		<h3 class="text-xs text-gold-dark uppercase tracking-wide">Share to a user</h3>
		<label class="block text-xs text-gold-dark uppercase tracking-wide">Search by username
			<input class="mt-1 w-full border border-gold/40 rounded px-3 py-2 text-espresso normal-case" placeholder="username" bind:value={searchQuery} on:input={onSearchInput} />
		</label>
		{#if searching}<p class="text-xs text-gold-muted">Searching...</p>{/if}
		{#if searchResults.length > 0}
			<div class="space-y-1">
				{#each searchResults as result (result.id)}
					<button type="button" class="w-full text-left border border-gold/30 rounded p-2 hover:border-gold transition-colors" on:click={() => shareToUser(result)}>
						<span class="block text-sm text-espresso">@{result.username}</span>
						<span class="block text-[11px] text-gold-dark">{result.displayName}</span>
					</button>
				{/each}
			</div>
		{/if}
		{#if shareStatus}<p class="text-xs text-gold-dark">{shareStatus}</p>{/if}
		{#if shareError}<p class="text-xs text-red-700">{shareError}</p>{/if}
	</section>

	<hr class="border-gold/20" />

	<!-- Outgoing shares -->
	<section class="space-y-2">
		<h3 class="text-xs text-gold-dark uppercase tracking-wide">Sharing with</h3>
		{#if sharesLoading}
			<p class="text-xs text-gold-muted">Loading...</p>
		{:else if sharesError}
			<p class="text-xs text-red-700">{sharesError}</p>
		{:else if outgoingShares.length === 0}
			<p class="text-xs text-gold-muted italic">Not shared with anyone yet.</p>
		{:else}
			<div class="space-y-1">
				{#each outgoingShares as share (share.viewerId)}
					<div class="flex items-center justify-between border border-gold/30 rounded p-2">
						<div>
							<span class="block text-sm text-espresso">@{share.viewer.username}</span>
							<span class="block text-[11px] text-gold-dark">{share.viewer.displayName}</span>
						</div>
						<button type="button" disabled={revokingId === share.viewerId} class="text-[10px] text-red-700 underline disabled:opacity-60" on:click={() => revokeShare(share)}>
							{revokingId === share.viewerId ? 'Revoking...' : 'Revoke'}
						</button>
					</div>
				{/each}
			</div>
		{/if}
	</section>
</div>
