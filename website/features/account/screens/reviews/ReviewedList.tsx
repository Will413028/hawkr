"use client";

import AppImage from "@/components/shared/AppImage";
import Link from "@/components/shared/Link";
import Button from "@/components/ui/Button";
import { Stars } from "@/components/ui/Stars";
import { AccountEmptyStateIcon } from "@/features/account/components/AccountEmptyIcon";
import { formatTaipeiDate as formatDate } from "@/lib/date/taipei";
import { useMyReviews } from "@/lib/hooks/useMyReviews";
import { resolveCdnUrl } from "@/lib/media/cdn";

/** "已評價" tab — reviews the buyer has already written (view-only). */
export default function ReviewedList() {
	const { reviews, loading, loadingMore, error, hasMore, loadMore } =
		useMyReviews();

	if (loading && reviews.length === 0) {
		return <p className="text-foreground-soft text-sm">載入中…</p>;
	}
	if (error && reviews.length === 0) {
		return <p className="text-danger text-sm">載入評價失敗，請稍後再試。</p>;
	}
	if (reviews.length === 0) {
		return (
			<div className="flex flex-col items-center justify-center gap-3 py-8 px-5 text-center text-foreground-soft">
				<AccountEmptyStateIcon variant="reviews" />
				<p className="text-base font-extrabold text-foreground m-0">
					還沒有評價過的商品
				</p>
				<p className="max-w-[32ch] text-[0.8125rem] leading-[1.6] m-0">
					完成評價後會顯示在這裡。
				</p>
			</div>
		);
	}

	return (
		<div className="flex flex-col gap-3">
			<ul className="list-none m-0 p-0 flex flex-col gap-3">
				{reviews.map((r) => {
					const card = (
						<div className="flex flex-col gap-3 p-4">
							<div className="grid grid-cols-[64px_1fr_auto] gap-3 items-start">
								<AppImage
									src={resolveCdnUrl(r.productImageKey)}
									width={64}
									height={64}
									sizes="64px"
									alt={r.skuName || "商品圖片"}
									className="w-16 h-16 rounded-hkui-md object-cover bg-default-100"
									loading="eager"
								/>
								<div className="flex flex-col gap-1 min-w-0">
									<strong className="text-sm font-semibold truncate">
										{r.skuName || "商品"}
									</strong>
									<div className="flex gap-0.5">
										<Stars count={r.rating} size={14} />
									</div>
									{r.content ? (
										<p className="text-sm text-hk-body m-0 line-clamp-3">
											{r.content}
										</p>
									) : null}
								</div>
								<small className="text-xs text-foreground-soft whitespace-nowrap">
									{formatDate(r.createdAt)}
								</small>
							</div>
							{r.imageKeys.length > 0 ? (
								<div className="flex gap-2 flex-wrap">
									{r.imageKeys.map((img) => (
										<AppImage
											key={img}
											src={resolveCdnUrl(img)}
											width={56}
											height={56}
											sizes="56px"
											alt=""
											className="w-14 h-14 rounded-hkui-md object-cover bg-default-100"
											loading="eager"
										/>
									))}
								</div>
							) : null}
						</div>
					);
					return (
						<li
							key={r.id}
							className="bg-content1 border border-divider rounded-hkui-lg transition-colors duration-200 hover:border-hkui-primary-200"
						>
							{r.orderNo ? (
								<Link
									href={`/account/orders/${r.orderNo}`}
									className="block text-inherit no-underline"
								>
									{card}
								</Link>
							) : (
								card
							)}
						</li>
					);
				})}
			</ul>
			{hasMore ? (
				<div className="flex justify-center">
					<Button
						onClick={loadMore}
						variant="bordered"
						color="default"
						disabled={loadingMore}
						isLoading={loadingMore}
						loadingLabel="載入評論中…"
					>
						載更多
					</Button>
				</div>
			) : null}
		</div>
	);
}
