import AppIcon from "@/components/shared/AppIcon";
import {
	FOOTER_COLUMN,
	FOOTER_HEADING,
	FOOTER_HEADING_DOT,
	FOOTER_LINK_LIST,
	FOOTER_SUMMARY,
	FOOTER_SUMMARY_CHEVRON,
	FOOTER_TEXT_LINK,
} from "@/components/shared/footerChrome";
import Link from "@/components/shared/Link";
import {
	getCachedCategoryTree,
	getCachedTrendingPostTags,
} from "@/lib/api/cached";

/**
 * The two data-driven footer columns (商品分類 / 熱門話題).
 *
 * Both lists arrive with the server render. They are identical for every
 * visitor and change rarely, so they are read through the shared
 * `unstable_cache` layer: one tagged entry serves every page and every user,
 * and the browser makes no request at all.
 *
 * This used to be a client island that waited for an IntersectionObserver, so
 * the two public reads fired only once someone scrolled near the bottom. That
 * broke the site-stability contract ("started a content request only because
 * the user scrolled") on every long public route. Fetching on mount instead
 * would have satisfied the contract while paying two browser requests on every
 * page view; the cached server read pays none.
 *
 * RoleAwareFooter is a client component, so these columns are rendered in the
 * root layout and passed down as an element. That also means the markup is not
 * in the initial HTML — the footer itself is still client-gated, which is a
 * separate concern from where the data is read.
 *
 * Sources: top-level nodes of the anonymous category tree (the footer renders
 * on logged-out pages too, where `/app/categories/tree` would 401) and
 * `/public/tags/trending`.
 *
 * The hrefs are copied from `MegaMenuPanel`, not re-derived:
 *   category → `/market/products?category=<name>`
 *   hashtag  → `/feed?hashtag=<tag>`
 *
 * Failure shape: a failed or empty source never blanks a column — it falls back
 * to the one link that always works (全部商品 / 看動態), so the footer keeps a
 * stable set of landmarks and a gateway blip never fails a public page render.
 */

/** Max links per column; the tail of a long category tree is noise in a footer. */
export const FOOTER_COLUMN_ITEM_CAP = 8;

interface FooterColumnLink {
	label: string;
	href: string;
}

export function footerCategoryHref(name: string): string {
	return `/market/products?category=${encodeURIComponent(name)}`;
}

export function footerHashtagHref(tag: string): string {
	return `/feed?hashtag=${encodeURIComponent(tag)}`;
}

const CATEGORY_FALLBACK: FooterColumnLink = {
	label: "全部商品",
	href: "/market/products",
};

const HASHTAG_FALLBACK: FooterColumnLink = {
	label: "看動態",
	href: "/feed",
};

function FooterDataColumn({
	heading,
	links,
	fallback,
}: {
	heading: string;
	links: FooterColumnLink[];
	fallback: FooterColumnLink;
}) {
	const items = links.length > 0 ? links : [fallback];
	return (
		<details className={FOOTER_COLUMN} open>
			<summary className={FOOTER_SUMMARY}>
				<h2 className={FOOTER_HEADING}>
					<span className={FOOTER_HEADING_DOT} />
					{heading}
				</h2>
				<AppIcon
					name="chevron-down"
					size={18}
					className={FOOTER_SUMMARY_CHEVRON}
				/>
			</summary>
			<ul className={FOOTER_LINK_LIST}>
				{items.map((link) => (
					<li key={link.href}>
						<Link href={link.href} className={FOOTER_TEXT_LINK}>
							{link.label}
						</Link>
					</li>
				))}
			</ul>
		</details>
	);
}

export default async function FooterCategoryColumns() {
	// `.catch` per source, not one shared try: a failing taxonomy read must not
	// also blank the hashtag column.
	const [categoryTree, trendingTags] = await Promise.all([
		getCachedCategoryTree().catch(() => []),
		getCachedTrendingPostTags(FOOTER_COLUMN_ITEM_CAP).catch(() => []),
	]);

	const categoryLinks: FooterColumnLink[] = (
		Array.isArray(categoryTree) ? categoryTree : []
	)
		.flatMap((node) => {
			const name = String((node as { name?: unknown })?.name ?? "").trim();
			if (!name) return [];
			return [{ label: name, href: footerCategoryHref(name) }];
		})
		.slice(0, FOOTER_COLUMN_ITEM_CAP);

	const hashtagLinks: FooterColumnLink[] = (trendingTags ?? [])
		.flatMap((tag) => {
			if (!tag.name) return [];
			return [
				{
					label: tag.label || `#${tag.name}`,
					href: footerHashtagHref(tag.name),
				},
			];
		})
		.slice(0, FOOTER_COLUMN_ITEM_CAP);

	return (
		<>
			<FooterDataColumn
				heading="商品分類"
				links={categoryLinks}
				fallback={CATEGORY_FALLBACK}
			/>
			<FooterDataColumn
				heading="熱門話題"
				links={hashtagLinks}
				fallback={HASHTAG_FALLBACK}
			/>
		</>
	);
}
