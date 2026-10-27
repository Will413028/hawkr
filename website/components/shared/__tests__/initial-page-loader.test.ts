// @vitest-environment jsdom

import { act, createElement, StrictMode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import InitialPageLoader from "@/components/shared/InitialPageLoader";
import {
	INITIAL_PUBLIC_CONTENT_ENABLED_CLASS,
	INITIAL_PUBLIC_CONTENT_READY_AT,
	INITIAL_PUBLIC_CONTENT_READY_CLASS,
} from "@/lib/loading/initial-public-content";

vi.mock("@sentry/nextjs", () => ({ captureMessage: vi.fn() }));

let root: Root;
let host: HTMLDivElement;

beforeEach(() => {
	vi.useFakeTimers();
	vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
	vi.stubGlobal("matchMedia", () => ({ matches: false }));
	window.history.replaceState({}, "", "/");
	document.documentElement.className = "initial-loader-active";
	document.documentElement.removeAttribute(INITIAL_PUBLIC_CONTENT_READY_AT);
	document.body.innerHTML =
		'<main><span data-hk-initial-content-ready="home" hidden></span><h1>Hawkr</h1></main>';
	host = document.createElement("div");
	document.body.append(host);
	root = createRoot(host);
});

afterEach(async () => {
	await act(async () => root.unmount());
	vi.useRealTimers();
	vi.unstubAllGlobals();
	document.body.replaceChildren();
});

async function mount() {
	await act(async () =>
		root.render(
			createElement(StrictMode, null, createElement(InitialPageLoader)),
		),
	);
}

async function advance(ms: number) {
	await act(async () => vi.advanceTimersByTimeAsync(ms));
}

describe("InitialPageLoader Strict Mode lifecycle", () => {
	it("adopts a completed pre-hydration fade without adding a new minimum hold", async () => {
		document.documentElement.classList.add(
			INITIAL_PUBLIC_CONTENT_ENABLED_CLASS,
			INITIAL_PUBLIC_CONTENT_READY_CLASS,
		);
		document.documentElement.setAttribute(
			INITIAL_PUBLIC_CONTENT_READY_AT,
			String(performance.now() - 1_000),
		);
		await mount();
		await advance(100);
		expect(host.querySelector(".initial-loader")).toBeNull();
		expect(
			document.documentElement.classList.contains("initial-loader-active"),
		).toBe(false);
		await advance(6_000);
		expect(host.querySelector(".initial-loader")).toBeNull();
	});

	it("lets React win the ready-marker race without the old 700ms hold", async () => {
		document.documentElement.classList.add(
			INITIAL_PUBLIC_CONTENT_ENABLED_CLASS,
		);
		await mount();
		await advance(500);
		expect(host.querySelector(".initial-loader")).toBeNull();
		expect(
			document.documentElement.classList.contains("initial-loader-active"),
		).toBe(false);
	});

	it("keeps the original minimum for a document not admitted to the fast path", async () => {
		await mount();
		await advance(500);
		expect(host.querySelector(".initial-loader")).not.toBeNull();
		await advance(800);
		expect(host.querySelector(".initial-loader")).toBeNull();
	});

	it("does not trust a public marker still inside a hidden Flight subtree", async () => {
		document.documentElement.classList.add(
			INITIAL_PUBLIC_CONTENT_ENABLED_CLASS,
		);
		const content = document.querySelector("main");
		if (!content) throw new Error("missing fixture");
		content.hidden = true;
		await mount();
		await advance(600);
		expect(host.querySelector(".initial-loader")).not.toBeNull();
		await act(async () => {
			content.hidden = false;
		});
		await advance(500);
		expect(host.querySelector(".initial-loader")).toBeNull();
	});
});
