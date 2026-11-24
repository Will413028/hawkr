"use client";

import * as React from "react";
import {
	NotificationToastViewport,
	type ToastView,
} from "@/components/shared/NotificationToast";

/**
 * Imperative in-app toast queue. `show()` pushes a drop-down notification
 * banner (see `NotificationToast`); the provider renders the viewport via a
 * portal so toasts float above the whole app. Triggers live in
 * `NotificationToastBridge`.
 *
 * Keeps at most `MAX_TOTAL` toasts; `dedupeKey` replaces the toast that is
 * still on screen with the same key (so a re-delivered socket message or an
 * aggregated notification update doesn't stack). Toasts sharing a `groupKey` (same
 * chat sender / same notification title) are collapsed into one expandable
 * stack by the viewport — `MAX_TOTAL` is raised accordingly so a burst from
 * one source stacks instead of evicting unrelated toasts.
 */

export type ToastInput = Omit<ToastView, "id">;

interface ToastApi {
	show: (toast: ToastInput) => void;
	dismiss: (id: number) => void;
}

const ToastContext = React.createContext<ToastApi | null>(null);
const MAX_TOTAL = 12;

export function NotificationToastProvider({
	children,
}: {
	children: React.ReactNode;
}) {
	const [toasts, setToasts] = React.useState<ToastView[]>([]);
	const idRef = React.useRef(0);
	const activeKeys = React.useRef<Set<string>>(new Set());

	const dismiss = React.useCallback((id: number) => {
		setToasts((prev) => {
			const gone = prev.find((t) => t.id === id);
			if (gone?.dedupeKey) activeKeys.current.delete(gone.dedupeKey);
			return prev.filter((t) => t.id !== id);
		});
	}, []);

	const show = React.useCallback((toast: ToastInput) => {
		idRef.current += 1;
		const next: ToastView = { ...toast, id: idRef.current };
		const replacing =
			!!toast.dedupeKey && activeKeys.current.has(toast.dedupeKey);
		if (toast.dedupeKey) activeKeys.current.add(toast.dedupeKey);
		setToasts((prev) => {
			if (replacing) {
				// Same key still on screen: update that toast in place (new id
				// restarts its timer) instead of stacking a duplicate.
				const at = prev.findIndex((t) => t.dedupeKey === toast.dedupeKey);
				if (at >= 0) return prev.map((t, i) => (i === at ? next : t));
			}
			const merged = [...prev, next];
			if (merged.length <= MAX_TOTAL) return merged;
			// Drop the oldest overflow and release its dedupe key.
			for (const dropped of merged.slice(0, merged.length - MAX_TOTAL)) {
				if (dropped.dedupeKey) activeKeys.current.delete(dropped.dedupeKey);
			}
			return merged.slice(-MAX_TOTAL);
		});
	}, []);

	const api = React.useMemo<ToastApi>(
		() => ({ show, dismiss }),
		[show, dismiss],
	);

	return (
		<ToastContext.Provider value={api}>
			{children}
			<NotificationToastViewport toasts={toasts} onDismiss={dismiss} />
		</ToastContext.Provider>
	);
}

/**
 * Read the toast API. Outside the provider returns no-ops so SSR / fallback
 * paths never crash.
 */
export function useNotificationToast(): ToastApi {
	const ctx = React.useContext(ToastContext);
	if (ctx) return ctx;
	return { show: () => {}, dismiss: () => {} };
}
