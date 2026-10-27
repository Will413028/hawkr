/** @vitest-environment jsdom */

import {
	act,
	cleanup,
	fireEvent,
	render,
	screen,
} from "@testing-library/react";
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ChatDrawerThreadBoundary } from "./ChatDrawerThread";
import {
	type ChatDrawerThreadProps,
	createChatDrawerThreadResource,
} from "./ChatDrawerThreadLoader";

function deferred<T>() {
	let resolve!: (value: T) => void;
	let reject!: (reason?: unknown) => void;
	const promise = new Promise<T>((resolvePromise, rejectPromise) => {
		resolve = resolvePromise;
		reject = rejectPromise;
	});
	return { promise, resolve, reject };
}

function FakeThread({ targetId }: ChatDrawerThreadProps) {
	return React.createElement("textarea", {
		"aria-label": `對話 ${targetId}`,
		defaultValue: "",
	});
}

describe("ChatDrawerThread lazy boundary", () => {
	afterEach(cleanup);

	it("deduplicates a cold request and keeps the resolved module warm", async () => {
		const request = deferred<{ default: typeof FakeThread }>();
		const importer = vi.fn(() => request.promise);
		const resource = createChatDrawerThreadResource(importer);

		const first = resource.load();
		const second = resource.load();
		expect(first).toBe(second);
		expect(importer).toHaveBeenCalledTimes(1);
		expect(resource.peek()).toBeNull();

		request.resolve({ default: FakeThread });
		expect(await first).toBe(FakeThread);
		expect(resource.peek()).toBe(FakeThread);
		expect(await resource.load()).toBe(FakeThread);
		expect(importer).toHaveBeenCalledTimes(1);
	});

	it("shows stable cold feedback, uses the latest target, and reopens warm", async () => {
		const request = deferred<{ default: typeof FakeThread }>();
		const importer = vi.fn(() => request.promise);
		const resource = createChatDrawerThreadResource(importer);
		const view = render(
			React.createElement(ChatDrawerThreadBoundary, {
				resource,
				targetId: "seller-a",
			}),
		);

		expect(screen.getByRole("status").textContent).toContain("載入聊天室中");
		expect(screen.getByLabelText("對話內容").getAttribute("aria-busy")).toBe(
			"true",
		);
		view.rerender(
			React.createElement(ChatDrawerThreadBoundary, {
				resource,
				targetId: "seller-b",
			}),
		);

		await act(async () => {
			request.resolve({ default: FakeThread });
			await request.promise;
		});
		expect(screen.getByRole("textbox", { name: "對話 seller-b" })).toBeTruthy();
		expect(importer).toHaveBeenCalledTimes(1);

		view.unmount();
		render(
			React.createElement(ChatDrawerThreadBoundary, {
				resource,
				targetId: "seller-c",
			}),
		);
		expect(screen.queryByRole("status")).toBeNull();
		expect(screen.getByRole("textbox", { name: "對話 seller-c" })).toBeTruthy();
		expect(importer).toHaveBeenCalledTimes(1);
	});

	it("releases a failed chunk request and recovers through the visible retry", async () => {
		const retryRequest = deferred<{ default: typeof FakeThread }>();
		const importer = vi
			.fn<() => Promise<{ default: typeof FakeThread }>>()
			.mockRejectedValueOnce(new Error("chunk unavailable"))
			.mockReturnValueOnce(retryRequest.promise);
		const resource = createChatDrawerThreadResource(importer);

		render(
			React.createElement(
				"div",
				{ role: "dialog", tabIndex: -1 },
				React.createElement(ChatDrawerThreadBoundary, {
					resource,
					targetId: "seller-retry",
				}),
			),
		);

		expect((await screen.findByRole("alert")).textContent).toContain(
			"聊天室暫時無法載入",
		);
		const dialog = screen.getByRole("dialog");
		const retry = screen.getByRole("button", { name: "重新載入" });
		retry.focus();
		expect(document.activeElement).toBe(retry);
		fireEvent.click(retry);
		expect(screen.getByRole("status").textContent).toContain("載入聊天室中");
		expect(document.activeElement).toBe(dialog);

		await act(async () => {
			retryRequest.resolve({ default: FakeThread });
			await retryRequest.promise;
		});
		expect(
			screen.getByRole("textbox", { name: "對話 seller-retry" }),
		).toBeTruthy();
		expect(document.activeElement).toBe(dialog);
		expect(importer).toHaveBeenCalledTimes(2);
	});

	it("does not commit a late request into an unmounted drawer", async () => {
		const request = deferred<{ default: typeof FakeThread }>();
		const importer = vi.fn(() => request.promise);
		const resource = createChatDrawerThreadResource(importer);
		const view = render(
			React.createElement(ChatDrawerThreadBoundary, {
				resource,
				targetId: "seller-closing",
			}),
		);

		view.unmount();
		await act(async () => {
			request.resolve({ default: FakeThread });
			await request.promise;
		});

		render(
			React.createElement(ChatDrawerThreadBoundary, {
				resource,
				targetId: "seller-reopened",
			}),
		);
		expect(
			screen.getByRole("textbox", { name: "對話 seller-reopened" }),
		).toBeTruthy();
		expect(importer).toHaveBeenCalledTimes(1);
	});
});
