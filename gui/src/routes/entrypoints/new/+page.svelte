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

	const protocols = [
		{ type: 'wireguard', label: 'WireGuard', name: 'home_wg', iface: 'wg0', subnet: '10.66.66.0/24', port: 51820 },
		{ type: 'openvpn', label: 'OpenVPN', name: 'home_ovpn', iface: 'tun0', subnet: '10.77.0.0/24', port: 1194 },
		{ type: 'amneziawg', label: 'AmneziaWG', name: 'home_awg', iface: 'awg0', subnet: '10.88.0.0/24', port: 51821 },
		{ type: 'xray', label: 'Xray', name: 'home_xray', iface: '', subnet: '', port: 443 },
		{ type: 'sing-box', label: 'sing-box', name: 'home_singbox', iface: '', subnet: '', port: 443 }
	];

	let selected = $state(protocols[0]);
	let name = $state(protocols[0].name);
	let iface = $state(protocols[0].iface);
	let subnet = $state(protocols[0].subnet);
	let port = $state(protocols[0].port);
	let message = $state('');
	let saving = $state(false);

	function choose(protocol: (typeof protocols)[number]) {
		selected = protocol;
		name = protocol.name;
		iface = protocol.iface;
		subnet = protocol.subnet;
		port = protocol.port;
	}

	async function create() {
		saving = true;
		try {
			const response = await fetch('/api/outputs', {
				method: 'POST',
				headers: { 'content-type': 'application/json', 'x-granger-request': '1' },
				body: JSON.stringify({
					name,
					output: { type: selected.type, interface: iface, subnet, listen_port: Number(port), clients: [] }
				})
			});
			if (!response.ok) throw new Error(await response.text());
			window.location.href = '/entrypoints';
		} catch (error) {
			message = error instanceof Error ? error.message : String(error);
		} finally {
			saving = false;
		}
	}
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
					<Button variant={selected.type === protocol.type ? 'default' : 'outline'} size="sm" onclick={() => choose(protocol)}>{protocol.label}</Button>
				{/each}
			</div>
			<div class="grid gap-3 md:grid-cols-2">
				<label class="grid gap-1.5 text-xs text-muted-foreground">{copy[$locale].name}<Input bind:value={name} placeholder="home_wg" /></label>
				<label class="grid gap-1.5 text-xs text-muted-foreground">{copy[$locale].iface}<Input bind:value={iface} placeholder="wg0" /></label>
				<label class="grid gap-1.5 text-xs text-muted-foreground">{copy[$locale].subnet}<Input bind:value={subnet} placeholder="10.66.66.0/24" /></label>
				<label class="grid gap-1.5 text-xs text-muted-foreground">{copy[$locale].port}<Input bind:value={port} type="number" placeholder="51820" /></label>
			</div>
			{#if message}<p class="text-sm text-red-300">{message}</p>{/if}
			<Button disabled={saving} onclick={create}><PlusIcon />{copy[$locale].create}</Button>
		</Card.Content>
	</Card.Root>
</section>
