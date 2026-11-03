/**
 * Taiwan-specific validators used by the seller
 * certification wizard. Each function returns `null` on success or a
 * Chinese-language error string the form can surface inline.
 *
 * Mirrors the App's behaviour:
 *   - 統一編號 has the MoF UBN checksum (8 digits, with the "7×0+1×1+...")
 *   - Email / phone use lightweight format checks (server is the truth)
 */

/* ===== Email / Phone ===== */

const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

/** 客服電子信箱的長度上限：PayUni 的 MerEmail、Email 與 MBCMail 至多 50 字，店家欄位的上限與它相同。 */
export const SUPPORT_EMAIL_MAX_LENGTH = 50;

export function validateEmail(value: string): string | null {
	const v = value.trim();
	if (!v) return "請輸入客服電子信箱";
	if (!EMAIL_RE.test(v)) return "Email 格式不正確";
	if ([...v].length > SUPPORT_EMAIL_MAX_LENGTH) {
		return `客服電子信箱最多 ${SUPPORT_EMAIL_MAX_LENGTH} 字`;
	}
	return null;
}

/**
 * Loose Taiwan phone validator. Accepts mobile (09XX-XXX-XXX) or
 * landline with optional area code (e.g. 02-XXXX-XXXX). Strips spaces /
 * dashes / parentheses before checking digit length.
 */
export function validatePhone(value: string): string | null {
	const v = value.trim();
	if (!v) return "請輸入聯絡電話";
	const digits = v.replace(/[\s\-()]/g, "");
	if (!/^\d{8,11}$/.test(digits)) return "電話格式不正確";
	return null;
}

/** 09 開頭的 10 碼手機（KYC 與 PayUni 的聯絡手機）；空白、橫線與括號先去掉。 */
export function validateMobile(value: string): string | null {
	const v = value.trim();
	if (!v) return "請輸入聯絡手機";
	const digits = v.replace(/[\s\-()]/g, "");
	if (!/^09\d{8}$/.test(digits)) return "請輸入 09 開頭的 10 碼手機號碼";
	return null;
}

/** 聯絡人姓名：必填，最多 20 字（PayUni 的 ContactName 上限）。 */
export function validateContactName(value: string): string | null {
	const v = value.trim();
	if (!v) return "請輸入聯絡人姓名";
	if ([...v].length > 20) return "聯絡人姓名最多 20 字";
	return null;
}
