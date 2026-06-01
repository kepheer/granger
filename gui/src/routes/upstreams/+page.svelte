<script lang="ts">
	import { onMount } from 'svelte';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { locale } from '$lib/i18n';
	import { ArrowClockwiseIcon, CheckCircleIcon, PlusIcon, WarningCircleIcon } from 'phosphor-svelte';

	type ProtocolStatus = {
		name: string;
		display_name: string;
		installed: boolean;
		installable: boolean;
		state: string;
		summary: string;
	};

	type ApiResponse<T> = {
		ok: boolean;
		error?: string;
		data?: T;
	};

	const copy = {
		en: {
			title: 'Upstreams',
			description:
				'Upstreams are exits. Granger sends selected traffic through direct uplink, VPN tunnels, Xray, or sing-box according to bypass rules.',
			add: 'Add upstream',
			name: 'Name',
			type: 'Type',
			iface: 'Interface',
			config: 'Config',
			status: 'Status',
			enabled: 'enabled',
			disabled: 'disabled',
			pending: 'pending',
			hint: 'Click an upstream to inspect routes, DNS, service state, and access controls.',
			packages: 'Protocol packages',
			packagesHint:
				'Install only runtime packages here. Tunnel configs, interfaces, routes, and access rules are configured separately.',
			installed: 'Installed',
			install: 'Install',
			manual: 'Manual source required',
			unavailable: 'Installer unavailable in dev mode',
			refresh: 'Refresh'
		},
		ru: {
			title: 'Апстримы',
			description:
				'Апстримы — это выходы наружу. Granger отправляет выбранный трафик через direct uplink, VPN-туннели, Xray или sing-box по правилам байпасов.',
			add: 'Добавить апстрим',
			name: 'Имя',
			type: 'Тип',
			iface: 'Интерфейс',
			config: 'Конфиг',
			status: 'Статус',
			enabled: 'включен',
			disabled: 'выключен',
			pending: 'ожидает',
			hint: 'Нажмите на апстрим, чтобы посмотреть маршруты, DNS, состояние сервиса и управление доступом.',
			packages: 'Пакеты протоколов',
			packagesHint:
				'Здесь ставятся только runtime-пакеты. Конфиги туннелей, интерфейсы, маршруты и доступы настраиваются отдельно.',
			installed: 'Установлено',
			install: 'Установить',
			manual: 'Нужен источник',
			unavailable: 'Инсталлер недоступен в dev-режиме',
			refresh: 'Обновить'
		}
	} as const;

	let protocols = $state<ProtocolStatus[]>([
		{
			name: 'wireguard',
			display_name: 'WireGuard',
			installed: false,
			installable: true,
			state: 'unknown',
			summary: 'wireguard-tools'
		},
		{
			name: 'openvpn',
			display_name: 'OpenVPN',
			installed: false,
			installable: true,
			state: 'unknown',
			summary: 'openvpn'
		},
		{
			name: 'amneziawg',
			display_name: 'AmneziaWG',
			installed: false,
			installable: true,
			state: 'available',
			summary: 'Managed installer uses the upstream Amnezia package source.'
		},
		{
			name: 'snx-rs',
			display_name: 'SNX-RS',
			installed: false,
			installable: true,
			state: 'available',
			summary: 'Managed installer uses the upstream SNX-RS APT repository.'
		},
		{
			name: 'xray',
			display_name: 'Xray',
			installed: false,
			installable: true,
			state: 'unknown',
			summary: 'xray'
		},
		{
			name: 'sing-box',
			display_name: 'sing-box',
			installed: false,
			installable: true,
			state: 'unknown',
			summary: 'sing-box'
		}
	]);

	let protocolMessage = $state('');
	let installing = $state('');

	onMount(() => {
		void loadProtocols();
		void loadUpstreams();
	});

	async function loadProtocols() {
		try {
			const response = await fetch('/api/protocols');
			if (!response.ok) throw new Error(response.statusText);
			const body = (await response.json()) as ApiResponse<ProtocolStatus[]>;
			if (body.data) protocols = body.data;
			protocolMessage = '';
		} catch {
			protocolMessage = copy[$locale].unavailable;
		}
	}

	async function loadUpstreams() {
		try {
			const response = await fetch('/api/config');
			if (!response.ok) throw new Error(response.statusText);
			const body = await response.json();
			const configured = body.config?.upstreams ?? {};
			upstreams = Object.entries(configured).map(([name, value]) => {
				const item = value as Record<string, unknown>;
				return {
					name,
					type: String(item.type ?? ''),
					iface: String(item.interface ?? ''),
					config: String(item.config ?? 'managed configuration'),
					status: item.enabled === false ? 'disabled' : 'enabled'
				};
			});
		} catch {
			// Keep preview data while running the frontend without the Go backend.
		}
	}

	async function installProtocol(name: string) {
		installing = name;
		protocolMessage = '';
		try {
			const response = await fetch('/api/protocols/install', {
				method: 'POST',
				headers: { 'content-type': 'application/json', 'x-granger-request': '1' },
				body: JSON.stringify({ name })
			});
			const body = (await response.json()) as ApiResponse<ProtocolStatus>;
			if (body.data) {
				protocols = protocols.map((item) => (item.name === name ? body.data! : item));
			}
			protocolMessage = body.error ?? body.data?.summary ?? '';
		} catch {
			protocolMessage = copy[$locale].unavailable;
		} finally {
			installing = '';
		}
	}

	let upstreams = $state([
		{
			name: 'direct_ru',
			type: 'direct',
			iface: 'auto',
			config: 'system uplink',
			status: 'enabled'
		},
		{
			name: 'default_awg',
			type: 'amneziawg',
			iface: 'awg0',
			config: '/etc/granger/upstreams/awg0.conf',
			status: 'enabled'
		},
		{
			name: 'media_singbox',
			type: 'sing-box',
			iface: 'tun-sb0',
			config: '/etc/granger/upstreams/media.json',
			status: 'pending'
		}
	]);
</script>

<svelte:head>
	<title>{copy[$locale].title} · Granger</title>
</svelte:head>

<section class="space-y-5">
	<div class="flex flex-col justify-between gap-4 md:flex-row md:items-end">
		<div>
			<h1 class="font-display text-2xl font-bold text-white">{copy[$locale].title}</h1>
			<p class="mt-2 max-w-3xl text-sm text-muted-foreground">{copy[$locale].description}</p>
		</div>
		<Button href="/upstreams/new"><PlusIcon />{copy[$locale].add}</Button>
	</div>

	<Card.Root class="rounded-lg">
		<Card.Header class="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
			<div>
				<Card.Title>{copy[$locale].packages}</Card.Title>
				<Card.Description>{copy[$locale].packagesHint}</Card.Description>
			</div>
			<Button variant="outline" onclick={loadProtocols}><ArrowClockwiseIcon />{copy[$locale].refresh}</Button>
		</Card.Header>
		<Card.Content class="space-y-3">
			{#if protocolMessage}
				<div class="rounded-md border border-border bg-white/[0.03] px-3 py-2 text-sm text-muted-foreground">
					{protocolMessage}
				</div>
			{/if}
			<div class="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
				{#each protocols as protocol}
					<div class="rounded-lg border border-border bg-card/60 p-4">
						<div class="flex items-start justify-between gap-3">
							<div>
								<div class="font-medium text-white">{protocol.display_name}</div>
								<div class="mt-1 text-xs text-muted-foreground">{protocol.summary}</div>
							</div>
							<span
								class="inline-flex items-center gap-1 rounded-md border border-border px-2 py-1 text-xs {protocol.installed
									? 'text-emerald-300'
									: protocol.installable
										? 'text-amber-300'
										: 'text-muted-foreground'}"
							>
								{#if protocol.installed}
									<CheckCircleIcon class="size-3.5" />{copy[$locale].installed}
								{:else}
									<WarningCircleIcon class="size-3.5" />{protocol.installable
										? protocol.state
										: copy[$locale].manual}
								{/if}
							</span>
						</div>
						<div class="mt-4">
							<Button
								size="sm"
								variant={protocol.installable && !protocol.installed ? 'default' : 'outline'}
								disabled={!protocol.installable || protocol.installed || installing === protocol.name}
								onclick={() => installProtocol(protocol.name)}
							>
								{installing === protocol.name ? copy[$locale].pending : copy[$locale].install}
							</Button>
						</div>
					</div>
				{/each}
			</div>
		</Card.Content>
	</Card.Root>

	<Card.Root class="rounded-lg">
		<Card.Header>
			<Card.Title>{copy[$locale].title}</Card.Title>
			<Card.Description>{copy[$locale].hint}</Card.Description>
		</Card.Header>
		<Card.Content>
			<div class="overflow-x-auto rounded-lg border border-border">
				<div class="grid min-w-[760px] grid-cols-[1.1fr_.9fr_.9fr_1.8fr_.8fr] bg-white/[0.035] px-4 py-2 text-xs font-medium text-muted-foreground">
					<div>{copy[$locale].name}</div>
					<div>{copy[$locale].type}</div>
					<div>{copy[$locale].iface}</div>
					<div>{copy[$locale].config}</div>
					<div>{copy[$locale].status}</div>
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
									? copy[$locale].enabled
									: item.status === 'disabled'
										? copy[$locale].disabled
										: copy[$locale].pending}
							</span>
						</div>
					</a>
				{/each}
			</div>
		</Card.Content>
	</Card.Root>
</section>
