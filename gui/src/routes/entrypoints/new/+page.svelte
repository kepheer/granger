<script lang="ts">
	import type { ModuleDescriptor } from '$lib/modules';
	import {
		availableName,
		availableInterface,
		availablePort,
		availableSubnet,
		type ExistingConfig
	} from '$lib/config-suggestions';
	import { onMount } from 'svelte';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { Input } from '$lib/components/ui/input';
	import { Textarea } from '$lib/components/ui/textarea';
	import { ArrowLeftIcon, PlusIcon } from 'phosphor-svelte';

	const copy = {
		title: 'Add entrypoint',
		description: 'Choose how users connect to your server',
		back: 'Back',
		type: 'Protocol',
		name: 'Name',
		iface: 'Interface',
		subnet: 'Subnet',
		port: 'Port',
		create: 'Create entrypoint'
	} as const;

	let protocols = $state<
		Array<{
			type: string;
			label: string;
			name: string;
			iface: string;
			subnet: string;
			port: number;
		}>
	>([]);
	const initial = {
		type: '',
		label: '',
		name: '',
		iface: '',
		subnet: '',
		port: 0
	};

	let selected = $state(initial);
	let proxy = $derived(
		selected.type === 'xray' || selected.type === 'sing-box'
	);
	let name = $state(initial.name);
	let iface = $state(initial.iface);
	let subnet = $state(initial.subnet);
	let port = $state(initial.port);
	let message = $state('');
	let saving = $state(false);
	let inlineConfig = $state('');
	let service = $state('');
	let protocolStatusLoaded = $state(false);
	let existing = $state<ExistingConfig>({ outputs: {}, upstreams: {} });
	let installed = $state<string[]>([]);
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
				.filter((item: ModuleDescriptor) => item.entrypoint === true)
				.map((item: ModuleDescriptor) => ({
					type: item.id,
					label: item.label,
					name: item.id.replaceAll('-', '_'),
					iface: item.interface,
					subnet: item.subnet,
					port: item.port
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

			installed = (body.data ?? [])
				.filter((item: { installed: boolean }) => item.installed === true)
				.map((item: { name: string }) => item.name);
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
		name = availableName(protocol.name, Object.keys(existing.outputs));
		iface = availableInterface(protocol.iface, existing);
		subnet = availableSubnet(protocol.subnet, existing);
		port = availablePort(protocol.port, existing);
	}

	async function create() {
		if (saving || installed.includes(selected.type) === false) {
			return;
		}

		saving = true;
		message = '';
		try {
			const response = await fetch('/api/outputs', {
				method: 'POST',
				headers: {
					'content-type': 'application/json',
					'x-granger-request': '1'
				},
				body: JSON.stringify({
					name,
					output: {
						type: selected.type,
						interface: iface,
						subnet,
						listen_port: Number(port),
						service: inlineConfig.trim() !== '' ? service : '',
						clients: []
					},
					inline_config: inlineConfig,
					config_name:
						selected.type === 'xray' || selected.type === 'sing-box'
							? 'server.json'
							: 'server.conf'
				})
			});
			const body = await response.json();
			if (response.ok === false || body.ok !== true) {
				const failure = body.results?.find(
					(result: { OK: boolean }) => result.OK === false
				);
				throw new Error(
					body.error ?? failure?.Output ?? 'Entrypoint could not start'
				);
			}
			window.location.href = '/entrypoints';
		} catch (error) {
			message = error instanceof Error ? error.message : String(error);
		} finally {
			saving = false;
		}
	}
</script>

<svelte:head>
	<title>{copy.title} · Granger</title>
</svelte:head>

<section class="w-full min-w-0 space-y-5">
	<Button href="/entrypoints" variant="ghost"
		><ArrowLeftIcon />{copy.back}</Button
	>
	<div>
		<h1 class="font-display text-2xl font-bold text-white">{copy.title}</h1>
		<p class="mt-2 max-w-3xl text-sm text-muted-foreground">
			{copy.description}
		</p>
	</div>
	<Card.Root class="rounded-lg">
		<Card.Content class="space-y-5 pt-6">
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
				<label class="grid gap-1.5 text-xs text-muted-foreground"
					>{copy.name}<Input
						bind:value={name}
						placeholder={selected.name}
					/></label
				>
				{#if proxy === false}
					<label class="grid gap-1.5 text-xs text-muted-foreground"
						>{copy.iface}<Input
							bind:value={iface}
							placeholder={selected.iface}
						/></label
					>
					<label class="grid gap-1.5 text-xs text-muted-foreground"
						>{copy.subnet}<Input
							bind:value={subnet}
							placeholder={selected.subnet}
						/></label
					>
				{/if}
				<label class="grid gap-1.5 text-xs text-muted-foreground"
					>{copy.port}<Input
						bind:value={port}
						type="number"
						min="1"
						max="65535"
					/></label
				>
			</div>
			<p class="text-xs text-muted-foreground">
				Granger generates the server config and user profiles
			</p>
			{#if selected.type !== 'wireguard'}
				<details class="rounded-md border border-border p-3">
					<summary class="cursor-pointer text-sm"
						>Use an existing config</summary
					>
					<div class="mt-4 space-y-4">
						<label class="grid gap-1.5 text-xs text-muted-foreground"
							>Server config (optional)<Textarea
								bind:value={inlineConfig}
								class="min-h-48 font-mono text-xs"
							/></label
						>
						{#if inlineConfig.trim() !== ''}<label
								class="grid gap-1.5 text-xs text-muted-foreground"
								>Systemd service (optional)<Input bind:value={service} /></label
							>{/if}
					</div>
				</details>
			{/if}
			{#if protocolStatusLoaded && installed.includes(selected.type) === false}
				<p class="text-sm text-muted-foreground">
					Install the driver to continue <a class="underline" href="/upstreams"
						>View drivers</a
					>
				</p>
			{/if}
			{#if message}<p role="status" class="text-sm text-red-300">
					{message}
				</p>{/if}
			<Button
				disabled={saving ||
					protocolStatusLoaded === false ||
					installed.includes(selected.type) === false}
				onclick={create}><PlusIcon />{copy.create}</Button
			>
		</Card.Content>
	</Card.Root>
</section>
