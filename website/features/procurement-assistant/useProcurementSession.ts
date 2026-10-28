"use client";

import { type QueryClient, useQueryClient } from "@tanstack/react-query";
import * as React from "react";
import { ApiClientError, apiClient } from "@/lib/api/client";
import type {
	ProcurementCandidate,
	ProcurementCandidateSearch,
	ProcurementRunStatus,
	ProcurementSlots,
	ProcurementSnapshot,
} from "@/lib/api/openapi-types";
import { toast } from "@/lib/feedback/toast";
import { useQueryIdentity } from "@/lib/hooks/useQueryIdentity";
import { toBrowserHref } from "@/lib/navigation/resolve-url";
import { type QueryIdentity, queryKeys } from "@/lib/query/keys";
import type { AgentActivityEvent } from "./activity";
import { validateFreeTextAnswer } from "./answer-validation";
import {
	type AgentTranscript,
	cancelProcurementRun,
	fetchAgentTranscript,
} from "./conversations";
import {
	CONVERSATION_GONE,
	type ProcurementError,
	parseProcurementError,
	procurementErrorFromStatus,
} from "./errors";
import {
	type RfqCommandFailure,
	type RfqCommandResult,
	type RfqConfirmation,
	type RfqConfirmationCommand,
	type RfqPricePreview,
	type RfqPricePreviewCommand,
	rfqConfirmationSchema,
	rfqPricePreviewSchema,
} from "./rfq-price-contract";
import {
	publicationExecuteBody,
	publicationPreviewBody,
	type RfqPublicationApproval,
	type RfqPublicationExecuteCommand,
	type RfqPublicationPreviewCommand,
	type RfqPublicationResult,
	rfqPublicationApprovalSchema,
	rfqPublicationResultSchema,
} from "./rfq-publication-contract";
import { assistantConversationPath } from "./routes";
import {
	type CancellableRun,
	type RunCancelDisposition,
	runCancelResultSchema,
	runtimeSnapshotSchema,
	selectCancellableRun,
} from "./runtime-snapshot-contract";
import {
	applyOptimisticSelection,
	deriveStageFromStatus,
	isTerminalCandidateStatus,
	mapSlots,
	type NormalizedProcurementTurnResult,
	normalizeCandidateSelectionResult,
	normalizeProcurementTurnResult,
	type RawProcurementSlots,
	slotLabel,
} from "./session-logic";
import {
	createExclusiveMutationGuard,
	createSessionGenerationGuard,
	createSessionOperationGuard,
	type ExclusiveMutationGuard,
	type SessionGenerationGuard,
	type SessionOperation,
	type SessionOperationGuard,
} from "./session-operation";
import type { AgentStep } from "./steps";
import { monotonicNow, trackHawkrAgentFirstStepLatency } from "./telemetry";
import {
	mergeLiveActivity,
	mergeLiveStep,
	streamProcurementTurn,
	TurnStreamEventError,
	TurnStreamUnavailableError,
} from "./turn-stream";
import type {
	ClarifyQuestion,
	CropDisambiguation,
	CropDisambiguationOption,
	ProcurementChatTurn,
	ProcurementStage,
	RecordedField,
	RejectedField,
} from "./types";

const MAX_MESSAGE_RUNES = 2000;
const POLL_INTERVAL_MS = 1500;
const POLL_BACKOFF_MS = [1500, 3000, 6000] as const;
const TURN_IDEMPOTENCY_CONFLICT: ProcurementError = Object.freeze({
	code: "IDEMPOTENCY_CONFLICT",
	detail:
		"這則訊息的識別碼已綁定其他內容，系統沒有建立新的操作。請修改文字後再送出。",
});
const TURN_IDENTITY_MISMATCH: ProcurementError = Object.freeze({
	code: "TURN_IDENTITY_MISMATCH",
	detail: "伺服器回應不屬於目前這筆需求，已停止套用。請重新整理後再試。",
});
const TURN_RESULT_INVALID: ProcurementError = Object.freeze({
	code: "TURN_RESULT_INVALID",
	detail: "伺服器回應格式不完整，未套用這次結果。請保留原訊息後再試。",
});
const CANDIDATE_SELECTION_UNCONFIRMED: ProcurementError = Object.freeze({
	code: "CANDIDATE_SELECTION_UNCONFIRMED",
	detail: "無法確認這筆候選的選取狀態，請重新搜尋後再操作。",
});

function rfqCommandFailure(
	error: unknown,
	operation: "preview" | "confirmation",
): RfqCommandFailure {
	if (!(error instanceof ApiClientError)) {
		return {
			code: "NETWORK_ERROR",
			detail: "連線中斷，無法確認這次操作的結果。請用原操作重試。",
			retryable: true,
		};
	}

	const byReason: Partial<Record<string, RfqCommandFailure>> = {
		PRICE_BASIS_UNAVAILABLE: {
			code: "PRICE_BASIS_UNAVAILABLE",
			detail: "目前數量或單位無法形成可確認的價格基準。",
			retryable: false,
		},
		STALE_REVISION:
			operation === "confirmation"
				? {
						code: "PRICE_PREVIEW_STALE",
						detail: "這份價格試算已過期或不屬於目前版本，請重新試算。",
						retryable: false,
					}
				: {
						code: "STALE_REVISION",
						detail: "需求已更新，這份價格試算不再適用。",
						retryable: false,
					},
		IDEMPOTENCY_KEY_CONFLICT: {
			code: "IDEMPOTENCY_KEY_CONFLICT",
			detail: "這次操作識別碼已綁定其他內容，未建立新結果。",
			retryable: false,
		},
		REQUEST_NOT_FOUND: {
			code: "REQUEST_NOT_FOUND",
			detail: "找不到這筆採購需求，請回到對話重新確認。",
			retryable: false,
		},
		RESOURCE_NOT_FOUND: {
			code: "RESOURCE_NOT_FOUND",
			detail: "找不到這份價格試算，請重新試算後再確認。",
			retryable: false,
		},
	};
	const mapped = byReason[error.reason];
	if (mapped) return mapped;
	if (error.status === 401) {
		return {
			code: "UNAUTHORIZED",
			detail: "登入已失效，請重新登入後再試。",
			retryable: false,
		};
	}
	if (error.status === 403) {
		return {
			code: "FORBIDDEN",
			detail: "目前帳號沒有執行這項操作的權限。",
			retryable: false,
		};
	}
	if (error.status === 408 || error.status === 429) {
		return {
			code: "RATE_LIMITED",
			detail:
				error.status === 429
					? "價格服務請求過於頻繁，請稍候並用原操作重試。"
					: "價格服務逾時，無法確認這次操作的結果。請用原操作重試。",
			retryable: true,
		};
	}
	if (error.status >= 500) {
		return {
			code: "SERVER_ERROR",
			detail: "價格服務暫時忙碌，無法確認這次操作的結果。請用原操作重試。",
			retryable: true,
		};
	}
	return {
		code: "UNKNOWN_ERROR",
		detail: error.message || "目前無法完成這項操作。",
		retryable: false,
	};
}

function normalizeOptions(
	raw: unknown,
): Array<{ optionId: string; label: string; value: string }> {
	if (!Array.isArray(raw)) return [];
	return raw.flatMap((o) => {
		if (!o || typeof o !== "object") return [];
		const opt = o as Record<string, unknown>;
		if (
			typeof opt.optionId !== "string" ||
			typeof opt.label !== "string" ||
			typeof opt.value !== "string"
		)
			return [];
		return [{ optionId: opt.optionId, label: opt.label, value: opt.value }];
	});
}

function normalizeQuestion(raw: unknown): ClarifyQuestion | null {
	if (!raw || typeof raw !== "object") return null;
	const r = raw as Record<string, unknown>;
	if (typeof r.slotKey !== "string" || typeof r.prompt !== "string")
		return null;
	const unitOptions = normalizeOptions(r.unitOptions);
	return {
		slotKey: r.slotKey,
		prompt: r.prompt,
		options: normalizeOptions(r.options),
		unitOptions: unitOptions.length > 0 ? unitOptions : undefined,
		allowFreeText: r.allowFreeText !== false,
		source: r.source === "PHRASE_LLM" ? "PHRASE_LLM" : "FIXED",
		canSkip: r.canSkip === true,
	};
}

function normalizeRecorded(raw: unknown): RecordedField[] {
	if (!Array.isArray(raw)) return [];
	return raw.flatMap((item) => {
		if (!item || typeof item !== "object") return [];
		const r = item as Record<string, unknown>;
		if (typeof r.key !== "string" || typeof r.value !== "string") return [];
		return [
			{
				key: r.key,
				label: typeof r.label === "string" && r.label ? r.label : r.key,
				value: r.value,
			},
		];
	});
}

function normalizeRejected(raw: unknown): RejectedField[] {
	if (!Array.isArray(raw)) return [];
	return raw.flatMap((item) => {
		if (!item || typeof item !== "object") return [];
		const r = item as Record<string, unknown>;
		if (typeof r.key !== "string" || typeof r.reason !== "string") return [];
		return [{ key: r.key, reason: r.reason }];
	});
}

function normalizeCropDisambiguation(raw: unknown): CropDisambiguation | null {
	if (!raw || typeof raw !== "object") return null;
	const r = raw as Record<string, unknown>;
	if (typeof r.prompt !== "string") return null;
	const options = Array.isArray(r.options)
		? r.options.flatMap((o) => {
				if (!o || typeof o !== "object") return [];
				const opt = o as Record<string, unknown>;
				if (typeof opt.optionId !== "string" || typeof opt.label !== "string")
					return [];
				return [
					{
						optionId: opt.optionId,
						label: opt.label,
					},
				];
			})
		: [];
	return { prompt: r.prompt, options };
}

function normalizeOpenConfirmation(
	raw: unknown,
): { interruptId: string; reason: string } | null {
	if (!raw || typeof raw !== "object") return null;
	const r = raw as Record<string, unknown>;
	if (
		typeof r.interruptId !== "string" ||
		typeof r.confirmationKind !== "string"
	)
		return null;
	return { interruptId: r.interruptId, reason: r.confirmationKind };
}

function normalizeOpenConfirmationQuestion(
	raw: unknown,
): ClarifyQuestion | null {
	if (!normalizeOpenConfirmation(raw)) return null;
	return normalizeQuestion((raw as Record<string, unknown>).question);
}

export interface PublishedRfqPublication {
	publicationId: string;
	version: string;
	publishedAt: string;
}

function normalizePublishedPublication(
	raw: unknown,
): PublishedRfqPublication | null {
	if (!raw || typeof raw !== "object") return null;
	const r = raw as Record<string, unknown>;
	if (typeof r.publicationId !== "string" || r.publicationId === "")
		return null;
	return {
		publicationId: r.publicationId,
		version: typeof r.version === "string" ? r.version : "",
		publishedAt: typeof r.publishedAt === "string" ? r.publishedAt : "",
	};
}

interface RawSnapshot {
	requestId: string;
	revisionId: string;
	status: ProcurementSnapshot["status"];
	slots: RawProcurementSlots;
	missingRequired: string[];
	cropDisambiguation?: unknown;
	runs?: unknown;
	activeRunId?: string;
	openConfirmation?: unknown;
	publishedPublication?: unknown;
}

export interface ProcurementResumeOutcome {
	resolved: boolean;
	turnResult?: unknown;
}

interface TerminalTelemetryObservation {
	observedAt: number;
	duplicateStepSuppressedCount: number;
	operationGeneration: number;
}

interface TurnTelemetryAccumulator {
	commandInvokedAt: number;
	firstStepTracked: boolean;
	duplicateStepSuppressedCount: number;
	terminalObservedAt: number | null;
}

function terminalTelemetryObservation(
	observedAt: number,
	operationGeneration: number,
	duplicateStepSuppressedCount = 0,
): TerminalTelemetryObservation {
	return {
		observedAt,
		duplicateStepSuppressedCount,
		operationGeneration,
	};
}

/**
 * 停止鈕在畫面上的狀態。
 *
 * 刻意不是 `boolean | error`：取消回執有四個 disposition，其中三個講的是
 * 「這一輪已經到終點」而不是「已為您取消」，把它們塌成同一個成功態就會讓
 * 買家以為自己的停止作廢了一個已完成的結果（設計書明文「不得謊報取消
 * 成功」）。所以 disposition 原樣帶到畫面層。
 *
 * `stale` 是第五種、**伺服器沒講過結果**的情況：CAS 版本過期，重讀快照之後
 * 也拿不到新的 token。這時唯一誠實的話是「狀態已經變了，請重新整理確認」，
 * 不能挑一個最像的 disposition 填上去。
 */
export type RunCancelState =
	| { status: "idle" }
	| { status: "requesting" }
	| { status: "settled"; disposition: RunCancelDisposition }
	| { status: "stale" }
	| { status: "error"; detail: string };

const IDLE_CANCEL_STATE: RunCancelState = { status: "idle" };

const INVALID_CANCEL_ACK: RunCancelState = {
	status: "error",
	detail: "無法確認這次停止的結果，請重新整理確認目前狀態。",
};

const STALE_CANCEL_STATE: RunCancelState = {
	status: "stale",
	// 不編 disposition：伺服器沒有回過這次命令的結果。
};

/**
 * CAS 沒過。
 *
 * 這支端點用 409 表達「手上的 `expectedRunVersion` 已經不是現在的版本」。只認
 * 狀態碼、不押某一個 `reason` 常數：reason 字串在不同 gateway 版本之間沒有
 * 保證，押錯會讓一個「重讀一次就好」的情況被端成「停止失敗」。
 */
function isRunVersionConflict(error: unknown): boolean {
	return error instanceof ApiClientError && error.status === 409;
}

interface SessionState {
	stage: ProcurementStage;
	requestId: string | null;
	revisionId: string | null;
	conversationId: string | null;
	turns: ProcurementChatTurn[];
	justCompletedTurnId: string | null;
	restoredSummary: string | null;
	slots: ProcurementSlots;
	missingRequired: string[];
	question: ClarifyQuestion | null;
	pendingTurn: boolean;
	turnStartObserved: boolean;
	streamFallback: string | null;
	liveSteps: AgentStep[];
	liveActivity: AgentActivityEvent[];
	runId: string | null;
	candidateSet: ProcurementCandidateSearch | null;
	pollError: boolean;
	candidatesStale: boolean;
	error: ProcurementError | null;
	cropDisambiguation: CropDisambiguation | null;
	pendingInterrupt: { interruptId: string; reason: string } | null;
	publishedPublication: PublishedRfqPublication | null;
	/**
	 * 現在按下停止會停到的那一個 run（含 CAS token）。null＝**這一輪沒有可送出
	 * 的取消命令**，畫面要讓按鈕維持不可用，而不是送一個會被 400 擋下的請求。
	 */
	activeRun: CancellableRun | null;
	cancelState: RunCancelState;
	terminalTelemetry: TerminalTelemetryObservation | null;
}

export function initialState(): SessionState {
	return {
		stage: "clarifying",
		requestId: null,
		revisionId: null,
		conversationId: null,
		turns: [],
		justCompletedTurnId: null,
		restoredSummary: null,
		slots: {},
		missingRequired: [],
		question: null,
		pendingTurn: false,
		turnStartObserved: false,
		liveSteps: [],
		liveActivity: [],
		streamFallback: null,
		runId: null,
		candidateSet: null,
		pollError: false,
		candidatesStale: false,
		error: null,
		cropDisambiguation: null,
		pendingInterrupt: null,
		publishedPublication: null,
		activeRun: null,
		cancelState: IDLE_CANCEL_STATE,
		terminalTelemetry: null,
	};
}

export function restoredStateFrom(
	transcript: AgentTranscript | null,
	snapshot: RawSnapshot,
): Partial<SessionState> {
	const slots = mapSlots(snapshot.slots);
	const question = normalizeOpenConfirmationQuestion(snapshot.openConfirmation);
	const filled = (Object.keys(slots) as Array<keyof ProcurementSlots>)
		.filter((key) => slots[key] !== undefined)
		.map((key) => `${slotLabel(key)}=${String(slots[key])}`);
	const turns: ProcurementChatTurn[] = (transcript?.records ?? []).map((m) => ({
		id: m.id,
		role: m.role,
		text: m.text,
		recordedThisTurn: m.recordedThisTurn,
		createdAt: Date.parse(m.createdAt) || 0,
	}));
	return {
		requestId: snapshot.requestId,
		revisionId: snapshot.revisionId,
		conversationId: transcript?.conversationId ?? null,
		turns,
		slots,
		missingRequired: snapshot.missingRequired,
		stage: deriveStageFromStatus(snapshot.status),
		question,
		cropDisambiguation: question
			? null
			: normalizeCropDisambiguation(snapshot.cropDisambiguation),
		pendingInterrupt: normalizeOpenConfirmation(snapshot.openConfirmation),
		publishedPublication: normalizePublishedPublication(
			snapshot.publishedPublication,
		),
		runId: null,
		activeRun: null,
		cancelState: IDLE_CANCEL_STATE,
		candidateSet: null,
		pollError: false,
		candidatesStale: false,
		restoredSummary:
			turns.length === 0 && filled.length > 0
				? `已恢復先前提供的資訊：${filled.join("、")}`
				: null,
	};
}

function readCachedRestore(
	queryClient: QueryClient,
	identity: QueryIdentity,
	conversationId: string | null,
): Partial<SessionState> | null {
	if (!conversationId) return null;
	const transcript = queryClient.getQueryData<AgentTranscript>(
		queryKeys.assistant.transcript(identity, conversationId),
	);
	if (!transcript?.requestId) return null;
	const snapshot = queryClient.getQueryData<RawSnapshot>(
		queryKeys.assistant.snapshot(identity, transcript.requestId),
	);
	if (!snapshot) return null;
	return restoredStateFrom(transcript, snapshot);
}

function writeConversationUrl(conversationId: string) {
	if (typeof window === "undefined") return;
	const target = toBrowserHref(assistantConversationPath(conversationId));
	if (window.location.pathname === target) return;
	window.history.replaceState(null, "", target);
}

async function runTurnStream(
	body: { requestId?: string; message: string; clientRequestId: string },
	setState: React.Dispatch<React.SetStateAction<SessionState>>,
	isCurrent: () => boolean,
	signal: AbortSignal,
	telemetry: TurnTelemetryAccumulator,
): Promise<NormalizedProcurementTurnResult | null> {
	try {
		const result = await streamProcurementTurn(
			body,
			{
				onStartObserved: (observedAt) => {
					if (!isCurrent()) return;
					if (!telemetry.firstStepTracked) {
						telemetry.firstStepTracked = true;
						trackHawkrAgentFirstStepLatency({
							commandInvokedAt: telemetry.commandInvokedAt,
							firstStepObservedAt: observedAt,
						});
					}
					setState((s) =>
						isCurrent() && s.pendingTurn
							? { ...s, turnStartObserved: true }
							: s,
					);
				},
				onStep: (step) => {
					if (!isCurrent()) return;
					setState((s) =>
						isCurrent() && s.pendingTurn
							? {
									...s,
									turnStartObserved: true,
									liveSteps: mergeLiveStep(s.liveSteps, step),
								}
							: s,
					);
				},
				onActivity: (activity) => {
					if (!isCurrent()) return;
					setState((s) =>
						isCurrent() && s.pendingTurn
							? {
									...s,
									turnStartObserved: true,
									liveActivity: mergeLiveActivity(s.liveActivity, activity),
								}
							: s,
					);
				},
				onDuplicateStepSuppressed: () => {
					if (!isCurrent()) return;
					telemetry.duplicateStepSuppressedCount += 1;
				},
				onTerminalObserved: (observation) => {
					if (!isCurrent()) return;
					telemetry.terminalObservedAt = observation.observedAt;
					telemetry.duplicateStepSuppressedCount =
						observation.duplicateStepSuppressedCount;
				},
			},
			signal,
		);
		if (!isCurrent()) return null;
		const normalized = normalizeProcurementTurnResult(result);
		if (normalized) return normalized;
		setState((s) =>
			isCurrent()
				? {
						...s,
						streamFallback: "串流終態格式不完整",
						liveSteps: [],
						liveActivity: [],
					}
				: s,
		);
		return null;
	} catch (err) {
		if (!isCurrent()) return null;
		if (err instanceof TurnStreamEventError) throw err;
		const why =
			err instanceof TurnStreamUnavailableError
				? err.reason === "NOT_DEPLOYED"
					? "gateway／ai-service 沒有 /turns/stream"
					: "串流忙線或中途斷線"
				: `串流開場失敗（${err instanceof Error ? err.name : "未知"}）`;
		setState((s) =>
			isCurrent()
				? {
						...s,
						streamFallback: why,
						...(err instanceof TurnStreamUnavailableError
							? {}
							: { liveSteps: [], liveActivity: [] }),
					}
				: s,
		);
		return null;
	}
}

export function useProcurementSession(
	initialRequestId: string | null,
	initialConversationId: string | null = null,
) {
	const queryClient = useQueryClient();
	const queryIdentity = useQueryIdentity();

	const [state, setState] = React.useState<SessionState>(() => {
		const cached = readCachedRestore(
			queryClient,
			queryIdentity,
			initialConversationId,
		);
		return cached ? { ...initialState(), ...cached } : initialState();
	});
	const stateRef = React.useRef(state);
	React.useEffect(() => {
		stateRef.current = state;
	}, [state]);
	const operationGuardRef = React.useRef<SessionOperationGuard | null>(null);
	if (!operationGuardRef.current) {
		operationGuardRef.current = createSessionOperationGuard();
	}
	const operationGuard = operationGuardRef.current;
	const sessionGenerationGuardRef = React.useRef<SessionGenerationGuard | null>(
		null,
	);
	if (!sessionGenerationGuardRef.current) {
		sessionGenerationGuardRef.current = createSessionGenerationGuard();
	}
	const sessionGenerationGuard = sessionGenerationGuardRef.current;
	const selectionGuardRef = React.useRef<ExclusiveMutationGuard | null>(null);
	if (!selectionGuardRef.current) {
		selectionGuardRef.current = createExclusiveMutationGuard();
	}
	const selectionGuard = selectionGuardRef.current;
	const [pendingSelectionCandidateIds, setPendingSelectionCandidateIds] =
		React.useState<ReadonlySet<string>>(() => new Set());
	const invalidateCandidateSelections = React.useCallback(() => {
		selectionGuard.invalidate();
		setPendingSelectionCandidateIds(new Set());
	}, [selectionGuard]);
	React.useEffect(
		() => () => {
			operationGuard.invalidate();
			sessionGenerationGuard.invalidate();
			selectionGuard.invalidate();
		},
		[operationGuard, selectionGuard, sessionGenerationGuard],
	);

	const [restoring, setRestoring] = React.useState(
		() =>
			state.requestId === null &&
			Boolean(initialRequestId || initialConversationId),
	);

	const reset = React.useCallback(() => {
		operationGuard.invalidate();
		sessionGenerationGuard.invalidate();
		invalidateCandidateSelections();
		const nextState = initialState();
		stateRef.current = nextState;
		setState(nextState);
		setRestoring(false);
	}, [invalidateCandidateSelections, operationGuard, sessionGenerationGuard]);
	const [pollRetryToken, setPollRetryToken] = React.useState(0);

	const fetchSnapshot = React.useCallback(
		(requestId: string): Promise<RawSnapshot> =>
			queryClient.fetchQuery({
				queryKey: queryKeys.assistant.snapshot(queryIdentity, requestId),
				queryFn: async () => {
					const env = await apiClient.authed.get<RawSnapshot>(
						`/buyers/me/procurement/requests/${requestId}/snapshot`,
					);
					return env.data;
				},
				staleTime: 0,
			}),
		[queryClient, queryIdentity],
	);

	const fetchTranscript = React.useCallback(
		(conversationId: string): Promise<AgentTranscript> =>
			queryClient.fetchQuery({
				queryKey: queryKeys.assistant.transcript(queryIdentity, conversationId),
				queryFn: () => fetchAgentTranscript(conversationId),
				staleTime: 60_000,
			}),
		[queryClient, queryIdentity],
	);

	React.useEffect(() => {
		if (!initialRequestId && !initialConversationId) return;
		let cancelled = false;
		sessionGenerationGuard.invalidate();
		const restoreIdentity = sessionGenerationGuard.capture(
			stateRef.current.requestId,
		);
		const isCurrentRestore = () =>
			!cancelled &&
			sessionGenerationGuard.isCurrent(
				restoreIdentity,
				stateRef.current.requestId,
			);
		invalidateCandidateSelections();
		const restoringDifferentIdentity = initialRequestId
			? initialRequestId !== stateRef.current.requestId
			: initialConversationId !== stateRef.current.conversationId;
		if (restoringDifferentIdentity || stateRef.current.requestId === null) {
			setRestoring(true);
		}
		setState((s) =>
			sessionGenerationGuard.isCurrent(restoreIdentity, s.requestId)
				? {
						...s,
						runId: null,
						candidateSet: null,
						pollError: false,
						candidatesStale: false,
					}
				: s,
		);
		(async () => {
			try {
				let requestId = initialRequestId;
				let transcript: AgentTranscript | null = null;
				if (initialConversationId) {
					transcript = await fetchTranscript(initialConversationId);
					if (!isCurrentRestore()) return;
					requestId = transcript.requestId || null;
				}
				if (!requestId) throw new Error("transcript missing requestId");
				const snapshot = await fetchSnapshot(requestId);
				if (!isCurrentRestore()) return;
				const restored = restoredStateFrom(transcript, snapshot);
				setState((s) =>
					!cancelled &&
					sessionGenerationGuard.isCurrent(restoreIdentity, s.requestId)
						? {
								...s,
								...restored,
								turns: restored.turns?.length ? restored.turns : s.turns,
							}
						: s,
				);
			} catch {
				if (!isCurrentRestore()) return;
				setState((s) =>
					!cancelled &&
					sessionGenerationGuard.isCurrent(restoreIdentity, s.requestId)
						? {
								...s,
								error: {
									code: "RESTORE_FAILED",
									detail: "無法讀取先前的需求，請重新開始。",
								},
							}
						: s,
				);
			} finally {
				if (isCurrentRestore()) setRestoring(false);
			}
		})();
		return () => {
			cancelled = true;
		};
	}, [
		initialRequestId,
		initialConversationId,
		fetchSnapshot,
		fetchTranscript,
		invalidateCandidateSelections,
		sessionGenerationGuard,
	]);

	const refreshCurrentSnapshot =
		React.useCallback(async (): Promise<boolean> => {
			const current = stateRef.current;
			if (!current.requestId) return false;
			const requestId = current.requestId;
			const sessionIdentity = sessionGenerationGuard.capture(requestId);
			try {
				const snapshot = await fetchSnapshot(requestId);
				if (
					!sessionGenerationGuard.isCurrent(
						sessionIdentity,
						stateRef.current.requestId,
					) ||
					snapshot.requestId !== requestId
				)
					return false;
				invalidateCandidateSelections();
				setState((existing) => {
					if (
						!sessionGenerationGuard.isCurrent(
							sessionIdentity,
							existing.requestId,
						)
					)
						return existing;
					const pendingInterrupt = normalizeOpenConfirmation(
						snapshot.openConfirmation,
					);
					const rawConfirmation = snapshot.openConfirmation as
						| Record<string, unknown>
						| undefined;
					const cropDisambiguation = normalizeCropDisambiguation(
						snapshot.cropDisambiguation,
					);
					const preserveRegularQuestion =
						snapshot.requestId === existing.requestId &&
						snapshot.revisionId === existing.revisionId &&
						(snapshot.status === "CLARIFYING" ||
							snapshot.status === "REQUIREMENT_READY") &&
						snapshot.openConfirmation == null &&
						!existing.pendingInterrupt &&
						!cropDisambiguation &&
						existing.question?.slotKey !== "crop_disambiguation";
					const question =
						normalizeOpenConfirmationQuestion(snapshot.openConfirmation) ??
						((rawConfirmation?.question === undefined &&
							pendingInterrupt &&
							pendingInterrupt.interruptId ===
								existing.pendingInterrupt?.interruptId) ||
						preserveRegularQuestion
							? existing.question
							: null);
					return {
						...existing,
						revisionId: snapshot.revisionId,
						slots: mapSlots(snapshot.slots),
						missingRequired: snapshot.missingRequired,
						stage: deriveStageFromStatus(snapshot.status),
						question,
						cropDisambiguation: question ? null : cropDisambiguation,
						pendingInterrupt,
						publishedPublication: normalizePublishedPublication(
							snapshot.publishedPublication,
						),
						runId: null,
						candidateSet: null,
						pollError: false,
						candidatesStale: false,
					};
				});
				return true;
			} catch {
				toast.error("無法重新讀取最新需求，請稍後再試");
				return false;
			}
		}, [fetchSnapshot, invalidateCandidateSelections, sessionGenerationGuard]);

	const resume = React.useCallback(
		async (payload: unknown): Promise<ProcurementResumeOutcome> => {
			const current = stateRef.current;
			if (!current.requestId || !current.pendingInterrupt) {
				return { resolved: false };
			}
			const requestId = current.requestId;
			const interruptId = current.pendingInterrupt.interruptId;
			const sessionIdentity = sessionGenerationGuard.capture(requestId);
			const isCurrentInterrupt = () =>
				sessionGenerationGuard.isCurrent(
					sessionIdentity,
					stateRef.current.requestId,
				) && stateRef.current.pendingInterrupt?.interruptId === interruptId;
			const clientRequestId = crypto.randomUUID();
			try {
				const env = await apiClient.authed.post<unknown>(
					`/buyers/me/procurement/runs/${interruptId}/resume`,
					{
						json: { interruptId, clientRequestId, payload },
					},
				);
				if (!isCurrentInterrupt()) return { resolved: false };
				setState((s) =>
					sessionGenerationGuard.isCurrent(sessionIdentity, s.requestId) &&
					s.pendingInterrupt?.interruptId === interruptId
						? { ...s, pendingInterrupt: null, error: null }
						: s,
				);
				const raw = (env.data as { result?: unknown } | null | undefined)
					?.result;
				return { resolved: true, turnResult: raw ?? undefined };
			} catch (err) {
				if (!isCurrentInterrupt()) return { resolved: false };
				const detail =
					err instanceof ApiClientError
						? procurementErrorFromStatus(err.status, err.reason, err.message)
								.detail
						: "恢復失敗，請重試";
				toast.error(detail);
				await refreshCurrentSnapshot();
				return { resolved: false };
			}
		},
		[refreshCurrentSnapshot, sessionGenerationGuard],
	);

	const applyTurnResult = React.useCallback(
		async (
			rawData: unknown,
			assistantTurnId: string,
			operation: SessionOperation,
			terminalTelemetry: TerminalTelemetryObservation | null,
		) => {
			if (!operationGuard.isCurrent(operation)) return;
			const data = normalizeProcurementTurnResult(rawData);
			if (!data) {
				setState((s) =>
					operationGuard.isCurrent(operation)
						? {
								...s,
								pendingTurn: false,
								liveSteps: [],
								liveActivity: [],
								error: TURN_RESULT_INVALID,
								terminalTelemetry,
							}
						: s,
				);
				return;
			}
			if (!operationGuard.acceptsResult(operation, data.requestId)) {
				setState((s) =>
					operationGuard.isCurrent(operation)
						? {
								...s,
								pendingTurn: false,
								liveSteps: [],
								liveActivity: [],
								error: TURN_IDENTITY_MISMATCH,
								terminalTelemetry,
							}
						: s,
				);
				return;
			}
			const assistantTurn: ProcurementChatTurn = {
				id: assistantTurnId,
				role: "assistant",
				text: data.reply,
				blocks: data.blocks,
				recordedThisTurn: (() => {
					const recorded = normalizeRecorded(data.recordedThisTurn);
					return recorded.length > 0 ? recorded : undefined;
				})(),
				rejectedThisTurn: (() => {
					const rejected = normalizeRejected(data.rejected);
					return rejected.length > 0 ? rejected : undefined;
				})(),
				createdAt: Date.now(),
			};
			const conversationId = data.conversationId || null;
			const revisionChanged = stateRef.current.revisionId !== data.revisionId;
			if (revisionChanged) invalidateCandidateSelections();
			setState((s) => {
				if (!operationGuard.isCurrent(operation)) return s;
				return {
					...s,
					requestId: data.requestId,
					revisionId: data.revisionId,
					conversationId: conversationId ?? s.conversationId,
					slots: mapSlots(data.slots),
					missingRequired: data.missingRequired,
					question: normalizeQuestion(data.question),
					turns: [...s.turns, assistantTurn],
					justCompletedTurnId: assistantTurn.id,
					pendingTurn: false,
					liveSteps: [],
					liveActivity: [],
					terminalTelemetry,
					cropDisambiguation: normalizeCropDisambiguation(
						data.cropDisambiguation,
					),
					pendingInterrupt: data.interruptId
						? {
								interruptId: data.interruptId,
								reason: data.interruptKind ?? "",
							}
						: null,
					...(revisionChanged
						? {
								runId: null,
								candidateSet: null,
								pollError: false,
								candidatesStale: false,
							}
						: {}),
				};
			});
			if (!operationGuard.isCurrent(operation)) return;
			if (conversationId) {
				writeConversationUrl(conversationId);
				void queryClient.invalidateQueries({
					queryKey: queryKeys.assistant.transcript(
						queryIdentity,
						conversationId,
					),
				});
				void queryClient.invalidateQueries({
					queryKey: queryKeys.assistant.conversations(queryIdentity),
				});
			}

			try {
				const snapshot = await fetchSnapshot(data.requestId);
				if (!operationGuard.isCurrent(operation)) return;
				if (
					snapshot.requestId !== data.requestId ||
					snapshot.revisionId !== data.revisionId
				)
					return;
				setState((s) =>
					operationGuard.isCurrent(operation)
						? {
								...s,
								stage: deriveStageFromStatus(snapshot.status),
							}
						: s,
				);
			} catch {}
		},
		[
			fetchSnapshot,
			invalidateCandidateSelections,
			operationGuard,
			queryClient,
			queryIdentity,
		],
	);

	const submitTurn = React.useCallback(
		async (message: string, clientRequestId?: string) => {
			const trimmed = message.trim();
			if (!trimmed) return;
			const current = stateRef.current;
			if (current.pendingTurn) return;
			if (Array.from(trimmed).length > MAX_MESSAGE_RUNES) {
				setState((s) => ({
					...s,
					error: {
						code: "INPUT_TOO_LONG",
						detail: `單則訊息上限 ${MAX_MESSAGE_RUNES} 字`,
					},
				}));
				return;
			}

			const effectiveClientRequestId = clientRequestId ?? crypto.randomUUID();
			const turnTelemetry: TurnTelemetryAccumulator = {
				commandInvokedAt: monotonicNow(),
				firstStepTracked: false,
				duplicateStepSuppressedCount: 0,
				terminalObservedAt: null,
			};
			sessionGenerationGuard.invalidate();
			setRestoring(false);
			const operation = operationGuard.begin({
				clientRequestId: effectiveClientRequestId,
				requestId: current.requestId,
			});
			stateRef.current = { ...current, pendingTurn: true };
			const buyerTurn: ProcurementChatTurn = {
				id: effectiveClientRequestId,
				role: "buyer",
				text: trimmed,
				createdAt: Date.now(),
			};
			setState((s) =>
				operationGuard.isCurrent(operation)
					? {
							...s,
							turns: s.turns.some(
								(turn) => turn.id === effectiveClientRequestId,
							)
								? s.turns
								: [...s.turns, buyerTurn],
							justCompletedTurnId: null,
							pendingTurn: true,
							turnStartObserved: false,
							liveSteps: [],
							liveActivity: [],
							error: null,
							pendingInterrupt: null,
							terminalTelemetry: null,
						}
					: s,
			);

			const execute = async (): Promise<void> => {
				try {
					setState((s) =>
						operationGuard.isCurrent(operation) && s.streamFallback
							? { ...s, streamFallback: null }
							: s,
					);
					const streamed = await runTurnStream(
						{
							requestId: operation.requestId ?? undefined,
							message: trimmed,
							clientRequestId: operation.clientRequestId,
						},
						setState,
						() => operationGuard.isCurrent(operation),
						operation.signal,
						turnTelemetry,
					);
					if (!operationGuard.isCurrent(operation)) return;
					if (streamed) {
						await applyTurnResult(
							streamed,
							`${operation.clientRequestId}-assistant`,
							operation,
							turnTelemetry.terminalObservedAt === null
								? null
								: terminalTelemetryObservation(
										turnTelemetry.terminalObservedAt,
										operation.generation,
										turnTelemetry.duplicateStepSuppressedCount,
									),
						);
						return;
					}
					if (!operationGuard.isCurrent(operation)) return;
					const env = await apiClient.authed.post<unknown>(
						"/buyers/me/procurement/turns",
						{
							signal: operation.signal,
							json: {
								requestId: operation.requestId ?? undefined,
								message: trimmed,
								clientRequestId: operation.clientRequestId,
							},
						},
					);
					if (!operationGuard.isCurrent(operation)) return;
					const syncTerminalObservedAt = monotonicNow();
					await applyTurnResult(
						env.data,
						`${operation.clientRequestId}-assistant`,
						operation,
						terminalTelemetryObservation(
							syncTerminalObservedAt,
							operation.generation,
							turnTelemetry.duplicateStepSuppressedCount,
						),
					);
				} catch (err) {
					if (!operationGuard.isCurrent(operation)) return;
					const terminalTelemetry = terminalTelemetryObservation(
						turnTelemetry.terminalObservedAt ?? monotonicNow(),
						operation.generation,
						turnTelemetry.duplicateStepSuppressedCount,
					);
					if (err instanceof TurnStreamEventError) {
						if (err.code === 409 && err.reason === "APPROVAL_PENDING") {
							setState((s) =>
								operationGuard.isCurrent(operation)
									? {
											...s,
											pendingTurn: false,
											liveSteps: [],
											liveActivity: [],
											error: procurementErrorFromStatus(
												err.code,
												err.reason,
												err.message,
											),
											terminalTelemetry,
										}
									: s,
							);
							await refreshCurrentSnapshot();
							return;
						}
						setState((s) =>
							operationGuard.isCurrent(operation)
								? {
										...s,
										pendingTurn: false,
										liveSteps: [],
										liveActivity: [],
										error:
											err.code === 404
												? CONVERSATION_GONE
												: err.code === 409
													? TURN_IDEMPOTENCY_CONFLICT
													: {
															code: "TURN_FAILED",
															detail: err.message,
														},
										terminalTelemetry,
									}
								: s,
						);
						return;
					}
					if (err instanceof ApiClientError && err.status === 409) {
						if (err.reason === "APPROVAL_PENDING") {
							setState((s) =>
								operationGuard.isCurrent(operation)
									? {
											...s,
											pendingTurn: false,
											liveSteps: [],
											liveActivity: [],
											error: procurementErrorFromStatus(
												err.status,
												err.reason,
												err.message,
											),
											terminalTelemetry,
										}
									: s,
							);
							await refreshCurrentSnapshot();
							return;
						}
						setState((s) =>
							operationGuard.isCurrent(operation)
								? {
										...s,
										pendingTurn: false,
										liveSteps: [],
										liveActivity: [],
										error: TURN_IDEMPOTENCY_CONFLICT,
										terminalTelemetry,
									}
								: s,
						);
						return;
					}
					const detail =
						err instanceof ApiClientError
							? procurementErrorFromStatus(err.status, err.reason, err.message)
							: {
									code: "NETWORK_ERROR",
									detail: "訊息送出失敗，請檢查網路連線後重試",
								};
					setState((s) =>
						operationGuard.isCurrent(operation)
							? {
									...s,
									pendingTurn: false,
									liveSteps: [],
									liveActivity: [],
									error: detail,
									terminalTelemetry,
								}
							: s,
					);
				}
			};
			await execute();
		},
		[
			applyTurnResult,
			operationGuard,
			refreshCurrentSnapshot,
			sessionGenerationGuard,
		],
	);

	const submitAnswer = React.useCallback(
		async (answer: {
			slotKey: string;
			optionId?: string;
			optionLabel?: string;
			freeText?: string;
			unitOptionId?: string;
			unitLabel?: string;
			skip?: boolean;
		}) => {
			const current = stateRef.current;
			if (!current.requestId || !current.revisionId || current.pendingTurn)
				return;
			if (answer.freeText !== undefined) {
				const validationError = validateFreeTextAnswer(
					answer.slotKey,
					answer.freeText,
				);
				if (validationError) {
					setState((s) => ({
						...s,
						error: {
							code: "ANSWER_SCHEMA_INVALID",
							detail: validationError,
						},
					}));
					return;
				}
			}
			const buyerText = answer.skip
				? "略過"
				: answer.unitOptionId
					? [answer.freeText, answer.unitLabel].filter(Boolean).join(" ")
					: (answer.optionLabel ?? answer.freeText ?? "");
			if (!buyerText) return;
			const clientRequestId = crypto.randomUUID();
			sessionGenerationGuard.invalidate();
			setRestoring(false);
			const operation = operationGuard.begin({
				clientRequestId,
				requestId: current.requestId,
			});
			stateRef.current = { ...current, pendingTurn: true };
			setState((s) =>
				operationGuard.isCurrent(operation)
					? {
							...s,
							turns: [
								...s.turns,
								{
									id: clientRequestId,
									role: "buyer",
									text: buyerText,
									createdAt: Date.now(),
								},
							],
							justCompletedTurnId: null,
							pendingTurn: true,
							error: null,
							terminalTelemetry: null,
						}
					: s,
			);

			if (current.pendingInterrupt) {
				const outcome = await resume({
					optionId: answer.optionId,
					freeText: answer.freeText,
				});
				if (!operationGuard.isCurrent(operation)) return;
				if (outcome.resolved && outcome.turnResult !== undefined) {
					await applyTurnResult(
						outcome.turnResult,
						`${clientRequestId}-assistant`,
						operation,
						terminalTelemetryObservation(monotonicNow(), operation.generation),
					);
					return;
				}
				setState((s) =>
					operationGuard.isCurrent(operation)
						? {
								...s,
								pendingTurn: false,
								question: outcome.resolved ? null : s.question,
								turns: outcome.resolved
									? s.turns
									: s.turns.filter((t) => t.id !== clientRequestId),
								terminalTelemetry: terminalTelemetryObservation(
									monotonicNow(),
									operation.generation,
								),
							}
						: s,
				);
				if (outcome.resolved) await refreshCurrentSnapshot();
				return;
			}

			try {
				const env = await apiClient.authed.post<unknown>(
					`/buyers/me/procurement/requests/${operation.requestId}/answers`,
					{
						signal: operation.signal,
						json: {
							slotKey: answer.slotKey,
							optionId: answer.optionId,
							freeText: answer.freeText,
							unitOptionId: answer.unitOptionId,
							skip: answer.skip,
							expectedRevisionId: current.revisionId,
							clientRequestId,
						},
					},
				);
				if (!operationGuard.isCurrent(operation)) return;
				const terminalTelemetry = terminalTelemetryObservation(
					monotonicNow(),
					operation.generation,
				);
				await applyTurnResult(
					env.data,
					`${clientRequestId}-assistant`,
					operation,
					terminalTelemetry,
				);
			} catch (err) {
				if (!operationGuard.isCurrent(operation)) return;
				const terminalTelemetry = terminalTelemetryObservation(
					monotonicNow(),
					operation.generation,
				);
				const parsed =
					err instanceof ApiClientError
						? procurementErrorFromStatus(err.status, err.reason, err.message)
						: { code: "NETWORK_ERROR", detail: "答案送出失敗，請重試" };
				if (parsed.code === "STALE_REVISION" && operation.requestId) {
					invalidateCandidateSelections();
					toast.error("需求已在其他裝置更新，剛才的回答未送出，請重新確認");
					setState((s) =>
						operationGuard.isCurrent(operation)
							? {
									...s,
									pendingTurn: false,
									turns: s.turns.filter((t) => t.id !== clientRequestId),
									question: null,
									terminalTelemetry,
									runId: null,
									candidateSet: null,
									pollError: false,
									candidatesStale: false,
								}
							: s,
					);
					const snapshot = await fetchSnapshot(operation.requestId).catch(
						() => null,
					);
					if (!operationGuard.isCurrent(operation)) return;
					if (snapshot?.requestId === operation.requestId) {
						setState((s) =>
							operationGuard.isCurrent(operation)
								? {
										...s,
										revisionId: snapshot.revisionId,
										slots: mapSlots(snapshot.slots),
										missingRequired: snapshot.missingRequired,
										stage: deriveStageFromStatus(snapshot.status),
									}
								: s,
						);
					}
					return;
				}
				setState((s) =>
					operationGuard.isCurrent(operation)
						? { ...s, pendingTurn: false, error: parsed, terminalTelemetry }
						: s,
				);
			}
		},
		[
			applyTurnResult,
			fetchSnapshot,
			invalidateCandidateSelections,
			operationGuard,
			refreshCurrentSnapshot,
			resume,
			sessionGenerationGuard,
		],
	);

	const rewindSlot = React.useCallback(
		async (slotKey: string) => {
			const current = stateRef.current;
			if (!current.requestId || current.pendingTurn) return;
			const clientRequestId = crypto.randomUUID();
			sessionGenerationGuard.invalidate();
			setRestoring(false);
			const operation = operationGuard.begin({
				clientRequestId,
				requestId: current.requestId,
			});
			stateRef.current = { ...current, pendingTurn: true };
			setState((s) =>
				operationGuard.isCurrent(operation)
					? {
							...s,
							justCompletedTurnId: null,
							pendingTurn: true,
							error: null,
							pendingInterrupt: null,
							terminalTelemetry: null,
						}
					: s,
			);
			try {
				const env = await apiClient.authed.post<unknown>(
					`/buyers/me/procurement/requests/${operation.requestId}/rewinds`,
					{
						signal: operation.signal,
						json: { slotKey, clientRequestId },
					},
				);
				if (!operationGuard.isCurrent(operation)) return;
				const terminalTelemetry = terminalTelemetryObservation(
					monotonicNow(),
					operation.generation,
				);
				await applyTurnResult(
					env.data,
					`${clientRequestId}-assistant`,
					operation,
					terminalTelemetry,
				);
			} catch (err) {
				if (!operationGuard.isCurrent(operation)) return;
				const terminalTelemetry = terminalTelemetryObservation(
					monotonicNow(),
					operation.generation,
				);
				setState((s) =>
					operationGuard.isCurrent(operation)
						? {
								...s,
								pendingTurn: false,
								error:
									err instanceof ApiClientError
										? procurementErrorFromStatus(
												err.status,
												err.reason,
												err.message,
											)
										: {
												code: "NETWORK_ERROR",
												detail: "修改失敗，請重試",
											},
								terminalTelemetry,
							}
						: s,
				);
			}
		},
		[applyTurnResult, operationGuard, sessionGenerationGuard],
	);

	const startSearch = React.useCallback(async () => {
		const requestId = stateRef.current.requestId;
		if (!requestId) return;
		const sessionIdentity = sessionGenerationGuard.capture(requestId);
		const isCurrentSession = () =>
			sessionGenerationGuard.isCurrent(
				sessionIdentity,
				stateRef.current.requestId,
			);
		setState((s) =>
			sessionGenerationGuard.isCurrent(sessionIdentity, s.requestId)
				? {
						...s,
						error: null,
					}
				: s,
		);
		try {
			const env = await apiClient.authed.post<{
				requestId: string;
				runId: string;
				status: ProcurementRunStatus;
			}>(`/buyers/me/procurement/requests/${requestId}/candidate-searches`);
			if (!isCurrentSession() || env.data.requestId !== requestId) return;
			invalidateCandidateSelections();
			setState((s) =>
				sessionGenerationGuard.isCurrent(sessionIdentity, s.requestId)
					? {
							...s,
							stage: "searching",
							runId: env.data.runId,
							candidateSet: null,
							pollError: false,
							candidatesStale: false,
							error: null,
						}
					: s,
			);
		} catch (err) {
			if (!isCurrentSession()) return;
			if (err instanceof ApiClientError && err.status === 409) {
				const snapshot = await fetchSnapshot(requestId).catch(() => null);
				if (!isCurrentSession()) return;
				setState((s) =>
					sessionGenerationGuard.isCurrent(sessionIdentity, s.requestId)
						? {
								...s,
								stage:
									snapshot?.requestId === requestId
										? deriveStageFromStatus(snapshot.status)
										: s.stage,
								error: parseProcurementError(err.reason, err.message),
							}
						: s,
				);
				return;
			}
			setState((s) =>
				sessionGenerationGuard.isCurrent(sessionIdentity, s.requestId)
					? {
							...s,
							error:
								err instanceof ApiClientError
									? parseProcurementError(err.reason, err.message)
									: {
											code: "NETWORK_ERROR",
											detail: "候選搜尋建立失敗，請重試",
										},
						}
					: s,
			);
		}
	}, [fetchSnapshot, invalidateCandidateSelections, sessionGenerationGuard]);

	// biome-ignore lint/correctness/useExhaustiveDependencies: pollRetryToken is the explicit user-triggered retry signal, not a value read inside the effect body.
	React.useEffect(() => {
		const requestId = state.requestId;
		const runId = state.runId;
		if (state.stage !== "searching" || !requestId || !runId) return;
		let cancelled = false;
		const sessionIdentity = sessionGenerationGuard.capture(requestId);
		const isCurrentPoll = () =>
			!cancelled &&
			sessionGenerationGuard.isCurrent(
				sessionIdentity,
				stateRef.current.requestId,
			) &&
			stateRef.current.runId === runId;
		let timer: ReturnType<typeof setTimeout> | undefined;
		let failureCount = 0;

		const poll = async () => {
			try {
				const env = await apiClient.authed.get<ProcurementCandidateSearch>(
					`/buyers/me/procurement/requests/${requestId}/candidate-searches/${runId}`,
				);
				if (
					!isCurrentPoll() ||
					env.data.requestId !== requestId ||
					env.data.runId !== runId
				)
					return;
				failureCount = 0;
				if (isTerminalCandidateStatus(env.data.status)) {
					setState((s) =>
						!cancelled &&
						sessionGenerationGuard.isCurrent(sessionIdentity, s.requestId) &&
						s.runId === runId
							? {
									...s,
									stage: "candidates",
									candidateSet: env.data,
								}
							: s,
					);
					return;
				}
				timer = setTimeout(poll, POLL_INTERVAL_MS);
			} catch {
				if (!isCurrentPoll()) return;
				if (failureCount >= POLL_BACKOFF_MS.length) {
					setState((s) =>
						!cancelled &&
						sessionGenerationGuard.isCurrent(sessionIdentity, s.requestId) &&
						s.runId === runId
							? { ...s, pollError: true }
							: s,
					);
					return;
				}
				const delay = POLL_BACKOFF_MS[failureCount];
				failureCount += 1;
				timer = setTimeout(poll, delay);
			}
		};
		void poll();
		return () => {
			cancelled = true;
			if (timer) clearTimeout(timer);
		};
	}, [
		state.stage,
		state.requestId,
		state.runId,
		pollRetryToken,
		sessionGenerationGuard,
	]);

	const retryPolling = React.useCallback(() => {
		setState((s) => ({ ...s, pollError: false, error: null }));
		setPollRetryToken((token) => token + 1);
	}, []);

	const toggleSelection = React.useCallback(
		async (candidate: ProcurementCandidate) => {
			const current = stateRef.current;
			if (
				!current.requestId ||
				!current.candidateSet ||
				current.candidatesStale
			)
				return;
			const requestId = current.requestId;
			const candidateSetId = current.candidateSet.candidateSetId;
			const currentCandidate = [
				...current.candidateSet.candidates.platform,
				...current.candidateSet.candidates.external,
			].find((item) => item.candidateId === candidate.candidateId);
			if (!currentCandidate) return;
			const selectionIdentity = selectionGuard.begin(
				`${candidateSetId}:${currentCandidate.candidateId}`,
			);
			if (!selectionIdentity) return;
			const sessionIdentity = sessionGenerationGuard.capture(requestId);
			const isCurrentSelection = () =>
				selectionGuard.isCurrent(selectionIdentity) &&
				sessionGenerationGuard.isCurrent(
					sessionIdentity,
					stateRef.current.requestId,
				) &&
				stateRef.current.candidateSet?.candidateSetId === candidateSetId;
			const previousSelected = currentCandidate.selected;
			const nextSelected = !previousSelected;
			setPendingSelectionCandidateIds((currentIds) => {
				const next = new Set(currentIds);
				next.add(currentCandidate.candidateId);
				return next;
			});

			const applySelectedValue = (
				candidateSet: ProcurementCandidateSearch,
				selected: boolean,
			): ProcurementCandidateSearch => ({
				...candidateSet,
				candidates: {
					platform: applyOptimisticSelection(
						candidateSet.candidates.platform,
						currentCandidate.candidateId,
						selected,
					),
					external: applyOptimisticSelection(
						candidateSet.candidates.external,
						currentCandidate.candidateId,
						selected,
					),
				},
			});

			setState((s) =>
				sessionGenerationGuard.isCurrent(sessionIdentity, s.requestId) &&
				s.candidateSet?.candidateSetId === candidateSetId
					? {
							...s,
							candidateSet: applySelectedValue(s.candidateSet, nextSelected),
						}
					: s,
			);

			let invalidResponse = false;
			try {
				const env = await apiClient.authed.post<unknown>(
					`/buyers/me/procurement/requests/${requestId}/selections`,
					{
						json: {
							candidateSetId,
							lane: currentCandidate.lane,
							candidateId: currentCandidate.candidateId,
							selected: nextSelected,
						},
					},
				);
				if (!isCurrentSelection()) return;
				const response = normalizeCandidateSelectionResult(
					env.data,
					currentCandidate.candidateId,
				);
				if (!response) {
					invalidResponse = true;
					throw new Error("invalid candidate selection response");
				}
				setState((s) =>
					sessionGenerationGuard.isCurrent(sessionIdentity, s.requestId) &&
					s.candidateSet?.candidateSetId === candidateSetId
						? {
								...s,
								candidateSet: applySelectedValue(
									s.candidateSet,
									response.selected,
								),
							}
						: s,
				);
			} catch (err) {
				if (!isCurrentSelection()) return;
				const selectionUnconfirmed =
					invalidResponse || !(err instanceof ApiClientError);
				setState((s) =>
					sessionGenerationGuard.isCurrent(sessionIdentity, s.requestId) &&
					s.candidateSet?.candidateSetId === candidateSetId
						? {
								...s,
								candidateSet: applySelectedValue(
									s.candidateSet,
									previousSelected,
								),
								...(selectionUnconfirmed
									? {
											candidatesStale: true,
											error: CANDIDATE_SELECTION_UNCONFIRMED,
										}
									: {}),
							}
						: s,
				);
				if (selectionUnconfirmed) {
					toast.error(CANDIDATE_SELECTION_UNCONFIRMED.detail);
					return;
				}
				if (err instanceof ApiClientError) {
					const parsed = parseProcurementError(err.reason, err.message);
					if (parsed.code === "CANDIDATE_FACTS_STALE") {
						setState((s) =>
							sessionGenerationGuard.isCurrent(sessionIdentity, s.requestId) &&
							s.candidateSet?.candidateSetId === candidateSetId
								? { ...s, candidatesStale: true, error: parsed }
								: s,
						);
						return;
					}
					toast.error(parsed.detail || "勾選失敗，請重試");
					return;
				}
			} finally {
				const shouldClearPending = selectionGuard.isCurrent(selectionIdentity);
				selectionGuard.finish(selectionIdentity);
				if (shouldClearPending) {
					setPendingSelectionCandidateIds((currentIds) => {
						const next = new Set(currentIds);
						next.delete(currentCandidate.candidateId);
						return next;
					});
				}
			}
		},
		[selectionGuard, sessionGenerationGuard],
	);

	const previewRfqPrice = React.useCallback(
		async (
			command: RfqPricePreviewCommand,
		): Promise<RfqCommandResult<RfqPricePreview>> => {
			const current = stateRef.current;
			if (
				!current.requestId ||
				!current.revisionId ||
				current.revisionId !== command.expectedRevisionId
			) {
				return {
					ok: false,
					error: {
						code: "STALE_CONTEXT",
						detail: "目前畫面不是這份需求版本，請回到對話重新整理。",
						retryable: false,
					},
				};
			}
			const requestId = current.requestId;
			const revisionId = current.revisionId;
			const sessionIdentity = sessionGenerationGuard.capture(requestId);
			const isCurrentRfq = () =>
				sessionGenerationGuard.isCurrent(
					sessionIdentity,
					stateRef.current.requestId,
				) && stateRef.current.revisionId === revisionId;
			try {
				const env = await apiClient.authed.post<unknown>(
					`/buyers/me/procurement/requests/${requestId}/rfq-price-previews`,
					{ json: command },
				);
				if (!isCurrentRfq()) {
					return {
						ok: false,
						error: {
							code: "STALE_CONTEXT",
							detail: "需求已切換，舊的價格試算未套用。",
							retryable: false,
						},
					};
				}
				const parsed = rfqPricePreviewSchema.safeParse(env.data);
				if (!parsed.success) {
					return {
						ok: false,
						error: {
							code: "INVALID_RESPONSE",
							detail:
								"價格服務回傳不完整，無法確認這次操作的結果。請用原操作重試。",
							retryable: true,
						},
					};
				}
				return { ok: true, data: parsed.data };
			} catch (error) {
				if (!isCurrentRfq()) {
					return {
						ok: false,
						error: {
							code: "STALE_CONTEXT",
							detail: "需求已切換，舊的價格試算未套用。",
							retryable: false,
						},
					};
				}
				return { ok: false, error: rfqCommandFailure(error, "preview") };
			}
		},
		[sessionGenerationGuard],
	);

	const confirmRfqSnapshot = React.useCallback(
		async (
			command: RfqConfirmationCommand,
		): Promise<RfqCommandResult<RfqConfirmation>> => {
			const current = stateRef.current;
			if (
				!current.requestId ||
				!current.revisionId ||
				current.revisionId !== command.expectedRevisionId
			) {
				return {
					ok: false,
					error: {
						code: "STALE_CONTEXT",
						detail: "目前畫面不是這份需求版本，請回到對話重新整理。",
						retryable: false,
					},
				};
			}
			const requestId = current.requestId;
			const revisionId = current.revisionId;
			const sessionIdentity = sessionGenerationGuard.capture(requestId);
			const isCurrentRfq = () =>
				sessionGenerationGuard.isCurrent(
					sessionIdentity,
					stateRef.current.requestId,
				) && stateRef.current.revisionId === revisionId;
			try {
				const env = await apiClient.authed.post<unknown>(
					`/buyers/me/procurement/requests/${requestId}/rfq-confirmations`,
					{ json: command },
				);
				if (!isCurrentRfq()) {
					return {
						ok: false,
						error: {
							code: "STALE_CONTEXT",
							detail: "需求已切換，舊的確認結果未套用。",
							retryable: false,
						},
					};
				}
				const parsed = rfqConfirmationSchema.safeParse(env.data);
				if (!parsed.success) {
					return {
						ok: false,
						error: {
							code: "INVALID_RESPONSE",
							detail:
								"確認服務回傳不完整，無法確認這次操作的結果。請用原操作重試；這個畫面不會宣稱已固定。",
							retryable: true,
						},
					};
				}
				return { ok: true, data: parsed.data };
			} catch (error) {
				if (!isCurrentRfq()) {
					return {
						ok: false,
						error: {
							code: "STALE_CONTEXT",
							detail: "需求已切換，舊的確認結果未套用。",
							retryable: false,
						},
					};
				}
				return { ok: false, error: rfqCommandFailure(error, "confirmation") };
			}
		},
		[sessionGenerationGuard],
	);

	const previewRfqPublication = React.useCallback(
		async (
			command: RfqPublicationPreviewCommand,
		): Promise<RfqCommandResult<RfqPublicationApproval>> => {
			const current = stateRef.current;
			if (
				!current.requestId ||
				!current.revisionId ||
				current.revisionId !== command.expectedRevisionId
			) {
				return {
					ok: false,
					error: {
						code: "STALE_CONTEXT",
						detail: "目前畫面不是這份需求版本，請回到對話重新整理。",
						retryable: false,
					},
				};
			}
			const requestId = current.requestId;
			const revisionId = current.revisionId;
			const sessionIdentity = sessionGenerationGuard.capture(requestId);
			const isCurrentRfq = () =>
				sessionGenerationGuard.isCurrent(
					sessionIdentity,
					stateRef.current.requestId,
				) && stateRef.current.revisionId === revisionId;
			try {
				const env = await apiClient.authed.post<unknown>(
					`/buyers/me/procurement/requests/${requestId}/rfq-publications/preview`,
					{ json: publicationPreviewBody(command) },
				);
				if (!isCurrentRfq()) {
					return {
						ok: false,
						error: {
							code: "STALE_CONTEXT",
							detail: "需求已切換，舊的核准結果未套用。",
							retryable: false,
						},
					};
				}
				const parsed = rfqPublicationApprovalSchema.safeParse(env.data);
				if (!parsed.success) {
					return {
						ok: false,
						error: {
							code: "INVALID_RESPONSE",
							detail:
								"發布核准回傳不完整，無法確認這次操作的結果。請用原操作重試；這個畫面不會宣稱已核准。",
							retryable: true,
						},
					};
				}
				return { ok: true, data: parsed.data };
			} catch (error) {
				if (!isCurrentRfq()) {
					return {
						ok: false,
						error: {
							code: "STALE_CONTEXT",
							detail: "需求已切換，舊的核准結果未套用。",
							retryable: false,
						},
					};
				}
				return { ok: false, error: rfqCommandFailure(error, "confirmation") };
			}
		},
		[sessionGenerationGuard],
	);

	const executeRfqPublication = React.useCallback(
		async (
			command: RfqPublicationExecuteCommand,
		): Promise<RfqCommandResult<RfqPublicationResult>> => {
			const current = stateRef.current;
			if (
				!current.requestId ||
				!current.revisionId ||
				current.revisionId !== command.expectedRevisionId
			) {
				return {
					ok: false,
					error: {
						code: "STALE_CONTEXT",
						detail: "目前畫面不是這份需求版本，請回到對話重新整理。",
						retryable: false,
					},
				};
			}
			const requestId = current.requestId;
			const revisionId = current.revisionId;
			const sessionIdentity = sessionGenerationGuard.capture(requestId);
			const isCurrentRfq = () =>
				sessionGenerationGuard.isCurrent(
					sessionIdentity,
					stateRef.current.requestId,
				) && stateRef.current.revisionId === revisionId;
			try {
				const env = await apiClient.authed.post<unknown>(
					`/buyers/me/procurement/requests/${requestId}/rfq-publications`,
					{ json: publicationExecuteBody(command) },
				);
				if (!isCurrentRfq()) {
					return {
						ok: false,
						error: {
							code: "STALE_CONTEXT",
							detail: "需求已切換，舊的發布結果未套用。",
							retryable: false,
						},
					};
				}
				const parsed = rfqPublicationResultSchema.safeParse(env.data);
				if (!parsed.success) {
					return {
						ok: false,
						error: {
							code: "INVALID_RESPONSE",
							detail:
								"發布結果回傳不完整，無法確認這次操作的結果。請用原操作重試；這個畫面不會宣稱已發布。",
							retryable: true,
						},
					};
				}
				return { ok: true, data: parsed.data };
			} catch (error) {
				if (!isCurrentRfq()) {
					return {
						ok: false,
						error: {
							code: "STALE_CONTEXT",
							detail: "需求已切換，舊的發布結果未套用。",
							retryable: false,
						},
					};
				}
				return { ok: false, error: rfqCommandFailure(error, "confirmation") };
			}
		},
		[sessionGenerationGuard],
	);

	/**
	 * 讀出「現在按下停止會停到哪一個 run」。
	 *
	 * `GET .../conversations/{id}/runtime-snapshot` 是瀏覽器唯一拿得到
	 * `runVersion` 的地方——turn 的 SSE 與同步 `/turns` 都不帶 run 身分，所以
	 * 沒有這一次讀取就沒有 CAS token，停止鈕也就沒有誠實可按的版本。
	 *
	 * 讀失敗、端點還沒上線、或快照解析不過一律**安靜地**回 null：這是一顆
	 * affordance 的背景讀取，把它的失敗升級成 toast 只會在畫面上多一句與買家
	 * 當下動作無關的紅字，而按鈕維持不可用已經正確表達了「現在停不了」。
	 */
	const readActiveRun = React.useCallback(
		async (conversationId: string): Promise<CancellableRun | null> => {
			try {
				const env = await apiClient.authed.get<unknown>(
					`/buyers/me/procurement/conversations/${encodeURIComponent(
						conversationId,
					)}/runtime-snapshot`,
				);
				const parsed = runtimeSnapshotSchema.safeParse(env.data);
				if (!parsed.success) return null;
				return selectCancellableRun(parsed.data);
			} catch {
				return null;
			}
		},
		[],
	);

	// 只在 turn 執行中讀，而且讀兩次。
	//
	// 只讀一次不夠：送出的當下那一列 run 可能還沒寫進快照，讀回 null 之後按鈕
	// 就整輪都按不下去。`turnStartObserved`（伺服器已開始輸出這一輪的公開進度）
	// 之後那一列一定在了，所以它一翻成 true 就再讀一次。
	//
	// 不涵蓋 `stage === "searching"`：候選搜尋是脫離 graph 的背景 goroutine，
	// 後端的取消觀察器掛在 turn pipeline 上，而 `activeRunId` 本來就排除
	// `CANDIDATE_SEARCH`。搜尋中按鈕維持不可用是這件事的正確呈現，不是缺口。
	// biome-ignore lint/correctness/useExhaustiveDependencies: turnStartObserved is the re-read trigger (the run row exists once the server starts emitting this turn's public progress), not a value read inside the effect body.
	React.useEffect(() => {
		const conversationId = state.conversationId;
		if (!state.pendingTurn || !conversationId) {
			// 沒在跑就沒有可取消的 run。連同上一輪的回執一起清掉——讓「正在停止」
			// 停在一個已經結束的輪次上，比不顯示更糟。
			setState((s) =>
				s.activeRun === null && s.cancelState.status === "idle"
					? s
					: { ...s, activeRun: null, cancelState: IDLE_CANCEL_STATE },
			);
			return;
		}
		let cancelled = false;
		void (async () => {
			const run = await readActiveRun(conversationId);
			if (cancelled) return;
			setState((s) =>
				s.pendingTurn &&
				s.conversationId === conversationId &&
				// 使用者已經按下停止之後就不再覆寫：第二次讀回來的 token 會把
				// 已經送出的那次命令的結果蓋掉。
				s.cancelState.status === "idle"
					? { ...s, activeRun: run }
					: s,
			);
		})();
		return () => {
			cancelled = true;
		};
	}, [
		state.pendingTurn,
		state.turnStartObserved,
		state.conversationId,
		readActiveRun,
	]);

	/**
	 * 送出取消命令。
	 *
	 * 三條不變式：
	 *   1. **沒有 `activeRun` 就不送。** 呼叫端（按鈕）本來就該是 disabled；
	 *      這裡再擋一次，因為一個沒帶 `expectedRunVersion` 的請求只會換回 400，
	 *      而買家看到的會是「停止失敗」。
	 *   2. **CAS 沒過不是取消失敗。** 409 代表手上的版本過期，重讀快照換一把
	 *      新 token 再送一次。重讀之後拿不到 token 才收斂成 `stale`。
	 *   3. **不編 disposition。** 回執解析不過就說「無法確認結果」，不挑一個
	 *      最像的值填上去。
	 */
	const cancelActiveRun = React.useCallback(async (): Promise<void> => {
		const current = stateRef.current;
		const run = current.activeRun;
		const conversationId = current.conversationId;
		if (!run || !conversationId) return;
		if (current.cancelState.status === "requesting") return;
		const sessionIdentity = sessionGenerationGuard.capture(current.requestId);
		const isCurrentCancel = () =>
			sessionGenerationGuard.isCurrent(
				sessionIdentity,
				stateRef.current.requestId,
			) && stateRef.current.activeRun?.runId === run.runId;
		const settle = (next: RunCancelState, activeRun?: CancellableRun | null) =>
			setState((s) =>
				s.activeRun?.runId === run.runId
					? {
							...s,
							cancelState: next,
							...(activeRun === undefined ? {} : { activeRun }),
						}
					: s,
			);
		settle({ status: "requesting" });

		let raw: unknown;
		try {
			raw = await cancelProcurementRun({
				runId: run.runId,
				expectedRunVersion: run.runVersion,
			});
		} catch (error) {
			if (!isCurrentCancel()) return;
			if (!isRunVersionConflict(error)) {
				settle({
					status: "error",
					detail:
						error instanceof ApiClientError
							? procurementErrorFromStatus(
									error.status,
									error.reason,
									error.message,
								).detail
							: "停止沒有送出，請重試。",
				});
				return;
			}
			const fresh = await readActiveRun(conversationId);
			if (!isCurrentCancel()) return;
			if (!fresh || fresh.runId !== run.runId) {
				settle(STALE_CANCEL_STATE, fresh);
				return;
			}
			try {
				raw = await cancelProcurementRun({
					runId: fresh.runId,
					expectedRunVersion: fresh.runVersion,
				});
			} catch (retryError) {
				if (!isCurrentCancel()) return;
				settle(
					isRunVersionConflict(retryError)
						? STALE_CANCEL_STATE
						: {
								status: "error",
								detail:
									retryError instanceof ApiClientError
										? procurementErrorFromStatus(
												retryError.status,
												retryError.reason,
												retryError.message,
											).detail
										: "停止沒有送出，請重試。",
							},
					isRunVersionConflict(retryError) ? null : undefined,
				);
				return;
			}
		}
		if (!isCurrentCancel()) return;
		const parsed = runCancelResultSchema.safeParse(raw);
		if (!parsed.success) {
			settle(INVALID_CANCEL_ACK);
			return;
		}
		settle({ status: "settled", disposition: parsed.data.cancelDisposition });
	}, [readActiveRun, sessionGenerationGuard]);

	return {
		...state,
		restoring,
		pendingSelectionCandidateIds,
		cancelActiveRun,
		reset,
		submitTurn,
		submitAnswer,
		rewindSlot,
		startSearch,
		retryPolling,
		toggleSelection,
		previewRfqPrice,
		confirmRfqSnapshot,
		previewRfqPublication,
		executeRfqPublication,
		refreshCurrentSnapshot,
		resume,
	};
}
