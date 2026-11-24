import type { Payment } from "@hawkr/api/gen/hawkr/payment/v1alpha1/payment_pb";
import { isSettled, paymentView, type SettledView } from "./state";

export const POLL_INTERVAL_MS = 2000;
export const POLL_MAX_ATTEMPTS = 15;

export type PollResult =
	| { kind: "settled"; view: SettledView }
	| { kind: "timeout" };

export interface PollOptions {
	read: () => Promise<Payment | undefined>;
	intervalMs?: number;
	maxAttempts?: number;
	signal?: AbortSignal;
}

function sleep(ms: number, signal?: AbortSignal): Promise<void> {
	return new Promise((resolve) => {
		const timer = setTimeout(done, ms);
		function done() {
			clearTimeout(timer);
			signal?.removeEventListener("abort", done);
			resolve();
		}
		signal?.addEventListener("abort", done);
	});
}

/**
 * 以 `read`（GetPayment）重讀到出現要買家動作或終局的狀態為止，最多 `maxAttempts` 次，次數用完是 `timeout`
 * （不是失敗：付款可能仍在進行，由訂單頁確認）。`read` 的錯誤原樣往上拋，由呼叫端用 `errorText` 顯示；
 * Connect transport 已依錯誤模型重試過暫時性錯誤。`signal` 中止後回 `timeout` 且不再讀取。
 */
export async function pollPayment(options: PollOptions): Promise<PollResult> {
	const {
		read,
		intervalMs = POLL_INTERVAL_MS,
		maxAttempts = POLL_MAX_ATTEMPTS,
		signal,
	} = options;
	for (let attempt = 0; attempt < maxAttempts; attempt++) {
		if (signal?.aborted) return { kind: "timeout" };
		const view = paymentView(await read());
		if (isSettled(view)) return { kind: "settled", view };
		if (attempt < maxAttempts - 1) await sleep(intervalMs, signal);
	}
	return { kind: "timeout" };
}
