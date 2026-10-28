export const DEMO_CONVERSATION_ID = "demo";
export const ASSISTANT_DEMO_PATH =
	`/account/messages/assistant/${DEMO_CONVERSATION_ID}` as const;
/**
 * Legacy 新對話 segment。
 *
 * 設計書 v6 §V.10.12-4 把唯一正式入口凍結為 `ASSISTANT_PROCUREMENT_PATH`，
 * `/assistant/new` 與 `/account/find-sellers` 一律 redirect 過去——兩個入口
 * 各自掛一套 workspace 就是兩套 conversation truth。
 *
 * 常數留著是因為 §V.14.4-2 要求「以 access metric 與 deployed SHA 證明 legacy
 * 路徑零流量一個完整相容 slice」之後才移除；在那之前 route 仍要認得這個
 * segment 才能 redirect，直接刪掉會變成 404。
 */
export const ASSISTANT_LEGACY_LIVE_CONVERSATION_ID = "new";
export const ASSISTANT_LEGACY_LIVE_PATH =
	`/account/messages/assistant/${ASSISTANT_LEGACY_LIVE_CONVERSATION_ID}` as const;
export const ASSISTANT_PROCUREMENT_PATH =
	"/account/messages/assistant/procurement" as const;

/** 每個對話的路由位址（B-244：conversationId 是 TSID decimal string）。
 * 取代散落的硬編字面值；[conversationId] 路由對非保留字 segment 一律
 * 當真實 id 處理。 */
export function assistantConversationPath(conversationId: string): string {
	return `/account/messages/assistant/${encodeURIComponent(conversationId)}`;
}

/** TSID 一律是十進位字串——用這個判斷 segment 是保留字（demo/new/
 * procurement）還是真實對話 id，垃圾字串直接 404 不用打後端。 */
export function isAssistantConversationId(segment: string): boolean {
	return /^\d{1,20}$/.test(segment);
}
