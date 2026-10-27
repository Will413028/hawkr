import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { __setEnvForTest, getEnv } from "@/lib/config/env";

vi.mock("../upload", () => ({
	uploadImageFile: vi.fn(),
}));

import {
	gallerySignature,
	resolveStagedImages,
} from "../resolve-staged-images";
import { uploadImageFile } from "../upload";

const mockUpload = vi.mocked(uploadImageFile);

describe("resolveStagedImages", () => {
	beforeEach(() => {
		mockUpload.mockReset();
		__setEnvForTest({
			...getEnv(),
			flavor: "prod",
			mediaBaseUrl: "https://media.example.com",
		});
	});
	afterEach(() => {
		vi.unstubAllGlobals();
		__setEnvForTest(null);
	});

	it("passes through already-staged images without fetch or upload", async () => {
		const fetchSpy = vi.fn();
		vi.stubGlobal("fetch", fetchSpy);

		const out = await resolveStagedImages([
			{ url: "u0", stagedToken: "s0" },
			{ url: "u1", stagedToken: "s1" },
		]);

		expect(out).toEqual([{ stagedToken: "s0" }, { stagedToken: "s1" }]);
		expect(fetchSpy).not.toHaveBeenCalled();
		expect(mockUpload).not.toHaveBeenCalled();
	});

	it("sends an existing image back by assetId instead of re-uploading it", async () => {
		const fetchSpy = vi.fn();
		vi.stubGlobal("fetch", fetchSpy);

		// Shape of an edit draft hydrated from GET /app/products/{id}: every
		// existing gallery entry carries the assetId the backend matches on, plus
		// one freshly uploaded image. Re-uploading the existing ones would mint
		// duplicate media assets on every save; omitting them deletes them.
		const out = await resolveStagedImages([
			{
				url: "https://media.example.com/a.jpg",
				assetId: "866542659890990933",
			},
			{
				url: "https://media.example.com/b.jpg",
				assetId: "866542662277548002",
			},
			{ url: "blob:new", stagedToken: "staged-new" },
		]);

		expect(out).toEqual([
			{ assetId: "866542659890990933" },
			{ assetId: "866542662277548002" },
			{ stagedToken: "staged-new" },
		]);
		expect(fetchSpy).not.toHaveBeenCalled();
		expect(mockUpload).not.toHaveBeenCalled();
	});

	it("restages url-only images and preserves input order regardless of resolve timing", async () => {
		const fetchSpy = vi.fn(async () => ({
			ok: true,
			blob: async () => new Blob(["x"], { type: "image/jpeg" }),
		}));
		vi.stubGlobal("fetch", fetchSpy);

		// First url-only upload resolves SLOWER than the second — proves output
		// order follows input index (Promise.all), not resolution time.
		mockUpload
			.mockImplementationOnce(
				() =>
					new Promise((resolve) =>
						setTimeout(
							() => resolve({ previewUrl: "p1", stagedToken: "restaged-u1" }),
							20,
						),
					),
			)
			.mockImplementationOnce(() =>
				Promise.resolve({ previewUrl: "p3", stagedToken: "restaged-u3" }),
			);

		const out = await resolveStagedImages([
			{ url: "u0", stagedToken: "s0" }, // already staged → passthrough
			{ url: "u1" }, // restage (slow)
			{ url: "u2", stagedToken: "s2" }, // already staged → passthrough
			{ url: "u3" }, // restage (fast)
		]);

		expect(out).toEqual([
			{ stagedToken: "s0" },
			{ stagedToken: "restaged-u1" },
			{ stagedToken: "s2" },
			{ stagedToken: "restaged-u3" },
		]);
		// only the two url-only images were downloaded + re-uploaded
		expect(fetchSpy).toHaveBeenCalledTimes(2);
		expect(fetchSpy).toHaveBeenCalledWith(
			"https://media.example.com/i/original/u1",
		);
		expect(fetchSpy).toHaveBeenCalledWith(
			"https://media.example.com/i/original/u3",
		);
		expect(mockUpload).toHaveBeenCalledTimes(2);
	});

	it("downloads an existing image through the configured origin's original preset", async () => {
		const fetchSpy = vi.fn(async () => ({
			ok: true,
			blob: async () => new Blob(["x"], { type: "image/png" }),
		}));
		vi.stubGlobal("fetch", fetchSpy);
		mockUpload.mockResolvedValue({
			previewUrl: "https://media.example.com/new.png",
			stagedToken: "restaged",
		});

		await resolveStagedImages([
			{
				url: "https://media.example.com/i/detail/public/product/1/original.png",
			},
		]);

		expect(fetchSpy).toHaveBeenCalledWith(
			"https://media.example.com/i/original/public/product/1/original.png",
		);
	});

	it("keeps a signed URL on the configured origin byte-for-byte when restaging", async () => {
		const fetchSpy = vi.fn(async () => ({
			ok: true,
			blob: async () => new Blob(["x"], { type: "image/png" }),
		}));
		vi.stubGlobal("fetch", fetchSpy);
		mockUpload.mockResolvedValue({
			previewUrl: "https://media.example.com/new.png",
			stagedToken: "restaged",
		});

		const formalUrl =
			"https://media.example.com/i/detail/public/product/1/formal-only.png?signature=kept";
		await resolveStagedImages([{ url: formalUrl }]);

		expect(fetchSpy).toHaveBeenCalledWith(formalUrl);
	});

	it("throws naming the failing image when an existing image download fails", async () => {
		vi.stubGlobal(
			"fetch",
			vi.fn(async () => ({ ok: false, status: 404 })),
		);

		await expect(
			resolveStagedImages([{ url: "https://cdn/bad.jpg" }]),
		).rejects.toThrow("無法下載既有圖片 https://cdn/bad.jpg (HTTP 404)");
		expect(mockUpload).not.toHaveBeenCalled();
	});
});

describe("gallerySignature", () => {
	it("is equal for galleries with identical url + stagedToken in order", () => {
		const a = [{ url: "u0" }, { url: "u1", stagedToken: "s1" }];
		const b = [{ url: "u0" }, { url: "u1", stagedToken: "s1" }];
		expect(gallerySignature(a)).toBe(gallerySignature(b));
	});
	it("differs on reorder", () => {
		expect(gallerySignature([{ url: "a" }, { url: "b" }])).not.toBe(
			gallerySignature([{ url: "b" }, { url: "a" }]),
		);
	});
	it("differs on add / remove", () => {
		expect(gallerySignature([{ url: "a" }])).not.toBe(
			gallerySignature([{ url: "a" }, { url: "b" }]),
		);
	});
	it("differs when a stagedToken appears (existing image replaced by upload)", () => {
		expect(gallerySignature([{ url: "a" }])).not.toBe(
			gallerySignature([{ url: "a", stagedToken: "t" }]),
		);
	});
	it("does not collide when url/token boundaries shift", () => {
		// {url:"a b", token:""} must NOT share a signature with {url:"a", token:"b"}
		expect(gallerySignature([{ url: "a b" }])).not.toBe(
			gallerySignature([{ url: "a", stagedToken: "b" }]),
		);
	});
	it("differs when the same url points at a different asset", () => {
		// Identity of an existing gallery entry is its assetId; two entries could
		// otherwise share a signature and a swap would read as "untouched".
		expect(gallerySignature([{ url: "a", assetId: "1" }])).not.toBe(
			gallerySignature([{ url: "a", assetId: "2" }]),
		);
	});
});
