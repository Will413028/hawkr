import { createClient } from "@connectrpc/connect";
import { SellerPublicService } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import {
	optionalAuthTransport,
	type PublicTransportOptions,
	publicTransport,
} from "./transport";

/** SellerPublicService 的 client，給 React 之外的呼叫方（server 端的讀取、queryFn、beacon）。 */
export function sellerPublicClient(opts?: PublicTransportOptions) {
	return createClient(SellerPublicService, publicTransport(opts));
}

/**
 * 寫入型 beacon 用（RecordProductView）：登入時帶憑證，伺服器才能依 seller.proto 的約定不計登入者；
 * 沒登入照常匿名送出。讀取一律用 sellerPublicClient，維持公開快取。
 */
export function sellerBeaconClient(opts?: PublicTransportOptions) {
	return createClient(SellerPublicService, optionalAuthTransport(opts));
}
