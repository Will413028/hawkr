// @vitest-environment jsdom

import { act, cleanup, renderHook, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { GifPreflightError } from "@/lib/media/gif-crop-preflight";
import { type GifEditMetadata, inspectGifFile } from "@/lib/media/gif-edit";
import { useGifEditor } from "./GifEditControls";

vi.mock("@/lib/media/gif-edit", async (importOriginal) => ({
	...(await importOriginal<typeof import("@/lib/media/gif-edit")>()),
	inspectGifFile: vi.fn(),
}));

const inspect = vi.mocked(inspectGifFile);
const sourceMetadata: GifEditMetadata = {
	width: 1280,
	height: 720,
	frameCount: 60,
	durationMs: 1200,
};

function sourceFile(content = "a") {
	return new File([content], "animation.gif", {
		type: "image/gif",
		lastModified: 123,
	});
}

function pendingInspection() {
	let resolve!: (metadata: GifEditMetadata) => void;
	const promise = new Promise<GifEditMetadata>((complete) => {
		resolve = complete;
	});
	return { promise, resolve };
}

async function readyEditor(metadata: GifEditMetadata = sourceMetadata) {
	inspect.mockResolvedValueOnce(metadata);
	const file = sourceFile();
	const hook = renderHook(() => useGifEditor(file, true));
	await waitFor(() => expect(hook.result.current.ready).toBe(true));
	return hook;
}

beforeEach(() => {
	inspect.mockReset();
});

afterEach(cleanup);

describe("GIF editor source lifecycle", () => {
	it("keeps a replacement locked when its predecessor finishes inspecting", async () => {
		const original = sourceFile("a");
		// Matching name, length and mtime still represent different source files.
		const replacement = sourceFile("b");
		const first = pendingInspection();
		const second = pendingInspection();
		inspect
			.mockReturnValueOnce(first.promise)
			.mockReturnValueOnce(second.promise);
		const hook = renderHook(({ file }) => useGifEditor(file, true), {
			initialProps: { file: original },
		});
		const firstSignal = inspect.mock.calls[0][1];
		expect(firstSignal?.aborted).toBe(false);
		expect(hook.result.current.ready).toBe(false);
		expect(hook.result.current.loading).toBe(true);

		hook.rerender({ file: replacement });
		const secondSignal = inspect.mock.calls[1][1];
		expect(firstSignal?.aborted).toBe(true);
		expect(secondSignal?.aborted).toBe(false);

		// Deliberately resolve after abort: the reader cannot always stop its I/O.
		await act(async () => first.resolve(sourceMetadata));
		expect(hook.result.current.ready).toBe(false);
		expect(hook.result.current.loading).toBe(true);
		expect(hook.result.current.metadata).toBeNull();

		const replacementMetadata = {
			...sourceMetadata,
			width: 360,
			durationMs: 1230,
		};
		await act(async () => second.resolve(replacementMetadata));
		expect(hook.result.current.ready).toBe(true);
		expect(hook.result.current.metadata).toEqual(replacementMetadata);
		expect(hook.result.current.endSeconds).toBe("1.23");
		hook.unmount();
		expect(secondSignal?.aborted).toBe(true);
	});

	it("hides completed metadata and edits on the first render of a new file", async () => {
		const original = sourceFile("a");
		const replacement = sourceFile("b");
		const pending = pendingInspection();
		inspect
			.mockResolvedValueOnce(sourceMetadata)
			.mockReturnValueOnce(pending.promise);
		const replacementRenders: ReturnType<typeof useGifEditor>[] = [];
		const hook = renderHook(
			({ file }) => {
				const editor = useGifEditor(file, true);
				if (file === replacement) replacementRenders.push(editor);
				return editor;
			},
			{ initialProps: { file: original } },
		);
		await waitFor(() => expect(hook.result.current.ready).toBe(true));
		act(() => {
			hook.result.current.setMaxOutput(360);
			hook.result.current.setStartSeconds("0.5");
		});

		hook.rerender({ file: replacement });
		expect(replacementRenders[0].ready).toBe(false);
		expect(replacementRenders[0].metadata).toBeNull();
		expect(replacementRenders[0].startSeconds).toBe("0");
		expect(replacementRenders[0].maxOutput).toBe("auto");

		await act(async () =>
			pending.resolve({ ...sourceMetadata, durationMs: 2300 }),
		);
		expect(hook.result.current.ready).toBe(true);
		expect(hook.result.current.endSeconds).toBe("2.3");
		expect(hook.result.current.options).not.toHaveProperty("startTimeMs");
	});

	it("aborts pending inspection when the editor unmounts", async () => {
		const pending = pendingInspection();
		inspect.mockReturnValueOnce(pending.promise);
		const file = sourceFile();
		const hook = renderHook(() => useGifEditor(file, true));
		const signal = inspect.mock.calls[0][1];
		expect(signal?.aborted).toBe(false);
		hook.unmount();
		expect(signal?.aborted).toBe(true);
		await act(async () => pending.resolve(sourceMetadata));
	});

	it("keeps inspection failures locked and exposes actionable preflight errors", async () => {
		const message = "GIF 超過處理範圍，請縮短動畫後再試。";
		inspect.mockRejectedValueOnce(
			new GifPreflightError("resource-limit", message),
		);
		const file = sourceFile();
		const hook = renderHook(() => useGifEditor(file, true));
		await waitFor(() => expect(hook.result.current.loading).toBe(false));
		expect(hook.result.current.ready).toBe(false);
		expect(hook.result.current.metadata).toBeNull();
		expect(hook.result.current.error).toBe(message);
	});
});

describe("GIF editor trim and preview validity", () => {
	it("makes automatic sizing explicit and keeps manual dimensions exact", async () => {
		const hook = await readyEditor();
		expect(hook.result.current.maxOutput).toBe("auto");
		expect(hook.result.current.options).toMatchObject({
			maxOutput: 540,
			autoFit: true,
		});
		const automaticKey = hook.result.current.key;
		act(() => hook.result.current.setMaxOutput(540));
		expect(hook.result.current.options).toMatchObject({ maxOutput: 540 });
		expect(hook.result.current.options).not.toHaveProperty("autoFit");
		expect(hook.result.current.key).not.toBe(automaticKey);
		act(() => hook.result.current.setMaxFps("original"));
		expect(hook.result.current.options).not.toHaveProperty("maxFps");
		act(() => hook.result.current.setMaxOutput("auto"));
		expect(hook.result.current.options).toMatchObject({ autoFit: true });
		expect(hook.result.current.options).not.toHaveProperty("maxFps");
	});

	it.each([
		["", "1", "missing start"],
		["0", "", "missing end"],
		[" ", "1", "blank start"],
		["NaN", "1", "nonnumeric start"],
		["0", "Infinity", "nonfinite end"],
		["-0.01", "1", "negative start"],
		["1", "1", "equal boundaries"],
		["1", "0.5", "reversed range"],
		["0", "1.21", "end beyond source duration"],
		["0.001", "0.002", "boundaries collapsing to one GIF tick"],
	])("blocks %s–%s seconds (%s)", async (start, end) => {
		const hook = await readyEditor();
		act(() => {
			hook.result.current.setStartSeconds(start);
			hook.result.current.setEndSeconds(end);
		});
		expect(hook.result.current.ready).toBe(false);
		expect(hook.result.current.rangeError).toBeTruthy();
		expect(hook.result.current.error).toBe(hook.result.current.rangeError);
	});

	it("requires at least 0.02 seconds for browser playback", async () => {
		const hook = await readyEditor();
		act(() => hook.result.current.setEndSeconds("0.01"));
		expect(hook.result.current.ready).toBe(false);
		expect(hook.result.current.rangeError).toContain("0.02");
		act(() => hook.result.current.setEndSeconds("0.02"));
		expect(hook.result.current.ready).toBe(true);
		expect(hook.result.current.error).toBeNull();
		expect(hook.result.current.options).toMatchObject({
			startTimeMs: 0,
			endTimeMs: 20,
		});
	});

	it("invalidates an existing full-animation preview when a time field is cleared", async () => {
		const hook = await readyEditor();
		const previewKey = hook.result.current.key;
		const fullAnimationOptions = hook.result.current.options;
		act(() => hook.result.current.setStartSeconds(""));
		expect(hook.result.current.ready).toBe(false);
		// Invalid input has no usable trim, yet cannot reuse the old preview key.
		expect(hook.result.current.options).toEqual(fullAnimationOptions);
		expect(hook.result.current.key).not.toBe(previewKey);
		act(() => hook.result.current.setStartSeconds("0"));
		expect(hook.result.current.ready).toBe(true);
		expect(hook.result.current.key).toBe(previewKey);
	});

	it("resets to the full source duration even if reset was used before inspection", async () => {
		const pending = pendingInspection();
		inspect.mockReturnValueOnce(pending.promise);
		const file = sourceFile();
		const hook = renderHook(() => useGifEditor(file, true));
		act(() => hook.result.current.reset());
		await act(async () =>
			pending.resolve({ ...sourceMetadata, durationMs: 1230 }),
		);
		expect(hook.result.current.ready).toBe(true);
		expect(hook.result.current.endSeconds).toBe("1.23");
		const fullAnimationKey = hook.result.current.key;

		act(() => {
			hook.result.current.setMaxOutput(360);
			hook.result.current.setMaxFps(8);
			hook.result.current.setStartSeconds("0.1");
			hook.result.current.setEndSeconds("0.9");
		});
		expect(hook.result.current.options).toMatchObject({
			maxOutput: 360,
			maxFps: 8,
			startTimeMs: 100,
			endTimeMs: 900,
		});
		act(() => hook.result.current.reset());
		expect(hook.result.current.ready).toBe(true);
		expect(hook.result.current.startSeconds).toBe("0");
		expect(hook.result.current.endSeconds).toBe("1.23");
		expect(hook.result.current.options).toEqual({
			maxOutput: 540,
			autoFit: true,
			maxFps: 12,
		});
		expect(hook.result.current.key).toBe(fullAnimationKey);
	});

	it("uses normalized zero-delay playback duration for trimming", async () => {
		const hook = await readyEditor({
			...sourceMetadata,
			frameCount: 2,
			durationMs: 200,
		});
		expect(hook.result.current.startSeconds).toBe("0");
		expect(hook.result.current.endSeconds).toBe("0.2");
		expect(hook.result.current.error).toBeNull();
		expect(hook.result.current.options).not.toHaveProperty("startTimeMs");
		expect(hook.result.current.options).not.toHaveProperty("endTimeMs");
		act(() => hook.result.current.reset());
		expect(hook.result.current.ready).toBe(true);
		expect(hook.result.current.error).toBeNull();
	});
});
