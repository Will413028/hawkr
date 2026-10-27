"use client";

import type { Image } from "@hawkr/api/gen/hawkr/common/v1alpha1/image_pb";
import { useState } from "react";
import { type ImagePreset, imageUri } from "@/utils/image";

/**
 * 業者的 logo 或封面。顯示的圖是 API 回的 `Image`（只把樣板換成 preset，不做網域檢查）；選了檔案但還沒儲存時
 * 用 `preview`（本機檔案的 data URL）。
 */
export default function ProspectBrandImage({
	image,
	preset,
	preview,
	alt,
	empty,
	className,
}: {
	image?: Image;
	preset: ImagePreset;
	preview?: string;
	alt: string;
	empty: string;
	className: string;
}) {
	const [failedSource, setFailedSource] = useState<string>();
	const src = preview ?? imageUri(image, preset);
	return (
		<div className={className}>
			{src && failedSource !== src ? (
				// biome-ignore lint/performance/noImgElement: image gateway URL and local file preview; no image optimizer fetch.
				<img
					src={src}
					alt={alt}
					onError={() => setFailedSource(src)}
					referrerPolicy="no-referrer"
				/>
			) : (
				<span>{src ? "圖片無法載入" : empty}</span>
			)}
		</div>
	);
}
