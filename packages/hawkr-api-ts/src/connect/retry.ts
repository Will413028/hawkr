import { ConnectError, type Interceptor } from "@connectrpc/connect";
import { classifyError, factsFromError } from "./errors";

export interface RetryOptions {
	/**
	 * server 端（Cloudflare Workers）：不排 timer，只立即重試一次；需要等待的 RetryInfo 不重試。
	 * 不給時是瀏覽器的行為：最多重試兩次，退避等待。
	 */
	server?: boolean;
	sleep?: (ms: number) => Promise<void>;
	jitter?: () => number;
}

/**
 * unary 呼叫依 classifyError（錯誤模型 ADR 2026-09-25-api-error-model D7）自動重試。預設（瀏覽器）最多重試兩次，
 * 200ms 起每次加倍、上限 2s，加上 0 到同樣長度的 jitter，伺服器帶了 RetryInfo 時照它。server 端只立即重試一次、
 * 不等待：在串流的 Suspense 裡排的 setTimeout 可能在 Workers 收掉這個請求之後才觸發（website lib/api/client.ts
 * 的 B-199），所以需要等待的 RetryInfo 不在 server 端重試。串流不重試：已經收到的訊息無法撤回。
 */
export function retryInterceptor(opts: RetryOptions = {}): Interceptor {
	const maxRetries = opts.server ? 1 : 2;
	const sleep =
		opts.sleep ??
		((ms: number) => new Promise<void>((resolve) => setTimeout(resolve, ms)));
	const jitter = opts.jitter ?? Math.random;
	return (next) => async (req) => {
		if (req.stream) return next(req);
		for (let retry = 0; ; retry++) {
			try {
				return await next(req);
			} catch (raw) {
				if (retry >= maxRetries || req.signal.aborted) throw raw;
				// fetch 的網路錯誤在 interceptor 這一層還是原本的 TypeError，connect-es 在最外層才包成
				// ConnectError；先以同樣的規則轉換再分類。
				const err = ConnectError.from(raw);
				const d = classifyError(factsFromError(err, req.method, req.message));
				if (d.action !== "retry") throw raw;
				if (opts.server) {
					if (d.retryDelayMs !== undefined) throw raw;
					continue;
				}
				await sleep(d.retryDelayMs ?? backoff(retry, jitter()));
			}
		}
	};
}

function backoff(retry: number, jitter: number): number {
	const capped = Math.min(200 * 2 ** retry, 2000);
	return capped + capped * jitter;
}
