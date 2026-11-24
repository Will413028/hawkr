import type {
	RejectionReason,
	RunFailureReason,
} from "@hawkr/api/gen/hawkr/procurement/v1alpha1/procurement_pb";
import type { PublicNarrativeBlock } from "./narrative";

export type ProcurementStage =
	| "clarifying"
	| "ready"
	| "searching"
	| "candidates";

export interface RecordedField {
	key: string;
	label: string;
	value: string;
}

export interface RejectedField {
	key: string;
	reason: RejectionReason;
}

export interface ClarifyQuestion {
	slotKey: string;
	prompt: string;
	options: Array<{ optionId: string; label: string }>;
	unitOptions?: Array<{ optionId: string; label: string }>;
	allowFreeText: boolean;
	canSkip: boolean;
}

export interface CropDisambiguationOption {
	optionId: string;
	label: string;
}

export interface CropDisambiguation {
	prompt: string;
	options: CropDisambiguationOption[];
}

export interface ProcurementChatTurn {
	id: string;
	role: "buyer" | "assistant";
	text: string;
	blocks?: PublicNarrativeBlock[];
	recordedThisTurn?: RecordedField[];
	rejectedThisTurn?: RejectedField[];
	createdAt: number;
}

export interface ProcurementItemResolution {
	kind: "CATEGORY_CONFIRMED" | "BUYER_DESCRIBED";
	label?: string;
	/** 伺服器組好的顯示字串；不用 kind 與 label 自己重拼。 */
	displayValue: string;
}

export interface ProcurementSlots {
	item?: string;
	quantity?: number;
	unit?: string;
	packaging?: string;
	budgetPerUnit?: number;
	deliveryCity?: string;
	/** YYYY-MM-DD。 */
	deliveryDate?: string;
	exactMatchRequired?: boolean;
	itemResolution?: ProcurementItemResolution;
}

export type ProcurementRequestStatus =
	| "CLARIFYING"
	| "REQUIREMENT_READY"
	| "MATCHING";

export type ProcurementRunStatus =
	| "QUEUED"
	| "RUNNING"
	| "WAITING_FOR_INPUT"
	| "CANCEL_REQUESTED"
	| "SUCCEEDED"
	| "FAILED"
	| "CANCELLED";

export interface ProcurementCandidate {
	candidateId: string;
	/** 店家卡片的名稱；卡片沒有名稱（個資清除之後）時是「已關閉的店家」。 */
	displayName: string;
	matchedFacts: Array<{ key: string; label: string }>;
	unverifiableFacts: Array<{ key: string; reason: string }>;
	/** RFC3339。 */
	factsAsOf: string;
	selected: boolean;
	storeId?: string;
	storeCode?: string;
	logoUrl?: string;
}

export interface ProcurementCandidateSearch {
	requestId: string;
	runId: string;
	candidateSetId: string;
	status: ProcurementRunStatus;
	/** 沒有失敗時是 UNSPECIFIED。 */
	failureReason: RunFailureReason;
	candidates: ProcurementCandidate[];
}

export interface ProcurementInterrupt {
	interruptId: string;
	/** InterruptKind 的名稱，例如 CLARIFICATION。 */
	reason: string;
}

export interface PublishedRfqPublication {
	publicationId: string;
	version: string;
	/** RFC3339。 */
	publishedAt: string;
	recipientCount: number;
	queuedCount: number;
	/** 賣家實際收到內容的 hash；原樣帶進 AcceptSellerQuote 的 expected_seller_facing_hash。 */
	sellerFacingHash: string;
}

export interface ProcurementRunView {
	runId: string;
	runType: "TURN" | "CANDIDATE_SEARCH" | "CLARIFICATION_RESUME";
	status: ProcurementRunStatus;
	terminal: boolean;
}

export interface ProcurementSnapshot {
	requestId: string;
	revisionId: string;
	status: ProcurementRequestStatus;
	slots: ProcurementSlots;
	missingRequired: string[];
	runs: ProcurementRunView[];
	activeRunId: string;
	openConfirmation:
		| (ProcurementInterrupt & {
				question: ClarifyQuestion | null;
				/** 這個中斷已有進行中的回答 run。 */
				processing: boolean;
		  })
		| null;
	publishedPublication: PublishedRfqPublication | null;
}

/** 一輪釐清結束時的需求狀態（TurnResult）。 */
export interface ProcurementTurnResult {
	requestId: string;
	revisionId: string;
	conversationId?: string;
	runId?: string;
	slots: ProcurementSlots;
	rejected: RejectedField[];
	missingRequired: string[];
	reply: string;
	blocks?: PublicNarrativeBlock[];
	textFallback?: string;
	question: ClarifyQuestion | null;
	recordedThisTurn: RecordedField[];
	interrupt: ProcurementInterrupt | null;
}
