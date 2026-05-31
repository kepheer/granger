<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { locale } from '$lib/i18n';
	import {
		DownloadSimpleIcon,
		PlusIcon,
		QrCodeIcon,
		CopyIcon
	} from 'phosphor-svelte';

	const copy = {
		en: {
			title: 'Entrypoints',
			description:
				'Entrypoints are client-facing connection profiles. Granger issues configs, controls access by user, and keeps protocol defaults friendly.',
			add: 'Add entrypoint',
			name: 'Name',
			type: 'Type',
			iface: 'Interface',
			subnet: 'Subnet',
			port: 'Port',
			clients: 'Clients',
			status: 'Status',
			actions: 'Config',
			enabled: 'enabled',
			disabled: 'disabled',
			download: 'Download',
			copy: 'Copy',
			qr: 'QR code'
		},
		ru: {
			title: 'Подключения',
			description:
				'Подключения — это клиентские входы. Granger выдает конфиги, управляет доступом пользователей и сам предлагает безопасные дефолты протокола.',
			add: 'Добавить подключение',
			name: 'Имя',
			type: 'Тип',
			iface: 'Интерфейс',
			subnet: 'Подсеть',
			port: 'Порт',
			clients: 'Клиенты',
			status: 'Статус',
			actions: 'Конфиг',
			enabled: 'включен',
			disabled: 'отключен',
			download: 'Скачать',
			copy: 'Копировать',
			qr: 'QR-код'
		}
	} as const;

	const entrypoints = [
		{
			name: 'home_wg',
			type: 'wireguard',
			iface: 'wg0',
			subnet: '10.66.66.0/24',
			port: '51820',
			clients: [
				{ name: 'alice-phone', user: 'Alice', ip: '10.66.66.2', status: 'enabled' },
				{ name: 'bob-laptop', user: 'Bob', ip: '10.66.66.3', status: 'disabled' }
			]
		},
		{
			name: 'guest_ovpn',
			type: 'openvpn',
			iface: 'tun0',
			subnet: '10.77.0.0/24',
			port: '1194',
			clients: [{ name: 'guest-profile', user: 'Guest access', ip: '10.77.0.2', status: 'enabled' }]
		}
	];
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
		<Button href="/entrypoints/new"><PlusIcon />{copy[$locale].add}</Button>
	</div>

	<div class="space-y-4">
		{#each entrypoints as entrypoint}
			<Card.Root class="rounded-lg">
				<Card.Header class="flex-row items-start justify-between gap-4">
					<div>
						<Card.Title>{entrypoint.name}</Card.Title>
						<Card.Description>{entrypoint.type} · {entrypoint.iface} · {entrypoint.subnet}</Card.Description>
					</div>
					<div class="rounded-full border border-border px-3 py-1 text-xs text-muted-foreground">
						{copy[$locale].port} {entrypoint.port}
					</div>
				</Card.Header>
				<Card.Content>
					<div class="overflow-x-auto rounded-lg border border-border">
						<div
							class="grid min-w-[900px] grid-cols-[1fr_.75fr_.7fr_.7fr_1.9fr] bg-white/[0.035] px-4 py-2 text-xs font-medium text-muted-foreground"
						>
							<div>{copy[$locale].name}</div>
							<div>{copy[$locale].clients}</div>
							<div>IP</div>
							<div>{copy[$locale].status}</div>
							<div>{copy[$locale].actions}</div>
						</div>
						{#each entrypoint.clients as client}
							<div
								class="grid min-w-[900px] grid-cols-[1fr_.75fr_.7fr_.7fr_1.9fr] items-center border-t border-border px-4 py-3 text-sm"
							>
								<div class="font-medium text-white">{client.name}</div>
								<div class="text-muted-foreground">{client.user}</div>
								<div class="font-mono text-xs text-white/80">{client.ip}</div>
								<div class={client.status === 'enabled' ? 'text-emerald-300' : 'text-red-300'}>
									{client.status === 'enabled' ? copy[$locale].enabled : copy[$locale].disabled}
								</div>
								<div class="flex flex-wrap gap-1">
									<Button size="xs" variant="ghost"><DownloadSimpleIcon />{copy[$locale].download}</Button>
									<Button size="xs" variant="ghost"><CopyIcon />{copy[$locale].copy}</Button>
									<Button size="xs" variant="outline"><QrCodeIcon />{copy[$locale].qr}</Button>
								</div>
							</div>
						{/each}
					</div>
				</Card.Content>
			</Card.Root>
		{/each}
	</div>
</section>
