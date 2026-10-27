import AnchorLink from "@/components/shared/AnchorLink";
import Avatar from "@/components/shared/Avatar";
import { Stars } from "@/components/ui/Stars";
import type { ProductReview } from "@/features/product/ProductDetailLayout";
import { ossImageUrl } from "@/lib/media/cdn";
import { parseRating } from "@/lib/products/rating";
import {
	PRODUCT_REVIEW_CARD_CLASS,
	PRODUCT_REVIEW_TEXT_CLASS,
	PRODUCT_REVIEW_TITLE_CLASS,
} from "./productReviewsLayout";

/**
 * PDP「商品評價」區塊（presentational）。從 ProductDetailLayout 抽出，讓正式
 * PDP 的串流 path（ReviewsAsync）與預覽 path（ProductDetailLayout 直接內嵌）
 * 共用同一份 markup，不會雙份維護而漂移。無 client-only hook，server / client
 * 兩側都可渲染。
 */

function textOrEmpty(value: unknown): string {
	return typeof value === "string" ? value.trim() : "";
}

function textOrNumber(value: unknown): string {
	if (typeof value === "string") return value.trim();
	if (typeof value === "number" && Number.isFinite(value)) return String(value);
	return "";
}

export default function ProductReviewsSection({
	reviews,
	reviewTotal,
}: {
	reviews: ProductReview[];
	reviewTotal: number;
}) {
	return (
		<div className="mb-8" id="product-reviews">
			<div className="flex items-center justify-between mb-4">
				<h2 className={PRODUCT_REVIEW_TITLE_CLASS}>商品評價 ({reviewTotal})</h2>
				{reviews.length > 0 && (
					<AnchorLink
						href="#product-reviews"
						className="text-sm text-hk-primary font-medium no-underline hover:underline"
					>
						查看全部
					</AnchorLink>
				)}
			</div>
			{reviews.length === 0 ? (
				<div className="public-section-surface bg-white/85 backdrop-blur-[12px] border-[1.5px] border-white/60 rounded-hk-xl py-7 px-8 max-md:p-5 max-md:rounded-2xl dark:bg-white/4 dark:backdrop-blur-[12px] dark:backdrop-saturate-[120%] dark:border-[#f5dea5]/14 text-center py-8 px-6">
					<p className="m-0 text-[var(--ink-soft,var(--hkui-foreground-soft))] text-[0.9375rem] leading-[1.55]">
						還沒有評價，快來成為第一個分享心得的買家。
					</p>
				</div>
			) : null}
			<div className="flex flex-col gap-4">
				{reviews.map((review, index) => {
					const reviewPhotos = Array.isArray(review.photos)
						? review.photos.filter(
								(photo: unknown): photo is string =>
									typeof photo === "string" && photo.trim().length > 0,
							)
						: [];
					// The public reviews endpoint omits a per-review id (it normalizes
					// to 0), so id/content alone collide ("pdrev-0" for every card).
					// Prefix the list index to guarantee sibling-unique keys; the list
					// is fetched once and never reordered client-side.
					const reviewKey = `${index}-${
						textOrNumber(review.id ?? review.reviewId) ||
						textOrEmpty(review.text ?? review.content).slice(0, 32) ||
						"review"
					}`;

					return (
						<div
							key={`pdrev-${reviewKey}`}
							className={PRODUCT_REVIEW_CARD_CLASS}
						>
							<div className="flex items-center gap-3 mb-3">
								<Avatar
									src={textOrEmpty(review.avatar)}
									name={review.author || "買家"}
									alt={review.author || "買家"}
									entity="buyer"
									loading="eager"
									className="w-11 h-11 rounded-full object-cover"
								/>
								<div className="flex-1 flex flex-col gap-0.5">
									<span className="text-[0.9375rem] font-semibold text-hk-heading">
										{review.author || "買家"}
									</span>
									<span className="text-xs text-hk-body">
										{textOrEmpty(review.date ?? review.createdAt)}
									</span>
								</div>
								<div className="flex gap-0.5">
									<Stars
										count={Math.round(parseRating(review.rating) ?? 0)}
										size={14}
									/>
								</div>
							</div>
							<p className={PRODUCT_REVIEW_TEXT_CLASS}>
								{textOrEmpty(review.text ?? review.content)}
							</p>
							{reviewPhotos.length > 0 && (
								<div className="flex gap-2.5 flex-wrap">
									{reviewPhotos.map((photo) => {
										const photoSrc = ossImageUrl(photo, "productThumb");
										return (
											// biome-ignore lint/performance/noImgElement: review photo URL
											<img
												key={`rvp-${reviewKey}-${photoSrc}`}
												src={photoSrc}
												className="w-[100px] h-[72px] rounded-hk-md object-cover cursor-pointer transition-transform duration-200 ease-in-out hover:scale-105"
												alt="評價照片"
												loading="eager"
											/>
										);
									})}
								</div>
							)}
						</div>
					);
				})}
			</div>
		</div>
	);
}
