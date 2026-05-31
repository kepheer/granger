<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { Input } from '$lib/components/ui/input';
	import { Textarea } from '$lib/components/ui/textarea';
	import { locale } from '$lib/i18n';
	import { ArrowLeftIcon, ListPlusIcon } from 'phosphor-svelte';

	const copy = {
		en: {
			title: 'Add bypass',
			description:
				'Create a domain or CIDR rule, choose the primary upstream, and optionally set a fallback for domain traffic.',
			back: 'Back',
			type: 'Rule type',
			name: 'Name',
			match: 'Domains or CIDRs',
			via: 'Primary upstream',
			fallback: 'Fallback',
			priority: 'Priority',
			create: 'Create bypass'
		},
		ru: {
			title: 'Добавить байпас',
			description:
				'Создайте доменное или CIDR-правило, выберите основной апстрим и при необходимости задайте фолбэк для доменного трафика.',
			back: 'Назад',
			type: 'Тип правила',
			name: 'Имя',
			match: 'Домены или CIDR',
			via: 'Основной апстрим',
			fallback: 'Фолбэк',
			priority: 'Приоритет',
			create: 'Создать байпас'
		}
	} as const;
</script>

<svelte:head>
	<title>{copy[$locale].title} · Granger</title>
</svelte:head>

<section class="max-w-3xl space-y-5">
	<Button href="/bypasses" variant="ghost"><ArrowLeftIcon />{copy[$locale].back}</Button>
	<div>
		<h1 class="font-display text-2xl font-bold text-white">{copy[$locale].title}</h1>
		<p class="mt-2 max-w-3xl text-sm text-muted-foreground">{copy[$locale].description}</p>
	</div>
	<Card.Root class="rounded-lg">
		<Card.Content class="space-y-5 pt-6">
			<div class="flex flex-wrap gap-2">
				<Button variant="outline" size="sm">Domains</Button>
				<Button variant="outline" size="sm">CIDRs</Button>
			</div>
			<label class="grid gap-1.5 text-xs text-muted-foreground">{copy[$locale].name}<Input placeholder="ru-direct" /></label>
			<label class="grid gap-1.5 text-xs text-muted-foreground">
				{copy[$locale].match}
				<Textarea placeholder="ya.ru&#10;vk.com&#10;10.44.0.0/16" />
			</label>
			<div class="grid gap-3 md:grid-cols-3">
				<label class="grid gap-1.5 text-xs text-muted-foreground">{copy[$locale].via}<Input placeholder="direct_ru" /></label>
				<label class="grid gap-1.5 text-xs text-muted-foreground">{copy[$locale].fallback}<Input placeholder="default_awg" /></label>
				<label class="grid gap-1.5 text-xs text-muted-foreground">{copy[$locale].priority}<Input placeholder="900" /></label>
			</div>
			<Button><ListPlusIcon />{copy[$locale].create}</Button>
		</Card.Content>
	</Card.Root>
</section>
