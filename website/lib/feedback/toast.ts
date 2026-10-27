"use client";

import type { ReactNode } from "react";
import type { ExternalToast } from "sonner";
import { whenDocumentSettled } from "@/lib/hydration/document-settled";

type ToastMessage = ReactNode | (() => ReactNode);
type ToastKind = "success" | "info" | "warning" | "error" | "loading";
type PendingToast = {
	kind: ToastKind;
	message: ToastMessage;
	data: ExternalToast;
};

let toastSequence = 0;

export function createDeferredQueue<T>(deliver: (items: T[]) => void) {
	let ready = false;
	const pending: T[] = [];

	return {
		push(item: T) {
			if (ready) {
				deliver([item]);
				return;
			}
			pending.push(item);
		},
		markReady() {
			if (ready) return;
			ready = true;
			if (pending.length === 0) return;
			deliver(pending.splice(0));
		},
	};
}

const toastQueue = createDeferredQueue<PendingToast>((items) => {
	void import("sonner").then(({ toast }) => {
		for (const item of items) {
			toast[item.kind](item.message, item.data);
		}
	});
});

/** Called only after the shared app-level Sonner viewport has committed. */
export function markToastViewportReady(): void {
	toastQueue.markReady();
}

function enqueueToast(
	kind: ToastKind,
	message: ToastMessage,
	data: ExternalToast = {},
): string | number {
	const id = data.id ?? `hk-toast-${Date.now()}-${++toastSequence}`;

	// Keep calls queued until AppToaster has both crossed the document barrier
	// and committed Sonner's subscriber. This prevents an early call from either
	// mutating <head> during hydration or disappearing before Sonner is ready.
	void whenDocumentSettled().then(() => {
		toastQueue.push({ kind, message, data: { ...data, id } });
	});

	return id;
}

export const toast = {
	success: (message: ToastMessage, data?: ExternalToast) =>
		enqueueToast("success", message, data),
	info: (message: ToastMessage, data?: ExternalToast) =>
		enqueueToast("info", message, data),
	warning: (message: ToastMessage, data?: ExternalToast) =>
		enqueueToast("warning", message, data),
	error: (message: ToastMessage, data?: ExternalToast) =>
		enqueueToast("error", message, data),
	loading: (message: ToastMessage, data?: ExternalToast) =>
		enqueueToast("loading", message, data),
};
