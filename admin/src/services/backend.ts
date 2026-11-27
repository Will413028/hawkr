const LOCAL_BACKEND = "http://localhost:8080";

export function resolveBackendBaseUrl(
	env: Record<string, string | undefined>,
): string {
	const raw = env.BACKEND_BASE_URL?.trim();
	return (raw && raw.length > 0 ? raw : LOCAL_BACKEND).replace(/\/+$/, "");
}

export const BACKEND_BASE_URL = resolveBackendBaseUrl(process.env);
