import {
	useQuery as useConnectQuery,
	useTransport,
} from "@connectrpc/connect-query";
import { IdentityAdminService } from "@hawkr/api/gen/hawkr/admin/identity/v1alpha1/identity_pb";
import { SellerAdminService } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/seller_pb";
import { SellerPublicService } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { skipToken, useQuery } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import {
	kycApplicationOf,
	kycApplicationQueryOptions,
	payUniOf,
	payUniPollInterval,
	ratingText,
	registrationPollInterval,
} from "./kyc-application";
import { accountEmailOf } from "./store-detail";

// 商家詳情頁的資料來源（SellerAdminService.GetStore、GetKYCApplication、GetPayUniRegistration；IdentityAdminService.GetAccount；
// 公開的 SellerPublicService.GetStore 的評分統計）。查詢都經 connect-query
// 的 transport（K1），頁面只讀結果。

/** 店家詳情（GetStore）。已關閉的店照樣回傳；生效中的措施（active_measures）與店家是同一次讀取。storeId 為空時不發請求。 */
export function useStoreDetail(storeId: string) {
	return useConnectQuery(
		SellerAdminService.method.getStore,
		storeId ? { id: storeId } : skipToken,
	);
}

/**
 * 登入信箱（K6）：GetAccount 讀店家的帳號。頁首與刪帳號確認框共用這個查詢。不存在或已刪除回 not_found，
 * 畫面說「找不到登入帳號」；成功卻沒有帳號是讀取錯誤（可重試）；帳號 ID 為空時不發請求。狀態的判斷在 accountEmailOf。
 */
export function useAccountEmail(accountId: string | undefined) {
	const query = useConnectQuery(
		IdentityAdminService.method.getAccount,
		accountId ? { accountId } : skipToken,
	);
	return { value: accountEmailOf(query), retry: () => void query.refetch() };
}

// ---------------------------------------------------------------------------
// 商家認證（KYC）與 PayUni（第 2b 刀）

/**
 * 商家認證的申請內容（GetKYCApplication）。申請 ID 來自 GetStore.kyc.application_id；沒有送出過的申請時 ID 為空，不發請求。
 * 登記資料還在查詢（pending_registration_check）時，依伺服器給的期限（deadline_time）以退避輪詢到終態或期限（registrationPollInterval）；
 * 頁籤隱藏時不輪詢（TanStack 預設）。讀取狀態的判斷在 kycApplicationOf。
 */
export function useKycApplication(applicationId: string | undefined) {
	const transport = useTransport();
	const query = useQuery({
		...kycApplicationQueryOptions(transport, applicationId),
		refetchInterval: (q) =>
			registrationPollInterval(
				q.state.data?.application?.pendingRegistrationCheck,
				new Date(),
			),
	});
	return { query, value: kycApplicationOf(applicationId, query) };
}

/**
 * PayUni 子商店註冊（GetPayUniRegistration）。第一次核准前是 not_found（尚未註冊）。店家 ID 為空時不發請求。
 * 綁定中（bind_status = PENDING）時以退避輪詢到終態（伺服器的 payuni_sweep 保證會到 BOUND 或 FAILED，所以沒有次數上限，間隔有上限）。
 */
export function usePayUniRegistration(storeId: string | undefined) {
	const query = useConnectQuery(
		SellerAdminService.method.getPayUniRegistration,
		storeId ? { storeId } : skipToken,
		{
			refetchInterval: (q) =>
				payUniPollInterval(q.state.data?.registration, new Date()),
		},
	);
	return { query, value: payUniOf(query) };
}

/**
 * 店面的評分統計：與買家看到的店面同一個來源（公開的 GetStore 以 store_code 讀，StoreStats.rating_average）。
 * 沒有 store_code、或呼叫端說不需要（enabled 為 false）時不發請求；已關閉、未公開、讀不到都沒有值（那張卡不渲染）。
 */
export function useStoreRating(
	storeCode: string | undefined,
	enabled: boolean,
) {
	const query = useConnectQuery(
		SellerPublicService.method.getStore,
		enabled && storeCode ? { storeCode } : skipToken,
	);
	return ratingText(query.data?.stats);
}

/**
 * 到 target 那一刻重新渲染一次的現在時間：target 之前回傳建立時的時間，之後回傳 target 之後的時間（用來判斷「進行中的查詢是否已超過
 * 期限」）。target 為空時不排計時器。
 */
export function useNowUntil(target: Date | undefined): Date {
	const [now, setNow] = useState(() => new Date());
	const targetMs = target?.getTime();
	useEffect(() => {
		if (targetMs === undefined) return;
		const remaining = targetMs - Date.now();
		if (remaining <= 0) {
			setNow(new Date());
			return;
		}
		const timer = setTimeout(() => setNow(new Date()), remaining + 50);
		return () => clearTimeout(timer);
	}, [targetMs]);
	return now;
}
