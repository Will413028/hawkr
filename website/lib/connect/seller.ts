import { createClient } from "@connectrpc/connect";
import { SellerAppService } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { authedTransport } from "./transport";

/** Browser and server callers share the generated seller contract. */
export function sellerAppClient() {
	return createClient(SellerAppService, authedTransport());
}
