<script lang="ts">
	import { onMount } from 'svelte';
	import * as Card from '$lib/components/ui/card';
	import { locale, t } from '$lib/i18n';
	import {
		CpuIcon,
		DatabaseIcon,
		HardDrivesIcon,
		NetworkIcon,
		PlugsIcon,
		UsersIcon
	} from 'phosphor-svelte';

	type RuntimeStatus = {
		name: string;
		type: string;
		state: string;
		summary: string;
		interface?: string;
		service?: string;
	};

	type Dashboard = {
		cpu: { load_1: number; load_5: number; load_15: number; cores: number };
		memory: { total_bytes: number; available_bytes: number; used_percent: number };
		disk: { total_bytes: number; free_bytes: number; used_percent: number };
		network: { rx_bytes: number; tx_bytes: number };
		network_interfaces: number;
		route_rules: number;
		outputs: number;
		upstreams: number;
		users: number;
		runtime: RuntimeStatus[];
		services: Array<{ name: string; active: boolean; state: string }>;
	};

	const copy = {
		en: {
			subtitle: 'Live host state, configured routes, and tunnel health.',
			cpu: 'CPU load',
			memory: 'Memory',
			disk: 'Disk',
			interfaces: 'Interfaces',
			traffic: 'Network traffic',
			rules: 'Routing rules',
			upstreams: 'Upstreams',
			outputs: 'Entrypoints',
			users: 'Issued profiles',
			runtime: 'Tunnels and services',
			empty: 'No tunnels configured yet.',
			unavailable: 'Runtime metrics are unavailable while the backend is not connected.'
		},
		ru: {
			subtitle: 'Текущее состояние сервера, маршрутов и туннелей.',
			cpu: 'Нагрузка CPU',
			memory: 'Память',
			disk: 'Диск',
			interfaces: 'Интерфейсы',
			traffic: 'Сетевой трафик',
			rules: 'Правила маршрутизации',
			upstreams: 'Апстримы',
			outputs: 'Подключения',
			users: 'Выданные конфиги',
			runtime: 'Туннели и сервисы',
			empty: 'Туннели пока не настроены.',
			unavailable: 'Метрики недоступны, пока frontend не подключен к backend.'
		}
	} as const;

	let dashboard = $state<Dashboard | null>(null);
	let error = $state('');

	onMount(() => {
		void load();
		const timer = window.setInterval(load, 5000);
		return () => window.clearInterval(timer);
	});

	async function load() {
		try {
			const response = await fetch('/api/dashboard');
			if (!response.ok) throw new Error(response.statusText);
			const body = await response.json();
			dashboard = body.data as Dashboard;
			error = '';
		} catch {
			error = copy[$locale].unavailable;
		}
	}

	function percent(value = 0) {
		return `${value.toFixed(1)}%`;
	}

	function bytes(value = 0) {
		const units = ['B', 'KB', 'MB', 'GB', 'TB'];
		let result = value;
		let index = 0;
		while (result >= 1024 && index < units.length - 1) {
			result /= 1024;
			index += 1;
		}
		return `${result.toFixed(index > 1 ? 1 : 0)} ${units[index]}`;
	}

	function stateClass(state: string) {
		return state === 'healthy' || state === 'active'
			? 'text-emerald-300'
			: state === 'broken'
				? 'text-red-300'
				: 'text-amber-300';
	}
</script>

<svelte:head>
	<title>{t($locale, 'dashboard.title')} · Granger</title>
</svelte:head>

<section class="space-y-5">
	<div>
		<h1 class="font-display text-2xl font-bold text-white">{t($locale, 'dashboard.title')}</h1>
		<p class="mt-2 text-sm text-muted-foreground">{copy[$locale].subtitle}</p>
	</div>

	{#if error}
		<div class="rounded-md border border-border bg-white/[0.03] px-4 py-3 text-sm text-muted-foreground">{error}</div>
	{/if}

	<div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
		<Card.Root class="rounded-lg">
			<Card.Content class="flex items-start justify-between gap-3 p-4">
				<div><div class="text-xs text-muted-foreground">{copy[$locale].cpu}</div><div class="mt-2 text-xl font-bold text-white">{dashboard?.cpu.load_1.toFixed(2) ?? '—'}</div><div class="mt-1 text-xs text-muted-foreground">{dashboard?.cpu.cores ?? '—'} cores</div></div>
				<CpuIcon class="size-5 text-white/55" />
			</Card.Content>
		</Card.Root>
		<Card.Root class="rounded-lg">
			<Card.Content class="flex items-start justify-between gap-3 p-4">
				<div><div class="text-xs text-muted-foreground">{copy[$locale].memory}</div><div class="mt-2 text-xl font-bold text-white">{percent(dashboard?.memory.used_percent)}</div><div class="mt-1 text-xs text-muted-foreground">{bytes(dashboard?.memory.total_bytes)}</div></div>
				<DatabaseIcon class="size-5 text-white/55" />
			</Card.Content>
		</Card.Root>
		<Card.Root class="rounded-lg">
			<Card.Content class="flex items-start justify-between gap-3 p-4">
				<div><div class="text-xs text-muted-foreground">{copy[$locale].disk}</div><div class="mt-2 text-xl font-bold text-white">{percent(dashboard?.disk.used_percent)}</div><div class="mt-1 text-xs text-muted-foreground">{bytes(dashboard?.disk.total_bytes)}</div></div>
				<HardDrivesIcon class="size-5 text-white/55" />
			</Card.Content>
		</Card.Root>
		<Card.Root class="rounded-lg">
			<Card.Content class="flex items-start justify-between gap-3 p-4">
				<div><div class="text-xs text-muted-foreground">{copy[$locale].interfaces}</div><div class="mt-2 text-xl font-bold text-white">{dashboard?.network_interfaces ?? '—'}</div><div class="mt-1 text-xs text-muted-foreground">{copy[$locale].traffic}: ↓ {bytes(dashboard?.network.rx_bytes)} · ↑ {bytes(dashboard?.network.tx_bytes)}</div><div class="mt-1 text-xs text-muted-foreground">{copy[$locale].rules}: {dashboard?.route_rules ?? '—'}</div></div>
				<NetworkIcon class="size-5 text-white/55" />
			</Card.Content>
		</Card.Root>
	</div>

	<div class="grid gap-3 sm:grid-cols-3">
		{#each [
			{ label: copy[$locale].upstreams, value: dashboard?.upstreams, icon: PlugsIcon },
			{ label: copy[$locale].outputs, value: dashboard?.outputs, icon: NetworkIcon },
			{ label: copy[$locale].users, value: dashboard?.users, icon: UsersIcon }
		] as stat}
			<Card.Root class="rounded-lg">
				<Card.Content class="flex items-center justify-between p-4">
					<div><div class="text-xs text-muted-foreground">{stat.label}</div><div class="mt-1 text-xl font-bold text-white">{stat.value ?? '—'}</div></div>
					<stat.icon class="size-5 text-white/55" />
				</Card.Content>
			</Card.Root>
		{/each}
	</div>

	<Card.Root class="rounded-lg">
		<Card.Header><Card.Title>{copy[$locale].runtime}</Card.Title></Card.Header>
		<Card.Content class="space-y-2">
			{#if !dashboard || dashboard.runtime.length === 0}
				<p class="text-sm text-muted-foreground">{copy[$locale].empty}</p>
			{:else}
				{#each dashboard.runtime as item}
					<div class="grid gap-1 rounded-md border border-border bg-white/[0.025] px-3 py-3 text-sm md:grid-cols-[1fr_.7fr_.7fr_1.5fr]">
						<div class="font-medium text-white">{item.name}</div>
						<div class="text-muted-foreground">{item.type}</div>
						<div class={stateClass(item.state)}>{item.state}</div>
						<div class="text-muted-foreground">{item.summary}</div>
					</div>
				{/each}
			{/if}
			{#each dashboard?.services ?? [] as service}
				<div class="grid gap-1 rounded-md border border-border bg-white/[0.025] px-3 py-3 text-sm md:grid-cols-[1fr_.7fr_.7fr_1.5fr]">
					<div class="font-medium text-white">{service.name}</div>
					<div class="text-muted-foreground">systemd</div>
					<div class={stateClass(service.active ? 'active' : 'broken')}>{service.active ? 'active' : 'inactive'}</div>
					<div class="text-muted-foreground">{service.state}</div>
				</div>
			{/each}
		</Card.Content>
	</Card.Root>
</section>
