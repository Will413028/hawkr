const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;

/** 路由上的討論串 id 的標準寫法（小寫 UUID，與伺服器回傳的 `ForumEntry.id` 同形）；不是 UUID 回 null。 */
export function canonicalThreadId(raw: string): string | null {
	const id = raw.toLowerCase();
	return UUID.test(id) ? id : null;
}
