/** @vitest-environment jsdom */

import {
	act,
	cleanup,
	fireEvent,
	render,
	screen,
} from "@testing-library/react";
import * as React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
	acquireBodyScrollLock,
	resetBodyScrollLocksForTest,
} from "@/lib/browser/body-scroll-lock";
import { HomePostDetailViewerBoundary } from "./HomePostDetailViewer";
import {
	createHomePostDetailViewerResource,
	type HomePostDetailViewerProps,
	type HomePostDetailViewerResource,
} from "./HomePostDetailViewerLoader";

type ViewerModule = {
	default: React.ComponentType<HomePostDetailViewerProps>;
};

const originalScrollYDescriptor = Object.getOwnPropertyDescriptor(
	window,
	"scrollY",
);
const originalScrollToDescriptor = Object.getOwnPropertyDescriptor(
	window,
	"scrollTo",
);
let simulatedScrollY = 0;
let scrollToMock = vi.fn((_x: number, _y: number) => {});

function isScrollSurfaceHidden() {
	return (
		document.body.style.getPropertyValue("overflow") === "hidden" ||
		document.documentElement.style.getPropertyValue("overflow") === "hidden"
	);
}

function deferred<T>() {
	let resolve!: (value: T) => void;
	let reject!: (reason?: unknown) => void;
	const promise = new Promise<T>((resolvePromise, rejectPromise) => {
		resolve = resolvePromise;
		reject = rejectPromise;
	});
	return { promise, resolve, reject };
}

function FakeViewer({ onClose }: HomePostDetailViewerProps) {
	const closeRef = React.useRef<HTMLButtonElement>(null);
	const openerAtHandoff =
		document.activeElement?.getAttribute("data-home-post-test-opener") ===
		"true";

	// Mirrors PostDetailViewer's own passive-effect lock. The home boundary must
	// capture the real offset first so this nested lock cannot observe a clamped 0.
	React.useEffect(
		() => acquireBodyScrollLock({ preserveScrollPosition: true }),
		[],
	);

	React.useEffect(() => {
		const previous =
			document.activeElement instanceof HTMLElement
				? document.activeElement
				: null;
		closeRef.current?.focus({ preventScroll: true });
		return () => {
			if (previous?.isConnected) previous.focus({ preventScroll: true });
		};
	}, []);

	return React.createElement(
		"div",
		{
			role: "dialog",
			"aria-label": "已載入的貼文詳情",
			"data-opener-at-handoff": String(openerAtHandoff),
		},
		React.createElement(
			"button",
			{ ref: closeRef, type: "button", onClick: onClose },
			"關閉已載入貼文",
		),
	);
}

function Harness({ resource }: { resource: HomePostDetailViewerResource }) {
	const [open, setOpen] = React.useState(false);
	const openerRef = React.useRef<HTMLButtonElement>(null);

	return React.createElement(
		React.Fragment,
		null,
		React.createElement(
			"button",
			{
				ref: openerRef,
				type: "button",
				"data-home-post-test-opener": "true",
				onClick: () => setOpen(true),
			},
			"查看首頁貼文",
		),
		open
			? React.createElement(HomePostDetailViewerBoundary, {
					resource,
					returnFocusRef: openerRef,
					posts: [],
					initialIndex: 0,
					onClose: () => setOpen(false),
					hasMore: false,
					openRect: null,
				})
			: null,
	);
}

describe("home post detail viewer lazy boundary", () => {
	beforeEach(() => {
		simulatedScrollY = 0;
		scrollToMock = vi.fn((_x: number, nextY: number) => {
			simulatedScrollY = isScrollSurfaceHidden() ? 0 : nextY;
		});
		Object.defineProperty(window, "scrollY", {
			configurable: true,
			get: () => (isScrollSurfaceHidden() ? 0 : simulatedScrollY),
		});
		Object.defineProperty(window, "scrollTo", {
			configurable: true,
			value: scrollToMock,
		});
	});

	afterEach(() => {
		cleanup();
		resetBodyScrollLocksForTest();
		if (originalScrollYDescriptor) {
			Object.defineProperty(window, "scrollY", originalScrollYDescriptor);
		} else {
			Reflect.deleteProperty(window, "scrollY");
		}
		if (originalScrollToDescriptor) {
			Object.defineProperty(window, "scrollTo", originalScrollToDescriptor);
		} else {
			Reflect.deleteProperty(window, "scrollTo");
		}
	});

	it("keeps a cancellable pending dialog and ignores a late completion", async () => {
		simulatedScrollY = 120;
		const request = deferred<ViewerModule>();
		const resource = createHomePostDetailViewerResource(() => request.promise);
		render(React.createElement(Harness, { resource }));

		const opener = screen.getByRole("button", { name: "查看首頁貼文" });
		opener.focus();
		fireEvent.click(opener);

		const pending = screen.getByRole("dialog", { name: "貼文詳情" });
		expect(pending.getAttribute("data-home-post-detail-state")).toBe("loading");
		expect(pending.getAttribute("aria-busy")).toBe("true");
		expect(document.body.style.position).toBe("fixed");
		expect(document.body.style.top).toBe("-120px");
		expect(window.scrollY).toBe(0);
		expect(document.activeElement).toBe(
			screen.getByRole("button", { name: "關閉" }),
		);

		fireEvent.keyDown(document, { key: "Escape" });
		expect(screen.queryByRole("dialog", { name: "貼文詳情" })).toBeNull();
		expect(document.activeElement).toBe(opener);
		expect(document.body.style.position).toBe("");
		expect(scrollToMock).toHaveBeenCalledTimes(1);
		expect(scrollToMock).toHaveBeenLastCalledWith(0, 120);
		expect(window.scrollY).toBe(120);

		await act(async () => {
			request.resolve({ default: FakeViewer });
			await request.promise;
		});
		expect(screen.queryByRole("dialog")).toBeNull();
	});

	it("preserves a nonzero offset when a prewarmed viewer takes its nested lock", async () => {
		simulatedScrollY = 240;
		const importer = vi.fn(async () => ({ default: FakeViewer }));
		const resource = createHomePostDetailViewerResource(importer);
		await resource.load();
		render(React.createElement(Harness, { resource }));

		const opener = screen.getByRole("button", { name: "查看首頁貼文" });
		opener.focus();
		fireEvent.click(opener);

		expect(screen.queryByRole("dialog", { name: "貼文詳情" })).toBeNull();
		expect(
			screen.getByRole("dialog", { name: "已載入的貼文詳情" }),
		).toBeTruthy();
		expect(document.body.style.position).toBe("fixed");
		expect(document.body.style.top).toBe("-240px");
		expect(window.scrollY).toBe(0);

		fireEvent.click(screen.getByRole("button", { name: "關閉已載入貼文" }));
		expect(screen.queryByRole("dialog")).toBeNull();
		expect(document.activeElement).toBe(opener);
		expect(document.body.style.position).toBe("");
		expect(document.body.style.top).toBe("");
		expect(scrollToMock).toHaveBeenCalledTimes(1);
		expect(scrollToMock).toHaveBeenLastCalledWith(0, 240);
		expect(window.scrollY).toBe(240);
		expect(importer).toHaveBeenCalledTimes(1);
	});

	it("retries a failed import while preserving the opener for viewer focus handoff", async () => {
		const recovered = deferred<ViewerModule>();
		let attempt = 0;
		const importer = vi.fn(() => {
			attempt += 1;
			return attempt === 1
				? Promise.reject(new Error("chunk unavailable"))
				: recovered.promise;
		});
		const resource = createHomePostDetailViewerResource(importer);
		render(React.createElement(Harness, { resource }));

		const opener = screen.getByRole("button", { name: "查看首頁貼文" });
		opener.focus();
		fireEvent.click(opener);

		expect((await screen.findByRole("alert")).textContent).toContain(
			"貼文暫時無法載入",
		);
		const failedDialog = screen.getByRole("dialog", { name: "貼文詳情" });
		expect(failedDialog.getAttribute("data-home-post-detail-state")).toBe(
			"error",
		);
		const retry = screen.getByRole("button", { name: "重新載入" });
		retry.focus();
		fireEvent.click(retry);

		expect(failedDialog.getAttribute("data-home-post-detail-state")).toBe(
			"loading",
		);
		expect(document.activeElement).toBe(failedDialog);

		await act(async () => {
			recovered.resolve({ default: FakeViewer });
			await recovered.promise;
		});

		const ready = screen.getByRole("dialog", {
			name: "已載入的貼文詳情",
		});
		expect(ready.getAttribute("data-opener-at-handoff")).toBe("true");
		const close = screen.getByRole("button", { name: "關閉已載入貼文" });
		expect(document.activeElement).toBe(close);
		expect(importer).toHaveBeenCalledTimes(2);

		fireEvent.click(close);
		expect(screen.queryByRole("dialog")).toBeNull();
		expect(document.activeElement).toBe(opener);
		expect(document.body.style.position).toBe("");
	});
});
