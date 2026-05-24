<script lang="ts">
	type Props = {
		title: string;
		artist: string;
		priority: number;
		targetPrice: number | null;
		notes: string;
		label: string;
		onEdit: () => void;
		onRemove: () => void;
		onPurchased: () => void;
	};

	let { title, artist, priority, targetPrice, notes, label, onEdit, onRemove, onPurchased }: Props = $props();

	let priorityLabel = $derived(priority <= 3 ? 'High' : priority <= 6 ? 'Medium' : 'Low');
	let priorityClass = $derived(priority <= 3 ? 'text-gold-dark' : 'text-gold-muted');

	let theme = $derived(
		label === 'Blue Note' ? '#BA7517' :
		label === 'Impulse!' ? '#534AB7' :
		label === 'Island' ? '#0F6E56' :
		label === 'Capitol' ? '#B5291E' :
		label === 'Columbia' ? '#A0001C' :
		label === 'Verve' ? '#1A3C6E' :
		'#854F0B'
	);
</script>

<div class="bg-white border border-gold/50 rounded-lg px-5 py-4 flex items-center gap-5 hover:border-gold transition-colors">
	<div class="w-10 h-10 flex-shrink-0">
		<svg width="40" height="40" viewBox="0 0 40 40" aria-hidden="true">
			<circle cx="20" cy="20" r="19" fill="#1a1a18" stroke="rgba(255,255,255,0.06)" stroke-width="0.5"/>
			<circle cx="20" cy="20" r="15" fill="none" stroke="rgba(255,255,255,0.07)" stroke-width="0.7"/>
			<circle cx="20" cy="20" r="11" fill="none" stroke="rgba(255,255,255,0.07)" stroke-width="0.7"/>
			<circle cx="20" cy="20" r="7" fill={theme}/>
			<circle cx="20" cy="20" r="1.5" fill="rgba(0,0,0,0.5)"/>
		</svg>
	</div>
	<div class="flex-1 min-w-0">
		<div class="font-serif text-sm text-espresso">{title}</div>
		<div class="text-[11px] text-gold-dark mt-0.5">{artist}</div>
		{#if notes}
			<div class="text-[10px] text-gold-muted mt-1 italic">{notes}</div>
		{/if}
	</div>
	<div class="text-right flex-shrink-0">
		{#if targetPrice}
			<div class="text-xs font-medium text-espresso">${targetPrice}</div>
		{/if}
		<div class="text-[9px] tracking-[0.1em] uppercase mt-1 {priorityClass}">{priorityLabel} priority</div>
		<div class="flex justify-end gap-2 mt-2">
			<button type="button" class="text-[10px] text-gold-dark underline" on:click={onEdit}>Edit</button>
			<button type="button" class="text-[10px] text-gold-dark underline" on:click={onPurchased}>Purchased</button>
			<button type="button" class="text-[10px] text-red-700 underline" on:click={onRemove}>Remove</button>
		</div>
	</div>
</div>
