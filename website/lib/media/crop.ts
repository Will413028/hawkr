/**
 * Client-side image cropping for the upload flow.
 *
 * `cropImageFile` renders the selected crop area onto a canvas and re-packs
 * it as a File, so the cropped result rides the existing staged-upload
 * pipeline (`uploadImageFile`) with zero backend changes. Browsers apply
 * EXIF orientation at decode time (`image-orientation: from-image` is the
 * default), so the crop coordinates from react-easy-crop always match what
 * the user saw on screen.
 */

export interface CropAreaPixels {
	x: number;
	y: number;
	width: number;
	height: number;
}

export interface CropOutputOptions {
	/** Long-side cap in px for the encoded output. Defaults to 2048. */
	maxOutput?: number;
	/**
	 * When set, place the selected crop inside a transparent canvas with this
	 * aspect ratio instead of stretching the output to the crop itself.
	 */
	containAspect?: number;
}

export interface CropContainRect {
	x: number;
	y: number;
	width: number;
	height: number;
}

function assertPositiveDimension(value: number): void {
	if (!Number.isFinite(value) || value <= 0) {
		throw new Error("圖片裁切尺寸無效");
	}
}

/** Complete-image destination geometry; spare pixels remain transparent. */
export function calculateCropContainRect(
	sourceWidth: number,
	sourceHeight: number,
	targetWidth: number,
	targetHeight: number,
): CropContainRect {
	assertPositiveDimension(sourceWidth);
	assertPositiveDimension(sourceHeight);
	assertPositiveDimension(targetWidth);
	assertPositiveDimension(targetHeight);

	const scale = Math.min(
		targetWidth / sourceWidth,
		targetHeight / sourceHeight,
	);
	const width = sourceWidth * scale;
	const height = sourceHeight * scale;
	return {
		x: (targetWidth - width) / 2,
		y: (targetHeight - height) / 2,
		width,
		height,
	};
}

export function calculateContainedOutputSize(
	sourceWidth: number,
	sourceHeight: number,
	targetAspect: number,
	maxOutput: number,
): { width: number; height: number } {
	assertPositiveDimension(sourceWidth);
	assertPositiveDimension(sourceHeight);
	assertPositiveDimension(targetAspect);
	assertPositiveDimension(maxOutput);

	const sourceAspect = sourceWidth / sourceHeight;
	const idealWidth =
		sourceAspect > targetAspect ? sourceWidth : sourceHeight * targetAspect;
	const idealHeight =
		sourceAspect > targetAspect ? sourceWidth / targetAspect : sourceHeight;
	const scale = Math.min(1, maxOutput / Math.max(idealWidth, idealHeight));
	const scaledWidth = idealWidth * scale;
	const scaledHeight = idealHeight * scale;

	// Posts currently use the canonical 4:3 feed surface. Snap to whole 4x3
	// pixel units so the downstream card preset has no fractional strip to crop.
	if (Math.abs(targetAspect - 4 / 3) < 0.000001) {
		const units = Math.max(
			1,
			Math.floor(Math.min(scaledWidth / 4, scaledHeight / 3)),
		);
		return { width: units * 4, height: units * 3 };
	}

	if (targetAspect >= 1) {
		const width = Math.max(1, Math.round(scaledWidth));
		return { width, height: Math.max(1, Math.round(width / targetAspect)) };
	}
	const height = Math.max(1, Math.round(scaledHeight));
	return { width: Math.max(1, Math.round(height * targetAspect)), height };
}

export function loadImageElement(src: string): Promise<HTMLImageElement> {
	return new Promise((resolve, reject) => {
		const img = new Image();
		img.onload = () => resolve(img);
		img.onerror = () => reject(new Error("圖片載入失敗"));
		img.src = src;
	});
}

/** PNG keeps its alpha channel; legacy crops otherwise re-encode as JPEG. */
function outputMime(sourceType: string): "image/png" | "image/jpeg" {
	return sourceType === "image/png" ? "image/png" : "image/jpeg";
}

export async function cropImageFile(
	file: File,
	area: CropAreaPixels,
	options?: CropOutputOptions,
): Promise<File> {
	const maxOutput = options?.maxOutput ?? 2048;
	const containAspect = options?.containAspect;
	const url = URL.createObjectURL(file);
	try {
		const img = await loadImageElement(url);
		const containedOutput =
			containAspect === undefined
				? null
				: calculateContainedOutputSize(
						area.width,
						area.height,
						containAspect,
						maxOutput,
					);
		const scale = Math.min(1, maxOutput / Math.max(area.width, area.height));
		const width =
			containedOutput?.width ?? Math.max(1, Math.round(area.width * scale));
		const height =
			containedOutput?.height ?? Math.max(1, Math.round(area.height * scale));

		const canvas = document.createElement("canvas");
		canvas.width = width;
		canvas.height = height;
		const ctx = canvas.getContext("2d");
		if (!ctx) throw new Error("此瀏覽器不支援圖片裁切");
		ctx.imageSmoothingEnabled = true;
		ctx.imageSmoothingQuality = "high";
		let destination: CropContainRect | null = null;
		if (containedOutput) {
			destination = calculateCropContainRect(
				area.width,
				area.height,
				width,
				height,
			);
			ctx.drawImage(
				img,
				area.x,
				area.y,
				area.width,
				area.height,
				destination.x,
				destination.y,
				destination.width,
				destination.height,
			);
		} else {
			ctx.drawImage(
				img,
				area.x,
				area.y,
				area.width,
				area.height,
				0,
				0,
				width,
				height,
			);
		}

		// Transparent padding must never be flattened by JPEG encoding. An exact
		// target-ratio crop has no padding, so a JPEG source can remain compact.
		const hasTransparentPadding = Boolean(
			destination && (destination.x > 0.01 || destination.y > 0.01),
		);
		const mime = hasTransparentPadding ? "image/png" : outputMime(file.type);
		const blob = await new Promise<Blob | null>((resolve) => {
			canvas.toBlob(resolve, mime, 0.92);
		});
		if (!blob) throw new Error("圖片處理失敗，請再試一次");

		const base = file.name.replace(/\.[^.]+$/, "") || "image";
		const ext = mime === "image/png" ? "png" : "jpg";
		return new File([blob], `${base}-crop.${ext}`, {
			type: mime,
			lastModified: file.lastModified,
		});
	} finally {
		URL.revokeObjectURL(url);
	}
}
