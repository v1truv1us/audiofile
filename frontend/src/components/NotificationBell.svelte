<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { apiFetch } from '../lib/api';

	let loggedIn = $state(false);
	let unread = $state(0);
	let interval: ReturnType<typeof setInterval> | undefined;

	async function fetchUnreadCount() {
		try {
			const res = await apiFetch('/api/notifications/unread-count');
			if (!res.ok) return;
			const data = await res.json();
			unread = data.count ?? 0;
		} catch {
		}
	}

	onMount(async () => {
		const { supabase } = await import('../lib/supabase');
		const { data: { session } } = await supabase.auth.getSession();
		loggedIn = Boolean(session);
		if (!loggedIn) return;
		await fetchUnreadCount();
		interval = setInterval(fetchUnreadCount, 30000);
	});

	onDestroy(() => {
		if (interval) clearInterval(interval);
	});
</script>

{#if loggedIn}
	<a href="/notifications" class="relative text-gold opacity-70 hover:opacity-100 transition-opacity p-1" aria-label="Notifications">
		<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" aria-hidden="true">
			<path d="M18 8a6 6 0 0 0-12 0c0 7-3 9-3 9h18s-3-2-3-9" stroke-linecap="round" stroke-linejoin="round"/>
			<path d="M13.7 21a2 2 0 0 1-3.4 0" stroke-linecap="round" stroke-linejoin="round"/>
		</svg>
		{#if unread > 0}
			<span class="absolute -top-1 -right-1 bg-gold text-espresso text-[9px] font-medium min-w-[14px] h-[14px] px-0.5 rounded-full flex items-center justify-center">{unread > 9 ? '9+' : unread}</span>
		{/if}
	</a>
{/if}
