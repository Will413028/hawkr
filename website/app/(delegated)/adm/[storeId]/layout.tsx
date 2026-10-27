import { notFound } from "next/navigation";

/**
 * 代行工作區的 storeId 守門，也是這整棵子樹**唯一的 server 端驗證點**。
 *
 * 修復前 `app/(delegated)/adm/[storeId]/` 之下沒有任何 layout，storeId 的唯一
 * 判準在 client 的 `DelegatedRuntimeBoundary`（:28 `/^\d{1,19}$/`）。它對不合格
 * 的 segment 走 `return <>{children}</>` —— **不掛 Provider 也不掛 Frame**，
 * 於是各頁的嚴格 `useDelegatedSession()` 直接 throw，SSR 500。
 *
 * ⚠️ 500 不是「安全地擋下來」：它把一個**輸入格式錯誤**（該回 404）變成
 * 伺服器錯誤，對外表現成站台故障，對內吃掉一次 error budget，而真正該說的
 * 「這個店鋪不存在」一個字都沒說。
 *
 * 判準與 `next.config.ts` 的 `/adm/(\d+)` redirect、`DelegatedRuntimeBoundary`
 * 的 regex 同源（三處都是「純數字、1–19 位」＝ TSID 的十進位上界）。**三份規則
 * 已經是兩份太多**，但收斂成一份需要 client 與 server 共用模組，那是另一次改動；
 * 這裡先把行為對齊，並在此標明它們必須一起改。
 *
 * Next 16 的 `params` 是 Promise，必須 await —— 寫成 `params.storeId` 不會編譯錯，
 * 只會在執行期拿到 undefined 而讓每一頁都 404。
 */
export default async function DelegatedStoreLayout({
	children,
	params,
}: {
	children: React.ReactNode;
	params: Promise<{ storeId: string }>;
}) {
	const { storeId } = await params;
	if (!/^\d{1,19}$/.test(storeId)) notFound();
	return <>{children}</>;
}
