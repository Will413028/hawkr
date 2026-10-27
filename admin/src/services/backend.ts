// Server-only. 後端對外的入口（單體）：Connect 的方法與還沒搬進單體的 REST（/api/v1，由單體轉給舊的 gateway）都打這裡。
// 每個 deployment 綁一個固定的後端，由部署設定的 `BACKEND_BASE_URL` 注入；沒設時是本機的單體（backend 的
// `make serve`，預設聽 :8080）。
const LOCAL_BACKEND = "http://localhost:8080";

export function resolveBackendBaseUrl(
	env: Record<string, string | undefined>,
): string {
	const raw = env.BACKEND_BASE_URL?.trim();
	return (raw && raw.length > 0 ? raw : LOCAL_BACKEND).replace(/\/+$/, "");
}

export const BACKEND_BASE_URL = resolveBackendBaseUrl(process.env);
