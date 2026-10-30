<script lang="ts">
	import { onMount } from 'svelte';
	import {
		Background,
		BackgroundVariant,
		Controls,
		MiniMap,
		SvelteFlow,
		type Connection,
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
		version: string;
		nodes: Array<{
			id: string;
			type: string;
			label: string;
			position?: { x: number; y: number };
			data?: unknown;
		}>;
		edges: Array<{
			id: string;
			source: string;
			target: string;
			label?: string;
		}>;
		config: GraphConfig;
	};

	const copy = {
		title: 'Routing',
		description:
			'Choose where user traffic goes',
		addNode: 'Add rule',
		save: 'Save routing',
		dryRun: 'Preview changes',
		selected: 'Selected item',
		noSelection: 'Select an item in the graph or list',
		label: 'Name',
		type: 'Type',
		domains: 'Domains',
		cidrs: 'IP ranges (CIDR)',
		dns: 'DNS servers',
		via: 'Send through',
		fallback: 'Backup connection',
		defaultUpstream: 'Default route',
		blockFallback: 'Block traffic if the backup is unavailable',
		statusReady: 'Routing loaded',
		statusSaved: 'Routing saved',
		statusDryRun: 'Preview ready',
		statusError: 'Could not update routing'
	} as const;

	let nodes = $state<Node[]>([]);
	let edges = $state<Edge[]>([]);
	let selectedId = $state('');
	let status = $state('Loading routing…');
	let graphVersion = $state('');
	let graphLoaded = $state(false);
	let actionPending = $state(false);
	let newRuleName = $state('');
	let graphConfig = $state<GraphConfig>({
		users: {},
		outputs: {},
		upstreams: {},
		rules: []
	});

	let selectedNode = $derived(nodes.find((node) => node.id === selectedId));

	onMount(async () => {
		try {
			const response = await fetch('/api/routing/graph');
			const body = await response.json();

			if (
				response.ok === false ||
				body.ok !== true ||
				body.data?.config === undefined ||
				body.data?.config === null ||
				typeof body.data?.config?.server !== 'object' ||
				body.data?.config?.server === null
			) {
				throw new Error(body.error ?? 'Could not load routing configuration');
			}

			loadGraph(body.data);
			graphLoaded = true;
			status = copy.statusReady;
		} catch (error) {
			graphLoaded = false;
			status = `${copy.statusError}: ${error instanceof Error ? error.message : String(error)}`;
		}
	});

	function loadGraph(graph: GraphPayload) {
		graphConfig = graph.config;
		graphVersion = graph.version;
		const kinds = ['entrypoint', 'user', 'rule', 'upstream', 'fallback', 'dns'];
		const items = [...(graph.nodes ?? [])].sort((left, right) => {
			const kindOrder = kinds.indexOf(left.type) - kinds.indexOf(right.type);
			return kindOrder || left.label.localeCompare(right.label);
		});
		const positions = new Map<string, { x: number; y: number }>();
		let offset = 0;

		for (const kind of kinds) {
			const group = items.filter((item) => item.type === kind);
			group.forEach((item, index) => {
				positions.set(item.id, { x: (index % 3) * 230, y: offset + Math.floor(index / 3) * 100 });
			});
			if (group.length > 0) {
				offset += Math.ceil(group.length / 3) * 100 + 60;
			}
		}

		nodes = items.map((node) => ({
			id: node.id,
			type: 'default',
			position: positions.get(node.id) ?? { x: 0, y: offset },
			data: { label: node.label, kind: node.type, raw: node.data },
			style: nodeStyle(node.type),
			class: nodeClass(node.type),
			deletable: node.type === 'rule'
		}));
		const nodeIDs = new Set(nodes.map((node) => node.id));
		const links = [...(graph.edges ?? [])];

		for (const node of items.filter((item) => item.type === 'user')) {
			const raw = (node.data ?? {}) as Record<string, unknown>;
			const source = `output:${String(raw.output ?? '')}`;
			if (nodeIDs.has(source)) {
				links.push({ id: `profile:${node.id}`, source, target: node.id, label: 'profile' });
			}
			const target = `upstream:${String(raw.default_upstream ?? '')}`;
			if (nodeIDs.has(target)) {
				links.push({ id: `default:${node.id}`, source: node.id, target, label: 'default' });
			}
		}

		edges = links.map((edge) => ({
			id: edge.id,
			source: edge.source,
			target: edge.target,
			label: edge.label,
			animated: edge.label === 'fallback',
			deletable: edge.source.startsWith('rule:')
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

	function addRule() {
		if (graphLoaded === false || actionPending) {
			return;
		}

		const name = newRuleName.trim();
		if (/^[A-Za-z0-9_.-]+$/.test(name) === false) {
			status =
				'Use letters, numbers, dots, underscores or hyphens in the rule name';
			return;
		}

		const next = `rule:${name}`;
		if (nodes.some((node) => node.id === next)) {
			status = 'This rule name is already in use';
			return;
		}

		nodes = [
			...nodes,
			{
				id: next,
				type: 'default',
				position: { x: 120 + nodes.length * 36, y: 120 + nodes.length * 28 },
				data: {
					label: name,
					kind: 'rule',
					raw: { name, via: '', domains: [], cidrs: [] }
				},
				style: nodeStyle('rule'),
				class: nodeClass('rule')
			}
		];
		selectedId = next;
		newRuleName = '';
		status =
			'Choose destinations and an upstream for the new rule, then save routing';
	}

	function beforeConnect(connection: Connection) {
		if (graphLoaded === false || actionPending) {
			return false;
		}

		const source = nodes.find((node) => node.id === connection.source);
		const target = nodes.find((node) => node.id === connection.target);
		if (
			source?.data?.kind !== 'rule' ||
			target?.data?.kind !== 'upstream' ||
			target.id.startsWith('upstream:') === false
		) {
			status =
				'Connect a rule to an upstream and set a backup if needed';
			return false;
		}

		return connection;
	}

	function connectRule(connection: Connection) {
		if (beforeConnect(connection) === false) {
			return;
		}

		const upstream = connection.target.slice('upstream:'.length);
		edges = edges.filter(
			(edge) => edge.source !== connection.source || edge.label === 'fallback'
		);
		edges = [
			...edges,
			{
				id: `${connection.source}->${connection.target}`,
				source: connection.source,
				target: connection.target,
				sourceHandle: connection.sourceHandle,
				targetHandle: connection.targetHandle,
				label: 'via'
			}
		];
		nodes = nodes.map((node) => {
			if (node.id !== connection.source) {
				return node;
			}

			const raw =
				node.data?.raw && typeof node.data.raw === 'object'
					? { ...(node.data.raw as Record<string, unknown>) }
					: {};
			return {
				...node,
				data: { ...node.data, raw: { ...raw, via: upstream } }
			};
		});
		selectedId = connection.source;
		status = 'Save routing to apply the new connection';
	}

	function selectNode(id: string) {
		selectedId = id;
	}

	function updateSelectedLabel(value: string) {
		if (selectedNode?.data?.kind !== 'user') {
			return;
		}

		nodes = nodes.map((node) =>
			node.id === selectedId
				? { ...node, data: { ...node.data, label: value } }
				: node
		);
	}

	function selectedRawValue(key: string) {
		const raw = selectedNode?.data?.raw;
		if (raw === null || raw === undefined || typeof raw !== 'object') {
			return '';
		}
		const value = (raw as Record<string, unknown>)[key];
		if (Array.isArray(value)) {
			return value.join('\n');
		}
		return typeof value === 'string' || typeof value === 'number'
			? String(value)
			: '';
	}

	function updateSelectedRaw(key: string, value: string, list = false) {
		if (
			selectedNode?.data?.kind === 'rule' &&
			(key === 'via' || key === 'domain_fallback_via')
		) {
			const label = key === 'via' ? 'via' : 'fallback';
			edges = edges.filter(
				(edge) => edge.source !== selectedId || (edge.label ?? 'via') !== label
			);
			if (value.trim() !== '') {
				edges = [
					...edges,
					{
						id: `${selectedId}-${label}-${value}`,
						source: selectedId,
						target: `upstream:${value}`,
						label
					}
				];
			}
		}
		nodes = nodes.map((node) => {
			if (node.id !== selectedId) {
				return node;
			}
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
		return Boolean(
			raw && typeof raw === 'object' && (raw as Record<string, unknown>)[key]
		);
	}

	function updateSelectedBool(key: string, value: boolean) {
		nodes = nodes.map((node) => {
			if (node.id !== selectedId) {
				return node;
			}
			const raw =
				node.data?.raw && typeof node.data.raw === 'object'
					? { ...(node.data.raw as Record<string, unknown>) }
					: {};
			raw[key] = value;
			return { ...node, data: { ...node.data, raw } };
		});
	}

	async function saveGraph() {
		if (graphLoaded === false || actionPending) {
			return;
		}

		actionPending = true;
		try {
			const payload = toPayload();
			const response = await fetch('/api/routing/graph', {
				method: 'POST',
				headers: {
					'content-type': 'application/json',
					'x-granger-request': '1'
				},
				body: JSON.stringify(payload)
			});
			const body = await response.json();
			if (response.ok === false || body.ok !== true) {
				const failure = body.results?.find(
					(result: { OK: boolean }) => result.OK === false
				);
				throw new Error(
					body.error ?? failure?.Output ?? 'Could not apply routing'
				);
			}
			graphVersion = body.data.version;
			graphConfig = body.config;
			status = copy.statusSaved;
		} catch (error) {
			status = `${copy.statusError}: ${error instanceof Error ? error.message : String(error)}`;
		} finally {
			actionPending = false;
		}
	}

	async function dryRun() {
		if (graphLoaded === false || actionPending) {
			return;
		}

		actionPending = true;
		try {
			const response = await fetch('/api/routing/dry-run', {
				method: 'POST',
				headers: {
					'content-type': 'application/json',
					'x-granger-request': '1'
				},
				body: JSON.stringify(toPayload())
			});
			const body = await response.json();
			if (response.ok === false || body.ok !== true) {
				throw new Error(body.error ?? 'Could not preview routing');
			}
			status = copy.statusDryRun;
		} catch (error) {
			status = `${copy.statusError}: ${error instanceof Error ? error.message : String(error)}`;
		} finally {
			actionPending = false;
		}
	}

	function toPayload(): GraphPayload {
		return {
			version: graphVersion,
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
			<p class="mt-2 max-w-4xl text-sm text-muted-foreground">
				{copy.description}
			</p>
		</div>
		<div class="flex flex-wrap gap-2">
			<Button
				disabled={graphLoaded === false || actionPending}
				variant="outline"
				onclick={dryRun}><WarningCircleIcon />{copy.dryRun}</Button
			>
			<Button
				disabled={graphLoaded === false || actionPending}
				onclick={saveGraph}><FloppyDiskIcon />{copy.save}</Button
			>
		</div>
	</div>

	<p class="text-sm text-muted-foreground" role="status">{status}</p>

	<div class="grid items-start gap-4 xl:grid-cols-[minmax(0,1fr)_360px]">
		<Card.Root class="overflow-hidden rounded-lg">
			<Card.Content class="h-[min(640px,65vh)] min-h-[360px] p-0">
				<SvelteFlow
					bind:nodes
					bind:edges
					fitView
					colorMode="dark"
					minZoom={0.1}
					fitViewOptions={{ padding: 0.15, minZoom: 0.1 }}
					proOptions={{ hideAttribution: true }}
					onnodeclick={(event) => selectNode(event.node.id)}
					onbeforeconnect={beforeConnect}
					onconnect={connectRule}
				>
					<Controls />
					<MiniMap class="hidden sm:block" pannable zoomable bgColor="#101114" nodeColor="#71717a" maskColor="rgba(0,0,0,0.55)" />
					<Background variant={BackgroundVariant.Dots} />
				</SvelteFlow>
			</Card.Content>
		</Card.Root>

		<div class="space-y-4">
			<Card.Root class="rounded-lg">
				<Card.Header>
					<Card.Title>{copy.addNode}</Card.Title>
				</Card.Header>
				<Card.Content class="space-y-3">
					<Input
						aria-label="New rule name"
						placeholder="Rule name"
						bind:value={newRuleName}
						disabled={graphLoaded === false || actionPending}
					/>
					<Button
						disabled={graphLoaded === false ||
							actionPending ||
							newRuleName.trim() === ''}
						variant="outline"
						size="sm"
						onclick={addRule}><PlusIcon />Add rule</Button
					>
					<p class="text-xs text-muted-foreground">
						Manage users on the <a
							class="underline"
							href="/entrypoints">Entrypoints page</a
						> and connections on the
						<a class="underline" href="/upstreams">Upstreams page</a>
					</p>
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
							<Input
								value={String(selectedNode.data?.label ?? '')}
								readonly={selectedNode.data?.kind !== 'user'}
								oninput={(event) =>
									updateSelectedLabel(event.currentTarget.value)}
							/>
						</label>
						{#if selectedNode.data?.kind === 'rule' || selectedNode.data?.kind === 'upstream' || selectedNode.data?.kind === 'fallback'}
							<label class="grid gap-1.5 text-xs text-muted-foreground">
								{copy.fallback}
								<Input
									value={selectedRawValue('domain_fallback_via') ||
										selectedRawValue('fallback_when_down')}
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
						{/if}
						{#if String(selectedNode.data?.kind) === 'rule'}
							<Button
								aria-pressed={selectedBool('default')}
								variant={selectedBool('default') ? 'default' : 'outline'}
								size="sm"
								onclick={() =>
									updateSelectedBool(
										'default',
										selectedBool('default') === false
									)}
							>
								{copy.defaultUpstream}
							</Button>
						{/if}
						{#if String(selectedNode.data?.kind) === 'upstream'}
							<Button
								class="h-auto whitespace-normal text-left"
								aria-pressed={selectedBool('block_fallback')}
								variant={selectedBool('block_fallback')
									? 'destructive'
									: 'outline'}
								size="sm"
								onclick={() =>
									updateSelectedBool(
										'block_fallback',
										selectedBool('block_fallback') === false
									)}
							>
								{copy.blockFallback}
							</Button>
							<label class="grid gap-1.5 text-xs text-muted-foreground">
								{copy.dns}
								<Textarea
									value={selectedRawValue('dns')}
									placeholder="1.1.1.1&#10;9.9.9.9"
									oninput={(event) =>
										updateSelectedRaw('dns', event.currentTarget.value, true)}
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
								<Textarea
									value={selectedRawValue('domains')}
									placeholder="example.org&#10;*.media.example"
									oninput={(event) =>
										updateSelectedRaw(
											'domains',
											event.currentTarget.value,
											true
										)}
								/>
							</label>
							<label class="grid gap-1.5 text-xs text-muted-foreground">
								{copy.cidrs}
								<Textarea
									value={selectedRawValue('cidrs')}
									placeholder="10.44.0.0/16"
									oninput={(event) =>
										updateSelectedRaw('cidrs', event.currentTarget.value, true)}
								/>
							</label>
							<label class="grid gap-1.5 text-xs text-muted-foreground">
								{copy.via}
								<Input
									value={selectedRawValue('via')}
									placeholder="default_awg"
									oninput={(event) =>
										updateSelectedRaw('via', event.currentTarget.value)}
								/>
							</label>
						{/if}
					{:else}
						<p class="text-sm text-muted-foreground">{copy.noSelection}</p>
					{/if}
				</Card.Content>
			</Card.Root>

			<Card.Root class="rounded-lg">
				<Card.Header>
					<Card.Title>All items</Card.Title>
				</Card.Header>
				<Card.Content class="max-h-72 space-y-2 overflow-y-auto">
					{#each nodes as node}
						<button
							class="flex w-full items-center justify-between rounded-md border border-border bg-white/[0.03] px-3 py-2 text-left text-sm transition hover:bg-white/[0.06] {selectedId ===
							node.id
								? 'text-white'
								: 'text-muted-foreground'}"
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
