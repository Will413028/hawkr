import { createClient } from "@connectrpc/connect";
import {
	ReviewAppService,
	ReviewPublicService,
} from "@hawkr/api/gen/hawkr/review/v1alpha1/review_pb";
import {
	authedTransport,
	type PublicTransportOptions,
	publicTransport,
} from "./transport";

/** ReviewPublicService 的匿名 client：評價列表與統計與觀看者無關，可以進 server 端的共用快取。 */
export function reviewPublicClient(opts?: PublicTransportOptions) {
	return createClient(ReviewPublicService, publicTransport(opts));
}

/** ReviewAppService：買家寫評價、讀自己的評價，店主回覆評價。 */
export function reviewAppClient() {
	return createClient(ReviewAppService, authedTransport());
}
