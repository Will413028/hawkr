import {
	ConnectError,
	type Interceptor,
	type Transport,
} from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { classifyError, factsFromError } from "@hawkr/api/connect/errors";
import { retryInterceptor } from "@hawkr/api/connect/retry";


/** 同 services/api.ts 的逾時。 */
const TIMEOUT_MS = 10_000;

export interface AdminTransportOptions {
	/** 呼叫回 unauthenticated（後端不再認得這位管理員）時登出 Access；見 sessionInterceptor。 */
	onUnauthenticated?: () => void;
}

export function adminTransport(opts: AdminTransportOptions = {}): Transport {
	const interceptors: Interceptor[] = [retryInterceptor()];
	if (opts.onUnauthenticated) {
		interceptors.unshift(sessionInterceptor(opts.onUnauthenticated));
	}
	return createConnectTransport({
		baseUrl: "/api/bff",
		useHttpGet: true,
		defaultTimeoutMs: TIMEOUT_MS,
		interceptors,
	});
}

/**
 * 錯誤依 classifyError 是 reauthenticate（unauthenticated）時呼叫 onUnauthenticated。錯誤照樣丟給呼叫端。
 * 串流在建立之後的錯誤不經過這裡。
 */
export function sessionInterceptor(onUnauthenticated: () => void): Interceptor {
	return (next) => async (req) => {
		try {
			return await next(req);
		} catch (raw) {
			const err = ConnectError.from(raw);
			const message = req.stream ? undefined : req.message;
			const d = classifyError(factsFromError(err, req.method, message));
			if (d.action === "reauthenticate") onUnauthenticated();
			throw raw;
		}
	};
}
