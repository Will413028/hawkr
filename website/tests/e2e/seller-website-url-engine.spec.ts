import { expect, test } from "@playwright/test";
import {
	isValidHostname,
	isValidWebsiteUrl,
} from "../../lib/seller/website-url";

/**
 * 這支守的不是畫面，是**引擎**。
 *
 * `new URL()` 對非法主機名的處置逐引擎相反：Chrome 把它 percent-encode 後照收
 * （`https://not a url` → hostname `not%20a%20url`），Node 直接拋 `Invalid URL`。
 * 於是任何「靠 new URL 拋不拋」寫成的前端驗證，在 `tests/*.test.ts`（純 Node）
 * 裡恆綠、在使用者的瀏覽器裡完全不存在——這正是 `isValidWebsiteUrl` 曾經的形狀：
 * 賣家在官方網站欄位填 `not a url` 一路放行到後端才被 9260 擋下。
 *
 * 所以這一層的斷言只能在真的瀏覽器引擎裡下。做法是把**出貨的那份實作原始碼**
 * 送進頁面執行，而不是在測試裡重寫一次判準（重寫＝第二份實作，它會與正本各自
 * 演化）。兩個函式都自帶正則、不引用模組作用域的任何東西，所以 `toString()`
 * 就是一份可直接執行的完整來源。
 *
 * ⚠️ `tests/e2e/**` 只在 push dev/main 或進 main 的 PR 跑（10 檔 §17.5）。改動
 * `lib/seller/website-url.ts` 之後要主動 `PW_CHANNEL=chrome pnpm exec playwright
 * test tests/e2e/seller-website-url-engine.spec.ts`，不能等 merge。
 */

/** 這些在瀏覽器裡都「解析成功」，正是單元測試在 Node 裡看不到的那條路。 */
const REJECTED = [
	"https://not a url",
	"https://exa mple.com/x",
	"https://not%20a%20url",
	"https://not!a!url",
	"https://a*b",
];

/** 收緊主機名判準的反向錨：兩個引擎都解析得出，且必須維持合法。 */
const ACCEPTED = [
	"https://example.com",
	"http://example.com/shop",
	"https://例子.tw",
	"https://example.com.",
	"https://my_site.example.com",
	"http://[::1]",
	"https://example.com/my page",
];

const SHIPPED_SOURCE = [
	isValidHostname.toString(),
	isValidWebsiteUrl.toString(),
	"window.__sellerWebsiteUrl = { isValidWebsiteUrl };",
].join("\n");

declare global {
	interface Window {
		__sellerWebsiteUrl?: { isValidWebsiteUrl: (raw: string) => boolean };
	}
}

test.describe("官方網站驗證在真實瀏覽器引擎下成立", { tag: ["@ui"] }, () => {
	test("shipped isValidWebsiteUrl agrees with Node inside the browser", async ({
		page,
	}) => {
		// about:blank：這支與應用程式無關，也不該受站台 CSP 影響。
		await page.goto("about:blank");
		await page.addScriptTag({ content: SHIPPED_SOURCE });

		const result = await page.evaluate(
			({ rejected, accepted }) => {
				const shipped = window.__sellerWebsiteUrl;
				if (!shipped) throw new Error("shipped source failed to load");
				const parse = (raw: string) => {
					try {
						return new URL(raw).hostname;
					} catch {
						return "THROW";
					}
				};
				return {
					rejectedVerdicts: rejected.map((c) => shipped.isValidWebsiteUrl(c)),
					acceptedVerdicts: accepted.map((c) => shipped.isValidWebsiteUrl(c)),
					parsedHosts: rejected.map(parse),
				};
			},
			{ rejected: REJECTED, accepted: ACCEPTED },
		);

		// 引擎事實錨。這條轉紅不代表我們的實作壞了，而是瀏覽器改了行為——若哪天
		// 它也改成拋錯，主機名那一層就可以簡化掉，這是唯一會通知我們的地方。
		expect(result.parsedHosts).toEqual([
			"not%20a%20url",
			"exa%20mple.com",
			"not%20a%20url",
			"not!a!url",
			"a%2Ab",
		]);

		// 契約本身：瀏覽器裡的判定必須與 Node 單元測試給的答案一致。
		expect(REJECTED.filter((_, i) => result.rejectedVerdicts[i])).toEqual([]);
		expect(ACCEPTED.filter((_, i) => !result.acceptedVerdicts[i])).toEqual([]);
		for (const raw of REJECTED) expect(isValidWebsiteUrl(raw)).toBe(false);
		for (const raw of ACCEPTED) expect(isValidWebsiteUrl(raw)).toBe(true);
	});
});
