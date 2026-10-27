import { calculateCenterCrop } from "@/components/banner/crop";
import {
	type ProspectMediaKind,
	validateProspectImage,
} from "@/lib/seller-prospect-media";

/** Rasterize local files only. Logo keeps the full image; cover previews the exact 4:1 center crop. */
export async function prepareProspectImage(
	file: File,
	kind: ProspectMediaKind,
): Promise<File> {
	const invalid = validateProspectImage(file);
	if (invalid) throw new Error(invalid);
	const bitmap = await createImageBitmap(file);
	try {
		if (
			!bitmap.width ||
			!bitmap.height ||
			bitmap.width * bitmap.height > 40_000_000
		) {
			throw new Error("圖片尺寸無效或超過 4,000 萬像素，請先縮小圖片。");
		}
		const crop =
			kind === "cover"
				? calculateCenterCrop(bitmap.width, bitmap.height, 4 / 1)
				: { sx: 0, sy: 0, sWidth: bitmap.width, sHeight: bitmap.height };
		const scale = Math.min(
			1,
			(kind === "cover" ? 1600 : 600) / Math.max(crop.sWidth, crop.sHeight),
		);
		const canvas = document.createElement("canvas");
		canvas.width = Math.max(1, Math.round(crop.sWidth * scale));
		canvas.height = Math.max(1, Math.round(crop.sHeight * scale));
		const context = canvas.getContext("2d");
		if (!context) throw new Error("此瀏覽器無法處理圖片，請換一個瀏覽器再試。");
		context.drawImage(
			bitmap,
			crop.sx,
			crop.sy,
			crop.sWidth,
			crop.sHeight,
			0,
			0,
			canvas.width,
			canvas.height,
		);
		const blob = await new Promise<Blob>((resolve, reject) => {
			canvas.toBlob(
				(value) =>
					value
						? resolve(value)
						: reject(new Error("圖片處理失敗，請重新選擇。")),
				"image/png",
			);
		});
		const prepared = new File([blob], `${kind}.png`, { type: "image/png" });
		const preparedError = validateProspectImage(prepared);
		if (preparedError) throw new Error(preparedError);
		return prepared;
	} finally {
		bitmap.close();
	}
}
