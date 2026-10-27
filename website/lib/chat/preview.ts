/**
 * Friendly preview text for a chat message's content. Mirrors the buyer
 * APP's `_previewTextForConversation` in
 * `hawkr_buyer/lib/features/chat/presentation/chat_list_screen.dart`
 * so card / image payloads collapse to "傳送了一張卡片" / "傳送了一張圖片"
 * instead of leaking raw JSON. Falls back to the original text when
 * nothing structured is detected.
 *
 * Shared by the conversation list (`type` = REST `lastMessageType`, reliable)
 * and the live WS toast (`type` is always the envelope value `"chat"` for
 * every real message — im-service's `domain.Message.Type` — so the content
 * sniff below is what actually does the work there; the `type` fast-path is
 * still checked first since it's free when it does apply).
 */
export function messagePreview(content: string, type?: string): string {
	const last = (content ?? "").trim();
	const t = (type ?? "").toLowerCase();

	const isImageType = (t: string) =>
		t.includes("image") ||
		t.includes("video") ||
		t.includes("picture") ||
		t.includes("photo");
	const isCardType = (t: string) =>
		t === "order_prev" ||
		t === "order_check" ||
		t === "product" ||
		t.includes("card");

	if (isImageType(t)) return "傳送了一張圖片";
	if (isCardType(t)) return "傳送了一張卡片";
	if (!last) return "尚無訊息";

	// Old reply format "↩︎ preview\nactual" → strip the quoted preview.
	if (last.startsWith("↩︎")) {
		const nl = last.indexOf("\n");
		if (nl > 0 && nl + 1 < last.length) {
			const body = last.slice(nl + 1).trim();
			if (body) return body;
		}
	}

	const lower = last.toLowerCase();
	if (
		(lower.startsWith("http://") || lower.startsWith("https://")) &&
		(/\.(png|jpe?g|gif|webp)$/.test(lower) ||
			lower.includes("/image/") ||
			lower.includes("=image"))
	) {
		return "傳送了一張圖片";
	}
	if (lower.startsWith("data:image/")) return "傳送了一張圖片";
	// Chat images are stored/relayed as a bare R2 objectKey (49 §6.6 im/S4),
	// not a URL — the two checks above only catch a fully-qualified URL, so
	// this catches the raw-key form the WS payload actually carries.
	if (lower.startsWith("public/chat_message/")) return "傳送了一張圖片";

	if (last.startsWith("{") || last.startsWith("[")) {
		try {
			const decoded = JSON.parse(last);
			if (decoded && typeof decoded === "object" && !Array.isArray(decoded)) {
				const obj = decoded as Record<string, unknown>;
				if (typeof obj.text === "string" && obj.text.trim()) {
					return obj.text.trim();
				}
				const innerType = String(
					obj.messageType ?? obj.type ?? "",
				).toLowerCase();
				if (Array.isArray(obj.orders) && obj.title) return "傳送了一張卡片";
				if (isCardType(innerType)) return "傳送了一張卡片";
				if (isImageType(innerType)) return "傳送了一張圖片";
				for (const k of ["url", "image", "imageUrl", "img", "src", "path"]) {
					const v = obj[k];
					if (typeof v === "string" && v.trim()) return "傳送了一張圖片";
				}
			}
		} catch {
			// Parse failed → fall through to raw last message.
		}
	}

	return last;
}
