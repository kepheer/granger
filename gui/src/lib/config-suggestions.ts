export type ExistingConfig = {
	outputs: Record<string, { interface?: string; subnet?: string; listen_port?: number }>;
	upstreams: Record<string, { interface?: string }>;
};

export function availableName(base: string, used: string[]) {
	if (base === '' || used.includes(base) === false) {
		return base;
	}

	const match = base.match(/^(.*?)(\d+)$/);
	const prefix = match === null ? `${base}_` : match[1];
	let index = match === null ? 2 : Number(match[2]) + 1;

	while (used.includes(`${prefix}${index}`)) {
		index += 1;
	}

	return `${prefix}${index}`;
}

export function availablePort(base: number, config: ExistingConfig) {
	const ports = Object.values(config.outputs).map((output) => output.listen_port);
	let port = base;

	while (port < 65535 && ports.includes(port)) {
		port += 1;
	}

	return port;
}

export function availableSubnet(base: string, config: ExistingConfig) {
	const subnets = Object.values(config.outputs).map((output) => output.subnet);
	const match = base.match(/^(10\.\d+\.)(\d+)(\.0\/24)$/);

	if (match === null) {
		return base;
	}

	let octet = Number(match[2]);

	while (octet < 255 && subnets.includes(`${match[1]}${octet}${match[3]}`)) {
		octet += 1;
	}

	return `${match[1]}${octet}${match[3]}`;
}

export function availableInterface(base: string, config: ExistingConfig) {
	const used = [...Object.values(config.outputs), ...Object.values(config.upstreams)]
		.map((item) => item.interface ?? '');

	return availableName(base, used);
}
