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
	type Props = {
		title: string;
		artist: string;
		year: number | null;
		grade: string;
		pressing: string;
		label: string;
	};

	let { title, artist, year, grade, pressing, label }: Props = $props();

	let theme = $derived(labelThemeFor(label));
	let labelWords = $derived(theme.labelText.split(' '));
</script>

<div class="bg-white border border-gold/60 rounded-lg overflow-hidden cursor-pointer hover:border-gold transition-colors group">
	<div class="h-28 flex items-center justify-center" style="background: {theme.discBg};">
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
	</div>
	<div class="p-3.5 border-t border-gold/40">
		<div class="font-serif text-sm text-espresso truncate mb-0.5">{title}</div>
		<div class="text-[11px] text-gold-dark mb-2.5">{artist}</div>
		<div class="flex items-center justify-between">
			<span class="text-[10px] bg-espresso text-gold px-2 py-0.5 rounded tracking-wide">{grade}</span>
			<span class="text-[10px] text-gold-muted">{year ?? ''} · {pressing}</span>
		</div>
	</div>
</div>
