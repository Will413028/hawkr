import { createClient } from "@connectrpc/connect";
import { BuyerProcurementAppService } from "@hawkr/api/gen/hawkr/procurement/v1alpha1/procurement_pb";
import { authedTransport, streamTransport } from "./transport";

/** BuyerProcurementAppService：登入買家的 AI 採購（釐清、候選、詢價發布、比較與接受報價）。 */
export function buyerProcurementClient() {
	return createClient(BuyerProcurementAppService, authedTransport());
}

/**
 * BuyerProcurementAppService 的串流呼叫（`WatchRun`）：用沒有逾時、沒有重試的 `streamTransport`，
 * 串流的壽命由伺服器決定（最長 100 秒），接續由 `features/procurement-assistant/watch-run.ts` 以最後的 cursor 重新訂閱。
 */
export function buyerProcurementStreamClient() {
	return createClient(BuyerProcurementAppService, streamTransport());
}
