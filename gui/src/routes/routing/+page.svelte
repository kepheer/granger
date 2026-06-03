<script lang="ts">
	import { onMount } from 'svelte';
	import {
		Background,
		BackgroundVariant,
		Controls,
		MiniMap,
		SvelteFlow,
		type Edge,
		type Node
	} from '@xyflow/svelte';
	import '@xyflow/svelte/dist/base.css';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { Input } from '$lib/components/ui/input';
	import { Textarea } from '$lib/components/ui/textarea';
	import { FloppyDiskIcon, PlusIcon, WarningCircleIcon } from 'phosphor-svelte';

	type GraphConfig = {
		server?: Record<string, unknown>;
		users: Record<string, unknown>;
		outputs: Record<string, unknown>;
		upstreams: Record<string, unknown>;
		rules: Array<Record<string, unknown>>;
	};

	type GraphPayload = {
		nodes: Array<{ id: string; type: string; label: string; position?: { x: number; y: number }; data?: unknown }>;
		edges: Array<{ id: string; source: string; target: string; label?: string }>;
		config: GraphConfig;
	};

	const copy = {
		title: 'Routing',
		description:
			'Build routing as a graph: entrypoints feed rules, rules choose upstreams, and fallback/DNS nodes explain what happens when a tunnel is down.',
		addNode: 'Add node',
		save: 'Save graph',
		dryRun: 'Dry run',
		selected: 'Selected node',
		noSelection: 'Select a node in the graph or from the list.',
		label: 'Label',
		type: 'Type',
		domains: 'Domains',
		cidrs: 'CIDRs',
		dns: 'DNS resolvers',
		via: 'Via / target',
		fallback: 'Fallback upstream',
		defaultUpstream: 'Default upstream',
		blockFallback: 'Block traffic when fallback is unavailable',
		statusReady: 'Graph ready',
		statusSaved: 'Graph saved',
		statusDryRun: 'Dry-run finished',
		statusError: 'Graph action failed'
	} as const;

	let nodes = $state<Node[]>([]);
	let edges = $state<Edge[]>([]);
	let selectedId = $state('');
	let status = $state('');
	let graphConfig = $state<GraphConfig>({
		users: {},
		outputs: {},
		upstreams: {},
		rules: []
	});

	const nodeTypes = [
		{ type: 'entrypoint', label: 'Entrypoint' },
		{ type: 'user', label: 'User group' },
		{ type: 'rule', label: 'Rule / bypass' },
		{ type: 'upstream', label: 'Upstream' },
		{ type: 'fallback', label: 'Fallback' },
		{ type: 'dns', label: 'DNS resolver' }
	];

	const fallbackGraph: GraphPayload = {
		config: {
			users: {},
			outputs: {},
			upstreams: {
				default_awg: { type: 'amneziawg', interface: 'awg0', enabled: true },
				direct: { type: 'direct', interface: 'auto', enabled: true }
			},
			rules: [{ name: 'default', default: true, via: 'default_awg' }]
		},
		nodes: [
			{ id: 'output:home_wg', type: 'entrypoint', label: 'home_wg', position: { x: 40, y: 120 } },
			{ id: 'rule:default', type: 'rule', label: 'default', position: { x: 360, y: 120 } },
			{ id: 'upstream:default_awg', type: 'upstream', label: 'default_awg', position: { x: 700, y: 80 } },
			{ id: 'upstream:direct', type: 'fallback', label: 'direct fallback', position: { x: 700, y: 240 } }
		],
		edges: [
			{ id: 'output:home_wg->rule:default', source: 'output:home_wg', target: 'rule:default', label: 'traffic' },
			{ id: 'rule:default->upstream:default_awg', source: 'rule:default', target: 'upstream:default_awg', label: 'via' }
		]
	};

	let selectedNode = $derived(nodes.find((node) => node.id === selectedId));

	onMount(async () => {
		try {
			const response = await fetch('/api/routing/graph');
			if (!response.ok) throw new Error(await response.text());
			const body = await response.json();
			loadGraph(body.data ?? fallbackGraph);
		} catch {
			loadGraph(fallbackGraph);
		}
		status = copy.statusReady;
	});

	function loadGraph(graph: GraphPayload) {
		graphConfig = graph.config;
		nodes =
			graph.nodes.map((node) => ({
				id: node.id,
				type: 'default',
				position: node.position ?? { x: 0, y: 0 },
				data: {
					label: node.label,
					kind: node.type,
					raw: node.data
				},
				style: nodeStyle(node.type),
				class: nodeClass(node.type)
			}));
		edges =
			graph.edges.map((edge) => ({
				id: edge.id,
				source: edge.source,
				target: edge.target,
				label: edge.label,
				animated: edge.label === 'fallback'
			}));
	}

	function nodeClass(type: string) {
		return `gr-node gr-node-${type}`;
	}

	function nodeStyle(type: string) {
		const color =
			type === 'upstream'
				? 'rgba(116,242,161,.42)'
				: type === 'fallback'
					? 'rgba(255,209,102,.42)'
					: type === 'rule'
						? 'rgba(140,180,255,.42)'
						: 'rgba(255,255,255,.24)';
		return `background: #101114; color: white; border: 1px solid ${color}; border-radius: 8px; font-weight: 650; min-width: 150px; box-shadow: 0 12px 32px rgba(0,0,0,.28);`;
	}

	function addNode(type: string) {
		const next = `${type}:${Date.now().toString(36)}`;
		nodes = [
			...nodes,
			{
				id: next,
				type: 'default',
				position: { x: 120 + nodes.length * 36, y: 120 + nodes.length * 28 },
				data: { label: type, kind: type, raw: {} },
				style: nodeStyle(type),
				class: nodeClass(type)
			}
		];
		selectedId = next;
	}

	function selectNode(id: string) {
		selectedId = id;
	}

	function updateSelectedLabel(value: string) {
		nodes = nodes.map((node) =>
			node.id === selectedId ? { ...node, data: { ...node.data, label: value } } : node
		);
	}

	function selectedRawValue(key: string) {
		const raw = selectedNode?.data?.raw;
		if (!raw || typeof raw !== 'object') return '';
		const value = (raw as Record<string, unknown>)[key];
		if (Array.isArray(value)) return value.join('\n');
		return typeof value === 'string' || typeof value === 'number' ? String(value) : '';
	}

	function updateSelectedRaw(key: string, value: string, list = false) {
		nodes = nodes.map((node) => {
			if (node.id !== selectedId) return node;
			const raw =
				node.data?.raw && typeof node.data.raw === 'object'
					? { ...(node.data.raw as Record<string, unknown>) }
					: {};
			raw[key] = list
				? value
						.split('\n')
						.map((line) => line.trim())
						.filter(Boolean)
				: value;
			return { ...node, data: { ...node.data, raw } };
		});
	}

	function selectedBool(key: string) {
		const raw = selectedNode?.data?.raw;
		return Boolean(raw && typeof raw === 'object' && (raw as Record<string, unknown>)[key]);
	}

	function updateSelectedBool(key: string, value: boolean) {
		nodes = nodes.map((node) => {
			if (node.id !== selectedId) return node;
			const raw =
				node.data?.raw && typeof node.data.raw === 'object'
					? { ...(node.data.raw as Record<string, unknown>) }
					: {};
			raw[key] = value;
			return { ...node, data: { ...node.data, raw } };
		});
	}

	async function saveGraph() {
		try {
			const payload = toPayload();
			const response = await fetch('/api/routing/graph', {
				method: 'POST',
				headers: { 'content-type': 'application/json', 'x-granger-request': '1' },
				body: JSON.stringify(payload)
			});
			if (!response.ok) throw new Error(await response.text());
			status = copy.statusSaved;
		} catch (error) {
			status = `${copy.statusError}: ${error instanceof Error ? error.message : String(error)}`;
		}
	}

	async function dryRun() {
		try {
			const response = await fetch('/api/routing/dry-run', { method: 'POST', headers: { 'x-granger-request': '1' } });
			if (!response.ok) throw new Error(await response.text());
			status = copy.statusDryRun;
		} catch (error) {
			status = `${copy.statusError}: ${error instanceof Error ? error.message : String(error)}`;
		}
	}

	function toPayload(): GraphPayload {
		return {
			config: graphConfig,
			nodes: nodes.map((node) => ({
				id: node.id,
				type: String(node.data?.kind ?? 'rule'),
				label: String(node.data?.label ?? node.id),
				position: node.position,
				data: node.data?.raw
			})),
			edges: edges.map((edge) => ({
				id: edge.id,
				source: edge.source,
				target: edge.target,
				label: typeof edge.label === 'string' ? edge.label : undefined
			}))
		};
	}
</script>

<svelte:head>
	<title>{copy.title} · Granger</title>
</svelte:head>

<section class="space-y-5">
	<div class="flex flex-col justify-between gap-4 xl:flex-row xl:items-end">
		<div>
			<h1 class="font-display text-2xl font-bold text-white">{copy.title}</h1>
			<p class="mt-2 max-w-4xl text-sm text-muted-foreground">{copy.description}</p>
		</div>
		<div class="flex flex-wrap gap-2">
			<Button variant="outline" onclick={dryRun}><WarningCircleIcon />{copy.dryRun}</Button>
			<Button onclick={saveGraph}><FloppyDiskIcon />{copy.save}</Button>
		</div>
	</div>

	<div class="grid gap-4 xl:grid-cols-[1fr_360px]">
		<Card.Root class="overflow-hidden rounded-lg">
			<Card.Content class="h-[640px] p-0">
				<SvelteFlow
					bind:nodes
					bind:edges
					fitView
					proOptions={{ hideAttribution: true }}
					onnodeclick={(event) => selectNode(event.node.id)}
				>
					<Controls />
					<MiniMap pannable zoomable />
					<Background variant={BackgroundVariant.Dots} />
				</SvelteFlow>
			</Card.Content>
		</Card.Root>

		<div class="space-y-4">
			<Card.Root class="rounded-lg">
				<Card.Header>
					<Card.Title>{copy.addNode}</Card.Title>
				</Card.Header>
				<Card.Content class="flex flex-wrap gap-2">
					{#each nodeTypes as item}
						<Button variant="outline" size="sm" onclick={() => addNode(item.type)}>
							<PlusIcon />{item.label}
						</Button>
					{/each}
				</Card.Content>
			</Card.Root>

			<Card.Root class="rounded-lg">
				<Card.Header>
					<Card.Title>{copy.selected}</Card.Title>
					<Card.Description>{status}</Card.Description>
				</Card.Header>
				<Card.Content class="space-y-4">
					{#if selectedNode}
						<label class="grid gap-1.5 text-xs text-muted-foreground">
							{copy.label}
							<Input value={String(selectedNode.data?.label ?? '')} oninput={(event) => updateSelectedLabel(event.currentTarget.value)} />
						</label>
						<label class="grid gap-1.5 text-xs text-muted-foreground">
							{copy.fallback}
							<Input
								value={selectedRawValue('domain_fallback_via') || selectedRawValue('fallback_when_down')}
								placeholder="direct"
								oninput={(event) =>
									updateSelectedRaw(
										String(selectedNode.data?.kind) === 'rule'
											? 'domain_fallback_via'
											: 'fallback_when_down',
										event.currentTarget.value
									)}
							/>
						</label>
						{#if String(selectedNode.data?.kind) === 'upstream'}
							<Button variant={selectedBool('default') ? 'default' : 'outline'} size="sm" onclick={() => updateSelectedBool('default', !selectedBool('default'))}>
								{copy.defaultUpstream}
							</Button>
							<Button variant={selectedBool('block_fallback') ? 'destructive' : 'outline'} size="sm" onclick={() => updateSelectedBool('block_fallback', !selectedBool('block_fallback'))}>
								{copy.blockFallback}
							</Button>
							<label class="grid gap-1.5 text-xs text-muted-foreground">
								{copy.dns}
								<Textarea
									value={selectedRawValue('dns')}
									placeholder="1.1.1.1&#10;9.9.9.9"
									oninput={(event) => updateSelectedRaw('dns', event.currentTarget.value, true)}
								/>
							</label>
						{/if}
						<label class="grid gap-1.5 text-xs text-muted-foreground">
							{copy.type}
							<Input value={String(selectedNode.data?.kind ?? '')} readonly />
						</label>
						{#if String(selectedNode.data?.kind) === 'rule'}
							<label class="grid gap-1.5 text-xs text-muted-foreground">
								{copy.domains}
								<Textarea value={selectedRawValue('domains')} placeholder="example.org&#10;*.media.example" oninput={(event) => updateSelectedRaw('domains', event.currentTarget.value, true)} />
							</label>
							<label class="grid gap-1.5 text-xs text-muted-foreground">
								{copy.cidrs}
								<Textarea value={selectedRawValue('cidrs')} placeholder="10.44.0.0/16" oninput={(event) => updateSelectedRaw('cidrs', event.currentTarget.value, true)} />
							</label>
							<label class="grid gap-1.5 text-xs text-muted-foreground">
								{copy.via}
								<Input value={selectedRawValue('via')} placeholder="default_awg" oninput={(event) => updateSelectedRaw('via', event.currentTarget.value)} />
							</label>
						{/if}
					{:else}
						<p class="text-sm text-muted-foreground">{copy.noSelection}</p>
					{/if}
				</Card.Content>
			</Card.Root>

			<Card.Root class="rounded-lg">
				<Card.Header>
					<Card.Title>Nodes</Card.Title>
				</Card.Header>
				<Card.Content class="space-y-2">
					{#each nodes as node}
						<button
							class="flex w-full items-center justify-between rounded-md border border-border bg-white/[0.03] px-3 py-2 text-left text-sm transition hover:bg-white/[0.06] {selectedId === node.id ? 'text-white' : 'text-muted-foreground'}"
							onclick={() => selectNode(node.id)}
						>
							<span>{node.data?.label}</span>
							<span class="font-mono text-xs">{node.data?.kind}</span>
						</button>
					{/each}
				</Card.Content>
			</Card.Root>
		</div>
	</div>
</section>

<style>
	:global(.svelte-flow) {
		background: #08090b;
		--xy-background-color: #08090b;
		--xy-edge-stroke: rgba(255, 255, 255, 0.35);
		--xy-edge-stroke-selected: rgba(255, 255, 255, 0.72);
		--xy-controls-button-background-color: #15161a;
		--xy-controls-button-color: rgba(255, 255, 255, 0.72);
		--xy-controls-button-border-color: rgba(255, 255, 255, 0.1);
		--xy-minimap-background-color: #111216;
	}

	:global(.svelte-flow__pane) {
		background: #08090b;
	}

	:global(.svelte-flow__node-default.gr-node) {
		border: 1px solid var(--border);
		border-radius: 8px;
		background: #101114;
		color: white;
		font-family: var(--font-sans);
		font-size: 12px;
		font-weight: 600;
		padding: 10px 12px;
		min-width: 150px;
		box-shadow: 0 12px 32px rgba(0, 0, 0, 0.28);
	}

	:global(.gr-node-entrypoint) {
		border-color: rgba(255, 255, 255, 0.28);
	}

	:global(.gr-node-rule) {
		border-color: rgba(140, 180, 255, 0.38);
	}

	:global(.gr-node-upstream) {
		border-color: rgba(116, 242, 161, 0.34);
	}

	:global(.gr-node-fallback) {
		border-color: rgba(255, 209, 102, 0.34);
	}

	:global(.gr-node-dns) {
		border-color: rgba(180, 180, 255, 0.3);
	}
</style>
