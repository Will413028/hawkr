import { createClient } from "@connectrpc/connect";
import {
	SocialAppService,
	SocialPublicService,
} from "@hawkr/api/gen/hawkr/social/v1alpha1/social_pb";
import {
	authedTransport,
	optionalAuthTransport,
	type PublicTransportOptions,
	publicTransport,
} from "./transport";

/**
 * SocialPublicService 的匿名 client：結果與觀看者無關，才可以放進 server 端的共用快取
 * （`lib/api/cached.ts`）。伺服器依觀看者算的欄位（`viewer_*`、被買家封鎖的店的貼文）在這裡一律是匿名的答案。
 */
export function socialPublicClient(opts?: PublicTransportOptions) {
	return createClient(SocialPublicService, publicTransport(opts));
}

/**
 * SocialPublicService 的「依觀看者」client：瀏覽器帶同源 session cookie，登入者讀到自己的 `viewer_*` 與
 * 封鎖後的列表；沒登入照常匿名。只在瀏覽器使用，不進共用快取（server 端沒有 cookie，等同匿名）。
 */
export function socialViewerClient(opts?: PublicTransportOptions) {
	return createClient(SocialPublicService, optionalAuthTransport(opts));
}

/** SocialAppService：登入後的互動與店主自己的貼文。 */
export function socialAppClient() {
	return createClient(SocialAppService, authedTransport());
}
