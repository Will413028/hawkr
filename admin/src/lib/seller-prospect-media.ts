import type { MessageInitShape } from "@bufbuild/protobuf";
import {
	ProspectSchema,
	type UpdateProspectRequestSchema,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/prospect_pb";
import { fieldMask } from "@hawkr/api/proto/field-mask";

export type ProspectMediaKind = "logo" | "cover";
export const PROSPECT_IMAGE_MAX_BYTES = 5 * 1024 * 1024;
export const PROSPECT_IMAGE_TYPES = ["image/jpeg", "image/png", "image/webp"];

export function validateProspectImage(
	file: Pick<File, "size" | "type">,
): string | null {
	if (!PROSPECT_IMAGE_TYPES.includes(file.type)) {
		return "請選擇 JPG、PNG 或 WebP 圖片。";
	}
	if (file.size <= 0 || file.size > PROSPECT_IMAGE_MAX_BYTES) {
		return "圖片須大於 0 bytes 且不超過 5 MB。";
	}
	return null;
}

export const SOURCE_URL_MAX_LENGTH = 2048;

/**
 * 素材來源網址的檢查，規則同伺服器（`normalizeSourceURL`）：去前後空白後是 https、有主機、最長 2048 bytes。來源網址只作稽核、
 * 伺服器不會去取，所以不另擋內網位址。通過回去空白後的網址，沒通過回 null。
 */
export function normalizeSourceUrl(value: string): string | null {
	const trimmed = value.trim();
	if (new TextEncoder().encode(trimmed).length > SOURCE_URL_MAX_LENGTH) {
		return null;
	}
	try {
		const url = new URL(trimmed);
		return url.protocol === "https:" && url.host !== "" ? trimmed : null;
	} catch {
		return null;
	}
}

/** 換 logo 或封面的請求：mask 只列這一張圖，帶共用的 etag；來源網址選填。 */
export function prospectImageUpdate(input: {
	id: string;
	etag: string;
	kind: ProspectMediaKind;
	stagedToken: string;
	sourceUrl: string;
}): MessageInitShape<typeof UpdateProspectRequestSchema> {
	const upload = {
		stagedToken: input.stagedToken,
		sourceUrl: input.sourceUrl,
	};
	const prospect: MessageInitShape<typeof ProspectSchema> = {
		id: input.id,
		etag: input.etag,
		...(input.kind === "logo"
			? { logoUpload: upload }
			: { coverUpload: upload }),
	};
	return {
		prospect,
		updateMask: fieldMask(ProspectSchema, [
			input.kind === "logo" ? "logo_upload" : "cover_upload",
		]),
	};
}
