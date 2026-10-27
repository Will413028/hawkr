"use client";

import * as React from "react";
import { UnauthorizedError } from "@/lib/api/client";
import { type RoleSlot, roleToHeader } from "@/lib/auth/active-role";
import { getChatSocket } from "@/lib/chat/ws";

/**
 * Event-driven chat-unread source. Replaces the 30s polling hook
 * (`useChatUnreadPolling`): the backend now pushes an authoritative
 * `unread_changed` WS event on every send / mark-read, so we SET the
 * count from that and only hit REST for the initial baseline + once on
 * each WS (re)connect. No interval. Mirrors notification's
 * `useUnreadCount`, but the channel is the existing chat WebSocket.
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
	/** Current active role; live events whose role !== this are ignored (stale socket guard). */
	activeRole?: RoleSlot | null;
}

export function useChatUnreadStream({
	fetcher,
	enabled,
	resetSignal,
	activeRole,
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
			if (err instanceof UnauthorizedError) setCountState(0);
			// other errors: keep last known count, don't flap
		} finally {
			if (!cancelledRef.current) setLoading(false);
		}
	}, [enabled]);

	// Baseline on mount + resetSignal change.
	React.useEffect(() => {
		void resetSignal;
		cancelledRef.current = false;
		setCountState(0);
		void load();
		return () => {
			cancelledRef.current = true;
		};
	}, [load, resetSignal]);

	// Authoritative SET on unread_changed (filtered to current role) + re-baseline on reconnect.
	React.useEffect(() => {
		if (!enabled) return;
		const want = roleToHeader(activeRole ?? null); // "BUYER" | "SELLER" | undefined
		const sock = getChatSocket();
		const offUnread = sock.onUnreadChanged(({ role, unreadCount }) => {
			// Stale-socket guard: ignore events for a role other than the active one.
			if (want && role && role !== want) return;
			setCountState(Math.max(0, unreadCount));
		});
		const offState = sock.onState((s) => {
			if (s === "connected") void load();
		});
		return () => {
			offUnread();
			offState();
		};
	}, [enabled, load, activeRole]);

	// setCount/decrement/increment are kept for UnreadPollingResult shape parity
	// (mirrors notification's useUnreadCount) and optimistic-update consumers.
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
