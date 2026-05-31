<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { locale } from '$lib/i18n';
	import { ArrowLeftIcon, PowerIcon } from 'phosphor-svelte';

	const copy = {
		en: {
			title: 'Upstream details',
			description:
				'Inspect how this upstream is represented in Granger: config path, service state, interface, and whether routing through it is enabled.',
			back: 'Back',
			enable: 'Enable upstream',
			disable: 'Disable upstream',
			config: 'Config',
			routes: 'Routes',
			dns: 'DNS mode',
			service: 'Service'
		},
		ru: {
			title: 'Апстрим',
			description:
				'Здесь видно, как апстрим представлен в Granger: путь конфига, состояние сервиса, интерфейс и включена ли маршрутизация через него.',
			back: 'Назад',
			enable: 'Включить апстрим',
			disable: 'Выключить апстрим',
			config: 'Конфиг',
			routes: 'Маршруты',
			dns: 'DNS-режим',
			service: 'Сервис'
		}
	} as const;
</script>

<svelte:head>
	<title>{copy[$locale].title} · Granger</title>
</svelte:head>

<section class="max-w-5xl space-y-5">
	<Button href="/upstreams" variant="ghost"><ArrowLeftIcon />{copy[$locale].back}</Button>

	<div class="flex flex-col justify-between gap-4 md:flex-row md:items-end">
		<div>
			<h1 class="font-display text-2xl font-bold text-white">default_awg</h1>
			<p class="mt-2 max-w-3xl text-sm text-muted-foreground">{copy[$locale].description}</p>
		</div>
		<Button variant="destructive"><PowerIcon />{copy[$locale].disable}</Button>
	</div>

	<div class="grid gap-4 md:grid-cols-2">
		<Card.Root class="rounded-lg">
			<Card.Header><Card.Title>{copy[$locale].config}</Card.Title></Card.Header>
			<Card.Content class="space-y-2 text-sm text-muted-foreground">
				<div>type: <span class="font-mono text-white">amneziawg</span></div>
				<div>interface: <span class="font-mono text-white">awg0</span></div>
				<div>path: <span class="font-mono text-white">/etc/granger/upstreams/awg0.conf</span></div>
			</Card.Content>
		</Card.Root>

		<Card.Root class="rounded-lg">
			<Card.Header><Card.Title>{copy[$locale].service}</Card.Title></Card.Header>
			<Card.Content class="space-y-2 text-sm text-muted-foreground">
				<div>state: <span class="text-emerald-300">enabled</span></div>
				<div>runtime: <span class="text-emerald-300">up</span></div>
				<div>traffic: <span class="font-mono text-white">default route</span></div>
			</Card.Content>
		</Card.Root>
	</div>

	<Card.Root class="rounded-lg">
		<Card.Header><Card.Title>{copy[$locale].routes}</Card.Title></Card.Header>
		<Card.Content>
			<pre class="overflow-auto rounded-md border border-border bg-black/35 p-4 text-xs text-white/80">default dev awg0 table granger_default
10.66.66.0/24 dev wg0 table granger_default</pre>
		</Card.Content>
	</Card.Root>
</section>
