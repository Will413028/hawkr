// tests/seo/h1-uniqueness.seo.spec.ts
import { expect, test } from "@playwright/test";

import { SEO_API_BASE as API } from "./_helpers";

// 從 marketplace 取一個真實 product id + seller(account) id，驅動 on-demand 頁。
async function pickIds(): Promise<{
	productId: string;
	sellerId: string;
} | null> {
	const res = await fetch(
		`${API}/public/marketplace?recommended_sellers=5`,
	).catch(() => null);
	if (!res?.ok) return null;
	const json = await res.json().catch(() => null);
	if (json == null) return null;
	const text = JSON.stringify(json);
	const productId = text.match(/"id":"?(\d{6,})/)?.[1];
	const sellerId = text.match(/"storeId":"?(\d{6,})/)?.[1];
	return productId && sellerId ? { productId, sellerId } : null;
}

test.describe("h1 uniqueness (on-demand SEO entity pages)", () => {
	test("each entity page renders exactly one <h1> in the final DOM", async ({
		page,
	}) => {
		const ids = await pickIds();
		if (!ids) {
			console.log(`[skip] ${API} 不可達或沒有 product/seller id，略過 h1 檢查`);
			test.skip(true, `${API} unreachable or empty`);
			return;
		}
		const { productId, sellerId } = ids;

		const urls = [
			`/products/${productId}`,
			`/sellers/${sellerId}`,
			`/sellers/${sellerId}/posts`,
			`/sellers/${sellerId}/reviews`,
			`/sellers/${sellerId}/messages`,
		];

		for (const url of urls) {
			await page.goto(url, { waitUntil: "networkidle" });
			await page.waitForTimeout(1000); // 等 Suspense streaming swap 完成
			const count = await page.locator("h1").count();
			expect(count, `${url} 最終 DOM 應只有 1 個 <h1>`).toBe(1);
		}
	});
});
