<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { Input } from '$lib/components/ui/input';
	import { Textarea } from '$lib/components/ui/textarea';
	import { t } from '$lib/i18n';
	import { DownloadSimpleIcon, UploadSimpleIcon } from 'phosphor-svelte';

	const copy = {
		description: 'Export the declarative Granger configuration or import a reviewed YAML file. Import creates a timestamped backup and applies the new routing state immediately.',
		exportTitle: 'Export configuration',
		exportDescription: 'Download the active /etc/granger/granger.yaml file.',
		export: 'Download YAML',
		importTitle: 'Import configuration',
		importDescription: 'Paste YAML or choose a local file. Unknown fields and invalid references are rejected before the active config is changed.',
		paste: 'Configuration YAML',
		file: 'Choose YAML file',
		import: 'Import and apply',
		confirm: 'Import this configuration and apply routing changes now?',
		success: 'Configuration imported and applied.',
		error: 'Import failed'
	} as const;

	let yaml = $state('');
	let message = $state('');
	let failed = $state(false);
	let saving = $state(false);

	async function chooseFile(event: Event) {
		const file = (event.currentTarget as HTMLInputElement).files?.[0];
		if (!file) return;
		yaml = await file.text();
		message = '';
	}

	async function importConfig() {
		if (!yaml.trim() || !confirm(copy.confirm)) return;
		saving = true;
		message = '';
		failed = false;
		try {
			const response = await fetch('/api/config/import', {
				method: 'POST',
				headers: { 'content-type': 'application/json', 'x-granger-request': '1' },
				body: JSON.stringify({ config: yaml })
			});
			const body = await response.json();
			if (!response.ok || !body.ok) throw new Error(body.error ?? 'apply failed');
			message = copy.success;
		} catch (error) {
			failed = true;
			message = `${copy.error}: ${error instanceof Error ? error.message : String(error)}`;
		} finally {
			saving = false;
		}
	}
</script>

<svelte:head>
	<title>{t('settings.title')} · Granger</title>
</svelte:head>

<section class="max-w-4xl space-y-5">
	<div>
		<h1 class="font-display text-2xl font-bold text-white">{t('settings.title')}</h1>
		<p class="mt-2 max-w-3xl text-sm text-muted-foreground">{copy.description}</p>
	</div>

	<Card.Root class="rounded-lg">
		<Card.Header>
			<Card.Title>{copy.exportTitle}</Card.Title>
			<Card.Description>{copy.exportDescription}</Card.Description>
		</Card.Header>
		<Card.Content>
			<Button href="/api/config/export" variant="outline"><DownloadSimpleIcon />{copy.export}</Button>
		</Card.Content>
	</Card.Root>

	<Card.Root class="rounded-lg">
		<Card.Header>
			<Card.Title>{copy.importTitle}</Card.Title>
			<Card.Description>{copy.importDescription}</Card.Description>
		</Card.Header>
		<Card.Content class="space-y-4">
			<label class="grid gap-1.5 text-xs text-muted-foreground">
				{copy.file}
				<Input type="file" accept=".yaml,.yml,text/yaml,application/x-yaml" onchange={chooseFile} />
			</label>
			<label class="grid gap-1.5 text-xs text-muted-foreground">
				{copy.paste}
				<Textarea bind:value={yaml} class="min-h-80 font-mono text-xs" placeholder="server:&#10;  uplink_if: eth0&#10;outputs: &#123;&#125;&#10;upstreams: &#123;&#125;&#10;rules: []" />
			</label>
			{#if message}<p class="text-sm {failed ? 'text-red-300' : 'text-emerald-300'}">{message}</p>{/if}
			<Button disabled={saving || !yaml.trim()} onclick={importConfig}><UploadSimpleIcon />{copy.import}</Button>
		</Card.Content>
	</Card.Root>
</section>
