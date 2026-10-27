import assert from "node:assert/strict";
import test from "node:test";
import {
	isFormalProductRemoved,
	PRODUCT_PUBLICATION_STATE_REMOVED,
} from "../features/seller-center/products-import/itemState";
import type { ImportItemBrief } from "../features/seller-center/products-import/types";

// backend #2819：匯入紀錄裡「正式商品已移除」與「後端查不到狀態」是**兩件事**，
// 修復前兩者都表現為欄位為空，賣家無從分辨——而「前往正式商品」按鈕照樣顯示，
// 點下去是 404「商品已永久移除」。
//
// 這支守的是前端這一半：已移除時不得再給連結。

function brief(over: Partial<ImportItemBrief>): ImportItemBrief {
	return {
		importItemId: "1",
		productKey: "K1",
		status: "COMMITTED",
		name: "測試商品",
		skuCount: "1",
		blockingErrorCount: "0",
		warningCount: "0",
		needsFix: false,
		hasDuplicateExisting: false,
		commitRevision: "1",
		version: "0",
		formalProductId: "873101296545857928",
		...over,
	} as ImportItemBrief;
}

test("publicationState=4014 判定為已移除（連結必須收起來）", () => {
	const it = brief({
		formalProductPublicationState: PRODUCT_PUBLICATION_STATE_REMOVED,
	});
	assert.equal(isFormalProductRemoved(it), true);
});

test("在架商品（4012 PUBLIC）不是已移除", () => {
	assert.equal(
		isFormalProductRemoved(brief({ formalProductPublicationState: "4012" })),
		false,
	);
});

test("後端查不到（unavailable）不等於已移除 —— 商品可能還在，連結要留著", () => {
	const it = brief({
		formalProductStatusUnavailable: true,
		formalProductPublicationState: "",
	});
	assert.equal(
		isFormalProductRemoved(it),
		false,
		"「不知道」不得被當成「確定已移除」——那會把還在的商品連結拿掉",
	);
});

test("欄位缺席（舊後端尚未部署）不得誤判為已移除", () => {
	assert.equal(isFormalProductRemoved(brief({})), false);
});
