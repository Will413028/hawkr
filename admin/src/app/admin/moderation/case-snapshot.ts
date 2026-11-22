import { timestampDate } from "@bufbuild/protobuf/wkt";
import type {
	CaseSnapshot,
	ChatContextMessage,
} from "@hawkr/api/gen/hawkr/admin/moderation/v1alpha1/moderation_pb";
import { formatDateTimeShort } from "@/lib/format";
import { unknownText } from "@/lib/ui-text";

export interface SnapshotField {
	label: string;
	value: string;
}

export interface SnapshotView {
	title: string;
	fields: SnapshotField[];
	/** 聊天前後文（seq 由小到大）；其他種類是空陣列。 */
	context: ContextLine[];
}

export interface ContextLine {
	seq: string;
	text: string;
	tombstone: boolean;
}

function when(value: Parameters<typeof timestampDate>[0] | undefined): string {
	return value ? formatDateTimeShort(timestampDate(value)) : "-";
}

function dash(value: string): string {
	return value === "" ? "-" : value;
}

function authorKindText(kind: string): string {
	switch (kind) {
		case "BUYER":
			return "買家";
		case "STORE":
			return "店家";
		default:
			return unknownText();
	}
}

function assets(ids: readonly string[]): SnapshotField {
	return { label: "附件", value: ids.length === 0 ? "-" : ids.join("、") };
}

/** 聊天前後文的一則：已隱藏或已抹除的是墓碑，不顯示內文與附件。 */
export function contextLine(message: ChatContextMessage): ContextLine {
	const sender = `${authorKindText(message.senderKind)} ${message.senderId}`;
	if (message.erased) {
		return {
			seq: message.seq.toString(),
			text: `${sender}：（已抹除）`,
			tombstone: true,
		};
	}
	if (message.hidden) {
		return {
			seq: message.seq.toString(),
			text: `${sender}：（已隱藏）`,
			tombstone: true,
		};
	}
	const attachments =
		message.assetIds.length > 0
			? `（附件：${message.assetIds.join("、")}）`
			: "";
	return {
		seq: message.seq.toString(),
		text: `${sender}：${message.body}${attachments}`,
		tombstone: false,
	};
}

/** 被檢舉內容快照的顯示列；依 oneof 種類各一份，不認得的種類回 undefined（呼叫端顯示未知）。 */
export function snapshotView(
	snapshot: CaseSnapshot | undefined,
): SnapshotView | undefined {
	const content = snapshot?.content;
	switch (content?.case) {
		case "social":
			return {
				title: "社群內容",
				context: [],
				fields: [
					{
						label: "作者",
						value: `${authorKindText(content.value.authorKind)} ${content.value.authorId}`,
					},
					{ label: "內容", value: dash(content.value.body) },
					assets(content.value.assetIds),
					{ label: "發表時間", value: when(content.value.authoredTime) },
				],
			};
		case "buyer":
			return {
				title: "買家檔案",
				context: [],
				fields: [
					{ label: "顯示名稱", value: dash(content.value.displayName) },
					{ label: "頭像", value: dash(content.value.avatar) },
				],
			};
		case "review":
			return {
				title: "評價",
				context: [],
				fields: [
					{ label: "作者（買家）", value: content.value.authorId },
					{ label: "店家", value: content.value.storeId },
					{ label: "商品", value: dash(content.value.productId) },
					{ label: "評分", value: content.value.rating.toString() },
					{ label: "內容", value: dash(content.value.body) },
					assets(content.value.assetIds),
					{ label: "發表時間", value: when(content.value.authoredTime) },
					{ label: "店家回覆（上下文）", value: dash(content.value.replyBody) },
				],
			};
		case "reviewReply":
			return {
				title: "評價回覆",
				context: [],
				fields: [
					{ label: "作者（店家）", value: content.value.storeId },
					{ label: "評價", value: content.value.reviewId },
					{ label: "評價的買家", value: content.value.reviewerId },
					{ label: "內容", value: dash(content.value.body) },
					{ label: "發表時間", value: when(content.value.authoredTime) },
				],
			};
		case "chat":
			return {
				title: "聊天訊息",
				fields: [
					{
						label: "寄件方",
						value: `${authorKindText(content.value.authorKind)} ${content.value.authorId}`,
					},
					{ label: "對話", value: content.value.conversationId },
					{ label: "序號", value: content.value.seq.toString() },
					{ label: "內容", value: dash(content.value.body) },
					assets(content.value.assetIds),
					{ label: "發送時間", value: when(content.value.authoredTime) },
				],
				context: content.value.context.map(contextLine),
			};
		default:
			return undefined;
	}
}
