<script lang="ts">
	import { onMount } from 'svelte';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { ArrowClockwiseIcon, CheckCircleIcon, PlusIcon, WarningCircleIcon } from 'phosphor-svelte';

	type ProtocolStatus = {
		name: string;
		display_name: string;
		installed: boolean;
		installable: boolean;
		state: string;
		summary: string;
		commands?: CommandResult[];
	};

	type ApiResponse<T> = {
		ok: boolean;
		error?: string;
		data?: T;
		results?: CommandResult[];
	};

	type CommandResult = {
		Title: string;
		Command: string;
		Output: string;
		OK: boolean;
		Status: string;
	};

	const copy = {
		title: 'Upstreams',
		description:
			'Choose where user traffic leaves the server',
		add: 'Add upstream',
		name: 'Name',
		type: 'Type',
		iface: 'Interface',
		config: 'Config',
		status: 'Status',
		enabled: 'Enabled',
		disabled: 'Disabled',
		pending: 'Pending',
		hint: 'Select a connection to view its settings',
		packages: 'Drivers',
		packagesHint:
			'Install a driver before adding a connection',
		installed: 'Installed',
		install: 'Install',
		manual: 'Manual installation',
		unavailable: 'Cannot reach the server — try Refresh',
		refresh: 'Refresh',
		installLog: 'Installation log'
	} as const;

	let protocols = $state<ProtocolStatus[]>([]);
	let protocolsLoading = $state(true);
	let upstreamsLoading = $state(true);
	let upstreamError = $state('');
	let protocolFailed = $state(false);

	let protocolMessage = $state('');
	let installing = $state('');
	let protocolResults = $state<CommandResult[]>([]);

	onMount(() => {
		void loadProtocols();
		void loadUpstreams();
	});

	async function loadProtocols() {
		try {
			const response = await fetch('/api/protocols');
			if (response.ok === false) {
				throw new Error(response.statusText);
			}
			const body = (await response.json()) as ApiResponse<ProtocolStatus[]>;
			if (body.ok !== true || Array.isArray(body.data) === false) {
				throw new Error(body.error ?? copy.unavailable);
			}
			protocols = body.data ?? [];
			protocolFailed = false;
			protocolMessage = '';
		} catch {
			protocolFailed = true;
			protocolMessage = copy.unavailable;
		} finally {
			protocolsLoading = false;
		}
	}

	async function loadUpstreams() {
		try {
			const response = await fetch('/api/config');
			if (response.ok === false) {
				throw new Error(response.statusText);
			}
			const body = await response.json();
			if (body.ok !== true || body.config === undefined) {
				throw new Error(body.error ?? 'Could not load upstreams');
			}
			const configured = body.config.upstreams ?? {};
			upstreams = Object.entries(configured).map(([name, value]) => {
				const item = value as Record<string, unknown>;
				return {
					name,
					type: String(item.type ?? ''),
					iface: String(item.interface ?? ''),
					config: String(item.config || (item.type === 'direct' ? 'System uplink' : item.type === 'interface' ? 'Existing network interface' : 'No configuration')),
					status: item.enabled === false ? 'disabled' : 'enabled'
				};
			});
			upstreamError = '';
		} catch (error) {
			upstreamError = error instanceof Error ? error.message : String(error);
		} finally {
			upstreamsLoading = false;
		}
	}

	async function installProtocol(name: string) {
		if (installing !== '') {
			return;
		}

		installing = name;
		protocolMessage = 'Installing driver — this may take a few minutes…';
		protocolFailed = false;
		protocolResults = [];
		try {
			const response = await fetch('/api/protocols/install', {
				method: 'POST',
				headers: { 'content-type': 'application/json', 'x-granger-request': '1' },
				body: JSON.stringify({ name })
			});
			const body = (await response.json()) as ApiResponse<ProtocolStatus>;
			if (body.data) {
				const status = body.data;
				protocols = protocols.map((item) => (item.name === name ? status : item));
			}
			protocolFailed = response.ok === false || body.ok !== true;
			protocolResults = body.results ?? body.data?.commands ?? [];
			protocolMessage = body.error ?? (body.data?.installed === true ? 'Driver installed' : body.data?.summary ?? '');
		} catch {
			protocolFailed = true;
			protocolMessage = copy.unavailable;
		} finally {
			installing = '';
		}
	}

	let upstreams = $state<Array<{ name: string; type: string; iface: string; config: string; status: string }>>([]);
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
		<Button href="/upstreams/new"><PlusIcon />{copy.add}</Button>
	</div>

	<Card.Root class="rounded-lg">
		<Card.Header class="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
			<div>
				<Card.Title>{copy.packages}</Card.Title>
				<Card.Description>{copy.packagesHint}</Card.Description>
			</div>
			<Button disabled={installing !== ''} variant="outline" onclick={loadProtocols}><ArrowClockwiseIcon />{copy.refresh}</Button>
		</Card.Header>
		<Card.Content class="space-y-3">
			{#if protocolMessage}
				<div role={protocolFailed ? 'alert' : 'status'} class="rounded-md border border-border bg-white/[0.03] px-3 py-2 text-sm {protocolFailed ? 'text-red-300' : 'text-muted-foreground'}">
					{protocolMessage}
				</div>
			{/if}
			{#if protocolResults.length}
				<div class="rounded-lg border border-border bg-black/25">
					<div class="border-b border-border px-4 py-3 text-sm font-medium text-white">
						{copy.installLog}
					</div>
					<div class="divide-y divide-border">
						{#each protocolResults as result}
							<div class="space-y-2 p-4">
								<div class="flex items-center justify-between gap-3">
									<div class="text-sm font-medium text-white">{result.Title}</div>
									<span class={result.OK ? 'text-xs text-emerald-300' : 'text-xs text-red-300'}>
										{result.Status}
									</span>
								</div>
								<pre class="max-h-72 overflow-auto whitespace-pre-wrap break-words rounded-md border border-border bg-black/35 p-3 text-xs text-white/75">$ {result.Command}
{result.Output}</pre>
							</div>
						{/each}
					</div>
				</div>
			{/if}
			<div class="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
				{#if protocolsLoading}<p role="status" class="text-sm text-muted-foreground">Loading drivers…</p>{/if}
			{#each protocols as protocol}
					<div class="rounded-lg border border-border bg-card/60 p-4">
						<div class="flex items-start justify-between gap-3">
							<div>
								<div class="font-medium text-white">{protocol.display_name}</div>
								{#if protocol.installed === false && protocol.summary !== 'Not installed'}<div class="mt-1 text-xs text-muted-foreground">{protocol.summary}</div>{/if}
							</div>
							<span
								class="inline-flex items-center gap-1 rounded-md border border-border px-2 py-1 text-xs {protocol.installed
									? 'text-emerald-300'
									: protocol.installable
										? 'text-amber-300'
										: 'text-muted-foreground'}"
							>
								{#if protocol.installed}
									<CheckCircleIcon class="size-3.5" />{copy.installed}
								{:else}
									<WarningCircleIcon class="size-3.5" />{protocol.installable
										? protocol.state === 'failed' ? 'Installation failed' : 'Not installed'
										: copy.manual}
								{/if}
							</span>
						</div>
						<div class="mt-4">
							<Button
								size="sm"
								variant={protocol.installable && protocol.installed === false ? 'default' : 'outline'}
								disabled={protocol.installable === false || protocol.installed || installing !== ''}
								onclick={() => installProtocol(protocol.name)}
							>
								{installing === protocol.name ? 'Installing…' : protocol.state === 'failed' ? 'Retry installation' : copy.install}
							</Button>
						</div>
					</div>
				{/each}
			</div>
		</Card.Content>
	</Card.Root>

	<Card.Root class="rounded-lg">
		<Card.Header>
			<Card.Title>{copy.title}</Card.Title>
			<Card.Description>{copy.hint}</Card.Description>
		</Card.Header>
		<Card.Content>
			{#if upstreamError}<p role="alert" class="mb-3 text-sm text-red-300">{upstreamError}</p>{/if}
			{#if upstreamsLoading}<p role="status" class="mb-3 text-sm text-muted-foreground">Loading upstreams…</p>{/if}
			{#if upstreamsLoading === false && upstreamError === '' && upstreams.length === 0}<p class="mb-3 text-sm text-muted-foreground">No upstreams yet — add a connection to route traffic</p>{/if}
			{#if upstreams.length > 0}
			<div class="overflow-x-auto rounded-lg border border-border">
				<div class="grid min-w-[760px] grid-cols-[1.1fr_.9fr_.9fr_1.8fr_.8fr] bg-white/[0.035] px-4 py-2 text-xs font-medium text-muted-foreground">
					<div>{copy.name}</div>
					<div>{copy.type}</div>
					<div>{copy.iface}</div>
					<div>{copy.config}</div>
					<div>{copy.status}</div>
				</div>
				{#each upstreams as item}
					<a
						href={`/upstreams/${item.name}`}
						class="grid min-w-[760px] grid-cols-[1.1fr_.9fr_.9fr_1.8fr_.8fr] items-center border-t border-border px-4 py-3 text-sm transition hover:bg-white/[0.04]"
					>
						<div class="font-medium text-white">{item.name}</div>
						<div class="text-muted-foreground">{item.type}</div>
						<div class="font-mono text-xs text-white/80">{item.iface}</div>
						<div class="truncate font-mono text-xs text-muted-foreground">{item.config}</div>
						<div>
							<span
								class="rounded-full border border-border px-2 py-1 text-xs {item.status ===
								'enabled'
									? 'text-emerald-300'
									: item.status === 'disabled'
										? 'text-red-300'
										: 'text-amber-300'}"
							>
								{item.status === 'enabled'
									? copy.enabled
									: item.status === 'disabled'
										? copy.disabled
										: copy.pending}
							</span>
						</div>
					</a>
				{/each}
			</div>
			{/if}
		</Card.Content>
	</Card.Root>
</section>
