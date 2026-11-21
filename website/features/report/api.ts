import { createClient } from "@connectrpc/connect";
import type { PolicyCategory } from "@hawkr/api/gen/hawkr/common/v1alpha1/policy_category_pb";
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
