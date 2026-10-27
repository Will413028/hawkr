import { createClient } from "@connectrpc/connect";
import { BuyerAppService } from "@hawkr/api/gen/hawkr/buyer/v1alpha1/buyer_pb";
import { authedTransport } from "./transport";

/** Browser and server callers share the generated buyer contract. */
export function buyerAppClient() {
	return createClient(BuyerAppService, authedTransport());
}
