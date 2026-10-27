import { describe, expect, it, vi } from "vitest";
import {
	safeSellerWebsiteUrl,
	streamSellerMatchChat,
} from "./seller-match-chat";

vi.mock("@/lib/api/client", async (importOriginal) => {
	const actual = await importOriginal<typeof import("@/lib/api/client")>();
	return {
		...actual,
		apiClient: {
			...actual.apiClient,
			buildUrl: vi.fn(
				() => "https://gw.test/api/v1/app/buyers/me/assistant/seller-match",
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

/** 一次 `search_candidate_sellers` tool call 的完整 AG-UI 三段frame，跟
 * 2026-08-18 對 dev 的 live 探測結果同形狀。 */
function toolResultFrames(toolCallId: string, content: unknown): string[] {
	return [
		aguiFrame({
			type: "TOOL_CALL_START",
			toolCallId,
			toolCallName: "tool_result",
		}),
		aguiFrame({ type: "TOOL_CALL_END", toolCallId }),
		aguiFrame({
			type: "TOOL_CALL_RESULT",
			messageId: "msg-golden",
			toolCallId,
			content: JSON.stringify(content),
		}),
	];
}

function doneFrame(result: unknown): string {
	return aguiFrame({
		type: "RUN_FINISHED",
		threadId: "thread-golden",
		runId: "r1",
		result,
	});
}

describe("streamSellerMatchChat", () => {
	it("sends message and sessionId (camelCase) in the request body", async () => {
		const fetchMock = vi.fn().mockResolvedValue(sseResponse([doneFrame({})]));
		vi.stubGlobal("fetch", fetchMock);

		await streamSellerMatchChat(
			{ message: "我要找台南水果禮盒", sessionId: "s1" },
			{
				onText: vi.fn(),
				onCandidates: vi.fn(),
				onDone: vi.fn(),
				onError: vi.fn(),
			},
		);

		const body = JSON.parse(fetchMock.mock.calls[0][1].body as string);
		expect(body).toEqual({ message: "我要找台南水果禮盒", sessionId: "s1" });
	});

	it("parses a seller_candidates tool_result into SellerCandidate[]", async () => {
		const privateEmail = "private-candidate@example.test";
		const privatePhone = "0912-345-678";
		const privateSocial = "https://social.example.test/private-candidate";
		vi.stubGlobal(
			"fetch",
			vi.fn().mockResolvedValue(
				sseResponse(
					toolResultFrames("call-1", {
						type: "seller_candidates",
						candidates: [
							{
								id: 1,
								brand_name: "十八養場",
								category: "水果",
								product_summary: "水梨禮盒",
								region: "台南",
								positioning: "在地直送",
								research_summary: "",
								public_website: "https://x.com",
								public_email: privateEmail,
								public_phone: privatePhone,
								public_social: privateSocial,
							},
						],
					}),
				),
			),
		);
		const onCandidates = vi.fn();
		await streamSellerMatchChat(
			{ message: "hi" },
			{ onText: vi.fn(), onCandidates, onDone: vi.fn(), onError: vi.fn() },
		);
		expect(onCandidates).toHaveBeenCalledWith([
			{
				id: "1",
				brandName: "十八養場",
				category: "水果",
				productSummary: "水梨禮盒",
				region: "台南",
				positioning: "在地直送",
				researchSummary: "",
				publicWebsite: "https://x.com",
			},
		]);
		const serializedCandidates = JSON.stringify(onCandidates.mock.calls[0][0]);
		expect(serializedCandidates).not.toContain(privateEmail);
		expect(serializedCandidates).not.toContain(privatePhone);
		expect(serializedCandidates).not.toContain(privateSocial);
		expect(onCandidates.mock.calls[0][0][0]).not.toHaveProperty("publicEmail");
		expect(onCandidates.mock.calls[0][0][0]).not.toHaveProperty("publicPhone");
		expect(onCandidates.mock.calls[0][0][0]).not.toHaveProperty("publicSocial");
	});

	it("accumulates candidates across multiple tool_result events in one turn instead of replacing them", async () => {
		// Live-reproduced 2026-08-06 (buyer-flow-verify flow 16, scenario D):
		// the LLM split "雞肉或蛋品類的供應商" into two search_candidate_sellers
		// calls in the same turn — the second call legitimately found nothing,
		// but the first call's real candidates must not be lost.
		vi.stubGlobal(
			"fetch",
			vi.fn().mockResolvedValue(
				sseResponse([
					...toolResultFrames("call-1", {
						type: "seller_candidates",
						candidates: [
							{
								id: 1,
								brand_name: "範例丁商行",
								category: "蛋品",
								product_summary: "",
								region: "",
								positioning: "",
								research_summary: "",
								public_website: "",
								public_email: "",
								public_phone: "",
								public_social: "",
							},
						],
					}),
					...toolResultFrames("call-2", {
						type: "seller_candidates",
						candidates: [],
					}),
				]),
			),
		);
		const onCandidates = vi.fn();
		await streamSellerMatchChat(
			{ message: "hi" },
			{ onText: vi.fn(), onCandidates, onDone: vi.fn(), onError: vi.fn() },
		);
		expect(onCandidates).toHaveBeenCalledTimes(2);
		// Second call (the empty batch) must still carry the first call's
		// candidate forward — this is the exact bug: a naive implementation
		// would call onCandidates([]) here and the caller would render zero.
		expect(onCandidates.mock.calls[1][0]).toHaveLength(1);
		expect(onCandidates.mock.calls[1][0][0].id).toBe("1");
	});

	it("dedupes by id when the same candidate appears in more than one tool_result batch", async () => {
		vi.stubGlobal(
			"fetch",
			vi.fn().mockResolvedValue(
				sseResponse([
					...toolResultFrames("call-1", {
						type: "seller_candidates",
						candidates: [
							{
								id: 1,
								brand_name: "範例丁商行",
								category: "蛋品",
								product_summary: "",
								region: "",
								positioning: "",
								research_summary: "",
								public_website: "",
								public_email: "",
								public_phone: "",
								public_social: "",
							},
						],
					}),
					...toolResultFrames("call-2", {
						type: "seller_candidates",
						candidates: [
							{
								id: 1,
								brand_name: "範例丁商行",
								category: "蛋品",
								product_summary: "",
								region: "",
								positioning: "",
								research_summary: "",
								public_website: "",
								public_email: "",
								public_phone: "",
								public_social: "",
							},
							{
								id: 2,
								brand_name: "雞肉行",
								category: "肉品",
								product_summary: "",
								region: "",
								positioning: "",
								research_summary: "",
								public_website: "",
								public_email: "",
								public_phone: "",
								public_social: "",
							},
						],
					}),
				]),
			),
		);
		const onCandidates = vi.fn();
		await streamSellerMatchChat(
			{ message: "hi" },
			{ onText: vi.fn(), onCandidates, onDone: vi.fn(), onError: vi.fn() },
		);
		const finalCandidates = onCandidates.mock.calls[1][0];
		expect(finalCandidates).toHaveLength(2);
		expect(finalCandidates.map((c: { id: string }) => c.id)).toEqual([
			"1",
			"2",
		]);
	});

	it("preserves a full-width 64-bit TSID id as an exact string (no Number precision loss)", async () => {
		// 19-digit TSID > Number.MAX_SAFE_INTEGER (2^53 ≈ 9.007e15). A bare
		// JSON.parse would silently round this to …344560 or similar; the fix
		// routes tool_result parsing through lib/api/json.ts's parseJsonSafe so
		// the id round-trips byte-for-byte as a string.
		const tsid = "741058396217344558";
		vi.stubGlobal(
			"fetch",
			vi.fn().mockResolvedValue(
				sseResponse(
					// content is itself a JSON string on the wire (TOOL_CALL_RESULT.content
					// is a string field) — build it by hand here so the 19-digit id
					// survives as a bare JSON number the way the real ai-service sends it,
					// not pre-stringified through JSON.stringify (which would already be
					// lossy in this very test file, defeating the point of the assertion).
					[
						aguiFrame({
							type: "TOOL_CALL_START",
							toolCallId: "call-1",
							toolCallName: "tool_result",
						}),
						`data: {"type":"TOOL_CALL_RESULT","messageId":"msg-golden","toolCallId":"call-1","content":"{\\"type\\":\\"seller_candidates\\",\\"candidates\\":[{\\"id\\":${tsid},\\"brand_name\\":\\"大和果園\\",\\"category\\":\\"水果\\",\\"product_summary\\":\\"\\",\\"region\\":\\"\\",\\"positioning\\":\\"\\",\\"research_summary\\":\\"\\",\\"public_website\\":\\"\\",\\"public_email\\":\\"\\",\\"public_phone\\":\\"\\",\\"public_social\\":\\"\\"}]}"}\n\n`,
					],
				),
			),
		);
		const onCandidates = vi.fn();
		await streamSellerMatchChat(
			{ message: "hi" },
			{ onText: vi.fn(), onCandidates, onDone: vi.fn(), onError: vi.fn() },
		);
		expect(onCandidates.mock.calls[0][0][0].id).toBe(tsid);
		expect(typeof onCandidates.mock.calls[0][0][0].id).toBe("string");
	});

	it("ignores a tool_result whose type is not seller_candidates", async () => {
		vi.stubGlobal(
			"fetch",
			vi
				.fn()
				.mockResolvedValue(
					sseResponse(
						toolResultFrames("call-1", { type: "order_list", orders: [] }),
					),
				),
		);
		const onCandidates = vi.fn();
		await streamSellerMatchChat(
			{ message: "hi" },
			{ onText: vi.fn(), onCandidates, onDone: vi.fn(), onError: vi.fn() },
		);
		expect(onCandidates).not.toHaveBeenCalled();
	});

	it("reads sessionId (camelCase) from the done payload", async () => {
		vi.stubGlobal(
			"fetch",
			vi.fn().mockResolvedValue(sseResponse([doneFrame({ sessionId: "s42" })])),
		);
		const onDone = vi.fn();
		await streamSellerMatchChat(
			{ message: "hi" },
			{ onText: vi.fn(), onCandidates: vi.fn(), onDone, onError: vi.fn() },
		);
		expect(onDone).toHaveBeenCalledWith("s42");
	});

	it("omits sessionId from the request body when not provided", async () => {
		const fetchMock = vi.fn().mockResolvedValue(sseResponse([doneFrame({})]));
		vi.stubGlobal("fetch", fetchMock);
		await streamSellerMatchChat(
			{ message: "hi" },
			{
				onText: vi.fn(),
				onCandidates: vi.fn(),
				onDone: vi.fn(),
				onError: vi.fn(),
			},
		);
		const body = JSON.parse(fetchMock.mock.calls[0][1].body as string);
		expect(body).not.toHaveProperty("sessionId");
	});
});

describe("safeSellerWebsiteUrl", () => {
	it("normalizes ordinary public web destinations", () => {
		expect(safeSellerWebsiteUrl("example.com/seller")).toBe(
			"https://example.com/seller",
		);
		expect(safeSellerWebsiteUrl(" https://example.test/brand ")).toBe(
			"https://example.test/brand",
		);
		expect(safeSellerWebsiteUrl("http://example.test/brand")).toBe(
			"http://example.test/brand",
		);
	});

	it("rejects executable, credential-bearing, malformed, and empty values", () => {
		for (const value of [
			"javascript:alert(1)",
			"data:text/html,<script>alert(1)</script>",
			"ftp://example.test/file",
			"https://user:password@example.test/private",
			"https://",
			"",
			"   ",
		]) {
			expect(safeSellerWebsiteUrl(value), value).toBeNull();
		}
	});
});
