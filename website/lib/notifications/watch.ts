/**
 * 通知訊號串流（`WatchNotifications`，server streaming）的 client。
 *
 * 串流的生命週期（見 notification.proto）：連線建立後第一個 frame 是 inbox 訊號，之後每有變化再送；訊號只表示「該重讀了」，
 * 不帶內容、沒有續傳游標、沒有心跳。所以：
 * - 每次重連後的第一個 frame 發 `reconnected`，`afterAbnormalEnd` 說明上一條怎麼結束的：斷線期間的訊號可能遺漏，上一條是錯誤結束的，訂閱者要整批重讀；
 *   上一條是壽命到期正常結束的（立刻重連，空窗是毫秒），不必重讀，重連後的第一則 inbox 訊號已涵蓋收件匣。第一次連線不發。
 * - 伺服器到了壽命正常結束（沒有錯誤）：立刻重連，不算失敗；沒收到任何 frame 就結束則當失敗走 backoff（避免熱迴圈）。
 * - 網路錯誤與 `unavailable`：full-jitter backoff 重連，連續失敗 `MAX_ATTEMPTS` 次後停下（`unavailable`），等 focus／online 的 `resume()`；
 *   瀏覽器離線時停在 `offline`，同樣等 `resume()`。
 * - `resource_exhausted`（同帳號串流太多，或開串流太頻繁）：可重試，伺服器帶 RetryInfo 就等那麼久，沒有就 full-jitter backoff；
 *   跟網路錯誤共用連續失敗上限，用盡後等 `resume()`，不會永久停住。
 * - `unauthenticated`、`permission_denied`、`not_found`（STORE 沒有店）、`failed_precondition`：
 *   不重連，狀態是 `unavailable`（登入失效由 transport 發 AUTH_EXPIRED_EVENT）。
 */

import { Code, ConnectError } from "@connectrpc/connect";
import { factsFromError } from "@hawkr/api/connect/errors";
import {
	type NotificationAudience,
	SellerWorkspaceChange,
	type WatchNotificationsResponse,
} from "@hawkr/api/gen/hawkr/notification/v1alpha1/notification_pb";
import { notificationStreamClient } from "@/lib/connect/notification";
import type { SseConnectionStatus } from "@/lib/sse/client";

/** 重連前的最多連續失敗次數，超過後等 `resume()`。 */
export const MAX_ATTEMPTS = 5;
/** backoff 的上限（毫秒）：1、2、4、8、16 秒，full jitter。 */
export const BACKOFF_CAP_MS = 16_000;

export type NotificationSignal =
	| {
			type: "inbox";
			unreadCount: number;
			latestInboxItemId?: string;
			newInboxItemIds: string[];
			newItemsTruncated: boolean;
			hints: number[];
	  }
	| { type: "sellerWorkspace"; change: "order" | "afterSale" }
	| { type: "reconnected"; afterAbnormalEnd: boolean };

export type OpenWatch = (
	audience: NotificationAudience,
	signal: AbortSignal,
) => AsyncIterable<WatchNotificationsResponse>;

export interface WatchDeps {
	open?: OpenWatch;
	/** [0, 1)；測試注入。 */
	random?: () => number;
	setTimer?: (fn: () => void, ms: number) => unknown;
	clearTimer?: (t: unknown) => void;
	isOnline?: () => boolean;
}

export interface WatchHandle {
	stop(): void;
	/** focus／online：停在 `offline` 或 `unavailable`（重試用盡）的串流立刻再試。 */
	resume(): void;
}

function isTerminal(code: Code): boolean {
	return (
		code === Code.Unauthenticated ||
		code === Code.PermissionDenied ||
		code === Code.NotFound ||
		code === Code.FailedPrecondition
	);
}

const defaultOpen: OpenWatch = async function* (audience, signal) {
	const client = notificationStreamClient();
	for await (const res of client.watchNotifications({ audience }, { signal })) {
		yield res;
	}
};

export function watchNotifications(
	audience: NotificationAudience,
	onSignal: (s: NotificationSignal) => void,
	onStatus: (s: SseConnectionStatus) => void,
	deps: WatchDeps = {},
): WatchHandle {
	const open = deps.open ?? defaultOpen;
	const random = deps.random ?? Math.random;
	const setTimer = deps.setTimer ?? ((fn, ms) => setTimeout(fn, ms));
	const clearTimer =
		deps.clearTimer ??
		((t) => clearTimeout(t as ReturnType<typeof setTimeout>));
	const isOnline =
		deps.isOnline ??
		(() => typeof navigator === "undefined" || navigator.onLine !== false);

	let stopped = false;
	let abort: AbortController | null = null;
	let wake: (() => void) | null = null;
	let status: SseConnectionStatus = "idle";
	const setStatus = (s: SseConnectionStatus) => {
		if (s === status) return;
		status = s;
		onStatus(s);
	};
	const sleep = (ms: number) =>
		new Promise<void>((resolve) => {
			const t = setTimer(() => {
				wake = null;
				resolve();
			}, ms);
			wake = () => {
				clearTimer(t);
				wake = null;
				resolve();
			};
		});
	const waitForResume = () =>
		new Promise<void>((resolve) => {
			wake = () => {
				wake = null;
				resolve();
			};
		});

	type Outcome =
		| { kind: "ended"; gotFrame: boolean }
		| { kind: "terminal" }
		| { kind: "error"; gotFrame: boolean; retryDelayMs?: number };

	let everConnected = false;
	let lastEndAbnormal = false;
	const runOnce = async (): Promise<Outcome> => {
		const ac = new AbortController();
		abort = ac;
		let gotFrame = false;
		try {
			for await (const res of open(audience, ac.signal)) {
				if (!gotFrame) {
					gotFrame = true;
					setStatus("connected");
					if (everConnected)
						onSignal({
							type: "reconnected",
							afterAbnormalEnd: lastEndAbnormal,
						});
					everConnected = true;
				}
				const e = res.event;
				if (e.case === "inbox") {
					onSignal({
						type: "inbox",
						unreadCount: e.value.unreadCount,
						latestInboxItemId: e.value.latestInboxItemId,
						newInboxItemIds: [...e.value.newInboxItemIds],
						newItemsTruncated: e.value.newItemsTruncated,
						hints: [...e.value.stateRefreshHints],
					});
				} else if (e.case === "sellerWorkspace") {
					const change = e.value.change;
					if (change === SellerWorkspaceChange.ORDER) {
						onSignal({ type: "sellerWorkspace", change: "order" });
					} else if (change === SellerWorkspaceChange.AFTER_SALE) {
						onSignal({ type: "sellerWorkspace", change: "afterSale" });
					}
				}
			}
			return { kind: "ended", gotFrame };
		} catch (err) {
			if (stopped) return { kind: "ended", gotFrame };
			const e = ConnectError.from(err);
			if (isTerminal(e.code)) return { kind: "terminal" };
			return {
				kind: "error",
				gotFrame,
				retryDelayMs:
					e.code === Code.ResourceExhausted
						? factsFromError(e).retryDelayMs
						: undefined,
			};
		} finally {
			abort = null;
		}
	};

	const loop = async () => {
		let failures = 0;
		while (!stopped) {
			setStatus("connecting");
			const outcome = await runOnce();
			if (stopped) return;
			lastEndAbnormal = !(outcome.kind === "ended" && outcome.gotFrame);
			if (outcome.kind === "terminal") {
				setStatus("unavailable");
				return;
			}
			if (outcome.kind === "ended" && outcome.gotFrame) {
				failures = 0;
				continue;
			}
			if (outcome.gotFrame) failures = 0;
			if (!isOnline()) {
				setStatus("offline");
				await waitForResume();
				failures = 0;
				continue;
			}
			failures++;
			if (failures >= MAX_ATTEMPTS) {
				setStatus("unavailable");
				await waitForResume();
				failures = 0;
				continue;
			}
			const ceiling = Math.min(BACKOFF_CAP_MS, 1000 * 2 ** (failures - 1));
			const serverDelay =
				outcome.kind === "error" ? outcome.retryDelayMs : undefined;
			await sleep(serverDelay ?? Math.floor(random() * ceiling));
		}
	};
	void loop();

	return {
		stop() {
			stopped = true;
			abort?.abort();
			wake?.();
			setStatus("idle");
		},
		resume() {
			wake?.();
		},
	};
}
