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
// Alias with no renderable page — render per request so redirect()'s
// request-scoped (Sentry) instrumentation is never pulled into a static build.
export const dynamic = "force-dynamic";

// A redirect route still needs a title. `redirect()` throws NEXT_REDIRECT, but
// the response has already begun streaming by then, so this route answers
// 200 with a complete document (head included) and hands the navigation to the
// client — measured, not assumed. Without this the seller sees the marketing
// site's title on a workspace URL for as long as the hop takes.
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
