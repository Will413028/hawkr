import { createClient } from "@connectrpc/connect";
import { BuyerProcurementAppService } from "@hawkr/api/gen/hawkr/procurement/v1alpha1/procurement_pb";
import { authedTransport } from "./transport";

/** BuyerProcurementAppService：登入買家的 AI 採購（釐清、候選、詢價發布、比較與接受報價）。 */
export function buyerProcurementClient() {
	return createClient(BuyerProcurementAppService, authedTransport());
}
