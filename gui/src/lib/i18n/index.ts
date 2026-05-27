import { browser } from '$app/environment';
import { writable } from 'svelte/store';

export const locales = ['en', 'ru'] as const;
export type Locale = (typeof locales)[number];

const storageKey = 'granger.locale';

export const messages = {
	en: {
		'app.name': 'Granger',
		'nav.dashboard': 'Dashboard',
		'nav.upstreams': 'Upstreams',
		'nav.entrypoints': 'Entrypoints',
		'nav.bypasses': 'Bypasses',
		'dashboard.welcome': 'Welcome to Granger',
		'dashboard.title': 'Dashboard',
		'upstreams.title': 'Upstreams',
		'entrypoints.title': 'Entrypoints',
		'bypasses.title': 'Bypasses',
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
		'nav.bypasses': 'Байпасы',
		'dashboard.welcome': 'Добро пожаловать в Granger',
		'dashboard.title': 'Дашборд',
		'upstreams.title': 'Апстримы',
		'entrypoints.title': 'Подключения',
		'bypasses.title': 'Байпасы',
		'rules.title': 'Правила',
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

function initialLocale(): Locale {
	if (!browser) return 'en';

	const saved = localStorage.getItem(storageKey);
	if (isLocale(saved)) return saved;

	const language = navigator.language.toLowerCase();
	return language.startsWith('ru') ? 'ru' : 'en';
}

export const locale = writable<Locale>(initialLocale());

if (browser) {
	locale.subscribe((value) => {
		localStorage.setItem(storageKey, value);
		document.documentElement.lang = value;
	});
}

export function setLocale(value: Locale) {
	locale.set(value);
}

export function t(locale: Locale, key: MessageKey) {
	return messages[locale][key] ?? messages.en[key];
}
