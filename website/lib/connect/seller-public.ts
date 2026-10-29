import { createClient } from "@connectrpc/connect";
import { SellerPublicService } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { type PublicTransportOptions, publicTransport } from "./transport";

/** SellerPublicService 的 client，給 React 之外的呼叫方（server 端的讀取、queryFn、beacon）。 */
export function sellerPublicClient(opts?: PublicTransportOptions) {
	return createClient(SellerPublicService, publicTransport(opts));
}
