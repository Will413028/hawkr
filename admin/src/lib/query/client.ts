import { ConnectError } from "@connectrpc/connect";
import { QueryClient } from "@tanstack/react-query";

/**
 * 每種錯誤只在一層重試（錯誤模型 ADR 2026-09-25-api-error-model D7）：Connect 的呼叫由 transport 依 D7 重試
 * （lib/connect/transport.ts），query 層不再重試；舊的 REST（services/api.ts）沒有 transport 的重試，維持
 * query 層的一次。
 */
export function shouldRetryQuery(failureCount: number, error: unknown) {
	return !(error instanceof ConnectError) && failureCount < 1;
}

export function createAppQueryClient() {
	return new QueryClient({
		defaultOptions: {
			queries: {
				refetchOnWindowFocus: false,
				retry: shouldRetryQuery,
				staleTime: 30_000,
			},
			mutations: {
				retry: false,
			},
		},
	});
}
