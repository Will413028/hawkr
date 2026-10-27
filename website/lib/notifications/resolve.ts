import type { RoleSlot } from "@/lib/auth/active-role";
import { normalizeWebHref } from "@/lib/navigation/web-href";
import type { NotificationItem } from "./types";

export interface ResolvedNotification {
	title: string;
	body?: string;
	href?: string;
	orderNo?: string;
	buyerName?: string;
}

const ORDER_REF_TYPES = new Set([
	"order",
	"orders",
	"order_update",
	"seller_order",
	"buyer_order",
]);

function pickString(
	source: Record<string, unknown> | undefined,
	keys: string[],
): string {
	if (!source) return "";
	for (const key of keys) {
		const value = source[key];
		if (value == null) continue;
		const text = String(value).trim();
		if (text) return text;
	}
	return "";
}

function notificationData(item: NotificationItem): Record<string, unknown> {
	return (item.data ?? {}) as Record<string, unknown>;
}

function isOrderNotification(item: NotificationItem): boolean {
	const type = String(item.type ?? "").toLowerCase();
	const refType = String(item.refType ?? "").toLowerCase();
	return (
		type === "order" ||
		type.startsWith("order_") ||
		type.includes("shipment") ||
		type.includes("payment") ||
		ORDER_REF_TYPES.has(refType)
	);
}

// 只認聊天私訊；後端 `message` 是「留言」(comment) 不是聊天，刻意不納入。
const CHAT_REF_TYPES = new Set([
	"chat",
	"chat_message",
	"conversation",
	"dm",
	"private_message",
]);

export function isChatNotification(item: NotificationItem): boolean {
	const type = String(item.type ?? "").toLowerCase();
	const refType = String(item.refType ?? "").toLowerCase();
	return (
		type === "chat" || type.startsWith("chat") || CHAT_REF_TYPES.has(refType)
	);
}

/* ── 動態貼文 / 留言通知 ────────────────────────────────────────────
 * 貼文被按讚 / 被留言 / 留言被回覆。買家面跳 `/feed?post=`、賣家面跳
 * `/seller-center/posts?post=`，帶 `&comment=` 讓詳情頁捲到並高亮該則留言。
 * 聊天已在 `isChatNotification` 先攔掉，故這裡的 "comment" 一律是動態留言。 */
const POST_REF_TYPES = new Set([
	"post",
	"posts",
	"feed_post",
	"comment",
	"post_comment",
	"comment_reply",
	"reply",
]);

export function isPostNotification(item: NotificationItem): boolean {
	const type = String(item.type ?? "").toLowerCase();
	const refType = String(item.refType ?? "").toLowerCase();
	return (
		type.startsWith("post_") ||
		type === "post" ||
		type.includes("comment") ||
		type === "seller_post" ||
		POST_REF_TYPES.has(refType)
	);
}

/** Feed post id for a post/comment notification. When `refType` is the post
 *  itself `refId` is the post id; when it's a comment the post id only lives
 *  in `data`. Returns "" → caller leaves the row non-clickable. */
function resolvePostId(item: NotificationItem): string {
	const data = notificationData(item);
	const fromData = pickString(data, [
		"postId",
		"post_id",
		"feedId",
		"feed_id",
		"targetPostId",
		"target_post_id",
	]);
	if (fromData) return fromData;
	const refType = String(item.refType ?? "").toLowerCase();
	if (
		(refType === "post" || refType === "posts" || refType === "feed_post") &&
		item.refId != null
	) {
		return String(item.refId).trim();
	}
	return "";
}

/** The comment / reply to highlight. When `refType` is a comment `refId` is it;
 *  otherwise read the explicit id from `data`. Optional — without it the detail
 *  page just opens at the comment section. */
function resolveCommentId(item: NotificationItem): string {
	const data = notificationData(item);
	const fromData = pickString(data, [
		"commentId",
		"comment_id",
		"replyId",
		"reply_id",
		"targetCommentId",
		"target_comment_id",
	]);
	if (fromData) return fromData;
	const refType = String(item.refType ?? "").toLowerCase();
	if (
		(refType === "comment" ||
			refType === "post_comment" ||
			refType === "comment_reply" ||
			refType === "reply") &&
		item.refId != null
	) {
		return String(item.refId).trim();
	}
	return "";
}

function resolvePostHref(
	item: NotificationItem,
	role: RoleSlot,
): string | undefined {
	const postId = resolvePostId(item);
	if (!postId) return webHref(item.actionUrl);
	const base = role === "seller" ? "/seller-center/posts" : "/feed";
	const params = new URLSearchParams({ post: postId });
	const commentId = resolveCommentId(item);
	if (commentId) params.set("comment", commentId);
	return `${base}?${params.toString()}`;
}

/** Only return a navigable web href. Backend sometimes packs an app deep link
 *  (`hawkrbuyer://…`) into `actionUrl`, which a browser can't open — drop
 *  it so we fall through to a degraded (non-clickable) row instead of a 404. */
function webHref(url: string | undefined): string | undefined {
	return normalizeWebHref(url, {
		allowExternal: true,
	});
}

/**
 * The other party's account id, used to deep-link a chat notification to its
 * thread (`/account/messages/{id}` 買家 ｜ `/seller-center/messages/{id}` 賣家).
 * Tries the explicit id fields first, then `refId`, then parses a `roomId` of
 * the form `b{buyerId}_s{sellerId}` against the viewer's role. Returns "" when
 * nothing usable is present — the caller then leaves the row non-clickable
 * rather than risk a wrong link.
 */
function resolveChatTargetId(item: NotificationItem, role: RoleSlot): string {
	const data = notificationData(item);
	const fromData = pickString(data, [
		"targetUserId",
		"target_user_id",
		"targetId",
		"target_id",
		"senderId",
		"sender_id",
		"fromAccountId",
		"from_account_id",
		"fromId",
		"from_id",
		"from",
		"peerId",
		"peer_id",
		"sellerAccountId",
		"seller_account_id",
	]);
	if (/^\d+$/.test(fromData)) return fromData;
	const refId = String(item.refId ?? "").trim();
	if (/^\d+$/.test(refId)) return refId;
	const roomId = pickString(data, ["roomId", "room_id"]) || refId;
	const room = /b(\d+)_s(\d+)/i.exec(roomId);
	if (room) return role === "seller" ? room[1] : room[2];
	return "";
}

function resolveChatHref(
	item: NotificationItem,
	role: RoleSlot,
): string | undefined {
	const targetId = resolveChatTargetId(item, role);
	if (!targetId) return undefined;
	const id = encodeURIComponent(targetId);
	// 賣家聊天是工作區的半彈窗（非整頁路由）：帶 ?chat= 讓工作區自動開抽屜。
	// 買家聊天是整頁含側欄版面，直接進對話路由即可。
	return role === "seller"
		? `/seller-center/messages?chat=${id}`
		: `/account/messages/${id}`;
}

function resolveOrderNo(item: NotificationItem): string {
	const data = notificationData(item);
	const fromData = pickString(data, [
		"orderNo",
		"order_no",
		"orderSn",
		"order_sn",
		"orderId",
		"order_id",
		"sn",
	]);
	if (fromData) return fromData;
	if (isOrderNotification(item) && item.refId != null) {
		return String(item.refId).trim();
	}
	return "";
}

function resolveBuyerName(item: NotificationItem): string {
	const data = notificationData(item);
	return pickString(data, [
		"buyerName",
		"buyer_name",
		"customerName",
		"customer_name",
		"receiverName",
		"receiver_name",
		"memberName",
		"member_name",
		"nickname",
		"name",
	]);
}

function resolveBuyerId(item: NotificationItem): string {
	const data = notificationData(item);
	return pickString(data, [
		"buyerAccountId",
		"buyer_account_id",
		"customerAccountId",
		"customer_account_id",
		"buyerId",
		"buyer_id",
		"customerId",
		"customer_id",
	]);
}

function hasBuyerCopy(text: string, buyerName: string): boolean {
	if (!text || !buyerName) return false;
	return text.includes(buyerName) || text.includes("買家");
}

function appendMeta(body: string | undefined, meta: string): string {
	const current = body?.trim() ?? "";
	if (!current) return meta;
	if (current.includes(meta)) return current;
	return `${current}\n${meta}`;
}

/* ── 58 號電子發票通知 ───────────────────────────────────────────────
 * 到得了前端的 `item.data` 只有 `refType` 與 `refId`（＝`order_no`）兩把 key
 * （58 §6.3.9 的 `data_fields` 寫死那兩把）⇒ **不要**去取 `complaint_id` /
 * `invoice_operation_id`，那會產出 `/…/jobs/undefined`（58A §B-2.0 T14）。
 *
 * 賣家實際收得到六則。本波接得住的是這幾種：
 *   P1  invoice.issued.seller            → 訂單詳情的發票卡（S07）
 *   P2  invoice.issue_failed.seller      → 同上（卡內再走 latestJobId 去 S08）
 *   P7  invoice.operation.review_required.seller → 同上
 *   P8  invoice.complaint.created.seller → 買家發票申訴列表（S09）
 *   P10 invoice.complaint.overdue.seller → 同上
 *   P11 invoice.seller_tax_profile.status_changed.seller → 稅務身分（S02）
 *
 * 🔴 P8／P10／P11 都必須在**訂單分支之前**攔截，理由不同但症狀相同：
 *   - P11 的 `data_fields` 是 NULL ⇒ 它沒有 order_no，落到訂單分支會算出
 *     `/seller-center/orders?focus=`（空 focus）（58A §B-2.0 T15）
 *   - P8／P10 **有** order_no，落到訂單分支不會壞掉、只會導錯地方（訂單詳情而不是
 *     申訴列表）——這種錯更難發現，因為連結是可點的
 *
 * 🔴 P8／P10 一律導**列表**不導詳情：`item.data` 只有 `refType`／`refId` 兩把 key
 * （58 §6.3.9 的 `data_fields` 寫死），拿不到 `complaint_id`；自己拼一個會產出
 * `/…/appeals/undefined`（§B-2.0 T14）。
 */
function isInvoiceNotification(item: NotificationItem): boolean {
	return String(item.type ?? "")
		.toLowerCase()
		.startsWith("invoice.");
}

const TAX_STATUS_HREF = "/seller-center/invoices/tax-status";
const APPEALS_HREF = "/seller-center/invoices/appeals";

/** 賣家面的發票通知 → href；本波接不住的回 `undefined`（維持不可點）。 */
function resolveSellerInvoiceHref(
	item: NotificationItem,
	orderNo: string,
): string | undefined {
	const type = String(item.type ?? "").toLowerCase();
	// P11：靜態 href，只依 `item.type` 判——它沒有 order_no。
	if (type.includes("seller_tax_profile")) return TAX_STATUS_HREF;
	// P8／P10：靜態 href。它們**有** order_no，所以必須在訂單分支之前攔截，否則
	// 會被導去訂單詳情——連結可點、但去錯地方。詳情頁到不了（拿不到 complaint_id）。
	if (type.includes("complaint")) return APPEALS_HREF;
	// P1 / P2 / P7：訂單詳情的發票卡。
	if (orderNo) {
		return `/seller-center/orders/${encodeURIComponent(orderNo)}`;
	}
	return undefined;
}

export function resolveNotification(
	item: NotificationItem,
	role: RoleSlot,
): ResolvedNotification {
	const title = item.title || (isOrderNotification(item) ? "新訂單" : "新通知");
	const orderNo = resolveOrderNo(item);
	const buyerName = resolveBuyerName(item);
	const buyerId = resolveBuyerId(item);
	const isOrder = isOrderNotification(item);
	// 聊天私訊：跳轉對應聊天室。優先用解析出的 thread href，而不是 actionUrl
	// （後端可能塞 app deep link，瀏覽器開不了）；解不出 target 時不給 href。
	// 動態貼文 / 留言回覆：跳到貼文詳情並捲到該則留言（見 resolvePostHref）。
	const href = isChatNotification(item)
		? resolveChatHref(item, role)
		: isPostNotification(item)
			? resolvePostHref(item, role)
			: // 發票類必須在訂單分支之前攔截 —— 見 `resolveSellerInvoiceHref` 的檔頭。
				isInvoiceNotification(item) && role === "seller"
				? resolveSellerInvoiceHref(item, orderNo)
				: webHref(item.actionUrl) ||
					(orderNo
						? role === "seller"
							? `/seller-center/orders?focus=${encodeURIComponent(orderNo)}`
							: `/account/orders/${encodeURIComponent(orderNo)}`
						: undefined);

	let body = item.body;
	if (isOrder && role === "seller") {
		if (buyerName && !hasBuyerCopy(`${title}\n${body ?? ""}`, buyerName)) {
			body = appendMeta(body, `買家：${buyerName}`);
		} else if (!buyerName && buyerId && !String(body ?? "").includes(buyerId)) {
			body = appendMeta(body, `買家 ID：${buyerId}`);
		}
	}

	return {
		title,
		body,
		href,
		orderNo,
		buyerName,
	};
}
