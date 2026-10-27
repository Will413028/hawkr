import { createClient } from "@connectrpc/connect";
import { ContentPublicService } from "@hawkr/api/gen/hawkr/content/v1alpha1/content_pb";
import { type PublicTransportOptions, publicTransport } from "./transport";

/**
 * content 的公開 service 的 client，給 React 之外的呼叫方（server 端的讀取、TanStack Query 的 queryFn）。
 * 元件裡的讀取用 connect-query 的 useQuery 與 AppProviders 的 transport。
 */
export function contentPublicClient(opts?: PublicTransportOptions) {
	return createClient(ContentPublicService, publicTransport(opts));
}
