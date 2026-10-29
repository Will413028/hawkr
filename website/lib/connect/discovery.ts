import { createClient } from "@connectrpc/connect";
import { DiscoveryPublicService } from "@hawkr/api/gen/hawkr/discovery/v1alpha1/discovery_pb";
import { type PublicTransportOptions, publicTransport } from "./transport";

/** DiscoveryPublicService 的 client，給 React 之外的呼叫方（server 端的讀取、queryFn）。 */
export function discoveryPublicClient(opts?: PublicTransportOptions) {
	return createClient(DiscoveryPublicService, publicTransport(opts));
}
