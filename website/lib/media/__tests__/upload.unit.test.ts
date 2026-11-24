import { RejectReason } from "@hawkr/api/connect/upload";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { RepoFetcher } from "@/lib/api/delegated";

const mocks = vi.hoisted(() => ({
	uploadPublicFile: vi.fn(),
	uploadDelegatedPublicFile: vi.fn(),
}));

vi.mock("@hawkr/api/connect/upload", async (importOriginal) => ({
	...(await importOriginal<typeof import("@hawkr/api/connect/upload")>()),
	uploadPublicFile: mocks.uploadPublicFile,
	uploadDelegatedPublicFile: mocks.uploadDelegatedPublicFile,
}));

vi.mock("@/lib/intl/active-locale", () => ({
	activeLocale: () => "zh-TW",
}));

import { createMediaUploadRepo, uploadImageFile } from "../upload";

beforeEach(() => {
	mocks.uploadPublicFile.mockReset();
	mocks.uploadDelegatedPublicFile.mockReset();
	vi.stubGlobal("URL", {
		...URL,
		createObjectURL: () => "blob:local-preview",
	});
});

afterEach(() => {
	vi.unstubAllGlobals();
});

describe("uploadImageFile", () => {
	it("uploads the bytes through MediaAppService and previews from a local object URL", async () => {
		const file = new File([new Uint8Array([1, 2, 3])], "avatar.png", {
			type: "image/png",
		});
		mocks.uploadPublicFile.mockResolvedValue({
			state: "ready",
			stagedToken: "staged-token",
		});

		const result = await uploadImageFile(file, undefined, { compress: false });

		expect(result).toEqual({
			previewUrl: "blob:local-preview",
			stagedToken: "staged-token",
		});
		expect(mocks.uploadPublicFile).toHaveBeenCalledTimes(1);
		expect(mocks.uploadPublicFile.mock.calls[0]?.[1]).toBe(file);
		expect(mocks.uploadDelegatedPublicFile).not.toHaveBeenCalled();
	});

	it("sends a processed GIF as is, without flattening or WebP recompression", async () => {
		const bytes = new Uint8Array(2 * 1024 * 1024 + 1);
		bytes.set([0x47, 0x49, 0x46, 0x38, 0x39, 0x61]);
		const file = new File([bytes], "animation-crop.gif", { type: "image/gif" });
		mocks.uploadPublicFile.mockResolvedValue({
			state: "ready",
			stagedToken: "gif-token",
		});

		await uploadImageFile(file);

		expect(mocks.uploadPublicFile.mock.calls[0]?.[1]).toBe(file);
	});

	it("throws the RejectReason catalog text when the upload is rejected", async () => {
		const file = new File([new Uint8Array([1])], "a.png", {
			type: "image/png",
		});
		mocks.uploadPublicFile.mockResolvedValue({
			state: "rejected",
			stagedToken: "t",
			reason: RejectReason.UNRECOGNIZED_FORMAT,
		});

		await expect(
			uploadImageFile(file, undefined, { compress: false }),
		).rejects.toThrow("無法辨識檔案格式");
	});

	it("rejects non-image files before any request", async () => {
		const file = new File(["x"], "a.txt", { type: "text/plain" });
		await expect(uploadImageFile(file)).rejects.toThrow("僅支援圖片檔案");
		expect(mocks.uploadPublicFile).not.toHaveBeenCalled();
	});
});

describe("delegated media upload repo", () => {
	it("runs begin, complete and get through the injected fetcher under the BFF root", async () => {
		const calls: { path: string; method: string; json?: unknown }[] = [];
		const fetcher = (async (
			path: string,
			opts: { method?: string; json?: unknown } = {},
		) => {
			calls.push({ path, method: opts.method ?? "GET", json: opts.json });
			if (path === "/media/upload_sessions") {
				return {
					code: 0,
					data: {
						stagedToken: "tok.1",
						uploadUrl: "https://r2/put",
						uploadHeaders: {},
					},
				};
			}
			if (opts.method === "POST") return { code: 0, data: {} };
			return { code: 0, data: { state: 3, rejectReason: null } };
		}) as RepoFetcher;
		mocks.uploadDelegatedPublicFile.mockImplementation(async (client) => {
			const begin = await client.beginUpload({
				size: BigInt(3),
				contentType: "image/png",
				md5: "m",
			});
			await client.completeUpload({ stagedToken: begin.stagedToken });
			await client.getUpload({ stagedToken: begin.stagedToken });
			return { state: "ready", stagedToken: begin.stagedToken };
		});
		const file = new File([new Uint8Array([1, 2, 3])], "a.png", {
			type: "image/png",
		});

		const result = await createMediaUploadRepo(fetcher, {
			root: "/media/upload_sessions",
		}).uploadImageFile(file, undefined, { compress: false });

		expect(result.stagedToken).toBe("tok.1");
		expect(calls).toEqual([
			{
				path: "/media/upload_sessions",
				method: "POST",
				json: { size: "3", contentType: "image/png", md5: "m" },
			},
			{
				path: "/media/upload_sessions/tok.1/complete",
				method: "POST",
				json: undefined,
			},
			{ path: "/media/upload_sessions/tok.1", method: "GET", json: undefined },
		]);
		expect(mocks.uploadPublicFile).not.toHaveBeenCalled();
	});
});
