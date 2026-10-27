"use client";

import { keepPreviousData, useMutation, useQuery } from "@tanstack/react-query";
import * as React from "react";
import { queryKeys } from "@/lib/query/keys";
import { useDelegatedSession } from "../state/DelegatedSessionContext";
import {
	canRenderDelegatedWorkspace,
	DELEGATED_PENDING_IDENTITY,
	type DelegatedIdentity,
	delegatedIdentityOf,
} from "../state/session";
import type { DelegatedOrderSearchFilters } from "./filters";
import {
	createDelegatedOrderRepo,
	type DelegatedOrderDetail,
	type DelegatedOrderListResult,
	type DelegatedOrderRepo,
	type DelegatedRecipientContactResult,
	fetchDelegatedOrderStatusTotals,
} from "./repo";

/**
 * 代行訂單面的查詢層（56 號 §6.1.10、定案第 4 條）。
 *
 * ⚠️ 快取鍵一律走 `queryKeys.delegated.*`，identity 取自 `GET current`。**不得**用
 * `useQueryIdentity()`：那份 identity 由 `useAuth()` 來，而代行的瀏覽器在 www 上沒有
 * 登入態 ⇒ 兩家店會共用同一個快取槽，第二家直接看到第一家的訂單。
 *
 * ⚠️ 這裡沒有任何 `status_counts` 之類的彙總查詢：代行訂單面只有列表與明細兩支唯讀
 * 端點，`getSellerOrderStatusCounts` 明令不動。KPI 一律由「帶 status 的列表 `total`」
 * 呈現。
 */

/** mockup §4.8.6 的「顯示 N 筆」。10 是圖上的預設值。 */
export const DELEGATED_ORDER_PAGE_SIZES = [10, 20, 50] as const;
export const DELEGATED_ORDER_DEFAULT_PAGE_SIZE = 10;

interface DelegatedOrderQueryBase {
	identity: DelegatedIdentity;
	repo: DelegatedOrderRepo | null;
	/** capability ＋ 工作階段狀態都成立才發請求。 */
	enabled: boolean;
}

/**
 * 訂單面的查詢前提。
 *
 * `orders.read` 不足時**一個請求都不發** —— 打一支必然 403 的端點來探測，會在
 * `admin_operation_audit_logs` 製造一批 `*_READ` ＋ `FAILED` 的列，把「有人試圖跨店
 * 讀取」這個安全訊號淹掉，而稽核表看起來完全正常。
 */
function useDelegatedOrderQueryBase(): DelegatedOrderQueryBase {
	const { summary, state, hasCapability } = useDelegatedSession();
	const storeId = summary?.target.storeId ?? null;
	const repo = React.useMemo(
		() => (storeId ? createDelegatedOrderRepo(storeId) : null),
		[storeId],
	);
	return {
		identity: summary
			? delegatedIdentityOf(summary)
			: DELEGATED_PENDING_IDENTITY,
		repo,
		enabled:
			repo !== null &&
			canRenderDelegatedWorkspace(state) &&
			hasCapability("orders.read"),
	};
}

/** 沒有 repo 就不會被呼叫；真的被呼叫代表 `enabled` 的判斷與這裡脫鉤了。 */
function requireRepo(repo: DelegatedOrderRepo | null): DelegatedOrderRepo {
	if (!repo) throw new Error("代行訂單資料層尚未就緒（缺少目標店家）");
	return repo;
}

export function useDelegatedOrdersReadable(): boolean {
	return useDelegatedOrderQueryBase().enabled;
}

export interface UseDelegatedOrderListArgs extends DelegatedOrderSearchFilters {
	page: number;
	pageSize: number;
	/** CodeMeta `OrderStatus` 數字碼；空字串＝全部。 */
	status: string;
}

export interface UseDelegatedOrderListResult {
	data: DelegatedOrderListResult | undefined;
	loading: boolean;
	refreshing: boolean;
	error: Error | null;
	refresh: () => Promise<void>;
}

export function useDelegatedOrderList({
	page,
	pageSize,
	status,
	keyword,
	dateFrom,
	dateTo,
}: UseDelegatedOrderListArgs): UseDelegatedOrderListResult {
	const { identity, repo, enabled } = useDelegatedOrderQueryBase();
	const query = useQuery({
		// `kind` 這個判別字讓列表與 KPI 兩種查詢即使參數巧合相同也不會共用同一槽。
		queryKey: queryKeys.delegated.orders(identity, {
			kind: "list",
			page,
			pageSize,
			status,
			keyword,
			dateFrom,
			dateTo,
		}),
		queryFn: ({ signal }) =>
			requireRepo(repo).list({
				page,
				pageSize,
				status,
				keyword,
				dateFrom,
				dateTo,
				signal,
			}),
		enabled,
		// 翻頁時保留上一頁的列，而不是閃一次空清單。
		placeholderData: keepPreviousData,
	});

	return {
		data: enabled ? query.data : undefined,
		loading: query.isLoading,
		refreshing: query.isFetching,
		error: query.error instanceof Error ? query.error : null,
		refresh: async () => {
			await query.refetch();
		},
	};
}

/**
 * KPI 卡的五個數字。
 *
 * 每一張卡 ＝ 一次 `page_size=1` 的帶狀態列表，只讀它的 `total`。這是設計書明列的
 * 唯一合法作法（B15）：放寬一支統計 RPC 是本案最容易被靜默鑿開的洞 —— 不碰
 * registry、不觸發 boot 斷言、不需新 capability，**CI 全綠而攻擊面已擴大**。
 *
 * ⚠️ `filters` 是**必填**，而且必須是清單當下用的那一份：卡片與清單各自算一遍，
 * 條件不同步時兩個數字會在同一個畫面上互相矛盾（搜尋「番茄」剩 3 筆、卡片仍寫 81），
 * 而操作者無從判斷哪一個才是真的。必填讓漏接變成編譯錯誤，不是視覺 bug。
 */
export function useDelegatedOrderStatusTotals(
	statuses: readonly string[],
	filters: DelegatedOrderSearchFilters,
): {
	totals: Record<string, number>;
	loading: boolean;
} {
	const { identity, repo, enabled } = useDelegatedOrderQueryBase();
	const query = useQuery({
		queryKey: queryKeys.delegated.orders(identity, {
			kind: "status-totals",
			statuses,
			keyword: filters.keyword,
			dateFrom: filters.dateFrom,
			dateTo: filters.dateTo,
		}),
		queryFn: ({ signal }) =>
			fetchDelegatedOrderStatusTotals(
				requireRepo(repo),
				statuses,
				filters,
				signal,
			),
		enabled,
		placeholderData: keepPreviousData,
	});

	return {
		totals: enabled ? (query.data ?? {}) : {},
		loading: query.isLoading,
	};
}

export interface UseDelegatedOrderDetailResult {
	order: DelegatedOrderDetail | null;
	loading: boolean;
	error: Error | null;
}

export function useDelegatedOrderDetail(
	orderNo: string | null,
): UseDelegatedOrderDetailResult {
	const { identity, repo, enabled } = useDelegatedOrderQueryBase();
	const query = useQuery({
		queryKey: queryKeys.delegated.order(identity, orderNo ?? "", {
			kind: "detail",
		}),
		queryFn: ({ signal }) =>
			requireRepo(repo).detail(orderNo ?? "", { signal }),
		enabled: enabled && Boolean(orderNo),
	});

	return {
		order: query.data ?? null,
		loading: query.isLoading,
		error: query.error instanceof Error ? query.error : null,
	};
}

export interface UseRevealRecipientContactResult {
	reveal: (args: { orderNo: string; operationReason: string }) => void;
	revealing: boolean;
	error: Error | null;
	/** 揭露結果**只活在記憶體**，不進 react-query 快取（見下方註解）。 */
	contact: DelegatedRecipientContactResult | null;
	reset: () => void;
}

/**
 * 收件資訊揭露。
 *
 * 三個刻意的設計：
 * 1. **用 mutation 不用 query**：同一筆揭露兩次是**兩個獨立的稽核事實**，後端的
 *    `idempotency_key` 為 `NULL`、刻意不去重。做成 query 會讓第二次點擊命中快取
 *    而完全不落稽核 —— 稽核表上少掉的那一列不會有任何東西提醒你。
 * 2. **結果不寫進 react-query**：那是全平台唯一的未遮罩收件資訊，放進快取等於讓它
 *    活到工作階段結束之後（`removeQueries` 只在終態／結束時跑）。
 * 3. **綁 `registerWriteController`**：它是 `risk_level=high` 的稽核動作，工作階段
 *    進入結束流程時必須跟寫入一起被中止。
 */
export function useRevealRecipientContact(): UseRevealRecipientContactResult {
	const { repo, enabled } = useDelegatedOrderQueryBase();
	const { registerWriteController } = useDelegatedSession();
	const [contact, setContact] =
		React.useState<DelegatedRecipientContactResult | null>(null);

	const mutation = useMutation({
		mutationFn: async ({
			orderNo,
			operationReason,
		}: {
			orderNo: string;
			operationReason: string;
		}) => {
			if (!enabled) {
				throw new Error("本次代行沒有訂單讀取權限，無法查看完整收件資訊。");
			}
			const controller = new AbortController();
			const unregister = registerWriteController(controller);
			try {
				return await requireRepo(repo).revealRecipientContact(
					orderNo,
					operationReason,
					{ signal: controller.signal },
				);
			} finally {
				unregister();
			}
		},
		onSuccess: (result) => setContact(result),
	});

	const reset = React.useCallback(() => {
		setContact(null);
		mutation.reset();
	}, [mutation.reset]);

	return {
		reveal: (args) => mutation.mutate(args),
		revealing: mutation.isPending,
		error: mutation.error instanceof Error ? mutation.error : null,
		contact,
		reset,
	};
}
