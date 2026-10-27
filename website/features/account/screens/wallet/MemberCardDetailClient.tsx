"use client";

import { useInfiniteQuery } from "@tanstack/react-query";
import * as React from "react";
import Link from "@/components/shared/Link";
import { SellerPageShell } from "@/components/shared/SellerPageShell";
import Button from "@/components/ui/Button";
import { AccountEmptyStateIcon } from "@/features/account/components/AccountEmptyIcon";
import OrderQuickActionDrawer from "@/features/account/screens/orders/OrderQuickActionDrawer";
import { useAuth } from "@/lib/auth/context";
import { formatTaipeiDate } from "@/lib/date/taipei";
import { useOrder } from "@/lib/hooks/useOrders";
import { useQueryIdentity } from "@/lib/hooks/useQueryIdentity";
import { fetchSpendingRecords } from "@/lib/member-cards/repo";
import { queryKeys } from "@/lib/query/keys";

function formatAmount(raw: string): string {
	const n = Number(raw);
	if (!Number.isFinite(n)) return raw || "0";
	return n.toLocaleString("zh-TW", { maximumFractionDigits: 0 });
}

function formatDate(raw: string): string {
	// `orderDate` 的契約是**台北牆上時間的字串**（gateway
	// `SpendingRecordResponse.OrderDate`：`YYYY/MM/DD（Asia/Taipei）`），不是
	// instant。`toEpochMs` 會把這一族當台北牆上時間還原，所以輸出與瀏覽器時區
	// 脫鉤 —— 在此之前它走 `Date.parse` 的本地時區解讀，UTC+8 以東的使用者
	// （例：日本）看到的日期會少一天。無法解讀的值仍原字串照登。
	return formatTaipeiDate(raw) || raw;
}

export default function MemberCardDetailClient({
	sellerId,
	sellerName,
}: {
	sellerId: string;
	sellerName?: string;
}) {
	const { isLoggedIn } = useAuth();
	const identity = useQueryIdentity();

	const query = useInfiniteQuery({
		queryKey: queryKeys.buyer.memberCardRecords(identity, sellerId, {
			size: 50,
		}),
		queryFn: ({ pageParam }) =>
			fetchSpendingRecords({ sellerId, size: 50, pageToken: pageParam }),
		initialPageParam: "",
		getNextPageParam: (page) => page.nextPageToken || undefined,
		enabled: isLoggedIn && Boolean(sellerId),
	});
	const records = query.data?.pages.flatMap((page) => page.records) ?? [];
	const totalSpending = query.data?.pages[0]?.totalSpending ?? "0";
	const totalRefund = query.data?.pages[0]?.totalRefund ?? "0";
	const loading = query.isLoading;
	const error = query.error instanceof Error ? query.error.message : null;

	// Spending rows are slim (orderSn only), so clicking one opens the shared
	// order drawer and fetches the full order on demand — same UX as the orders
	// list, where a row opens the drawer first and only the「查看訂單詳情」
	// button navigates to the full page.
	const [activeOrderSn, setActiveOrderSn] = React.useState<string | null>(null);
	const { order: activeOrder } = useOrder(activeOrderSn ?? undefined);
	// Guard against showing a stale order while a newly-clicked one is loading.
	const drawerOrder =
		activeOrder && activeOrder.orderNo === activeOrderSn ? activeOrder : null;

	const hasRefund = Number(totalRefund) > 0;

	return (
		<main className="buyer-wallet-page">
			<SellerPageShell
				title={sellerName ? `${sellerName} · 會員卡` : "會員卡明細"}
				subtitle="你在這家商家的消費紀錄。"
				toolbar={
					<Link
						href="/account/wallet"
						className="inline-flex items-center gap-1 text-sm font-semibold text-hkui-primary no-underline hover:underline"
					>
						← 回會員卡
					</Link>
				}
			>
				<section className="seller-card grid grid-cols-2 gap-4 mb-5 max-[600px]:grid-cols-1">
					<div className="flex flex-col gap-1">
						<small className="text-xs text-foreground-soft">累計消費</small>
						<strong className="font-display text-3xl text-hkui-primary tracking-[var(--hkui-tracking-tight)] leading-none">
							NT$ {formatAmount(totalSpending)}
						</strong>
					</div>
					{hasRefund && (
						<div className="flex flex-col gap-1">
							<small className="text-xs text-foreground-soft">累計退款</small>
							<span className="text-lg font-semibold text-foreground">
								NT$ {formatAmount(totalRefund)}
							</span>
						</div>
					)}
				</section>

				<section className="seller-card">
					<header className="seller-card__head">
						<h2>消費明細</h2>
					</header>

					{error && (
						<p className="text-danger-700 text-sm font-semibold" role="alert">
							{error}
						</p>
					)}

					{loading && records.length === 0 ? (
						<p className="text-foreground-soft text-sm">載入中…</p>
					) : records.length === 0 ? (
						<div className="flex flex-col items-center justify-center gap-3 py-8 px-5 text-center text-foreground-soft">
							<AccountEmptyStateIcon variant="wallet" />
							<p className="text-base font-extrabold text-foreground m-0">
								還沒有消費紀錄
							</p>
						</div>
					) : (
						<ul className="list-none m-0 p-0 flex flex-col">
							{records.map((rec) => {
								const refund = Number(rec.refund);
								return (
									<li
										key={rec.orderSn}
										className="border-b border-divider last:border-b-0"
									>
										<button
											type="button"
											onClick={() => setActiveOrderSn(rec.orderSn)}
											className="flex w-full justify-between items-center py-3 gap-3 text-left cursor-pointer"
										>
											<div className="flex flex-col gap-0.5 min-w-0">
												<strong className="text-base text-foreground truncate">
													訂單 {rec.orderSn}
												</strong>
												<small className="text-xs text-foreground-soft flex items-center gap-2">
													<span>{formatDate(rec.orderDate)}</span>
													{rec.hasCoupon && (
														<span className="text-hkui-primary">
															已用優惠券
														</span>
													)}
													{refund > 0 && (
														<span className="text-danger">
															退款 NT$ {formatAmount(rec.refund)}
														</span>
													)}
												</small>
											</div>
											<b className="text-base font-bold text-foreground shrink-0">
												NT$ {formatAmount(rec.amount)}
											</b>
										</button>
									</li>
								);
							})}
						</ul>
					)}
					{query.hasNextPage && (
						<div className="flex justify-center mt-4">
							<Button
								variant="bordered"
								isLoading={query.isFetchingNextPage}
								loadingLabel="載入更多消費紀錄中…"
								onClick={() => void query.fetchNextPage()}
							>
								載入更多
							</Button>
						</div>
					)}
				</section>
			</SellerPageShell>

			<OrderQuickActionDrawer
				order={drawerOrder}
				open={activeOrderSn !== null}
				loading={activeOrderSn !== null && !drawerOrder}
				onClose={() => setActiveOrderSn(null)}
			/>
		</main>
	);
}
