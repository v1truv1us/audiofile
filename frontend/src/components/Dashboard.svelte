<script lang="ts">
	import { apiFetch } from '../lib/api';

	type Stats = {
		collectionCount: number;
		forSaleCount: number;
		wishlistCount: number;
		totalValue: number;
	};

	let stats: Stats = $state({ collectionCount: 0, forSaleCount: 0, wishlistCount: 0, totalValue: 0 });
	let recent: any[] = $state([]);
	let loading = $state(true);

	async function fetchDashboard() {
		loading = true;
		try {
			const [statsRes, collectionRes] = await Promise.all([
				apiFetch('/api/collection/stats'),
				apiFetch('/api/collection?limit=3'),
			]);
			stats = await statsRes.json();
			recent = await collectionRes.json();
		} catch (e) {
			console.error('Failed to fetch dashboard', e);
		} finally {
			loading = false;
		}
	}

	$effect(() => {
		fetchDashboard();
	});

	let formattedValue = $derived(
		stats.totalValue ? '$' + stats.totalValue.toLocaleString('en-US', { minimumFractionDigits: 0, maximumFractionDigits: 0 }) : '$0'
	);
</script>

<div class="space-y-10">
	<div class="border-b border-gold-muted/30 pb-6">
		<h1 class="font-serif text-4xl text-espresso mb-1 font-normal">Your Crates</h1>
		<p class="text-gold-dark text-xs tracking-[0.12em] uppercase">
			{stats.collectionCount} pressings · {stats.wishlistCount} on the wishlist · {stats.forSaleCount} for sale
		</p>
	</div>

	<div class="grid grid-cols-1 sm:grid-cols-3 gap-4">
		<div class="bg-espresso rounded-lg p-5">
			<div class="font-serif text-4xl text-gold leading-none mb-2">{stats.collectionCount}</div>
			<div class="text-gold-mid text-[10px] tracking-[0.12em] uppercase">In the collection</div>
		</div>
		<div class="bg-espresso rounded-lg p-5">
			<div class="font-serif text-4xl text-gold leading-none mb-2">{formattedValue}</div>
			<div class="text-gold-mid text-[10px] tracking-[0.12em] uppercase">Estimated value</div>
		</div>
		<div class="bg-espresso rounded-lg p-5">
			<div class="font-serif text-4xl text-gold leading-none mb-2">{stats.wishlistCount}</div>
			<div class="text-gold-mid text-[10px] tracking-[0.12em] uppercase">On the wishlist</div>
		</div>
	</div>

	{#if loading}
		<div class="text-center py-12 text-gold-muted text-xs tracking-widest uppercase">Loading...</div>
	{:else if recent.length > 0}
		<div>
			<div class="flex items-center gap-3 mb-5">
				<span class="text-gold-dark text-[10px] tracking-[0.14em] uppercase">Recent additions</span>
				<div class="flex-1 h-px bg-gold-muted opacity-30"></div>
			</div>

			<div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
				{#each recent as item (item.id)}
					{@const theme = {
						'Blue Note': { labelColor: '#BA7517', discBg: '#2C2C2A' },
						'Impulse!': { labelColor: '#534AB7', discBg: '#1e1d3a' },
						'Island': { labelColor: '#0F6E56', discBg: '#0d2b24' },
						'Warner': { labelColor: '#712B13', discBg: '#2a1a2e' },
						'Reprise': { labelColor: '#185FA5', discBg: '#0e1f30' },
						'Capitol': { labelColor: '#B5291E', discBg: '#2a1515' },
						'Columbia': { labelColor: '#A0001C', discBg: '#1e0e12' },
						'Verve': { labelColor: '#1A3C6E', discBg: '#0e1a2e' },
					}[item.release.label] ?? { labelColor: '#854F0B', discBg: '#1a1a18' }}
					{@const labelWords = (item.release.label ?? '').split(' ')}
					<div class="bg-white border border-gold/60 rounded-lg overflow-hidden">
						<div class="h-28 flex items-center justify-center" style="background: {theme.discBg};">
							<svg width="88" height="88" viewBox="0 0 88 88" aria-hidden="true">
								<circle cx="44" cy="44" r="43" fill="#111" stroke="rgba(255,255,255,0.06)" stroke-width="0.5"/>
								<circle cx="44" cy="44" r="37" fill="none" stroke="rgba(255,255,255,0.07)" stroke-width="0.8"/>
								<circle cx="44" cy="44" r="31" fill="none" stroke="rgba(255,255,255,0.07)" stroke-width="0.8"/>
								<circle cx="44" cy="44" r="25" fill="none" stroke="rgba(255,255,255,0.07)" stroke-width="0.8"/>
								<circle cx="44" cy="44" r="19" fill="none" stroke="rgba(255,255,255,0.07)" stroke-width="0.8"/>
								<circle cx="44" cy="44" r="14" fill={theme.labelColor}/>
								<text x="44" y="41" text-anchor="middle" font-size="5" fill="rgba(255,255,255,0.9)" font-family="Georgia,serif">{labelWords[0]}</text>
								{#if labelWords[1]}
									<text x="44" y="48" text-anchor="middle" font-size="5" fill="rgba(255,255,255,0.9)" font-family="Georgia,serif">{labelWords[1]}</text>
								{/if}
								<circle cx="44" cy="44" r="2" fill="rgba(0,0,0,0.5)"/>
							</svg>
						</div>
						<div class="p-3.5 border-t border-gold/40">
							<div class="font-serif text-sm text-espresso truncate mb-0.5">{item.release.title}</div>
							<div class="text-[11px] text-gold-dark mb-2.5">{item.release.artist}</div>
							<div class="flex items-center justify-between">
								<span class="text-[10px] bg-espresso text-gold px-2 py-0.5 rounded tracking-wide">{item.mediaCondition}</span>
								<span class="text-[10px] text-gold-muted">{item.release.year} · {item.notes || 'Unknown'}</span>
							</div>
						</div>
					</div>
				{/each}
			</div>
		</div>
	{/if}

	<div class="flex items-center justify-between pt-2">
		<a href="/collection" class="font-serif italic text-sm text-gold-dark hover:text-espresso transition-colors">Browse all {stats.collectionCount} records →</a>
		<a href="/collection?add=1" class="bg-espresso text-gold text-xs tracking-[0.1em] uppercase px-5 py-2.5 rounded hover:bg-espresso/90 transition-colors">+ Add a Record</a>
	</div>
</div>
