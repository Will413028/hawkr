import { Code, ConnectError } from "@connectrpc/connect";
import {
	NotificationAudience,
	SellerWorkspaceChange,
	StateRefreshHint,
	type WatchNotificationsResponse,
} from "@hawkr/api/gen/hawkr/notification/v1alpha1/notification_pb";
import { describe, expect, it } from "vitest";
import type { SseConnectionStatus } from "@/lib/sse/client";
import {
	MAX_ATTEMPTS,
	type NotificationSignal,
	type OpenWatch,
	watchNotifications,
} from "./watch";

const inbox = (unreadCount: number, hints: StateRefreshHint[] = []) =>
	({
		event: {
			case: "inbox",
			value: { unreadCount, stateRefreshHints: hints },
		},
	}) as unknown as WatchNotificationsResponse;
const workspace = (change: SellerWorkspaceChange) =>
	({
		event: { case: "sellerWorkspace", value: { change } },
	}) as unknown as WatchNotificationsResponse;

type Script = Array<{
	frames?: WatchNotificationsResponse[];
	error?: Code;
	/** 送完 frames 之後不結束，等 abort。 */
	hold?: boolean;
}>;

function harness(script: Script, opts: { online?: () => boolean } = {}) {
	const signals: NotificationSignal[] = [];
	const statuses: SseConnectionStatus[] = [];
	const audiences: NotificationAudience[] = [];
	let opens = 0;
	const timers: Array<{ fn: () => void; ms: number }> = [];
	const open: OpenWatch = async function* (audience, signal) {
		const step = script[opens++] ?? { hold: true };
		audiences.push(audience);
		for (const f of step.frames ?? []) yield f;
		if (step.error !== undefined) throw new ConnectError("x", step.error);
		if (step.hold) {
			await new Promise<void>((_, reject) =>
				signal.addEventListener("abort", () =>
					reject(new ConnectError("aborted", Code.Canceled)),
				),
			);
		}
	};
	const handle = watchNotifications(
		NotificationAudience.STORE,
		(s) => signals.push(s),
		(s) => statuses.push(s),
		{
			open,
			random: () => 0.5,
			setTimer: (fn, ms) => {
				timers.push({ fn, ms });
				return timers.length - 1;
			},
			clearTimer: () => {},
			isOnline: opts.online,
		},
	);
	const flush = async () => {
		for (let i = 0; i < 20; i++) await Promise.resolve();
	};
	return {
		signals,
		statuses,
		audiences,
		timers,
		handle,
		flush,
		opens: () => opens,
	};
}

describe("watchNotifications", () => {
	it("把 inbox 與賣家工作區 frame 轉成訊號，第一次連線不發 reconnected", async () => {
		const h = harness([
			{
				frames: [
					inbox(2, [StateRefreshHint.ORDER]),
					workspace(SellerWorkspaceChange.ORDER),
					workspace(SellerWorkspaceChange.AFTER_SALE),
				],
				hold: true,
			},
		]);
		await h.flush();
		expect(h.audiences).toEqual([NotificationAudience.STORE]);
		expect(h.signals).toEqual([
			{
				type: "inbox",
				unreadCount: 2,
				latestInboxItemId: undefined,
				hints: [StateRefreshHint.ORDER],
			},
			{ type: "sellerWorkspace", change: "order" },
			{ type: "sellerWorkspace", change: "afterSale" },
		]);
		expect(h.statuses).toEqual(["connecting", "connected"]);
		h.handle.stop();
	});

	it("伺服器到壽命正常結束：立刻重連，重連後第一個 frame 發 reconnected", async () => {
		const h = harness([
			{ frames: [inbox(0)] },
			{ frames: [inbox(1)], hold: true },
		]);
		await h.flush();
		expect(h.opens()).toBe(2);
		expect(h.timers).toHaveLength(0);
		expect(h.signals.map((s) => s.type)).toEqual([
			"inbox",
			"reconnected",
			"inbox",
		]);
		h.handle.stop();
	});

	it("終止型 code 不重連，狀態是 unavailable", async () => {
		for (const code of [
			Code.NotFound,
			Code.ResourceExhausted,
			Code.Unauthenticated,
			Code.PermissionDenied,
			Code.FailedPrecondition,
		]) {
			const h = harness([{ error: code }]);
			await h.flush();
			expect(h.opens()).toBe(1);
			expect(h.statuses.at(-1)).toBe("unavailable");
		}
	});

	it("unavailable 以 backoff 重連，連續失敗用盡後停在 unavailable 並等 resume", async () => {
		const h = harness(
			Array.from({ length: 20 }, () => ({ error: Code.Unavailable })),
		);
		await h.flush();
		for (let i = 0; i < MAX_ATTEMPTS - 1; i++) {
			expect(h.timers).toHaveLength(i + 1);
			h.timers[i]?.fn();
			await h.flush();
		}
		expect(h.opens()).toBe(MAX_ATTEMPTS);
		expect(h.statuses.at(-1)).toBe("unavailable");
		const before = h.opens();
		h.handle.resume();
		await h.flush();
		expect(h.opens()).toBeGreaterThan(before);
		h.handle.stop();
	});

	it("瀏覽器離線時停在 offline，resume 才再試", async () => {
		let online = false;
		const h = harness(
			[{ error: Code.Unavailable }, { frames: [inbox(0)], hold: true }],
			{ online: () => online },
		);
		await h.flush();
		expect(h.statuses.at(-1)).toBe("offline");
		expect(h.opens()).toBe(1);
		online = true;
		h.handle.resume();
		await h.flush();
		expect(h.opens()).toBe(2);
		expect(h.statuses.at(-1)).toBe("connected");
		h.handle.stop();
	});

	it("沒收到任何 frame 就正常結束算失敗，走 backoff 而不是熱迴圈", async () => {
		const h = harness([{}, {}, { hold: true }]);
		await h.flush();
		expect(h.opens()).toBe(1);
		expect(h.timers).toHaveLength(1);
		h.handle.stop();
	});

	it("stop 中止連線並回到 idle，不再重連", async () => {
		const h = harness([{ frames: [inbox(0)], hold: true }]);
		await h.flush();
		h.handle.stop();
		await h.flush();
		expect(h.opens()).toBe(1);
		expect(h.statuses.at(-1)).toBe("idle");
	});
});
