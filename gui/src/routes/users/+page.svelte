<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { locale } from '$lib/i18n';
	import { PlusIcon, PowerIcon, UserCirclePlusIcon } from 'phosphor-svelte';

	const copy = {
		en: {
			title: 'Users',
			description:
				'Users describe people or devices that may receive entrypoint configs. Granger generates internal IDs automatically, so you only manage names and access.',
			add: 'Add user',
			name: 'Display name',
			status: 'Status',
			access: 'Entrypoint access',
			active: 'active',
			disabled: 'disabled',
			disable: 'Disable',
			enable: 'Enable'
		},
		ru: {
			title: 'Пользователи',
			description:
				'Пользователи описывают людей или устройства, которым можно выдавать конфиги подключений. Granger сам генерирует внутренние ID, а вы управляете именем и доступами.',
			add: 'Добавить пользователя',
			name: 'Отображаемое имя',
			status: 'Статус',
			access: 'Доступ к подключениям',
			active: 'активен',
			disabled: 'отключен',
			disable: 'Отключить',
			enable: 'Включить'
		}
	} as const;

	const users = [
		{ name: 'Alice', status: 'active', access: ['home_wg'] },
		{ name: 'Bob', status: 'disabled', access: ['home_wg'] },
		{ name: 'Guest access', status: 'active', access: ['guest_ovpn'] }
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
		<Button href="/users/new"><UserCirclePlusIcon />{copy[$locale].add}</Button>
	</div>

	<Card.Root class="rounded-lg">
		<Card.Content>
			<div class="overflow-x-auto rounded-lg border border-border">
				<div class="grid min-w-[680px] grid-cols-[1.2fr_.8fr_1.6fr_1fr] bg-white/[0.035] px-4 py-2 text-xs font-medium text-muted-foreground">
					<div>{copy[$locale].name}</div>
					<div>{copy[$locale].status}</div>
					<div>{copy[$locale].access}</div>
					<div></div>
				</div>
				{#each users as user}
					<div class="grid min-w-[680px] grid-cols-[1.2fr_.8fr_1.6fr_1fr] items-center border-t border-border px-4 py-3 text-sm">
						<div class="font-medium text-white">{user.name}</div>
						<div class={user.status === 'active' ? 'text-emerald-300' : 'text-red-300'}>
							{user.status === 'active' ? copy[$locale].active : copy[$locale].disabled}
						</div>
						<div class="truncate text-muted-foreground">{user.access.join(', ')}</div>
						<div class="flex justify-end">
							<Button size="xs" variant={user.status === 'active' ? 'destructive' : 'outline'}>
								<PowerIcon />{user.status === 'active' ? copy[$locale].disable : copy[$locale].enable}
							</Button>
						</div>
					</div>
				{/each}
			</div>
		</Card.Content>
	</Card.Root>
</section>
