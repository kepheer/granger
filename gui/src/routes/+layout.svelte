<script lang="ts">
	import { onMount } from 'svelte';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import * as Sidebar from '$lib/components/ui/sidebar';
	import { t, type MessageKey } from '$lib/i18n';
	import { page } from '$app/state';
	import {
		CompassIcon,
		PlugsIcon,
		SignpostIcon,
		GitBranchIcon,
		GearSixIcon
	} from 'phosphor-svelte';
	import '../app.css';
	import favicon from '$lib/assets/favicon.svg';

	let { children } = $props();
	let authenticated = $state(false);
	let checking = $state(true);
	let token = $state('');
	let loginError = $state('');
	let loggingIn = $state(false);

	onMount(async () => {
		try {
			const response = await fetch('/api/config');
			authenticated = response.ok;
		} catch (error) {
			loginError = error instanceof Error ? error.message : String(error);
		} finally {
			checking = false;
		}
	});

	async function login() {
		if (loggingIn || token.trim() === '') {
			return;
		}
		loggingIn = true;
		loginError = '';
		try {
			const response = await fetch('/api/session', {
				method: 'POST',
				headers: {
					'content-type': 'application/json',
					'x-granger-request': '1'
				},
				body: JSON.stringify({ token })
			});
			const body = await response.json();
			if (response.ok === false || body.ok !== true) {
				throw new Error(body.error ?? 'Could not sign in');
			}
			token = '';
			authenticated = true;
		} catch (error) {
			loginError = error instanceof Error ? error.message : String(error);
		} finally {
			loggingIn = false;
		}
	}

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
			labelKey: 'nav.routing',
			titleKey: 'routing.title',
			href: '/routing',
			icon: GitBranchIcon
		},
		{
			labelKey: 'nav.settings',
			titleKey: 'settings.title',
			href: '/settings',
			icon: GearSixIcon
		}
	] as const;

	function isActive(href: string, pathname: string) {
		return href === '/' ? pathname === '/' : pathname.startsWith(href);
	}

	function titleKey(pathname: string): MessageKey {
		return (
			menu.find((item) => isActive(item.href, pathname))?.titleKey ?? 'app.name'
		);
	}
</script>

<svelte:head>
	{#if authenticated === false}<title>Sign in · Granger</title>{/if}
	<link rel="icon" href={favicon} />
</svelte:head>

<div
	class="dark min-h-screen bg-background text-foreground selection:bg-white/20 selection:text-white"
>
	<div
		class="pointer-events-none fixed inset-0 bg-[linear-gradient(rgba(255,255,255,.026)_1px,transparent_1px),linear-gradient(90deg,rgba(255,255,255,.02)_1px,transparent_1px)] bg-[size:32px_32px]"
	></div>
	<div class="pointer-events-none fixed inset-x-0 top-0 h-px bg-white/18"></div>

	{#if checking}
		<main class="relative mx-auto max-w-md px-6 py-20" aria-busy="true">
			Connecting to Granger…
		</main>
	{:else if authenticated === false}
		<main class="relative mx-auto max-w-md space-y-5 px-6 py-20">
			<h1 class="font-display text-3xl font-bold">Granger</h1>
			<p class="text-sm text-muted-foreground">
				Enter your admin token to sign in
			</p>
			<form
				class="space-y-4"
				onsubmit={(event) => {
					event.preventDefault();
					void login();
				}}
			>
				<label class="grid gap-2 text-sm" for="admin-token">
					Admin token
					<Input
						id="admin-token"
						name="admin-token"
						type="password"
						autocomplete="current-password"
						required
						aria-invalid={loginError !== ''}
						aria-describedby={loginError !== '' ? 'login-error' : undefined}
						bind:value={token}
					/>
				</label>
				{#if loginError}<p
						id="login-error"
						class="text-sm text-red-300"
						role="alert"
					>
						{loginError}
					</p>{/if}
				<Button type="submit" disabled={loggingIn} aria-busy={loggingIn}
					>{loggingIn ? 'Signing in…' : 'Sign in'}</Button
				>
			</form>
		</main>
	{:else}
		<a
			href="#main-content"
			class="fixed top-3 left-3 z-50 -translate-y-24 rounded-md bg-white px-4 py-3 text-black focus:translate-y-0"
			>Skip to main content</a
		>
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
							<nav aria-label="Main navigation">
								<Sidebar.Menu>
									{#each menu as item}
										<Sidebar.MenuItem>
											<Sidebar.MenuButton
												isActive={isActive(item.href, page.url.pathname)}
												tooltipContent={t(item.labelKey)}
												class="h-9 rounded-md text-[13px] text-sidebar-foreground/72 hover:bg-sidebar-accent/80 hover:text-sidebar-accent-foreground data-active:bg-sidebar-accent data-active:text-sidebar-accent-foreground data-active:shadow-[inset_0_0_0_1px_var(--sidebar-border)]"
											>
												{#snippet child({ props })}
													<a
														href={item.href}
														aria-current={isActive(item.href, page.url.pathname)
															? 'page'
															: undefined}
														{...props}
														type={undefined}
													>
														<item.icon
															size={16}
															weight="bold"
															class="text-current/70"
														/>
														<span>{t(item.labelKey)}</span>
													</a>
												{/snippet}
											</Sidebar.MenuButton>
										</Sidebar.MenuItem>
									{/each}
								</Sidebar.Menu>
							</nav>
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
							{t(titleKey(page.url.pathname))}
						</div>
					</div>

					<div class="flex shrink-0 items-center gap-3 md:gap-4">
						<div class="hidden items-center gap-2 sm:flex">
							<div
								class="h-2 w-2 rounded-full bg-white/40"
							></div>
							<span class="text-[12px] text-white/58"
								>Admin</span
							>
						</div>
					</div>
				</header>

				<main
					id="main-content"
					tabindex="-1"
					class="min-h-[calc(100vh-60px)] min-w-0 p-4 sm:p-6"
				>
					{@render children()}
				</main>
			</Sidebar.Inset>
		</Sidebar.Provider>
	{/if}
</div>
