"use client";

import { errorText } from "@hawkr/api/connect/errors";
import {
	type InfiniteData,
	useInfiniteQuery,
	useMutation,
	useQueryClient,
} from "@tanstack/react-query";
import * as React from "react";
import Avatar from "@/components/shared/Avatar";
import EmptyStatePanel from "@/components/shared/EmptyStatePanel";
import { SellerPageShell } from "@/components/shared/SellerPageShell";
import Button from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { Dialog } from "@/components/ui/Dialog";
import { SellerCardGridSkeleton } from "@/features/seller-center/components/SellerSkeletons";
import { accountQueryFn } from "@/lib/auth/account-query";
import { useAccountStatus } from "@/lib/auth/use-account-status";
import { formatTaipeiDate } from "@/lib/date/taipei";
import { useQueryIdentity } from "@/lib/hooks/useQueryIdentity";
import { useDefaultErrorText } from "@/lib/i18n/ui-text";
import { queryKeys } from "@/lib/query/keys";
import {
	type BlockedBuyerItem,
	listBlockedBuyers,
	type BlockedBuyersPage as Page,
	unblockBuyer,
} from "@/lib/seller/block-buyer";

const DELETED_BUYER_LABEL = "已刪除帳號";
const FALLBACK_BUYER_LABEL = "買家";

function buyerLabel(item: BlockedBuyerItem): string {
	if (item.deleted) return DELETED_BUYER_LABEL;
	return item.name !== "" ? item.name : FALLBACK_BUYER_LABEL;
}

/**
 * 封鎖名單頁（`/seller-center/blocked-buyers`）：店家封鎖的買家，新到舊，可解除封鎖。與 seller-app 的封鎖名單對等；
 * 封鎖是保護動作，停權時仍可管理（`CONTENT_CONTRACT`）。
 */
export default function BlockedBuyersPage() {
	const defaults = useDefaultErrorText();
	const status = useAccountStatus();
	const identity = useQueryIdentity();
	const queryClient = useQueryClient();
	const queryKey = queryKeys.seller.blockedBuyers(identity);
	const query = useInfiniteQuery({
		queryKey,
		queryFn: accountQueryFn(status, ({ pageParam }: { pageParam: string }) =>
			listBlockedBuyers(pageParam),
		),
		initialPageParam: "",
		getNextPageParam: (lastPage: Page) => lastPage.nextPageToken || undefined,
	});
	const items = query.data?.pages.flatMap((page) => page.items) ?? [];

	const [confirming, setConfirming] = React.useState<BlockedBuyerItem | null>(
		null,
	);
	const [rowErrors, setRowErrors] = React.useState<Record<string, string>>({});
	const unblock = useMutation({
		mutationFn: (buyerId: string) => unblockBuyer(buyerId),
		onSuccess: (_result, buyerId) => {
			queryClient.setQueryData<InfiniteData<Page, string>>(queryKey, (data) =>
				data
					? {
							...data,
							pages: data.pages.map((page) => ({
								...page,
								items: page.items.filter((item) => item.buyerId !== buyerId),
							})),
						}
					: data,
			);
			void queryClient.invalidateQueries({ queryKey });
		},
	});

	const unblocking = React.useRef(false);
	const confirmUnblock = async () => {
		const target = confirming;
		if (!target || unblocking.current) return;
		unblocking.current = true;
		setRowErrors(({ [target.buyerId]: _cleared, ...rest }) => rest);
		try {
			await unblock.mutateAsync(target.buyerId);
		} catch (e) {
			setRowErrors((current) => ({
				...current,
				[target.buyerId]: errorText(e, defaults),
			}));
		} finally {
			unblocking.current = false;
			setConfirming(null);
		}
	};

	return (
		<SellerPageShell title="封鎖名單">
			{query.isPending ? (
				<SellerCardGridSkeleton variant="panel" count={3} />
			) : query.isError && items.length === 0 ? (
				<Card className="flex flex-col items-center gap-3 p-6 text-center text-sm text-[var(--hkui-foreground-soft)]">
					<p className="m-0">{errorText(query.error, defaults)}</p>
					<Button
						variant="bordered"
						color="default"
						size="sm"
						onClick={() => void query.refetch()}
					>
						重試
					</Button>
				</Card>
			) : items.length === 0 && !query.hasNextPage ? (
				<EmptyStatePanel
					icon="search"
					title="尚無封鎖名單"
					description="你封鎖的使用者會顯示在這裡。"
				/>
			) : (
				<div className="flex flex-col gap-[var(--hkui-space-4)]">
					<ul className="m-0 flex list-none flex-col gap-3 p-0">
						{items.map((item) => {
							const label = buyerLabel(item);
							const date = item.blockedAt
								? formatTaipeiDate(item.blockedAt)
								: "";
							const rowError = rowErrors[item.buyerId];
							return (
								<li
									key={item.buyerId}
									className="seller-card flex flex-col gap-2"
								>
									<div className="flex items-center gap-3">
										<Avatar
											entity="buyer"
											src={item.avatarUrl}
											name={label}
											className="h-9 w-9 rounded-full object-cover"
										/>
										<div className="flex min-w-0 flex-1 flex-col">
											<strong className="truncate text-base font-medium text-hk-heading">
												{label}
											</strong>
											{date ? (
												<small className="text-xs text-foreground-soft">
													{date}
												</small>
											) : null}
										</div>
										<Button
											variant="bordered"
											color="default"
											size="sm"
											onClick={() => {
												setRowErrors(
													({ [item.buyerId]: _cleared, ...rest }) => rest,
												);
												setConfirming(item);
											}}
										>
											解除封鎖
										</Button>
									</div>
									{rowError ? (
										<p
											className="m-0 text-sm text-[var(--hkui-danger,#d4380d)]"
											role="alert"
										>
											{rowError}
										</p>
									) : null}
								</li>
							);
						})}
					</ul>
					{query.isError ? (
						<p
							className="m-0 text-center text-sm text-[var(--hkui-danger,#d4380d)]"
							role="alert"
						>
							{errorText(query.error, defaults)}
						</p>
					) : null}
					{query.hasNextPage ? (
						<div className="flex justify-center">
							<Button
								variant="bordered"
								color="default"
								size="sm"
								disabled={query.isFetchingNextPage}
								onClick={() => void query.fetchNextPage()}
							>
								載入更多
							</Button>
						</div>
					) : null}
				</div>
			)}
			<Dialog
				open={confirming !== null}
				onClose={() => {
					if (!unblock.isPending) setConfirming(null);
				}}
				size="sm"
				title={confirming ? `解除封鎖「${buyerLabel(confirming)}」？` : ""}
				footer={
					<div className="flex w-full justify-end gap-2">
						<Button
							variant="light"
							color="default"
							onClick={() => setConfirming(null)}
							disabled={unblock.isPending}
						>
							取消
						</Button>
						<Button
							onClick={() => void confirmUnblock()}
							disabled={unblock.isPending}
						>
							解除封鎖
						</Button>
					</div>
				}
			>
				<p className="text-sm text-[var(--hkui-default-700)]">
					解除後對方可以重新傳送訊息或留言給你。
				</p>
			</Dialog>
		</SellerPageShell>
	);
}
