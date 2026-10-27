import * as React from "react";
import { describe, expect, it, vi } from "vitest";
import {
	createHomePostDetailViewerResource,
	type HomePostDetailViewerProps,
} from "./HomePostDetailViewerLoader";

type ViewerModule = {
	default: React.ComponentType<HomePostDetailViewerProps>;
};

function deferred<T>() {
	let resolve!: (value: T) => void;
	let reject!: (reason?: unknown) => void;
	const promise = new Promise<T>((resolvePromise, rejectPromise) => {
		resolve = resolvePromise;
		reject = rejectPromise;
	});
	return { promise, resolve, reject };
}

function FakeViewer(_props: HomePostDetailViewerProps) {
	return React.createElement("div");
}

describe("home post detail viewer resource", () => {
	it("deduplicates an in-flight import and keeps a resolved component warm", async () => {
		const request = deferred<ViewerModule>();
		const importer = vi.fn(() => request.promise);
		const resource = createHomePostDetailViewerResource(importer);

		const first = resource.load();
		const second = resource.load();

		expect(first).toBe(second);
		expect(importer).toHaveBeenCalledTimes(1);
		expect(resource.peek()).toBeNull();

		request.resolve({ default: FakeViewer });
		expect(await first).toBe(FakeViewer);
		expect(resource.peek()).toBe(FakeViewer);
		expect(await resource.load()).toBe(FakeViewer);
		expect(importer).toHaveBeenCalledTimes(1);
	});

	it("releases a rejected import so a visible retry starts a fresh request", async () => {
		const recovered = deferred<ViewerModule>();
		let attempt = 0;
		const importer = vi.fn(() => {
			attempt += 1;
			return attempt === 1
				? Promise.reject(new Error("chunk unavailable"))
				: recovered.promise;
		});
		const resource = createHomePostDetailViewerResource(importer);

		await expect(resource.load()).rejects.toThrow("chunk unavailable");
		expect(resource.peek()).toBeNull();

		const retry = resource.load();
		expect(importer).toHaveBeenCalledTimes(2);
		recovered.resolve({ default: FakeViewer });
		expect(await retry).toBe(FakeViewer);
		expect(resource.peek()).toBe(FakeViewer);
	});

	it("turns a synchronous importer throw into a rejection without poisoning retry", async () => {
		let attempt = 0;
		const importer = vi.fn((): Promise<ViewerModule> => {
			attempt += 1;
			if (attempt === 1) throw new Error("sync import failure");
			return Promise.resolve({ default: FakeViewer });
		});
		const resource = createHomePostDetailViewerResource(importer);

		await expect(resource.load()).rejects.toThrow("sync import failure");
		expect(await resource.load()).toBe(FakeViewer);
		expect(importer).toHaveBeenCalledTimes(2);
	});

	it("swallows a failed speculative prewarm and leaves the real open retryable", async () => {
		let attempt = 0;
		const importer = vi.fn((): Promise<ViewerModule> => {
			attempt += 1;
			return attempt === 1
				? Promise.reject(new Error("abandoned speculative request"))
				: Promise.resolve({ default: FakeViewer });
		});
		const resource = createHomePostDetailViewerResource(importer);

		expect(resource.prewarm()).toBeUndefined();
		await new Promise((resolve) => setTimeout(resolve, 0));
		expect(resource.peek()).toBeNull();

		expect(await resource.load()).toBe(FakeViewer);
		expect(importer).toHaveBeenCalledTimes(2);
	});
});
