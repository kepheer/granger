<script lang="ts">
	import { stateLabel } from '$lib/labels';
	import ConfirmAction from '$lib/components/confirm-action.svelte';
	import { onMount, tick } from 'svelte';
	import { page } from '$app/state';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { Input } from '$lib/components/ui/input';
	import { Textarea } from '$lib/components/ui/textarea';
	import {
		ArrowLeftIcon,
		PaperPlaneTiltIcon,
		PowerIcon,
		TrashIcon,
		XIcon
	} from 'phosphor-svelte';

	type Upstream = {
		type: string;
		interface?: string;
		config?: string;
		service?: string;
		enabled?: boolean;
		dns?: string[];
	};

	type Pending = {
		session_id?: string;
		pending: boolean;
		busy?: boolean;
		secret?: boolean;
		result?: { OK: boolean; Output: string; Status: string };
		step?: string;
		step_type?: string;
		label?: string;
		output?: string;
	};

	const copy = {
		description: 'Connection settings and status',
		back: 'Back',
		enable: 'Enable upstream',
		disable: 'Disable upstream',
		remove: 'Delete upstream',
		config: 'Configuration',
		state: 'Status',
		dns: 'DNS servers',
		snx: 'SNX-RS sign-in',
		username: 'Username',
		password: 'Password',
		start: 'Sign in',
		submit: 'Continue',
		cancel: 'Cancel',
		disconnect: 'Disconnect',
		value: 'Verification code',
		output: 'Connection log'
	} as const;

	const name = page.params.name ?? '';
	let upstream = $state<Upstream | null>(null);
	let pending = $state<Pending>({ pending: false });
	let username = $state('');
	let password = $state('');
	let initialStep = $state({
		name: 'password',
		type: 'password',
		label: 'Password',
		secret: true
	});
	let value = $state('');
	let stepInput = $state<HTMLInputElement | null>(null);
	let usernameInput = $state<HTMLInputElement | null>(null);
	let output = $state('');
	let message = $state('');
	let busy = $state(false);
	let removeOpen = $state(false);
	let requesting = $state(false);
	const authenticating = $derived(requesting || pending.busy === true);
	let refreshing = false;
	let inlineConfig = $state('');
	let savingConfig = $state(false);
	let configSaved = $state('');
	let configStatus = $state<HTMLParagraphElement | null>(null);
	let configReady = $state(false);
	let driverInstalled = $state(false);
	let setupLoaded = $state(false);
	let configError = $state('');
	const canSignIn = $derived(
		configReady && driverInstalled && upstream?.enabled !== false
	);
	let runtime = $state<{
		State: string;
		Summary: string;
		Service?: string;
	} | null>(null);

	onMount(() => {
		void load();
		const timer = window.setInterval(refreshStatus, 5000);
		return () => window.clearInterval(timer);
	});

	async function api(url: string, method = 'GET', body?: unknown) {
		const response = await fetch(url, {
			method,
			headers:
				method === 'GET'
					? undefined
					: { 'content-type': 'application/json', 'x-granger-request': '1' },
			body: body === undefined ? undefined : JSON.stringify(body),
			signal: url.startsWith('/api/snx/')
				? AbortSignal.timeout(15000)
				: undefined
		});
		const result = await response.json();
		if (response.ok === false || result.ok !== true) {
			const failure = result.results?.find(
				(item: { OK: boolean }) => item.OK === false
			);
			throw new Error(result.error ?? failure?.Output ?? response.statusText);
		}
		return result;
	}

	async function load() {
		try {
			upstream = (await api(`/api/upstreams/${encodeURIComponent(name)}`)).data;
			await refreshStatus();
			output = pending.output ?? output;
			message = '';
		} catch (error) {
			message = error instanceof Error ? error.message : String(error);
		}
	}

	async function refreshStatus() {
		if (refreshing) {
			return;
		}
		refreshing = true;
		try {
			if (upstream?.type === 'snx-rs') {
				const status = (
					await api(`/api/snx/${encodeURIComponent(name)}/status`)
				).data;
				await updatePending(status);
				initialStep = status.initial_step ?? initialStep;
				configReady = status.config_ready === true;
				driverInstalled = status.driver_installed === true;
				configError = status.config_error ?? '';
				setupLoaded = true;
				output = pending.output ?? output;
			}
			const result = await api('/api/runtime');
			runtime =
				result.data.find(
					(item: { Name: string; Type: string }) =>
						item.Name === name && item.Type === upstream?.type
				) ?? null;
		} catch (error) {
			message = error instanceof Error ? error.message : String(error);
		} finally {
			refreshing = false;
		}
	}

	async function chooseConfig(event: Event) {
		const file = (event.currentTarget as HTMLInputElement).files?.[0];
		if (file === undefined) {
			return;
		}
		message = '';
		configSaved = '';
		try {
			if (file.size > 4 * 1024 * 1024) {
				throw new Error('Choose a config smaller than 4 MB');
			}
			inlineConfig = await file.text();
		} catch (error) {
			message = error instanceof Error ? error.message : String(error);
		}
	}

	async function saveConnectionConfig() {
		if (savingConfig || inlineConfig.trim() === '') {
			return;
		}
		savingConfig = true;
		configSaved = '';
		message = '';
		try {
			await api(`/api/upstreams/${encodeURIComponent(name)}/config`, 'POST', {
				inline_config: inlineConfig
			});
			inlineConfig = '';
			await load();
			configSaved = 'VPN config saved';
			await tick();
			configStatus?.focus();
		} catch (error) {
			message = error instanceof Error ? error.message : String(error);
		} finally {
			savingConfig = false;
		}
	}

	async function toggle() {
		if (upstream === null || busy) {
			return;
		}

		busy = true;
		try {
			await api(`/api/upstreams/${encodeURIComponent(name)}/enable`, 'POST', {
				enabled: upstream.enabled === false
			});
			await load();
		} catch (error) {
			message = error instanceof Error ? error.message : String(error);
		} finally {
			busy = false;
		}
	}

	async function remove() {
		if (busy) {
			return;
		}

		busy = true;
		message = '';
		try {
			await api(`/api/upstreams/${encodeURIComponent(name)}`, 'DELETE');
			window.location.href = '/upstreams';
		} catch (error) {
			message = error instanceof Error ? error.message : String(error);
		} finally {
			busy = false;
		}
	}

	async function updatePending(status: Pending) {
		const focusStep =
			pending.pending === false && status.pending && status.busy !== true;
		pending = status;
		output = status.output ?? status.result?.Output ?? output;
		if (status.result?.OK === false) {
			message = status.result.Output || 'Could not sign in — try again';
		}
		if (focusStep) {
			await tick();
			stepInput?.focus();
		}
	}

	async function snx(action: string, inputs: Record<string, string> = {}) {
		if (requesting || (pending.busy === true && action !== 'cancel')) {
			return;
		}

		requesting = true;
		message = '';
		try {
			const result = await api(
				`/api/snx/${encodeURIComponent(name)}/${action}`,
				'POST',
				{ inputs, session_id: pending.session_id }
			);
			await updatePending(result.data ?? { pending: false });
			output =
				pending.output ??
				result.results
					?.map((item: { Output?: string }) => item.Output)
					.filter(Boolean)
					.join('\n\n') ??
				'';
			password = '';
			value = '';
			await tick();
			if (pending.pending) {
				stepInput?.focus();
			}
			if (pending.pending === false) {
				usernameInput?.focus();
			}
		} catch (error) {
			message = error instanceof Error ? error.message : String(error);
		} finally {
			requesting = false;
		}
	}
</script>

<svelte:head>
	<title>{name} · Granger</title>
</svelte:head>

<section class="w-full min-w-0 space-y-5">
	<Button href="/upstreams" variant="ghost"><ArrowLeftIcon />{copy.back}</Button
	>

	<div class="flex flex-col justify-between gap-4 md:flex-row md:items-end">
		<div>
			<h1 class="font-display text-2xl font-bold text-white">{name}</h1>
			<p class="mt-2 max-w-3xl text-sm text-muted-foreground">
				{copy.description}
			</p>
		</div>
		<div class="flex shrink-0 flex-wrap gap-2">
			<Button
				disabled={upstream === null ||
					busy ||
					(upstream.type === 'snx-rs' &&
						upstream.enabled === false &&
						(configReady === false || driverInstalled === false))}
				variant="outline"
				onclick={toggle}
				><PowerIcon />{upstream?.enabled === false
					? copy.enable
					: copy.disable}</Button
			>
			<Button
				disabled={upstream === null || busy}
				variant="destructive"
				onclick={() => {
					message = '';
					removeOpen = true;
				}}><TrashIcon />{copy.remove}</Button
			>
		</div>
	</div>

	{#if message}<p
			role="alert"
			class="rounded-md border border-border bg-white/[0.03] p-3 text-sm text-red-300"
		>
			{message}
		</p>{/if}

	<div class="grid gap-4 md:grid-cols-2">
		<Card.Root class="min-w-0 rounded-lg">
			<Card.Header><Card.Title>{copy.config}</Card.Title></Card.Header>
			<Card.Content class="space-y-2 text-sm break-words text-muted-foreground">
				<div>
					Protocol: <span class="font-mono text-white"
						>{upstream?.type ?? '—'}</span
					>
				</div>
				<div>
					Interface: <span class="font-mono text-white"
						>{upstream?.interface ?? '—'}</span
					>
				</div>
				<div>
					Config file: <span class="font-mono text-white"
						>{upstream?.config ?? '—'}</span
					>
				</div>
			</Card.Content>
		</Card.Root>
		<Card.Root class="min-w-0 rounded-lg">
			<Card.Header><Card.Title>{copy.state}</Card.Title></Card.Header>
			<Card.Content class="space-y-2 text-sm break-words text-muted-foreground">
				<div>
					Access: <span
						class={upstream === null
							? 'text-muted-foreground'
							: upstream.enabled === false
								? 'text-red-300'
								: 'text-emerald-300'}
						>{upstream === null
							? 'Unavailable'
							: upstream.enabled === false
								? 'Disabled'
								: 'Enabled'}</span
					>
				</div>
				<div>
					Connection: <span
						class={runtime?.State === 'healthy'
							? 'text-emerald-300'
							: 'text-amber-300'}
						>{runtime?.State === undefined
							? upstream === null
								? 'Unavailable'
								: 'Unknown'
							: stateLabel(runtime.State)}</span
					>
				</div>
				{#if runtime?.Summary}<p>{runtime.Summary}</p>{/if}
				<div>
					{copy.dns}:
					<span class="font-mono text-white"
						>{upstream?.dns?.join(', ') || 'System default'}</span
					>
				</div>
				<div>
					Service: <span class="font-mono text-white"
						>{runtime?.Service || upstream?.service || 'Not required'}</span
					>
				</div>
			</Card.Content>
		</Card.Root>
	</div>

	{#if upstream?.type === 'snx-rs'}
		<Card.Root class="min-w-0 rounded-lg">
			<Card.Header>
				<Card.Title>VPN config</Card.Title>
				<Card.Description
					>Paste or upload the config from your VPN administrator</Card.Description
				>
			</Card.Header>
			<Card.Content>
				<form
					class="space-y-4"
					aria-busy={savingConfig}
					onsubmit={(event) => {
						event.preventDefault();
						void saveConnectionConfig();
					}}
				>
					{#if setupLoaded && configReady === false}<p
							role="status"
							class="text-sm text-amber-200"
						>
							{configError}
						</p>{/if}
					<label class="grid gap-2 text-sm"
						>Upload config<Input
							type="file"
							accept=".conf,.txt,text/plain"
							disabled={savingConfig || authenticating || pending.pending}
							onchange={chooseConfig}
						/></label
					>
					<label class="grid gap-2 text-sm"
						>Connection config<Textarea
							class="min-h-40 font-mono text-xs"
							bind:value={inlineConfig}
							placeholder="Paste your VPN config"
							disabled={savingConfig || authenticating || pending.pending}
						/></label
					>
					{#if configSaved}<p
							bind:this={configStatus}
							tabindex="-1"
							role="status"
							class="text-sm text-emerald-300"
						>
							{configSaved}
						</p>{/if}
					<Button
						type="submit"
						disabled={savingConfig ||
							authenticating ||
							inlineConfig.trim() === ''}
						>{savingConfig ? 'Saving…' : 'Save config'}</Button
					>
				</form>
			</Card.Content>
		</Card.Root>
		<Card.Root class="min-w-0 rounded-lg">
			<Card.Header><Card.Title>{copy.snx}</Card.Title></Card.Header>
			<Card.Content class="space-y-4">
				{#if setupLoaded && driverInstalled === false}<p
						class="text-sm text-muted-foreground"
					>
						<a class="underline" href="/upstreams">Install the SNX-RS driver</a> to
						sign in
					</p>{/if}
				{#if configReady && driverInstalled && upstream.enabled === false}<p
						class="text-sm text-muted-foreground"
					>
						Enable this connection to sign in
					</p>{/if}
				{#if pending.busy}
					<p role="status" class="text-sm text-muted-foreground">
						Waiting for the VPN server
					</p>
					<Button
						variant="outline"
						disabled={requesting}
						onclick={() => snx('cancel')}
					>
						<XIcon />{copy.cancel}
					</Button>
				{:else if pending.pending}
					<label class="grid gap-1.5 text-xs text-muted-foreground"
						>{pending.label || pending.step_type || copy.value}<Input
							bind:ref={stepInput}
							bind:value
							type={pending.secret === false ? 'text' : 'password'}
							autocomplete="one-time-code"
						/></label
					>
					<div class="flex shrink-0 flex-wrap gap-2">
						<Button
							disabled={authenticating || value.trim() === ''}
							onclick={() =>
								snx('submit', {
									[pending.step ?? pending.step_type ?? 'value']: value
								})}><PaperPlaneTiltIcon />{copy.submit}</Button
						>
						<Button
							disabled={authenticating}
							variant="outline"
							onclick={() => snx('cancel')}><XIcon />{copy.cancel}</Button
						>
					</div>
				{:else}
					<div class="grid gap-3 md:grid-cols-2">
						<label class="grid gap-1.5 text-xs text-muted-foreground"
							>{copy.username}<Input
								disabled={canSignIn === false || authenticating}
								bind:ref={usernameInput}
								bind:value={username}
								autocomplete="username"
							/></label
						>
						<label class="grid gap-1.5 text-xs text-muted-foreground"
							>{initialStep.label || initialStep.type}<Input
								disabled={canSignIn === false || authenticating}
								bind:value={password}
								type={initialStep.secret || initialStep.type === 'password'
									? 'password'
									: 'text'}
								autocomplete="current-password"
							/></label
						>
					</div>
					<div class="flex shrink-0 flex-wrap gap-2">
						<Button
							disabled={authenticating ||
								canSignIn === false ||
								savingConfig ||
								password === ''}
							onclick={() =>
								snx('start', {
									username,
									[initialStep.name || initialStep.type]: password
								})}><PowerIcon />{copy.start}</Button
						>
						<Button
							disabled={authenticating}
							variant="outline"
							onclick={() => snx('disconnect')}
							><XIcon />{copy.disconnect}</Button
						>
					</div>
				{/if}
				{#if output}<div>
						<div class="mb-2 text-xs text-muted-foreground">{copy.output}</div>
						<pre
							class="max-h-72 overflow-auto whitespace-pre-wrap break-words rounded-md border border-border bg-black/35 p-4 text-xs text-white/80">{output}</pre>
					</div>{/if}
			</Card.Content>
		</Card.Root>
	{/if}
</section>

<ConfirmAction
	bind:open={removeOpen}
	title={`Delete upstream ${name}?`}
	description="Delete this connection and keep its driver installed. Remove it from routing rules and user access first"
	action="Delete upstream"
	danger
	{busy}
	error={message}
	onconfirm={remove}
/>
