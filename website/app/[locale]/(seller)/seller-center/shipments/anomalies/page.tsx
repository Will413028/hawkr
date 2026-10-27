import { redirect } from "@/lib/intl/navigation";
import { localeFromParam } from "@/lib/seo/metadata";

/**
 * 配送異常 was merged into the orders page — first its own 4th tab, then the
 * 異常處理 umbrella tab's 配送異常 sub-view, and now folded inline into the
 * 配送中 list (anomalous orders show a 「異常」 badge in the 訂單狀態 column).
 * Keep this route as a redirect so old bookmarks / banner links still land on
 * the right place.
 */
// Alias with no renderable page — render per request so redirect()'s
// request-scoped (Sentry) instrumentation is never pulled into a static build.
export const dynamic = "force-dynamic";

// Same as /seller-center: the redirect resolves on the client because the
// document has already started streaming, so this route really does serve a
// full head and needs a title of its own.
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
