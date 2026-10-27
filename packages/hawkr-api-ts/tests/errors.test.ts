import { readFileSync } from "node:fs";
import { join } from "node:path";
import { create } from "@bufbuild/protobuf";
import {
	DurationSchema,
	MethodOptions_IdempotencyLevel,
} from "@bufbuild/protobuf/wkt";
import { Code, ConnectError } from "@connectrpc/connect";
import { describe, expect, it } from "vitest";
import {
	classifyError,
	type DefaultErrorText,
	type ErrorFacts,
	errorText,
	factsFromError,
	isNoResponse,
	metadataList,
	userMessage,
} from "../src/connect/errors";
import {
	BadRequest_FieldViolationSchema,
	BadRequestSchema,
	ErrorInfoSchema,
	LocalizedMessageSchema,
	RetryInfoSchema,
} from "../src/gen/google/rpc/error_details_pb";
import { ErrorReason } from "../src/gen/hawkr/common/v1alpha1/error_reason_pb";
import { ContentPublicService } from "../src/gen/hawkr/content/v1alpha1/content_pb";

// 四個 client 共用的情境表（ADR 2026-09-25-api-error-model 的 D7）。
const table = JSON.parse(
	readFileSync(
		join(import.meta.dirname, "../../../api/testdata/error-dispatch.json"),
		"utf8",
	),
) as {
	scenarios: Array<{
		name: string;
		input: {
			method: { idempotency: string; requestId: boolean };
			error: {
				code: string;
				noResponse?: boolean;
				errorInfo?: { reason: string; domain: string };
				localizedMessage?: string;
				badRequest?: Array<{ field: string; message: string }>;
				retryDelayMs?: number;
			};
		};
		expected: {
			action: string;
			message?: string | null;
			retryDelayMs?: number | null;
			reason?: string;
			fields?: Array<{ field: string; message: string }>;
		};
	}>;
};

// 預設文字由呼叫端從目錄注入；這裡的替身讓「用了預設文字」可以和任何伺服器文字區分。
const defaults: DefaultErrorText = (code) => `預設:${Code[code]}`;

function codeOf(name: string): Code {
	const pascal = name
		.split("_")
		.map((p) => p[0].toUpperCase() + p.slice(1))
		.join("");
	const code = (Code as unknown as Record<string, number>)[pascal];
	if (typeof code !== "number") throw new Error(`情境表的 code ${name} 找不到`);
	return code;
}

function facts(input: (typeof table.scenarios)[number]["input"]): ErrorFacts {
	const e = input.error;
	return {
		code: codeOf(e.code),
		noResponse: e.noResponse ?? false,
		idempotency:
			MethodOptions_IdempotencyLevel[
				input.method.idempotency === "UNKNOWN"
					? "IDEMPOTENCY_UNKNOWN"
					: (input.method.idempotency as "NO_SIDE_EFFECTS" | "IDEMPOTENT")
			],
		hasRequestId: input.method.requestId,
		reason: e.errorInfo?.reason,
		reasonDomain: e.errorInfo?.domain,
		metadata: {},
		localizedMessage: e.localizedMessage,
		fieldErrors: e.badRequest ?? [],
		retryDelayMs: e.retryDelayMs,
	};
}

describe("情境表", () => {
	it("情境表不是空的", () => {
		expect(table.scenarios.length).toBeGreaterThan(20);
	});

	for (const s of table.scenarios) {
		it(s.name, () => {
			const got = classifyError(facts(s.input));
			expect(got.action).toBe(s.expected.action);
			if ("message" in s.expected) {
				expect(got.message ?? null).toBe(s.expected.message);
			}
			if ("retryDelayMs" in s.expected) {
				expect(got.retryDelayMs ?? null).toBe(s.expected.retryDelayMs);
			}
			if ("reason" in s.expected) {
				expect(
					got.reason === undefined ? undefined : ErrorReason[got.reason],
				).toBe(s.expected.reason);
			}
			if ("fields" in s.expected) {
				expect(got.fields).toEqual(s.expected.fields);
			}
		});
	}
});

describe("轉接層", () => {
	it("從 details 取出 reason、文字、欄位與 RetryInfo", () => {
		const err = new ConnectError(
			"dev message",
			Code.InvalidArgument,
			undefined,
			[
				{
					desc: ErrorInfoSchema,
					value: { reason: "VERSION_CONFLICT", domain: "hawkr" },
				},
				{
					desc: LocalizedMessageSchema,
					value: { locale: "zh-TW", message: "已被修改" },
				},
				{
					desc: BadRequestSchema,
					value: {
						fieldViolations: [
							create(BadRequest_FieldViolationSchema, {
								field: "banner.title",
								description: "標題 必填",
								localizedMessage: create(LocalizedMessageSchema, {
									message: "標題 必填",
								}),
							}),
						],
					},
				},
				{
					desc: RetryInfoSchema,
					value: {
						retryDelay: create(DurationSchema, {
							seconds: BigInt(2),
							nanos: 500_000_000,
						}),
					},
				},
			],
		);
		const f = factsFromError(err, ContentPublicService.method.listLiveBanners);
		expect(f).toMatchObject({
			code: Code.InvalidArgument,
			noResponse: false,
			idempotency: MethodOptions_IdempotencyLevel.NO_SIDE_EFFECTS,
			hasRequestId: false,
			reason: "VERSION_CONFLICT",
			reasonDomain: "hawkr",
			localizedMessage: "已被修改",
			fieldErrors: [{ field: "banner.title", message: "標題 必填" }],
			retryDelayMs: 2500,
		});
		expect(userMessage(err, defaults)).toBe("已被修改");
	});

	it("fetch 的 TypeError 是「沒收到回應」，伺服器的錯誤不是", () => {
		expect(
			factsFromError(ConnectError.from(new TypeError("fetch failed")))
				.noResponse,
		).toBe(true);
		expect(
			factsFromError(new ConnectError("down", Code.Unavailable)).noResponse,
		).toBe(false);
		expect(
			factsFromError(
				ConnectError.from(new SyntaxError("bad json"), Code.Internal),
			).noResponse,
		).toBe(false);
	});

	it("isNoResponse：包過與沒包過的 fetch TypeError 是，伺服器的錯誤與其他失敗不是", () => {
		expect(isNoResponse(new TypeError("Failed to fetch"))).toBe(true);
		expect(isNoResponse(ConnectError.from(new TypeError("fetch failed")))).toBe(
			true,
		);
		// 伺服器回的錯誤（含 Unavailable、Unknown）沒有 TypeError 的 cause。
		expect(isNoResponse(new ConnectError("down", Code.Unavailable))).toBe(
			false,
		);
		expect(isNoResponse(new ConnectError("x", Code.Unknown))).toBe(false);
		expect(
			isNoResponse(
				ConnectError.from(new SyntaxError("bad json"), Code.Internal),
			),
		).toBe(false);
		expect(isNoResponse(new Error("x"))).toBe(false);
		expect(isNoResponse("x")).toBe(false);
		expect(isNoResponse(undefined)).toBe(false);
	});

	it("LocalizedMessage 優先於預設文字（Aborted）", () => {
		const err = new ConnectError("dev", Code.Aborted, undefined, [
			{
				desc: LocalizedMessageSchema,
				value: { locale: "zh-TW", message: "已被修改" },
			},
		]);
		expect(userMessage(err, defaults)).toBe("已被修改");
	});

	it("沒有 LocalizedMessage 時用注入的預設文字，開發者訊息不外洩", () => {
		expect(
			userMessage(
				new ConnectError("dev message", Code.PermissionDenied),
				defaults,
			),
		).toBe("預設:PermissionDenied");
	});

	it("取消不顯示", () => {
		expect(
			userMessage(new ConnectError("x", Code.Canceled), defaults),
		).toBeUndefined();
	});

	it("errorText 一定有字：伺服器文字、該 code 的預設文字，取消當成 Internal", () => {
		const served = new ConnectError("dev", Code.Aborted, undefined, [
			{
				desc: LocalizedMessageSchema,
				value: { locale: "zh-TW", message: "已被修改" },
			},
		]);
		expect(errorText(served, defaults)).toBe("已被修改");
		expect(errorText(new ConnectError("dev", Code.NotFound), defaults)).toBe(
			"預設:NotFound",
		);
		expect(errorText(new ConnectError("x", Code.Canceled), defaults)).toBe(
			"預設:Internal",
		);
		expect(errorText(new Error("沒有 code 的錯誤"), defaults)).toBe(
			"預設:Unknown",
		);
	});

	it("欄位只有 description 沒有 LocalizedMessage 時，欄位文字是 undefined（不退回 description）", () => {
		const err = new ConnectError("x", Code.InvalidArgument, undefined, [
			{
				desc: BadRequestSchema,
				value: {
					fieldViolations: [{ field: "banner.title", description: "dev only" }],
				},
			},
		]);
		expect(factsFromError(err).fieldErrors).toEqual([
			{ field: "banner.title", message: undefined },
		]);
	});
});

describe("metadata", () => {
	const withInfo = (
		domain: string,
		reason: string,
		metadata: Record<string, string>,
	) =>
		new ConnectError("x", Code.FailedPrecondition, undefined, [
			{
				desc: ErrorInfoSchema,
				value: create(ErrorInfoSchema, { domain, reason, metadata }),
			},
		]);

	it("carries our ErrorInfo.metadata through facts to the reason decision", () => {
		const f = factsFromError(
			withInfo("hawkr", "PROSPECT_MATCH_FOUND", {
				prospectIds: "3,12,40",
			}),
		);
		expect(f.metadata).toEqual({ prospectIds: "3,12,40" });
		const d = classifyError(f);
		expect(d.action).toBe("reason");
		expect(d.reason).toBe(ErrorReason.PROSPECT_MATCH_FOUND);
		expect(metadataList(d.metadata?.prospectIds)).toEqual(["3", "12", "40"]);
	});

	it("has empty metadata for another domain, no ErrorInfo or a non-Connect error", () => {
		expect(
			factsFromError(withInfo("other", "PROSPECT_MATCH_FOUND", { a: "1" }))
				.metadata,
		).toEqual({});
		expect(
			factsFromError(new ConnectError("x", Code.Internal)).metadata,
		).toEqual({});
		expect(
			factsFromError(ConnectError.from(new Error("boom"))).metadata,
		).toEqual({});
	});

	it("splits list values and treats missing or empty as an empty list", () => {
		expect(metadataList("KYC_APPROVED,NOT_PENDING_DELETION")).toEqual([
			"KYC_APPROVED",
			"NOT_PENDING_DELETION",
		]);
		expect(metadataList("7")).toEqual(["7"]);
		expect(metadataList("")).toEqual([]);
		expect(metadataList(undefined)).toEqual([]);
	});
});
