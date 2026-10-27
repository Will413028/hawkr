import type {
	ProductImageDraft,
	ProductImageInput,
} from "@/lib/products/seller-repo";
import { resolveCdnUrl } from "./cdn";
import { uploadImageFile } from "./upload";

/**
 * Order-sensitive identity of a gallery, used to detect an untouched gallery
 * on edit so the caller can skip re-sending it entirely. Keyed on every field
 * that identifies an entry (`assetId` + `url` + `stagedToken`) so add / remove
 * / reorder / new-upload all change the signature. JSON encoding keeps it
 * collision-free regardless of what characters a URL or token contains.
 */
export function gallerySignature(images: ProductImageDraft[]): string {
	return JSON.stringify(
		images.map((i) => [i.assetId ?? "", i.url, i.stagedToken ?? ""]),
	);
}

/** 把 draft gallery 轉成後端收得懂的 wire 輸入，保留原始順序：
 *  新上傳送 stagedToken、商品既有圖送 assetId、兩者皆無（匯入來源圖）才下載 url→bytes→重傳。 */
export async function resolveStagedImages(
	images: ProductImageDraft[],
): Promise<ProductImageInput[]> {
	const out = await Promise.all(
		images.map(async (img): Promise<ProductImageInput> => {
			// A fresh upload already has its token → pass it straight through.
			if (img.stagedToken) return { stagedToken: img.stagedToken };
			// An image the product already owns round-trips by assetId; the backend
			// keeps the existing gallery entry, so there is nothing to re-upload.
			if (img.assetId) return { assetId: img.assetId };
			// Enforce the production media boundary at the final network sink too.
			// Even if a future caller forgets to normalize its draft, a persisted
			// test-media URL is restored to the production CDN before download.
			const sourceUrl = resolveCdnUrl(img.url);
			const res = await fetch(sourceUrl);
			if (!res.ok) {
				// Name the offending image — a transient CDN failure on ONE existing
				// image otherwise surfaces as an opaque "save failed".
				throw new Error(`無法下載既有圖片 ${sourceUrl} (HTTP ${res.status})`);
			}
			const blob = await res.blob();
			const file = new File([blob], `existing-${Date.now()}.jpg`, {
				type: blob.type || "image/jpeg",
			});
			const asset = await uploadImageFile(file);
			return { stagedToken: asset.stagedToken };
		}),
	);
	return out;
}
