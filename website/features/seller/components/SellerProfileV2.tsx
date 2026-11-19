import "@/styles/features/seller-profile.css";
import AppIcon from "@/components/shared/AppIcon";
import Avatar from "@/components/shared/Avatar";
import { BackLink } from "@/components/shared/BackLink";
import {
	COMPACT_BACK_CONTROL_CLASSNAME,
	CompactBackIcon,
} from "@/components/shared/CompactBackControl";
import SellerChatButton from "@/features/seller/components/SellerChatButton";
import SellerCoverMedia from "@/features/seller/components/SellerCoverMedia";
import SellerProfileDescription from "@/features/seller/components/SellerProfileDescription";
import SellerProfileTabs from "@/features/seller/components/SellerProfileTabs";
import SellerShareButton from "@/features/seller/components/SellerShareButton";
import SellerSubscribeButton from "@/features/seller/components/SellerSubscribeButton";
import SellerWebsiteLink from "@/features/seller/components/SellerWebsiteLink";
import { resolveSellerChatTargetId } from "@/features/seller/components/seller-chat-guard";
import {
	getCachedSellerReviewStats,
	getCachedSellerStats,
} from "@/lib/api/cached";
import { prerenderSafe } from "@/lib/api/prerender";
import { resolveCdnUrl } from "@/lib/media/cdn";
import { buildSellerShareCardData } from "@/lib/seller/share-card";
import { isExternalWebsiteUrl } from "@/lib/seller/website-url";

/**
 * `fetchSeller` 的回傳（lib/api/index.ts）。`coverImage` / `logo_url` /
 * `name` 是那一層組出來的**前端顯示契約**；其餘（`storeId` / `storeCode` /
 * `websiteUrl` …）直接來自 wire，backend#2235 之後全是 camelCase。
 */
interface SellerProfileLike {
	/** `GetStore` 的統計；讀不到的欄位不設，顯示「—」。賣家預覽沒有，改讀舊的統計端點。 */
	followerCount?: number;
	reviewCount?: number;
	ratingAverage?: number;
	city?: string;
	coverImage?: string;
	description?: string;
	district?: string;
	logo_url?: string;
	name?: string;
	prize?: string;
	rating?: number | string;
	storeId?: string;
	storeCode?: string;
	totalReviews?: number;
	websiteUrl?: string;
}

interface SellerProfileV2Props {
	seller: SellerProfileLike | null;
	/** 網址上的店家段（公開店面是 store_code）；追蹤與統計用 `seller.storeId`。 */
	sellerId: string;
	/** Seller-center preview uses the same buyer component but blocks writes. */
	previewMode?: boolean;
	/** Keeps storefront tabs inside the preview route instead of public routes. */
	tabBasePath?: string;
}

/** 統計讀不到時顯示的字。 */
const UNSET_STAT = "—";

const StarIcon = ({ size = 14 }: { size?: number }) => (
	<AppIcon name="star" size={size} fill />
);
const PinIcon = ({ size = 13 }: { size?: number }) => (
	<AppIcon name="location" size={size} fill />
);
const UsersIcon = ({ size = 15 }: { size?: number }) => (
	<AppIcon name="people" size={size} />
);
export function sellerProfileLocation(seller: {
	city?: string;
	district?: string;
}): string {
	return [seller.city?.trim(), seller.district?.trim()]
		.filter((part): part is string => Boolean(part))
		.join(" · ");
}

export function SellerProfileMeta({
	rating,
	totalReviews,
	location,
	subscribers,
	websiteUrl,
}: {
	/** null：評分讀不到，顯示「—」；空字串或 0：沒有評分，整段不顯示。 */
	rating: string | null;
	/** null：評價數讀不到，顯示「—」。 */
	totalReviews: number | null;
	location: string;
	/** null：粉絲數讀不到，顯示「—」。 */
	subscribers: number | null;
	websiteUrl: string;
}) {
	const ratingUnknown = rating === null;
	const hasRating = ratingUnknown || Number(rating) > 0;
	const hasLocation = Boolean(location);
	return (
		<div className="hkui-seller-profile__meta">
			{hasRating && (
				<span className="hkui-seller-profile__meta-item hkui-seller-profile__meta-item--rating">
					<StarIcon />
					<strong>{rating ?? UNSET_STAT}</strong>
					{ratingUnknown ? null : <span>({totalReviews ?? UNSET_STAT})</span>}
				</span>
			)}
			{hasLocation && (
				<>
					{hasRating && (
						<span
							className="hkui-seller-profile__meta-dot"
							aria-hidden="true"
						/>
					)}
					<span className="hkui-seller-profile__meta-item hkui-seller-profile__meta-item--location">
						<PinIcon />
						<span>{location}</span>
					</span>
				</>
			)}
			{(hasRating || hasLocation) && (
				<span className="hkui-seller-profile__meta-dot" aria-hidden="true" />
			)}
			<span className="hkui-seller-profile__meta-item hkui-seller-profile__meta-item--users">
				<UsersIcon />
				<strong>{subscribers ?? UNSET_STAT}</strong>
				<span>位訂閱</span>
			</span>
			{websiteUrl && (
				<>
					<span className="hkui-seller-profile__meta-dot" aria-hidden="true" />
					<SellerWebsiteLink href={websiteUrl} />
				</>
			)}
		</div>
	);
}

interface ProfileStats {
	rating: string | null;
	totalReviews: number | null;
	subscribers: number | null;
}

/**
 * 店面頁頂部的三項統計。公開店面取 `GetStore` 的 `StoreStats`：欄位沒有設定是讀不到（評分另有
 * 「沒有評價」的情形，由評價數 0 區分），一律顯示「—」，不當成 0。賣家預覽還沒有店主端點，沿用舊的
 * 兩支統計讀取，隨預覽改讀 `GetMyStore` 一起退場。
 */
async function profileStats(
	s: SellerProfileLike,
	storeId: string,
	previewMode: boolean,
): Promise<ProfileStats> {
	if (!previewMode) {
		const reviewCount = s.reviewCount;
		const rating =
			reviewCount === 0
				? ""
				: s.ratingAverage !== undefined
					? s.ratingAverage.toFixed(1)
					: null;
		return {
			rating,
			totalReviews: reviewCount ?? null,
			subscribers: s.followerCount ?? null,
		};
	}
	const [stats, publicStats] = await Promise.all([
		getCachedSellerReviewStats(storeId).catch(prerenderSafe(null)),
		getCachedSellerStats(storeId).catch(prerenderSafe(null)),
	]);
	const avgRating = stats ? Number(stats.avgRating) || 0 : 0;
	return {
		rating:
			avgRating > 0 ? avgRating.toFixed(1) : Number(s.rating || 0).toFixed(1),
		totalReviews: stats ? Number(stats.totalCount) || 0 : s.totalReviews || 0,
		subscribers: publicStats ? Number(publicStats.followers) || 0 : 0,
	};
}

export default async function SellerProfileV2({
	seller,
	sellerId,
	previewMode = false,
	tabBasePath,
}: SellerProfileV2Props) {
	const s = seller || {};
	const storeId =
		String(s.storeId ?? "").trim() || (previewMode ? sellerId : "");
	const { rating, totalReviews, subscribers } = await profileStats(
		s,
		storeId,
		previewMode,
	);
	const coverSrc = s.coverImage || "";
	const coverImageSrc = coverSrc ? resolveCdnUrl(coverSrc) : "";
	const avatarSrc = s.logo_url?.trim() || "";
	const name = s.name || "商家";
	const location = sellerProfileLocation(s);
	const description = s.description || "";
	const prize = String(s.prize || "").trim();
	// 明列欄位而不是把整個 loose 物件丟進去：這一頁的來源是 `fetchSeller`
	// （前端顯示契約 `logo_url`），seller-center 那張卡的來源是 `GetMyStore`
	// 的店家（圖片經 `imageUri` 組成 `logoUrl`）。兩邊形狀不同，讓對映在呼叫點看得見。
	const shareCard = buildSellerShareCardData(
		sellerId,
		{
			storeId: s.storeId,
			city: s.city,
			coverImage: s.coverImage,
			description: s.description,
			district: s.district,
			logoUrl: s.logo_url,
			prize: s.prize,
			storeName: s.name,
		},
		s.storeCode,
	);
	// `websiteUrl` only earns the 官網 affordance when it points off-platform —
	// the registration flow seeds it with the seller's own storefront URL, which
	// would make "訪問官網" a circular self-link.
	const websiteUrl = isExternalWebsiteUrl(s.websiteUrl ?? "")
		? (s.websiteUrl ?? "")
		: "";
	// The canonical storefront route uses storeCode, while chat history,
	// presence and websocket state use the internal storeId. Do not pass the
	// route id into chat; if public seller data lacks storeId, withhold the CTA.
	const chatSellerId = resolveSellerChatTargetId(s);

	return (
		<header className="hkui-seller-profile" data-hk-seller-profile-ready>
			<div className="hkui-seller-profile__cover">
				{coverImageSrc ? (
					<SellerCoverMedia
						src={coverImageSrc}
						quality={70}
						loading="eager"
						/* Banner is centered with max-width:1440px, so cap the
						   size basis there — otherwise wide external monitors
						   (1440p/4K) fetch the 1920–3840 variant for a strip
						   that never renders past 1440px. */
						sizes="(min-width: 1456px) 1440px, 100vw"
					/>
				) : (
					<div className="hkui-seller-profile__cover-placeholder" />
				)}
				<BackLink
					fallbackHref="/"
					className={`${COMPACT_BACK_CONTROL_CLASSNAME} hkui-seller-profile__back`}
					data-hkui-button=""
					aria-label="返回"
				>
					<CompactBackIcon />
					<span>返回</span>
				</BackLink>
			</div>

			<div className="hkui-seller-profile__head">
				<div className="hkui-seller-profile__avatar">
					<Avatar
						src={avatarSrc}
						name={name}
						alt={name}
						entity="seller"
						fallback="empty"
					/>
				</div>

				<div className="hkui-seller-profile__title">
					{/* The route subtree owns the single semantic <h1>; this
					    persistent visible store name stays a non-heading so tab
					    changes cannot duplicate or remount the page heading. */}
					<div className="hkui-seller-profile__name-row">
						<p className="hkui-seller-profile__name">{name}</p>
						{prize && (
							<span className="hkui-seller-profile__prize" title={prize}>
								<AppIcon name="trophy" size={13} />
								<span>{prize}</span>
							</span>
						)}
					</div>
					<SellerProfileMeta
						rating={rating}
						totalReviews={totalReviews}
						location={location}
						subscribers={subscribers}
						websiteUrl={websiteUrl}
					/>
				</div>

				<div className="hkui-seller-profile__actions">
					{chatSellerId ? (
						<SellerChatButton
							sellerId={chatSellerId}
							className="hkui-seller-profile__cta hkui-seller-profile__cta--ghost hkui-seller-profile__cta--chat"
							navbarIcon
							previewMode={previewMode}
						/>
					) : null}
					<SellerShareButton
						data={shareCard}
						className="hkui-seller-profile__cta hkui-seller-profile__cta--ghost hkui-seller-profile__cta--card"
					/>
					<SellerSubscribeButton
						sellerId={storeId}
						className="hkui-seller-profile__cta"
						label="訂閱"
						activeLabel="已訂閱"
						previewMode={previewMode}
					/>
				</div>
			</div>

			{description && <SellerProfileDescription description={description} />}

			<SellerProfileTabs sellerId={sellerId} basePath={tabBasePath} />
		</header>
	);
}
