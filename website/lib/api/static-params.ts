/**
 * Shared generateStaticParams for every /sellers/[id] route (storefront + posts +
 * reviews + messages).
 *
 * Cache Components 要求至少一筆：回一個格式合法但不存在的 sentinel id，build 不再
 * 預渲染任何真實賣家（後端對任一賣家的 5xx 不會炸 build），真實賣家全部 on-demand，
 * 由 `getCachedSeller` 的 `cacheLife` 窗與 sellerTag 失效刷新。sentinel 頁拿到 404
 * 就是找不到賣家的頁面，不會 throw。
 *
 * h1 唯一性由 tests/seo/h1-uniqueness.seo.spec.ts 以最終 DOM 驗證（各 route
 * 於 subtree 內只 emit 一個語意 <h1>）。
 */
export function sellerStaticParams(): { id: string }[] {
	return [{ id: SENTINEL_ENTITY_ID }];
}

/** 格式合法（純數字 TSID）但不存在的 id；`generateStaticParams` 的 sentinel，不是真實資料。 */
export const SENTINEL_ENTITY_ID = "1";
