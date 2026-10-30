<script lang="ts">
	import { onMount, tick } from 'svelte';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Checkbox } from '$lib/components/ui/checkbox';
	import { Label } from '$lib/components/ui/label';
	import UpstreamSelect from '$lib/components/upstream-select.svelte';
	import * as Card from '$lib/components/ui/card';
	import * as Sheet from '$lib/components/ui/sheet';
	import * as Select from '$lib/components/ui/select';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import * as Tooltip from '$lib/components/ui/tooltip';
	import QRCode from 'qrcode';
	import {
		DownloadSimpleIcon,
		PlusIcon,
		QrCodeIcon,
		CopyIcon,
		PowerIcon,
		DotsThreeIcon,
		ShieldCheckIcon
	} from 'phosphor-svelte';

	const copy = {
		title: 'Entrypoints',
		description: 'Manage user connections, profiles and access',
		add: 'Add entrypoint',
		name: 'Profile ID',
		type: 'Type',
		iface: 'Interface',
		subnet: 'Subnet',
		port: 'Port',
		clients: 'User',
		status: 'Status',
		actions: 'Actions',
		enabled: 'Enabled',
		disabled: 'Disabled',
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
		disabled?: boolean;
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
	let accessTrigger = $state<HTMLButtonElement | null>(null);
	let actionTriggers = $state<Record<string, HTMLButtonElement | null>>({});

	let createOpen = $state(false);
	let createOutput = $state('');
	let existingUser = $state('');
	let profileUserTrigger = $state<HTMLButtonElement | null>(null);
	const profileUserPortal = $derived(
		profileUserTrigger?.closest('[role="dialog"]') ?? undefined
	);
	let createError = $state('');
	let createNameError = $state('');
	let createSuccess = $state('');
	let createTrigger = $state<HTMLElement | null>(null);
	let createNameInput = $state<HTMLInputElement | null>(null);
	let createErrorSummary = $state<HTMLParagraphElement | null>(null);
	let createProgress = $state<HTMLParagraphElement | null>(null);

	function openCreateUser(output: string, trigger: HTMLElement) {
		createOutput = output;
		existingUser = '';
		createTrigger = trigger;
		if (busy === '') {
			createError = '';
			createNameError = '';
		}
		createSuccess = '';
		createOpen = true;
	}

	function createAllowed(output: string) {
		return allowedByOutput[output] ?? [];
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

	function editAccess(user: string, trigger: HTMLButtonElement | null) {
		accessTrigger = trigger;
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
		if (displayName === '' && existingUser === '') {
			createNameError = 'Enter a user name';
			await tick();
			createNameInput?.focus();
			return;
		}
		busy = output;
		message = '';
		createError = '';
		createNameError = '';
		await tick();
		createProgress?.focus();
		try {
			const response = await fetch(
				existingUser === ''
					? '/api/users'
					: `/api/users/${encodeURIComponent(existingUser)}/profiles`,
				{
					method: 'POST',
					headers: {
						'content-type': 'application/json',
						'x-granger-request': '1'
					},
					body: JSON.stringify(
						existingUser !== ''
							? { output }
							: {
									display_name: displayName,
									output,
									access_restricted: true,
									allowed_upstreams: createAllowed(output),
									default_upstream: defaultByOutput[output] ?? ''
								}
					)
				}
			);
			const body = await response.json();
			if (response.ok === false || body.ok !== true) {
				const failed = body.results?.find(
					(result: { OK: boolean }) => result.OK === false
				);
				throw new Error(
					body.error ?? failed?.Output ?? 'Could not issue profile'
				);
			}
			createOpen = false;
			createSuccess =
				existingUser === '' ? `User ${displayName} created` : 'Profile created';
			userNames[output] = '';
			await loadEntrypoints();
		} catch (error) {
			const errorMessage =
				error instanceof Error ? error.message : String(error);
			if (createOpen) {
				createError = errorMessage;
				await tick();
				createErrorSummary?.focus();
			}
			if (createOpen === false) {
				message = errorMessage;
			}
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

	async function toggleProfile(
		output: string,
		client: string,
		user: string,
		disabled: boolean,
		trigger: HTMLButtonElement | null
	) {
		if (profileBusy !== '') {
			return;
		}

		profileBusy = user;
		message = '';
		try {
			const response = await fetch(
				`/api/profiles/${encodeURIComponent(output)}/${encodeURIComponent(client)}`,
				{
					method: 'PUT',
					headers: {
						'content-type': 'application/json',
						'x-granger-request': '1'
					},
					body: JSON.stringify({ disabled })
				}
			);
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
			await tick();
			trigger?.focus();
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
	{#if createSuccess}<p role="status" class="text-sm text-emerald-300">
			{createSuccess}
		</p>{/if}
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
				Add an entrypoint to start creating user profiles
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
					<div class="mb-4">
						<Button
							type="button"
							aria-haspopup="dialog"
							aria-controls={createOpen && createOutput === entrypoint.name
								? 'create-user-dialog'
								: undefined}
							disabled={busy !== '' && busy !== entrypoint.name}
							onclick={(event) =>
								openCreateUser(entrypoint.name, event.currentTarget)}
							><PlusIcon />Create user</Button
						>
					</div>
					<div
						role="table"
						aria-label={`Users of ${entrypoint.name}`}
						class="overflow-x-auto rounded-lg border border-border"
					>
						<div
							role="row"
							class="grid min-w-[760px] grid-cols-[minmax(150px,1fr)_minmax(120px,.9fr)_minmax(116px,.8fr)_96px_240px] bg-white/[0.035] px-4 py-2 text-xs font-medium text-muted-foreground"
						>
							<div role="columnheader">{copy.name}</div>
							<div role="columnheader">{copy.clients}</div>
							<div role="columnheader">IP</div>
							<div role="columnheader">{copy.status}</div>
							<div role="columnheader" class="text-right">{copy.actions}</div>
						</div>
						{#each entrypoint.clients as client}
							<div
								role="row"
								class="grid min-w-[760px] grid-cols-[minmax(150px,1fr)_minmax(120px,.9fr)_minmax(116px,.8fr)_96px_240px] items-center border-t border-border px-4 py-3 text-sm"
							>
								<div role="rowheader" class="break-all font-medium text-white">
									{client.name}
								</div>
								<div role="cell" class="break-words text-muted-foreground">
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
								<div
									role="cell"
									class="flex items-center justify-end gap-1.5 whitespace-nowrap"
								>
									<Button
										disabled={client.status !== 'enabled'}
										size="sm"
										href={`/api/profiles/${entrypoint.name}/${client.name}`}
									>
										<DownloadSimpleIcon aria-hidden="true" />{copy.download}
									</Button>
									<Tooltip.Provider delayDuration={250}>
										<Tooltip.Root>
											<Tooltip.Trigger>
												{#snippet child({ props })}
													<Button
														{...props}
														disabled={client.status !== 'enabled'}
														size="icon-sm"
														variant="ghost"
														aria-label={`Copy config for ${client.displayName}`}
														onclick={() =>
															copyProfile(entrypoint.name, client.name)}
													>
														<CopyIcon aria-hidden="true" />
													</Button>
												{/snippet}
											</Tooltip.Trigger>
											<Tooltip.Content>Copy config</Tooltip.Content>
										</Tooltip.Root>
										<Tooltip.Root>
											<Tooltip.Trigger>
												{#snippet child({ props })}
													<Button
														{...props}
														disabled={client.status !== 'enabled'}
														size="icon-sm"
														variant="ghost"
														aria-label={`QR code for ${client.displayName}`}
														onclick={() => showQR(entrypoint.name, client.name)}
													>
														<QrCodeIcon aria-hidden="true" />
													</Button>
												{/snippet}
											</Tooltip.Trigger>
											<Tooltip.Content>QR code</Tooltip.Content>
										</Tooltip.Root>
									</Tooltip.Provider>
									<span class="mx-0.5 h-4 w-px bg-border" aria-hidden="true"
									></span>
									<DropdownMenu.Root>
										<DropdownMenu.Trigger>
											{#snippet child({ props })}
												<Button
													{...props}
													disabled={client.user === '' || profileBusy !== ''}
													size="icon-sm"
													variant="ghost"
													aria-label={`More actions for ${client.displayName}`}
													bind:ref={
														() =>
															actionTriggers[
																`${entrypoint.name}/${client.name}`
															] ?? null,
														(value) => {
															actionTriggers[
																`${entrypoint.name}/${client.name}`
															] = value;
														}
													}
												>
													<DotsThreeIcon weight="bold" aria-hidden="true" />
												</Button>
											{/snippet}
										</DropdownMenu.Trigger>
										<DropdownMenu.Content
											align="end"
											onCloseAutoFocus={(event) => {
												if (accessOpen) {
													event.preventDefault();
												}
											}}
										>
											<DropdownMenu.Item
												onSelect={() =>
													editAccess(
														client.user,
														actionTriggers[
															`${entrypoint.name}/${client.name}`
														] ?? null
													)}
											>
												<ShieldCheckIcon aria-hidden="true" />Manage access
											</DropdownMenu.Item>
											<DropdownMenu.Separator
												class="-mx-1 my-1 h-px bg-border"
											/>
											<DropdownMenu.Item
												variant={client.status === 'enabled'
													? 'destructive'
													: 'default'}
												onSelect={() =>
													toggleProfile(
														entrypoint.name,
														client.name,
														client.user,
														client.status === 'enabled',
														actionTriggers[
															`${entrypoint.name}/${client.name}`
														] ?? null
													)}
											>
												<PowerIcon aria-hidden="true" />{client.status ===
												'enabled'
													? copy.disable
													: copy.enable}
											</DropdownMenu.Item>
										</DropdownMenu.Content>
									</DropdownMenu.Root>
								</div>
							</div>
						{:else}<p class="p-4 text-sm text-muted-foreground">
								No users yet — select Create user to add one
							</p>
						{/each}
					</div>
				</Card.Content>
			</Card.Root>
		{/each}
	</div>
</section>

<Sheet.Root bind:open={createOpen}>
	<Sheet.Content
		id="create-user-dialog"
		class="data-[side=right]:w-full data-[side=right]:sm:max-w-lg"
		onOpenAutoFocus={(event) => {
			event.preventDefault();
			createNameInput?.focus();
		}}
		onCloseAutoFocus={(event) => {
			event.preventDefault();
			createTrigger?.focus();
		}}
	>
		<Sheet.Header class="pr-12">
			<Sheet.Title
				>{existingUser === '' ? 'Create user' : 'Create profile'}</Sheet.Title
			>
			<Sheet.Description>New user on {createOutput}</Sheet.Description>
		</Sheet.Header>
		{#if createOutput !== ''}
			<form
				class="flex min-h-0 flex-1 flex-col"
				aria-busy={busy === createOutput}
				novalidate
				onsubmit={(event) => {
					event.preventDefault();
					void createUser(createOutput);
				}}
			>
				<div class="min-h-0 flex-1 space-y-6 overflow-y-auto px-4 pb-6">
					{#if createError}
						<p
							bind:this={createErrorSummary}
							tabindex="-1"
							role="alert"
							class="rounded-lg border border-red-400/40 bg-red-400/10 p-3 text-sm text-red-300"
						>
							{createError}
						</p>
					{/if}
					<div class="grid gap-2">
						<Label for="create-profile-user">User</Label>
						<Select.Root
							type="single"
							value={existingUser || '__new__'}
							onValueChange={(value) => {
								existingUser = value === '__new__' ? '' : value;
							}}
						>
							<Select.Trigger
								bind:ref={profileUserTrigger}
								id="create-profile-user"
								class="w-full"
								><span
									>{existingUser === ''
										? 'New user'
										: users[existingUser]?.display_name || existingUser}</span
								></Select.Trigger
							>
							<Select.Content
								><Select.Item value="__new__">New user</Select.Item>
								{#each Object.entries(users) as [id, user]}
									{#if user.disabled !== true && entrypoints
											.find((item) => item.name === createOutput)
											?.clients.some((client) => client.user === id) === false}
										<Select.Item value={id}
											>{user.display_name || id}</Select.Item
										>
									{/if}
								{/each}
							</Select.Content>
						</Select.Root>
					</div>
					{#if existingUser === ''}
						<div class="grid gap-2">
							<Label for="create-user-name"
								>User name <span class="text-muted-foreground">(required)</span
								></Label
							>
							<Input
								id="create-user-name"
								class="border-muted-foreground/70"
								name="display_name"
								bind:ref={createNameInput}
								bind:value={userNames[createOutput]}
								required
								autocomplete="off"
								disabled={busy === createOutput}
								aria-invalid={createNameError !== '' ? true : undefined}
								aria-describedby={createNameError !== ''
									? 'create-user-name-error'
									: undefined}
								oninput={() => {
									createNameError = '';
								}}
							/>
							{#if createNameError}<p
									id="create-user-name-error"
									role="alert"
									class="text-sm text-red-300"
								>
									{createNameError}
								</p>{/if}
						</div>
					{/if}
					{#if existingUser === ''}
						<fieldset
							disabled={busy === createOutput}
							aria-describedby="create-upstream-help"
							class="space-y-2"
						>
							<legend class="mb-2 text-sm font-medium">Allowed upstreams</legend
							>
							<p
								id="create-upstream-help"
								class="mb-3 text-sm text-muted-foreground"
							>
								Choose the connections this user can use
							</p>
							{#each upstreamNames as upstream}
								<div
									class="flex min-h-11 items-center gap-3 rounded-lg border border-border px-3"
								>
									<Checkbox
										id={`new-user-${upstream}`}
										class="size-6 border-muted-foreground/70"
										checked={createAllowed(createOutput).includes(upstream)}
										disabled={busy === createOutput}
										onCheckedChange={(checked) =>
											changeCreateAllowed(createOutput, upstream, checked)}
									/>
									<Label
										for={`new-user-${upstream}`}
										class="flex min-h-11 flex-1 cursor-pointer items-center"
										>{upstream}</Label
									>
								</div>
							{/each}
							{#if upstreamNames.length === 0}<p
									class="text-sm text-muted-foreground"
								>
									No upstreams yet
								</p>{/if}
						</fieldset>
						<div class="grid gap-2">
							<Label for="create-user-default">Default route</Label>
							<UpstreamSelect
								id="create-user-default"
								class="border-muted-foreground/70"
								options={createAllowed(createOutput)}
								bind:value={defaultByOutput[createOutput]}
								disabled={busy === createOutput}
								aria-describedby="create-default-help"
							/>
							<p id="create-default-help" class="text-sm text-muted-foreground">
								Unavailable or restricted routes use this default; traffic is
								blocked if no allowed default is available
							</p>
						</div>
						{#if createAllowed(createOutput).length === 0}<p
								role="status"
								class="text-sm text-amber-200"
							>
								Select at least one upstream to allow network access
							</p>{/if}
					{:else}<p class="text-sm text-muted-foreground">
							This profile uses the user's existing access
						</p>{/if}
					{#if busy === createOutput}<p
							bind:this={createProgress}
							tabindex="-1"
							role="status"
							class="text-sm text-muted-foreground"
						>
							Creating profile…
						</p>{/if}
				</div>
				<div
					class="flex flex-wrap justify-end gap-2 border-t border-border p-4"
				>
					<Button
						type="button"
						variant="outline"
						onclick={() => {
							createOpen = false;
						}}>{busy === createOutput ? 'Close' : 'Cancel'}</Button
					>
					<Button type="submit" disabled={busy !== ''}
						>{busy === createOutput
							? 'Creating…'
							: existingUser === ''
								? 'Create user and config'
								: 'Create profile'}</Button
					>
				</div>
			</form>
		{/if}
	</Sheet.Content>
</Sheet.Root>

<Sheet.Root bind:open={qrOpen}>
	<Sheet.Content class="w-full sm:max-w-md">
		<Sheet.Header>
			<Sheet.Title>{copy.qr}: {qrName}</Sheet.Title>
			<Sheet.Description>Scan to connect</Sheet.Description>
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
	<Sheet.Content
		class="w-full sm:max-w-md"
		onCloseAutoFocus={(event) => {
			event.preventDefault();
			accessTrigger?.focus();
		}}
	>
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
				Restricted routes use the allowed default. Traffic is blocked if no
				allowed default is available. Uncheck all upstreams to block network
				access
			</p>
			<Button disabled={accessSaving} class="mt-6" onclick={saveAccess}
				>Save access</Button
			>
		</div></Sheet.Content
	>
</Sheet.Root>
