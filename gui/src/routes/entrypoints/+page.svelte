<script lang="ts">
	import { onMount } from 'svelte';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Checkbox } from '$lib/components/ui/checkbox';
	import { Label } from '$lib/components/ui/label';
	import UpstreamSelect from '$lib/components/upstream-select.svelte';
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
		name: 'Profile ID',
		type: 'Type',
		iface: 'Interface',
		subnet: 'Subnet',
		port: 'Port',
		clients: 'User',
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

	type Entrypoint = {
		name: string;
		type: string;
		iface: string;
		subnet: string;
		port: string;
		clients: Array<{
			name: string;
			user: string;
			displayName: string;
			ip: string;
			status: string;
		}>;
	};
	let entrypoints = $state<Entrypoint[]>([]);
	let userNames = $state<Record<string, string>>({});
	let busy = $state('');
	let loaded = $state(false);
	let profileBusy = $state('');
	let message = $state('');
	let accessSaving = $state(false);
	type UserAccess = {
		access_restricted?: boolean;
		allowed_upstreams?: string[];
		default_upstream?: string;
		display_name?: string;
	};
	let upstreamNames = $state<string[]>([]);
	let users = $state<Record<string, UserAccess>>({});
	let allowedByOutput = $state<Record<string, string[]>>({});
	let defaultByOutput = $state<Record<string, string>>({});
	let accessUser = $state('');
	let accessOpen = $state(false);
	let accessAllowed = $state<string[]>([]);
	let accessDefault = $state('');

	function createAllowed(output: string) {
		return (
			allowedByOutput[output] ??
			upstreamNames.filter((name) => name === 'direct')
		);
	}

	function changeCreateAllowed(
		output: string,
		upstream: string,
		enabled: boolean
	) {
		const allowed = createAllowed(output).filter((name) => name !== upstream);
		allowedByOutput[output] = enabled ? [...allowed, upstream] : allowed;

		if (
			allowedByOutput[output].includes(defaultByOutput[output] ?? '') === false
		) {
			defaultByOutput[output] = '';
		}
	}

	function editAccess(user: string) {
		accessUser = user;
		const current = users[user];
		accessAllowed =
			current?.access_restricted === true
				? [...(current.allowed_upstreams ?? [])]
				: [...upstreamNames];
		accessDefault = current?.default_upstream ?? '';
		accessOpen = true;
	}

	function changeAccessAllowed(upstream: string, enabled: boolean) {
		const allowed = accessAllowed.filter((name) => name !== upstream);
		accessAllowed = enabled ? [...allowed, upstream] : allowed;
		if (accessAllowed.includes(accessDefault) === false) {
			accessDefault = '';
		}
	}

	async function saveAccess() {
		if (accessSaving || accessUser === '') {
			return;
		}

		accessSaving = true;
		message = '';
		try {
			const response = await fetch(
				`/api/users/${encodeURIComponent(accessUser)}`,
				{
					method: 'PUT',
					headers: {
						'content-type': 'application/json',
						'x-granger-request': '1'
					},
					body: JSON.stringify({
						access_restricted: true,
						allowed_upstreams: accessAllowed,
						default_upstream: accessDefault
					})
				}
			);
			const body = await response.json();
			if (response.ok === false || body.ok !== true) {
				const failure = body.results?.find(
					(result: { OK: boolean }) => result.OK === false
				);
				throw new Error(
					body.error ?? failure?.Output ?? 'Could not apply user access'
				);
			}
			accessOpen = false;
			await loadEntrypoints();
		} catch (error) {
			message = error instanceof Error ? error.message : String(error);
		} finally {
			accessSaving = false;
		}
	}

	async function createUser(output: string) {
		if (busy !== '') {
			return;
		}

		const displayName = (userNames[output] ?? '').trim();
		if (displayName === '') {
			message = 'Enter a user name';
			return;
		}
		busy = output;
		message = '';
		try {
			const response = await fetch('/api/users', {
				method: 'POST',
				headers: {
					'content-type': 'application/json',
					'x-granger-request': '1'
				},
				body: JSON.stringify({
					display_name: displayName,
					output,
					access_restricted: true,
					allowed_upstreams: createAllowed(output),
					default_upstream: defaultByOutput[output] ?? ''
				})
			});
			const body = await response.json();
			if (response.ok === false || body.ok !== true) {
				const failed = body.results?.find(
					(result: { OK: boolean }) => result.OK === false
				);
				throw new Error(
					body.error ?? failed?.Output ?? 'Could not issue profile'
				);
			}
			userNames[output] = '';
			await loadEntrypoints();
		} catch (error) {
			message = error instanceof Error ? error.message : String(error);
		} finally {
			busy = '';
		}
	}

	let qrOpen = $state(false);
	let qrImage = $state('');
	let qrName = $state('');

	onMount(() => {
		void loadEntrypoints();
	});

	async function copyProfile(output: string, client: string) {
		message = '';
		try {
			const response = await fetch(
				`/api/profiles/${encodeURIComponent(output)}/${encodeURIComponent(client)}`
			);
			if (response.ok === false) {
				throw new Error('Could not download the profile');
			}

			if (navigator.clipboard === undefined) {
				throw new Error(
					'Copy is unavailable in this connection; download the profile instead'
				);
			}

			await navigator.clipboard.writeText(await response.text());
		} catch (error) {
			message = error instanceof Error ? error.message : String(error);
		}
	}

	async function showQR(output: string, client: string) {
		message = '';
		try {
			const response = await fetch(
				`/api/profiles/${encodeURIComponent(output)}/${encodeURIComponent(client)}`
			);
			if (response.ok === false) {
				throw new Error('Could not download the profile');
			}

			qrName = client;
			qrImage = await QRCode.toDataURL(await response.text(), {
				margin: 2,
				width: 360
			});
			qrOpen = true;
		} catch (error) {
			message = error instanceof Error ? error.message : String(error);
		}
	}

	async function toggleProfile(user: string, disabled: boolean) {
		if (profileBusy !== '') {
			return;
		}

		profileBusy = user;
		message = '';
		try {
			const response = await fetch(`/api/users/${encodeURIComponent(user)}`, {
				method: 'PUT',
				headers: {
					'content-type': 'application/json',
					'x-granger-request': '1'
				},
				body: JSON.stringify({ disabled })
			});
			const body = await response.json();
			if (response.ok === false || body.ok !== true) {
				const failure = body.results?.find(
					(result: { OK: boolean }) => result.OK === false
				);
				throw new Error(
					body.error ?? failure?.Output ?? 'Could not change access'
				);
			}

			await loadEntrypoints();
		} catch (error) {
			message = error instanceof Error ? error.message : String(error);
		} finally {
			profileBusy = '';
		}
	}

	async function loadEntrypoints() {
		try {
			const response = await fetch('/api/config');
			if (response.ok === false) {
				throw new Error(response.statusText);
			}
			const body = await response.json();
			if (body.ok !== true || body.config === undefined) {
				throw new Error(body.error ?? 'Could not load entrypoints');
			}
			const outputs = body.config.outputs ?? {};
			upstreamNames = Object.keys(body.config?.upstreams ?? {}).sort();
			users = body.config?.users ?? {};
			entrypoints = Object.entries(outputs).map(([name, value]) => {
				const item = value as Record<string, unknown>;
				return {
					name,
					type: String(item.type ?? ''),
					iface: String(item.interface ?? ''),
					subnet: String(item.subnet ?? ''),
					port: String(item.listen_port ?? ''),
					clients: ((item.clients as Array<Record<string, unknown>>) ?? []).map(
						(client) => ({
							name: String(client.name ?? ''),
							user: String(client.user ?? ''),
							displayName: String(
								body.config?.users?.[String(client.user ?? '')]?.display_name ??
									client.user ??
									''
							),
							ip: String(client.ip ?? ''),
							status:
								client.disabled === true ||
								body.config?.users?.[String(client.user ?? '')]?.disabled ===
									true
									? 'disabled'
									: 'enabled'
						})
					)
				};
			});
		} catch (error) {
			message = error instanceof Error ? error.message : String(error);
		} finally {
			loaded = true;
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
			<p class="mt-2 max-w-3xl text-sm text-muted-foreground">
				{copy.description}
			</p>
		</div>
		<div class="flex flex-wrap gap-2">
			<Button href="/entrypoints/new"><PlusIcon />{copy.add}</Button>
		</div>
	</div>

	{#if message}<p class="text-sm text-red-300" role="alert">{message}</p>{/if}
	<div class="space-y-4">
		{#if loaded === false}<p
				role="status"
				class="text-sm text-muted-foreground"
			>
				Loading entrypoints…
			</p>{/if}
		{#if loaded && entrypoints.length === 0 && message === ''}<p
				class="rounded-lg border border-border p-6 text-sm text-muted-foreground"
			>
				No entrypoints configured. Add your first entrypoint to issue client
				profiles.
			</p>{/if}
		{#each entrypoints as entrypoint}
			<Card.Root class="rounded-lg">
				<Card.Header
					class="flex flex-row flex-wrap items-start justify-between gap-4"
				>
					<div>
						<Card.Title>{entrypoint.name}</Card.Title>
						<Card.Description
							>{[entrypoint.type, entrypoint.iface, entrypoint.subnet]
								.filter((value) => value !== '')
								.join(' · ')}</Card.Description
						>
					</div>
					<div
						class="rounded-full border border-border px-3 py-1 text-xs text-muted-foreground"
					>
						{copy.port}
						{entrypoint.port}
					</div>
				</Card.Header>
				<Card.Content>
					<div class="mb-4 flex flex-wrap gap-2">
						<label class="grid w-full gap-2 text-xs text-muted-foreground">
							User name
							<Input
								aria-label={`User name for ${entrypoint.name}`}
								placeholder="User name"
								bind:value={userNames[entrypoint.name]}
							/>
						</label>
						<Button
							disabled={busy !== ''}
							onclick={() => createUser(entrypoint.name)}
							><PlusIcon />Create user and config</Button
						>
					</div>
					<fieldset
						class="mb-4 flex flex-wrap gap-4 text-xs text-muted-foreground"
					>
						<legend class="mb-2">Allowed upstreams for the new user</legend>
						{#each upstreamNames as upstream}
							<div class="flex items-center gap-2">
								<Checkbox
									id={`create-${entrypoint.name}-${upstream}`}
									checked={createAllowed(entrypoint.name).includes(upstream)}
									disabled={busy !== ''}
									onCheckedChange={(checked) =>
										changeCreateAllowed(entrypoint.name, upstream, checked)}
								/>
								<Label for={`create-${entrypoint.name}-${upstream}`}
									>{upstream}</Label
								>
							</div>
						{/each}
						<div class="grid w-full gap-2 sm:max-w-xs">
							<Label for={`default-${entrypoint.name}`}>Default route</Label>
							<UpstreamSelect
								id={`default-${entrypoint.name}`}
								options={createAllowed(entrypoint.name)}
								bind:value={defaultByOutput[entrypoint.name]}
								disabled={busy !== ''}
							/>
						</div>
					</fieldset>
					<div
						role="table"
						aria-label={`Users of ${entrypoint.name}`}
						class="overflow-x-auto rounded-lg border border-border"
					>
						<div
							role="row"
							class="grid min-w-[900px] grid-cols-[1fr_.75fr_.7fr_.7fr_1.9fr] bg-white/[0.035] px-4 py-2 text-xs font-medium text-muted-foreground"
						>
							<div role="columnheader">{copy.name}</div>
							<div role="columnheader">{copy.clients}</div>
							<div role="columnheader">IP</div>
							<div role="columnheader">{copy.status}</div>
							<div role="columnheader">{copy.actions}</div>
						</div>
						{#each entrypoint.clients as client}
							<div
								role="row"
								class="grid min-w-[900px] grid-cols-[1fr_.75fr_.7fr_.7fr_1.9fr] items-center border-t border-border px-4 py-3 text-sm"
							>
								<div role="rowheader" class="font-medium text-white">
									{client.name}
								</div>
								<div role="cell" class="text-muted-foreground">
									{client.displayName}
								</div>
								<div role="cell" class="font-mono text-xs text-white/80">
									{client.ip || '—'}
								</div>
								<div
									role="cell"
									class={client.status === 'enabled'
										? 'text-emerald-300'
										: 'text-red-300'}
								>
									{client.status === 'enabled' ? copy.enabled : copy.disabled}
								</div>
								<div role="cell" class="flex flex-wrap gap-1">
									<Button
										disabled={client.user === '' || profileBusy !== ''}
										size="xs"
										variant="outline"
										onclick={() => editAccess(client.user)}>Access</Button
									>
									<Button
										disabled={client.status !== 'enabled'}
										size="xs"
										variant="ghost"
										href={`/api/profiles/${entrypoint.name}/${client.name}`}
										><DownloadSimpleIcon />{copy.download}</Button
									>
									<Button
										disabled={client.status !== 'enabled'}
										size="xs"
										variant="ghost"
										onclick={() => copyProfile(entrypoint.name, client.name)}
										><CopyIcon />{copy.copy}</Button
									>
									<Button
										disabled={client.status !== 'enabled'}
										size="xs"
										variant="outline"
										onclick={() => showQR(entrypoint.name, client.name)}
										><QrCodeIcon />{copy.qr}</Button
									>
									<Button
										disabled={client.user === '' || profileBusy !== ''}
										size="xs"
										variant={client.status === 'enabled'
											? 'destructive'
											: 'outline'}
										onclick={() =>
											toggleProfile(client.user, client.status === 'enabled')}
										><PowerIcon />{profileBusy === client.user
											? 'Applying…'
											: client.status === 'enabled'
												? copy.disable
												: copy.enable}</Button
									>
								</div>
							</div>
						{:else}<p class="p-4 text-sm text-muted-foreground">
								No profiles issued yet. Create a user above to generate a
								connection config.
							</p>
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
			<Sheet.Description>Personal connection profile</Sheet.Description>
		</Sheet.Header>
		{#if qrImage}
			<div class="mx-4 mb-6 mt-2 overflow-y-auto rounded-md bg-white p-4">
				<img
					class="mx-auto block w-full max-w-[360px]"
					src={qrImage}
					alt={qrName}
				/>
			</div>
		{/if}
	</Sheet.Content>
</Sheet.Root>

<Sheet.Root bind:open={accessOpen}>
	<Sheet.Content class="w-full sm:max-w-md">
		<Sheet.Header
			><Sheet.Title>User access</Sheet.Title><Sheet.Description
				>{users[accessUser]?.display_name ?? accessUser}</Sheet.Description
			></Sheet.Header
		>
		<div class="overflow-y-auto px-4 pb-6">
			<fieldset disabled={accessSaving} class="mt-6 space-y-3">
				<legend class="mb-3">Allowed upstreams</legend>
				{#each upstreamNames as upstream}
					<div class="flex items-center gap-2">
						<Checkbox
							id={`access-${upstream}`}
							checked={accessAllowed.includes(upstream)}
							disabled={accessSaving}
							onCheckedChange={(checked) =>
								changeAccessAllowed(upstream, checked)}
						/>
						<Label for={`access-${upstream}`}>{upstream}</Label>
					</div>
				{/each}
			</fieldset>
			<div class="mt-6 grid gap-2">
				<Label for="access-default">Default route</Label>
				<UpstreamSelect
					id="access-default"
					options={accessAllowed}
					bind:value={accessDefault}
					disabled={accessSaving}
				/>
			</div>
			{#if message}<p class="mt-4 text-sm text-red-300" role="alert">
					{message}
				</p>{/if}
			<p class="mt-4 text-xs text-muted-foreground">
				When a shared rule selects an unchecked upstream, traffic uses this
				user's allowed default route. If no allowed default is available,
				traffic is blocked. Leave all unchecked to block every upstream.
			</p>
			<Button disabled={accessSaving} class="mt-6" onclick={saveAccess}
				>Apply access</Button
			>
		</div></Sheet.Content
	>
</Sheet.Root>
