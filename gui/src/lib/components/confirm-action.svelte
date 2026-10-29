<script lang="ts">
	import { Dialog } from 'bits-ui';
	import { Button } from '$lib/components/ui/button';

	let {
		open = $bindable(false),
		title,
		description,
		action,
		danger = false,
		busy = false,
		error = '',
		onconfirm
	}: {
		open?: boolean;
		title: string;
		description: string;
		action: string;
		danger?: boolean;
		busy?: boolean;
		error?: string;
		onconfirm: () => void | Promise<void>;
	} = $props();
</script>

<Dialog.Root bind:open>
	<Dialog.Portal>
		<Dialog.Overlay class="fixed inset-0 z-50 bg-black/70 backdrop-blur-sm" />
		<Dialog.Content class="fixed left-1/2 top-1/2 z-50 w-[calc(100%-2rem)] max-w-md -translate-x-1/2 -translate-y-1/2 space-y-4 rounded-xl border border-border bg-popover p-6 text-popover-foreground shadow-2xl">
			<Dialog.Title class="font-display text-lg font-semibold">{title}</Dialog.Title>
			<Dialog.Description class="text-sm text-muted-foreground">{description}</Dialog.Description>
			{#if error}
				<p role="alert" class="text-sm text-red-300">{error}</p>
			{/if}
			<div class="flex flex-wrap justify-end gap-2 pt-2">
				<Dialog.Close>
					{#snippet child({ props })}
						<Button {...props} variant="outline" disabled={busy}>Cancel</Button>
					{/snippet}
				</Dialog.Close>
				<Button
					variant={danger ? 'destructive' : 'default'}
					aria-disabled={busy}
					aria-busy={busy}
					onclick={() => {
						if (busy) {
							return;
						}

						void onconfirm();
					}}
				>
					{busy ? 'Applying…' : action}
				</Button>
			</div>
		</Dialog.Content>
	</Dialog.Portal>
</Dialog.Root>
