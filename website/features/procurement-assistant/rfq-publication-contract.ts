import type {
	ExecuteRfqPublicationResponse,
	PreviewRfqPublicationResponse,
} from "@hawkr/api/gen/hawkr/procurement/v1alpha1/procurement_pb";
import { timestampIso } from "./proto-map";

/**
 * 本地 UI 的受眾身分：用來判斷「手上這份核准，是不是針對現在畫面上的這份名單發的」。
 * 不是 server 的 audienceHash，只活在瀏覽器記憶體裡，不上 wire。
 * 有序：audienceHash 涵蓋收件人順序，先 sort 再比會把真實差異抹掉。
 * 存在這裡而不是元件內，是為了讓「比較用的身分」與「送出去的名單」只有一個定義。
 */
export function publicationAudienceIdentityOf(
	candidateSetId: string,
	candidateIds: readonly string[],
): string {
	return JSON.stringify([candidateSetId, [...candidateIds]]);
}

export interface RfqPublicationPreviewCommand {
	/** 預覽的 client_request_id；同一次動作的重試沿用同一個。 */
	clientRequestId: string;
	expectedRevisionId: string;
	buyerRequestedRange: {
		lowerBoundAmount: string;
		upperBoundAmount: string;
	};
	buyerNote: string;
	candidateSetId: string;
	/** 順序有意義：audienceHash 涵蓋它，換順序等於換受眾。 */
	selectedCandidateIds: string[];
	/** YYYY-MM-DD；沒設就不設期限。 */
	responseDeadline?: string;
}

/** 不可變、只能執行一次、會到期的發布核准。 */
export interface RfqPublicationApproval {
	approvalId: string;
	approvalVersion: string;
	audienceHash: string;
	sellerFacingPayloadHash: string;
	/** RFC3339。 */
	expiresAt: string;
	recipientCount: number;
}

export function approvalFromProto(
	res: PreviewRfqPublicationResponse,
): RfqPublicationApproval | null {
	if (!res.approvalId || !res.approvalVersion || !res.sellerFacingPayloadHash)
		return null;
	return {
		approvalId: res.approvalId,
		approvalVersion: res.approvalVersion,
		audienceHash: res.audienceHash,
		sellerFacingPayloadHash: res.sellerFacingPayloadHash,
		expiresAt: timestampIso(res.expireTime),
		recipientCount: res.recipientCount,
	};
}

export interface RfqPublicationExecuteCommand {
	clientRequestId: string;
	expectedRevisionId: string;
	approval: RfqPublicationApproval;
}

export interface RfqPublicationResult {
	publicationId: string;
	publicationVersion: string;
	audienceHash: string;
	/** 賣家實際收到內容的 hash；原樣帶進接受報價的比對欄位。 */
	sellerFacingPayloadHash: string;
	publishedAt: string;
	replayed: boolean;
}

export function publicationResultFromProto(
	res: ExecuteRfqPublicationResponse,
): RfqPublicationResult | null {
	if (!res.publicationId || !res.publicationVersion) return null;
	return {
		publicationId: res.publicationId,
		publicationVersion: res.publicationVersion,
		audienceHash: res.audienceHash,
		sellerFacingPayloadHash: res.sellerFacingPayloadHash,
		publishedAt: timestampIso(res.publishTime),
		replayed: res.replayed,
	};
}
