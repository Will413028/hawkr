import NotFoundView from "@/components/shared/NotFoundView";

/**
 * App-wide 404 (below the root layout, so navbar/footer chrome stays up).
 *
 * A server component purely so it can own a title: without one this route falls
 * back to the root `title.default`, so every dead link — and every seller id the
 * middleware rewrites to `/_not-found` — served 「Hawkr — 在地職人的市集」,
 * the home page's exact title, in the tab and in the share card. The product
 * route's own miss branch has always answered 「找不到商品」; this is the same
 * rule for the site-wide miss. Markup lives in NotFoundView (client).
 */
export const metadata = { title: "找不到頁面" };

export default function NotFound() {
	return <NotFoundView />;
}
