import AppIcon from "@/components/shared/AppIcon";
import Avatar from "@/components/shared/Avatar";
import BrandLogo from "@/components/shared/BrandLogo";
import Link from "@/components/shared/Link";
import SellerCoverMedia from "@/features/seller/components/SellerCoverMedia";
import {
	getCachedSellerProducts,
	getCachedSellerReviewStats,
	getCachedSellerStats,
	getCachedSellerStorefrontProducts,
} from "@/lib/api/cached";
import { prerenderSafe } from "@/lib/api/prerender";
import { mapProduct, type ProductRecord } from "@/lib/market/listFilters";
import { resolveCdnUrl } from "@/lib/media/cdn";
import { sellerPath } from "@/lib/seo/urls";
import { resolveSellerCardProductState } from "./product-state";
import SellerCardProductRail from "./SellerCardProductRail";

export interface SellerCardSeller {
	city?: string;
	coverImage?: string;
	description?: string;
	district?: string;
	logo_url?: string;
	name?: string;
	prize?: string;
	rating?: number | string;
	storeCode?: string | number;
	storeId?: string | number;
	totalReviews?: number;
}

interface SellerCardPageProps {
	seller: SellerCardSeller;
	sellerId: string;
}

function sellerLocation(seller: SellerCardSeller): string {
	return [seller.city?.trim(), seller.district?.trim()]
		.filter((part): part is string => Boolean(part))
		.join(" · ");
}

export default async function SellerCardPage({
	seller,
	sellerId,
}: SellerCardPageProps) {
	const rawStoreId = String(seller.storeId ?? "").trim();
	const storeId = /^[1-9]\d*$/.test(rawStoreId) ? rawStoreId : "";
	const [reviewStats, publicStats, richProducts] = await Promise.all([
		storeId
			? getCachedSellerReviewStats(storeId).catch(prerenderSafe(null))
			: Promise.resolve(null),
		storeId
			? getCachedSellerStats(storeId).catch(prerenderSafe(null))
			: Promise.resolve(null),
		storeId
			? getCachedSellerStorefrontProducts(storeId, 1, 8).catch(
					prerenderSafe(null),
				)
			: Promise.resolve(null),
	]);

	const leanProducts =
		storeId && (!richProducts || richProducts.records.length === 0)
			? await getCachedSellerProducts(storeId, 1, 8).catch(() => null)
			: undefined;
	const productState = resolveSellerCardProductState<ProductRecord>(
		richProducts,
		leanProducts
			? { records: leanProducts.records as ProductRecord[] }
			: leanProducts,
	);

	const products = productState.records.slice(0, 8).map(mapProduct);
	const name = seller.name?.trim() || "商家";
	const description = seller.description?.trim() || "";
	const location = sellerLocation(seller);
	const prize = seller.prize?.trim() || "";
	const avatarUrl = seller.logo_url?.trim() || "";
	const coverUrl = seller.coverImage ? resolveCdnUrl(seller.coverImage) : "";
	const storefrontHref = sellerPath(seller.storeCode, sellerId);
	const avgRating = reviewStats
		? Number(reviewStats.avgRating) || 0
		: Number(seller.rating) || 0;
	const totalReviews = reviewStats
		? Number(reviewStats.totalCount) || 0
		: Number(seller.totalReviews) || 0;
	const followerCount = publicStats?.followers;
	const followers =
		typeof followerCount === "number" &&
		Number.isFinite(followerCount) &&
		followerCount >= 0
			? followerCount
			: null;
	return (
		<div className="seller-card-page" data-seller-card-ready>
			<article className="seller-public-card">
				<div className="seller-public-card__cover">
					{coverUrl ? (
						<SellerCoverMedia
							src={coverUrl}
							sizes="(max-width: 960px) calc(100vw - 32px), 920px"
							quality={70}
							loading="eager"
						/>
					) : (
						<div className="seller-public-card__cover-placeholder" />
					)}
					<BrandLogo
						variant="horizontal"
						size="sm"
						surface="pill"
						className="seller-public-card__brand"
					/>
				</div>

				<div className="seller-public-card__profile">
					<div className="seller-public-card__avatar">
						<Avatar
							src={avatarUrl}
							name={name}
							alt={name}
							entity="seller"
							fallback="empty"
							loading="eager"
						/>
					</div>

					<div className="seller-public-card__name-row">
						<h1>{name}</h1>
						{prize ? (
							<span className="seller-public-card__prize" title={prize}>
								<AppIcon name="trophy" size={14} />
								<span>{prize}</span>
							</span>
						) : null}
					</div>

					<div className="seller-public-card__meta">
						{avgRating > 0 ? (
							<span>
								<AppIcon name="star" size={15} fill />
								<strong>{avgRating.toFixed(1)}</strong>
								<small>{totalReviews} 則評價</small>
							</span>
						) : null}
						{location ? (
							<span>
								<AppIcon name="location" size={15} fill />
								{location}
							</span>
						) : null}
						{followers !== null ? (
							<span>
								<AppIcon name="people" size={16} />
								<strong>{followers}</strong>
								<small>位訂閱</small>
							</span>
						) : null}
					</div>

					{description ? (
						<p className="seller-public-card__description">{description}</p>
					) : null}

					<Link
						href={storefrontHref}
						data-hkui-button=""
						className="seller-public-card__storefront-cta"
					>
						<span>前往賣場</span>
						<AppIcon name="arrow-right" size={18} />
					</Link>
				</div>

				<SellerCardProductRail
					products={products}
					sellerName={name}
					status={productState.status}
				/>
			</article>
		</div>
	);
}
