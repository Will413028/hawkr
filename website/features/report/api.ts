import { createClient } from "@connectrpc/connect";
import type { PolicyCategory } from "@hawkr/api/gen/hawkr/common/v1alpha1/policy_category_pb";
import {
	ModerationAppService,
	ReportTargetKind,
} from "@hawkr/api/gen/hawkr/moderation/v1alpha1/moderation_pb";
import { SellerReportAppService } from "@hawkr/api/gen/hawkr/seller/v1alpha1/report_pb";
import { authedTransport } from "@/lib/connect/transport";

export interface CreateReportInput {
	storeId: string;
	category: PolicyCategory;
	description: string;
	productId?: string;
	orderNo?: string;
	evidenceStagedTokens: string[];
}

export function createReport(input: CreateReportInput) {
	return createClient(SellerReportAppService, authedTransport()).createReport({
		storeId: input.storeId,
		category: input.category,
		description: input.description,
		productId: input.productId ?? "",
		orderNo: input.orderNo ?? "",
		evidenceStagedTokens: input.evidenceStagedTokens,
	});
}

export function listMyReports(pageToken: string) {
	return createClient(ModerationAppService, authedTransport()).listMyReports({
		pageToken,
	});
}

export function getMyReport(reportId: string) {
	return createClient(SellerReportAppService, authedTransport()).getMyReport({
		reportId,
	});
}

export function addReportSupplement(input: {
	reportId: string;
	description: string;
	evidenceStagedTokens: string[];
}) {
	return createClient(
		SellerReportAppService,
		authedTransport(),
	).addReportSupplement(input);
}

export type CommunityReportTarget = {
	kind: "review" | "chatMessage";
	id: string;
};

const TARGET_KIND = {
	review: ReportTargetKind.REVIEW,
	chatMessage: ReportTargetKind.CHAT_MESSAGE,
} as const;

/** 檢舉一則評價或一則聊天訊息（`ModerationAppService.SubmitReport`）；同一個 `requestId` 重送回第一次的結果。 */
export function submitCommunityReport(input: {
	target: CommunityReportTarget;
	category: PolicyCategory;
	description: string;
	requestId: string;
}) {
	return createClient(ModerationAppService, authedTransport()).submitReport({
		target: { kind: TARGET_KIND[input.target.kind], id: input.target.id },
		category: input.category,
		description: input.description,
		requestId: input.requestId,
	});
}
