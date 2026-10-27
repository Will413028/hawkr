"use client";

import type {
	KYCApplication,
	KYCFlow,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useAuth } from "@/lib/auth/context";
import { queryKeys } from "@/lib/query/keys";
import { fetchMyKyc } from "@/lib/seller/kyc";
import { flowOf } from "@/lib/seller/kyc-flow";
import { isStoreMissing, type MyStore } from "@/lib/seller/store";
import { useSellerDataScope } from "@/lib/seller-data/scope";
import { useQueryIdentity } from "./useQueryIdentity";

interface UseSellerSelfResult {
	data: MyStore | null;
	loading: boolean;
	/** Raw fetch error, kept as the Error object (not a string) so callers can
	 * introspect it — `isStoreMissing` reads it to mean "no store yet". */
	error: Error | null;
	refresh: () => Promise<void>;
}

/**
 * `GetMyStore` as a single TanStack-managed server-state. The navbar, seller-center
 * shell, `useCertGate` and each seller page all mount this; TanStack collapses the
 * concurrent reads into one identity-scoped request + shared cache. After a
 * profile / cert / image mutation, invalidate `queryKeys.seller.self` (see
 * `useInvalidateSellerSelf`) so every consumer refetches.
 */
export function useSellerSelf(): UseSellerSelfResult {
	const { isLoggedIn, hasSeller } = useAuth();
	const scope = useSellerDataScope();
	const query = useQuery({
		queryKey: scope.keys.storefront(),
		queryFn: () => scope.repos.storefront.fetchMyStore(),
		// Fire whenever the account carries the seller role — matches the old
		// `useSellerIdentity` gate (the navbar reads this even while a dual-role
		// user browses as a buyer) and holds on every seller-center page.
		// 代行下這條由 scope 換成 capability，且工作區的 chrome 不掛這支 hook：側欄
		// 每頁都渲染，讓它讀店面等於每次換頁都多落一列 STOREFRONT_READ 稽核（§6.5.3
		// 那四個 READ action 的用途是「有人試圖跨店讀取」的安全訊號，會被灌成噪音）。
		enabled: scope.canRead("storefront", isLoggedIn && hasSeller),
		staleTime: 30_000,
		// 還沒開店（not_found）是預期狀態，不是故障：沒有這個 meta，每個還沒開店的
		// 帳號都會往 Sentry 灌一筆噪音。
		meta: { isExpectedError: isStoreMissing },
	});
	return {
		data: query.data ?? null,
		loading: query.isLoading,
		error: query.error,
		refresh: async () => {
			await query.refetch();
		},
	};
}

/**
 * Returns a callback that invalidates the store and KYC-flow queries so all
 * consumers refetch — call it after a profile / cert / logo / cover mutation.
 */
export function useInvalidateSellerSelf(): () => Promise<void> {
	const queryClient = useQueryClient();
	const scope = useSellerDataScope();
	const identity = useQueryIdentity();
	return async () => {
		await Promise.all([
			queryClient.invalidateQueries({ queryKey: scope.keys.storefront() }),
			queryClient.invalidateQueries({
				queryKey: queryKeys.seller.kyc(identity),
			}),
		]);
	};
}

interface UseMyKycResult {
	/** 目前的申請；沒有申請（或已取消）是 null。 */
	application: KYCApplication | null;
	loading: boolean;
	error: Error | null;
}

function useMyKycQuery<T>(
	select: (app: KYCApplication | null) => T,
	enabledOption: boolean | undefined,
) {
	const { isLoggedIn, hasSeller } = useAuth();
	const scope = useSellerDataScope();
	const identity = useQueryIdentity();
	return useQuery({
		queryKey: queryKeys.seller.kyc(identity),
		queryFn: () => fetchMyKyc(),
		select,
		enabled:
			scope.mode === "seller" &&
			isLoggedIn &&
			hasSeller &&
			(enabledOption ?? true),
		staleTime: 30_000,
		// PayUni 的審核結果由伺服器的背景程序與 PayUni 通知更新，沒有賣家端的刷新動作：回到分頁時重新讀一次。
		refetchOnWindowFocus: true,
	});
}

/**
 * KYC 申請（`GetMyKYC`）。停權與等待刪除的店家照樣讀得到（CAP_ACCOUNT_SELF_READ）；
 * 代行工作區沒有這個資料來源，不發查詢。
 */
export function useMyKyc(options?: { enabled?: boolean }): UseMyKycResult {
	const query = useMyKycQuery((app) => app, options?.enabled);
	return {
		application: query.data ?? null,
		loading: query.isLoading,
		error: query.error,
	};
}

interface UseMyKycFlowResult {
	flow: KYCFlow | null;
	loading: boolean;
	error: Error | null;
}

/** 申請的流程狀態；沒有申請是 NOT_STARTED。與 `useMyKyc` 共用同一份快取。 */
export function useMyKycFlow(options?: {
	enabled?: boolean;
}): UseMyKycFlowResult {
	const query = useMyKycQuery(flowOf, options?.enabled);
	return {
		flow: query.data ?? null,
		loading: query.isLoading,
		error: query.error,
	};
}

/** 把伺服器剛回的申請寫進快取（存檔、送出、取消的回應就是最新的申請）。 */
export function useSetMyKyc(): (application: KYCApplication | null) => void {
	const queryClient = useQueryClient();
	const identity = useQueryIdentity();
	return (application) => {
		queryClient.setQueryData(queryKeys.seller.kyc(identity), application);
	};
}
