/**
 * Shared generateStaticParams for every /sellers/[id] route (storefront + posts +
 * reviews + messages).
 *
 * 傳統 ISR 模式（cacheComponents off）：不在 build 期列舉真實賣家（後端對任一
 * 賣家的 5xx 不會炸 build），全部 on-demand + ISR（dynamicParams 預設 true）。
 * 空陣列 = 零預建、首訪生成、依 revalidate 窗與 sellerTag 失效刷新。
 *
 * h1 唯一性由 tests/seo/h1-uniqueness.seo.spec.ts 以最終 DOM 驗證（各 route
 * 於 subtree 內只 emit 一個語意 <h1>）。
 */
export function sellerStaticParams(): { id: string }[] {
	return [];
}
