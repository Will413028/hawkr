import type { Metadata } from "next";
import { notFound } from "next/navigation";
import {
	getCachedSellerReviewStats,
	getCachedSellerReviews,
	getCachedStore,
} from "@/lib/api/cached";
import { prerenderSafe } from "@/lib/api/prerender";
import { storeRatingClause } from "@/lib/products/rating";
import { STORE_REVIEWS_PAGE_SIZE } from "@/lib/reviews/repo";
import { sellerNotFoundTitle } from "@/lib/seller/not-found-title";
import { resolveStorefront } from "@/lib/seller/storefront-seller";
import {
	localeFromParam,
	sellerSocialImage,
	siteNameFor,
	socialImages,
} from "@/lib/seo/metadata";
import { sellerCanonicalUrl } from "@/lib/seo/urls";
import ReviewStatsCard, { type SellerReviewStats } from "./ReviewStatsCard";
import ReviewsClient from "./ReviewsClient";
import { sellerReviewItem } from "./reviewItem";

interface Props {
	params: Promise<{ locale: string; id: string }>;
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
	const { id, locale } = await params;
	const s = await getCachedStore(id);
	if (!s) return { title: sellerNotFoundTitle(locale) };
	const socialImage = sellerSocialImage(s);
	const pageLocale = localeFromParam(locale);
	const canonical = sellerCanonicalUrl(s, id, pageLocale, "/reviews");
	return {
		title: `${s.name} 的買家評價`,
		description: `查看 ${s.name} 的買家評價${storeRatingClause(s)}`,
		alternates: { canonical },
		openGraph: {
			title: `${s.name} 的買家評價 | ${siteNameFor(pageLocale)}`,
			description: `查看 ${s.name} 的買家評價${storeRatingClause(s)}`,
			url: canonical,
			images: socialImages(socialImage),
		},
	};
}

export default async function ReviewsPage({ params }: Props) {
	const { id: routeId } = await params;
	const resolved = await resolveStorefront(routeId);
	if (!resolved) notFound();
	const id = resolved.storeId;

	const [reviewsData, stats] = await Promise.all([
		getCachedSellerReviews(id).catch(
			prerenderSafe({ reviews: [], nextPageToken: "" }),
		),
		(getCachedSellerReviewStats(id) as Promise<SellerReviewStats | null>).catch(
			prerenderSafe(null),
		),
	]);

	const reviews = reviewsData.reviews.map(sellerReviewItem);
	const totalPages = Math.ceil(
		(stats?.totalCount ?? 0) / STORE_REVIEWS_PAGE_SIZE,
	);
	const statsCard = <ReviewStatsCard stats={stats} />;

	return (
		<ReviewsClient
			sellerId={id}
			statsCard={statsCard}
			initialReviews={reviews}
			initialTotalPages={totalPages}
		/>
	);
}
