import assert from "node:assert/strict";
import test from "node:test";
import { Code, ConnectError } from "@connectrpc/connect";
import { fieldMessage, submitError } from "@hawkr/api/connect/submit";
import {
	BadRequestSchema,
	ErrorInfoSchema,
	LocalizedMessageSchema,
} from "@hawkr/api/gen/google/rpc/error_details_pb";
import { ContentAdminService } from "@hawkr/api/gen/hawkr/admin/content/v1alpha1/content_pb";
import { defaultErrorTextZhTW } from "@hawkr/ui-text/zh-TW";

const update = ContentAdminService.method.updateBanner;

test("VERSION_CONFLICT 是版本衝突：保留輸入、顯示最新版本", () => {
	const err = new ConnectError("etag mismatch", Code.Aborted, undefined, [
		{
			desc: ErrorInfoSchema,
			value: { reason: "VERSION_CONFLICT", domain: "hawkr" },
		},
	]);
	assert.deepEqual(submitError(err, defaultErrorTextZhTW, update), {
		kind: "conflict",
		metadata: {},
	});
});

test("Create 的 ALREADY_EXISTS 帶 VERSION_CONFLICT 時同樣是版本衝突", () => {
	const err = new ConnectError("exists", Code.AlreadyExists, undefined, [
		{
			desc: ErrorInfoSchema,
			value: { reason: "VERSION_CONFLICT", domain: "hawkr" },
		},
	]);
	assert.deepEqual(
		submitError(
			err,
			defaultErrorTextZhTW,
			ContentAdminService.method.createAppVersionGate,
		),
		{ kind: "conflict", metadata: {} },
	);
});

test("別的 domain 的 VERSION_CONFLICT 不是我們的 reason", () => {
	const err = new ConnectError("x", Code.Aborted, undefined, [
		{
			desc: ErrorInfoSchema,
			value: { reason: "VERSION_CONFLICT", domain: "example.com" },
		},
	]);
	assert.deepEqual(submitError(err, defaultErrorTextZhTW, update), {
		kind: "message",
		message: "資源衝突",
	});
});

test("BadRequest 轉成以 proto 路徑為 key 的欄位錯誤", () => {
	const err = new ConnectError("invalid", Code.InvalidArgument, undefined, [
		{
			desc: BadRequestSchema,
			value: {
				fieldViolations: [
					{
						field: "banner.title",
						description: "title is required",
						localizedMessage: { locale: "zh-TW", message: "請輸入標題" },
					},
					{ field: "banner.end_time", description: "before start" },
				],
			},
		},
		{
			desc: LocalizedMessageSchema,
			value: { locale: "zh-TW", message: "資料有誤" },
		},
	]);
	assert.deepEqual(submitError(err, defaultErrorTextZhTW, update), {
		kind: "fields",
		fields: {
			"banner.title": "請輸入標題",
			// 沒有 LocalizedMessage 時用 InvalidArgument 的預設文字，不退回開發者看的 description。
			"banner.end_time": "參數錯誤",
		},
		message: "資料有誤",
	});
});

test("其他錯誤顯示 LocalizedMessage，沒有時顯示該 code 的預設文字", () => {
	const localized = new ConnectError("x", Code.PermissionDenied, undefined, [
		{
			desc: LocalizedMessageSchema,
			value: { locale: "zh-TW", message: "只有管理員可以修改" },
		},
	]);
	assert.deepEqual(submitError(localized, defaultErrorTextZhTW, update), {
		kind: "message",
		message: "只有管理員可以修改",
	});
	assert.deepEqual(
		submitError(
			new ConnectError("x", Code.NotFound),
			defaultErrorTextZhTW,
			update,
		),
		{
			kind: "message",
			message: "資源不存在",
		},
	);
});

test("fieldMessage 比對完整路徑、子欄位與索引，不比對同前綴的其他欄位", () => {
	const fields = {
		"banner.action.url": "網址格式錯誤",
		"product.variants[2].name": "請輸入規格名稱",
		"banner.actions_note": "不相干的欄位",
	};
	assert.equal(fieldMessage(fields, "banner.action"), "網址格式錯誤");
	assert.equal(fieldMessage(fields, "product.variants"), "請輸入規格名稱");
	assert.equal(
		fieldMessage({ "banner.actions_note": "x" }, "banner.action"),
		undefined,
	);
	// 多個路徑依序找第一個有錯誤的。
	assert.equal(
		fieldMessage(fields, "banner.title", "banner.action"),
		"網址格式錯誤",
	);
	assert.equal(fieldMessage(fields, "banner.title"), undefined);
});
