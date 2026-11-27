import { redirect } from "@/lib/intl/navigation";
import { localeFromParam } from "@/lib/seo/metadata";

/**
 * The workspace has no landing page of its own — every entry point links to a
 * specific section, and 總覽 is the dashboard. Without this route the bare
 * `/seller-center` answered the app's 404, which is a dead end for the one
 * visitor most likely to type it: a signed-in seller. Worse, that 404's only
 * CTAs are 逛逛市集 / 回到首頁, and a seller who takes either gets pulled back
 * into the workspace by SellerWorkspaceLock — a loop with no exit (#377).
 *
 * Redirect rather than render the dashboard here, so the workspace keeps one
 * canonical URL per section instead of two that render the same thing.
 */

export const metadata = { title: "賣家中心" };

export default async function Page({
	params,
}: {
	params: Promise<{ locale: string }>;
}) {
	redirect({
		href: "/seller-center/dashboard",
		locale: localeFromParam((await params).locale),
	});
}
