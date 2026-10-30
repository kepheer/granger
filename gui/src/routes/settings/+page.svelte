<script lang="ts">
	import ConfirmAction from '$lib/components/confirm-action.svelte';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { Input } from '$lib/components/ui/input';
	import { Textarea } from '$lib/components/ui/textarea';
	import { t } from '$lib/i18n';
	import { DownloadSimpleIcon, UploadSimpleIcon } from 'phosphor-svelte';

	const copy = {
		description: 'Download or restore your Granger settings',
		exportTitle: 'Export configuration',
		exportDescription: 'Save the current settings as a YAML file',
		export: 'Download YAML',
		importTitle: 'Import configuration',
		importDescription: 'Paste your settings or choose a YAML file',
		paste: 'Configuration YAML',
		file: 'Choose YAML file',
		import: 'Import and apply',
		confirm: 'Import this configuration and apply routing changes now?',
		success: 'Settings imported and applied',
		error: 'Import failed'
	} as const;

	let yaml = $state('');
	let message = $state('');
	let failed = $state(false);
	let saving = $state(false);
	let confirmOpen = $state(false);

	async function chooseFile(event: Event) {
		const file = (event.currentTarget as HTMLInputElement).files?.[0];
		if (file === undefined || file === null) {
			return;
		}
		yaml = await file.text();
		message = '';
	}

	async function importConfig() {
		if (yaml.trim() === '' || saving) {
			return;
		}
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
			if (response.ok === false || body.ok !== true) {
				throw new Error(body.error ?? 'apply failed');
			}
			message = copy.success;
			confirmOpen = false;
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

<section class="w-full min-w-0 space-y-5">
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
				<Textarea bind:value={yaml} class="min-h-80 font-mono text-xs" placeholder={'server:\n  uplink_if: eth0\noutputs: {}\nupstreams: {}\nrules: []'} />
			</label>
			{#if message}<p role={failed ? 'alert' : 'status'} class="text-sm {failed ? 'text-red-300' : 'text-emerald-300'}">{message}</p>{/if}
			<Button disabled={saving || yaml.trim() === ''} onclick={() => { message = ''; confirmOpen = true; }}><UploadSimpleIcon />{copy.import}</Button>
		</Card.Content>
	</Card.Root>
</section>

<ConfirmAction bind:open={confirmOpen} title="Import configuration?" description="Replace the current settings and apply routes now, keeping a backup of the current settings" action="Import and apply" busy={saving} error={failed ? message : ''} onconfirm={importConfig} />
