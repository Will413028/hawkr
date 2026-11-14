/**
 * 帳號的聯絡信箱（identity 的 `GetContactEmail`／`SetContactEmail`）：賣家的收款帳戶聯絡人用它。
 * provider 驗證過的信箱唯讀；沒有的時候賣家自己填，未驗證。送出平台審核（`SubmitKYC`）之前要有其中之一。
 */

import { Code, ConnectError, createClient } from "@connectrpc/connect";
import { classifyError, factsFromError } from "@hawkr/api/connect/errors";
import { ErrorReason } from "@hawkr/api/gen/hawkr/common/v1alpha1/error_reason_pb";
import {
	type ContactEmail,
	ContactEmailSource,
	IdentityAppService,
} from "@hawkr/api/gen/hawkr/identity/v1alpha1/identity_pb";
import { authedTransport } from "@/lib/connect/transport";

export interface AccountContactEmail {
	email: string;
	/** `verified`：provider 驗證過、唯讀；`self-entered`：賣家自己填的，可以改。 */
	source: "verified" | "self-entered";
}

/** 伺服器的格式規則：去前後空白、轉小寫後至多 254 字，`local@domain` 且網域要有點。 */
export const CONTACT_EMAIL_MAX_LENGTH = 254;
const CONTACT_EMAIL_RE = /^[^\s@]+@[^\s@.]+(\.[^\s@.]+)+$/;

export function normalizeContactEmail(value: string): string {
	return value.trim().toLowerCase();
}

export function validateContactEmail(value: string): string | null {
	const v = normalizeContactEmail(value);
	if (!v) return "請輸入金流商聯絡信箱";
	if (!CONTACT_EMAIL_RE.test(v)) return "Email 格式不正確";
	if ([...v].length > CONTACT_EMAIL_MAX_LENGTH) {
		return `信箱最多 ${CONTACT_EMAIL_MAX_LENGTH} 字`;
	}
	return null;
}

function identityApp() {
	return createClient(IdentityAppService, authedTransport());
}

function contactOf(
	contact: ContactEmail | undefined,
): AccountContactEmail | null {
	if (!contact?.email) return null;
	switch (contact.source) {
		case ContactEmailSource.VERIFIED:
			return { email: contact.email, source: "verified" };
		case ContactEmailSource.SELF_ENTERED:
			return { email: contact.email, source: "self-entered" };
		default:
			throw new Error("不認得的聯絡信箱來源");
	}
}

/** 目前的聯絡信箱；帳號還沒有是 null。 */
export async function fetchContactEmail(): Promise<AccountContactEmail | null> {
	const res = await identityApp().getContactEmail({});
	return contactOf(res.contactEmail);
}

/** 設定自填的聯絡信箱（帳號有 provider 驗證過的信箱時伺服器拒絕）。 */
export async function setContactEmail(
	email: string,
): Promise<AccountContactEmail> {
	const res = await identityApp().setContactEmail({
		email: normalizeContactEmail(email),
	});
	const contact = contactOf(res.contactEmail);
	if (!contact) throw new Error("聯絡信箱為空");
	return contact;
}

/** `SubmitKYC` 回 failed_precondition 加 reason CONTACT_EMAIL_REQUIRED；顯示的文字由錯誤分類（伺服器的 LocalizedMessage 或預設文字）決定。 */
export function isContactEmailRequired(error: unknown): boolean {
	const e = ConnectError.from(error);
	if (e.code !== Code.FailedPrecondition) return false;
	const d = classifyError(factsFromError(e));
	return (
		d.action === "reason" && d.reason === ErrorReason.CONTACT_EMAIL_REQUIRED
	);
}
