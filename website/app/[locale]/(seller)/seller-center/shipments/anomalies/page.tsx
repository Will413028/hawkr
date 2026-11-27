import { redirect } from "@/lib/intl/navigation";
import { localeFromParam } from "@/lib/seo/metadata";

/**
 * 配送異常 was merged into the orders page — first its own 4th tab, then the
 * 異常處理 umbrella tab's 配送異常 sub-view, and now folded inline into the
 * 配送中 list (anomalous orders show a 「異常」 badge in the 訂單狀態 column).
 * Keep this route as a redirect so old bookmarks / banner links still land on
 * the right place.
 */

export const metadata = { title: "出貨異常 — 賣家中心" };

export default async function Page({
	params,
}: {
	params: Promise<{ locale: string }>;
}) {
	redirect({
		href: "/seller-center/orders?tab=in_transit",
		locale: localeFromParam((await params).locale),
	});
}
