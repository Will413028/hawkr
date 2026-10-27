/**
 * Chat REST repository (`/api/v1/app/chat/*`).
 *
 *   POST  /messages                       send text/image (auth)
 *   GET   /history/{targetId}             load history (beforeId/pageSize)
 *   GET   /conversations                  conv list
 *   GET   /unread                         total unread badge
 *   PATCH /messages                       { messageIds[] } batch ack
 *   PATCH /conversations/{targetId}       whole-thread ack
 *   PATCH /conversations                  mark all read
 *   GET   /online                         online userIds
 *
 * Wire shape varies between buyer + seller (the latter sends/receives
 * fromUserId/toUserId; legacy uses fromId/toId). Normalizers cover both.
 */

import { ApiClientError, authedFetch } from "../api/client";
import type { ChatMessage, Conversation } from "./types";

const BASE = "/app/chat";

interface ServerMessage {
	id?: number | string;
	messageId?: number | string;
	fromUserId?: number | string;
	toUserId?: number | string;
	fromId?: number | string;
	toId?: number | string;
	senderId?: number | string;
	receiverId?: number | string;
	content?: string;
	messageContent?: string;
	type?: string;
	messageType?: string;
	timestamp?: number;
	createdAt?: number;
	isRead?: boolean;
	read?: boolean;
	status?: string;
	conversationId?: string;
	seqId?: number | string;
	msgType?: number;
}

function normalizeMessage(raw: ServerMessage): ChatMessage {
	const senderId = String(raw.fromUserId ?? raw.fromId ?? raw.senderId ?? "");
	const receiverId = String(raw.toUserId ?? raw.toId ?? raw.receiverId ?? "");
	let type = String(raw.type ?? raw.messageType ?? "text");
	if (raw.msgType === 2) type = "image";
	else if (type === "MESSAGE_TYPE_IMAGE") type = "image";
	else if (type === "MESSAGE_TYPE_TEXT") type = "text";
	const ts =
		coerceTimestampMs(
			raw.timestamp ??
				raw.createdAt ??
				(raw as Record<string, unknown>).created_at,
		) || Date.now();
	return {
		id: String(raw.id ?? raw.messageId ?? ""),
		senderId,
		receiverId,
		content: String(raw.content ?? raw.messageContent ?? ""),
		type,
		timestamp: ts,
		read: raw.isRead === true || raw.read === true || raw.status === "read",
		conversationId: raw.conversationId,
		// seqId 是 64-bit TSID，保字串以免精度丟失、且與 WS 通道型別一致（dedup key 穩定）。
		seqId: raw.seqId != null ? String(raw.seqId) : undefined,
	};
}

/**
 * Coerce a wire timestamp (number ms / number sec / ISO string / null) to
 * a ms-epoch number. Returns 0 when the input can't be parsed — callers
 * treat 0 as "unknown time" and hide the timestamp UI. Mirrors the APP's
 * `chat_list_screen._formatTimeAgo` parsing path which accepts the same
 * mix of types.
 */
function coerceTimestampMs(raw: unknown): number {
	if (raw == null || raw === "") return 0;
	if (typeof raw === "number") {
		if (!Number.isFinite(raw) || raw <= 0) return 0;
		return raw < 1e12 ? raw * 1000 : raw;
	}
	if (typeof raw === "string") {
		const n = Number(raw);
		if (Number.isFinite(n) && n > 0) return n < 1e12 ? n * 1000 : n;
		const parsed = Date.parse(raw);
		return Number.isNaN(parsed) ? 0 : parsed;
	}
	return 0;
}

export function normalizeConversation(
	raw: Record<string, unknown>,
): Conversation {
	// GetConversations exposes the peer avatar as the flat pair `avatarKey`
	// (objectKey, SSOT) + `avatarUrl` (Gateway-composed URL); §231 — read the
	// key, buildImageUrl resolves it.
	const targetAvatarRaw = raw.avatarKey;
	return {
		targetUserId: (raw.targetUserId ?? raw.target_user_id ?? "") as
			| number
			| string,
		targetNickname: String(
			raw.targetNickname ?? raw.storeName ?? raw.nickname ?? "未知用戶",
		),
		targetAvatar:
			typeof targetAvatarRaw === "string"
				? targetAvatarRaw.trim() || undefined
				: undefined,
		logo_url:
			typeof raw.logoUrl === "string"
				? raw.logoUrl.trim() || undefined
				: undefined,
		storeCode:
			typeof raw.storeCode === "string"
				? raw.storeCode.trim() || undefined
				: undefined,
		lastMessage: String(raw.lastMessage ?? raw.last_message ?? ""),
		lastMessageTime: coerceTimestampMs(
			raw.lastMessageTime ??
				raw.last_message_time ??
				raw.lastMessageAt ??
				raw.last_message_at ??
				raw.updatedAt ??
				raw.updated_at,
		),
		lastMessageType: (raw.lastMessageType ?? raw.last_message_type) as
			| string
			| undefined,
		unreadCount: Number(raw.unreadCount ?? raw.unread_count ?? 0),
	};
}

function pickRecordList(data: unknown): Record<string, unknown>[] {
	if (Array.isArray(data)) return data as Record<string, unknown>[];
	if (!data || typeof data !== "object") return [];
	const obj = data as Record<string, unknown>;
	for (const key of [
		"conversations",
		"records",
		"items",
		"list",
		"rows",
		"data",
	] as const) {
		const value = obj[key];
		if (Array.isArray(value)) return value as Record<string, unknown>[];
	}
	if (obj.data && typeof obj.data === "object") return pickRecordList(obj.data);
	return [];
}

/* ===== Send (HTTP — primary path; WS only for live receive) ===== */

export interface SendMessageBody {
	toUserId: number | string;
	content: string;
	type?: string;
	extra?: Record<string, unknown>;
}

export async function sendMessage(body: SendMessageBody): Promise<{
	messageId: string;
	content?: string;
	timestamp: number;
	seqId?: string;
}> {
	const env = await authedFetch<{
		messageId?: number | string;
		content?: string;
		timestamp?: number | string;
		seqId?: number | string;
	}>(`${BASE}/messages`, {
		method: "POST",
		json: {
			toUserId: String(body.toUserId),
			content: body.content,
			type: body.type ?? "text",
			extra: body.extra,
		},
	});
	return {
		messageId: String(env.data?.messageId ?? ""),
		// For image messages the server finalizes the staged token and echoes
		// the permanent CDN URL here — callers swap it in for the temporary
		// preview URL (the tmp object is deleted on finalize).
		content: env.data?.content != null ? String(env.data.content) : undefined,
		timestamp: coerceTimestampMs(env.data?.timestamp) || Date.now(),
		// The room-monotonic seqId the timeline sorts by. The sender gets no WS
		// self-echo, so this is the only place an own-send acquires its seqId —
		// drop it and the message pins under every received message. String to
		// preserve the 64-bit TSID precision.
		seqId: env.data?.seqId != null ? String(env.data.seqId) : undefined,
	};
}

/* ===== History + lists ===== */

export async function fetchChatHistory({
	targetId,
	beforeId,
	pageSize = 50,
}: {
	targetId: number | string;
	beforeId?: number | string;
	pageSize?: number;
}): Promise<ChatMessage[]> {
	const qs = new URLSearchParams({
		pageSize: String(pageSize),
	});
	if (beforeId != null && beforeId !== "") qs.set("beforeId", String(beforeId));
	const env = await authedFetch<{
		messages?: ServerMessage[];
	}>(`${BASE}/history/${encodeURIComponent(String(targetId))}?${qs}`, {
		throwOnHttpError: false,
		throwOnEnvelopeError: false,
	});
	if (env.code === 404) {
		return [];
	}
	if (env.code != null && env.code !== 0 && env.code !== 200) {
		throw new ApiClientError(
			env.message || env.msg || `code ${env.code}`,
			env.code,
			env.code,
			env,
		);
	}
	const raw =
		env.data?.messages ?? (pickRecordList(env.data) as ServerMessage[]);
	return raw.map(normalizeMessage);
}

export async function fetchConversations({
	page = 1,
	pageSize = 50,
}: {
	page?: number;
	pageSize?: number;
} = {}): Promise<Conversation[]> {
	const qs = new URLSearchParams({
		page: String(page),
		pageSize: String(pageSize),
	});
	const env = await authedFetch<unknown>(`${BASE}/conversations?${qs}`, {
		throwOnEnvelopeError: false,
	});
	if (env.code === 404) return [];
	if (env.code != null && env.code !== 0 && env.code !== 200) {
		throw new ApiClientError(
			env.message || env.msg || `code ${env.code}`,
			env.code,
			env.code,
			env,
		);
	}
	const list = pickRecordList(env.data);
	return list.map(normalizeConversation);
}

export async function fetchUnreadChatCount(): Promise<number> {
	const env = await authedFetch<{ unread?: number; count?: number }>(
		`${BASE}/unread`,
		{ throwOnHttpError: false },
	);
	return Number(env.data?.unread ?? env.data?.count ?? 0);
}

/* ===== Read receipts ===== */

export async function markMessagesRead(
	messageIds: Array<number | string>,
): Promise<void> {
	if (messageIds.length === 0) return;
	await authedFetch(`${BASE}/messages`, {
		method: "PATCH",
		json: { messageIds },
	});
}

export async function markConversationRead(
	targetId: number | string,
): Promise<void> {
	await authedFetch(
		`${BASE}/conversations/${encodeURIComponent(String(targetId))}`,
		{
			method: "PATCH",
			throwOnHttpError: false,
		},
	);
}

export async function markAllChatRead(): Promise<void> {
	await authedFetch(`${BASE}/conversations`, { method: "PATCH" });
}

/* ===== Online users ===== */

export async function fetchOnlineUsers(): Promise<number[]> {
	const env = await authedFetch<{ users?: number[] } | number[]>(
		`${BASE}/online`,
		{ throwOnHttpError: false },
	);
	const d = env.data;
	if (Array.isArray(d)) return d.map(Number);
	return ((d as { users?: number[] })?.users ?? []).map(Number);
}
