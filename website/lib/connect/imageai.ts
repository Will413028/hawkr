import { createClient } from "@connectrpc/connect";
import { ImageAiAppService } from "@hawkr/api/gen/hawkr/imageai/v1alpha1/imageai_pb";
import { authedTransport } from "./transport";

/** ImageAiAppService：賣家的修圖風格與修圖（TransformImage 是一次生成式呼叫，逾時由呼叫端另設）。 */
export function imageAiAppClient() {
	return createClient(ImageAiAppService, authedTransport());
}
