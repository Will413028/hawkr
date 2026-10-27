import { describe, expect, it, vi } from "vitest";
import { streamAiListingChat } from "./ai-listing-chat";

vi.mock("@/lib/api/client", async (importOriginal) => {
	const actual = await importOriginal<typeof import("@/lib/api/client")>();
	return {
		...actual,
		apiClient: {
			...actual.apiClient,
			buildUrl: vi.fn(
				() =>
					"https://gw.test/api/v1/app/sellers/me/assistant/product-listing/chat",
			),
		},
	};
});

function sseResponse(rawChunks: string[]): Response {
	const encoder = new TextEncoder();
	const stream = new ReadableStream<Uint8Array>({
		start(controller) {
			for (const chunk of rawChunks) controller.enqueue(encoder.encode(chunk));
			controller.close();
		},
	});
	return new Response(stream, { status: 200 });
}

function aguiFrame(event: Record<string, unknown>): string {
	return `data: ${JSON.stringify(event)}\n\n`;
}

describe("streamAiListingChat (regression — behavior must survive the stream-client extraction)", () => {
	it("sends sessionId/imageUrl in camelCase", async () => {
		const fetchMock = vi.fn().mockResolvedValue(
			sseResponse([
				aguiFrame({
					type: "RUN_FINISHED",
					threadId: "thread-golden",
					runId: "r1",
				}),
			]),
		);
		vi.stubGlobal("fetch", fetchMock);

		await streamAiListingChat(
			{ message: "hi", sessionId: "s1", imageUrl: "https://x/img.png" },
			{
				onText: vi.fn(),
				onDraftPatch: vi.fn(),
				onDone: vi.fn(),
				onError: vi.fn(),
			},
		);

		const body = JSON.parse(fetchMock.mock.calls[0][1].body as string);
		expect(body).toEqual({
			message: "hi",
			sessionId: "s1",
			imageUrl: "https://x/img.png",
		});
		expect(body).not.toHaveProperty("session_id");
		expect(body).not.toHaveProperty("image_url");
	});

	it("parses a draft_patch CUSTOM event into AiDraftPatch and calls onDraftPatch", async () => {
		vi.stubGlobal(
			"fetch",
			vi.fn().mockResolvedValue(
				sseResponse([
					aguiFrame({
						type: "CUSTOM",
						name: "draft_patch",
						value: { name: "高麗菜" },
					}),
				]),
			),
		);
		const onDraftPatch = vi.fn();
		await streamAiListingChat(
			{ message: "hi" },
			{ onText: vi.fn(), onDraftPatch, onDone: vi.fn(), onError: vi.fn() },
		);
		expect(onDraftPatch).toHaveBeenCalledWith({ name: "高麗菜" });
	});

	it("ignores a CUSTOM event whose name isn't draft_patch", async () => {
		vi.stubGlobal(
			"fetch",
			vi.fn().mockResolvedValue(
				sseResponse([
					aguiFrame({
						type: "CUSTOM",
						name: "reasoning",
						value: "some chain-of-thought text",
					}),
				]),
			),
		);
		const onDraftPatch = vi.fn();
		await streamAiListingChat(
			{ message: "hi" },
			{ onText: vi.fn(), onDraftPatch, onDone: vi.fn(), onError: vi.fn() },
		);
		expect(onDraftPatch).not.toHaveBeenCalled();
	});

	it("reads sessionId (camelCase) from the done payload", async () => {
		vi.stubGlobal(
			"fetch",
			vi.fn().mockResolvedValue(
				sseResponse([
					aguiFrame({
						type: "RUN_FINISHED",
						threadId: "thread-golden",
						runId: "r1",
						result: { sessionId: "s99" },
					}),
				]),
			),
		);
		const onDone = vi.fn();
		await streamAiListingChat(
			{ message: "hi" },
			{ onText: vi.fn(), onDraftPatch: vi.fn(), onDone, onError: vi.fn() },
		);
		expect(onDone).toHaveBeenCalledWith("s99");
	});
});
