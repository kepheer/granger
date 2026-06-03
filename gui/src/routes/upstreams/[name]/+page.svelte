<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { Input } from '$lib/components/ui/input';
	import { ArrowLeftIcon, PaperPlaneTiltIcon, PowerIcon, TrashIcon, XIcon } from 'phosphor-svelte';

	type Upstream = {
		type: string;
		interface?: string;
		config?: string;
		service?: string;
		enabled?: boolean;
		dns?: string[];
	};

	type Pending = {
		pending: boolean;
		step?: string;
		step_type?: string;
		label?: string;
		output?: string;
	};

	const copy = {
		description: 'Inspect runtime state, control availability, and complete interactive authentication when a tunnel requires it.',
		back: 'Back',
		enable: 'Enable upstream',
		disable: 'Disable upstream',
		remove: 'Delete upstream',
		config: 'Configuration',
		state: 'State',
		dns: 'DNS resolvers',
		snx: 'SNX-RS authentication',
		username: 'Username',
		password: 'Password',
		start: 'Start authentication',
		submit: 'Submit step',
		cancel: 'Cancel',
		disconnect: 'Disconnect',
		value: 'Code or value',
		output: 'Command output'
	} as const;

	const name = page.params.name ?? '';
	let upstream = $state<Upstream | null>(null);
	let pending = $state<Pending>({ pending: false });
	let username = $state('');
	let password = $state('');
	let value = $state('');
	let output = $state('');
	let message = $state('');

	onMount(() => {
		void load();
	});

	async function api(url: string, method = 'GET', body?: unknown) {
		const response = await fetch(url, {
			method,
			headers: method === 'GET' ? undefined : { 'content-type': 'application/json', 'x-granger-request': '1' },
			body: body === undefined ? undefined : JSON.stringify(body)
		});
		const result = await response.json();
		if (!response.ok) throw new Error(result.error ?? response.statusText);
		return result;
	}

	async function load() {
		try {
			upstream = (await api(`/api/upstreams/${encodeURIComponent(name)}`)).data;
			if (upstream?.type === 'snx-rs') pending = (await api(`/api/snx/${encodeURIComponent(name)}/status`)).data;
			output = pending.output ?? output;
			message = '';
		} catch (error) {
			message = error instanceof Error ? error.message : String(error);
		}
	}

	async function toggle() {
		if (!upstream) return;
		await api(`/api/upstreams/${encodeURIComponent(name)}/enable`, 'POST', { enabled: upstream.enabled === false });
		await load();
	}

	async function remove() {
		if (!confirm(copy.remove + '?')) return;
		await api(`/api/upstreams/${encodeURIComponent(name)}?uninstall_protocol=true`, 'DELETE');
		window.location.href = '/upstreams';
	}

	async function snx(action: string, inputs: Record<string, string> = {}) {
		try {
			const result = await api(`/api/snx/${encodeURIComponent(name)}/${action}`, 'POST', { inputs });
			pending = result.data ?? { pending: false };
			output = pending.output ?? result.results?.map((item: { output?: string }) => item.output).filter(Boolean).join('\n\n') ?? '';
			password = '';
			value = '';
		} catch (error) {
			message = error instanceof Error ? error.message : String(error);
		}
	}
</script>

<svelte:head>
	<title>{name} · Granger</title>
</svelte:head>

<section class="max-w-5xl space-y-5">
	<Button href="/upstreams" variant="ghost"><ArrowLeftIcon />{copy.back}</Button>

	<div class="flex flex-col justify-between gap-4 md:flex-row md:items-end">
		<div>
			<h1 class="font-display text-2xl font-bold text-white">{name}</h1>
			<p class="mt-2 max-w-3xl text-sm text-muted-foreground">{copy.description}</p>
		</div>
		<div class="flex flex-wrap gap-2">
			<Button variant="outline" onclick={toggle}><PowerIcon />{upstream?.enabled === false ? copy.enable : copy.disable}</Button>
			<Button variant="destructive" onclick={remove}><TrashIcon />{copy.remove}</Button>
		</div>
	</div>

	{#if message}<p class="rounded-md border border-border bg-white/[0.03] p-3 text-sm text-red-300">{message}</p>{/if}

	<div class="grid gap-4 md:grid-cols-2">
		<Card.Root class="rounded-lg">
			<Card.Header><Card.Title>{copy.config}</Card.Title></Card.Header>
			<Card.Content class="space-y-2 text-sm text-muted-foreground">
				<div>type: <span class="font-mono text-white">{upstream?.type ?? '—'}</span></div>
				<div>interface: <span class="font-mono text-white">{upstream?.interface ?? '—'}</span></div>
				<div>path: <span class="font-mono text-white">{upstream?.config ?? '—'}</span></div>
			</Card.Content>
		</Card.Root>
		<Card.Root class="rounded-lg">
			<Card.Header><Card.Title>{copy.state}</Card.Title></Card.Header>
			<Card.Content class="space-y-2 text-sm text-muted-foreground">
				<div>enabled: <span class={upstream?.enabled === false ? 'text-red-300' : 'text-emerald-300'}>{upstream?.enabled === false ? 'false' : 'true'}</span></div>
				<div>{copy.dns}: <span class="font-mono text-white">{upstream?.dns?.join(', ') || 'default'}</span></div>
				<div>service: <span class="font-mono text-white">{upstream?.service || 'auto'}</span></div>
			</Card.Content>
		</Card.Root>
	</div>

	{#if upstream?.type === 'snx-rs'}
		<Card.Root class="rounded-lg">
			<Card.Header><Card.Title>{copy.snx}</Card.Title></Card.Header>
			<Card.Content class="space-y-4">
				{#if pending.pending}
					<label class="grid gap-1.5 text-xs text-muted-foreground">{pending.label || pending.step_type || copy.value}<Input bind:value={value} type="password" /></label>
					<div class="flex flex-wrap gap-2">
						<Button onclick={() => snx('submit', { [pending.step ?? pending.step_type ?? 'value']: value })}><PaperPlaneTiltIcon />{copy.submit}</Button>
						<Button variant="outline" onclick={() => snx('cancel')}><XIcon />{copy.cancel}</Button>
					</div>
				{:else}
					<div class="grid gap-3 md:grid-cols-2">
						<label class="grid gap-1.5 text-xs text-muted-foreground">{copy.username}<Input bind:value={username} /></label>
						<label class="grid gap-1.5 text-xs text-muted-foreground">{copy.password}<Input bind:value={password} type="password" /></label>
					</div>
					<div class="flex flex-wrap gap-2">
						<Button onclick={() => snx('start', { username, password })}><PowerIcon />{copy.start}</Button>
						<Button variant="outline" onclick={() => snx('disconnect')}><XIcon />{copy.disconnect}</Button>
					</div>
				{/if}
				{#if output}<div><div class="mb-2 text-xs text-muted-foreground">{copy.output}</div><pre class="max-h-72 overflow-auto rounded-md border border-border bg-black/35 p-4 text-xs text-white/80">{output}</pre></div>{/if}
			</Card.Content>
		</Card.Root>
	{/if}
</section>
