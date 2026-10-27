import assert from "node:assert/strict";
import test from "node:test";
import { Code, ConnectError } from "@connectrpc/connect";
import { defaultErrorTextZhTW } from "@hawkr/ui-text/zh-TW";
import { actionErrorText, runAction } from "../src/lib/connect-action";
import { connectErrorText } from "../src/lib/error-text";

/**
 * 網路層失敗（沒有收到回應）：判斷在共用套件的 `isNoResponse`（`packages/hawkr-api-ts/tests/errors.test.ts` 測）。瀏覽器裡到畫面的是
 * connect-es 包過的那一種（ConnectError，cause 是 TypeError），這裡測畫面的文字選擇用它。
 */

const wrapped = () => ConnectError.from(new TypeError("Failed to fetch"));

test("connectErrorText：網路層失敗用呼叫端的句子（不是「系統錯誤」），Connect 錯誤照預設文字", () => {
	assert.equal(connectErrorText(wrapped(), "通報讀取失敗"), "通報讀取失敗");
	assert.equal(
		connectErrorText(new TypeError("x"), "通報讀取失敗"),
		"通報讀取失敗",
	);
	assert.equal(
		connectErrorText(
			new ConnectError("down", Code.Unavailable),
			"通報讀取失敗",
		),
		defaultErrorTextZhTW(Code.Unavailable),
	);
});

test("actionErrorText 與 runAction：網路層失敗回呼叫端的句子，不當成版本衝突、不重讀", async () => {
	assert.equal(actionErrorText(wrapped(), "分案失敗", "衝突"), "分案失敗");
	let reloads = 0;
	const result = await runAction(() => Promise.reject(wrapped()), {
		reload: async () => {
			reloads++;
		},
		fallback: "分案失敗，通報沒有變動；請確認網路狀態後再試一次。",
		conflictText: "衝突",
	});
	assert.deepEqual(result, {
		ok: false,
		kind: "message",
		text: "分案失敗，通報沒有變動；請確認網路狀態後再試一次。",
		noResponse: true,
	});
	assert.equal(reloads, 0);
});
