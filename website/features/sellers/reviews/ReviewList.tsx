import AppImage from "@/components/shared/AppImage";
import Avatar from "@/components/shared/Avatar";
import Link from "@/components/shared/Link";
import { resolveCdnUrl } from "@/lib/media/cdn";
import { parseRating } from "@/lib/products/rating";
import { formatReviewDate } from "./formatReviewDate";

const STAR_PATH =
	"M12 2l3.09 6.26L22 9.27l-5 4.87 1.18 6.88L12 17.77l-6.18 3.25L7 14.14 2 9.27l6.91-1.01L12 2z";

export interface SellerReviewItem {
	id: string | number;
	author?: string;
	userName?: string;
	/** Buyer avatar objectKey — `fetchSellerReviews` folds the wire's
	 * `avatarKey` onto this field; `<Avatar>` resolves it. */
	avatar?: string;
	rating?: number;
	score?: number;
	date?: string;
	createdAt?: string | number | Date;
	text?: string;
	content?: string;
	/** Review photo objectKeys — `fetchSellerReviews` folds the wire's
	 * `imageKeys` onto this field. It is the only producer of this type, so
	 * there is no second field name to guess (website#430 D4). */
	images?: string[];
	/** 後端 PublicSellerReviewItem 已回傳：評論對應的商品名 / 商品圖 / 商品 ID。 */
	skuName?: string;
	productImageUrl?: string;
	/** 商品 ID（#1155）；有值且非 "0" 時商品名/縮圖連到商品頁。legacy 評論可能無此欄位。 */
	productId?: string;
}

export function StarsSvg({
	count,
	size = 18,
}: {
	count: number;
	size?: number;
}) {
	return (
		<>
			{[1, 2, 3, 4, 5].map((s) => (
				<svg
					aria-hidden="true"
					key={s}
					width={size}
					height={size}
					viewBox="0 0 24 24"
					fill={s <= count ? "var(--accent-warm)" : "#ddd"}
					stroke="none"
				>
					<path d={STAR_PATH} />
				</svg>
			))}
		</>
	);
}

/**
 * Presentational seller-review list. Rendered server-side for the first
 * (SEO/ISR) page inside {@link ReviewsClient} and re-rendered on the client
 * when pagination swaps to a later page. `busy` only stamps `aria-busy` while a
 * client page fetch is in flight — no visual chrome is added.
 */
export default function ReviewList({
	reviews,
	busy = false,
}: {
	reviews: SellerReviewItem[];
	busy?: boolean;
}) {
	return (
		<div
			className="flex flex-col gap-[var(--space-4)]"
			aria-busy={busy || undefined}
		>
			{reviews.map((review) => {
				const reviewImageList = review.images ?? [];
				const reviewPhotos = reviewImageList.filter(
					(photo): photo is string =>
						typeof photo === "string" && photo.trim().length > 0,
				);

				return (
					<div
						key={`rev-${review.id}`}
						data-hk-seller-review-item
						className="p-[var(--space-5)] border border-[var(--hkui-divider)] rounded-[var(--hkui-radius-lg)] bg-[var(--hkui-content1)] shadow-[0_10px_28px_rgb(var(--color-ink-850-rgb)/0.06)] dark:border-[rgba(242,165,65,0.2)] dark:bg-[rgb(var(--color-ink-950-rgb)/0.72)] dark:shadow-[0_16px_36px_rgba(0,0,0,0.22)]"
					>
						{review.skuName || review.productImageUrl
							? (() => {
									// #1155：有 productId（非 "0"）時整個商品列連到商品頁；legacy 無 productId 維持純顯示
									const linkableProductId =
										review.productId && review.productId !== "0"
											? review.productId
											: null;
									const rowClass =
										"flex items-center gap-[var(--space-3)] mb-[var(--space-3)] pb-[var(--space-3)] border-b border-[var(--hkui-divider)]";
									const rowInner = (
										<>
											{review.productImageUrl ? (
												<AppImage
													src={resolveCdnUrl(review.productImageUrl)}
													width={40}
													height={40}
													sizes="40px"
													alt={review.skuName || "商品"}
													className="w-10 h-10 shrink-0 rounded-[var(--hkui-radius-sm)] object-cover bg-[var(--hkui-content2)]"
													loading="eager"
												/>
											) : null}
											<span className="min-w-0 truncate text-[color:var(--text-body)] text-[length:var(--text-sm)] font-medium">
												{review.skuName || "商品"}
											</span>
										</>
									);
									return linkableProductId ? (
										<Link
											href={`/products/${encodeURIComponent(linkableProductId)}`}
											className={`${rowClass} transition-opacity hover:opacity-80`}
											aria-label={`查看商品${review.skuName ? `：${review.skuName}` : ""}`}
										>
											{rowInner}
										</Link>
									) : (
										<div className={rowClass}>{rowInner}</div>
									);
								})()
							: null}
						<div className="flex items-center gap-[var(--space-3)] mb-[var(--space-3)]">
							<Avatar
								src={review.avatar}
								name={review.author || review.userName}
								alt={review.author || review.userName}
								entity="buyer"
								className="w-10 h-10 shrink-0 rounded-full object-cover"
								fallbackClassName="w-10 h-10 shrink-0 rounded-full object-cover"
							/>
							<div className="flex-1 flex flex-col gap-[2px] min-w-0">
								<span className="text-[color:var(--text-heading)] text-[length:var(--text-sm)] font-bold">
									{review.author || review.userName}
								</span>
								<div className="flex gap-[1px] text-[var(--accent-warm)]">
									<StarsSvg
										count={Math.round(
											parseRating(review.rating ?? review.score) ?? 0,
										)}
										size={14}
									/>
								</div>
							</div>
							<span className="shrink-0 text-[var(--color-gray-mid)] text-[length:var(--text-xs)]">
								{review.date || formatReviewDate(review.createdAt)}
							</span>
						</div>
						<p className="mb-[var(--space-3)] text-[color:var(--text-body)] text-[length:var(--text-sm)] leading-relaxed">
							{review.text || review.content}
						</p>
						{reviewPhotos.length > 0 && (
							<div className="flex flex-wrap gap-[var(--space-2)]">
								{reviewPhotos.map((photo: string) => (
									<AppImage
										key={`${review.id || "review"}-${photo}`}
										className="w-[120px] h-20 rounded-[var(--hkui-radius-sm)] object-cover cursor-pointer transition-transform duration-[var(--transition-fast)] hover:scale-105 max-[480px]:w-20 max-[480px]:h-14"
										src={resolveCdnUrl(photo)}
										alt="評價照片"
										width={120}
										height={80}
										loading="eager"
										sizes="(max-width: 480px) 80px, 120px"
									/>
								))}
							</div>
						)}
					</div>
				);
			})}
			{!reviews.length && (
				<p
					style={{
						textAlign: "center",
						padding: "40px 0",
						color: "#888",
					}}
				>
					目前沒有買家評價
				</p>
			)}
		</div>
	);
}
