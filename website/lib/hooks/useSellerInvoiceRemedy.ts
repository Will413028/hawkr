"use client";

/**
 * 賣家面發票的 **停權放行類** 查詢（58A §B-2.2 (4)(c)）。
 *
 * 🔴 本檔每一支 hook 的 `enabled` **都不得含 `!isSuspended`**。四支端點的 capability
 * 都在 `gateway/internal/authz/policy.go` 的 `suspendedWhitelist` 內：
 *
 *   端點 17   稅務身分            `CapInvoiceTaxRemedy`
 *   端點 62R  本訂單的自行開立案件  `CapInvoiceTaxRemedy`
 *   端點 62   回報自行開立的發票號碼 `CapInvoiceTaxRemedy`（**寫入**，一樣放行）
 *   端點 34   買家申訴列表         `CapOrderFulfill`
 *   端點 35   申訴詳情            `CapOrderFulfill`
 *
 * 前兩支是救濟管道（補齊稅務件與回報自行開立的發票，正是解除停權的路徑本身）；
 * 後兩支是「處理既有訂單的義務」——申訴有 3 工作天 SLA、逾期即計違規、90 天內
 * 3 次即停權 ⇒ 鎖住等於「因為停權所以無法回覆，因為無法回覆所以繼續累積違規」
 * （58 §7.3.5 的死結）。在前端加上 `!isSuspended` 等於把後端的放行抵銷掉，而所有
 * 查詢都回 200、沒有任何測試會叫（§B-2.0 T04／T05）。
 *
 * 🔴 62R 的 `enabled` **只看 `orderNo`**，不得掛在端點 30／31 的成功狀態上——那兩支
 * 對停權賣家 BLOCK，掛上去就是「停權賣家永遠看不到 S07 ⑦」。未停權時兩種寫法看起來
 * 一模一樣，所以這裡的鑑別力只能靠停權帳號驗（S07 驗收第 4 項）。
 */

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useAuth } from "@/lib/auth/context";
import { isInvoiceBizCode } from "@/lib/invoices/errorCopy";
import {
	fetchSellerInvoiceComplaint,
	fetchSellerTaxProfile,
	listOrderExternalIssuances,
	listSellerInvoiceComplaints,
	reportExternalIssuance,
	requestSellerTaxExtension,
	respondSellerInvoiceComplaint,
	SELLER_INVOICE_BIZ_CODE,
	type SellerComplaintAction,
	saveSellerTaxProfileDraft,
	submitSellerTaxProfile,
} from "@/lib/invoices/seller-repo";
import type {
	InvoiceJobInfo,
	SellerInvoiceComplaint,
	SellerPagedResult,
	SellerTaxIdentityInput,
	SellerTaxProfile,
} from "@/lib/invoices/seller-types";
import { queryKeys } from "@/lib/query/keys";
import { useQueryIdentity } from "./useQueryIdentity";

function isTaxProfileMissing(error: unknown): boolean {
	return isInvoiceBizCode(error, SELLER_INVOICE_BIZ_CODE.TAX_PROFILE_NOT_FOUND);
}

export interface UseSellerTaxProfileResult {
	profile: SellerTaxProfile | null;
	/**
	 * 這個賣家還沒有任何稅務身分資料（後端 404 ＋ `9320`）。
	 *
	 * 🔴 **這不是錯誤態**：新賣家天天都是這個狀態，畫面要顯示「尚未開始稅務身分
	 * 設定」而不是紅字。（另有一條路徑會給出同樣語意：端點 16／17 直接回
	 * `taxReviewStatus = "NOT_STARTED"`，兩者都要接得住。）
	 */
	notStarted: boolean;
	loading: boolean;
	/** 真正的失敗（5xx／網路）。`9320` 不走這裡。 */
	error: unknown;
	refresh: () => Promise<void>;
}

/** 端點 17（S02 唯一資料源）。停權賣家必須查得到——見檔頭。 */
export function useSellerTaxProfile(): UseSellerTaxProfileResult {
	const { isLoggedIn, activeRole } = useAuth();
	const identity = useQueryIdentity();
	const enabled = isLoggedIn && activeRole === "seller";

	const query = useQuery({
		queryKey: queryKeys.seller.invoiceTaxProfile(identity),
		queryFn: fetchSellerTaxProfile,
		enabled,
		// 9320 是「這個賣家還沒建過稅務身分」——新賣家的常態，上報它等於把正常
		// 狀態灌進 Sentry（形狀照 useOrderInvoice 對 9301 的處理）。
		meta: { isExpectedError: isTaxProfileMissing },
	});

	const notStarted = query.error != null && isTaxProfileMissing(query.error);

	return {
		profile: enabled ? (query.data ?? null) : null,
		notStarted,
		loading: enabled && query.isLoading,
		error: notStarted ? null : query.error,
		refresh: async () => {
			await query.refetch();
		},
	};
}

export interface UseOrderExternalIssuancesResult {
	records: InvoiceJobInfo[];
	loading: boolean;
	error: unknown;
	refresh: () => Promise<void>;
}

/**
 * 端點 62R（S07 ⑦ 的唯一資料源）。
 *
 * `enabled` 只看 `orderNo` 與登入態——見檔頭第二條紅線。
 */
export function useOrderExternalIssuances(
	orderNo: string,
): UseOrderExternalIssuancesResult {
	const { isLoggedIn, activeRole } = useAuth();
	const identity = useQueryIdentity();
	const enabled = isLoggedIn && activeRole === "seller" && orderNo !== "";

	const query = useQuery({
		queryKey: queryKeys.seller.orderExternalIssuances(identity, orderNo),
		queryFn: () => listOrderExternalIssuances(orderNo),
		enabled,
	});

	return {
		records: enabled ? (query.data?.records ?? []) : [],
		loading: enabled && query.isLoading,
		error: query.error,
		refresh: async () => {
			await query.refetch();
		},
	};
}

export interface UseSubmitSellerTaxProfileResult {
	submit: (acceptedTerms: boolean) => void;
	submitting: boolean;
	/** 上一次送出的失敗；成功後歸零。 */
	error: unknown;
}

/**
 * 端點 19：送出稅務資格審核（S02）。
 *
 * 🔴 `acceptedTerms` 由呼叫端傳，**不在這裡寫死 true**：寫死等於把「賣家有沒有
 * 勾」這件事從程式碼裡抹掉，而那正是後端 9325 要求的意思表示。條款簽署記錄會
 * 連同狀態一起落庫（裁決 C），所以這一格是有法律意義的。
 *
 * 成功後 `invalidateQueries` 兩把鑰匙：稅務身分本身（S02）與發票總覽（S01 的
 * `taxReviewStatus` 是同一份資料的投影，不失效會讓兩個畫面顯示不同狀態）。
 */
export function useSubmitSellerTaxProfile(
	expectedVersion: string,
): UseSubmitSellerTaxProfileResult {
	const identity = useQueryIdentity();
	const queryClient = useQueryClient();

	const mutation = useMutation({
		mutationFn: (acceptedTerms: boolean) =>
			submitSellerTaxProfile({ expectedVersion, acceptedTerms }),
		onSuccess: async () => {
			await Promise.all([
				queryClient.invalidateQueries({
					queryKey: queryKeys.seller.invoiceTaxProfile(identity),
				}),
				queryClient.invalidateQueries({
					queryKey: queryKeys.seller.invoiceOverview(identity),
				}),
			]);
		},
	});

	return {
		submit: (acceptedTerms: boolean) => mutation.mutate(acceptedTerms),
		submitting: mutation.isPending,
		error: mutation.error,
	};
}

export interface UseSaveTaxDraftResult {
	save: (args: {
		taxIdentity: SellerTaxIdentityInput;
		evidenceObjectKeys: string[];
		evidenceStagedTokens: string[];
	}) => void;
	saving: boolean;
	error: unknown;
	succeeded: boolean;
	reset: () => void;
}

/**
 * 端點 18（S02 表單）。**整條發票鏈的第一步**：只有這支會寫
 * `evidence_object_keys`，而端點 19 的守衛讀的正是那一欄。
 */
export function useSaveSellerTaxDraft(
	expectedVersion: string,
): UseSaveTaxDraftResult {
	const identity = useQueryIdentity();
	const queryClient = useQueryClient();

	const mutation = useMutation({
		mutationFn: (args: {
			taxIdentity: SellerTaxIdentityInput;
			evidenceObjectKeys: string[];
			evidenceStagedTokens: string[];
		}) => saveSellerTaxProfileDraft({ ...args, expectedVersion }),
		onSuccess: async () => {
			await Promise.all([
				queryClient.invalidateQueries({
					queryKey: queryKeys.seller.invoiceTaxProfile(identity),
				}),
				queryClient.invalidateQueries({
					queryKey: queryKeys.seller.invoiceOverview(identity),
				}),
			]);
		},
	});

	return {
		save: (args) => mutation.mutate(args),
		saving: mutation.isPending,
		error: mutation.error,
		succeeded: mutation.isSuccess,
		reset: () => mutation.reset(),
	};
}

export interface UseRequestExtensionResult {
	request: (reason: string) => void;
	requesting: boolean;
	error: unknown;
	succeeded: boolean;
}

/**
 * 端點 20。
 *
 * ⚠️ 能不能按看端點 17 的 `canRequestExtension`，不要自己算 `extensionCount`
 * ——後端的條件不只次數。
 */
export function useRequestSellerTaxExtension(
	expectedVersion: string,
): UseRequestExtensionResult {
	const identity = useQueryIdentity();
	const queryClient = useQueryClient();

	const mutation = useMutation({
		mutationFn: (reason: string) =>
			requestSellerTaxExtension({ reason, expectedVersion }),
		onSuccess: async () => {
			await Promise.all([
				queryClient.invalidateQueries({
					queryKey: queryKeys.seller.invoiceTaxProfile(identity),
				}),
				queryClient.invalidateQueries({
					queryKey: queryKeys.seller.invoiceOverview(identity),
				}),
			]);
		},
	});

	return {
		request: (reason) => mutation.mutate(reason),
		requesting: mutation.isPending,
		error: mutation.error,
		succeeded: mutation.isSuccess,
	};
}

export interface UseRespondComplaintResult {
	respond: (args: { action: SellerComplaintAction; note: string }) => void;
	responding: boolean;
	error: unknown;
	succeeded: boolean;
	reset: () => void;
}

/**
 * 端點 36（S10 ⑥）。回覆是**追加**語意——同一件案可以回覆多次，所以成功後不關閉
 * 整個區塊，只把 dialog 收起來並重新拉一次詳情。
 *
 * 🔴 `expectedVersion` 由呼叫端從 `complaint.version` 取，不得用 `responses.length`
 * 頂替：平台側也會追加事件，兩個數字會分岔。
 */
export function useRespondSellerInvoiceComplaint(
	complaintId: string,
	expectedVersion: string,
): UseRespondComplaintResult {
	const identity = useQueryIdentity();
	const queryClient = useQueryClient();

	const mutation = useMutation({
		mutationFn: (args: { action: SellerComplaintAction; note: string }) =>
			respondSellerInvoiceComplaint({ complaintId, expectedVersion, ...args }),
		onSuccess: async () => {
			// 詳情、列表、總覽三者都要失效：S09 的「待你處理」筆數與 S01 卡 3 的
			// 未結申訴數顯示的是同一批案件。
			await Promise.all([
				queryClient.invalidateQueries({
					queryKey: queryKeys.seller.invoiceComplaint(identity, complaintId),
				}),
				queryClient.invalidateQueries({
					queryKey: queryKeys.seller
						.invoiceComplaints(identity)
						.slice(0, -1) as unknown as readonly string[],
				}),
				queryClient.invalidateQueries({
					queryKey: queryKeys.seller.invoiceOverview(identity),
				}),
			]);
		},
	});

	return {
		respond: (args) => mutation.mutate(args),
		responding: mutation.isPending,
		error: mutation.error,
		succeeded: mutation.isSuccess,
		reset: () => mutation.reset(),
	};
}

export interface UseSellerInvoiceComplaintsResult {
	page: SellerPagedResult<SellerInvoiceComplaint> | null;
	loading: boolean;
	error: unknown;
	refresh: () => Promise<void>;
}

/**
 * 端點 34（S09 唯一資料源）。停權賣家必須查得到——見檔頭。
 *
 * `view` 是**視圖名**（`AWAITING_SELLER`／`SELLER_RESPONDED`／`CLOSED`），不是 DB
 * 狀態值；轉換由 `lib/invoices/seller-repo.ts` 的 query 型別把關。
 */
export function useSellerInvoiceComplaints(params: {
	view: string;
	page?: number;
	pageSize?: number;
}): UseSellerInvoiceComplaintsResult {
	const { isLoggedIn, activeRole } = useAuth();
	const identity = useQueryIdentity();
	const enabled = isLoggedIn && activeRole === "seller";

	const query = useQuery({
		queryKey: queryKeys.seller.invoiceComplaints(
			identity,
			params as Record<string, unknown>,
		),
		queryFn: () => listSellerInvoiceComplaints(params),
		enabled,
	});

	return {
		page: enabled ? (query.data ?? null) : null,
		loading: enabled && query.isLoading,
		error: query.error,
		refresh: async () => {
			await query.refetch();
		},
	};
}

export interface UseSellerInvoiceComplaintResult {
	complaint: SellerInvoiceComplaint | null;
	loading: boolean;
	error: unknown;
	refresh: () => Promise<void>;
}

/** 端點 35（S10 唯一資料源）。停權賣家必須查得到——見檔頭。 */
export function useSellerInvoiceComplaint(
	complaintId: string,
): UseSellerInvoiceComplaintResult {
	const { isLoggedIn, activeRole } = useAuth();
	const identity = useQueryIdentity();
	const enabled = isLoggedIn && activeRole === "seller" && complaintId !== "";

	const query = useQuery({
		queryKey: queryKeys.seller.invoiceComplaint(identity, complaintId),
		queryFn: () => fetchSellerInvoiceComplaint(complaintId),
		enabled,
	});

	return {
		complaint: enabled ? (query.data ?? null) : null,
		loading: enabled && query.isLoading,
		error: query.error,
		refresh: async () => {
			await query.refetch();
		},
	};
}

export interface ReportExternalIssuanceArgs {
	invoiceNumber: string;
	invoiceDate: string;
	note: string;
}

export interface UseReportExternalIssuanceResult {
	report: (args: ReportExternalIssuanceArgs) => void;
	reporting: boolean;
	error: unknown;
	succeeded: boolean;
	/** 這張發票有處理中的作業（`9351`）。**不是失敗**：顯示進度並鎖住重送。 */
	inProgress: boolean;
	reset: () => void;
}

/**
 * 端點 62｜回報自行開立的發票號碼（S07 ⑦／S08 ⑤）。
 *
 * 🔴 `expectedVersion` 由呼叫端從**當前畫面讀到的那筆作業**取
 * （S08 走端點 33、S07 走端點 62R），不得寫死也不得傳 `0`——那不是樂觀鎖。
 *
 * 🔴 `9351`（本張發票有處理中的作業）從 `error` 裡分出來：它的語意是「上一次操作
 * 還在跑」而不是「你填錯了」，當成紅字會讓賣家把正確的號碼刪掉重打。
 *
 * ⚠️ 成功後失效四族。其中 `invoiceJob`／`invoiceDocuments`／`invoiceOverview` 是
 * **BLOCK 類**（`CapInvoiceManage`）：停權賣家身上那些查詢的 `enabled` 是 `false`，
 * `invalidateQueries` 只會把它們標成 stale、不會發出請求 ⇒ 不會觸發 9240 的全域
 * 停權廣播（§B-2.0 T18 要防的是把它們併進同一個 key family，不是禁止失效）。
 */
export function useReportExternalIssuance(args: {
	jobId: string;
	orderNo: string;
	expectedVersion: string;
}): UseReportExternalIssuanceResult {
	const identity = useQueryIdentity();
	const queryClient = useQueryClient();

	const mutation = useMutation({
		mutationFn: (input: ReportExternalIssuanceArgs) =>
			reportExternalIssuance({
				jobId: args.jobId,
				expectedVersion: args.expectedVersion,
				...input,
			}),
		onSuccess: async () => {
			await Promise.all([
				queryClient.invalidateQueries({
					queryKey: queryKeys.seller.invoiceJob(identity, args.jobId),
				}),
				queryClient.invalidateQueries({
					queryKey: queryKeys.seller.orderExternalIssuances(
						identity,
						args.orderNo,
					),
				}),
				queryClient.invalidateQueries({
					queryKey: queryKeys.seller
						.invoiceDocuments(identity)
						.slice(0, -1) as unknown as readonly string[],
				}),
				queryClient.invalidateQueries({
					queryKey: queryKeys.seller.invoiceOverview(identity),
				}),
			]);
		},
	});

	const inProgress =
		mutation.error != null &&
		isInvoiceBizCode(
			mutation.error,
			SELLER_INVOICE_BIZ_CODE.OPERATION_IN_PROGRESS,
		);

	return {
		report: (input) => mutation.mutate(input),
		reporting: mutation.isPending,
		error: inProgress ? null : mutation.error,
		succeeded: mutation.isSuccess,
		inProgress,
		reset: () => mutation.reset(),
	};
}
