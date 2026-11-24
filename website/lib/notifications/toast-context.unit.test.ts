// @vitest-environment jsdom

import { act, cleanup, render } from "@testing-library/react";
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

const viewport = vi.hoisted(() => ({
	toasts: [] as Array<{ id: number; title: string; dedupeKey?: string }>,
}));
vi.mock("@/components/shared/NotificationToast", () => ({
	NotificationToastViewport: (p: { toasts: typeof viewport.toasts }) => {
		viewport.toasts = p.toasts;
		return null;
	},
}));

import {
	NotificationToastProvider,
	useNotificationToast,
} from "./toast-context";

afterEach(cleanup);

describe("NotificationToastProvider dedupeKey", () => {
	it("replaces the on-screen toast that has the same key instead of stacking", () => {
		let api!: ReturnType<typeof useNotificationToast>;
		const Probe = () => {
			api = useNotificationToast();
			return null;
		};
		render(
			React.createElement(
				NotificationToastProvider,
				null,
				React.createElement(Probe),
			),
		);
		const base = { kind: "system" as const, body: "", dedupeKey: "notif:1" };
		act(() => api.show({ ...base, title: "first" }));
		act(() => api.show({ ...base, title: "second" }));
		expect(viewport.toasts.map((t) => t.title)).toEqual(["second"]);
		act(() => api.show({ ...base, dedupeKey: "notif:2", title: "other" }));
		expect(viewport.toasts.map((t) => t.title)).toEqual(["second", "other"]);
	});
});
