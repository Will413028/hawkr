import { createClient } from "@connectrpc/connect";
import { CrmAppService } from "@hawkr/api/gen/hawkr/crm/v1alpha1/crm_pb";
import { authedTransport } from "./transport";

/** CrmAppService：店家的客戶與標籤（每個方法帶 store_id，呼叫者必須是這家店的店主）。 */
export function crmAppClient() {
	return createClient(CrmAppService, authedTransport());
}
