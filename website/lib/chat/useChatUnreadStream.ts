"use client";

import { Code, ConnectError } from "@connectrpc/connect";
import * as React from "react";
import type { ChatSide } from "./types";
import { useChatStream } from "./useChatStream";

/**
 * 事件驅動的聊天未讀數：基準由 `fetcher` 讀一次（掛載與身分改變時），之後以串流的 `Ready` 與 `UnreadChanged`
 * 的絕對值覆寫；沒有輪詢。
 */
export interface UnreadPollingResult {
	count: number;
	loading: boolean;
	refresh: () => Promise<void>;
	setCount: (next: number) => void;
	decrement: (by?: number) => void;
	increment: (by?: number) => void;
}

interface Options {
	fetcher: () => Promise<number>;
	enabled: boolean;
	resetSignal?: unknown;
	/** 目前工作區對應的對話方；串流依它開買家版或店家版。 */
	side: ChatSide;
}

export function useChatUnreadStream({
	fetcher,
	enabled,
	resetSignal,
	side,
}: Options): UnreadPollingResult {
	const [count, setCountState] = React.useState(0);
	const [loading, setLoading] = React.useState(false);
	const cancelledRef = React.useRef(false);
	const fetcherRef = React.useRef(fetcher);
	React.useEffect(() => {
		fetcherRef.current = fetcher;
	}, [fetcher]);

	const load = React.useCallback(async () => {
		if (!enabled) {
			setCountState(0);
			return;
		}
		setLoading(true);
		try {
			const next = await fetcherRef.current();
			if (!cancelledRef.current) setCountState(next);
		} catch (err) {
			if (ConnectError.from(err).code === Code.Unauthenticated)
				setCountState(0);
			// 其他錯誤：保留最後已知的數字，不閃動。
		} finally {
			if (!cancelledRef.current) setLoading(false);
		}
	}, [enabled]);

	React.useEffect(() => {
		void resetSignal;
		cancelledRef.current = false;
		setCountState(0);
		void load();
		return () => {
			cancelledRef.current = true;
		};
	}, [load, resetSignal]);

	useChatStream(side, enabled, (e) => {
		if (e.type === "ready" || e.type === "unread") {
			setCountState(Math.max(0, e.unreadCount));
		}
	});

	const setCount = React.useCallback(
		(next: number) => setCountState(Math.max(0, next)),
		[],
	);
	const decrement = React.useCallback(
		(by = 1) => setCountState((p) => Math.max(0, p - by)),
		[],
	);
	const increment = React.useCallback(
		(by = 1) => setCountState((p) => p + by),
		[],
	);

	return { count, loading, refresh: load, setCount, decrement, increment };
}
