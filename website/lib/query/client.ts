import { Code, ConnectError } from "@connectrpc/connect";
import * as Sentry from "@sentry/nextjs";
import {
	CancelledError,
	MutationCache,
	QueryCache,
	QueryClient,
} from "@tanstack/react-query";

// Connect 的錯誤中，transport 已經上報的（伺服器端的失敗）與預期中的狀態（取消、未登入、無權限）不再上報。
const CONNECT_NOT_REPORTED = new Set([
	Code.Canceled,
	Code.Unauthenticated,
	Code.PermissionDenied,
	Code.Unknown,
	Code.Internal,
	Code.DataLoss,
	Code.Unavailable,
	Code.DeadlineExceeded,
	Code.Unimplemented,
]);

/**
 * 通用 escape hatch：某些 domain 的失敗有自己的「這其實是預期狀態」判斷（例如
 * `isStoreMissing`——GetMyStore 回 not_found＝帳號還沒有店家，UI 要導去 onboarding
 * 而非顯示錯誤），但這類語意屬於該 domain，不該讓這層通用 reporter 認得個別業務碼。
 * 呼叫端用 `meta: { isExpectedError }` 掛上自己的判斷式即可跳過上報。
 */
declare module "@tanstack/react-query" {
	interface Register {
		queryMeta: { isExpectedError?: (error: unknown) => boolean };
		mutationMeta: { isExpectedError?: (error: unknown) => boolean };
	}
}

// 同 fingerprint 60s 內只報一次：backend outage 時大量 query/mutation 會同時
// 失敗，transport 的 reportApiFailure 有 30s dedupe，這層沒有的話事件量會
// 倍增（照抄 lib/api/client.ts 的 process-local map 手法）。
const DATA_LAYER_DEDUP_MS = 60_000;
const recentDataLayerErrors = new Map<string, number>();

/**
 * 全域資料層錯誤出口：queryFn/mutationFn 內的失敗若元件只渲染空態，對監控
 * 完全沉默（「沒有商品」其實是壞掉）。過濾規則：
 * - UnauthorizedError / CancelledError（instanceof）/ AbortError：401 走
 *   AUTH_EXPIRED_EVENT 事件鏈；取消是導航/teardown 的正常路徑。
 * - 帶 HTTP status 的錯誤（含 CartError 等 wrapApiError 包裝類——用 status
 *   判斷而非 error.name，包裝類才不會繞過過濾）：5xx transport 已報；
 *   401/403 是角色 gate 的預期狀態（例：純賣家帳號打 buyer-gated 端點）。
 * - ConnectError：伺服器端的失敗 lib/connect/transport.ts 已報；取消、未登入、
 *   無權限是預期狀態；其他以 Connect code 分群。
 * - `meta.isExpectedError`：domain 自己判定「這個失敗是預期狀態」時的逃生口
 *   （見上方 `Register` 擴充的註解）。
 * fingerprint 帶 status/業務 code 分群（「unkeyed」mutation 眾多，只靠 name
 * 會把全站失敗揉成一個 issue）。只 tag queryKey 首段 domain 字串（keys 工廠
 * 保證非個資），不 serialize 整個 key（後段可能含 id）。
 */
function reportDataLayerError(
	error: unknown,
	domain: string,
	kind: string,
	meta: { isExpectedError?: (error: unknown) => boolean } | undefined,
) {
	if (!(error instanceof Error)) return;
	if (error instanceof CancelledError) return;
	if (error.name === "UnauthorizedError" || error.name === "AbortError") return;
	if (meta?.isExpectedError?.(error)) return;
	if (error instanceof ConnectError) {
		if (CONNECT_NOT_REPORTED.has(error.code)) return;
		reportOnce(
			error,
			["data-layer", kind, domain, "connect", Code[error.code]],
			{
				query_domain: domain,
				data_layer_kind: kind,
			},
		);
		return;
	}
	const status = (error as { status?: number }).status;
	if (typeof status === "number") {
		if (status >= 500 || status === 401 || status === 403) return;
	}
	const code = (error as { code?: number }).code;
	const fingerprint = [
		"data-layer",
		kind,
		domain,
		typeof status === "number" ? String(status) : error.name,
		code != null ? String(code) : "no-code",
	];

	reportOnce(error, fingerprint, {
		query_domain: domain,
		data_layer_kind: kind,
	});
}

function reportOnce(
	error: Error,
	fingerprint: string[],
	tags: Record<string, string>,
) {
	const dedupeKey = fingerprint.join(":");
	const now = Date.now();
	const lastSeen = recentDataLayerErrors.get(dedupeKey);
	if (lastSeen !== undefined && now - lastSeen < DATA_LAYER_DEDUP_MS) return;
	recentDataLayerErrors.set(dedupeKey, now);
	if (recentDataLayerErrors.size > 200) {
		for (const [candidate, timestamp] of recentDataLayerErrors) {
			if (now - timestamp >= DATA_LAYER_DEDUP_MS)
				recentDataLayerErrors.delete(candidate);
		}
	}

	Sentry.captureException(error, { fingerprint, tags });
}

export function createAppQueryClient() {
	return new QueryClient({
		queryCache: new QueryCache({
			onError: (error, query) => {
				reportDataLayerError(
					error,
					String(query.queryKey[0] ?? "?"),
					"query",
					query.meta,
				);
			},
		}),
		mutationCache: new MutationCache({
			onError: (error, _variables, _context, mutation) => {
				const key = mutation.options.mutationKey?.[0];
				reportDataLayerError(
					error,
					String(key ?? "unkeyed"),
					"mutation",
					mutation.meta,
				);
			},
		}),
		defaultOptions: {
			queries: {
				refetchOnWindowFocus: false,
				// 重試只在 transport 做一層（錯誤模型 ADR 2026-09-25-api-error-model D7）：REST 由 lib/api/client.ts
				// 對 GET 的 5xx 與網路錯誤重試一次，Connect 由 lib/connect/transport.ts 依 D7 重試。
				retry: false,
				staleTime: 30_000,
			},
			mutations: {
				retry: false,
			},
		},
	});
}
