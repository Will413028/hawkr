/**
 * 站外業者邀請連結的 fragment 解析（A12）。
 *
 * secret 刻意放在 URL fragment 而不是 query：fragment 不會進 access log、也不會
 * 經 Referer 外洩，後端有測試釘住這個形狀。代價是 server 端永遠看不到它——
 * `/vendor/rfq` 必須在瀏覽器讀 `window.location.hash`，任何 server-side 讀取都
 * 只會拿到空字串。
 */
export function parseInviteSecret(hash: string): string | null {
	const raw = hash.startsWith("#") ? hash.slice(1) : hash;
	if (!raw) return null;
	const secret = new URLSearchParams(raw).get("invite")?.trim();
	return secret ? secret : null;
}

/**
 * 兌換之後的頁面網址帶收件人 ID：`/vendor/rfq?r=<rfq_recipient_id>`，重新整理時以它指名要用哪一張邀請的
 * session（瀏覽器可以同時持有多張邀請的 cookie）。ID 是不透明的十進位字串，不解析成數字；它不是憑證，
 * 沒有那張邀請的 cookie 讀不到任何東西。
 */
export function recipientQuery(rfqRecipientId: string): string {
	return `?r=${encodeURIComponent(rfqRecipientId)}`;
}

const INT64_MAX = "9223372036854775807";

/**
 * 網址的 `r` 必須是正的十進位標準寫法（無前導 0、不超過 int64）：其他值不可能是收件人 ID，
 * 當作沒有，不去問伺服器。ID 本身仍是不透明字串，這裡只擋格式、不轉數字給呼叫端。
 */
export function parseRecipientId(search: string): string | null {
	const id = new URLSearchParams(search).get("r")?.trim();
	if (!id || !/^[1-9][0-9]*$/.test(id)) return null;
	// 無前導 0 的十進位字串：位數較少、或位數相同而字典序不大於上限，就不超過 int64。
	return id.length < INT64_MAX.length ||
		(id.length === INT64_MAX.length && id <= INT64_MAX)
		? id
		: null;
}
