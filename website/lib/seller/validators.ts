/**
 * Taiwan-specific identity / company validators used by the seller
 * certification wizard. Each function returns `null` on success or a
 * Chinese-language error string the form can surface inline.
 *
 * Mirrors the App's behaviour:
 *   - 身分證字號 has the official MOI checksum (A-Z + 9 digits)
 *   - 統一編號 has the MoF UBN checksum (8 digits, with the "7×0+1×1+...")
 *   - Email / phone use lightweight format checks (server is the truth)
 */

/* ===== 身分證字號 ===== */

const ID_LETTER_VALUES: Record<string, number> = {
	A: 10,
	B: 11,
	C: 12,
	D: 13,
	E: 14,
	F: 15,
	G: 16,
	H: 17,
	I: 34,
	J: 18,
	K: 19,
	L: 20,
	M: 21,
	N: 22,
	O: 35,
	P: 23,
	Q: 24,
	R: 25,
	S: 26,
	T: 27,
	U: 28,
	V: 29,
	W: 32,
	X: 30,
	Y: 31,
	Z: 33,
};

/**
 * Validate Taiwan national ID. Format: 1 letter + 9 digits.
 * Second char must be 1 (male) or 2 (female).
 * Checksum: letter mapped to two digits → first digit ×1, second ×9,
 * then digits 1..8 ×(8..1), then digit 9 ×1; total mod 10 must be 0.
 */
export function validateTwId(value: string): string | null {
	const v = value.trim().toUpperCase();
	if (!v) return "請輸入身分證字號";
	if (!/^[A-Z][12]\d{8}$/.test(v)) {
		return "格式錯誤（A123456789）";
	}
	const letterValue = ID_LETTER_VALUES[v[0] as string];
	if (letterValue == null) return "無效的英文字母";
	const n1 = Math.floor(letterValue / 10);
	const n2 = letterValue % 10;
	let sum = n1 + n2 * 9;
	for (let i = 1; i <= 8; i++) {
		sum += Number(v[i]) * (9 - i);
	}
	sum += Number(v[9]);
	if (sum % 10 !== 0) return "身分證字號檢查碼錯誤，請確認再輸入";
	return null;
}

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

/* ===== Bank account ===== */

export function validateBankAccount(value: string): string | null {
	const v = value.trim();
	if (!v) return "請輸入銀行帳號";
	if (!/^\d{8,16}$/.test(v)) return "銀行帳號需為 8–16 位數字";
	return null;
}
