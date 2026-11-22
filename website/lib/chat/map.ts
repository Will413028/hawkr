import { timestampMs } from "@bufbuild/protobuf/wkt";
import {
	type ChatBuyer,
	ChatParty,
	MessageKind,
	type Conversation as ProtoConversation,
	type Message as ProtoMessage,
	type Preview as ProtoPreview,
} from "@hawkr/api/gen/hawkr/chat/v1alpha1/chat_pb";
import {
	chatPreviewErased,
	chatPreviewHidden,
	chatPreviewImage,
} from "@hawkr/ui-text/keys";
import type { UiText } from "@/lib/i18n/ui-text";
import { imageUri } from "@/lib/media/image";
import type {
	ChatMessage,
	ChatMessageKind,
	ChatPreview,
	ChatSide,
	Conversation,
} from "./types";

function partyToSide(p: ChatParty): ChatSide {
	return p === ChatParty.STORE ? "store" : "buyer";
}

function kindOf(k: MessageKind): ChatMessageKind {
	if (k === MessageKind.TEXT) return "text";
	if (k === MessageKind.IMAGE) return "image";
	return "unsupported";
}

export function messageFromProto(m: ProtoMessage): ChatMessage {
	return {
		id: m.id,
		conversationId: m.conversationId,
		seq: Number(m.seq),
		sender: partyToSide(m.sender),
		kind: kindOf(m.kind),
		text: m.text,
		imageUrl: imageUri(m.image, "chat"),
		hidden: m.hidden,
		erased: m.erased,
		timeMs: m.createTime ? timestampMs(m.createTime) : 0,
	};
}

function previewFromProto(
	p: ProtoPreview | undefined,
): ChatPreview | undefined {
	if (!p) return undefined;
	return {
		kind: kindOf(p.kind),
		text: p.text,
		hidden: p.hidden,
		erased: p.erased,
		sender: partyToSide(p.sender),
	};
}

function buyerName(b: ChatBuyer | undefined): string {
	return b?.deleted ? "" : (b?.name ?? "");
}

/** 對話列表的一列。`side` 是呼叫者的一方，決定「對方」是店家還是買家。 */
export function conversationFromProto(
	c: ProtoConversation,
	side: ChatSide,
): Conversation {
	const base = {
		id: c.id,
		preview: previewFromProto(c.preview),
		lastMessageTimeMs: c.lastMessageTime ? timestampMs(c.lastMessageTime) : 0,
		unreadCount: Number(c.unreadCount),
		peerReadSeq: Number(c.peerReadSeq),
	};
	if (side === "buyer") {
		return {
			...base,
			targetUserId: c.store?.id ?? "",
			targetNickname: c.store?.name ?? "",
			logo_url: imageUri(c.store?.logo, "avatar"),
			storeCode: c.store?.storeCode || undefined,
		};
	}
	return {
		...base,
		targetUserId: c.buyer?.id ?? "",
		targetNickname: buyerName(c.buyer),
		targetAvatar: imageUri(c.buyer?.avatar, "avatar"),
	};
}

/**
 * 對話列表與通知上的預覽文字：文字取伺服器給的預覽，其餘取共用文案目錄；
 * 不支援的種類沒有文字（空字串），不自己寫句子。
 */
export function previewText(ut: UiText, p: ChatPreview | undefined): string {
	if (!p) return "";
	if (p.erased) return ut(chatPreviewErased());
	if (p.hidden) return ut(chatPreviewHidden());
	if (p.kind === "image") return ut(chatPreviewImage());
	if (p.kind === "text") return p.text ?? "";
	return "";
}
