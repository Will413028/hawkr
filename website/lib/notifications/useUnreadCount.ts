"use client";

/**
 * 導覽列通知鈴的未讀數狀態。串流由 `NotificationUnreadProvider` 持有（一個分頁一條，這個 hook 不開連線）：
 *
 *   - 掛載與 resetSignal 改變 → 讀一次 `GetUnreadCount` 當基準。
 *   - provider 收到串流的收件匣訊號時呼叫 `setCount`：訊號帶的是伺服器的絕對未讀數。
 *   - 串流在可能漏訊號之後（異常斷線後重連、重試用盡）呼叫 `refresh`，補讀一次，不輪詢。
 */

import type { NotificationAudience } from "@hawkr/api/gen/hawkr/notification/v1alpha1/notification_pb";
import * as React from "react";
import { fetchUnreadCount } from "@/lib/notifications/inbox";

export interface UnreadCountState {
	count: number;
	loading: boolean;
	/** The count is the server's answer for the current account and workspace. Unlike a loading edge, it cannot be skipped by batched renders. */
	baselineReady: boolean;
	refresh: () => Promise<void>;
	setCount: (next: number) => void;
	decrement: (by?: number) => void;
	increment: (by?: number) => void;
}

interface UseUnreadCountOptions {
	enabled: boolean;
	/** 目前工作區的收件匣受眾。 */
	audience: NotificationAudience;
	/** Restart the baseline read when this signal changes. */
	resetSignal?: unknown;
}

export function useUnreadCount({
	enabled,
	audience,
	resetSignal,
}: UseUnreadCountOptions): UnreadCountState {
	const [count, setCountState] = React.useState(0);
	const [loading, setLoading] = React.useState(false);
	const [settledSignal, setSettledSignal] = React.useState<unknown>(null);
	const resetSignalRef = React.useRef(resetSignal);
	resetSignalRef.current = resetSignal;
	// 單調遞增的世代：每個比較舊的回應都永久作廢（布林旗標會被下一個 effect 重設）。
	const generationRef = React.useRef(0);
	const settle = React.useCallback(() => {
		setLoading(false);
		setSettledSignal(resetSignalRef.current);
	}, []);

	const load = React.useCallback(async () => {
		const generation = ++generationRef.current;
		if (!enabled) {
			setCountState(0);
			setLoading(false);
			setSettledSignal(null);
			return;
		}
		setLoading(true);
		try {
			const next = await fetchUnreadCount(audience);
			if (generation === generationRef.current) setCountState(next);
		} catch {
			// 暫時性錯誤保留上一個已知的數字，鈴鐺不閃爍；登入失效由 transport 發事件。
		} finally {
			if (generation === generationRef.current) settle();
		}
	}, [audience, enabled, settle]);

	React.useEffect(() => {
		void resetSignal;
		setCountState(0);
		setSettledSignal(null);
		void load();
		return () => {
			generationRef.current += 1;
		};
	}, [load, resetSignal]);

	const setCount = React.useCallback(
		(next: number) => {
			generationRef.current += 1;
			settle();
			setCountState(Math.max(0, next));
		},
		[settle],
	);

	const decrement = React.useCallback(
		(by = 1) => {
			generationRef.current += 1;
			settle();
			setCountState((prev) => Math.max(0, prev - by));
		},
		[settle],
	);

	const increment = React.useCallback(
		(by = 1) => {
			generationRef.current += 1;
			settle();
			setCountState((prev) => prev + by);
		},
		[settle],
	);

	return React.useMemo<UnreadCountState>(
		() => ({
			count,
			loading,
			baselineReady: settledSignal === resetSignal,
			refresh: load,
			setCount,
			decrement,
			increment,
		}),
		[
			count,
			loading,
			settledSignal,
			resetSignal,
			load,
			setCount,
			decrement,
			increment,
		],
	);
}
