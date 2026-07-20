<script lang="ts">
	import { apiFetch } from '../lib/api';

	type NotificationActor = {
		id: string;
		username: string;
		displayName: string;
	};

	type Notification = {
		id: string;
		type: string;
		actor: NotificationActor;
		data: Record<string, string>;
		readAt: string | null;
		createdAt: string;
	};

	let notifications: Notification[] = $state([]);
	let loading = $state(true);
	let error = $state('');

	function actorName(n: Notification): string {
		return n.actor.displayName || n.actor.username || 'Someone';
	}

	function message(n: Notification): string {
		if (n.type === 'wishlist_shared') return `${actorName(n)} shared their wishlist with you`;
		if (n.type === 'wishlist_claimed') return `${actorName(n)} added your wishlist`;
		return 'Notification';
	}

	function linkFor(n: Notification): string {
		if (n.type === 'wishlist_shared') return '/shared';
		if (n.type === 'wishlist_claimed') return '/wishlist';
		return '/notifications';
	}

	function relativeTime(iso: string): string {
		const seconds = Math.floor((Date.now() - new Date(iso).getTime()) / 1000);
		if (seconds < 60) return 'just now';
		const minutes = Math.floor(seconds / 60);
		if (minutes < 60) return `${minutes}m ago`;
		const hours = Math.floor(minutes / 60);
		if (hours < 24) return `${hours}h ago`;
		const days = Math.floor(hours / 24);
		if (days < 7) return `${days}d ago`;
		return new Date(iso).toLocaleDateString();
	}

	async function fetchNotifications() {
		loading = true;
		error = '';
		try {
			const res = await apiFetch('/api/notifications');
			if (!res.ok) throw new Error(await res.text());
			notifications = await res.json();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load notifications';
		} finally {
			loading = false;
		}
	}

	async function openNotification(n: Notification) {
		if (!n.readAt) {
			n.readAt = new Date().toISOString();
			apiFetch(`/api/notifications/${n.id}/read`, { method: 'POST' }).catch(() => {});
		}
		window.location.href = linkFor(n);
	}

	async function markAllRead() {
		notifications = notifications.map((n) => ({ ...n, readAt: n.readAt ?? new Date().toISOString() }));
		try {
			await apiFetch('/api/notifications/read-all', { method: 'POST' });
		} catch {
			await fetchNotifications();
		}
	}

	$effect(() => {
		fetchNotifications();
	});
</script>

<div class="space-y-6">
	<div class="flex items-end justify-between border-b border-gold-muted/30 pb-6">
		<div>
			<h1 class="font-serif text-4xl text-espresso font-normal mb-1">Notifications</h1>
			<p class="text-gold-dark text-xs tracking-[0.12em] uppercase">Shares and activity</p>
		</div>
		{#if notifications.some((n) => !n.readAt)}
			<button type="button" class="border border-espresso/30 text-espresso text-xs tracking-[0.1em] uppercase px-4 py-2 rounded" on:click={markAllRead}>Mark all read</button>
		{/if}
	</div>

	{#if error}<p class="text-xs text-red-700">{error}</p>{/if}

	{#if loading}
		<div class="text-center py-12 text-gold-muted text-xs tracking-widest uppercase">Loading...</div>
	{:else if notifications.length === 0}
		<div class="bg-espresso/5 border border-gold-muted/20 rounded-lg p-8 text-center">
			<div class="font-serif text-espresso text-lg mb-1">All quiet</div>
			<p class="text-gold-dark text-xs tracking-wide">When someone shares a wishlist with you, it'll show up here.</p>
		</div>
	{:else}
		<div class="space-y-2">
			{#each notifications as n (n.id)}
				<button
					type="button"
					class="w-full text-left bg-white border rounded-lg p-4 flex items-center gap-3 hover:border-gold transition-colors {n.readAt ? 'border-gold/30' : 'border-gold'}"
					on:click={() => openNotification(n)}
				>
					{#if !n.readAt}
						<span class="w-2 h-2 rounded-full bg-gold shrink-0" aria-label="Unread"></span>
					{/if}
					<span class="flex-1">
						<span class="block text-sm text-espresso">{message(n)}</span>
						{#if n.type === 'wishlist_shared' && n.data?.message}
							<span class="block text-xs text-gold-dark mt-0.5">"{n.data.message}"</span>
						{/if}
					</span>
					<span class="text-[11px] text-gold-muted shrink-0">{relativeTime(n.createdAt)}</span>
				</button>
			{/each}
		</div>
	{/if}
</div>
