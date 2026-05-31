<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { locale } from '$lib/i18n';
	import { ArrowLeftIcon } from 'phosphor-svelte';

	const copy = {
		en: {
			title: 'Bypass details',
			description: 'Review matching values, routing target, fallback, and current priority.',
			back: 'Back',
			match: 'Match',
			route: 'Routing'
		},
		ru: {
			title: 'Байпас',
			description: 'Проверьте значения матчинга, целевой маршрут, фолбэк и текущий приоритет.',
			back: 'Назад',
			match: 'Матчинг',
			route: 'Маршрутизация'
		}
	} as const;
</script>

<svelte:head>
	<title>{copy[$locale].title} · Granger</title>
</svelte:head>

<section class="max-w-4xl space-y-5">
	<Button href="/bypasses" variant="ghost"><ArrowLeftIcon />{copy[$locale].back}</Button>
	<div>
		<h1 class="font-display text-2xl font-bold text-white">ru-direct</h1>
		<p class="mt-2 max-w-3xl text-sm text-muted-foreground">{copy[$locale].description}</p>
	</div>
	<div class="grid gap-4 md:grid-cols-2">
		<Card.Root class="rounded-lg">
			<Card.Header><Card.Title>{copy[$locale].match}</Card.Title></Card.Header>
			<Card.Content>
				<pre class="overflow-auto rounded-md border border-border bg-black/35 p-4 text-xs text-white/80">ya.ru
vk.com
gosuslugi.ru</pre>
			</Card.Content>
		</Card.Root>
		<Card.Root class="rounded-lg">
			<Card.Header><Card.Title>{copy[$locale].route}</Card.Title></Card.Header>
			<Card.Content class="space-y-2 text-sm text-muted-foreground">
				<div>via: <span class="font-mono text-white">direct_ru</span></div>
				<div>fallback: <span class="font-mono text-white">none</span></div>
				<div>priority: <span class="font-mono text-white">900</span></div>
			</Card.Content>
		</Card.Root>
	</div>
</section>
