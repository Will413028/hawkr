import { hasAsciiControlCharacters } from "../policy";

/** `dhof_` ＋ 43 字 base64url ＝ 48 字（56 號 §8.1.6）。 */
const HANDOFF_CODE_PATTERN = /^dhof_[A-Za-z0-9_-]{43}$/u;

/**
 * The operation credential never reaches the browser: it is read from the
 * create response, spent on the reconciling `GET current`, and dropped. This
 * guard only keeps a malformed value out of an outbound header.
 */
export function isSafeOperationCredential(value: unknown): value is string {
	return (
		typeof value === "string" &&
		value.length >= 16 &&
		value.length <= 8192 &&
		!hasAsciiControlCharacters(value) &&
		!value.includes(" ") &&
		!value.includes(";") &&
		!value.includes(",")
	);
}

export function isDelegatedHandoffCode(value: unknown): value is string {
	return typeof value === "string" && HANDOFF_CODE_PATTERN.test(value);
}

/**
 * 兌換去向只能來自部署設定，**不得由請求的 `Host` 推導**：那讓任何能控制
 * `Host` 標頭的人指定 credential 的兌換去向。缺少或形狀不對時回 `null`，
 * 呼叫端據此拒絕建立工作階段（fail-closed）。
 */
export function resolveWebOrigin(
	env: Record<string, string | undefined>,
): string | null {
	const raw = env.WEB_ORIGIN?.trim();
	if (!raw) return null;
	let parsed: URL;
	try {
		parsed = new URL(raw);
	} catch {
		return null;
	}
	// 只允許 origin 本身：帶路徑／查詢／片段的值會讓 `${origin}/adm/enter` 組出
	// 一個看起來合法、實際打到別處的 URL。
	if (parsed.pathname !== "/" || parsed.search || parsed.hash) return null;
	if (parsed.username || parsed.password) return null;
	if (parsed.protocol === "https:") return parsed.origin;
	// 本機 website dev server 才可能是 http；部署環境一律 https。
	return parsed.protocol === "http:" && env.NODE_ENV !== "production"
		? parsed.origin
		: null;
}

export const WEB_ORIGIN = resolveWebOrigin(process.env);

export function buildHandoffLocation(
	webOrigin: string,
	handoffCode: string,
): string {
	return `${webOrigin}/adm/enter?code=${encodeURIComponent(handoffCode)}`;
}

/**
 * 建立工作階段是頂層表單導覽（瀏覽器要能跟著 303 跨 host），因此送不出
 * `X-Requested-With` 這類自訂標頭 —— 原本靠自訂標頭順帶擋下的跨站表單提交，
 * 改由這道 `Origin` 檢查承擔。`SameSite=Lax` 已擋掉跨站 POST 的 cookie，
 * 兩者是同一件事的兩層。
 */
export function isSameOriginSubmission(headers: Headers): boolean {
	const origin = headers.get("origin");
	const host = headers.get("host");
	if (!origin || !host) return false;
	try {
		return new URL(origin).host === host;
	} catch {
		return false;
	}
}
