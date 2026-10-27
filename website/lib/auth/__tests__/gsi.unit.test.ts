import { afterEach, describe, expect, it, vi } from "vitest";
import {
	__resetGsiForTest,
	GsiUnavailableError,
	getGsiFailureStage,
	getGsiFirstFailureStage,
	loadGsi,
	renderGoogleButton,
} from "../gsi";

const buttonContainer = {
	getBoundingClientRect: () => ({ width: 320 }),
} as HTMLElement;

interface FakeScript {
	async: boolean;
	defer: boolean;
	onerror: ((event?: unknown) => void) | null;
	onload: (() => void) | null;
	removed: boolean;
	src: string;
	attributes: Record<string, string>;
	remove: ReturnType<typeof vi.fn>;
	setAttribute: (name: string, value: string) => void;
}

function installScriptDom() {
	const created: FakeScript[] = [];
	const appended: FakeScript[] = [];
	const windowValue: {
		google?: {
			accounts: {
				id: {
					initialize: ReturnType<typeof vi.fn>;
					renderButton: ReturnType<typeof vi.fn>;
					prompt: ReturnType<typeof vi.fn>;
				};
			};
		};
	} = {};

	const createElement = vi.fn(() => {
		const script: FakeScript = {
			async: false,
			defer: false,
			onerror: null,
			onload: null,
			removed: false,
			src: "",
			attributes: {},
			remove: vi.fn(() => {
				script.removed = true;
			}),
			setAttribute(name, value) {
				script.attributes[name] = value;
			},
		};
		created.push(script);
		return script;
	});
	const appendChild = vi.fn((script: FakeScript) => {
		appended.push(script);
		return script;
	});
	const querySelectorAll = vi.fn(() =>
		created.filter(
			(script) =>
				!script.removed &&
				script.attributes["data-hawkr-gsi-loader"] === "true",
		),
	);

	vi.stubGlobal("window", windowValue);
	vi.stubGlobal("document", {
		createElement,
		head: { appendChild },
		querySelectorAll,
	});

	return { appended, created, windowValue };
}

function installGoogleApi(
	windowValue: ReturnType<typeof installScriptDom>["windowValue"],
) {
	const id = {
		initialize: vi.fn(),
		renderButton: vi.fn(),
		prompt: vi.fn(),
	};
	windowValue.google = { accounts: { id } };
	return id;
}

afterEach(() => {
	__resetGsiForTest();
	vi.useRealTimers();
	vi.unstubAllGlobals();
});

describe("Google Identity Services diagnostics", () => {
	it("keeps a stable stage for Sentry grouping", () => {
		expect(getGsiFailureStage(new GsiUnavailableError("script_load"))).toBe(
			"script_load",
		);
		expect(getGsiFailureStage(new Error("untrusted SDK detail"))).toBe(
			"unknown",
		);
		expect(
			getGsiFirstFailureStage(
				new GsiUnavailableError("script_timeout", {
					firstFailureStage: "script_load",
				}),
			),
		).toBe("script_load");
	});

	it("passes the server's nonce to initialize, and omits it when none is given", async () => {
		const initialize = vi.fn();
		vi.stubGlobal("window", {
			google: { accounts: { id: { initialize, renderButton: vi.fn() } } },
		});

		await renderGoogleButton(buttonContainer, "client-id", vi.fn(), {
			nonce: "server-nonce",
		});
		expect(initialize).toHaveBeenLastCalledWith(
			expect.objectContaining({
				client_id: "client-id",
				nonce: "server-nonce",
			}),
		);

		await renderGoogleButton(buttonContainer, "client-id", vi.fn());
		expect(initialize.mock.calls.at(-1)?.[0]).not.toHaveProperty("nonce");
	});

	it("labels initialize failures without exposing the SDK error", async () => {
		vi.stubGlobal("window", {
			google: {
				accounts: {
					id: {
						initialize: () => {
							throw new Error("provider detail");
						},
						renderButton: vi.fn(),
					},
				},
			},
		});

		await expect(
			renderGoogleButton(buttonContainer, "client-id", vi.fn()),
		).rejects.toMatchObject({ stage: "initialize" });
	});

	it("labels render failures separately from initialization", async () => {
		vi.stubGlobal("window", {
			google: {
				accounts: {
					id: {
						initialize: vi.fn(),
						renderButton: () => {
							throw new Error("provider detail");
						},
					},
				},
			},
		});

		await expect(
			renderGoogleButton(buttonContainer, "client-id", vi.fn()),
		).rejects.toMatchObject({ stage: "render" });
	});
});

describe("Google Identity Services loader recovery", () => {
	it("shares one in-flight script between concurrent callers", async () => {
		const { appended, windowValue } = installScriptDom();

		const first = loadGsi({ timeoutMs: 1_000, retryBackoffMs: [] });
		const second = loadGsi({ timeoutMs: 1_000, retryBackoffMs: [] });

		expect(second).toBe(first);
		expect(appended).toHaveLength(1);
		installGoogleApi(windowValue);
		appended[0]?.onload?.();

		await expect(first).resolves.toMatchObject({
			attempts: 1,
			recovered: false,
		});
		await expect(second).resolves.toMatchObject({ attempts: 1 });
	});

	it("removes a failed script and automatically retries once", async () => {
		vi.useFakeTimers();
		const { appended, windowValue } = installScriptDom();
		const resultPromise = loadGsi({
			timeoutMs: 1_000,
			retryBackoffMs: [750],
		});

		const firstScript = appended[0];
		firstScript?.onerror?.();
		expect(firstScript?.removed).toBe(true);

		await vi.advanceTimersByTimeAsync(750);
		expect(appended).toHaveLength(2);
		installGoogleApi(windowValue);
		appended[1]?.onload?.();

		await expect(resultPromise).resolves.toMatchObject({
			attempts: 2,
			recovered: true,
			recoveredFrom: "script_load",
		});
	});

	it("times out both attempts, cleans up, and preserves the first failure", async () => {
		vi.useFakeTimers();
		const { appended } = installScriptDom();
		const resultPromise = loadGsi({
			timeoutMs: 8_000,
			retryBackoffMs: [750],
		});
		const rejection = expect(resultPromise).rejects.toMatchObject({
			stage: "script_timeout",
			firstFailureStage: "script_timeout",
			attempts: 2,
		});

		await vi.advanceTimersByTimeAsync(8_000);
		expect(appended[0]?.removed).toBe(true);
		await vi.advanceTimersByTimeAsync(750);
		expect(appended).toHaveLength(2);
		await vi.advanceTimersByTimeAsync(8_000);

		await rejection;
		expect(appended[1]?.removed).toBe(true);
		expect(vi.getTimerCount()).toBe(0);
	});

	it("retries when script load fires before the API exists", async () => {
		vi.useFakeTimers();
		const { appended, windowValue } = installScriptDom();
		const resultPromise = loadGsi({
			timeoutMs: 1_000,
			retryBackoffMs: [0],
		});

		appended[0]?.onload?.();
		expect(appended[0]?.removed).toBe(true);
		await vi.advanceTimersByTimeAsync(0);
		installGoogleApi(windowValue);
		appended[1]?.onload?.();

		await expect(resultPromise).resolves.toMatchObject({
			attempts: 2,
			recovered: true,
			recoveredFrom: "api_missing",
		});
	});

	it("uses an API that appears during backoff without appending another script", async () => {
		vi.useFakeTimers();
		const { appended, windowValue } = installScriptDom();
		const resultPromise = loadGsi({
			timeoutMs: 1_000,
			retryBackoffMs: [750],
		});

		appended[0]?.onerror?.();
		await vi.advanceTimersByTimeAsync(0);
		expect(vi.getTimerCount()).toBe(1);
		installGoogleApi(windowValue);
		await vi.advanceTimersByTimeAsync(750);

		expect(appended).toHaveLength(1);
		await expect(resultPromise).resolves.toMatchObject({
			attempts: 1,
			recovered: true,
			recoveredFrom: "script_load",
		});
	});

	it("does not report failure when an earlier script exposes the API late", async () => {
		vi.useFakeTimers();
		const { appended, windowValue } = installScriptDom();
		const resultPromise = loadGsi({
			timeoutMs: 1_000,
			retryBackoffMs: [50],
		});

		appended[0]?.onerror?.();
		await vi.advanceTimersByTimeAsync(50);
		expect(appended).toHaveLength(2);
		installGoogleApi(windowValue);
		appended[1]?.onerror?.();

		await expect(resultPromise).resolves.toMatchObject({
			attempts: 2,
			recovered: true,
			recoveredFrom: "script_load",
		});
		expect(appended[1]?.removed).toBe(true);
	});

	it("removes a timed-out retry when another script exposed the API", async () => {
		vi.useFakeTimers();
		const { appended, windowValue } = installScriptDom();
		const resultPromise = loadGsi({
			timeoutMs: 1_000,
			retryBackoffMs: [50],
		});

		appended[0]?.onerror?.();
		await vi.advanceTimersByTimeAsync(50);
		expect(appended).toHaveLength(2);
		installGoogleApi(windowValue);
		await vi.advanceTimersByTimeAsync(1_000);

		await expect(resultPromise).resolves.toMatchObject({
			attempts: 2,
			recovered: true,
			recoveredFrom: "script_load",
		});
		expect(appended[1]?.removed).toBe(true);
	});

	it("starts fresh after a terminal API-missing failure", async () => {
		const { appended, windowValue } = installScriptDom();
		const failed = loadGsi({ timeoutMs: 1_000, retryBackoffMs: [] });
		appended[0]?.onload?.();
		await expect(failed).rejects.toMatchObject({
			stage: "api_missing",
			attempts: 1,
		});

		const restarted = loadGsi({ timeoutMs: 1_000, retryBackoffMs: [] });
		expect(appended).toHaveLength(2);
		installGoogleApi(windowValue);
		appended[1]?.onload?.();
		await expect(restarted).resolves.toMatchObject({ attempts: 1 });
	});

	it("ignores a timed-out script's late load event", async () => {
		vi.useFakeTimers();
		const { appended, windowValue } = installScriptDom();
		const resultPromise = loadGsi({
			timeoutMs: 100,
			retryBackoffMs: [50],
		});
		const lateLoad = appended[0]?.onload;

		await vi.advanceTimersByTimeAsync(100);
		await vi.advanceTimersByTimeAsync(50);
		expect(appended).toHaveLength(2);
		installGoogleApi(windowValue);
		lateLoad?.();
		expect(appended).toHaveLength(2);
		appended[1]?.onload?.();

		await expect(resultPromise).resolves.toMatchObject({
			attempts: 2,
			recoveredFrom: "script_timeout",
		});
	});

	it("does not initialize or render after the caller aborts", async () => {
		const { appended, windowValue } = installScriptDom();
		const controller = new AbortController();
		const renderPromise = renderGoogleButton(
			buttonContainer,
			"client-id",
			vi.fn(),
			{ signal: controller.signal },
		);
		const id = installGoogleApi(windowValue);

		controller.abort();
		appended[0]?.onload?.();

		await expect(renderPromise).rejects.toMatchObject({ name: "AbortError" });
		expect(id.initialize).not.toHaveBeenCalled();
		expect(id.renderButton).not.toHaveBeenCalled();
	});
});
