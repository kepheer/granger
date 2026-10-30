<script lang="ts">
	import type { ModuleDescriptor } from '$lib/modules';
	import {
		availableName,
		availableInterface,
		type ExistingConfig
	} from '$lib/config-suggestions';
	import { onMount } from 'svelte';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { Input } from '$lib/components/ui/input';
	import { Textarea } from '$lib/components/ui/textarea';
	import { ArrowLeftIcon, UploadSimpleIcon } from 'phosphor-svelte';

	const copy = {
		title: 'Add upstream',
		description: 'Choose a protocol and add your connection settings',
		back: 'Back',
		type: 'Protocol',
		name: 'Name',
		iface: 'Interface',
		config: 'Connection config',
		upload: 'Upload config',
		create: 'Create upstream',
		suggested: 'Connection settings',
		saving: 'Creating upstream…',
		error: 'Could not create upstream'
	} as const;

	let protocols = $state<
		Array<{ type: string; label: string; name: string; iface: string }>
	>([]);
	const initial = { type: '', label: '', name: '', iface: '' };

	let selected = $state(initial);
	let name = $state(initial.name);
	let iface = $state(initial.iface);
	let inlineConfig = $state('');
	let configName = $state('');
	let message = $state('');
	let saving = $state(false);
	let protocolStatusLoaded = $state(false);
	let existing = $state<ExistingConfig>({ outputs: {}, upstreams: {} });
	let installed = $state<string[]>(['direct', 'interface']);
	let requiresConfig = $derived(
		selected.type !== 'direct' && selected.type !== 'interface'
	);
	let configPlaceholder = $derived(
		selected.type === 'xray' || selected.type === 'sing-box'
			? '{\n  \"outbounds\": []\n}'
			: selected.type === 'openvpn'
				? 'client\ndev tun\nremote vpn.example.org 1194'
				: selected.type === 'snx-rs'
					? 'Paste your SNX-RS configuration here'
					: '[Interface]\nPrivateKey = ...\n\n[Peer]\nPublicKey = ...'
	);

	onMount(async () => {
		try {
			const [response, configResponse, modulesResponse] = await Promise.all([
				fetch('/api/protocols'),
				fetch('/api/config'),
				fetch('/api/modules')
			]);
			const moduleBody = await modulesResponse.json();
			if (modulesResponse.ok === false || moduleBody.ok !== true) {
				throw new Error('Could not load connection types');
			}
			protocols = moduleBody.data
				.filter((item: ModuleDescriptor) => item.upstream === true)
				.map((item: ModuleDescriptor) => ({
					type: item.id,
					label: item.label,
					name: item.id.replaceAll('-', '_'),
					iface: item.upstream_interface
				}));
			const preferred =
				protocols.find((item) => item.type === 'wireguard') ?? protocols[0];
			if (preferred === undefined) {
				throw new Error('No connection types available');
			}
			choose(preferred);
			const configBody = await configResponse.json();
			if (configResponse.ok === false || configBody.ok !== true) {
				throw new Error(
					configBody.error ?? 'Could not load existing connections'
				);
			}
			existing = configBody.config;
			const body = await response.json();
			if (response.ok === false || body.ok !== true) {
				throw new Error(body.error ?? 'Could not load protocol status');
			}

			installed = [
				'direct',
				'interface',
				...(body.data ?? [])
					.filter((item: { installed: boolean }) => item.installed === true)
					.map((item: { name: string }) => item.name)
			];
			protocolStatusLoaded = true;
			if (name === selected.name) {
				choose(selected);
			}
		} catch (error) {
			message = error instanceof Error ? error.message : String(error);
		}
	});

	function choose(protocol: (typeof protocols)[number]) {
		selected = protocol;
		name = availableName(protocol.name, Object.keys(existing.upstreams));
		iface =
			protocol.type === 'direct' || protocol.type === 'interface'
				? protocol.iface
				: availableInterface(protocol.iface, existing);
	}

	async function create() {
		if (saving || installed.includes(selected.type) === false) {
			return;
		}

		saving = true;
		message = copy.saving;
		try {
			const response = await fetch('/api/upstreams', {
				method: 'POST',
				headers: {
					'content-type': 'application/json',
					'x-granger-request': '1'
				},
				body: JSON.stringify({
					name,
					upstream: { type: selected.type, interface: iface, enabled: true },
					inline_config: inlineConfig,
					config_name: configName
				})
			});
			const body = await response.json();
			if (response.ok === false || body.ok !== true) {
				const failure = body.results?.find(
					(result: { OK: boolean }) => result.OK === false
				);
				throw new Error(
					body.error ?? failure?.Output ?? 'Upstream could not start'
				);
			}
			window.location.href = '/upstreams';
		} catch (error) {
			message = `${copy.error}: ${error instanceof Error ? error.message : String(error)}`;
		} finally {
			saving = false;
		}
	}

	async function upload(event: Event) {
		const file = (event.currentTarget as HTMLInputElement).files?.[0];
		if (file === undefined || file === null) {
			return;
		}
		configName = file.name;
		inlineConfig = await file.text();
	}
</script>

<svelte:head>
	<title>{copy.title} · Granger</title>
</svelte:head>

<section class="w-full min-w-0 space-y-5">
	<Button href="/upstreams" variant="ghost"><ArrowLeftIcon />{copy.back}</Button
	>

	<div>
		<h1 class="font-display text-2xl font-bold text-white">{copy.title}</h1>
		<p class="mt-2 max-w-3xl text-sm text-muted-foreground">
			{copy.description}
		</p>
	</div>

	<Card.Root class="rounded-lg">
		<Card.Header>
			<Card.Title>{copy.suggested}</Card.Title>
		</Card.Header>
		<Card.Content class="space-y-5">
			<div class="flex flex-wrap gap-2">
				{#each protocols as protocol}
					<Button
						aria-pressed={selected.type === protocol.type}
						variant={selected.type === protocol.type ? 'default' : 'outline'}
						size="sm"
						onclick={() => choose(protocol)}>{protocol.label}</Button
					>
				{/each}
			</div>

			<div class="grid gap-3 md:grid-cols-2">
				<label class="grid gap-1.5 text-xs text-muted-foreground">
					{copy.name}
					<Input bind:value={name} placeholder={selected.name} />
				</label>
				<label class="grid gap-1.5 text-xs text-muted-foreground">
					{copy.iface}
					<Input bind:value={iface} placeholder={selected.iface} />
				</label>
			</div>

			{#if requiresConfig}
				<label class="grid gap-1.5 text-xs text-muted-foreground">
					{copy.config}
					<Textarea
						bind:value={inlineConfig}
						class="min-h-36 font-mono text-xs"
						placeholder={configPlaceholder}
					/>
				</label>

				<label class="grid gap-1.5 text-xs text-muted-foreground">
					{copy.upload}
					<Input type="file" onchange={upload} />
				</label>
			{/if}
			{#if protocolStatusLoaded && installed.includes(selected.type) === false}
				<p class="text-sm text-muted-foreground">
					Install the driver to continue <a class="underline" href="/upstreams"
						>View drivers</a
					>
				</p>
			{/if}
			{#if message}<p role="status" class="text-sm text-muted-foreground">
					{message}
				</p>{/if}
			<Button
				disabled={saving ||
					protocolStatusLoaded === false ||
					installed.includes(selected.type) === false}
				onclick={create}><UploadSimpleIcon />{copy.create}</Button
			>
		</Card.Content>
	</Card.Root>
</section>
