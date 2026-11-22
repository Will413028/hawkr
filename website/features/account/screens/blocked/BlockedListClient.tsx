"use client";

import { create } from "@bufbuild/protobuf";
import { timestampDate } from "@bufbuild/protobuf/wkt";
import { errorText } from "@hawkr/api/connect/errors";
import type {
	BlockedBuyer,
	BlockedStore,
} from "@hawkr/api/gen/hawkr/buyer/v1alpha1/buyer_pb";
import { StoreCardSchema } from "@hawkr/api/gen/hawkr/common/v1alpha1/store_card_pb";
import { buyerProfileDeletedName } from "@hawkr/ui-text/keys";
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
import { Dialog } from "@/components/ui/Dialog";
import { accountQueryFn } from "@/lib/auth/account-query";
import { useAccountStatus } from "@/lib/auth/use-account-status";
import {
	type BlockedPage,
	listBlockedBuyers,
	listBlockedStores,
	unblockBuyer,
	unblockStore,
} from "@/lib/buyer/blocklist";
import { formatTaipeiDate } from "@/lib/date/taipei";
import { useQueryIdentity } from "@/lib/hooks/useQueryIdentity";
import { useDefaultErrorText, useUiText } from "@/lib/i18n/ui-text";
import { imageUri } from "@/lib/media/image";
import { queryKeys } from "@/lib/query/keys";
import { storeCardView } from "@/lib/seller/storeCardView";
import { useStoreCardText } from "@/lib/seller/useStoreCardViews";

interface Row {
	id: string;
	name: string;
	note?: string;
	avatarUrl?: string;
	blockedAt?: string;
}

interface Target {
	kind: "store" | "buyer";
	row: Row;
}

function isoOf(t: Parameters<typeof timestampDate>[0] | undefined) {
	return t ? timestampDate(t).toISOString() : undefined;
}

function useSection<T>(
	key: readonly unknown[],
	fetchPage: (token: string) => Promise<BlockedPage<T>>,
) {
	const status = useAccountStatus();
	return useInfiniteQuery({
		queryKey: key,
		queryFn: accountQueryFn(status, ({ pageParam }: { pageParam: string }) =>
			fetchPage(pageParam),
		),
		initialPageParam: "",
		getNextPageParam: (last: BlockedPage<T>) => last.nextPageToken || undefined,
	});
}

function dropRow<T>(
	data: InfiniteData<BlockedPage<T>, string> | undefined,
	keep: (item: T) => boolean,
): InfiniteData<BlockedPage<T>, string> | undefined {
	if (!data) return data;
	return {
		...data,
		pages: data.pages.map((page) => ({
			...page,
			items: page.items.filter(keep),
		})),
	};
}

type SectionQuery<T> = ReturnType<typeof useSection<T>>;

function Section<T>({
	title,
	testId,
	query,
	toRow,
	rowErrors,
	onUnblock,
}: {
	title: string;
	testId: string;
	query: SectionQuery<T>;
	toRow: (item: T) => Row;
	rowErrors: Record<string, string>;
	onUnblock: (row: Row) => void;
}) {
	const defaults = useDefaultErrorText();
	const rows = (query.data?.pages.flatMap((p) => p.items) ?? []).map(toRow);
	const noteOf = (row: Row) =>
		[row.note, row.blockedAt ? formatTaipeiDate(row.blockedAt) : ""]
			.filter(Boolean)
			.join("．");
	if (query.isPending) return null;
	if (rows.length === 0 && !query.hasNextPage && !query.isError) return null;
	return (
		<section className="flex flex-col gap-3" data-testid={testId}>
			<h2 className="m-0 text-sm font-semibold text-foreground-soft">
				{title}
			</h2>
			{query.isError && rows.length === 0 ? (
				<div className="flex flex-col items-start gap-2">
					<p className="m-0 text-sm font-semibold text-danger-700" role="alert">
						{errorText(query.error, defaults)}
					</p>
					<Button
						variant="bordered"
						color="default"
						size="sm"
						onClick={() => void query.refetch()}
					>
						重試
					</Button>
				</div>
			) : (
				<ul className="m-0 flex list-none flex-col gap-3 p-0">
					{rows.map((row) => (
						<li
							key={row.id}
							className="flex flex-col gap-2 rounded-hkui-lg border border-divider bg-content1 p-4"
						>
							<div className="flex items-center gap-3">
								<Avatar
									entity="buyer"
									src={row.avatarUrl}
									name={row.name}
									className="h-9 w-9 rounded-full object-cover"
								/>
								<div className="flex min-w-0 flex-1 flex-col">
									<strong className="truncate text-base font-medium text-hk-heading">
										{row.name}
									</strong>
									{noteOf(row) ? (
										<small className="text-xs text-foreground-soft">
											{noteOf(row)}
										</small>
									) : null}
								</div>
								<Button
									variant="bordered"
									color="default"
									size="sm"
									onClick={() => onUnblock(row)}
								>
									解除封鎖
								</Button>
							</div>
							{rowErrors[row.id] ? (
								<p
									className="m-0 text-sm font-semibold text-danger-700"
									role="alert"
								>
									{rowErrors[row.id]}
								</p>
							) : null}
						</li>
					))}
				</ul>
			)}
			{query.isError && rows.length > 0 ? (
				<p className="m-0 text-sm font-semibold text-danger-700" role="alert">
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
						查看更多
					</Button>
				</div>
			) : null}
		</section>
	);
}

/** 封鎖名單頁（`/account/blocked`）：買家封鎖的店家與買家兩段，各自分頁，可解除封鎖。與 buyer-app 的封鎖名單對等。 */
export default function BlockedListClient() {
	const defaults = useDefaultErrorText();
	const identity = useQueryIdentity();
	const queryClient = useQueryClient();
	const storeCardText = useStoreCardText();
	const ut = useUiText();
	const storesKey = queryKeys.buyer.blockedStores(identity);
	const buyersKey = queryKeys.buyer.blockedBuyers(identity);
	const stores = useSection<BlockedStore>(storesKey, listBlockedStores);
	const buyers = useSection<BlockedBuyer>(buyersKey, listBlockedBuyers);

	const [confirming, setConfirming] = React.useState<Target | null>(null);
	const [rowErrors, setRowErrors] = React.useState<Record<string, string>>({});
	const unblock = useMutation({
		mutationFn: (t: Target) =>
			t.kind === "store" ? unblockStore(t.row.id) : unblockBuyer(t.row.id),
		onSuccess: (_r, t) => {
			if (t.kind === "store") {
				queryClient.setQueryData<
					InfiniteData<BlockedPage<BlockedStore>, string>
				>(storesKey, (data) => dropRow(data, (s) => s.storeId !== t.row.id));
				void queryClient.invalidateQueries({ queryKey: storesKey });
			} else {
				queryClient.setQueryData<
					InfiniteData<BlockedPage<BlockedBuyer>, string>
				>(buyersKey, (data) => dropRow(data, (b) => b.buyerId !== t.row.id));
				void queryClient.invalidateQueries({ queryKey: buyersKey });
			}
		},
	});

	const unblocking = React.useRef(false);
	const confirmUnblock = async () => {
		const target = confirming;
		if (!target || unblocking.current) return;
		unblocking.current = true;
		setRowErrors(({ [target.row.id]: _cleared, ...rest }) => rest);
		try {
			await unblock.mutateAsync(target);
		} catch (e) {
			setRowErrors((current) => ({
				...current,
				[target.row.id]: errorText(e, defaults),
			}));
		} finally {
			unblocking.current = false;
			setConfirming(null);
		}
	};

	const storeRow = (s: BlockedStore): Row => {
		const view = storeCardView(
			s.store ?? create(StoreCardSchema, {}),
			storeCardText,
		);
		return {
			id: s.storeId,
			name: view?.name ?? storeCardText.closedStore,
			note: view?.labelText,
			avatarUrl: view?.logoUrl || undefined,
			blockedAt: isoOf(s.createTime),
		};
	};
	const buyerRow = (b: BlockedBuyer): Row => {
		const name = b.displayName.trim();
		const gone = b.deleted || name === "";
		return {
			id: b.buyerId,
			name: gone ? ut(buyerProfileDeletedName()) : name,
			avatarUrl: !gone ? imageUri(b.avatar, "avatar") : undefined,
			blockedAt: isoOf(b.createTime),
		};
	};
	const bothPending = stores.isPending && buyers.isPending;
	const bothFailed =
		stores.isError && buyers.isError && !stores.data && !buyers.data;
	const hasContent = (q: SectionQuery<unknown>) =>
		(q.data?.pages.some((p) => p.items.length > 0) ?? false) ||
		q.hasNextPage ||
		q.isError;
	const empty =
		!stores.isPending &&
		!buyers.isPending &&
		!hasContent(stores) &&
		!hasContent(buyers);
	const retryAll = () => {
		void stores.refetch();
		void buyers.refetch();
	};

	return (
		<SellerPageShell title="封鎖名單">
			{bothPending ? (
				<p className="text-sm text-foreground-soft">載入中...</p>
			) : bothFailed ? (
				<div className="flex flex-col items-start gap-2">
					<p className="m-0 text-sm font-semibold text-danger-700" role="alert">
						{errorText(stores.error, defaults)}
					</p>
					<Button variant="bordered" color="default" onClick={retryAll}>
						重試
					</Button>
				</div>
			) : empty ? (
				<EmptyStatePanel
					icon="search"
					title="尚無封鎖的使用者"
					description="你封鎖的店家與買家會顯示在這裡。"
				/>
			) : (
				<div className="flex flex-col gap-[var(--hkui-space-6)]">
					<Section
						title="店家"
						testId="blocked-stores"
						query={stores}
						toRow={storeRow}
						rowErrors={rowErrors}
						onUnblock={(row) => setConfirming({ kind: "store", row })}
					/>
					<Section
						title="買家"
						testId="blocked-buyers"
						query={buyers}
						toRow={buyerRow}
						rowErrors={rowErrors}
						onUnblock={(row) => setConfirming({ kind: "buyer", row })}
					/>
				</div>
			)}
			<Dialog
				open={confirming !== null}
				onClose={() => {
					if (!unblock.isPending) setConfirming(null);
				}}
				size="sm"
				title={confirming ? `解除封鎖「${confirming.row.name}」？` : ""}
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
					解除後，你與對方的貼文、討論區與訊息將恢復互通。
				</p>
			</Dialog>
		</SellerPageShell>
	);
}
