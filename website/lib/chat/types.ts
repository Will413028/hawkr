/**
 * 聊天的畫面型別（由 `ChatAppService` 的訊息轉出，見 `lib/chat/map.ts`）。
 * ID 是不透明字串，不解析、不轉數字；訊息以對話內的序號 `seq` 排序（從 1 起連號）。
 */

/** 對話的一方：買家方或店家方，由目前的工作區決定（賣家中心是店家方）。 */
export type ChatSide = "buyer" | "store";

export type ChatMessageKind = "text" | "image" | "unsupported";

export interface ChatMessage {
	/** 伺服器的訊息 ID；送出中的樂觀訊息是 `request_id`。 */
	id: string;
	conversationId: string;
	/** 對話內序號；送出中的樂觀訊息還沒有。 */
	seq: number | null;
	sender: ChatSide;
	kind: ChatMessageKind;
	text?: string;
	/** 圖片的顯示網址（伺服器的 `Image` 取 `chat` preset，或樂觀訊息的預覽網址）。 */
	imageUrl?: string;
	/** 被平台隱藏：沒有內容的墓碑。 */
	hidden: boolean;
	/** 寄件方帳號已刪除或店家已退場：沒有內容的墓碑。 */
	erased: boolean;
	timeMs: number;
}

export interface ChatPreview {
	kind: ChatMessageKind;
	text?: string;
	hidden: boolean;
	erased: boolean;
	sender: ChatSide;
}

export interface Conversation {
	id: string;
	/** 對方的 ID：買家方看到的是店家 ID，店家方看到的是買家 ID（路由的 `targetId`）。 */
	targetUserId: string;
	/** 對方的名稱；沒有名稱或買家已刪除時是空字串。 */
	targetNickname: string;
	/** 對方是已刪除的買家：名稱由呈現層取共用文案目錄（{@link peerName}）。 */
	targetDeleted?: boolean;
	/** 對方是買家時的頭像。 */
	targetAvatar?: string;
	/** 對方是店家時的 logo。 */
	logo_url?: string;
	/** 對方是店家時的店面短碼（canonical 路徑）。 */
	storeCode?: string;
	preview?: ChatPreview;
	lastMessageTimeMs: number;
	unreadCount: number;
	/** 對方已讀到的 seq。 */
	peerReadSeq: number;
}

export type ConnectionState =
	| "disconnected"
	| "connecting"
	| "connected"
	| "reconnecting";
