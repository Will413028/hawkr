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
	sellerId: string;
	/** Seller-center preview uses the same buyer component but blocks writes. */
	previewMode?: boolean;
	/** Keeps storefront tabs inside the preview route instead of public routes. */
	tabBasePath?: string;
}

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
	rating: string;
	totalReviews: number;
	location: string;
	subscribers: number;
	websiteUrl: string;
}) {
	const hasRating = Number(rating) > 0;
	const hasLocation = Boolean(location);
	return (
		<div className="hkui-seller-profile__meta">
			{hasRating && (
				<span className="hkui-seller-profile__meta-item hkui-seller-profile__meta-item--rating">
					<StarIcon />
					<strong>{rating}</strong>
					<span>({totalReviews})</span>
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
				<strong>{subscribers}</strong>
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

export default async function SellerProfileV2({
	seller,
	sellerId,
	previewMode = false,
	tabBasePath,
}: SellerProfileV2Props) {
	// 評價統計與訂閱數分屬兩支公開端點（/reviews/stats 帶 rating/count；
	// /stats 帶 followers），基本資料那支兩者皆無 — 併發抓、各自走同一顆
	// seller:<id> 共享快取；任一支掛了退回 0 而不是炸頁。
	const [stats, publicStats] = await Promise.all([
		getCachedSellerReviewStats(sellerId).catch(prerenderSafe(null)),
		getCachedSellerStats(sellerId).catch(prerenderSafe(null)),
	]);

	const s = seller || {};
	const coverSrc = s.coverImage || "";
	const coverImageSrc = coverSrc ? resolveCdnUrl(coverSrc) : "";
	const avatarSrc = s.logo_url?.trim() || "";
	// `fetchSeller` 已把 storeName/sellerName 收斂進 `name`（lib/api/index.ts），
	// 這裡再讀一次舊 snake 欄只會拿到 undefined。
	const name = s.name || "商家";
	const location = sellerProfileLocation(s);
	const avgRating = stats ? Number(stats.avgRating) || 0 : 0;
	const rating =
		avgRating > 0 ? avgRating.toFixed(1) : Number(s.rating || 0).toFixed(1);
	const totalReviews = stats
		? Number(stats.totalCount) || 0
		: s.totalReviews || 0;
	const subscribers = publicStats ? Number(publicStats.followers) || 0 : 0;
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
						sellerId={sellerId}
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
