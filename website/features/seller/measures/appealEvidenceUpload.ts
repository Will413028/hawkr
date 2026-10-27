import type { PhotoUpload } from "@/features/after-sales/components/AfterSalePhotoUploader";
import { uploadSecureDocument } from "@/lib/media/secure-upload";

/** 申覆的 docType；gateway 的 secure upload 不依 docType 分流，這個值只標記用途。 */
export const APPEAL_EVIDENCE_DOC_TYPE = "appeal_evidence";

/**
 * 申覆佐證走機敏上傳（bytes 不落公開 OSS，契約的 `evidence_staged_tokens` 只接受機敏資產）。
 * 機敏資產沒有公開網址，縮圖用本機的 object URL。
 */
export const uploadAppealEvidence: PhotoUpload = async (file, onProgress) => {
	const result = await uploadSecureDocument(
		file,
		APPEAL_EVIDENCE_DOC_TYPE,
		onProgress,
	);
	return { token: result.stagedToken, previewUrl: URL.createObjectURL(file) };
};
