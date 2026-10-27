"use client";

import { APP_ENV } from "./app-env";

export type Environment = "local" | "dev" | "prod";

function buildMediaUrl(path: string | undefined): string {
	if (!path) return "";
	if (path.startsWith("http")) return path;
	return path.startsWith("/") ? path : `/${path}`;
}

// 同源 BFF，永遠 /api/bff（保留給既有呼叫點）。
const GATEWAY_URL = "/api/bff";

export const environmentService = {
	getEnvironment: (): Environment => APP_ENV,
	getGatewayUrl: (): string => GATEWAY_URL,
	buildMediaUrl,
};

export function useEnvironment() {
	return {
		environment: APP_ENV,
		getGatewayUrl: () => GATEWAY_URL,
		buildMediaUrl: (path: string | undefined) => buildMediaUrl(path),
	};
}
