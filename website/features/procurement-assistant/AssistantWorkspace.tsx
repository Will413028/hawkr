"use client";

import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import { Drawer } from "@/components/shared/Drawer";
import { Button, Chip } from "@/components/ui";
import type { ProcurementCandidate } from "@/lib/api/openapi-types";
import { formatTaipeiDateTime } from "@/lib/date/taipei";
import { useKeyboardInset } from "@/lib/hooks/useKeyboardInset";
import Link from "@/lib/intl/native-link";
import { usePathname } from "@/lib/intl/navigation";
import AgentEntryPanel from "./AgentEntryPanel";
import AgentSidePanel from "./AgentSidePanel";
import AgentTaskCard from "./AgentTaskCard";
import AgentTaskPanel from "./AgentTaskPanel";
import { activityLabel, lastRunningActivity } from "./activity";
import { deriveMotionState } from "./agent-motion";
import { validateFreeTextAnswer } from "./answer-validation";
import { LANE_LABEL } from "./CandidateBrowser";
import CandidateCanvas from "./CandidateCanvas";
import { AGENT_CAPABILITIES_OFF, type AgentCapabilities } from "./capabilities";
import { ENTRY_COMPOSER_PLACEHOLDER } from "./entry-content";
import { consumeHawkrAgentHandoff } from "./handoff";
import ProcurementClarifyChat, {
	ProcurementClarifyComposer,
} from "./ProcurementClarifyChat";
import { ASSISTANT_PROCUREMENT_PATH } from "./routes";
import {
	hasProcurementResults,
	prepareTurnSubmission,
	questionHasUnitOptions,
	shouldAutofocusHandoff,
	shouldShowAgentEntry,
} from "./session-logic";
import { stepLabel } from "./steps";
import {
	EMPTY_RAIL_TELEMETRY_STATE,
	observeRailCommit,
	scheduleTerminalRailMeasurement,
	trackHawkrAgentEntryPromptSubmit,
	trackHawkrAgentEntryView,
} from "./telemetry";
import { useProcurementSession } from "./useProcurementSession";
import { useRunActivityStream } from "./useRunActivityStream";

function LoadingShell() {
	return (
		<section
			className="thread-page hawkr-agent-thread hawkr-agent-phase1-thread"
			aria-label="Hawkr Agent 採購經理"
			aria-busy="true"
			data-assistant-mode="live"
			data-assistant-loading
		>
			<header className="thread-page__head">
				<Link
					href="/account/messages"
					className="thread-page__back"
					aria-label="返回對話列表"
				>
					<AppIcon name="arrow-left" size={20} aria-hidden="true" />
				</Link>
				<div className="thread-page__head-meta">
					<h1>Hawkr Agent</h1>
					<span className="thread-page__head-status" data-state="assistant">
						正在恢復先前的需求
					</span>
				</div>
			</header>
			<div className="thread-page__body" role="status">
				<ul className="thread-page__list">
					<li className="thread-page__typing" aria-label="正在載入">
						<span /> <span /> <span />
					</li>
				</ul>
			</div>
		</section>
	);
}

function CandidateDetail({ candidate }: { candidate: ProcurementCandidate }) {
	return (
		<div className="assistant-candidate-detail">
			<div className="assistant-candidate-detail__status">
				<Chip
					variant="flat"
					color={candidate.lane === "PLATFORM" ? "primary" : "warning"}
					size="sm"
				>
					{LANE_LABEL[candidate.lane]}
				</Chip>
				<span>比對分數 {candidate.score}</span>
			</div>
			<section>
				<h3>已確認</h3>
				{candidate.matchedFacts.length > 0 ? (
					<ul className="assistant-candidate-detail__matches">
						{candidate.matchedFacts.map((fact) => (
							<li key={fact.key} data-status="match">
								<AppIcon name="check" size={17} aria-hidden="true" />
								<span>{fact.label}</span>
							</li>
						))}
					</ul>
				) : (
					<p>目前沒有已確認的欄位。</p>
				)}
			</section>
			<section>
				<h3>待確認</h3>
				{candidate.unverifiableFacts.length > 0 ? (
					<ul className="assistant-candidate-detail__matches">
						{candidate.unverifiableFacts.map((fact) => (
							<li key={fact.key} data-status="pending">
								<AppIcon name="clock" size={17} aria-hidden="true" />
								<span>{fact.reason}</span>
							</li>
						))}
					</ul>
				) : (
					<p>目前沒有待確認的欄位。</p>
				)}
			</section>
			<small>
				資料查核：
				{formatTaipeiDateTime(candidate.factsAsOf) || candidate.factsAsOf}
			</small>
		</div>
	);
}

interface AssistantWorkspaceProps {
	conversationId?: string;
	initialRequestId?: string | null;
	initialConversationId?: string | null;
	capabilities?: AgentCapabilities;
}

export default function AssistantWorkspace({
	conversationId,
	initialRequestId = null,
	initialConversationId = null,
	capabilities = AGENT_CAPABILITIES_OFF,
}: AssistantWorkspaceProps) {
	useKeyboardInset();
	const session = useProcurementSession(
		initialRequestId,
		initialConversationId,
	);
	const pathname = usePathname();
	const prevPathnameRef = React.useRef(pathname);
	const sessionReset = session.reset;
	const hasConversation = Boolean(session.conversationId);
	React.useEffect(() => {
		const prev = prevPathnameRef.current;
		prevPathnameRef.current = pathname;
		if (
			pathname === ASSISTANT_PROCUREMENT_PATH &&
			prev !== ASSISTANT_PROCUREMENT_PATH &&
			hasConversation
		) {
			sessionReset();
		}
	}, [pathname, hasConversation, sessionReset]);
	const [activeCandidate, setActiveCandidate] =
		React.useState<ProcurementCandidate | null>(null);
	const [panelOpen, setPanelOpen] = React.useState(false);
	const [resultInteractionLocked, setResultInteractionLocked] =
		React.useState(false);
	// ⛔ 掛載與可見性共用這一個值。原本掛載條件自己寫了一份（有吃 publication）、
	// hasResults 又寫了一份（沒吃），於是重整後面板掛得起來卻永遠是 hidden。
	const hasResults = hasProcurementResults(session);
	const needsRequirementRefresh =
		session.stage === "candidates" && !session.candidateSet;
	const canClarify =
		session.stage === "clarifying" ||
		session.stage === "ready" ||
		needsRequirementRefresh;
	const isProcessing = session.pendingTurn || session.stage === "searching";
	const resultsVisible = hasResults && panelOpen && !isProcessing;
	const selectionPending = session.pendingSelectionCandidateIds.size > 0;
	const candidateHeadingRef = React.useRef<HTMLHeadingElement | null>(null);
	const panelTriggerRef = React.useRef<HTMLButtonElement | null>(null);
	const panelReturnFocusFrameRef = React.useRef<number | null>(null);
	const [draft, setDraft] = React.useState("");
	const composerInputRef = React.useRef<HTMLTextAreaElement | null>(null);
	const [handoffNeedsFocus, setHandoffNeedsFocus] = React.useState(false);
	const pendingRequestKeyRef = React.useRef<string | null>(null);
	const lastSubmittedRef = React.useRef<string | null>(null);
	const consumedHandoffRef = React.useRef(false);
	const entryViewTrackedRef = React.useRef(false);
	const entrySubmitTrackedRef = React.useRef(false);
	const railTelemetryStateRef = React.useRef(EMPTY_RAIL_TELEMETRY_STATE);
	const threadRef = React.useRef<HTMLElement | null>(null);
	const transcriptRef = React.useRef<HTMLDivElement | null>(null);
	const stickToBottomRef = React.useRef(true);
	const searchActivity = useRunActivityStream({
		runId: session.runId,
		enabled: session.stage === "searching",
	});
	const searchActivityVersion = searchActivity
		.map((event) => `${event.rowId}:${event.status}:${event.text.length}`)
		.join("|");
	const liveStepVersion = session.liveSteps
		.map(
			(step) =>
				`${step.code}:${step.status}:${step.durationMs === undefined ? "" : step.durationMs}`,
		)
		.join("|");
	const liveActivityVersion = session.liveActivity
		.map((event) => `${event.rowId}:${event.status}:${event.text.length}`)
		.join("|");
	const transcriptVersion = `${session.stage}:${session.turns.length}:${session.pendingTurn}:${session.turnStartObserved}:${liveStepVersion}:${liveActivityVersion}:${searchActivityVersion}:${session.restoredSummary ?? ""}:${session.error?.code ?? ""}:${session.question?.slotKey ?? ""}`;

	React.useLayoutEffect(() => {
		void transcriptVersion;
		if (!stickToBottomRef.current) return;
		const transcript = transcriptRef.current;
		if (!transcript) return;
		transcript.scrollTop = transcript.scrollHeight;
	}, [transcriptVersion]);

	React.useEffect(() => {
		if (!hasResults) {
			setPanelOpen(false);
			return;
		}
		const desktopCanvas = window.matchMedia?.("(min-width: 1200px)");
		if (!desktopCanvas) return;
		let responsiveFocusFrame: number | null = null;
		const openOnDesktop = (
			event: MediaQueryList | MediaQueryListEvent = desktopCanvas,
		) => {
			if (event.matches) {
				setPanelOpen(true);
				return;
			}
			if (responsiveFocusFrame !== null)
				cancelAnimationFrame(responsiveFocusFrame);
			responsiveFocusFrame = requestAnimationFrame(() => {
				const panel = document.getElementById("hawkr-agent-side-panel");
				if (
					panel &&
					!panel.hidden &&
					threadRef.current?.contains(document.activeElement)
				) {
					panel
						.querySelector<HTMLButtonElement>(".hawkr-agent-panel__close")
						?.focus({ preventScroll: true });
				}
			});
		};
		openOnDesktop();
		desktopCanvas.addEventListener("change", openOnDesktop);
		return () => {
			desktopCanvas.removeEventListener("change", openOnDesktop);
			if (responsiveFocusFrame !== null)
				cancelAnimationFrame(responsiveFocusFrame);
		};
	}, [hasResults]);

	React.useEffect(() => {
		if (!resultsVisible) return;
		const frame = requestAnimationFrame(() => {
			const heading = candidateHeadingRef.current;
			if (heading && !heading.closest("[hidden]"))
				heading.focus({ preventScroll: true });
			else
				document
					.querySelector<HTMLButtonElement>(".hawkr-agent-panel__close")
					?.focus({ preventScroll: true });
		});
		return () => cancelAnimationFrame(frame);
	}, [resultsVisible]);

	React.useEffect(
		() => () => {
			if (panelReturnFocusFrameRef.current !== null) {
				cancelAnimationFrame(panelReturnFocusFrameRef.current);
			}
		},
		[],
	);

	const closePanel = React.useCallback(() => {
		setPanelOpen(false);
		if (panelReturnFocusFrameRef.current !== null) {
			cancelAnimationFrame(panelReturnFocusFrameRef.current);
		}
		panelReturnFocusFrameRef.current = requestAnimationFrame(() => {
			panelReturnFocusFrameRef.current = null;
			const input = composerInputRef.current;
			if (input && !input.disabled) input.focus({ preventScroll: true });
			else panelTriggerRef.current?.focus({ preventScroll: true });
		});
	}, []);

	const returnToConversation = closePanel;

	React.useEffect(() => {
		if (initialRequestId || initialConversationId || consumedHandoffRef.current)
			return;
		consumedHandoffRef.current = true;
		const handoff = consumeHawkrAgentHandoff();
		if (!handoff) return;
		setDraft(handoff.query);
		setHandoffNeedsFocus(
			shouldAutofocusHandoff(window.matchMedia?.bind(window)),
		);
	}, [initialRequestId, initialConversationId]);

	React.useEffect(() => {
		if (!handoffNeedsFocus || session.restoring) return;
		const frame = requestAnimationFrame(() => {
			composerInputRef.current?.focus();
			setHandoffNeedsFocus(false);
		});
		return () => cancelAnimationFrame(frame);
	}, [handoffNeedsFocus, session.restoring]);

	React.useEffect(() => {
		if (!session.pendingTurn || !session.turnStartObserved) return;
		const submitted = lastSubmittedRef.current;
		if (!submitted) return;
		setDraft((current) => (current.trim() === submitted ? "" : current));
	}, [session.pendingTurn, session.turnStartObserved]);

	React.useEffect(() => {
		if (session.pendingTurn || session.error) return;
		const submitted = lastSubmittedRef.current;
		if (submitted) {
			setDraft((current) => (current.trim() === submitted ? "" : current));
		}
		pendingRequestKeyRef.current = null;
		lastSubmittedRef.current = null;
	}, [session.pendingTurn, session.error]);

	React.useEffect(() => {
		if (!session.error) return;
		const failed = lastSubmittedRef.current;
		if (!failed) return;
		setDraft((current) => (current.trim() ? current : failed));
	}, [session.error]);

	const submitTurnFromComposer = (message: string) => {
		const previous =
			pendingRequestKeyRef.current && lastSubmittedRef.current
				? {
						clientRequestId: pendingRequestKeyRef.current,
						message: lastSubmittedRef.current,
					}
				: null;
		const submission = prepareTurnSubmission(previous, message, () =>
			crypto.randomUUID(),
		);
		pendingRequestKeyRef.current = submission.clientRequestId;
		lastSubmittedRef.current = submission.message;
		setPanelOpen(false);
		stickToBottomRef.current = true;
		void session.submitTurn(submission.message, submission.clientRequestId);
	};

	const handleDraftChange = (next: string) => {
		setDraft(next);
		const submitted = lastSubmittedRef.current;
		if (session.pendingTurn || !submitted || next.trim() === submitted) return;
		pendingRequestKeyRef.current = null;
		lastSubmittedRef.current = null;
	};

	const candidateTotal = session.candidateSet
		? session.candidateSet.candidates.platform.length +
			session.candidateSet.candidates.external.length
		: 0;

	const questionNeedsUnitPick = questionHasUnitOptions(session.question);
	const composerAnswerMode =
		canClarify && Boolean(session.question) && !questionNeedsUnitPick;

	const latestStep = session.turnStartObserved
		? (session.liveSteps.findLast((step) => step.status === "RUNNING") ??
			session.liveSteps.at(-1) ??
			null)
		: null;
	const latestActivity = session.turnStartObserved
		? (lastRunningActivity(session.liveActivity) ??
			session.liveActivity.at(-1) ??
			null)
		: null;
	const latestActivityLabel = latestActivity
		? activityLabel(latestActivity)
		: latestStep
			? stepLabel(latestStep)
			: undefined;
	const searchingRunningRow = lastRunningActivity(searchActivity);
	const searchingLabel =
		session.stage === "searching"
			? searchingRunningRow
				? activityLabel(searchingRunningRow)
				: "正在搜尋候選供應商"
			: null;
	const motion = deriveMotionState({
		hasError: Boolean(session.error),
		pending: session.pendingTurn,
		latestStepCode: latestActivity?.code ?? latestStep?.code ?? null,
		awaitingInput: Boolean(session.question),
		stage: session.stage,
	});

	const showEntry = shouldShowAgentEntry({
		conversationId,
		initialRequestId,
		initialConversationId,
		sessionConversationId: session.conversationId,
		restoredSummary: session.restoredSummary,
		turnCount: session.turns.length,
		stage: session.stage,
	});
	const entryVisible = showEntry && !session.restoring;
	const visibleStepCount = session.pendingTurn
		? session.liveActivity.length || session.liveSteps.length
		: 0;
	const terminalObservedAt = session.terminalTelemetry?.observedAt ?? null;
	const terminalOperationGeneration =
		session.terminalTelemetry?.operationGeneration ?? null;
	const duplicateStepSuppressedCount =
		session.terminalTelemetry?.duplicateStepSuppressedCount ?? 0;

	React.useEffect(() => {
		if (!entryVisible) {
			entryViewTrackedRef.current = false;
			entrySubmitTrackedRef.current = false;
			return;
		}
		if (entryViewTrackedRef.current) return;
		entryViewTrackedRef.current = true;
		trackHawkrAgentEntryView();
	}, [entryVisible]);

	React.useLayoutEffect(() => {
		const result = observeRailCommit(railTelemetryStateRef.current, {
			pending: session.pendingTurn,
			visibleStepCount,
			terminalObservedAt,
			terminalOperationGeneration,
			duplicateStepSuppressedCount,
		});
		railTelemetryStateRef.current = result.state;
		if (!result.terminal) return;

		return scheduleTerminalRailMeasurement({
			visibleStepCount: result.terminal.visibleStepCount,
			duplicateStepSuppressedCount:
				result.terminal.duplicateStepSuppressedCount,
			terminalObservedAt: result.terminal.terminalObservedAt,
			pageVisibleAtCommit: document.visibilityState === "visible",
		});
	}, [
		session.pendingTurn,
		visibleStepCount,
		terminalObservedAt,
		terminalOperationGeneration,
		duplicateStepSuppressedCount,
	]);

	if (session.restoring) {
		return <LoadingShell />;
	}

	const composer = (
		<ProcurementClarifyComposer
			pending={
				isProcessing ||
				Boolean(session.pendingInterrupt) ||
				selectionPending ||
				resultInteractionLocked
			}
			value={draft}
			onValueChange={handleDraftChange}
			inputRef={composerInputRef}
			// 停止鈕的三個 prop 都由 session 供給：run 身分與 CAS token 只有伺服器
			// 的 runtime snapshot 給得出來，composer 自己打 API 會讓同一個畫面
			// 出現第二套執行狀態。activeRun 為 null 時按鈕維持不可用。
			canStop={Boolean(session.activeRun)}
			stopState={session.cancelState}
			onStop={() => void session.cancelActiveRun()}
			variant={showEntry ? "entry" : "thread"}
			onSubmit={(message) => {
				if (composerAnswerMode && session.question?.allowFreeText) {
					void session.submitAnswer({
						slotKey: session.question.slotKey,
						freeText: message,
					});
					setDraft("");
					return;
				}
				if (showEntry && !entrySubmitTrackedRef.current) {
					entrySubmitTrackedRef.current = true;
					trackHawkrAgentEntryPromptSubmit();
				}
				submitTurnFromComposer(message);
			}}
			maxLength={composerAnswerMode ? 200 : undefined}
			placeholder={
				session.stage === "searching"
					? "搜尋中，完成後可繼續補充需求"
					: selectionPending
						? "正在更新選取，請稍候"
						: resultInteractionLocked
							? "正在確認價格，請稍候"
							: composerAnswerMode && session.question
								? `直接回答：${session.question.prompt}${["quantity", "budget_per_unit"].includes(session.question.slotKey) ? "（只填數字）" : ""}`
								: showEntry
									? ENTRY_COMPOSER_PLACEHOLDER
									: undefined
			}
			validate={
				composerAnswerMode
					? (message) =>
							validateFreeTextAnswer(session.question?.slotKey ?? "", message)
					: undefined
			}
		/>
	);

	if (showEntry) {
		return (
			<AgentEntryPanel
				draft={draft}
				onPrefill={(text) => {
					handleDraftChange(text);
					requestAnimationFrame(() => composerInputRef.current?.focus());
				}}
				composer={composer}
			/>
		);
	}

	const thread = (
		<section
			ref={threadRef}
			className="thread-page hawkr-agent-thread hawkr-agent-phase1-thread"
			aria-label="Hawkr Agent 採購經理"
			data-assistant-mode="live"
		>
			<header className="thread-page__head">
				<Link
					href="/account/messages"
					className="thread-page__back"
					aria-label="返回對話列表"
				>
					<AppIcon name="arrow-left" size={20} aria-hidden="true" />
				</Link>
				{}
				<div className="thread-page__head-meta">
					<h1>Hawkr Agent</h1>
					<span className="thread-page__head-status" data-state="assistant">
						採購經理
					</span>
				</div>
				{hasResults ? (
					<button
						ref={panelTriggerRef}
						type="button"
						className="hawkr-agent-phase1__panel-open"
						data-testid="agent-panel-open"
						onClick={() => setPanelOpen(true)}
						aria-controls="hawkr-agent-side-panel"
						aria-expanded={resultsVisible}
						disabled={session.pendingTurn}
					>
						查看成果
					</button>
				) : null}
			</header>

			<div
				ref={transcriptRef}
				className="thread-page__body"
				role="log"
				aria-label="Hawkr Agent 採購經理紀錄"
				aria-live="off"
				onScroll={(event) => {
					const target = event.currentTarget;
					stickToBottomRef.current =
						target.scrollHeight - target.scrollTop - target.clientHeight < 80;
				}}
			>
				<ul className="thread-page__list">
					{}
					{process.env.NODE_ENV !== "production" && session.streamFallback ? (
						<li className="thread-page__notice" role="status">
							思考步驟未即時顯示：{session.streamFallback}（僅開發環境提示）
						</li>
					) : null}

					<ProcurementClarifyChat
						turns={session.turns}
						restoredSummary={session.restoredSummary}
						pending={isProcessing}
						justCompletedTurnId={session.justCompletedTurnId}
						latestActivityLabel={latestActivityLabel}
						processing={
							<AgentTaskPanel
								stage={session.stage}
								pending={session.pendingTurn}
								liveSteps={session.liveSteps}
								liveActivity={session.liveActivity}
								searchActivity={searchActivity}
								motion={motion}
								candidateSet={session.candidateSet}
								pollError={session.pollError}
								onRetryPolling={session.retryPolling}
							/>
						}
						question={canClarify ? session.question : null}
						cropDisambiguation={session.cropDisambiguation}
						onChooseCropDisambiguation={
							session.pendingInterrupt
								? (option) => void session.resume({ optionId: option.optionId })
								: undefined
						}
						onPickOption={(option) => void session.submitAnswer(option)}
						onSubmitFreeText={({ slotKey, freeText }) =>
							void session.submitAnswer({ slotKey, freeText })
						}
						onSubmitCropOther={({ slotKey, description }) =>
							void session.submitAnswer({
								slotKey,
								optionId: "crop-other",
								freeText: description,
							})
						}
						onSkipOption={
							session.question?.canSkip
								? (payload) => void session.submitAnswer(payload)
								: undefined
						}
						onPickUnit={(payload) => void session.submitAnswer(payload)}
					/>

					{needsRequirementRefresh && !isProcessing ? (
						<li className="thread-page__notice">
							<p>需求已更新，可重新取得最新狀態以繼續搜尋。</p>
							<Button
								size="sm"
								variant="bordered"
								onClick={() => void session.refreshCurrentSnapshot()}
							>
								重新取得需求狀態
							</Button>
						</li>
					) : null}

					{}
					{session.stage === "ready" &&
					!session.question &&
					!isProcessing &&
					!session.pendingInterrupt ? (
						<AgentTaskCard onAssign={session.startSearch} />
					) : null}

					{session.stage === "candidates" &&
					session.candidateSet &&
					!isProcessing ? (
						<li
							className="hawkr-agent-phase1__candidate-bubble"
							data-candidate-artifact
						>
							<p>
								找到 {candidateTotal} 筆可能符合的候選資料，已整理在工作台。
							</p>
							{}
							{!resultsVisible ? (
								<Button
									variant="bordered"
									size="sm"
									className="hawkr-agent-phase1__canvas-open"
									disabled={isProcessing}
									onClick={() => {
										setPanelOpen(true);
									}}
									aria-controls="hawkr-agent-side-panel"
									aria-expanded={resultsVisible}
								>
									查看候選資料
								</Button>
							) : null}
						</li>
					) : null}

					{}
					{session.error ? (
						<li
							className="thread-page__bubble thread-page__bubble--card hawkr-agent-phase1__artifact-bubble assistant-invalidated"
							role="alert"
						>
							<AppIcon name="warning" size={20} aria-hidden="true" />
							{}
							<div>
								<strong>{session.error.code}</strong>
								<p>{session.error.detail}</p>
							</div>
						</li>
					) : null}
				</ul>
			</div>

			{composer}

			<Drawer
				open={Boolean(activeCandidate)}
				onClose={() => setActiveCandidate(null)}
				title={activeCandidate?.displayName}
				description="候選詳情"
				side="right"
				className="assistant-candidate-sheet"
				footer={
					<Button
						variant="bordered"
						color="default"
						fullWidth
						onClick={() => setActiveCandidate(null)}
					>
						關閉詳情
					</Button>
				}
			>
				{activeCandidate ? (
					<CandidateDetail candidate={activeCandidate} />
				) : null}
			</Drawer>
		</section>
	);

	return (
		<div
			className="hawkr-agent-workspace"
			data-panel={resultsVisible ? "open" : "closed"}
		>
			{thread}
			{}
			<div
				className="sr-only"
				role="status"
				aria-live="polite"
				aria-atomic="true"
			>
				{session.pendingTurn
					? (latestActivityLabel ?? "Hawkr Agent 正在整理你的需求")
					: (searchingLabel ?? "")}
			</div>
			{hasResults ? (
				<AgentSidePanel hidden={!resultsVisible} onClose={closePanel}>
					<CandidateCanvas
						candidateSet={session.candidateSet}
						publishedPublication={session.publishedPublication}
						candidatesStale={session.candidatesStale}
						pendingSelectionCandidateIds={session.pendingSelectionCandidateIds}
						onToggleSelection={(candidate) =>
							void session.toggleSelection(candidate)
						}
						onOpenCandidate={setActiveCandidate}
						onRetrySearch={() => void session.startSearch()}
						requestId={session.requestId}
						revisionId={session.revisionId}
						conversationId={session.conversationId ?? null}
						slots={session.slots}
						onReturnToConversation={returnToConversation}
						onInteractionLockChange={setResultInteractionLocked}
						headingRef={candidateHeadingRef}
						onPreviewRfqPrice={session.previewRfqPrice}
						onConfirmRfqSnapshot={session.confirmRfqSnapshot}
						onPreviewRfqPublication={session.previewRfqPublication}
						onExecuteRfqPublication={session.executeRfqPublication}
						onRefreshSnapshot={session.refreshCurrentSnapshot}
						rfqPriceRangeEnabled={
							capabilities.WEB_PROCUREMENT_RFQ_PRICE_RANGE_V1
						}
						rfqApprovalEnabled={capabilities.WEB_PROCUREMENT_RFQ_APPROVAL_V1}
					/>
				</AgentSidePanel>
			) : null}
		</div>
	);
}
