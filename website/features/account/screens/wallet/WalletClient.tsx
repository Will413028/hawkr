"use client";

import * as React from "react";
import Avatar from "@/components/shared/Avatar";
import Link from "@/components/shared/Link";
import { SellerPageShell } from "@/components/shared/SellerPageShell";
import Button from "@/components/ui/Button";
import { AccountEmptyStateIcon } from "@/features/account/components/AccountEmptyIcon";
import { useMemberCards } from "@/lib/hooks/useAccount";
import { storeCardView } from "@/lib/seller/storeCardView";
import { useStoreCardText } from "@/lib/seller/useStoreCardViews";

function formatSpending(raw: string): string {
	const n = Number(raw);
	if (!Number.isFinite(n)) return raw || "0";
	return n.toLocaleString("zh-TW", { maximumFractionDigits: 0 });
}

export default function WalletClient() {
	const { cards, loading, error, hasMore, loadMore } = useMemberCards();
	const storeCardText = useStoreCardText();
	const [loadingMore, setLoadingMore] = React.useState(false);
	const [loadError, setLoadError] = React.useState<string | null>(null);
	const onLoadMore = async () => {
		if (loadingMore) return;
		setLoadingMore(true);
		setLoadError(null);
		try {
			await loadMore();
		} catch {
			setLoadError("載入會員卡失敗");
		} finally {
			setLoadingMore(false);
		}
	};

	return (
		<main className="buyer-wallet-page">
			<SellerPageShell
				title="會員卡"
				subtitle="你曾消費過的商家，以及在每一家的累計金額。"
			>
				<section className="seller-card">
					<header className="seller-card__head">
						<h2>我的會員卡</h2>
					</header>

					{error && (
						<p className="text-danger-700 text-sm font-semibold" role="alert">
							{error}
						</p>
					)}

					{loading && cards.length === 0 ? (
						<p className="text-foreground-soft text-sm">載入中…</p>
					) : cards.length === 0 ? (
						<div className="flex flex-col items-center justify-center gap-3 py-8 px-5 text-center text-foreground-soft">
							<AccountEmptyStateIcon variant="wallet" />
							<p className="text-base font-extrabold text-foreground m-0">
								還沒有會員卡
							</p>
							<p className="max-w-[32ch] text-[0.8125rem] leading-[1.6] m-0">
								在商家完成第一筆訂單後，就會自動建立該商家的會員卡。
							</p>
							<Button href="/market/products" size="sm">
								去逛逛
							</Button>
						</div>
					) : (
						<ul className="list-none m-0 p-0 flex flex-col gap-3">
							{cards.map((card) => {
								const storeView = storeCardView(card.store, storeCardText);
								return (
									<li key={card.sellerId}>
										<Link
											href={`/account/wallet/${card.sellerId}?name=${encodeURIComponent(card.sellerName)}`}
											className="flex items-center gap-3 p-4 bg-content1 dark:bg-white/4 border border-divider rounded-hkui-lg no-underline transition-colors hover:border-hkui-primary-200"
										>
											<span className="inline-flex items-center justify-center w-12 h-12 rounded-full overflow-hidden bg-default-100 shrink-0">
												<Avatar
													className="w-full h-full object-cover"
													src={card.sellerLogo}
													name={card.sellerName}
													alt=""
													entity="seller"
													fallback="empty"
												/>
											</span>
											<div className="flex flex-col gap-0.5 min-w-0 flex-1">
												<div className="flex items-center gap-2 min-w-0">
													<strong className="text-base text-foreground truncate">
														{storeView?.name || card.sellerName || "商家"}
													</strong>
													{storeView?.labelText ? (
														<small className="shrink-0 text-xs text-foreground-soft">
															{storeView.labelText}
														</small>
													) : null}
													{card.isVip && (
														<span className="shrink-0 text-xs font-bold px-2 py-0.5 rounded-full bg-warning-50 text-warning-700 dark:bg-[#f1cd70]/16 dark:text-[#f1cd70]">
															VIP
														</span>
													)}
												</div>
												<small className="text-xs text-foreground-soft">
													累計消費 NT$ {formatSpending(card.totalSpending)}
												</small>
											</div>
										</Link>
									</li>
								);
							})}
						</ul>
					)}
				</section>
				{loadError && <p role="alert">{loadError}</p>}
				{hasMore && (
					<div className="flex justify-center mt-4">
						<Button
							variant="bordered"
							isLoading={loadingMore}
							loadingLabel="載入更多會員卡中…"
							onClick={() => void onLoadMore()}
						>
							載入更多
						</Button>
					</div>
				)}
			</SellerPageShell>
		</main>
	);
}
