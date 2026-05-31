<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { locale } from '$lib/i18n';
	import { ListPlusIcon } from 'phosphor-svelte';

	const copy = {
		en: {
			title: 'Bypasses',
			description:
				'Bypasses decide which domains and CIDRs leave through a non-default upstream. Keep them readable: one rule should explain one routing intention.',
			add: 'Add bypass',
			name: 'Name',
			match: 'Match',
			via: 'Via',
			fallback: 'Fallback',
			priority: 'Priority'
		},
		ru: {
			title: 'Байпасы',
			description:
				'Байпасы решают, какие домены и CIDR уйдут не через маршрут по умолчанию. Держите их читаемыми: одно правило — одна понятная идея маршрутизации.',
			add: 'Добавить байпас',
			name: 'Имя',
			match: 'Матчинг',
			via: 'Через',
			fallback: 'Фолбэк',
			priority: 'Приоритет'
		}
	} as const;

	const bypasses = [
		{
			name: 'ru-direct',
			match: 'ya.ru, vk.com, gosuslugi.ru',
			via: 'direct_ru',
			fallback: '—',
			priority: 900
		},
		{
			name: 'media',
			match: 'netflix.com, nflxvideo.net',
			via: 'media_singbox',
			fallback: 'default_awg',
			priority: 850
		},
		{
			name: 'private-lab',
			match: '10.44.0.0/16',
			via: 'lab_openvpn',
			fallback: 'pending',
			priority: 800
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
		<Button href="/bypasses/new"><ListPlusIcon />{copy[$locale].add}</Button>
	</div>

	<Card.Root class="rounded-lg">
		<Card.Content>
			<div class="overflow-x-auto rounded-lg border border-border">
				<div class="grid min-w-[760px] grid-cols-[1fr_1.8fr_1fr_1fr_.7fr] bg-white/[0.035] px-4 py-2 text-xs font-medium text-muted-foreground">
					<div>{copy[$locale].name}</div>
					<div>{copy[$locale].match}</div>
					<div>{copy[$locale].via}</div>
					<div>{copy[$locale].fallback}</div>
					<div>{copy[$locale].priority}</div>
				</div>
				{#each bypasses as bypass}
					<a
						href={`/bypasses/${bypass.name}`}
						class="grid min-w-[760px] grid-cols-[1fr_1.8fr_1fr_1fr_.7fr] items-center border-t border-border px-4 py-3 text-sm transition hover:bg-white/[0.04]"
					>
						<div class="font-medium text-white">{bypass.name}</div>
						<div class="truncate font-mono text-xs text-muted-foreground">{bypass.match}</div>
						<div class="font-mono text-xs text-white/80">{bypass.via}</div>
						<div class="font-mono text-xs text-muted-foreground">{bypass.fallback}</div>
						<div class="text-muted-foreground">{bypass.priority}</div>
					</a>
				{/each}
			</div>
		</Card.Content>
	</Card.Root>
</section>
