import assert from "node:assert/strict";
import { test } from "node:test";
import { isCrossOriginWrite } from "../src/lib/cross-origin";

const OWN = "https://admin.example.test";

/**
 * 後台的 `/api/*` 只接受同源的寫入。後端看不到瀏覽器的來源（BFF 以伺服器身分呼叫它），而同一個網站的其他子網域
 * 可能帶上同源 cookie，所以這一道由後台擋下。middleware 的整合在
 * admin-middleware.test.ts。
 */

test("安全方法一律放行", () => {
	for (const method of ["GET", "HEAD", "OPTIONS"]) {
		const h = new Headers({ "sec-fetch-site": "cross-site" });
		assert.equal(isCrossOriginWrite(method, h, OWN), false, method);
	}
});

test("有 Sec-Fetch-Site 時只看它：same-origin 與 none 放行，same-site 與 cross-site 擋下", () => {
	const cases: [string, boolean][] = [
		["same-origin", false],
		["none", false],
		// 同一個網站的其他子網域（例如前台）：cookie 會跟著送，正是要擋的情況。
		["same-site", true],
		["cross-site", true],
	];
	for (const [site, blocked] of cases) {
		const h = new Headers({ "sec-fetch-site": site, origin: OWN });
		assert.equal(isCrossOriginWrite("POST", h, OWN), blocked, site);
	}
});

test("沒有 Sec-Fetch-Site 時比對 Origin", () => {
	assert.equal(
		isCrossOriginWrite("POST", new Headers({ origin: OWN }), OWN),
		false,
	);
	assert.equal(
		isCrossOriginWrite(
			"DELETE",
			new Headers({ origin: "https://example.test" }),
			OWN,
		),
		true,
	);
});

test("兩者都沒有的不是瀏覽器發的，放行", () => {
	assert.equal(isCrossOriginWrite("POST", new Headers(), OWN), false);
});
