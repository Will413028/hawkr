import type { Image } from "@hawkr/api/gen/hawkr/common/v1alpha1/image_pb";
import type { ImagePreset } from "./image-presets.generated";

export { IMAGE_PRESETS, type ImagePreset } from "./image-presets.generated";

/**
 * API 回的圖片在某個 preset 的網址（ADR 2026-09-25-api-image-uri-template D3）：只把 `uri_template` 的
 * `{preset}` 換成 preset 名稱，不解析、不改寫其他部分；外部圖片的樣板沒有變數，原樣回傳。沒有圖時回 undefined。
 */
export function imageUri(
	image: Image | undefined,
	preset: ImagePreset,
): string | undefined {
	return image?.uriTemplate.replaceAll("{preset}", preset);
}
