"use client";

import AppImage from "@/components/shared/AppImage";
import Link from "@/components/shared/Link";
import Button from "@/components/ui/Button";
import { AccountEmptyStateIcon } from "@/features/account/components/AccountEmptyIcon";
import { useOrderList } from "@/lib/hooks/useOrders";
import { resolveCdnUrl } from "@/lib/media/cdn";

/**
 * "待評價" tab — completed orders still awaiting a review. An order drops off
 * once the backend flags `isReviewed` (any item reviewed); `canReview` stays
 * true through the review window so it cannot gate this on its own.
 */
export default function PendingReviewsList() {
	const list = useOrderList("completed");
	const todo = list.orders.filter(
		(o) => o.canReview !== false && !o.isReviewed,
	);

	if (list.loading && list.orders.length === 0) {
		return <p className="text-foreground-soft text-sm">載入中…</p>;
	}
	if (todo.length === 0) {
		return (
			<div className="flex flex-col items-center justify-center gap-3 py-8 px-5 text-center text-foreground-soft">
				<AccountEmptyStateIcon variant="reviews" />
				<p className="text-base font-extrabold text-foreground m-0">
					沒有待評價的訂單
				</p>
				<p className="max-w-[32ch] text-[0.8125rem] leading-[1.6] m-0">
					訂單完成後可以在這裡撰寫評價。
				</p>
				<Button href="/account/orders" variant="bordered" color="default">
					看全部訂單
				</Button>
			</div>
		);
	}

	return (
		<ul className="list-none m-0 p-0 flex flex-col gap-3">
			{todo.map((order) => {
				const cover = order.items[0]?.image;
				const itemCount = order.items.reduce(
					(sum, it) => sum + Number(it.quantity ?? 0),
					0,
				);
				return (
					<li
						key={order.orderNo}
						className="bg-content1 border border-divider rounded-hkui-lg transition-colors duration-200 hover:border-hkui-primary-200"
					>
						<Link
							href={`/account/orders/${order.orderNo}/review`}
							className="flex flex-col gap-3 p-4 text-inherit no-underline"
						>
							<div className="flex justify-between items-center">
								<strong className="text-base font-semibold">
									{order.sellerName || "未知賣家"}
								</strong>
								<span className="text-xs font-semibold px-2.5 py-1 rounded-full bg-success-50 text-success-700">
									等你評價
								</span>
							</div>
							<div className="grid grid-cols-[64px_1fr_auto] gap-3 items-center">
								{cover ? (
									<AppImage
										src={resolveCdnUrl(cover)}
										width={64}
										height={64}
										sizes="64px"
										alt={order.items[0]?.name || ""}
										className="w-16 h-16 rounded-hkui-md object-cover bg-default-100"
										loading="eager"
									/>
								) : (
									<span
										className="w-16 h-16 rounded-hkui-md bg-default-100"
										aria-hidden="true"
									/>
								)}
								<div className="flex flex-col gap-[2px] text-sm text-hk-body">
									{order.items.slice(0, 2).map((it) => (
										<span key={String(it.orderItemId)}>
											{it.name}
											<small className="text-foreground-soft">
												{" "}
												× {it.quantity}
											</small>
										</span>
									))}
									{order.items.length > 2 && (
										<small className="text-foreground-soft">
											等 {order.items.length} 項商品
										</small>
									)}
								</div>
								<div className="text-right flex flex-col">
									<small className="text-xs text-foreground-soft">
										共 {itemCount} 件
									</small>
									<strong className="text-base text-hk-primary font-bold">
										寫評價 →
									</strong>
								</div>
							</div>
						</Link>
					</li>
				);
			})}
		</ul>
	);
}
