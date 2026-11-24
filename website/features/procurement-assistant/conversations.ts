import {
	CancelDisposition,
	ConversationRole,
} from "@hawkr/api/gen/hawkr/procurement/v1alpha1/procurement_pb";
import { buyerProcurementClient } from "@/lib/connect/procurement";
import {
	recordedFromProto,
	requestStatusCode,
	runStatusCode,
	timestampIso,
} from "./proto-map";
import type { RunCancelDisposition } from "./runtime-snapshot-contract";
import type { ProcurementRunStatus, RecordedField } from "./types";

/** 列表那一列。 */
export interface AgentConversationSummary {
	conversationId: string;
	title: string;
	lastMessagePreview: string;
	/** RFC3339。 */
	updatedAt: string;
}

export interface AgentConversationPage {
	records: AgentConversationSummary[];
	hasMore: boolean;
	/** 不透明的 next_page_token，原樣帶給下一次的 `pageToken`；client 不解讀內容。 */
	nextCursorId: string | null;
	/** false 只出現在還沒讀到列表的佔位頁。 */
	available: boolean;
}

export interface AgentTranscriptMessage {
	id: string;
	role: "buyer" | "assistant";
	text: string;
	/** assistant 訊息才有；後端 transcript 直接重播當輪已記下的欄位。 */
	recordedThisTurn?: RecordedField[];
	createdAt: string;
}

export interface AgentTranscript {
	conversationId: string;
	requestId: string;
	status: string;
	title: string;
	records: AgentTranscriptMessage[];
}

/** 伺服器每頁上限；要更多靠 nextCursorId 續抓。 */
const MAX_PAGE_SIZE = 50;

export async function fetchAgentConversations(params?: {
	cursorId?: string;
	pageSize?: number;
}): Promise<AgentConversationPage> {
	const res = await buyerProcurementClient().listConversations({
		pageSize: params?.pageSize ?? MAX_PAGE_SIZE,
		pageToken: params?.cursorId ?? "",
	});
	return {
		records: res.conversations.map((c) => ({
			conversationId: c.id,
			title: c.title,
			lastMessagePreview: c.lastMessagePreview,
			updatedAt: timestampIso(c.updateTime),
		})),
		hasMore: res.nextPageToken !== "",
		nextCursorId: res.nextPageToken === "" ? null : res.nextPageToken,
		available: true,
	};
}

/** 深連結續聊用的完整對話紀錄。這支方法不分頁，一次回整份對話。 */
export async function fetchAgentTranscript(
	conversationId: string,
): Promise<AgentTranscript> {
	const res = await buyerProcurementClient().getConversationMessages({
		conversationId,
	});
	return {
		conversationId: res.conversationId || conversationId,
		requestId: res.requestId,
		status: requestStatusCode(res.status),
		title: res.title,
		records: res.messages.map((m) => {
			const recorded = recordedFromProto(m.recordedThisTurn);
			return {
				id: m.id || crypto.randomUUID(),
				role: m.role === ConversationRole.USER ? "buyer" : "assistant",
				text: m.text,
				recordedThisTurn: recorded.length > 0 ? recorded : undefined,
				createdAt: timestampIso(m.createTime),
			};
		}),
	};
}

export async function renameAgentConversation(
	conversationId: string,
	title: string,
): Promise<void> {
	await buyerProcurementClient().renameConversation({
		conversationId,
		title,
		clientRequestId: crypto.randomUUID(),
	});
}

/** 軟刪。 */
export async function deleteAgentConversation(
	conversationId: string,
): Promise<void> {
	await buyerProcurementClient().deleteConversation({
		conversationId,
		clientRequestId: crypto.randomUUID(),
	});
}

export interface ProcurementRunCancelResult {
	runId: string;
	runStatus: ProcurementRunStatus;
	/** null＝伺服器沒給可辨識的處置，畫面不挑一句最像的講。 */
	disposition: RunCancelDisposition | null;
}

const DISPOSITION_CODE: Partial<
	Record<CancelDisposition, RunCancelDisposition>
> = {
	[CancelDisposition.REQUESTED]: "REQUESTED",
	[CancelDisposition.ALREADY_CANCELLED]: "ALREADY_CANCELLED",
	[CancelDisposition.ALREADY_TERMINAL]: "ALREADY_TERMINAL",
	[CancelDisposition.TOO_LATE]: "TOO_LATE",
};

/**
 * 對一個進行中的 run 下取消命令。
 *
 * 回 OK 不等於「已取消」：run 已不在進行中也回 OK，`disposition` 說明這次命令落在哪一種結果上，
 * 其中三個代表這一輪早就到終點了，所以處置原樣交出去，由狀態擁有者決定畫面要講哪一句。
 */
export async function cancelProcurementRun(
	runId: string,
): Promise<ProcurementRunCancelResult> {
	const res = await buyerProcurementClient().cancelRun({ runId });
	return {
		runId: res.runId,
		runStatus: runStatusCode(res.runStatus),
		disposition: DISPOSITION_CODE[res.cancelDisposition] ?? null,
	};
}
