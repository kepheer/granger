export type ModuleDescriptor = {
	id: string;
	label: string;
	upstream: boolean;
	entrypoint: boolean;
	interface: string;
	upstream_interface: string;
	subnet: string;
	port: number;
	managed_protocol?: string;
};
