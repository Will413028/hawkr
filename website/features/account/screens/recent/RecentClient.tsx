"use client";

import * as React from "react";
import AppImage from "@/components/shared/AppImage";
import Avatar from "@/components/shared/Avatar";
import Link from "@/components/shared/Link";
import { SellerPageShell } from "@/components/shared/SellerPageShell";
import Button from "@/components/ui/Button";
import { AccountEmptyStateIcon } from "@/features/account/components/AccountEmptyIcon";
import { formatTaipeiDate } from "@/lib/date/taipei";
import { useHistory } from "@/lib/hooks/useAccount";
import { resolveCdnUrl } from "@/lib/media/cdn";
import { sellerPath } from "@/lib/seo/urls";

function formatPriceRange(min?: string, max?: string): string {
	const toNum = (v?: string) => {
		const n = Number(v);
		return Number.isFinite(n) && n > 0 ? n : 0;
	};
	const lo = toNum(min);
	const hi = toNum(max);
	const fmt = (n: number) => `NT$ ${Math.round(n).toLocaleString("zh-TW")}`;
	if (lo > 0 && hi > 0 && lo !== hi) return `${fmt(lo)} ~ ${fmt(hi)}`;
	if (lo > 0) return fmt(lo);
	if (hi > 0) return fmt(hi);
	return "—";
}

function formatRating(raw?: string): string | null {
	const n = Number(raw);
	if (!Number.isFinite(n) || n <= 0) return null;
	return n.toFixed(1);
}

function formatVisitedAt(raw?: string): string {
	if (!raw) return "";
	const t = Date.parse(raw);
	if (Number.isNaN(t)) return raw;
	const diff = Date.now() - t;
	const minute = 60_000;
	const hour = 60 * minute;
	const day = 24 * hour;
	if (diff < minute) return "剛剛";
	if (diff < hour) return `${Math.floor(diff / minute)} 分鐘前`;
	if (diff < day) return `${Math.floor(diff / hour)} 小時前`;
	if (diff < 7 * day) return `${Math.floor(diff / day)} 天前`;
	return formatTaipeiDate(t);
}

export default function RecentClient() {
	const { items, loading, error, clear, hasMore, loadMore } = useHistory();
	const [loadingMore, setLoadingMore] = React.useState(false);
	const [loadError, setLoadError] = React.useState<string | null>(null);
	const onLoadMore = async () => {
		if (loadingMore) return;
		setLoadingMore(true);
		setLoadError(null);
		try {
			await loadMore();
		} catch {
			setLoadError("載入瀏覽紀錄失敗");
		} finally {
			setLoadingMore(false);
		}
	};

	// Hide products that are no longer visible: backend flags them with
	// `available === false`, or returns them with an empty productName
	// (deleted/unpublished — the row that used to render "商品 #<id>").
	const visibleItems = items.filter(
		(it) => it.available !== false && (it.productName?.trim() ?? "") !== "",
	);

	return (
		<main className="recent-page">
			<SellerPageShell
				title="瀏覽紀錄"
				subtitle="逛過的商品會自動出現在這裡。"
				actions={
					visibleItems.length > 0 ? (
						<Button
							variant="bordered"
							color="danger"
							size="sm"
							onClick={() => {
								if (window.confirm("清除全部瀏覽紀錄？")) {
									void clear();
								}
							}}
						>
							清除全部
						</Button>
					) : null
				}
			>
				{error && (
					<p className="" role="alert">
						{error}
					</p>
				)}

				{loading && visibleItems.length === 0 ? (
					<p className="">載入中…</p>
				) : visibleItems.length === 0 ? (
					<div className="flex flex-col items-center justify-center gap-3 py-8 px-5 text-center text-foreground-soft">
						<AccountEmptyStateIcon variant="recent" />
						<p className="text-base font-extrabold text-foreground m-0">
							沒有瀏覽紀錄
						</p>
						<p className="max-w-[32ch] text-[0.8125rem] leading-[1.6] m-0">
							逛過的商品會自動出現在這裡。
						</p>
					</div>
				) : (
					<ul className="list-none m-0 p-0 flex flex-col gap-2">
						{visibleItems.map((it) => {
							const rating = formatRating(it.rating);
							const visited = formatVisitedAt(it.visitedAt);
							const sellerInner = (
								<>
									<Avatar
										src={it.logoUrl}
										alt=""
										entity="seller"
										loading="eager"
										className="w-4 h-4 rounded-full object-cover bg-default-100 shrink-0"
										fallback="empty"
									/>
									<em className="not-italic overflow-hidden text-ellipsis whitespace-nowrap">
										{it.sellerName}
									</em>
								</>
							);
							return (
								<li
									key={String(it.id)}
									className="relative bg-content1 border border-divider rounded-hkui-lg transition-[border-color] duration-200 ease-[var(--hkui-ease)] hover:border-hkui-primary-200"
								>
									{/* Stretched link: the whole card navigates to the product,
									    while the seller link sits on top as a sibling (not nested). */}
									<Link
										href={`/products/${it.productId}?from=recent`}
										aria-label={it.productName || `商品 #${it.productId}`}
										className="absolute inset-0 z-0 rounded-hkui-lg"
									/>
									<div className="flex items-stretch gap-3 py-3 px-4 pointer-events-none">
										<AppImage
											src={resolveCdnUrl(it.imageKey || it.imageUrl)}
											alt=""
											width={72}
											height={72}
											sizes="72px"
											loading="eager"
											className="w-[72px] h-[72px] shrink-0 rounded-hkui-md object-cover bg-default-100"
										/>
										<div className="flex flex-col gap-1 min-w-0 flex-1">
											<strong className="text-base leading-[1.3] line-clamp-2">
												{it.productName || `商品 #${it.productId}`}
											</strong>
											{it.sellerName ? (
												sellerPath(it.storeCode) ? (
													<Link
														href={sellerPath(it.storeCode)}
														className="relative z-10 pointer-events-auto inline-flex items-center gap-1.5 w-fit max-w-full text-xs text-foreground-soft no-underline min-w-0 hover:text-hkui-primary-600"
													>
														{sellerInner}
													</Link>
												) : (
													<span className="inline-flex items-center gap-1.5 text-xs text-foreground-soft min-w-0">
														{sellerInner}
													</span>
												)
											) : null}
											<div className="flex items-center flex-wrap gap-2 mt-auto pt-0.5">
												<span className="text-base font-semibold text-hkui-primary-600">
													{formatPriceRange(it.minPrice, it.maxPrice)}
												</span>
												{rating ? (
													<span
														className="text-xs text-warning-600"
														title={`評價 ${rating} 顆星`}
													>
														★ {rating}
													</span>
												) : null}
												{visited ? (
													<span className="text-xs text-foreground-soft ml-auto">
														{visited}
													</span>
												) : null}
											</div>
										</div>
									</div>
								</li>
							);
						})}
					</ul>
				)}
				{loadError && <p role="alert">{loadError}</p>}
				{hasMore && (
					<div className="flex justify-center mt-4">
						<Button
							variant="bordered"
							isLoading={loadingMore}
							loadingLabel="載入更多瀏覽紀錄中…"
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
