<script lang="ts">
	import { onMount } from 'svelte';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import * as Sheet from '$lib/components/ui/sheet';
	import QRCode from 'qrcode';
	import {
		DownloadSimpleIcon,
		PlusIcon,
		QrCodeIcon,
		CopyIcon,
		PowerIcon
	} from 'phosphor-svelte';

	const copy = {
		title: 'Entrypoints',
		description:
			'Entrypoints are client-facing connection profiles. Granger issues configs, controls access by user, and keeps protocol defaults friendly.',
		add: 'Add entrypoint',
		name: 'Name',
		type: 'Type',
		iface: 'Interface',
		subnet: 'Subnet',
		port: 'Port',
		clients: 'Clients',
		status: 'Status',
		actions: 'Config',
		enabled: 'enabled',
		disabled: 'disabled',
		download: 'Download',
		copy: 'Copy',
		qr: 'QR code',
		disable: 'Revoke',
		enable: 'Restore'
	} as const;

	let entrypoints = $state([
		{
			name: 'home_wg',
			type: 'wireguard',
			iface: 'wg0',
			subnet: '10.66.66.0/24',
			port: '51820',
			clients: [
				{ name: 'alice-phone', user: 'Alice', ip: '10.66.66.2', status: 'enabled' },
				{ name: 'bob-laptop', user: 'Bob', ip: '10.66.66.3', status: 'disabled' }
			]
		},
		{
			name: 'guest_ovpn',
			type: 'openvpn',
			iface: 'tun0',
			subnet: '10.77.0.0/24',
			port: '1194',
			clients: [{ name: 'guest-profile', user: 'Guest access', ip: '10.77.0.2', status: 'enabled' }]
		}
	]);
	let qrOpen = $state(false);
	let qrImage = $state('');
	let qrName = $state('');

	onMount(() => {
		void loadEntrypoints();
	});

	async function copyProfile(output: string, client: string) {
		const response = await fetch(`/api/profiles/${encodeURIComponent(output)}/${encodeURIComponent(client)}`);
		if (!response.ok) return;
		await navigator.clipboard.writeText(await response.text());
	}

	async function showQR(output: string, client: string) {
		const response = await fetch(`/api/profiles/${encodeURIComponent(output)}/${encodeURIComponent(client)}`);
		if (!response.ok) return;
		qrName = client;
		qrImage = await QRCode.toDataURL(await response.text(), { margin: 2, width: 360 });
		qrOpen = true;
	}

	async function toggleProfile(user: string, disabled: boolean) {
		const response = await fetch(`/api/users/${encodeURIComponent(user)}`, {
			method: 'PUT',
			headers: { 'content-type': 'application/json', 'x-granger-request': '1' },
			body: JSON.stringify({ disabled })
		});
		if (response.ok) await loadEntrypoints();
	}

	async function loadEntrypoints() {
		try {
			const response = await fetch('/api/config');
			if (!response.ok) throw new Error(response.statusText);
			const body = await response.json();
			const outputs = body.config?.outputs ?? {};
			entrypoints = Object.entries(outputs).map(([name, value]) => {
				const item = value as Record<string, unknown>;
				return {
					name,
					type: String(item.type ?? ''),
					iface: String(item.interface ?? ''),
					subnet: String(item.subnet ?? ''),
					port: String(item.listen_port ?? ''),
					clients: ((item.clients as Array<Record<string, unknown>>) ?? []).map((client) => ({
						name: String(client.name ?? ''),
						user: String(client.user ?? ''),
						ip: String(client.ip ?? ''),
						status: client.disabled ? 'disabled' : 'enabled'
					}))
				};
			});
		} catch {
			// Keep preview data while running frontend-only development mode.
		}
	}
</script>

<svelte:head>
	<title>{copy.title} · Granger</title>
</svelte:head>

<section class="space-y-5">
	<div class="flex flex-col justify-between gap-4 md:flex-row md:items-end">
		<div>
			<h1 class="font-display text-2xl font-bold text-white">{copy.title}</h1>
			<p class="mt-2 max-w-3xl text-sm text-muted-foreground">{copy.description}</p>
		</div>
		<div class="flex flex-wrap gap-2">
			<Button href="/entrypoints/new"><PlusIcon />{copy.add}</Button>
		</div>
	</div>

	<div class="space-y-4">
		{#each entrypoints as entrypoint}
			<Card.Root class="rounded-lg">
				<Card.Header class="flex-row items-start justify-between gap-4">
					<div>
						<Card.Title>{entrypoint.name}</Card.Title>
						<Card.Description>{entrypoint.type} · {entrypoint.iface} · {entrypoint.subnet}</Card.Description>
					</div>
					<div class="rounded-full border border-border px-3 py-1 text-xs text-muted-foreground">
						{copy.port} {entrypoint.port}
					</div>
				</Card.Header>
				<Card.Content>
					<div class="overflow-x-auto rounded-lg border border-border">
						<div
							class="grid min-w-[900px] grid-cols-[1fr_.75fr_.7fr_.7fr_1.9fr] bg-white/[0.035] px-4 py-2 text-xs font-medium text-muted-foreground"
						>
							<div>{copy.name}</div>
							<div>{copy.clients}</div>
							<div>IP</div>
							<div>{copy.status}</div>
							<div>{copy.actions}</div>
						</div>
						{#each entrypoint.clients as client}
							<div
								class="grid min-w-[900px] grid-cols-[1fr_.75fr_.7fr_.7fr_1.9fr] items-center border-t border-border px-4 py-3 text-sm"
							>
								<div class="font-medium text-white">{client.name}</div>
								<div class="text-muted-foreground">{client.user}</div>
								<div class="font-mono text-xs text-white/80">{client.ip}</div>
								<div class={client.status === 'enabled' ? 'text-emerald-300' : 'text-red-300'}>
									{client.status === 'enabled' ? copy.enabled : copy.disabled}
								</div>
								<div class="flex flex-wrap gap-1">
									<Button size="xs" variant="ghost" href={`/api/profiles/${entrypoint.name}/${client.name}`}><DownloadSimpleIcon />{copy.download}</Button>
									<Button size="xs" variant="ghost" onclick={() => copyProfile(entrypoint.name, client.name)}><CopyIcon />{copy.copy}</Button>
									<Button size="xs" variant="outline" onclick={() => showQR(entrypoint.name, client.name)}><QrCodeIcon />{copy.qr}</Button>
									<Button size="xs" variant={client.status === 'enabled' ? 'destructive' : 'outline'} onclick={() => toggleProfile(client.user, client.status === 'enabled')}><PowerIcon />{client.status === 'enabled' ? copy.disable : copy.enable}</Button>
								</div>
							</div>
						{/each}
					</div>
				</Card.Content>
			</Card.Root>
		{/each}
	</div>
</section>

<Sheet.Root bind:open={qrOpen}>
	<Sheet.Content class="w-full sm:max-w-md">
		<Sheet.Header>
			<Sheet.Title>{copy.qr}: {qrName}</Sheet.Title>
			<Sheet.Description>WireGuard</Sheet.Description>
		</Sheet.Header>
		{#if qrImage}
			<div class="mt-6 rounded-md bg-white p-4">
				<img class="mx-auto block w-full max-w-[360px]" src={qrImage} alt={qrName} />
			</div>
		{/if}
	</Sheet.Content>
</Sheet.Root>
