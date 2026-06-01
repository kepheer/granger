<script lang="ts">
	import { onMount } from 'svelte';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { Input } from '$lib/components/ui/input';
	import { locale } from '$lib/i18n';
	import { ArrowLeftIcon, UserCirclePlusIcon } from 'phosphor-svelte';

	const copy = {
		en: {
			title: 'Add user',
			description:
				'Only the display name is required. Granger generates the internal ID and you choose which entrypoints this user may receive.',
			back: 'Back',
			name: 'Display name',
			access: 'Allowed entrypoints',
			create: 'Create user'
		},
		ru: {
			title: 'Добавить пользователя',
			description:
				'Нужно только отображаемое имя. Granger сам сгенерирует внутренний ID, а вы выберете подключения, для которых можно выдавать конфиги.',
			back: 'Назад',
			name: 'Отображаемое имя',
			access: 'Разрешенные подключения',
			create: 'Создать пользователя'
		}
	} as const;

	let displayName = $state('');
	let outputs = $state<string[]>([]);
	let selectedOutput = $state('');
	let message = $state('');

	onMount(async () => {
		try {
			const response = await fetch('/api/config');
			if (!response.ok) return;
			const body = await response.json();
			outputs = Object.keys(body.config?.outputs ?? {});
			selectedOutput = outputs[0] ?? '';
		} catch {
			// Frontend-only development mode.
		}
	});

	async function create() {
		try {
			const response = await fetch('/api/users', {
				method: 'POST',
				headers: { 'content-type': 'application/json', 'x-granger-request': '1' },
				body: JSON.stringify({ display_name: displayName, output: selectedOutput })
			});
			if (!response.ok) throw new Error(await response.text());
			window.location.href = '/entrypoints';
		} catch (error) {
			message = error instanceof Error ? error.message : String(error);
		}
	}
</script>

<svelte:head>
	<title>{copy[$locale].title} · Granger</title>
</svelte:head>

<section class="max-w-3xl space-y-5">
	<Button href="/users" variant="ghost"><ArrowLeftIcon />{copy[$locale].back}</Button>
	<div>
		<h1 class="font-display text-2xl font-bold text-white">{copy[$locale].title}</h1>
		<p class="mt-2 max-w-3xl text-sm text-muted-foreground">{copy[$locale].description}</p>
	</div>
	<Card.Root class="rounded-lg">
		<Card.Content class="space-y-5 pt-6">
			<label class="grid gap-1.5 text-xs text-muted-foreground">{copy[$locale].name}<Input bind:value={displayName} placeholder="Alice" /></label>
			<div class="space-y-2">
				<div class="text-xs text-muted-foreground">{copy[$locale].access}</div>
				<div class="flex flex-wrap gap-2">
					{#each outputs as output}
						<Button variant={selectedOutput === output ? 'default' : 'outline'} size="sm" onclick={() => (selectedOutput = output)}>{output}</Button>
					{/each}
				</div>
			</div>
			{#if message}<p class="text-sm text-red-300">{message}</p>{/if}
			<Button onclick={create}><UserCirclePlusIcon />{copy[$locale].create}</Button>
		</Card.Content>
	</Card.Root>
</section>
