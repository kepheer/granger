export const messages = {
	'app.name': 'Granger',
	'nav.dashboard': 'Dashboard',
	'nav.upstreams': 'Upstreams',
	'nav.entrypoints': 'Entrypoints',
	'nav.routing': 'Routing',
	'nav.settings': 'Settings',
	'dashboard.welcome': 'Welcome to Granger',
	'dashboard.title': 'Dashboard',
	'upstreams.title': 'Upstreams',
	'entrypoints.title': 'Entrypoints',
	'routing.title': 'Routing',
	'settings.title': 'Settings',
	'app.status.ready': 'Core ready'
} as const;

export type MessageKey = keyof typeof messages;

export function t(key: MessageKey) {
	return messages[key];
}
