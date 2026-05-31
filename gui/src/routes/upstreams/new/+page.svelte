<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { Input } from '$lib/components/ui/input';
	import { Textarea } from '$lib/components/ui/textarea';
	import { locale } from '$lib/i18n';
	import { ArrowLeftIcon, UploadSimpleIcon } from 'phosphor-svelte';

	const copy = {
		en: {
			title: 'Add upstream',
			description:
				'Choose a protocol, keep the suggested interface unless you need a specific name, then paste or upload the provider config. Granger will place it under the upstream config directory.',
			back: 'Back',
			type: 'Protocol',
			name: 'Name',
			iface: 'Interface',
			config: 'Inline config',
			upload: 'Upload config',
			create: 'Create upstream',
			suggested: 'Suggested defaults'
		},
		ru: {
			title: 'Добавить апстрим',
			description:
				'Выберите протокол, оставьте предложенный интерфейс, если не нужен особый нейминг, затем вставьте или загрузите конфиг провайдера. Granger сам положит его в каталог конфигов апстримов.',
			back: 'Назад',
			type: 'Протокол',
			name: 'Имя',
			iface: 'Интерфейс',
			config: 'Конфиг инлайном',
			upload: 'Загрузить конфиг',
			create: 'Создать апстрим',
			suggested: 'Предложенные дефолты'
		}
	} as const;

	const protocols = [
		{ label: 'WireGuard', name: 'wg_up_1', iface: 'wg-up0' },
		{ label: 'AmneziaWG', name: 'awg_default', iface: 'awg0' },
		{ label: 'OpenVPN', name: 'ovpn_exit', iface: 'tun0' },
		{ label: 'Xray', name: 'xray_exit', iface: 'tun-xray0' },
		{ label: 'sing-box', name: 'singbox_exit', iface: 'tun-sb0' },
		{ label: 'Existing interface', name: 'direct_uplink', iface: 'eth0' }
	];
</script>

<svelte:head>
	<title>{copy[$locale].title} · Granger</title>
</svelte:head>

<section class="max-w-4xl space-y-5">
	<Button href="/upstreams" variant="ghost"><ArrowLeftIcon />{copy[$locale].back}</Button>

	<div>
		<h1 class="font-display text-2xl font-bold text-white">{copy[$locale].title}</h1>
		<p class="mt-2 max-w-3xl text-sm text-muted-foreground">{copy[$locale].description}</p>
	</div>

	<Card.Root class="rounded-lg">
		<Card.Header>
			<Card.Title>{copy[$locale].suggested}</Card.Title>
		</Card.Header>
		<Card.Content class="space-y-5">
			<div class="flex flex-wrap gap-2">
				{#each protocols as protocol}
					<Button variant="outline" size="sm">{protocol.label}</Button>
				{/each}
			</div>

			<div class="grid gap-3 md:grid-cols-2">
				<label class="grid gap-1.5 text-xs text-muted-foreground">
					{copy[$locale].name}
					<Input placeholder="awg_default" />
				</label>
				<label class="grid gap-1.5 text-xs text-muted-foreground">
					{copy[$locale].iface}
					<Input placeholder="awg0" />
				</label>
			</div>

			<label class="grid gap-1.5 text-xs text-muted-foreground">
				{copy[$locale].config}
				<Textarea class="min-h-48 font-mono text-xs" placeholder="[Interface]&#10;PrivateKey = ..." />
			</label>

			<label class="grid gap-1.5 text-xs text-muted-foreground">
				{copy[$locale].upload}
				<Input type="file" />
			</label>

			<Button><UploadSimpleIcon />{copy[$locale].create}</Button>
		</Card.Content>
	</Card.Root>
</section>
