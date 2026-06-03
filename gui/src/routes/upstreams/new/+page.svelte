<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { Input } from '$lib/components/ui/input';
	import { Textarea } from '$lib/components/ui/textarea';
	import { ArrowLeftIcon, UploadSimpleIcon } from 'phosphor-svelte';

	const copy = {
		title: 'Add upstream',
		description:
			'Choose a protocol, keep the suggested interface unless you need a specific name, then paste or upload the provider config. Granger will place it under the upstream config directory.',
		back: 'Back',
		type: 'Protocol',
		name: 'Name',
		iface: 'Interface',
		config: 'Inline config',
		upload: 'Upload config',
		create: 'Create upstream',
		suggested: 'Suggested defaults',
		saving: 'Installing protocol and creating upstream…',
		error: 'Could not create upstream'
	} as const;

	const protocols = [
		{ type: 'wireguard', label: 'WireGuard', name: 'wg_up_1', iface: 'wg-up0' },
		{ type: 'amneziawg', label: 'AmneziaWG', name: 'awg_default', iface: 'awg0' },
		{ type: 'openvpn', label: 'OpenVPN', name: 'ovpn_exit', iface: 'tun0' },
		{ type: 'snx-rs', label: 'SNX-RS', name: 'snx_exit', iface: 'snx0' },
		{ type: 'xray', label: 'Xray', name: 'xray_exit', iface: 'tun-xray0' },
		{ type: 'sing-box', label: 'sing-box', name: 'singbox_exit', iface: 'tun-sb0' },
		{ type: 'direct', label: 'Direct uplink', name: 'direct', iface: 'auto' },
		{ type: 'interface', label: 'Existing interface', name: 'direct_uplink', iface: 'eth0' }
	];

	let selected = $state(protocols[0]);
	let name = $state(protocols[0].name);
	let iface = $state(protocols[0].iface);
	let inlineConfig = $state('');
	let configName = $state('');
	let message = $state('');
	let saving = $state(false);

	function choose(protocol: (typeof protocols)[number]) {
		selected = protocol;
		name = protocol.name;
		iface = protocol.iface;
	}

	async function create() {
		saving = true;
		message = copy.saving;
		try {
			const response = await fetch('/api/upstreams', {
				method: 'POST',
				headers: { 'content-type': 'application/json', 'x-granger-request': '1' },
				body: JSON.stringify({
					name,
					upstream: { type: selected.type, interface: iface, enabled: true },
					inline_config: inlineConfig,
					config_name: configName
				})
			});
			if (!response.ok) throw new Error(await response.text());
			window.location.href = '/upstreams';
		} catch (error) {
			message = `${copy.error}: ${error instanceof Error ? error.message : String(error)}`;
		} finally {
			saving = false;
		}
	}

	async function upload(event: Event) {
		const file = (event.currentTarget as HTMLInputElement).files?.[0];
		if (!file) return;
		configName = file.name;
		inlineConfig = await file.text();
	}
</script>

<svelte:head>
	<title>{copy.title} · Granger</title>
</svelte:head>

<section class="max-w-4xl space-y-5">
	<Button href="/upstreams" variant="ghost"><ArrowLeftIcon />{copy.back}</Button>

	<div>
		<h1 class="font-display text-2xl font-bold text-white">{copy.title}</h1>
		<p class="mt-2 max-w-3xl text-sm text-muted-foreground">{copy.description}</p>
	</div>

	<Card.Root class="rounded-lg">
		<Card.Header>
			<Card.Title>{copy.suggested}</Card.Title>
		</Card.Header>
		<Card.Content class="space-y-5">
			<div class="flex flex-wrap gap-2">
				{#each protocols as protocol}
					<Button variant={selected.type === protocol.type ? 'default' : 'outline'} size="sm" onclick={() => choose(protocol)}>{protocol.label}</Button>
				{/each}
			</div>

			<div class="grid gap-3 md:grid-cols-2">
				<label class="grid gap-1.5 text-xs text-muted-foreground">
					{copy.name}
					<Input bind:value={name} placeholder="awg_default" />
				</label>
				<label class="grid gap-1.5 text-xs text-muted-foreground">
					{copy.iface}
					<Input bind:value={iface} placeholder="awg0" />
				</label>
			</div>

			<label class="grid gap-1.5 text-xs text-muted-foreground">
				{copy.config}
				<Textarea bind:value={inlineConfig} class="min-h-48 font-mono text-xs" placeholder="[Interface]&#10;PrivateKey = ..." />
			</label>

			<label class="grid gap-1.5 text-xs text-muted-foreground">
				{copy.upload}
				<Input type="file" onchange={upload} />
			</label>

			{#if message}<p class="text-sm text-muted-foreground">{message}</p>{/if}
			<Button disabled={saving} onclick={create}><UploadSimpleIcon />{copy.create}</Button>
		</Card.Content>
	</Card.Root>
</section>
