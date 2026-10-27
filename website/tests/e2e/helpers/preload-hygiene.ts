/**
 * Chrome 的 "preloaded using link preload but not used" 警告把兩件性質完全
 * 不同的事講成同一句話：
 *
 *   (1) 這一頁自己宣告了一個資源提示，然後沒有用到 —— `as` 值寫錯、預載了根本
 *       不 render 的圖或 script。這是真缺陷，site-stability 要擋的就是它。
 *
 *   (2) 這一頁刻意為「下一條路由」暖身。App Router 的 `router.prefetch()` 會連
 *       目標路由的 stylesheet 一起預載（首頁商家 rail 就是這樣做的，見
 *       features/home/components/HomeCompactMerchants.tsx 那個 IntersectionObserver）。
 *       訪客幾秒內沒點進去，Chrome 就算它 unused —— 但那支 CSS 本來就不是給
 *       這一頁用的。
 *
 * 只看 console 文字分不出這兩者，而分不出來的代價已經發生過一次：首屏優化把
 * 首頁 CSS 從 10 支瘦到 5 支（storefront 的樣式不再順便打進首頁），暖身預載
 * 因此第一次真的多載了一支，這條斷言就紅了 —— 程式沒有變差、反而變好，紅綠卻
 * 取決於 bundler 把哪些 CSS 分在同一包。那不是這條 gate 該量的東西，而它擋掉了
 * 一次 production 發版。
 *
 * 所以改用 DOM 事實分類，不靠文字。一則 unused-preload 警告要同時滿足三個條件
 * 才算暖身：
 *
 *   1. 它是 `as="style"` 的提示 —— script / font / image 預載不到就是寫錯了，
 *      那些一律照舊算缺陷；
 *   2. 這份文件從來沒把它掛成 `rel="stylesheet"` —— 有掛就代表是本頁的 CSS，
 *      預載到卻沒用上是真問題；
 *   3. 它不在文件解析完成當下就存在的提示裡 —— 手寫進 layout 的
 *      `<link rel="preload" as="style">` 沒被用到仍算缺陷，只有 router 事後
 *      插入的才是暖身。
 *
 * 任何一條不成立就是缺陷，照舊讓測試紅。
 */

const UNUSED_PRELOAD_PATTERN =
	/was preloaded using link preload but not used|preloaded.+not used within/iu;

/** 同一個 route 量到的三組 DOM 事實，全部是絕對 URL（`HTMLLinkElement.href`）。 */
export interface DocumentResourceHints {
	/** 文件解析完成（DOMContentLoaded）當下就存在的 `link[rel=preload]`。 */
	authoredPreloads: readonly string[];
	/** 量測窗結束時所有的 `link[rel=preload][as=style]`。 */
	styleHints: readonly string[];
	/** 這份文件實際掛上的 `link[rel=stylesheet]`。 */
	appliedStylesheets: readonly string[];
}

export interface PartitionedStabilityWarnings {
	/** 要讓測試紅的警告。 */
	defects: string[];
	/** 為下一條路由暖身而生、刻意容忍的警告。 */
	routeWarmups: string[];
}

/** 從 Chrome 的警告文字取出被點名的資源 URL；取不到回 `null`。 */
export function warnedResourceUrl(message: string): string | null {
	return message.match(/The resource (\S+) was preloaded/iu)?.[1] ?? null;
}

export function partitionStabilityWarnings(
	warnings: readonly string[],
	hints: DocumentResourceHints,
): PartitionedStabilityWarnings {
	const styleHints = new Set(hints.styleHints);
	const appliedStylesheets = new Set(hints.appliedStylesheets);
	const authoredPreloads = new Set(hints.authoredPreloads);
	const defects: string[] = [];
	const routeWarmups: string[] = [];

	for (const warning of warnings) {
		const url = UNUSED_PRELOAD_PATTERN.test(warning)
			? warnedResourceUrl(warning)
			: null;
		const isRouteWarmup =
			url !== null &&
			styleHints.has(url) &&
			!appliedStylesheets.has(url) &&
			!authoredPreloads.has(url);
		(isRouteWarmup ? routeWarmups : defects).push(warning);
	}

	return { defects, routeWarmups };
}
