"use client";

import { useQuery } from "@tanstack/react-query";
import { useAuth } from "@/lib/auth/context";
import { queryKeys } from "@/lib/query/keys";
import {
	fetchSellerDashboard,
	type SellerDashboard,
} from "@/lib/seller/dashboard";
import { useQueryIdentity } from "./useQueryIdentity";

interface UseSellerDashboardResult {
	data: SellerDashboard | null;
	loading: boolean;
	/** 第一次讀取失敗（還沒有資料）。背景重讀失敗會保留舊資料，不算。 */
	loadFailed: boolean;
	error: string | null;
	refresh: () => Promise<void>;
}

/** 賣家總覽 KPI（`GetMyDashboard`）的單一 server-state 來源。
 *  SellerOrdersPage / DashboardPage / SellerCenterShell /
 *  WalletClient 全部走這個 hook，共用同一條 identity-scoped query——TanStack
 *  自動去重、共用快取，任一處 refresh() 都會即時更新所有消費者的畫面。
 *  queryFn 是無快取的純 fetcher（repo 層 dedupe 已移除）：新鮮度由 staleTime
 *  控、身分隔離由 queryKey 的 identity 控，不需要也沒有 force 旁路。
 *  options.enabled：在預設「已登入且 activeRole===seller」之外再加自訂 gate
 *  （例：WalletClient 認證通過才抓）。 */
export function useSellerDashboard(options?: {
	enabled?: boolean;
}): UseSellerDashboardResult {
	const { isLoggedIn, activeRole } = useAuth();
	const identity = useQueryIdentity();
	const enabled =
		isLoggedIn && activeRole === "seller" && (options?.enabled ?? true);
	const query = useQuery({
		queryKey: queryKeys.seller.dashboard(identity),
		queryFn: () => fetchSellerDashboard(),
		enabled,
		staleTime: 30_000,
		// 新鮮度靠推播（`useSellerWorkspaceRealtime` 讓快取失效）加回到畫面時重抓，不做固定間隔輪詢；
		// 本 repo 的預設是不在聚焦時重抓，所以在這裡明設。30 秒內讀過的就跳過（staleTime）。
		refetchOnWindowFocus: true,
	});
	return {
		data: query.data ?? null,
		loading: query.isLoading,
		loadFailed: query.isError && query.data === undefined,
		error: query.error instanceof Error ? query.error.message : null,
		refresh: async () => {
			await query.refetch();
		},
	};
}
