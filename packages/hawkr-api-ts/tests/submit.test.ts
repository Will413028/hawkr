import { Code, ConnectError } from "@connectrpc/connect";
import { describe, expect, it } from "vitest";
import type { DefaultErrorText } from "../src/connect/errors";
import { fieldMessage, submitError } from "../src/connect/submit";
import {
	BadRequestSchema,
	ErrorInfoSchema,
	LocalizedMessageSchema,
} from "../src/gen/google/rpc/error_details_pb";
import { ContentAdminService } from "../src/gen/hawkr/admin/content/v1alpha1/content_pb";

// 預設文字由呼叫端從目錄注入；替身讓「用了預設文字」可以和任何伺服器文字區分。
const defaults: DefaultErrorText = (code) => `預設:${Code[code]}`;

const update = ContentAdminService.method.updateBanner;

describe("submitError", () => {
	it("VERSION_CONFLICT 是版本衝突", () => {
		const err = new ConnectError("etag", Code.Aborted, undefined, [
			{
				desc: ErrorInfoSchema,
				value: { reason: "VERSION_CONFLICT", domain: "hawkr" },
			},
		]);
		expect(submitError(err, defaults, update)).toEqual({
			kind: "conflict",
			metadata: {},
		});
	});

	it("版本衝突帶出 ErrorInfo.metadata（哪個物件的 etag 過時）", () => {
		const err = new ConnectError("etag", Code.Aborted, undefined, [
			{
				desc: ErrorInfoSchema,
				value: {
					reason: "VERSION_CONFLICT",
					domain: "hawkr",
					metadata: { conflictTarget: "case" },
				},
			},
		]);
		expect(submitError(err, defaults, update)).toEqual({
			kind: "conflict",
			metadata: { conflictTarget: "case" },
		});
	});

	it("BadRequest 轉成以 proto 路徑為 key 的欄位錯誤", () => {
		const err = new ConnectError("invalid", Code.InvalidArgument, undefined, [
			{
				desc: BadRequestSchema,
				value: {
					fieldViolations: [
						{
							field: "banner.title",
							description: "required",
							localizedMessage: { locale: "zh-TW", message: "請輸入標題" },
						},
					],
				},
			},
			{
				desc: LocalizedMessageSchema,
				value: { locale: "zh-TW", message: "資料有誤" },
			},
		]);
		expect(submitError(err, defaults, update)).toEqual({
			kind: "fields",
			fields: { "banner.title": "請輸入標題" },
			message: "資料有誤",
		});
	});

	it("其他錯誤顯示該 code 的預設文字", () => {
		expect(
			submitError(new ConnectError("x", Code.NotFound), defaults, update),
		).toEqual({ kind: "message", message: "預設:NotFound" });
	});

	it("取消也顯示 Internal 的預設文字", () => {
		expect(
			submitError(new ConnectError("x", Code.Canceled), defaults, update),
		).toEqual({ kind: "message", message: "預設:Internal" });
	});

	it("欄位只有 description 時，欄位文字是 InvalidArgument 的預設文字，不是 description", () => {
		const err = new ConnectError("invalid", Code.InvalidArgument, undefined, [
			{
				desc: BadRequestSchema,
				value: {
					fieldViolations: [{ field: "banner.title", description: "required" }],
				},
			},
		]);
		expect(submitError(err, defaults, update)).toEqual({
			kind: "fields",
			fields: { "banner.title": "預設:InvalidArgument" },
			message: undefined,
		});
	});
});

describe("fieldMessage", () => {
	it("比對完整路徑、子欄位與索引，不比對同前綴的其他欄位", () => {
		const fields = {
			"banner.action.url": "網址格式錯誤",
			"product.variants[2].name": "請輸入規格名稱",
		};
		expect(fieldMessage(fields, "banner.action")).toBe("網址格式錯誤");
		expect(fieldMessage(fields, "product.variants")).toBe("請輸入規格名稱");
		expect(fieldMessage({ "banner.actions_note": "x" }, "banner.action")).toBe(
			undefined,
		);
	});
});
