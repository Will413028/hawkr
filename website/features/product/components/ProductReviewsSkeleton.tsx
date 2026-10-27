import {
	PRODUCT_REVIEW_CARD_CLASS,
	PRODUCT_REVIEW_TEXT_CLASS,
	PRODUCT_REVIEW_TITLE_CLASS,
} from "./productReviewsLayout";

const BONE =
	"inline-block max-w-full rounded bg-hk-border-soft/60 animate-pulse motion-reduce:animate-none";

/** Holds the same heading, avatar, rating and text line boxes as PDP reviews. */
export default function ProductReviewsSkeleton() {
	return (
		<div className="mb-8" aria-busy="true" role="status">
			<span className="sr-only">商品評價載入中</span>
			<div aria-hidden="true">
				<div className="flex items-center justify-between mb-4">
					<div className={PRODUCT_REVIEW_TITLE_CLASS}>
						<span className={`${BONE} h-[1em] w-40 align-middle`} />
					</div>
					<div className="text-sm">
						<span className={`${BONE} h-[1em] w-14 align-middle`} />
					</div>
				</div>
				<div className="flex flex-col gap-4">
					{[0, 1].map((index) => (
						<div key={index} className={PRODUCT_REVIEW_CARD_CLASS}>
							<div className="flex items-center gap-3 mb-3">
								<div className={`${BONE} size-11 shrink-0 rounded-full`} />
								<div className="min-w-0 flex-1 flex flex-col gap-0.5">
									<span className="text-[0.9375rem] font-semibold">
										<span className={`${BONE} h-[1em] w-28 align-middle`} />
									</span>
									<span className="text-xs">
										<span className={`${BONE} h-[1em] w-20 align-middle`} />
									</span>
								</div>
								<div className="flex shrink-0 gap-0.5">
									{[0, 1, 2, 3, 4].map((star) => (
										<span key={star} className={`${BONE} size-3.5`} />
									))}
								</div>
							</div>
							<p className={PRODUCT_REVIEW_TEXT_CLASS}>
								<span className="block h-[1lh]">
									<span className={`${BONE} h-[1em] w-full align-middle`} />
								</span>
								<span className="block h-[1lh]">
									<span className={`${BONE} h-[1em] w-3/4 align-middle`} />
								</span>
							</p>
						</div>
					))}
				</div>
			</div>
		</div>
	);
}
