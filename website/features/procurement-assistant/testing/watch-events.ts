import { create, type MessageInitShape } from "@bufbuild/protobuf";
import {
	GetConversationMessagesResponseSchema,
	GetSnapshotResponseSchema,
	InterruptKind,
	RequestStatus,
	RunFailureReason,
	RunStatus,
	SlotKey,
	SubmitTurnResponseSchema,
	TurnResultSchema,
	type WatchRunResponse,
	WatchRunResponseSchema,
} from "@hawkr/api/gen/hawkr/procurement/v1alpha1/procurement_pb";

export const REQUEST_ID = "req-1";
export const CONVERSATION_ID = "conv-1";
export const RUN_ID = "run-1";

export function submitTurnResponse(runId = RUN_ID) {
	return create(SubmitTurnResponseSchema, {
		requestId: REQUEST_ID,
		conversationId: CONVERSATION_ID,
		runId,
	});
}

export function acceptedEvent(cursor = "c1"): WatchRunResponse {
	return create(WatchRunResponseSchema, {
		cursor,
		event: {
			case: "runAccepted",
			value: { runId: RUN_ID, conversationId: CONVERSATION_ID },
		},
	});
}

export function turnResultEvent(
	cursor: string,
	init: MessageInitShape<typeof TurnResultSchema> = {},
): WatchRunResponse {
	return create(WatchRunResponseSchema, {
		cursor,
		event: {
			case: "turnResult",
			value: {
				requestId: REQUEST_ID,
				conversationId: CONVERSATION_ID,
				revisionId: "rev-1",
				reply: "好的，請問數量？",
				missingRequired: [SlotKey.QUANTITY],
				...init,
			},
		},
	});
}

export function finishedEvent(
	cursor: string,
	status: RunStatus = RunStatus.SUCCEEDED,
	failureReason: RunFailureReason = RunFailureReason.UNSPECIFIED,
): WatchRunResponse {
	return create(WatchRunResponseSchema, {
		cursor,
		event: { case: "finished", value: { status, failureReason } },
	});
}

export function interruptInit(interruptId = "int-1") {
	return { interruptId, kind: InterruptKind.CLARIFICATION };
}

export function snapshotResponse(
	init: MessageInitShape<typeof GetSnapshotResponseSchema> = {},
) {
	return create(GetSnapshotResponseSchema, {
		requestId: REQUEST_ID,
		revisionId: "rev-1",
		status: RequestStatus.CLARIFYING,
		missingRequired: [SlotKey.QUANTITY],
		...init,
	});
}

export function transcriptResponse(
	init: MessageInitShape<typeof GetConversationMessagesResponseSchema> = {},
) {
	return create(GetConversationMessagesResponseSchema, {
		conversationId: CONVERSATION_ID,
		requestId: REQUEST_ID,
		status: RequestStatus.CLARIFYING,
		title: "蘋果 100 公斤",
		...init,
	});
}
