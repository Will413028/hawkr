import { create } from "@bufbuild/protobuf";
import { createConnectQueryKey } from "@connectrpc/connect-query";
import {
	type GetProspectResponse,
	GetProspectResponseSchema,
	type Prospect,
	SellerProspectAdminService,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/prospect_pb";
import { useQueryClient } from "@tanstack/react-query";

const listKeys = () => [
	createConnectQueryKey({
		schema: SellerProspectAdminService.method.listProspects,
		cardinality: undefined,
	}),
	createConnectQueryKey({
		schema: SellerProspectAdminService.method.listProspectDuplicates,
		cardinality: undefined,
	}),
	createConnectQueryKey({
		schema: SellerProspectAdminService.method.listProspectTaxIDMatches,
		cardinality: undefined,
	}),
];

/**
 * 站外業者的 Connect 查詢快取。寫入成功後呼叫 `written`：詳情以寫入的回應直接換掉（不重讀），清單頁與兩張統計卡
 * 的清單失效重讀。合併讓輸家退場時另呼叫 `retired`：輸家的詳情查詢失效重讀。
 */
export function useProspectCache(): {
	written: (prospect?: Prospect) => void;
	retired: (id: string) => void;
} {
	const queryClient = useQueryClient();
	return {
		written: (prospect) => {
			if (prospect) {
				queryClient.setQueriesData<GetProspectResponse>(
					{
						queryKey: createConnectQueryKey({
							schema: SellerProspectAdminService.method.getProspect,
							cardinality: undefined,
						}),
					},
					(old) =>
						old?.prospect?.id === prospect.id
							? create(GetProspectResponseSchema, { prospect })
							: old,
				);
			}
			for (const queryKey of listKeys()) {
				void queryClient.invalidateQueries({ queryKey });
			}
		},
		retired: (id) => {
			void queryClient.invalidateQueries({
				queryKey: createConnectQueryKey({
					schema: SellerProspectAdminService.method.getProspect,
					input: { id },
					cardinality: "finite",
				}),
			});
		},
	};
}
