import "@/styles/features/seller-profile.css";
import AppIcon from "@/components/shared/AppIcon";
import Avatar from "@/components/shared/Avatar";
import { BackLink } from "@/components/shared/BackLink";
import {
	COMPACT_BACK_CONTROL_CLASSNAME,
	CompactBackIcon,
} from "@/components/shared/CompactBackControl";
import ReportMenu from "@/features/report/ReportMenu";
import SellerChatButton from "@/features/seller/components/SellerChatButton";
import SellerCoverMedia from "@/features/seller/components/SellerCoverMedia";
import SellerProfileDescription from "@/features/seller/components/SellerProfileDescription";
import SellerProfileTabs from "@/features/seller/components/SellerProfileTabs";
import SellerShareButton from "@/features/seller/components/SellerShareButton";
import SellerSubscribeButton from "@/features/seller/components/SellerSubscribeButton";
import SellerWebsiteLink from "@/features/seller/components/SellerWebsiteLink";
import { resolveSellerChatTargetId } from "@/features/seller/components/seller-chat-guard";
import { resolveCdnUrl } from "@/lib/media/cdn";
import { buildSellerShareCardData } from "@/lib/seller/share-card";
import { isExternalWebsiteUrl } from "@/lib/seller/website-url";

/**
 * 店面頂部讀的店家：`PublicStoreView`（`lib/seller/public-store.ts`）。`coverImage` / `logo_url` 是已組好的
 * 顯示欄位；統計欄位讀不到時不設。
 */
interface SellerProfileLike {
	/** `StoreStats`；讀不到的欄位不設，顯示「—」。 */
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
	storeId?: string;
	storeCode?: string;
	websiteUrl?: string;
}

interface SellerProfileV2Props {
	seller: SellerProfileLike | null;
	/** 網址上的店家段（公開店面是 store_code）；追蹤與統計用 `seller.storeId`。 */
	sellerId: string;
	/** 賣家預覽：聊天與訂閱按鈕只呈現、不送出。 */
	previewMode?: boolean;
	/** 賣家預覽的分頁留在預覽路由，不跳到公開店面。 */
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
 * 店面頁頂部的三項統計。取 `StoreStats`（公開店面是 `GetStore`，賣家預覽是 `GetMyStore`）：欄位沒有設定是
 * 讀不到（評分另有「沒有評價」的情形，由評價數 0 區分），一律顯示「—」，不當成 0。
 */
function profileStats(s: SellerProfileLike): ProfileStats {
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

export default function SellerProfileV2({
	seller,
	sellerId,
	previewMode = false,
	tabBasePath,
}: SellerProfileV2Props) {
	const s = seller || {};
	const storeId = String(s.storeId ?? "").trim();
	const { rating, totalReviews, subscribers } = profileStats(s);
	const coverSrc = s.coverImage || "";
	const coverImageSrc = coverSrc ? resolveCdnUrl(coverSrc) : "";
	const avatarSrc = s.logo_url?.trim() || "";
	const name = s.name || "商家";
	const location = sellerProfileLocation(s);
	const description = s.description || "";
	const prize = String(s.prize || "").trim();
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
	const websiteUrl = isExternalWebsiteUrl(s.websiteUrl ?? "")
		? (s.websiteUrl ?? "")
		: "";
	const chatSellerId = resolveSellerChatTargetId(s);

	return (
		<header className="hkui-seller-profile" data-hk-seller-profile-ready>
			<div className="hkui-seller-profile__cover">
				{coverImageSrc ? (
					<SellerCoverMedia
						src={coverImageSrc}
						quality={70}
						loading="eager"
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
					{!previewMode && storeId ? <ReportMenu storeId={storeId} /> : null}
				</div>
			</div>

			{description && <SellerProfileDescription description={description} />}

			<SellerProfileTabs sellerId={sellerId} basePath={tabBasePath} />
		</header>
	);
}
