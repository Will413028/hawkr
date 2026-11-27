export const BANNER_ASPECT_RATIO = 16 / 9;
export const BANNER_CANVAS_WIDTH = 1600;
export const BANNER_CANVAS_HEIGHT = 900;

export interface CropRect {
	sx: number;
	sy: number;
	sWidth: number;
	sHeight: number;
}

/**
 * 計算以 16:9 為目標長寬比的置中裁切座標與尺寸。
 */
export function calculateCenterCrop(
	srcWidth: number,
	srcHeight: number,
	targetAspect: number = BANNER_ASPECT_RATIO,
): CropRect {
	if (srcWidth <= 0 || srcHeight <= 0) {
		return {
			sx: 0,
			sy: 0,
			sWidth: Math.max(0, srcWidth),
			sHeight: Math.max(0, srcHeight),
		};
	}
	const currentAspect = srcWidth / srcHeight;
	if (Math.abs(currentAspect - targetAspect) < 0.001) {
		return { sx: 0, sy: 0, sWidth: srcWidth, sHeight: srcHeight };
	}
	if (currentAspect > targetAspect) {
		const sWidth = srcHeight * targetAspect;
		const sx = (srcWidth - sWidth) / 2;
		return { sx, sy: 0, sWidth, sHeight: srcHeight };
	}
	const sHeight = srcWidth / targetAspect;
	const sy = (srcHeight - sHeight) / 2;
	return { sx: 0, sy, sWidth: srcWidth, sHeight };
}

/**
 * 在瀏覽器環境將使用者上傳的圖片 File 經 16:9 置中裁切後轉換為標準 16:9 的 PNG File。
 */
export async function cropImageFileTo16x9(file: File): Promise<File> {
	if (typeof window === "undefined" || typeof document === "undefined") {
		return file;
	}
	return new Promise((resolve) => {
		const img = new Image();
		const blobUrl = URL.createObjectURL(file);
		img.onload = () => {
			URL.revokeObjectURL(blobUrl);
			const crop = calculateCenterCrop(
				img.naturalWidth,
				img.naturalHeight,
				BANNER_ASPECT_RATIO,
			);
			const canvas = document.createElement("canvas");
			const outWidth = Math.min(Math.round(crop.sWidth), BANNER_CANVAS_WIDTH);
			const outHeight = Math.round(outWidth / BANNER_ASPECT_RATIO);
			canvas.width = outWidth;
			canvas.height = outHeight;
			const ctx = canvas.getContext("2d");
			if (!ctx) {
				resolve(file);
				return;
			}
			ctx.drawImage(
				img,
				crop.sx,
				crop.sy,
				crop.sWidth,
				crop.sHeight,
				0,
				0,
				outWidth,
				outHeight,
			);
			canvas.toBlob((blob) => {
				if (!blob) {
					resolve(file);
					return;
				}
				const croppedFile = new File(
					[blob],
					file.name.replace(/\.[^.]+$/, ".png"),
					{
						type: "image/png",
					},
				);
				resolve(croppedFile);
			}, "image/png");
		};
		img.onerror = () => {
			URL.revokeObjectURL(blobUrl);
			resolve(file);
		};
		img.src = blobUrl;
	});
}
