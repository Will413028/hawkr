import type { Metadata } from "next";
import {
	getCachedSeller,
	getCachedSellerReviewStats,
	getCachedSellerReviews,
} from "@/lib/api/cached";
import { prerenderSafe } from "@/lib/api/prerender";
import { formatRatingSummary } from "@/lib/products/rating";
import {
	localeFromParam,
	sellerSocialImage,
	siteNameFor,
	socialImages,
} from "@/lib/seo/metadata";
import { sellerCanonicalUrl } from "@/lib/seo/urls";
import { type SellerReviewItem, StarsSvg } from "./ReviewList";
import ReviewsClient from "./ReviewsClient";

interface Props {
	params: Promise<{ locale: string; id: string }>;
}

interface SellerReviewStats {
	avgRating?: number | string;
	rating1Count?: number | string;
	rating2Count?: number | string;
	rating3Count?: number | string;
	rating4Count?: number | string;
	rating5Count?: number | string;
	totalCount?: number | string;
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
	const { id, locale } = await params;
	const seller = await getCachedSeller(id);
	const s = seller;
	if (!s) return { title: "找不到賣家" };
	const socialImage = sellerSocialImage(s);
	// Self-canonical on the store_code, the same way the storefront index does it.
	// Middleware already 308s a store_id URL here, but the redirect and this tag
	// are two independent signals and this tab used to emit neither: the file set
	// no `alternates`, so the only canonicalization a crawler could see on a tab
	// URL was the redirect. One signal is a single point of failure.
	//
	// The tab suffix is carried on purpose: this consolidates the two ids for the
	// SAME tab. Pointing the tab at `/sellers/{code}` instead would additionally
	// ask search engines to drop this tab's URL from the index — a separate
	// decision, and not one #365 asked for.
	const pageLocale = localeFromParam(locale);
	const canonical = sellerCanonicalUrl(s, id, pageLocale, "/reviews");
	return {
		title: `${s.name} 的買家評價`,
		description: `查看 ${s.name} 的買家評價，${formatRatingSummary(s.rating)}`,
		alternates: { canonical },
		openGraph: {
			title: `${s.name} 的買家評價 | ${siteNameFor(pageLocale)}`,
			description: `查看 ${s.name} 的買家評價，${formatRatingSummary(s.rating)}`,
			url: canonical,
			images: socialImages(socialImage),
		},
	};
}

function computeRatingPercents(stats: SellerReviewStats): number[] {
	const total = Number(stats.totalCount) || 0;
	if (total === 0) return [0, 0, 0, 0, 0];
	const counts = [
		Number(stats.rating5Count) || 0,
		Number(stats.rating4Count) || 0,
		Number(stats.rating3Count) || 0,
		Number(stats.rating2Count) || 0,
		Number(stats.rating1Count) || 0,
	];
	return counts.map((c) => Math.round((c / total) * 100));
}

export default async function ReviewsPage({ params }: Props) {
	const { id } = await params;

	// The first page is the SEO/ISR body (rendered in server HTML); later pages
	// are fetched client-side by ReviewsClient off the `?page` URL param, so this
	// route reads no dynamic API and stays statically renderable.
	const [reviewsData, stats] = await Promise.all([
		getCachedSellerReviews(id, 1).catch(
			prerenderSafe({ records: [], total: 0, totalPages: 0 }),
		),
		(getCachedSellerReviewStats(id) as Promise<SellerReviewStats | null>).catch(
			prerenderSafe(null),
		),
	]);

	const reviews = reviewsData.records as SellerReviewItem[];
	const totalCount = stats ? Number(stats.totalCount) || 0 : 0;
	const avgRating = stats ? Number(stats.avgRating) || 0 : 0;
	const ratingPercents = stats ? computeRatingPercents(stats) : [0, 0, 0, 0, 0];
	const ratingBars = [5, 4, 3, 2, 1].map((stars, index) => ({
		stars,
		pct: ratingPercents[index] ?? 0,
	}));

	// Rating-summary card — static (does not paginate), so it stays server-rendered
	// and is handed to the client boundary as a node.
	const statsCard = (
		<div className="flex gap-[var(--space-10)] items-start mb-[var(--space-8)] p-[var(--space-8)] border border-[var(--hkui-divider)] rounded-[var(--hkui-radius-xl)] bg-[var(--hkui-content1)] bg-gradient-to-br from-[rgba(255,255,255,0.8)] to-[rgba(248,246,236,0.66)] dark:from-[rgb(var(--color-ink-900-rgb)/0.9)] dark:to-[rgb(var(--color-ink-950-rgb)/0.9)] dark:border-[rgba(242,165,65,0.24)] shadow-[0_14px_38px_rgb(var(--color-ink-850-rgb)/0.08)] dark:shadow-[0_18px_46px_rgba(0,0,0,0.24)] max-[760px]:flex-col max-[760px]:items-stretch max-[760px]:gap-[var(--space-5)] max-[760px]:p-[var(--space-5)]">
			<div className="flex flex-col items-center justify-center flex-[0_0_128px] min-w-[128px] p-[var(--space-4)] rounded-[var(--hkui-radius-lg)] bg-[rgba(255,255,255,0.76)] dark:bg-[rgb(var(--color-ink-950-rgb)/0.48)] max-[760px]:w-full max-[760px]:min-w-0 max-[760px]:flex-none">
				<span className="mb-[var(--space-2)] text-[color:var(--text-heading)] text-[length:var(--text-5xl)] font-bold leading-none">
					{avgRating > 0 ? avgRating.toFixed(1) : "0"}
				</span>
				<div className="flex gap-[2px] mb-[var(--space-1)] text-[var(--accent-warm)]">
					<StarsSvg count={Math.round(avgRating)} />
				</div>
				<span className="text-[var(--color-gray-mid)] text-[length:var(--text-xs)]">
					{totalCount} 則評價
				</span>
			</div>
			<div className="flex-1 flex flex-col gap-[var(--space-2)] pt-[var(--space-1)] max-[760px]:w-full max-[760px]:min-w-0 max-[760px]:flex-none">
				{ratingBars.map(({ stars, pct }) => (
					<div
						key={`bar-${stars}`}
						className="grid grid-cols-[42px_minmax(0,1fr)_44px] items-center gap-[var(--space-3)] max-[760px]:grid-cols-[38px_minmax(0,1fr)_38px] max-[760px]:gap-[var(--space-2)]"
					>
						<span className="text-[var(--color-gray-mid)] text-[length:var(--text-xs)] whitespace-nowrap text-right">
							{stars} 星
						</span>
						<div className="w-full h-3 min-w-[24px] overflow-hidden rounded-full bg-[rgba(150,122,56,0.18)] dark:bg-[rgba(245,222,165,0.22)]">
							<div
								className="h-full min-w-0 rounded-inherit bg-gradient-to-r from-[var(--color-brand-secondary)] to-[var(--color-brand-primary)] transition-[width] duration-[var(--transition-base)]"
								style={{ width: `${pct}%` }}
							/>
						</div>
						<span className="text-[var(--color-gray-mid)] text-[length:var(--text-xs)] whitespace-nowrap tabular-nums text-right">
							{pct}%
						</span>
					</div>
				))}
			</div>
		</div>
	);

	// Chrome (profile header / banner / shell) lives in the persistent
	// (storefront) layout — this page renders only the reviews column.
	return (
		<ReviewsClient
			sellerId={id}
			statsCard={statsCard}
			initialReviews={reviews}
			initialTotalPages={reviewsData.totalPages}
		/>
	);
}
