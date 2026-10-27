import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { __setEnvForTest, getEnv } from "@/lib/config/env";

const apiMocks = vi.hoisted(() => ({
	authedFetch: vi.fn(),
}));

vi.mock("@/lib/api/client", () => ({
	authedFetch: apiMocks.authedFetch,
}));

import { uploadImageFile } from "../upload";

interface RecordedUploadRequest {
	method: string;
	url: string;
	headers: Record<string, string>;
	body: Document | XMLHttpRequestBodyInit | null;
}

const recordedUploadRequests: RecordedUploadRequest[] = [];

class RecordingSuccessfulUploadXhr {
	status = 204;
	upload = { onprogress: null as ((event: ProgressEvent) => void) | null };
	onload: (() => void) | null = null;
	onerror: (() => void) | null = null;
	onabort: (() => void) | null = null;
	private readonly request: RecordedUploadRequest = {
		method: "",
		url: "",
		headers: {},
		body: null,
	};

	constructor() {
		recordedUploadRequests.push(this.request);
	}

	open(method: string, url: string) {
		this.request.method = method;
		this.request.url = url;
	}

	setRequestHeader(name: string, value: string) {
		this.request.headers[name.toLowerCase()] = value;
	}

	send(body: Document | XMLHttpRequestBodyInit | null = null) {
		this.request.body = body;
		queueMicrotask(() => this.onload?.());
	}
}

beforeEach(() => {
	apiMocks.authedFetch.mockReset();
	recordedUploadRequests.length = 0;
	vi.stubGlobal("XMLHttpRequest", RecordingSuccessfulUploadXhr);
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

describe("upload preview media", () => {
	it("presigns, PUTs the exact destination, completes, and returns only the normalized display URL", async () => {
		const uploadUrl =
			"https://hawkr-media.account.r2.cloudflarestorage.com/public/user_avatar/1/avatar.png?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Signature=prod-signature";
		const file = new File([new Uint8Array([1, 2, 3])], "avatar.png", {
			type: "image/png",
		});
		apiMocks.authedFetch
			.mockResolvedValueOnce({
				data: {
					sessionId: "session-1",
					uploadUrl,
					expiresIn: "10m",
					isSecure: false,
				},
			})
			.mockResolvedValueOnce({
				data: {
					previewUrl:
						"https://media.example.com/public/user_avatar/1/avatar.png",
					stagedToken: "staged-token",
				},
			});

		const result = await uploadImageFile(file, undefined, { compress: false });

		expect(apiMocks.authedFetch).toHaveBeenNthCalledWith(
			1,
			"/app/media/upload_sessions",
			{
				method: "POST",
				json: {
					contentType: "image/png",
					fileSize: "3",
					filename: "avatar.png",
				},
			},
		);
		expect(recordedUploadRequests).toEqual([
			{
				method: "PUT",
				url: uploadUrl,
				headers: { "content-type": "image/png" },
				body: file,
			},
		]);
		expect(apiMocks.authedFetch).toHaveBeenNthCalledWith(
			2,
			"/app/media/upload_sessions/session-1/complete",
			{ method: "POST" },
		);
		expect(apiMocks.authedFetch).toHaveBeenCalledTimes(2);
		expect(result).toEqual({
			previewUrl:
				"https://media.example.com/i/original/public/user_avatar/1/avatar.png",
			stagedToken: "staged-token",
		});
	});

	it.each([
		"https://test-media.example.com/public/user_avatar/1/avatar.png",
		"https://uploads.example.net/bucket-a/public/avatar.png",
		"https://account.example.net/public/avatar.png",
	])("PUTs to whatever destination the backend presigned, without a host guard of its own: %s", async (uploadUrl) => {
		apiMocks.authedFetch
			.mockResolvedValueOnce({
				data: {
					sessionId: "session-1",
					uploadUrl,
					expiresIn: "10m",
					isSecure: false,
				},
			})
			.mockResolvedValueOnce({
				data: {
					previewUrl: "public/user_avatar/1/avatar.png",
					stagedToken: "staged-token",
				},
			});
		const file = new File([new Uint8Array([1, 2, 3])], "avatar.png", {
			type: "image/png",
		});

		await uploadImageFile(file, undefined, { compress: false });

		expect(recordedUploadRequests.map((request) => request.url)).toEqual([
			uploadUrl,
		]);
	});

	it("builds the preview from the configured media origin, and not at all when none is configured", async () => {
		const respond = () =>
			apiMocks.authedFetch
				.mockResolvedValueOnce({
					data: {
						sessionId: "s",
						uploadUrl: "https://uploads.example.net/a",
						expiresIn: "10m",
						isSecure: false,
					},
				})
				.mockResolvedValueOnce({
					data: { previewUrl: "public/a.png", stagedToken: "t" },
				});
		const file = () =>
			new File([new Uint8Array([1])], "a.png", { type: "image/png" });

		respond();
		expect(
			(await uploadImageFile(file(), undefined, { compress: false }))
				.previewUrl,
		).toBe("https://media.example.com/i/original/public/a.png");

		__setEnvForTest({ ...getEnv(), mediaBaseUrl: "" });
		respond();
		expect(
			(await uploadImageFile(file(), undefined, { compress: false }))
				.previewUrl,
		).toBe("");
	});

	it("sends a processed GIF to Media Service without flattening or WebP recompression", async () => {
		const uploadUrl =
			"https://hawkr-media.account.r2.cloudflarestorage.com/public/post/1/animation.gif?X-Amz-Signature=prod-signature";
		const bytes = new Uint8Array(2 * 1024 * 1024 + 1);
		bytes.set([0x47, 0x49, 0x46, 0x38, 0x39, 0x61]);
		const file = new File([bytes], "animation-crop.gif", {
			type: "image/gif",
		});
		apiMocks.authedFetch
			.mockResolvedValueOnce({
				data: {
					sessionId: "gif-session",
					uploadUrl,
					expiresIn: "10m",
					isSecure: false,
				},
			})
			.mockResolvedValueOnce({
				data: {
					previewUrl:
						"https://media.example.com/i/original/public/post/1/animation.gif",
					stagedToken: "gif-staged-token",
				},
			});

		await uploadImageFile(file);

		expect(apiMocks.authedFetch).toHaveBeenNthCalledWith(
			1,
			"/app/media/upload_sessions",
			{
				method: "POST",
				json: {
					contentType: "image/gif",
					fileSize: String(file.size),
					filename: "animation-crop.gif",
				},
			},
		);
		expect(recordedUploadRequests).toHaveLength(1);
		expect(recordedUploadRequests[0]).toMatchObject({
			method: "PUT",
			url: uploadUrl,
			headers: { "content-type": "image/gif" },
			body: file,
		});
	});
});
