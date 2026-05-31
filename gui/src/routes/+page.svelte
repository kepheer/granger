<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { locale, t } from '$lib/i18n';
	import { ArrowsClockwiseIcon, CheckCircleIcon, WarningCircleIcon } from 'phosphor-svelte';

	const copy = {
		en: {
			subtitle: 'Routing control plane',
			apply: 'Apply configuration',
			dryRun: 'Dry run',
			upstreams: 'Upstreams',
			entrypoints: 'Entrypoints',
			users: 'Users',
			bypasses: 'Bypasses',
			healthy: 'healthy',
			pending: 'pending',
			disabled: 'disabled'
		},
		ru: {
			subtitle: 'Панель управления маршрутами',
			apply: 'Применить конфигурацию',
			dryRun: 'Проверить план',
			upstreams: 'Апстримы',
			entrypoints: 'Подключения',
			users: 'Пользователи',
			bypasses: 'Байпасы',
			healthy: 'здорово',
			pending: 'ожидает',
			disabled: 'отключено'
		}
	} as const;

	const cards = [
		{ label: 'upstreams', value: '6', state: 'healthy' },
		{ label: 'entrypoints', value: '2', state: 'healthy' },
		{ label: 'users', value: '3', state: 'pending' },
		{ label: 'bypasses', value: '12', state: 'healthy' }
	] as const;
</script>

<svelte:head>
	<title>{t($locale, 'app.name')}</title>
</svelte:head>

<section class="space-y-5">
	<div class="flex flex-col justify-between gap-4 md:flex-row md:items-end">
		<div>
			<h1 class="font-display text-2xl font-bold text-white">Granger</h1>
			<p class="mt-2 max-w-2xl text-sm text-muted-foreground">{copy[$locale].subtitle}</p>
		</div>
		<div class="flex gap-2">
			<Button variant="outline"><WarningCircleIcon />{copy[$locale].dryRun}</Button>
			<Button><ArrowsClockwiseIcon />{copy[$locale].apply}</Button>
		</div>
	</div>

	<div class="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
		{#each cards as card}
			<Card.Root class="rounded-lg">
				<Card.Header>
					<Card.Description>{copy[$locale][card.label]}</Card.Description>
					<Card.Title class="text-3xl">{card.value}</Card.Title>
				</Card.Header>
				<Card.Content>
					<div class="flex items-center gap-2 text-sm {card.state === 'healthy' ? 'text-emerald-300' : 'text-amber-300'}">
						<CheckCircleIcon size={16} />
						<span>{card.state === 'healthy' ? copy[$locale].healthy : copy[$locale].pending}</span>
					</div>
				</Card.Content>
			</Card.Root>
		{/each}
	</div>

	<Card.Root class="rounded-lg">
		<Card.Header>
			<Card.Title>{copy[$locale].subtitle}</Card.Title>
			<Card.Description>upstreams -> rules/bypasses -> entrypoints -> users</Card.Description>
		</Card.Header>
		<Card.Content>
			<div class="grid gap-3 md:grid-cols-4">
				<a class="rounded-lg border border-border bg-white/[0.03] p-4 transition hover:bg-white/[0.06]" href="/upstreams">{copy[$locale].upstreams}</a>
				<a class="rounded-lg border border-border bg-white/[0.03] p-4 transition hover:bg-white/[0.06]" href="/entrypoints">{copy[$locale].entrypoints}</a>
				<a class="rounded-lg border border-border bg-white/[0.03] p-4 transition hover:bg-white/[0.06]" href="/users">{copy[$locale].users}</a>
				<a class="rounded-lg border border-border bg-white/[0.03] p-4 transition hover:bg-white/[0.06]" href="/bypasses">{copy[$locale].bypasses}</a>
			</div>
		</Card.Content>
	</Card.Root>
</section>
