<script lang="ts">
	import * as Select from '$lib/components/ui/select';
	import { cn } from '$lib/utils';

	let {
		id,
		class: className,
		options,
		value = $bindable(),
		disabled = false,
		'aria-describedby': ariaDescribedby
	}: {
		id: string;
		class?: string;
		options: string[];
		value?: string;
		disabled?: boolean;
		'aria-describedby'?: string;
	} = $props();

	let trigger = $state<HTMLButtonElement | null>(null);
	const portalTarget = $derived(
		trigger?.closest('[role="dialog"]') ?? undefined
	);
	const prefix = 'upstream:';

	const items = $derived([
		{ value: 'shared', label: 'Shared routing rules' },
		...options.map((upstream) => ({
			value: prefix + upstream,
			label: upstream
		}))
	]);
</script>

<Select.Root
	type="single"
	{items}
	{disabled}
	value={value === undefined || value === '' ? 'shared' : prefix + value}
	onValueChange={(selected) => {
		if (selected === 'shared') {
			value = '';
			return;
		}

		value = selected.slice(prefix.length);
	}}
>
	<Select.Trigger
		bind:ref={trigger}
		{id}
		aria-describedby={ariaDescribedby}
		class={cn('w-full min-w-0 text-foreground', className)}
	>
		<Select.Value />
	</Select.Trigger>
	<Select.Content portalProps={{ to: portalTarget }}>
		<Select.Group>
			{#each items as item (item.value)}
				<Select.Item value={item.value} label={item.label} />
			{/each}
		</Select.Group>
	</Select.Content>
</Select.Root>
