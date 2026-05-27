<script lang="ts">
	import { locale, t } from '$lib/i18n';
</script>

<svelte:head>
	<title>{t($locale, 'bypasses.title')} · Granger</title>
</svelte:head>

<section class="grid min-h-[calc(100vh-108px)] place-items-center">
	<h1
		class="font-display text-center text-[42px] leading-none font-bold tracking-normal text-white"
	>
		{t($locale, 'bypasses.title')}
	</h1>
</section>
