import { notFound } from "next/navigation";
import { HAWKR_AGENT_ENABLED } from "@/features/procurement-assistant/availability";
import BuyerOfferDemo from "@/features/procurement-rfq-demo/BuyerOfferDemo";

export const metadata = {
	title: { absolute: "Hawkr Agent｜買家出價商務線 demo" },
	robots: { index: false, follow: false },
};

/**
 * 需要一個真實的 request_id：demo 的買家那半吃這個帳號自己的 snapshot，
 * 沒有 id 就沒有東西可以載。id 在採購助手對話的網址上。
 *
 * 路由登記在 `tests/e2e/public-static-routes.spec.ts` 的公開靜態清冊裡
 * （2026-09-07 `bbfe47ec`）。它需要買家 session 才有內容，匿名進來只會看到
 * 下面那段「缺少 request_id」，因此清冊對它發的匿名 hard-load `@live` 測試
 * 驗到的是那段 fallback——這是刻意接受的涵蓋範圍，不是遺漏。
 */
export default async function Page({
	searchParams,
}: {
	searchParams: Promise<{ request_id?: string }>;
}) {
	if (!HAWKR_AGENT_ENABLED) notFound();

	const { request_id: requestId } = await searchParams;
	if (!requestId) {
		return (
			<main style={{ maxWidth: 640, margin: "0 auto", padding: "48px 24px" }}>
				<h1>缺少 request_id</h1>
				<p>
					網址加上 <code>?request_id=&lt;你的採購需求 id&gt;</code> 再開一次。
				</p>
			</main>
		);
	}

	return <BuyerOfferDemo requestId={requestId} />;
}
