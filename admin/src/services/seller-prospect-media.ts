"use client";

import { validateProspectImage } from "@/lib/seller-prospect-media";
import { uploadImageToMedia } from "./media-upload";

/** 檢查檔案後上傳到 media，回暫存 token（換圖時帶進 `UpdateProspect`）。 */
export async function stageProspectImage(file: File): Promise<string> {
	const invalid = validateProspectImage(file);
	if (invalid) throw new Error(invalid);
	return uploadImageToMedia(file);
}
