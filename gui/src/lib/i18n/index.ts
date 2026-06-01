import { browser } from '$app/environment';
import { writable } from 'svelte/store';

export const locales = ['en', 'ru'] as const;
export type Locale = (typeof locales)[number];

const localeCookieName = '__gr_loc';
const localeCookieMaxAge = 60 * 60 * 24 * 365;

export const messages = {
	en: {
		'app.name': 'Granger',
		'nav.dashboard': 'Dashboard',
		'nav.upstreams': 'Upstreams',
		'nav.entrypoints': 'Entrypoints',
		'nav.users': 'Users',
		'nav.bypasses': 'Bypasses',
		'nav.rules': 'Routing',
		'dashboard.welcome': 'Welcome to Granger',
		'dashboard.title': 'Dashboard',
		'upstreams.title': 'Upstreams',
		'entrypoints.title': 'Entrypoints',
		'users.title': 'Users',
		'bypasses.title': 'Bypasses',
		'rules.title': 'Routing',
		'app.status.ready': 'Core ready',
		'language.select': 'Select language',
		'language.en': 'EN',
		'language.ru': 'RU'
	},
	ru: {
		'app.name': 'Granger',
		'nav.dashboard': 'Дашборд',
		'nav.upstreams': 'Апстримы',
		'nav.entrypoints': 'Подключения',
		'nav.users': 'Пользователи',
		'nav.bypasses': 'Байпасы',
		'nav.rules': 'Маршрутизация',
		'dashboard.welcome': 'Добро пожаловать в Granger',
		'dashboard.title': 'Дашборд',
		'upstreams.title': 'Апстримы',
		'entrypoints.title': 'Подключения',
		'users.title': 'Пользователи',
		'bypasses.title': 'Байпасы',
		'rules.title': 'Маршрутизация',
		'app.status.ready': 'Ядро готово',
		'language.select': 'Выбрать язык',
		'language.en': 'EN',
		'language.ru': 'RU'
	}
} as const;

export type MessageKey = keyof (typeof messages)['en'];

function isLocale(value: string | null): value is Locale {
	return value === 'en' || value === 'ru';
}

function readCookie(name: string) {
	const prefix = `${encodeURIComponent(name)}=`;
	return (
		document.cookie
			.split(';')
			.map((part) => part.trim())
			.find((part) => part.startsWith(prefix))
			?.slice(prefix.length) ?? null
	);
}

function writeCookie(name: string, value: string) {
	document.cookie = `${encodeURIComponent(name)}=${encodeURIComponent(value)}; path=/; max-age=${localeCookieMaxAge}; samesite=lax`;
}

function initialLocale(): Locale {
	if (!browser) return 'en';

	const saved = readCookie(localeCookieName);
	if (isLocale(saved)) return saved;

	const language = navigator.language.toLowerCase();
	return language.startsWith('ru') ? 'ru' : 'en';
}

export const locale = writable<Locale>(initialLocale());

if (browser) {
	locale.subscribe((value) => {
		writeCookie(localeCookieName, value);
		document.documentElement.lang = value;
	});
}

export function setLocale(value: Locale) {
	locale.set(value);
}

export function t(locale: Locale, key: MessageKey) {
	return messages[locale][key] ?? messages.en[key];
}
