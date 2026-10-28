import { z } from "zod";
import { idSchema, payloadHashSchema } from "./rfq-price-contract";

// 「發給誰」是互斥的 discriminator，不是兩個各自可填的欄位（docx V.7.5）。
// 這個型別的存在理由就是讓「許願池帶著候選名單」在編譯期就寫不出來——Gateway
// 對那種 body 回 400，而 Wish Pool 用空名單假裝定向邀請是明文禁止的事。
export type RfqPublicationAudience =
	| { mode: "WISH_POOL" }
	| {
			mode: "DIRECT_RECIPIENTS";
			candidateSetId: string;
			// 順序是契約的一部分：audienceHash 涵蓋它，換順序等於換受眾。
			selectedCandidateIds: string[];
	  };

/**
 * 本地 UI 的受眾身分：用來判斷「手上這份核准，是不是針對現在畫面上的這份名單發的」。
 * ⛔ 不是 server 的 audienceHash——它由 Gateway compile，Client 不得提供或猜測
 * （#2450 Scope 2）。這個字串只活在瀏覽器記憶體裡，不上 wire、不進任何 command body。
 * ⛔ 有序：audienceHash 涵蓋收件人順序，先 sort 再比會把真實差異抹掉。
 * 存在這裡而不是元件內，是為了讓「比較用的身分」與「送出去的 audience」只有一個定義。
 */
export function publicationAudienceIdentityOf(
	candidateSetId: string,
	candidateIds: readonly string[],
): string {
	return JSON.stringify([candidateSetId, [...candidateIds]]);
}

export interface RfqPublicationPreviewCommand {
	previewClientRequestId: string;
	expectedRevisionId: string;
	confirmedRfqSnapshotId: string;
	audience: RfqPublicationAudience;
	// YYYY-MM-DD。買家沒設就不要出現在 body 裡——見 publicationPreviewBody。
	responseDeadline?: string;
}

// ⛔ 回傳型別刻意不含 audienceHash：它由 Server compile 並回傳，Client 不得提供
// 或猜測（#2450 Scope 2）。這裡少一個欄位是防線，不是遺漏。
export interface RfqPublicationPreviewBody {
	previewClientRequestId: string;
	expectedRevisionId: string;
	confirmedRfqSnapshotId: string;
	audienceMode: RfqPublicationAudience["mode"];
	candidateSetId: string;
	selectedCandidateIds: string[];
	responseDeadline?: string;
}

export function publicationPreviewBody(
	command: RfqPublicationPreviewCommand,
): RfqPublicationPreviewBody {
	const { audience, responseDeadline } = command;
	const body: RfqPublicationPreviewBody = {
		previewClientRequestId: command.previewClientRequestId,
		expectedRevisionId: command.expectedRevisionId,
		confirmedRfqSnapshotId: command.confirmedRfqSnapshotId,
		audienceMode: audience.mode,
		candidateSetId:
			audience.mode === "DIRECT_RECIPIENTS" ? audience.candidateSetId : "",
		selectedCandidateIds:
			audience.mode === "DIRECT_RECIPIENTS"
				? [...audience.selectedCandidateIds]
				: [],
	};
	// Gateway 的 responseDeadline 是 *string＋omitempty：送空字串會被讀成「有給
	// 截止日而且是空的」並寫進賣家看到的內容。沒填就不要有這個 key。
	if (responseDeadline) body.responseDeadline = responseDeadline;
	return body;
}

// preview 回應是 approvalVersion 唯一讀得到的地方（execute 拿它做 CAS），所以
// 這裡缺欄位一律 fail closed，不補值也不視為「大致成功」。
export const rfqPublicationApprovalSchema = z.object({
	approvalId: idSchema,
	approvalVersion: z.string().min(1),
	audienceHash: payloadHashSchema,
	sellerFacingPayloadHash: payloadHashSchema,
	sellerFacingPayloadJson: z.string(),
	expiresAt: z.string().min(1),
	// WISH_POOL v1 是 WISH_POOL_ALL_ACTIVE_SELLERS_V1／1；DIRECT_RECIPIENTS 兩欄
	// 為空字串／"0"，受眾大小改由 recipientCount 表達，所以這兩欄必須容許空值。
	audiencePolicyId: z.string(),
	audiencePolicyVersion: z.string(),
	// WISH_POOL 恆為 "0"（沒有名單是這個模式的定義），不能套用正整數 id 的規則。
	recipientCount: z
		.string()
		.regex(/^\d{1,9}$/, "recipient count must be a decimal string"),
});

export type RfqPublicationApproval = z.infer<
	typeof rfqPublicationApprovalSchema
>;

export interface RfqPublicationExecuteCommand {
	clientRequestId: string;
	expectedRevisionId: string;
	approval: RfqPublicationApproval;
}

export interface RfqPublicationExecuteBody {
	clientRequestId: string;
	approvalId: string;
	approvalVersion: string;
	expectedRevisionId: string;
	payloadHash: string;
}

export function publicationExecuteBody(
	command: RfqPublicationExecuteCommand,
): RfqPublicationExecuteBody {
	const { approval } = command;
	return {
		clientRequestId: command.clientRequestId,
		approvalId: approval.approvalId,
		approvalVersion: approval.approvalVersion,
		expectedRevisionId: command.expectedRevisionId,
		payloadHash: approval.sellerFacingPayloadHash,
	};
}

export const rfqPublicationResultSchema = z.object({
	publicationId: idSchema,
	publicationVersion: z.string().min(1),
	state: z.string().min(1),
	audienceMode: z.string().min(1),
	audienceHash: payloadHashSchema,
	sellerFacingPayloadHash: payloadHashSchema,
	publishedAt: z.string().min(1),
	replayed: z.boolean(),
});

export type RfqPublicationResult = z.infer<typeof rfqPublicationResultSchema>;
