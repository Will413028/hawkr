"use client";

import { Code, ConnectError } from "@connectrpc/connect";
import { errorText } from "@hawkr/api/connect/errors";
import { InquiryState } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useCallback } from "react";
import { accountQueryFn, accountQueryPending } from "@/lib/auth/account-query";
import { useAccountStatus } from "@/lib/auth/use-account-status";
import { useDefaultErrorText } from "@/lib/i18n/ui-text";
import { queryKeys } from "@/lib/query/keys";
import {
	fetchMyInquiries,
	fetchMyInquiry,
	fetchPendingInquiryCount,
	type ReplyInput,
	type SellerInquiry,
	type SellerInquiryList,
	submitInquiryReply,
} from "@/lib/seller/inquiries";
import { useQueryIdentity } from "./useQueryIdentity";

const NO_INQUIRIES: SellerInquiry[] = [];

interface UseSellerInquiriesResult {
	records: SellerInquiry[];
	pendingReplyCount: number;
	loading: boolean;
	error: string | null;
}

/**
 * 賣家自己的詢問（`ListMyInquiries`）。詢問以帳號為範圍，伺服器只要求登入（店家已關也讀得到、回得了），所以不看 active role。
 */
export function useSellerInquiries(
	state: InquiryState = InquiryState.UNSPECIFIED,
): UseSellerInquiriesResult {
	const defaults = useDefaultErrorText();
	const identity = useQueryIdentity();
	const status = useAccountStatus();
	const query = useQuery({
		queryKey: queryKeys.seller.inquiryList(identity, { state }),
		queryFn: accountQueryFn(status, () => fetchMyInquiries(state)),
	});
	const data: SellerInquiryList | undefined =
		status === "signedIn" ? query.data : undefined;
	return {
		records: data?.inquiries ?? NO_INQUIRIES,
		pendingReplyCount: data?.pendingReplyCount ?? 0,
		loading: accountQueryPending(status, query.isLoading),
		error: query.error ? errorText(query.error, defaults) : null,
	};
}

/** 導覽徽章的待回覆數；讀取中或失敗都是 0（徽章不顯示）。 */
export function usePendingInquiryCount(): number {
	const identity = useQueryIdentity();
	const status = useAccountStatus();
	const query = useQuery({
		queryKey: queryKeys.seller.inquiryPending(identity),
		queryFn: accountQueryFn(status, () => fetchPendingInquiryCount()),
	});
	return status === "signedIn" ? (query.data ?? 0) : 0;
}

interface UseSellerInquiryResult {
	inquiry: SellerInquiry | null;
	loading: boolean;
	error: string | null;
	/** 伺服器說找不到或不屬於目前帳號（`not_found`、`permission_denied`）。 */
	notFound: boolean;
	refetch: () => Promise<void>;
}

/** 單筆詢問（`GetMyInquiry`）；網址可以直接開啟或重新整理。 */
export function useSellerInquiry(inquiryId: string): UseSellerInquiryResult {
	const defaults = useDefaultErrorText();
	const identity = useQueryIdentity();
	const status = useAccountStatus();
	const query = useQuery({
		queryKey: queryKeys.seller.inquiry(identity, inquiryId),
		queryFn: accountQueryFn(status, () => fetchMyInquiry(inquiryId)),
	});
	const code = query.error ? ConnectError.from(query.error).code : undefined;
	return {
		inquiry: query.data ?? null,
		loading: query.isPending,
		error: query.error ? errorText(query.error, defaults) : null,
		notFound: code === Code.NotFound || code === Code.PermissionDenied,
		refetch: async () => {
			await query.refetch();
		},
	};
}

/** 把伺服器剛回的詢問寫回單筆快取，再讓詢問 family（列表、徽章、單筆）重新載入。 */
export function useAdoptInquiry(): (latest: SellerInquiry) => Promise<void> {
	const queryClient = useQueryClient();
	const identity = useQueryIdentity();
	return useCallback(
		async (latest) => {
			queryClient.setQueryData(
				queryKeys.seller.inquiry(identity, latest.id),
				latest,
			);
			await queryClient.invalidateQueries({
				queryKey: queryKeys.seller.inquiries(identity),
			});
		},
		[queryClient, identity],
	);
}

/** 送出回覆，成功後把回傳的詢問寫回快取。不包成 `useMutation`：回覆表單自己持有忙碌與錯誤狀態。 */
export function useSubmitInquiryReply(): (
	inquiryId: string,
	input: ReplyInput,
) => Promise<SellerInquiry> {
	const adopt = useAdoptInquiry();
	return useCallback(
		async (inquiryId, input) => {
			const result = await submitInquiryReply(inquiryId, input);
			await adopt(result);
			return result;
		},
		[adopt],
	);
}
