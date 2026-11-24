/**
 * TransformImage 的原圖上限（A9-D37；proto `TransformImageRequest.image_data`）：超過伺服器回 invalid_argument，
 * 所以 client 送出前先縮圖。
 */
export const AI_IMAGE_MAX_BYTES = 2 * 1024 * 1024;

/** 每一輪縮小的比例與 JPEG 品質；由大到小，第一個放得進上限的就用。 */
export const DOWNSIZE_STEPS: ReadonlyArray<{ scale: number; quality: number }> =
	[
		{ scale: 1, quality: 0.9 },
		{ scale: 0.85, quality: 0.85 },
		{ scale: 0.7, quality: 0.8 },
		{ scale: 0.55, quality: 0.8 },
		{ scale: 0.4, quality: 0.75 },
		{ scale: 0.3, quality: 0.7 },
		{ scale: 0.2, quality: 0.7 },
	];

export type EncodeImage = (scale: number, quality: number) => Promise<Blob>;

/**
 * 原圖已在上限內就原樣回傳；否則依 DOWNSIZE_STEPS 逐步重新編碼，回第一個在上限內的結果。
 * 全部步驟都超過時丟錯（圖片極端大，沒有可送的版本），不送出會被伺服器拒絕的請求。
 */
export async function fitToLimit(
	source: Blob,
	encode: EncodeImage,
	maxBytes = AI_IMAGE_MAX_BYTES,
): Promise<Blob> {
	if (source.size <= maxBytes) return source;
	for (const { scale, quality } of DOWNSIZE_STEPS) {
		const out = await encode(scale, quality);
		if (out.size <= maxBytes) return out;
	}
	throw new RangeError("image cannot be downsized under the limit");
}

/** 瀏覽器端：以 canvas 重新編碼成 JPEG。 */
export function canvasEncoder(source: Blob): EncodeImage {
	let bitmap: Promise<ImageBitmap> | undefined;
	return async (scale, quality) => {
		bitmap ??= createImageBitmap(source);
		const img = await bitmap;
		const width = Math.max(1, Math.round(img.width * scale));
		const height = Math.max(1, Math.round(img.height * scale));
		const canvas = document.createElement("canvas");
		canvas.width = width;
		canvas.height = height;
		const ctx = canvas.getContext("2d");
		if (!ctx) throw new Error("canvas 2d context unavailable");
		ctx.drawImage(img, 0, 0, width, height);
		return new Promise<Blob>((resolve, reject) => {
			canvas.toBlob(
				(blob) =>
					blob ? resolve(blob) : reject(new Error("canvas encode failed")),
				"image/jpeg",
				quality,
			);
		});
	};
}

export function fitImageToLimit(source: Blob): Promise<Blob> {
	return fitToLimit(source, canvasEncoder(source));
}
