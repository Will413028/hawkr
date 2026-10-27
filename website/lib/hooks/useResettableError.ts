"use client";

import * as React from "react";
import { useBFCacheReset } from "./useBFCacheReset";

/**
 * 取代 page 級 component 內常見的 `useState<string | null>(null)` 用來顯示
 * 「上一次非同步動作失敗訊息」的反模式。
 *
 * 為何需要這個 hook
 * ────────────────────────────────────────────────────────
 * Next.js App Router 的 Router Cache 與動態 segment 行為,讓「離開頁面再
 * 回來 component 一定 unmount → useState 自然回 null」這個直覺**不成立**:
 *
 *   · 同 segment 用 `router.push` 重新進入(例 /checkout → /cart → /checkout)
 *     React fiber tree 可能從 Router Cache 還原,useState 不會回初始值。
 *   · 動態 segment 切換不同 param(例 /orders/A → /orders/B)在 App Router
 *     是同個 page template,常常只 update params 而不 unmount。
 *   · 瀏覽器「上一頁 / 下一頁」走 BFCache,整個 React tree 連 state 一起
 *     被冰封還原。
 *
 * 上述三種 case 都會把上一次的失敗訊息帶到新的造訪/新的 session,造成
 * 「離開再回來訊息還在」的 bug(典型案例:切換不同訂單詳情看到上一筆
 * 的錯誤)。
 *
 * 兩道防線
 * ────────────────────────────────────────────────────────
 *  1. BFCache 還原(`pageshow.persisted=true`)時 reset。所有 caller 自動
 *     享有,cover 瀏覽器返回鍵 case。
 *  2. caller 傳入 `resetKey` 表達 session boundary。值變化時 hook 自動
 *     reset error 為 null。常見的 resetKey:
 *       · 動態 segment 內的 param(orderNo、postId、couponId…)
 *       · 「mode + id」組合(`${mode}:${id ?? ""}`)用於 create/edit 同 form
 *       · 父層用 router cache 不會變的 dialog → 傳當下操作的 entity id
 *
 * 不傳 resetKey 等於只依賴 mount + BFCache 兜底,適用於「該 component
 * 不會在不 unmount 的情況下重複進入新的 session」的場景(例:賣家中心
 * 單一設定頁,固定 pathname、無動態 param、無 router.push 重入路徑;
 * 提交開始時 caller 自己 `setError(null)` 已足夠覆蓋大多數殘留 case)。
 *
 * 若 page 級 component 的 transient state 不只一個 error(還有 submitting、
 * preview 結果等),`resetKey` 只能 reset error 一項;結構解是 component
 * 內用 useReducer 整合所有 transient state + `useEffect([sessionBoundary])`
 * 偵測 attempt 變化時一次 reset(範例見
 * `features/checkout/screens/CheckoutClient.tsx`)。
 *
 * API 與 `useState<string | null>(null)` 對齊,可直接代換(resetKey 可選)。
 */
export function useResettableError(
	resetKey?: unknown,
): [string | null, React.Dispatch<React.SetStateAction<string | null>>] {
	const [error, setError] = React.useState<string | null>(null);
	useBFCacheReset(() => setError(null));
	// biome-ignore lint/correctness/useExhaustiveDependencies: resetKey is the only intended trigger; setError is stable
	React.useEffect(() => {
		setError(null);
	}, [resetKey]);
	return [error, setError];
}
