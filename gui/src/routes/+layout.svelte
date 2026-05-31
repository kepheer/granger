<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Sidebar from '$lib/components/ui/sidebar';
	import {
		locale,
		locales,
		setLocale,
		t,
		type Locale,
		type MessageKey
	} from '$lib/i18n';
	import { page } from '$app/state';
	import {
		CompassIcon,
		PlugsIcon,
		SignpostIcon,
		TrafficSignIcon,
		UsersIcon
	} from 'phosphor-svelte';
	import '../app.css';
	import favicon from '$lib/assets/favicon.svg';

	let { children } = $props();

	const menu = [
		{
			labelKey: 'nav.dashboard',
			titleKey: 'dashboard.title',
			href: '/',
			icon: CompassIcon
		},
		{
			labelKey: 'nav.upstreams',
			titleKey: 'upstreams.title',
			href: '/upstreams',
			icon: PlugsIcon
		},
		{
			labelKey: 'nav.entrypoints',
			titleKey: 'entrypoints.title',
			href: '/entrypoints',
			icon: SignpostIcon
		},
		{
			labelKey: 'nav.users',
			titleKey: 'users.title',
			href: '/users',
			icon: UsersIcon
		},
		{
			labelKey: 'nav.bypasses',
			titleKey: 'bypasses.title',
			href: '/bypasses',
			icon: TrafficSignIcon
		},
		{
			labelKey: 'nav.rules',
			titleKey: 'rules.title',
			href: '/rules',
			icon: TrafficSignIcon
		}
	] as const;

	function isActive(href: string, pathname: string) {
		return href === '/' ? pathname === '/' : pathname.startsWith(href);
	}

	function titleKey(pathname: string): MessageKey {
		return menu.find((item) => isActive(item.href, pathname))?.titleKey ?? 'app.name';
	}
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
</svelte:head>

<div
	class="dark min-h-screen bg-background text-foreground selection:bg-white/20 selection:text-white"
>
	<div
		class="pointer-events-none fixed inset-0 bg-[linear-gradient(rgba(255,255,255,.026)_1px,transparent_1px),linear-gradient(90deg,rgba(255,255,255,.02)_1px,transparent_1px)] bg-[size:32px_32px]"
	></div>
	<div class="pointer-events-none fixed inset-x-0 top-0 h-px bg-white/18"></div>

	<Sidebar.Provider>
		<Sidebar.Root
			collapsible="offcanvas"
			class="border-border bg-sidebar/95 shadow-[8px_0_40px_rgba(0,0,0,.32)]"
		>
			<Sidebar.Header
				class="h-[60px] justify-center border-b border-border px-5"
			>
				<div
					class="font-display truncate text-[16px] font-bold tracking-normal text-white"
				>
					Granger
				</div>
			</Sidebar.Header>

			<Sidebar.Content class="px-2 py-3">
				<Sidebar.Group>
					<Sidebar.GroupContent>
						<Sidebar.Menu>
							{#each menu as item}
								<Sidebar.MenuItem>
									<Sidebar.MenuButton
										isActive={isActive(item.href, page.url.pathname)}
										tooltipContent={t($locale, item.labelKey)}
										class="h-9 rounded-md text-[13px] text-sidebar-foreground/72 hover:bg-sidebar-accent/80 hover:text-sidebar-accent-foreground data-active:bg-sidebar-accent data-active:text-sidebar-accent-foreground data-active:shadow-[inset_0_0_0_1px_var(--sidebar-border)]"
									>
										{#snippet child({ props })}
											<a
												href={item.href}
												aria-current={isActive(item.href, page.url.pathname)
													? 'page'
													: undefined}
												{...props}
											>
												<item.icon
													size={16}
													weight="bold"
													class="text-current/70"
												/>
												<span>{t($locale, item.labelKey)}</span>
											</a>
										{/snippet}
									</Sidebar.MenuButton>
								</Sidebar.MenuItem>
							{/each}
						</Sidebar.Menu>
					</Sidebar.GroupContent>
				</Sidebar.Group>
			</Sidebar.Content>

			<Sidebar.Rail />
		</Sidebar.Root>

		<Sidebar.Inset class="min-w-0 bg-background/94">
			<header
				class="flex h-[60px] items-center justify-between border-b border-border bg-card/70 px-6 backdrop-blur"
			>
				<div class="flex min-w-0 items-center gap-3">
					<Sidebar.Trigger class="-ml-2 md:hidden" />
					<div
						class="font-display truncate text-[15px] font-bold tracking-normal text-white md:hidden"
					>
						Granger
					</div>
					<div class="hidden h-4 w-px bg-border md:block"></div>
					<div class="text-[13px] font-bold text-white">
						{t($locale, titleKey(page.url.pathname))}
					</div>
				</div>

				<div class="flex shrink-0 items-center gap-3 md:gap-4">
					<div
						class="flex rounded-md border border-border bg-white/[0.04] p-0.5"
						aria-label={t($locale, 'language.select')}
					>
						{#each locales as code}
							<Button
								variant={$locale === code ? 'default' : 'ghost'}
								size="xs"
								class="h-6 rounded-[5px] px-2 text-[11px]"
								onclick={() => setLocale(code as Locale)}
							>
								{t($locale, `language.${code}`)}
							</Button>
						{/each}
					</div>

					<div class="hidden items-center gap-2 sm:flex">
						<div
							class="h-2 w-2 rounded-full bg-[#74f2a1] shadow-[0_0_12px_rgba(116,242,161,.58)]"
						></div>
						<span class="text-[12px] text-white/58"
							>{t($locale, 'app.status.ready')}</span
						>
					</div>
				</div>
			</header>

			<main class="min-h-[calc(100vh-60px)] p-6">
				{@render children()}
			</main>
		</Sidebar.Inset>
	</Sidebar.Provider>
</div>
