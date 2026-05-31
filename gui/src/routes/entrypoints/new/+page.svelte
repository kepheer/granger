<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { Input } from '$lib/components/ui/input';
	import { locale } from '$lib/i18n';
	import { ArrowLeftIcon, PlusIcon } from 'phosphor-svelte';

	const copy = {
		en: {
			title: 'Add entrypoint',
			description:
				'Start with the suggested interface, subnet, and port. Granger can later issue user configs from this entrypoint.',
			back: 'Back',
			type: 'Protocol',
			name: 'Name',
			iface: 'Interface',
			subnet: 'Subnet',
			port: 'Port',
			create: 'Create entrypoint'
		},
		ru: {
			title: 'Добавить подключение',
			description:
				'Начните с предложенного интерфейса, подсети и порта. Потом Granger сможет выдавать пользовательские конфиги из этого подключения.',
			back: 'Назад',
			type: 'Протокол',
			name: 'Имя',
			iface: 'Интерфейс',
			subnet: 'Подсеть',
			port: 'Порт',
			create: 'Создать подключение'
		}
	} as const;

	const protocols = ['WireGuard', 'OpenVPN', 'AmneziaWG', 'Xray'];
</script>

<svelte:head>
	<title>{copy[$locale].title} · Granger</title>
</svelte:head>

<section class="max-w-3xl space-y-5">
	<Button href="/entrypoints" variant="ghost"><ArrowLeftIcon />{copy[$locale].back}</Button>
	<div>
		<h1 class="font-display text-2xl font-bold text-white">{copy[$locale].title}</h1>
		<p class="mt-2 max-w-3xl text-sm text-muted-foreground">{copy[$locale].description}</p>
	</div>
	<Card.Root class="rounded-lg">
		<Card.Content class="space-y-5 pt-6">
			<div class="flex flex-wrap gap-2">
				{#each protocols as protocol}
					<Button variant="outline" size="sm">{protocol}</Button>
				{/each}
			</div>
			<div class="grid gap-3 md:grid-cols-2">
				<label class="grid gap-1.5 text-xs text-muted-foreground">{copy[$locale].name}<Input placeholder="home_wg" /></label>
				<label class="grid gap-1.5 text-xs text-muted-foreground">{copy[$locale].iface}<Input placeholder="wg0" /></label>
				<label class="grid gap-1.5 text-xs text-muted-foreground">{copy[$locale].subnet}<Input placeholder="10.66.66.0/24" /></label>
				<label class="grid gap-1.5 text-xs text-muted-foreground">{copy[$locale].port}<Input placeholder="51820" /></label>
			</div>
			<Button><PlusIcon />{copy[$locale].create}</Button>
		</Card.Content>
	</Card.Root>
</section>
