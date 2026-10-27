import assert from "node:assert/strict";
import test from "node:test";
import { create, equals } from "@bufbuild/protobuf";
import {
	type BannerAction,
	BannerAction_PopupSchema,
	BannerActionSchema,
} from "@hawkr/api/gen/hawkr/content/v1alpha1/content_pb";
import {
	actionFromProto,
	actionToProto,
	BANNER_ACTION_TYPES,
} from "../src/components/banner/action";

/**
 * Banner「點擊動作」的契約（ContentAdminService 的 Banner.action，oneof）。
 *
 * 這一組測試釘的是一個**資料遺失**的失敗形態（2026-08-05 dev 實際發生）：編輯器載入一張「顯示彈窗大圖」的
 * banner、**什麼都不改**按存檔，送出的是空的 action —— 修改是整個覆寫，popup 就這樣被洗掉，而且沒有任何
 * 錯誤。使用者只有在點 banner 沒反應時才會發現。
 *
 * 因此核心斷言是 **「載入 → 原封不動存檔 → 送出的 action 與載入時相同」**（round-trip）加上
 * **「actionToProto 永遠帶 kind」**。任何一種讓存檔吃掉資料的寫法都會讓這兩組轉紅。
 */

/** 四種合法形狀。popup 不帶附加值 —— 型別本身就是完整語意。 */
const SHAPES: Record<string, BannerAction> = {
	popup: create(BannerActionSchema, {
		kind: { case: "popup", value: create(BannerAction_PopupSchema) },
	}),
	url: create(BannerActionSchema, {
		kind: { case: "url", value: "https://example.com/promotion" },
	}),
	product: create(BannerActionSchema, {
		kind: { case: "productId", value: "7451098488051643148" },
	}),
	post: create(BannerActionSchema, {
		kind: { case: "postId", value: "7398765432109876543" },
	}),
};

test("round-trip：載入既有 action → 原封不動存檔 → 送出的與載入時相同", () => {
	for (const [name, action] of Object.entries(SHAPES)) {
		const out = actionToProto(actionFromProto(action));
		assert.ok(
			equals(BannerActionSchema, out, action),
			`${name}: 原封不動存檔後 action 應與載入時相同`,
		);
	}
});

test("round-trip 涵蓋所有合法 type（新增 type 忘了補分支會轉紅）", () => {
	assert.deepEqual(Object.keys(SHAPES).sort(), [...BANNER_ACTION_TYPES].sort());
});

test("actionToProto 永遠帶 kind", () => {
	for (const action of Object.values(SHAPES)) {
		assert.ok(actionToProto(actionFromProto(action)).kind.case);
	}
});

test("值留空時照樣帶 kind（空字串），由伺服器以欄位錯誤擋下，不會變成空的 action", () => {
	const cases = [
		["url", "url"],
		["product", "productId"],
		["post", "postId"],
	] as const;
	for (const [actionType, kindCase] of cases) {
		const out = actionToProto({ actionType, actionUrl: "  ", actionId: " " });
		assert.deepEqual(out.kind, { case: kindCase, value: "" }, actionType);
	}
});

test("送出時去掉值前後的空白", () => {
	const out = actionToProto({
		actionType: "url",
		actionUrl: "  https://example.com/x  ",
		actionId: "",
	});
	assert.deepEqual(out.kind, { case: "url", value: "https://example.com/x" });
});

test("讀回：沒有 action（或 oneof 沒有值）退回預設 popup，存檔會正規化成合法的 popup", () => {
	const expected = { actionType: "popup", actionUrl: "", actionId: "" };
	assert.deepEqual(actionFromProto(undefined), expected);
	assert.deepEqual(actionFromProto(create(BannerActionSchema)), expected);
	assert.equal(actionToProto(actionFromProto(undefined)).kind.case, "popup");
});
