"use client";

import { useInfiniteQuery } from "@connectrpc/connect-query";
import {
	SellerAdminService,
	type StoreListItem,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/seller_pb";
import { StoreStatus } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { skipToken } from "@tanstack/react-query";
import { useEffect, useMemo, useState } from "react";
import { Button } from "@/components/UI";
import { connectErrorText } from "@/lib/error-text";
import { REMOVE_TARGET_TEXT, type TargetRowForm } from "../authority-actions";

const STORE_PAGE_SIZE = 100;

/** 帳號在某個狀態（沒給＝預設的營運中與等待刪除）的店家，把所有頁讀完（有下一頁就繼續讀），不會因為超過一頁而悄悄少掉選項。 */
function useAllStores(accountId: string, status?: StoreStatus) {
	const query = useInfiniteQuery(
		SellerAdminService.method.listStores,
		accountId
			? {
					accountId,
					...(status === undefined ? {} : { status }),
					pageSize: STORE_PAGE_SIZE,
					pageToken: "",
				}
			: skipToken,
		{
			pageParamKey: "pageToken",
			getNextPageParam: (res) => res.nextPageToken || undefined,
		},
	);
	const { hasNextPage, isFetchingNextPage, isError, fetchNextPage } = query;
	useEffect(() => {
		if (hasNextPage && !isFetchingNextPage && !isError) void fetchNextPage();
	}, [hasNextPage, isFetchingNextPage, isError, fetchNextPage]);
	return query;
}

/** 帳號的店家（營運中與等待刪除的，加上已關閉的）：已關閉的店家照樣可以被來函指名。 */
function useAccountStores(accountId: string) {
	const current = useAllStores(accountId);
	const closed = useAllStores(accountId, StoreStatus.CLOSED);
	const currentPages = current.data?.pages;
	const closedPages = closed.data?.pages;
	const stores = useMemo(() => {
		const seen = new Set<string>();
		const out: StoreListItem[] = [];
		for (const item of [
			...(currentPages ?? []).flatMap((page) => page.stores),
			...(closedPages ?? []).flatMap((page) => page.stores),
		]) {
			const id = item.store?.id ?? "";
			if (id !== "" && !seen.has(id)) {
				seen.add(id);
				out.push(item);
			}
		}
		return out;
	}, [currentPages, closedPages]);
	return {
		stores,
		loading: current.isLoading || closed.isLoading,
		error: current.error ?? closed.error,
		retry: () => {
			void current.refetch();
			void closed.refetch();
		},
	};
}

interface TargetRowProps {
	index: number;
	row: TargetRowForm;
	disabled: boolean;
	/** 列數大於 1 才能移除。 */
	removable: boolean;
	onChange: (patch: Partial<TargetRowForm>) => void;
	onRemove: () => void;
}

/** 登記頁的一個指名對象（L11）：帳號 ID、店家（選填，讀這個帳號的店家）、商品 ID（選填，逗號分隔）。 */
export default function TargetRow({
	index,
	row,
	disabled,
	removable,
	onChange,
	onRemove,
}: TargetRowProps) {
	// 帳號 ID 輸入完（離開欄位）才去讀它的店家，不是每打一個字讀一次。
	const [committed, setCommitted] = useState("");
	const stores = useAccountStores(committed);
	const id = `authority-target-${index}`;
	return (
		<div
			style={{
				border: "1px solid var(--admin-border)",
				borderRadius: 8,
				padding: 12,
				marginBottom: 12,
			}}
		>
			<label className="form-label" htmlFor={`${id}-account`}>
				賣家帳號 ID <span className="required">*</span>
			</label>
			<input
				id={`${id}-account`}
				type="text"
				className="form-control"
				value={row.accountId}
				disabled={disabled}
				onChange={(e) => onChange({ accountId: e.target.value, storeId: "" })}
				onBlur={() => setCommitted(row.accountId.trim())}
			/>
			<label
				className="form-label"
				htmlFor={`${id}-store`}
				style={{ marginTop: 12 }}
			>
				店家（選填，不選＝這個帳號的全部店家）
			</label>
			<select
				id={`${id}-store`}
				className="form-select"
				value={row.storeId}
				disabled={
					disabled || committed === "" || committed !== row.accountId.trim()
				}
				onChange={(e) => onChange({ storeId: e.target.value })}
			>
				<option value="">請選擇</option>
				{stores.stores.map((item) => (
					<option key={item.store?.id} value={item.store?.id}>
						{item.store?.storeName || item.store?.id}
					</option>
				))}
			</select>
			{stores.error != null && (
				<div role="alert" style={{ marginTop: 8 }}>
					{connectErrorText(stores.error, "店家資料讀取失敗")}{" "}
					<Button variant="outline" onClick={stores.retry}>
						重試
					</Button>
				</div>
			)}
			<label
				className="form-label"
				htmlFor={`${id}-products`}
				style={{ marginTop: 12 }}
			>
				商品 ID（選填，以逗號分隔）
			</label>
			<input
				id={`${id}-products`}
				type="text"
				className="form-control"
				value={row.productIds}
				disabled={disabled}
				onChange={(e) => onChange({ productIds: e.target.value })}
			/>
			{removable && (
				<div style={{ marginTop: 12 }}>
					<Button
						variant="outline"
						onClick={onRemove}
						disabled={disabled}
						aria-describedby={`${id}-account`}
					>
						{REMOVE_TARGET_TEXT}
					</Button>
				</div>
			)}
		</div>
	);
}
