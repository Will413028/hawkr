import type { DescMessage, DescMethodUnary } from "@bufbuild/protobuf";
import { createConnectQueryKey } from "@connectrpc/connect-query";
import { SellerGovernanceAdminService } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import { SellerAdminService } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/seller_pb";
import { useQueryClient } from "@tanstack/react-query";

function keyOf<I extends DescMessage, O extends DescMessage>(
	schema: DescMethodUnary<I, O>,
) {
	return createConnectQueryKey({ schema, cardinality: undefined });
}

/**
 * 通報與案件的寫入成功之後，讓讀到的資料整組失效重讀：通報與案件（詳情、清單）、案件的詢問、主管機關來函（從來函建立措施會改變來函的導出措施）、導出的措施、案件註記（改嚴重度會寫一則）、
 * 店家（建立措施會改變店家的措施與可用的終止事由）。一個寫入會讓很多地方的 can_* 與選項改變，所以不挑著失效。
 */
export function useInvalidateGovernance(): () => void {
	const queryClient = useQueryClient();
	return () => {
		const G = SellerGovernanceAdminService.method;
		for (const queryKey of [
			keyOf(G.getReport),
			keyOf(G.listReports),
			keyOf(G.getCase),
			keyOf(G.listCases),
			keyOf(G.listMeasures),
			keyOf(G.listCaseNotes),
			keyOf(G.listCaseInquiries),
			keyOf(G.getAuthorityOrder),
			keyOf(G.listAuthorityOrders),
			keyOf(SellerAdminService.method.getStore),
		]) {
			void queryClient.invalidateQueries({ queryKey });
		}
	};
}
