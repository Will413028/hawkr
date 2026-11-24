"use client";

import { Code, ConnectError } from "@connectrpc/connect";
import {
	RunFailureReason,
	RunStatus,
} from "@hawkr/api/gen/hawkr/procurement/v1alpha1/procurement_pb";
import { parseDate } from "@hawkr/api/proto/date";
import {
	commonStoreCardClosedStore,
	procurementRunFailureReason,
} from "@hawkr/ui-text/keys";
import { type QueryClient, useQueryClient } from "@tanstack/react-query";
import * as React from "react";
import {
	buyerProcurementClient,
	buyerProcurementStreamClient,
} from "@/lib/connect/procurement";
import { toast } from "@/lib/feedback/toast";
import { useQueryIdentity } from "@/lib/hooks/useQueryIdentity";
import {
	optionalText,
	useDefaultErrorText,
	useUiText,
} from "@/lib/i18n/ui-text";
import { toBrowserHref } from "@/lib/navigation/resolve-url";
import { type QueryIdentity, queryKeys } from "@/lib/query/keys";
import { type AgentActivityEvent, mergeLiveActivity } from "./activity";
import { validateFreeTextAnswer } from "./answer-validation";
import {
	type AgentTranscript,
	cancelProcurementRun,
	fetchAgentTranscript,
} from "./conversations";
import {
	invalidResponseFailure,
	type ProcurementError,
	type ProcurementFailure,
	procurementFailureFrom,
} from "./errors";
import {
	activityFromProto,
	candidateSearchFromProto,
	slotKeyFromCode,
	stepFromProto,
	turnResultFromProto,
} from "./proto-map";
import {
	pricePreviewFromProto,
	type RfqCommandResult,
	type RfqPricePreview,
	type RfqPricePreviewCommand,
} from "./rfq-price-contract";
import {
	approvalFromProto,
	publicationResultFromProto,
	type RfqPublicationApproval,
	type RfqPublicationExecuteCommand,
	type RfqPublicationPreviewCommand,
	type RfqPublicationResult,
} from "./rfq-publication-contract";
import { assistantConversationPath } from "./routes";
import {
	type CancellableRun,
	type RunCancelDisposition,
	runtimeSnapshotFromProto,
	selectCancellableRun,
} from "./runtime-snapshot-contract";
import {
	applyOptimisticSelection,
	deriveStageFromStatus,
	isTerminalCandidateStatus,
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
import { fetchProcurementSnapshot } from "./snapshot";
import { type AgentStep, mergeLiveStep } from "./steps";
import { monotonicNow, trackHawkrAgentFirstStepLatency } from "./telemetry";
import type {
	ClarifyQuestion,
	ProcurementCandidate,
	ProcurementCandidateSearch,
	ProcurementChatTurn,
	ProcurementInterrupt,
	ProcurementSlots,
	ProcurementSnapshot,
	ProcurementStage,
	ProcurementTurnResult,
	PublishedRfqPublication,
} from "./types";
import { type WatchOutcome, watchRun } from "./watch-run";

const MAX_MESSAGE_RUNES = 2000;
const POLL_INTERVAL_MS = 1500;
const POLL_BACKOFF_MS = [1500, 3000, 6000] as const;
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

export interface ProcurementResumeOutcome {
	resolved: boolean;
	/** 回答中斷排入的 run；結果從這個 run 的事件取得。 */
	runId?: string;
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
 */
export type RunCancelState =
	| { status: "idle" }
	| { status: "requesting" }
	| { status: "settled"; disposition: RunCancelDisposition }
	| { status: "error"; detail: string };

const IDLE_CANCEL_STATE: RunCancelState = { status: "idle" };

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
	liveSteps: AgentStep[];
	liveActivity: AgentActivityEvent[];
	runId: string | null;
	candidateSet: ProcurementCandidateSearch | null;
	pollError: boolean;
	candidatesStale: boolean;
	error: ProcurementError | null;
	pendingInterrupt: ProcurementInterrupt | null;
	publishedPublication: PublishedRfqPublication | null;
	/**
	 * 現在按下停止會停到的那一個 run。null＝**這一輪沒有可送出的取消命令**，
	 * 畫面要讓按鈕維持不可用。
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
		runId: null,
		candidateSet: null,
		pollError: false,
		candidatesStale: false,
		error: null,
		pendingInterrupt: null,
		publishedPublication: null,
		activeRun: null,
		cancelState: IDLE_CANCEL_STATE,
		terminalTelemetry: null,
	};
}

export function restoredStateFrom(
	transcript: AgentTranscript | null,
	snapshot: ProcurementSnapshot,
): Partial<SessionState> {
	const slots = snapshot.slots;
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
	const confirmation = snapshot.openConfirmation;
	return {
		requestId: snapshot.requestId,
		revisionId: snapshot.revisionId,
		conversationId: transcript?.conversationId ?? null,
		turns,
		slots,
		missingRequired: snapshot.missingRequired,
		stage: deriveStageFromStatus(snapshot.status),
		question: confirmation?.question ?? null,
		pendingInterrupt: confirmation
			? { interruptId: confirmation.interruptId, reason: confirmation.reason }
			: null,
		publishedPublication: snapshot.publishedPublication,
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
	const snapshot = queryClient.getQueryData<ProcurementSnapshot>(
		queryKeys.assistant.snapshot(identity, transcript.requestId),
	);
	if (!snapshot) return null;
	return restoredStateFrom(transcript, snapshot);
}

function staleContextFailure(detail: string): RfqCommandResult<never> {
	return {
		ok: false,
		error: { code: "STALE_CONTEXT", detail, retryable: false },
	};
}

function writeConversationUrl(conversationId: string) {
	if (typeof window === "undefined") return;
	const target = toBrowserHref(assistantConversationPath(conversationId));
	if (window.location.pathname === target) return;
	window.history.replaceState(null, "", target);
}

type TurnWatchEnd =
	| { kind: "result"; result: ProcurementTurnResult }
	| { kind: "run-failed"; reason: RunFailureReason }
	| { kind: "cancelled" }
	| { kind: "invalid" }
	| { kind: "resync" }
	| { kind: "error"; error: unknown }
	| { kind: "aborted" };

function endFromOutcome(
	outcome: WatchOutcome,
	turnResult: ProcurementTurnResult | null,
	finished: { status: RunStatus; failureReason: RunFailureReason } | null,
): TurnWatchEnd {
	switch (outcome.kind) {
		case "aborted":
			return { kind: "aborted" };
		case "resync":
			return { kind: "resync" };
		case "failed":
			return { kind: "error", error: outcome.error };
		case "exhausted":
			return {
				kind: "error",
				error:
					outcome.error ?? new ConnectError("watch ended", Code.Unavailable),
			};
		case "finished": {
			if (finished?.status === RunStatus.FAILED) {
				return { kind: "run-failed", reason: finished.failureReason };
			}
			if (finished?.status === RunStatus.CANCELLED) {
				return { kind: "cancelled" };
			}
			return turnResult
				? { kind: "result", result: turnResult }
				: { kind: "invalid" };
		}
	}
}

async function watchTurnRun(
	runId: string,
	setState: React.Dispatch<React.SetStateAction<SessionState>>,
	isCurrent: () => boolean,
	signal: AbortSignal,
	telemetry: TurnTelemetryAccumulator,
): Promise<TurnWatchEnd> {
	const client = buyerProcurementStreamClient();
	let turnResult: ProcurementTurnResult | null = null;
	let finished: { status: RunStatus; failureReason: RunFailureReason } | null =
		null;
	let steps: AgentStep[] = [];
	let activity: AgentActivityEvent[] = [];
	const observeStart = () => {
		if (telemetry.firstStepTracked) return;
		telemetry.firstStepTracked = true;
		trackHawkrAgentFirstStepLatency({
			commandInvokedAt: telemetry.commandInvokedAt,
			firstStepObservedAt: monotonicNow(),
		});
	};
	const outcome = await watchRun({
		open: (request, openSignal) =>
			client.watchRun(request, { signal: openSignal }),
		runId,
		signal,
		onEvent: (event) => {
			if (!isCurrent()) return;
			switch (event.event.case) {
				case "step": {
					const step = event.event.value.step
						? stepFromProto(event.event.value.step)
						: null;
					if (!step) return;
					observeStart();
					const merged = mergeLiveStep(steps, step);
					if (merged === steps) {
						telemetry.duplicateStepSuppressedCount += 1;
						return;
					}
					steps = merged;
					setState((s) =>
						isCurrent() && s.pendingTurn
							? { ...s, turnStartObserved: true, liveSteps: merged }
							: s,
					);
					return;
				}
				case "activity": {
					const row = activityFromProto(event.event.value);
					if (!row) return;
					observeStart();
					activity = mergeLiveActivity(activity, row);
					const merged = activity;
					setState((s) =>
						isCurrent() && s.pendingTurn
							? { ...s, turnStartObserved: true, liveActivity: merged }
							: s,
					);
					return;
				}
				case "turnResult":
					turnResult = turnResultFromProto(event.event.value);
					return;
				case "finished":
					telemetry.terminalObservedAt = monotonicNow();
					finished = {
						status: event.event.value.status,
						failureReason: event.event.value.failureReason,
					};
					return;
				default:
					return;
			}
		},
	});
	return endFromOutcome(outcome, turnResult, finished);
}

export function useProcurementSession(
	initialRequestId: string | null,
	initialConversationId: string | null = null,
) {
	const queryClient = useQueryClient();
	const queryIdentity = useQueryIdentity();
	const defaults = useDefaultErrorText();
	const ut = useUiText();
	const closedStoreName = ut(commonStoreCardClosedStore());

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
		(requestId: string): Promise<ProcurementSnapshot> =>
			queryClient.fetchQuery({
				queryKey: queryKeys.assistant.snapshot(queryIdentity, requestId),
				queryFn: () => fetchProcurementSnapshot(requestId),
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
					const confirmation = snapshot.openConfirmation;
					const preserveRegularQuestion =
						snapshot.requestId === existing.requestId &&
						snapshot.revisionId === existing.revisionId &&
						(snapshot.status === "CLARIFYING" ||
							snapshot.status === "REQUIREMENT_READY") &&
						confirmation === null &&
						!existing.pendingInterrupt;
					const keepExistingQuestion =
						(confirmation !== null &&
							confirmation.question === null &&
							confirmation.interruptId ===
								existing.pendingInterrupt?.interruptId) ||
						preserveRegularQuestion;
					return {
						...existing,
						revisionId: snapshot.revisionId,
						slots: snapshot.slots,
						missingRequired: snapshot.missingRequired,
						stage: deriveStageFromStatus(snapshot.status),
						question:
							confirmation?.question ??
							(keepExistingQuestion ? existing.question : null),
						pendingInterrupt: confirmation
							? {
									interruptId: confirmation.interruptId,
									reason: confirmation.reason,
								}
							: null,
						publishedPublication: snapshot.publishedPublication,
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
		async (response: {
			optionId?: string;
			freeText?: string;
		}): Promise<ProcurementResumeOutcome> => {
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
				const res = await buyerProcurementClient().resumeRun({
					interruptId,
					clientRequestId,
					response: {
						case: "clarification",
						value: {
							optionId: response.optionId ?? "",
							freeText: response.freeText ?? "",
						},
					},
				});
				if (!isCurrentInterrupt()) return { resolved: false };
				setState((s) =>
					sessionGenerationGuard.isCurrent(sessionIdentity, s.requestId) &&
					s.pendingInterrupt?.interruptId === interruptId
						? { ...s, pendingInterrupt: null, error: null }
						: s,
				);
				return { resolved: true, runId: res.runId };
			} catch (err) {
				if (!isCurrentInterrupt()) return { resolved: false };
				toast.error(procurementFailureFrom(err, defaults).detail);
				await refreshCurrentSnapshot();
				return { resolved: false };
			}
		},
		[defaults, refreshCurrentSnapshot, sessionGenerationGuard],
	);

	const applyTurnResult = React.useCallback(
		async (
			data: ProcurementTurnResult,
			assistantTurnId: string,
			operation: SessionOperation,
			terminalTelemetry: TerminalTelemetryObservation | null,
		) => {
			if (!operationGuard.isCurrent(operation)) return;
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
				recordedThisTurn:
					data.recordedThisTurn.length > 0 ? data.recordedThisTurn : undefined,
				rejectedThisTurn: data.rejected.length > 0 ? data.rejected : undefined,
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
					slots: data.slots,
					missingRequired: data.missingRequired,
					question: data.question,
					turns: [...s.turns, assistantTurn],
					justCompletedTurnId: assistantTurn.id,
					pendingTurn: false,
					liveSteps: [],
					liveActivity: [],
					terminalTelemetry,
					pendingInterrupt: data.interrupt,
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

	const settleTurnWatch = React.useCallback(
		async (
			operation: SessionOperation,
			end: TurnWatchEnd,
			assistantTurnId: string,
			turnTelemetry: TurnTelemetryAccumulator,
			ids: { requestId: string | null; conversationId: string | null },
			dropTurnId?: string,
		) => {
			if (end.kind === "aborted" || !operationGuard.isCurrent(operation))
				return;
			const terminalTelemetry = terminalTelemetryObservation(
				turnTelemetry.terminalObservedAt ?? monotonicNow(),
				operation.generation,
				turnTelemetry.duplicateStepSuppressedCount,
			);
			const settle = (extra: Partial<SessionState>) =>
				setState((s) =>
					operationGuard.isCurrent(operation)
						? {
								...s,
								pendingTurn: false,
								liveSteps: [],
								liveActivity: [],
								terminalTelemetry,
								...extra,
							}
						: s,
				);
			const fail = (error: ProcurementError) =>
				setState((s) =>
					operationGuard.isCurrent(operation)
						? {
								...s,
								pendingTurn: false,
								liveSteps: [],
								liveActivity: [],
								terminalTelemetry,
								error,
								turns: dropTurnId
									? s.turns.filter((turn) => turn.id !== dropTurnId)
									: s.turns,
							}
						: s,
				);
			switch (end.kind) {
				case "result":
					await applyTurnResult(
						end.result,
						assistantTurnId,
						operation,
						terminalTelemetry,
					);
					return;
				case "invalid":
					fail(invalidResponseFailure(defaults));
					return;
				case "run-failed":
					fail({
						code:
							end.reason === RunFailureReason.UNSPECIFIED
								? "RUN_FAILED"
								: (RunFailureReason[end.reason] ?? "RUN_FAILED"),
						detail:
							optionalText(ut, procurementRunFailureReason(end.reason)) ??
							defaults(Code.Internal),
					});
					if (dropTurnId) await refreshCurrentSnapshot();
					return;
				case "cancelled":
					settle({});
					await refreshCurrentSnapshot();
					return;
				case "error":
					fail(procurementFailureFrom(end.error, defaults));
					return;
				case "resync":
					try {
						if (ids.conversationId) {
							await queryClient.invalidateQueries({
								queryKey: queryKeys.assistant.transcript(
									queryIdentity,
									ids.conversationId,
								),
							});
						}
						const transcript = ids.conversationId
							? await fetchTranscript(ids.conversationId)
							: null;
						const requestId = transcript?.requestId || ids.requestId;
						if (!requestId) {
							fail(invalidResponseFailure(defaults));
							return;
						}
						const snapshot = await fetchSnapshot(requestId);
						if (!operationGuard.isCurrent(operation)) return;
						const restored = restoredStateFrom(transcript, snapshot);
						invalidateCandidateSelections();
						settle({
							...restored,
							turns: restored.turns?.length
								? restored.turns
								: stateRef.current.turns,
							error: null,
						});
						if (ids.conversationId) {
							writeConversationUrl(ids.conversationId);
							void queryClient.invalidateQueries({
								queryKey: queryKeys.assistant.conversations(queryIdentity),
							});
						}
					} catch (err) {
						fail(procurementFailureFrom(err, defaults));
					}
					return;
			}
		},
		[
			applyTurnResult,
			defaults,
			fetchSnapshot,
			fetchTranscript,
			invalidateCandidateSelections,
			operationGuard,
			queryClient,
			queryIdentity,
			refreshCurrentSnapshot,
			ut,
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
					const submitted = await buyerProcurementClient().submitTurn(
						{
							requestId: operation.requestId ?? "",
							message: trimmed,
							clientRequestId: operation.clientRequestId,
						},
						{ signal: operation.signal },
					);
					if (!operationGuard.isCurrent(operation)) return;
					const end = await watchTurnRun(
						submitted.runId,
						setState,
						() => operationGuard.isCurrent(operation),
						operation.signal,
						turnTelemetry,
					);
					await settleTurnWatch(
						operation,
						end,
						`${operation.clientRequestId}-assistant`,
						turnTelemetry,
						{
							requestId: submitted.requestId || null,
							conversationId: submitted.conversationId || null,
						},
					);
				} catch (err) {
					if (!operationGuard.isCurrent(operation)) return;
					const failure = procurementFailureFrom(err, defaults);
					setState((s) =>
						operationGuard.isCurrent(operation)
							? {
									...s,
									pendingTurn: false,
									liveSteps: [],
									liveActivity: [],
									error: failure,
									terminalTelemetry: terminalTelemetryObservation(
										turnTelemetry.terminalObservedAt ?? monotonicNow(),
										operation.generation,
										turnTelemetry.duplicateStepSuppressedCount,
									),
								}
							: s,
					);
					if (failure.code === "APPROVAL_PENDING") {
						await refreshCurrentSnapshot();
					}
				}
			};
			await execute();
		},
		[
			defaults,
			operationGuard,
			refreshCurrentSnapshot,
			sessionGenerationGuard,
			settleTurnWatch,
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
				if (outcome.resolved && outcome.runId) {
					const resumeTelemetry: TurnTelemetryAccumulator = {
						commandInvokedAt: monotonicNow(),
						firstStepTracked: false,
						duplicateStepSuppressedCount: 0,
						terminalObservedAt: null,
					};
					setState((s) =>
						operationGuard.isCurrent(operation)
							? {
									...s,
									question: null,
									turnStartObserved: false,
									liveSteps: [],
									liveActivity: [],
								}
							: s,
					);
					const end = await watchTurnRun(
						outcome.runId,
						setState,
						() => operationGuard.isCurrent(operation),
						operation.signal,
						resumeTelemetry,
					);
					await settleTurnWatch(
						operation,
						end,
						`${clientRequestId}-assistant`,
						resumeTelemetry,
						{
							requestId: current.requestId,
							conversationId: current.conversationId,
						},
						clientRequestId,
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
				const res = await buyerProcurementClient().submitAnswer(
					{
						requestId: operation.requestId ?? "",
						slotKey: slotKeyFromCode(answer.slotKey),
						optionId: answer.optionId ?? "",
						freeText: answer.freeText ?? "",
						unitOptionId: answer.unitOptionId ?? "",
						skip: answer.skip ?? false,
						expectedRevisionId: current.revisionId,
						clientRequestId,
					},
					{ signal: operation.signal },
				);
				if (!operationGuard.isCurrent(operation)) return;
				const terminalTelemetry = terminalTelemetryObservation(
					monotonicNow(),
					operation.generation,
				);
				if (!res.result) {
					setState((s) =>
						operationGuard.isCurrent(operation)
							? {
									...s,
									pendingTurn: false,
									error: TURN_RESULT_INVALID,
									terminalTelemetry,
								}
							: s,
					);
					return;
				}
				await applyTurnResult(
					turnResultFromProto(res.result),
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
				const failure = procurementFailureFrom(err, defaults);
				if (failure.code === "STALE_REVISION" && operation.requestId) {
					invalidateCandidateSelections();
					toast.error(failure.detail);
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
										slots: snapshot.slots,
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
						? {
								...s,
								pendingTurn: false,
								error: failure,
								terminalTelemetry,
							}
						: s,
				);
			}
		},
		[
			applyTurnResult,
			settleTurnWatch,
			defaults,
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
			if (!current.requestId || !current.revisionId || current.pendingTurn)
				return;
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
				const res = await buyerProcurementClient().rewind(
					{
						requestId: operation.requestId ?? "",
						slotKey: slotKeyFromCode(slotKey),
						clientRequestId,
						expectedRevisionId: current.revisionId,
					},
					{ signal: operation.signal },
				);
				if (!operationGuard.isCurrent(operation)) return;
				const terminalTelemetry = terminalTelemetryObservation(
					monotonicNow(),
					operation.generation,
				);
				if (!res.result) {
					setState((s) =>
						operationGuard.isCurrent(operation)
							? {
									...s,
									pendingTurn: false,
									error: TURN_RESULT_INVALID,
									terminalTelemetry,
								}
							: s,
					);
					return;
				}
				await applyTurnResult(
					turnResultFromProto(res.result),
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
								error: procurementFailureFrom(err, defaults),
								terminalTelemetry,
							}
						: s,
				);
			}
		},
		[applyTurnResult, defaults, operationGuard, sessionGenerationGuard],
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
			const res = await buyerProcurementClient().startCandidateSearch({
				requestId,
				clientRequestId: crypto.randomUUID(),
			});
			if (!isCurrentSession() || res.requestId !== requestId) return;
			invalidateCandidateSelections();
			setState((s) =>
				sessionGenerationGuard.isCurrent(sessionIdentity, s.requestId)
					? {
							...s,
							stage: "searching",
							runId: res.runId,
							candidateSet: null,
							pollError: false,
							candidatesStale: false,
							error: null,
						}
					: s,
			);
		} catch (err) {
			if (!isCurrentSession()) return;
			const failure = procurementFailureFrom(err, defaults);
			if (
				failure.code === "PROCUREMENT_RUN_IN_PROGRESS" ||
				failure.code === "REQUIREMENT_NOT_READY"
			) {
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
								error: failure,
							}
						: s,
				);
				return;
			}
			setState((s) =>
				sessionGenerationGuard.isCurrent(sessionIdentity, s.requestId)
					? { ...s, error: failure }
					: s,
			);
		}
	}, [
		defaults,
		fetchSnapshot,
		invalidateCandidateSelections,
		sessionGenerationGuard,
	]);

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
				const search = candidateSearchFromProto(
					await buyerProcurementClient().getCandidateSearch({
						requestId,
						runId,
					}),
					closedStoreName,
				);
				if (
					!isCurrentPoll() ||
					search.requestId !== requestId ||
					search.runId !== runId
				)
					return;
				failureCount = 0;
				if (isTerminalCandidateStatus(search.status)) {
					setState((s) =>
						!cancelled &&
						sessionGenerationGuard.isCurrent(sessionIdentity, s.requestId) &&
						s.runId === runId
							? {
									...s,
									stage: "candidates",
									candidateSet: search,
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
		closedStoreName,
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
			const currentCandidate = current.candidateSet.candidates.find(
				(item) => item.candidateId === candidate.candidateId,
			);
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
				candidates: applyOptimisticSelection(
					candidateSet.candidates,
					currentCandidate.candidateId,
					selected,
				),
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
				const res = await buyerProcurementClient().toggleSelection({
					requestId,
					candidateSetId,
					candidateId: currentCandidate.candidateId,
					selected: nextSelected,
				});
				if (!isCurrentSelection()) return;
				if (res.candidateId !== currentCandidate.candidateId) {
					invalidResponse = true;
					throw new Error("invalid candidate selection response");
				}
				setState((s) =>
					sessionGenerationGuard.isCurrent(sessionIdentity, s.requestId) &&
					s.candidateSet?.candidateSetId === candidateSetId
						? {
								...s,
								candidateSet: applySelectedValue(s.candidateSet, res.selected),
							}
						: s,
				);
			} catch (err) {
				if (!isCurrentSelection()) return;
				const failure = procurementFailureFrom(err, defaults);
				const selectionUnconfirmed =
					invalidResponse || failure.code === "NETWORK_ERROR";
				const candidateSetStale = failure.code === "STALE_CANDIDATE_SET";
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
									: candidateSetStale
										? { candidatesStale: true, error: failure }
										: {}),
							}
						: s,
				);
				if (selectionUnconfirmed) {
					toast.error(CANDIDATE_SELECTION_UNCONFIRMED.detail);
					return;
				}
				if (!candidateSetStale) toast.error(failure.detail);
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
		[defaults, selectionGuard, sessionGenerationGuard],
	);

	const markCandidatesStale = React.useCallback(
		(failure: ProcurementFailure) => {
			setState((s) =>
				s.candidateSet ? { ...s, candidatesStale: true, error: failure } : s,
			);
		},
		[],
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
				return staleContextFailure(
					"目前畫面不是這份需求版本，請回到對話重新整理。",
				);
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
				const res = await buyerProcurementClient().previewRfqPrice({
					requestId,
					expectedRevisionId: command.expectedRevisionId,
					buyerRequestedRange: {
						lowerBoundAmount: BigInt(
							command.buyerRequestedRange.lowerBoundAmount,
						),
						upperBoundAmount: BigInt(
							command.buyerRequestedRange.upperBoundAmount,
						),
					},
				});
				if (!isCurrentRfq()) {
					return staleContextFailure("需求已切換，舊的價格試算未套用。");
				}
				const preview = pricePreviewFromProto(res);
				if (!preview)
					return { ok: false, error: invalidResponseFailure(defaults) };
				return { ok: true, data: preview };
			} catch (error) {
				if (!isCurrentRfq()) {
					return staleContextFailure("需求已切換，舊的價格試算未套用。");
				}
				return { ok: false, error: procurementFailureFrom(error, defaults) };
			}
		},
		[defaults, sessionGenerationGuard],
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
				return staleContextFailure(
					"目前畫面不是這份需求版本，請回到對話重新整理。",
				);
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
				const res = await buyerProcurementClient().previewRfqPublication({
					requestId,
					clientRequestId: command.clientRequestId,
					expectedRevisionId: command.expectedRevisionId,
					buyerRequestedRange: {
						lowerBoundAmount: BigInt(
							command.buyerRequestedRange.lowerBoundAmount,
						),
						upperBoundAmount: BigInt(
							command.buyerRequestedRange.upperBoundAmount,
						),
					},
					buyerNote: command.buyerNote,
					candidateSetId: command.candidateSetId,
					selectedCandidateIds: command.selectedCandidateIds,
					responseDeadline: command.responseDeadline
						? parseDate(command.responseDeadline)
						: undefined,
				});
				if (!isCurrentRfq()) {
					return staleContextFailure("需求已切換，舊的核准結果未套用。");
				}
				const approval = approvalFromProto(res);
				if (!approval)
					return { ok: false, error: invalidResponseFailure(defaults) };
				return { ok: true, data: approval };
			} catch (error) {
				if (!isCurrentRfq()) {
					return staleContextFailure("需求已切換，舊的核准結果未套用。");
				}
				const failure = procurementFailureFrom(error, defaults);
				if (failure.code === "STALE_CANDIDATE_SET")
					markCandidatesStale(failure);
				return { ok: false, error: failure };
			}
		},
		[defaults, markCandidatesStale, sessionGenerationGuard],
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
				return staleContextFailure(
					"目前畫面不是這份需求版本，請回到對話重新整理。",
				);
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
				const res = await buyerProcurementClient().executeRfqPublication({
					requestId,
					clientRequestId: command.clientRequestId,
					approvalId: command.approval.approvalId,
					approvalVersion: command.approval.approvalVersion,
					expectedRevisionId: command.expectedRevisionId,
					payloadHash: command.approval.sellerFacingPayloadHash,
				});
				if (!isCurrentRfq()) {
					return staleContextFailure("需求已切換，舊的發布結果未套用。");
				}
				const result = publicationResultFromProto(res);
				if (!result)
					return { ok: false, error: invalidResponseFailure(defaults) };
				return { ok: true, data: result };
			} catch (error) {
				if (!isCurrentRfq()) {
					return staleContextFailure("需求已切換，舊的發布結果未套用。");
				}
				const failure = procurementFailureFrom(error, defaults);
				if (failure.code === "STALE_CANDIDATE_SET")
					markCandidatesStale(failure);
				return { ok: false, error: failure };
			}
		},
		[defaults, markCandidatesStale, sessionGenerationGuard],
	);

	/**
	 * 讀取現在按下停止會停到哪一個 run。
	 *
	 * 讀失敗一律安靜地回 null：這是一顆 affordance 的背景讀取，把它的失敗升級成 toast
	 * 只會在畫面上多一句與買家當下動作無關的紅字，而按鈕維持不可用已經正確表達了「現在停不了」。
	 */
	const readActiveRun = React.useCallback(
		async (conversationId: string): Promise<CancellableRun | null> => {
			try {
				const res = await buyerProcurementClient().getRuntimeSnapshot({
					conversationId,
				});
				return selectCancellableRun(runtimeSnapshotFromProto(res));
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
	/**
	 * 送出取消命令。
	 *
	 *   1. 沒有 `activeRun` 就不送。呼叫端（按鈕）本來就該是 disabled，這裡再擋一次。
	 *   2. 回 OK 不是「已取消」：`disposition` 原樣帶到畫面層，不挑一個最像的值填上去。
	 *   3. 回應沒有可辨識的處置時說「無法確認結果」。
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
		const settle = (next: RunCancelState) =>
			setState((s) =>
				s.activeRun?.runId === run.runId ? { ...s, cancelState: next } : s,
			);
		settle({ status: "requesting" });

		try {
			const result = await cancelProcurementRun(run.runId);
			if (!isCurrentCancel()) return;
			if (result.disposition === null) {
				settle({ status: "error", detail: defaults(Code.Internal) });
				return;
			}
			settle({ status: "settled", disposition: result.disposition });
		} catch (error) {
			if (!isCurrentCancel()) return;
			settle({
				status: "error",
				detail: procurementFailureFrom(error, defaults).detail,
			});
		}
	}, [defaults, sessionGenerationGuard]);

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
		previewRfqPublication,
		executeRfqPublication,
		refreshCurrentSnapshot,
		resume,
	};
}
