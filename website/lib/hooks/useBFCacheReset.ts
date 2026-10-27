"use client";

import * as React from "react";

/**
 * 在 BFCache（Back/Forward Cache）還原時觸發 callback，用來清掉「上一次造訪
 * 殘留」的 component state。
 *
 * 為什麼需要這個 hook
 * ────────────────────────────────────────────────────────
 *  · 瀏覽器「上一頁 / 下一頁」會走 BFCache：整個 React tree 連 state
 *    一起被冰封還原，`useState` 不會 reset、`useEffect` 也不會重跑。
 *    殘留的「上次失敗訊息 / submitting 狀態」會跟著回來。
 *  · `pageshow` 事件帶 `persisted=true` 是公認用來偵測 BFCache 還原的
 *    瀏覽器 API（MDN / web.dev 均建議此 pattern）。
 *
 * 這個 hook 只解一半問題
 * ────────────────────────────────────────────────────────
 *  本 hook 只 cover BFCache。「前進方向」的 navigation cache（Next.js
 *  App Router 的 Router Cache、同 segment soft navigation、動態 segment
 *  param 變化）同樣會讓 page component 不 unmount、useState 不歸零，但這
 *  些 case **不會**觸發 `pageshow.persisted=true`，本 hook 抓不到。
 *
 *  對應策略:
 *   · 單一 error state → 用 `useResettableError(resetKey)` 配合 session
 *     boundary 識別。
 *   · 多個 transient state（submitting / preview / error 同時存在）→
 *     用外層 wrapper + `key` prop 強制 unmount/remount 整個 component
 *     instance（參考 `features/checkout/screens/CheckoutSession.tsx`）。
 *   · 同 layout 內切 section（navigation 不 unmount layout）→ 用
 *     `useEffect([pathname])` 顯式 reset（參考
 *     `features/seller-center/components/SellerCenterShell.tsx`）。
 *
 * 設計重點
 *  · 只在 `persisted=true` 時觸發；正常首次 pageshow 不會誤觸發。
 *  · 用 ref 包 callback，listener 一旦掛上後不會因為 callback identity
 *    改變而 re-bind，也不需要 caller 自己 useCallback。
 */
export function useBFCacheReset(onRestore: () => void): void {
	const cbRef = React.useRef(onRestore);
	cbRef.current = onRestore;

	React.useEffect(() => {
		if (typeof window === "undefined") return;
		const onPageShow = (e: PageTransitionEvent) => {
			if (e.persisted) cbRef.current();
		};
		window.addEventListener("pageshow", onPageShow);
		return () => window.removeEventListener("pageshow", onPageShow);
	}, []);
}
