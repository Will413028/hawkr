import { create } from "@bufbuild/protobuf";
import { Code, ConnectError } from "@connectrpc/connect";
import { ErrorInfoSchema } from "@hawkr/api/gen/google/rpc/error_details_pb";
import { ErrorReason } from "@hawkr/api/gen/hawkr/common/v1alpha1/error_reason_pb";
import {
	AiStyleCategory,
	type TransformImageRequest,
} from "@hawkr/api/gen/hawkr/imageai/v1alpha1/imageai_pb";
import { describe, expect, it, vi } from "vitest";
import {
	AiEnhanceError,
	AiQuotaExceededError,
	enhanceImage,
	fetchAiStyles,
} from "./ai-image";
import { AI_IMAGE_MAX_BYTES, fitToLimit } from "./ai-image-downsize";

const server = vi.hoisted(() => ({
	transform: undefined as undefined | ((req: unknown) => unknown),
	styles: undefined as undefined | ((req: unknown) => unknown),
	requests: [] as TransformImageRequest[],
}));

vi.mock("../connect/imageai", async () => {
	const { createClient, createRouterTransport } = await import(
		"@connectrpc/connect"
	);
	const { ImageAiAppService } = await import(
		"@hawkr/api/gen/hawkr/imageai/v1alpha1/imageai_pb"
	);
	return {
		imageAiAppClient: () =>
			createClient(
				ImageAiAppService,
				createRouterTransport((router) =>
					router.service(ImageAiAppService, {
						transformImage: (req) => server.transform?.(req) as never,
						listAiStyles: (req) => server.styles?.(req) as never,
					}),
				),
			),
	};
});

const defaults = (code: Code) => `default-${code}`;

function failWith(code: Code, reason?: ErrorReason, message = "server text") {
	return new ConnectError(
		message,
		code,
		undefined,
		reason === undefined
			? []
			: [
					{
						desc: ErrorInfoSchema,
						value: create(ErrorInfoSchema, {
							domain: "hawkr",
							reason: ErrorReason[reason],
						}),
					},
				],
	);
}

describe("enhanceImage reason classification", () => {
	const run = () =>
		enhanceImage(new Blob(["x"], { type: "image/png" }), {
			prompt: "p",
			defaults,
		}).catch((e) => e);

	it("IMAGE_QUOTA_EXCEEDED is AiQuotaExceededError, not retryable", async () => {
		server.transform = () => {
			throw failWith(Code.ResourceExhausted, ErrorReason.IMAGE_QUOTA_EXCEEDED);
		};
		const err = await run();
		expect(err).toBeInstanceOf(AiQuotaExceededError);
		expect((err as AiEnhanceError).reason).toBe(
			ErrorReason.IMAGE_QUOTA_EXCEEDED,
		);
		expect((err as AiEnhanceError).retryable).toBe(false);
	});

	it("resource_exhausted without a reason (concurrency cap) is not the quota", async () => {
		server.transform = () => {
			throw failWith(Code.ResourceExhausted);
		};
		const err = await run();
		expect(err).toBeInstanceOf(AiEnhanceError);
		expect(err).not.toBeInstanceOf(AiQuotaExceededError);
	});

	it("IMAGE_NOT_PRODUCED is retryable; safety and recitation are not", async () => {
		server.transform = () => {
			throw failWith(Code.Unavailable, ErrorReason.IMAGE_NOT_PRODUCED);
		};
		expect(((await run()) as AiEnhanceError).retryable).toBe(true);
		for (const reason of [
			ErrorReason.IMAGE_SAFETY_BLOCKED,
			ErrorReason.IMAGE_RECITATION_BLOCKED,
		]) {
			server.transform = () => {
				throw failWith(Code.InvalidArgument, reason);
			};
			const err = (await run()) as AiEnhanceError;
			expect(err.reason).toBe(reason);
			expect(err.retryable).toBe(false);
		}
	});

	it("shows the server's message, and the catalog default when there is none", async () => {
		server.transform = () => {
			throw failWith(Code.InvalidArgument, ErrorReason.IMAGE_SAFETY_BLOCKED);
		};
		expect(((await run()) as Error).message).toBe(
			`default-${Code.InvalidArgument}`,
		);
	});

	it("sends the bytes inline with the prompt and returns the quota counters", async () => {
		server.requests.length = 0;
		server.transform = (req) => {
			server.requests.push(req as TransformImageRequest);
			return {
				imageData: new Uint8Array([1, 2, 3]),
				mimeType: "image/png",
				weeklyUsage: 3,
				weeklyLimit: 10,
			};
		};
		const out = await enhanceImage(new Blob(["abc"], { type: "image/png" }), {
			prompt: "  fresh  ",
			defaults,
		});
		expect(out.weeklyUsage).toBe(3);
		expect(out.weeklyLimit).toBe(10);
		expect(out.blob.type).toBe("image/png");
		expect(server.requests[0].prompt).toBe("fresh");
		expect(server.requests[0].mimeType).toBe("image/png");
		expect(server.requests[0].imageData.length).toBe(3);
	});
});

describe("fetchAiStyles", () => {
	it("maps the proto styles and quota, and asks for the category", async () => {
		let asked: unknown;
		server.styles = (req) => {
			asked = req;
			return {
				defaultPrompt: "dp",
				weeklyUsage: 2,
				weeklyLimit: 10,
				styles: [
					{
						id: "s1",
						name: "n1",
						label: "L1",
						prompt: "P1",
						displayOrder: 1,
						thumbnail: { uriTemplate: "https://img.test/{preset}/a.jpg" },
					},
					{ id: "s2", name: "n2", label: "L2", prompt: "" },
				],
			};
		};
		const list = await fetchAiStyles("post");
		expect((asked as { category: number }).category).toBe(AiStyleCategory.POST);
		expect(list.weeklyLimit).toBe(10);
		expect(list.weeklyUsage).toBe(2);
		expect(list.defaultPrompt).toBe("dp");
		expect(list.styles).toEqual([
			{
				id: "s1",
				label: "L1",
				prompt: "P1",
				thumbnailUrl: "https://img.test/thumb/a.jpg",
			},
		]);
	});
});

describe("fitToLimit", () => {
	it("returns a source within the limit untouched", async () => {
		const src = new Blob(["x"]);
		const encode = vi.fn();
		expect(await fitToLimit(src, encode)).toBe(src);
		expect(encode).not.toHaveBeenCalled();
	});

	it("steps down until the encoding fits", async () => {
		const src = new Blob([new Uint8Array(AI_IMAGE_MAX_BYTES + 1)]);
		const sizes = [AI_IMAGE_MAX_BYTES + 5, AI_IMAGE_MAX_BYTES, 10];
		const encode = vi.fn(
			async () => new Blob([new Uint8Array(sizes.shift() ?? 0)]),
		);
		const out = await fitToLimit(src, encode);
		expect(encode).toHaveBeenCalledTimes(2);
		expect(out.size).toBe(AI_IMAGE_MAX_BYTES);
	});

	it("throws when no step fits, instead of sending a request the server rejects", async () => {
		const src = new Blob([new Uint8Array(AI_IMAGE_MAX_BYTES + 1)]);
		await expect(
			fitToLimit(
				src,
				async () => new Blob([new Uint8Array(AI_IMAGE_MAX_BYTES + 1)]),
			),
		).rejects.toThrow(RangeError);
	});
});
