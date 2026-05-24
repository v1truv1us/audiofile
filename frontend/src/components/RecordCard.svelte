<script context="module" lang="ts">
	export type LabelTheme = {
		labelColor: string;
		labelText: string;
		discBg: string;
	};

	export function labelThemeFor(label: string): LabelTheme {
		const themes: Record<string, LabelTheme> = {
			'Blue Note': { labelColor: '#BA7517', labelText: 'Blue Note', discBg: '#2C2C2A' },
			'Impulse!': { labelColor: '#534AB7', labelText: 'Impulse!', discBg: '#1e1d3a' },
			'Island': { labelColor: '#0F6E56', labelText: 'Island', discBg: '#0d2b24' },
			'Warner': { labelColor: '#712B13', labelText: 'Warner', discBg: '#2a1a2e' },
			'Reprise': { labelColor: '#185FA5', labelText: 'Reprise', discBg: '#0e1f30' },
			'Capitol': { labelColor: '#B5291E', labelText: 'Capitol', discBg: '#2a1515' },
			'Columbia': { labelColor: '#A0001C', labelText: 'Columbia', discBg: '#1e0e12' },
			'Verve': { labelColor: '#1A3C6E', labelText: 'Verve', discBg: '#0e1a2e' },
		};
		return themes[label] ?? { labelColor: '#854F0B', labelText: label, discBg: '#1a1a18' };
	}
</script>

<script lang="ts">
	import { apiFetch } from '../lib/api';

	type Props = {
		id: string;
		title: string;
		artist: string;
		year: number | null;
		grade: string;
		sleeveGrade: string;
		pressing: string;
		label: string;
		coverUrl?: string;
		purchasePrice: number | null;
		notes: string;
		onChanged?: () => void;
	};

	let {
		id, title, artist, year, grade, sleeveGrade = 'VG', pressing, label,
		coverUrl = '', purchasePrice = null, notes = '', onChanged,
	}: Props = $props();

	let theme = $derived(labelThemeFor(label));
	let labelWords = $derived(theme.labelText.split(' '));
	let editing = $state(false);
	let deleting = $state(false);
	let saving = $state(false);
	let error = $state('');

	let editForm = $state({
		mediaCondition: grade,
		sleeveCondition: sleeveGrade,
		purchasePrice: purchasePrice ? String(purchasePrice) : '',
		notes: notes,
	});

	async function saveEdit() {
		saving = true;
		error = '';
		try {
			const res = await apiFetch(`/api/collection/${id}`, {
				method: 'PUT',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({
					mediaCondition: editForm.mediaCondition,
					sleeveCondition: editForm.sleeveCondition,
					purchasePrice: editForm.purchasePrice ? Number(editForm.purchasePrice) : null,
					notes: editForm.notes,
				}),
			});
			if (!res.ok) throw new Error(await res.text());
			editing = false;
			grade = editForm.mediaCondition;
			sleeveGrade = editForm.sleeveCondition;
			onChanged?.();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to update';
		} finally {
			saving = false;
		}
	}

	async function deleteItem() {
		if (!confirm(`Remove "${title}" from your collection?`)) return;
		deleting = true;
		error = '';
		try {
			const res = await apiFetch(`/api/collection/${id}`, { method: 'DELETE' });
			if (!res.ok) throw new Error(await res.text());
			onChanged?.();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to delete';
			deleting = false;
		}
	}
</script>

<div class="bg-white border border-gold/60 rounded-lg overflow-hidden group">
	<div class="h-28 flex items-center justify-center overflow-hidden" style="background: {theme.discBg};">
		{#if coverUrl}
			<img src={coverUrl} alt="{title} cover art" class="h-full w-full object-cover group-hover:scale-105 transition-transform" loading="lazy" />
		{:else}
		<svg width="88" height="88" viewBox="0 0 88 88" aria-hidden="true" class="group-hover:scale-105 transition-transform">
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
		{/if}
	</div>
	<div class="p-3.5 border-t border-gold/40">
		<div class="font-serif text-sm text-espresso truncate mb-0.5">{title}</div>
		<div class="text-[11px] text-gold-dark mb-2.5">{artist}</div>

		{#if !editing}
			<div class="flex items-center justify-between mb-2">
				<span class="text-[10px] bg-espresso text-gold px-2 py-0.5 rounded tracking-wide">{grade}</span>
				<span class="text-[10px] text-gold-muted">{year ?? ''} · {pressing}</span>
			</div>
			<div class="flex gap-2">
				<button type="button" class="text-[10px] text-gold-dark hover:text-espresso transition-colors" onclick={() => { editForm.mediaCondition = grade; editForm.sleeveCondition = sleeveGrade; editForm.purchasePrice = purchasePrice ? String(purchasePrice) : ''; editForm.notes = notes; editing = true; }}>Edit</button>
				<button type="button" class="text-[10px] text-red-600/60 hover:text-red-700 transition-colors" onclick={deleteItem} disabled={deleting}>{deleting ? 'Removing...' : 'Remove'}</button>
			</div>
		{:else}
			<div class="space-y-2 mt-1">
				<label class="block text-[10px] text-gold-dark uppercase tracking-wide">Media
					<select class="block w-full border border-gold/40 rounded px-2 py-1 text-espresso text-xs normal-case" bind:value={editForm.mediaCondition}>
						{#each ['M','NM','VG+','VG','G+','G','F','P'] as c}<option value={c}>{c}</option>{/each}
					</select>
				</label>
				<label class="block text-[10px] text-gold-dark uppercase tracking-wide">Sleeve
					<select class="block w-full border border-gold/40 rounded px-2 py-1 text-espresso text-xs normal-case" bind:value={editForm.sleeveCondition}>
						{#each ['M','NM','VG+','VG','G+','G','F','P'] as c}<option value={c}>{c}</option>{/each}
					</select>
				</label>
				<label class="block text-[10px] text-gold-dark uppercase tracking-wide">Price
					<input type="number" step="0.01" class="block w-full border border-gold/40 rounded px-2 py-1 text-espresso text-xs normal-case" bind:value={editForm.purchasePrice} />
				</label>
				<label class="block text-[10px] text-gold-dark uppercase tracking-wide">Notes
					<textarea class="block w-full border border-gold/40 rounded px-2 py-1 text-espresso text-xs normal-case" rows="2" bind:value={editForm.notes}></textarea>
				</label>
				{#if error}<p class="text-[10px] text-red-700">{error}</p>{/if}
				<div class="flex gap-2">
					<button type="button" class="text-[10px] bg-espresso text-gold px-3 py-1 rounded disabled:opacity-60" onclick={saveEdit} disabled={saving}>{saving ? 'Saving...' : 'Save'}</button>
					<button type="button" class="text-[10px] text-gold-dark px-3 py-1" onclick={() => { editing = false; error = ''; }}>Cancel</button>
				</div>
			</div>
		{/if}
	</div>
</div>
