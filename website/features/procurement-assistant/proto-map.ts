import { timestampDate } from "@bufbuild/protobuf/wkt";
import {
	type Activity,
	BlockKind,
	type Candidate,
	type GetCandidateSearchResponse,
	type GetSnapshotResponse,
	InterruptKind,
	ItemResolutionKind,
	type OpenConfirmation,
	type ClarifyQuestion as ProtoClarifyQuestion,
	type RecordedField as ProtoRecordedField,
	type RejectedField as ProtoRejectedField,
	type Slots as ProtoSlots,
	type PublishedRfq,
	RequestStatus,
	RunStatus,
	type RunSummary,
	RunType,
	SlotKey,
	type Step,
	StepStatus,
	type TurnResult,
} from "@hawkr/api/gen/hawkr/procurement/v1alpha1/procurement_pb";
import { formatDate } from "@hawkr/api/proto/date";
import { imageUri } from "@/lib/media/image";
import type { AgentActivityEvent } from "./activity";
import type { PublicNarrativeBlock, PublicNarrativeKind } from "./narrative";
import { type AgentStep, type AgentStepStatus, isPublicAgentStepCode } from "./steps";
import type {
	ClarifyQuestion,
	ProcurementCandidate,
	ProcurementCandidateSearch,
	ProcurementInterrupt,
	ProcurementRequestStatus,
	ProcurementRunStatus,
	ProcurementRunView,
	ProcurementSlots,
	ProcurementSnapshot,
	ProcurementTurnResult,
	PublishedRfqPublication,
	RecordedField,
	RejectedField,
} from "./types";

/** 需求欄位在畫面層的代碼（沿用既有的 snake_case 字串）。 */
const SLOT_KEY_CODE: Partial<Record<SlotKey, string>> = {
	[SlotKey.ITEM]: "item",
	[SlotKey.ITEM_RESOLUTION]: "item_resolution",
	[SlotKey.QUANTITY]: "quantity",
	[SlotKey.UNIT]: "unit",
	[SlotKey.PACKAGING]: "packaging",
	[SlotKey.BUDGET_PER_UNIT]: "budget_per_unit",
	[SlotKey.DELIVERY_CITY]: "delivery_city",
	[SlotKey.DELIVERY_DATE]: "delivery_date",
	[SlotKey.EXACT_MATCH_REQUIRED]: "exact_match_required",
	[SlotKey.ITEM_DISAMBIGUATION]: "crop_disambiguation",
};

const SLOT_KEY_BY_CODE = new Map<string, SlotKey>(
	Object.entries(SLOT_KEY_CODE).map(([key, code]) => [
		code as string,
		Number(key) as SlotKey,
	]),
);

export function slotKeyCode(key: SlotKey): string {
	return SLOT_KEY_CODE[key] ?? "";
}

/** 畫面層的欄位代碼轉回契約的 SlotKey；不認得的代碼回 UNSPECIFIED（由伺服器拒絕，不在這裡猜）。 */
export function slotKeyFromCode(code: string): SlotKey {
	return SLOT_KEY_BY_CODE.get(code) ?? SlotKey.UNSPECIFIED;
}

export function slotsFromProto(
	slots: ProtoSlots | undefined,
): ProcurementSlots {
	if (!slots) return {};
	const resolution = slots.itemResolution;
	const kind =
		resolution?.kind === ItemResolutionKind.CATEGORY_CONFIRMED
			? "CATEGORY_CONFIRMED"
			: resolution?.kind === ItemResolutionKind.BUYER_DESCRIBED
				? "BUYER_DESCRIBED"
				: undefined;
	return {
		item: slots.item,
		quantity: slots.quantity,
		unit: slots.unit,
		packaging: slots.packaging,
		budgetPerUnit:
			slots.budgetPerUnit === undefined
				? undefined
				: Number(slots.budgetPerUnit),
		deliveryCity: slots.deliveryCity,
		deliveryDate: formatDate(slots.deliveryDate) || undefined,
		exactMatchRequired: slots.exactMatchRequired,
		itemResolution:
			kind && resolution && resolution.displayValue !== ""
				? {
						kind,
						displayValue: resolution.displayValue,
						label: resolution.label || undefined,
					}
				: undefined,
	};
}

export function questionFromProto(
	question: ProtoClarifyQuestion | undefined,
): ClarifyQuestion | null {
	if (!question) return null;
	const options = (list: ProtoClarifyQuestion["options"]) =>
		list.map((o) => ({ optionId: o.optionId, label: o.label }));
	return {
		slotKey: slotKeyCode(question.slotKey),
		prompt: question.prompt,
		options: options(question.options),
		unitOptions:
			question.unitOptions.length > 0
				? options(question.unitOptions)
				: undefined,
		allowFreeText: question.allowFreeText,
		canSkip: question.canSkip,
	};
}

export function recordedFromProto(
	fields: readonly ProtoRecordedField[],
): RecordedField[] {
	return fields.flatMap((f) => {
		const key = slotKeyCode(f.key);
		if (!key || !f.value) return [];
		return [{ key, label: f.label || key, value: f.value }];
	});
}

export function rejectedFromProto(
	fields: readonly ProtoRejectedField[],
): RejectedField[] {
	return fields.map((f) => ({ key: slotKeyCode(f.key), reason: f.reason }));
}

const BLOCK_KIND_CODE: Partial<Record<BlockKind, PublicNarrativeKind>> = {
	[BlockKind.UNDERSTOOD]: "UNDERSTOOD",
	[BlockKind.CONTEXT]: "CONTEXT",
	[BlockKind.RATIONALE]: "RATIONALE",
	[BlockKind.WARNING]: "WARNING",
	[BlockKind.NEXT_STEP]: "NEXT_STEP",
	[BlockKind.COMPLETE]: "COMPLETE",
};

function blocksFromProto(
	blocks: TurnResult["blocks"],
): PublicNarrativeBlock[] | undefined {
	const out = blocks.flatMap((b) => {
		const kind = BLOCK_KIND_CODE[b.kind];
		return kind ? [{ kind, text: b.text }] : [];
	});
	return out.length > 0 ? out : undefined;
}

function interruptKindCode(kind: InterruptKind): string {
	return kind === InterruptKind.CLARIFICATION ? "CLARIFICATION" : "";
}

export function interruptFromProto(
	id: string,
	kind: InterruptKind,
): ProcurementInterrupt {
	return { interruptId: id, reason: interruptKindCode(kind) };
}

export function turnResultFromProto(result: TurnResult): ProcurementTurnResult {
	return {
		requestId: result.requestId,
		revisionId: result.revisionId,
		conversationId: result.conversationId || undefined,
		runId: result.runId || undefined,
		slots: slotsFromProto(result.slots),
		rejected: rejectedFromProto(result.rejected),
		missingRequired: result.missingRequired.map(slotKeyCode),
		reply: result.reply,
		blocks: blocksFromProto(result.blocks),
		textFallback: result.textFallback || undefined,
		question: questionFromProto(result.question),
		recordedThisTurn: recordedFromProto(result.recordedThisTurn),
		interrupt: result.interrupt
			? interruptFromProto(result.interrupt.interruptId, result.interrupt.kind)
			: null,
	};
}

export function runStatusCode(status: RunStatus): ProcurementRunStatus {
	switch (status) {
		case RunStatus.RUNNING:
			return "RUNNING";
		case RunStatus.WAITING_FOR_INPUT:
			return "WAITING_FOR_INPUT";
		case RunStatus.CANCEL_REQUESTED:
			return "CANCEL_REQUESTED";
		case RunStatus.SUCCEEDED:
			return "SUCCEEDED";
		case RunStatus.FAILED:
			return "FAILED";
		case RunStatus.CANCELLED:
			return "CANCELLED";
		default:
			return "QUEUED";
	}
}

export function runViewFromProto(run: RunSummary): ProcurementRunView {
	return {
		runId: run.runId,
		runType:
			run.runType === RunType.CANDIDATE_SEARCH
				? "CANDIDATE_SEARCH"
				: run.runType === RunType.CLARIFICATION_RESUME
					? "CLARIFICATION_RESUME"
					: "TURN",
		status: runStatusCode(run.status),
		terminal: run.terminal,
	};
}

export function openConfirmationFromProto(
	confirmation: OpenConfirmation | undefined,
) {
	if (!confirmation) return null;
	return {
		...interruptFromProto(confirmation.interruptId, confirmation.kind),
		question: questionFromProto(confirmation.question),
		processing: confirmation.processing,
	};
}

export function publishedRfqFromProto(
	rfq: PublishedRfq | undefined,
): PublishedRfqPublication | null {
	if (!rfq || rfq.publicationId === "") return null;
	return {
		publicationId: rfq.publicationId,
		version: rfq.version,
		publishedAt: rfq.publishTime
			? timestampDate(rfq.publishTime).toISOString()
			: "",
		recipientCount: rfq.recipientCount,
		queuedCount: rfq.queuedCount,
		sellerFacingHash: rfq.sellerFacingHash,
	};
}

export function timestampIso(
	ts: Parameters<typeof timestampDate>[0] | undefined,
): string {
	return ts ? timestampDate(ts).toISOString() : "";
}

export function candidateFromProto(
	candidate: Candidate,
	closedStoreName: string,
): ProcurementCandidate {
	const store = candidate.store;
	return {
		candidateId: candidate.id,
		displayName: store?.name?.trim() || closedStoreName,
		matchedFacts: candidate.matchedFacts.map((f) => ({
			key: slotKeyCode(f.slotKey),
			label: f.label,
		})),
		unverifiableFacts: candidate.unverifiableFacts.map((f) => ({
			key: slotKeyCode(f.slotKey),
			reason: f.reason,
		})),
		factsAsOf: timestampIso(candidate.factsTime),
		selected: candidate.selected,
		storeId: store?.id || undefined,
		storeCode: store?.storeCode || undefined,
		logoUrl: imageUri(store?.logo, "avatar"),
	};
}

export function candidateSearchFromProto(
	res: GetCandidateSearchResponse,
	closedStoreName: string,
): ProcurementCandidateSearch {
	return {
		requestId: res.requestId,
		runId: res.runId,
		candidateSetId: res.candidateSetId,
		status: runStatusCode(res.status),
		failureReason: res.failureReason,
		candidates: res.candidates.map((c) =>
			candidateFromProto(c, closedStoreName),
		),
	};
}

export function requestStatusCode(
	status: RequestStatus,
): ProcurementRequestStatus {
	return status === RequestStatus.MATCHING
		? "MATCHING"
		: status === RequestStatus.REQUIREMENT_READY
			? "REQUIREMENT_READY"
			: "CLARIFYING";
}

export function snapshotFromProto(
	res: GetSnapshotResponse,
): ProcurementSnapshot {
	return {
		requestId: res.requestId,
		revisionId: res.revisionId,
		status: requestStatusCode(res.status),
		slots: slotsFromProto(res.slots),
		missingRequired: res.missingRequired.map(slotKeyCode),
		runs: res.runs.map(runViewFromProto),
		activeRunId: res.activeRunId,
		openConfirmation: openConfirmationFromProto(res.openConfirmation),
		publishedPublication: publishedRfqFromProto(res.publishedRfq),
	};
}

const STEP_STATUS_CODE: Partial<Record<StepStatus, AgentStepStatus>> = {
	[StepStatus.RUNNING]: "RUNNING",
	[StepStatus.WAITING_FOR_INPUT]: "RUNNING",
	[StepStatus.SUCCEEDED]: "SUCCEEDED",
	[StepStatus.FAILED]: "FAILED",
	[StepStatus.SKIPPED]: "SKIPPED",
};

/** 未知的步驟代碼與沒有對應畫面狀態的 status 回 null（不顯示）；伺服器的 label 不採用，固定短句由 `stepLabel` 決定。 */
export function stepFromProto(step: Step): AgentStep | null {
	if (!isPublicAgentStepCode(step.code)) return null;
	const status = STEP_STATUS_CODE[step.status];
	if (!status) return null;
	const safeArgs = { ...step.safeArgs };
	return {
		code: step.code,
		status,
		label: undefined,
		text: step.text.trim() !== "" ? step.text.trim() : undefined,
		durationMs:
			step.durationMs > BigInt(0) ? Number(step.durationMs) : undefined,
		safeArgs: Object.keys(safeArgs).length > 0 ? safeArgs : undefined,
	};
}

/** 沒有 row_id 或 code 的事件回 null；status 缺值時當作 PENDING。 */
export function activityFromProto(
	activity: Activity,
): AgentActivityEvent | null {
	if (activity.rowId === "" || activity.code === "") return null;
	return {
		rowId: activity.rowId,
		code: activity.code,
		label: activity.label,
		status:
			activity.status === StepStatus.UNSPECIFIED
				? "PENDING"
				: (StepStatus[activity.status] as AgentActivityEvent["status"]),
		parentRowId: activity.parentRowId,
		count: activity.count === undefined ? undefined : String(activity.count),
		text: activity.text,
	};
}
