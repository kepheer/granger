<script lang="ts">
	import * as Select from '$lib/components/ui/select';

	let {
		id,
		options,
		value = $bindable(),
		disabled = false
	}: {
		id: string;
		options: string[];
		value?: string;
		disabled?: boolean;
	} = $props();

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
	<Select.Trigger {id} class="w-full min-w-0 text-foreground">
		<Select.Value />
	</Select.Trigger>
	<Select.Content>
		<Select.Group>
			{#each items as item (item.value)}
				<Select.Item value={item.value} label={item.label} />
			{/each}
		</Select.Group>
	</Select.Content>
</Select.Root>
