import type { PhotoUpload } from "@/features/after-sales/components/AfterSalePhotoUploader";
import { uploadSecureDocument } from "@/lib/media/secure-upload";

/** 詢問回覆附件的 docType；gateway 的 secure upload 不依 docType 分流，這個值只標記用途。 */
export const INQUIRY_EVIDENCE_DOC_TYPE = "inquiry_evidence";

/**
 * 回覆附件走機敏上傳（契約的 `evidence_staged_tokens` 只接受機敏資產）。機敏資產沒有公開網址，縮圖用本機的 object URL，由回覆表單在照片移除與卸載時釋放。
 */
export const uploadInquiryEvidence: PhotoUpload = async (file, onProgress) => {
	const result = await uploadSecureDocument(
		file,
		INQUIRY_EVIDENCE_DOC_TYPE,
		onProgress,
	);
	return { token: result.stagedToken, previewUrl: URL.createObjectURL(file) };
};
