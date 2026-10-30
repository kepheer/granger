const states: Record<string, string> = {
	healthy: 'Connected',
	active: 'Running',
	inactive: 'Stopped',
	disabled: 'Disabled',
	broken: 'Connection failed',
	degraded: 'Needs attention',
	pending: 'Connecting',
	unknown: 'Unknown',
	installed: 'Installed',
	available: 'Not installed',
	failed: 'Failed'
};

export function stateLabel(value: string) {
	return states[value] ?? value;
}
