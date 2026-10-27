import { track } from "@/lib/analytics/gtag";

/**
 * Google Doc index 119419 names product metrics, not GA4 event names. Keep the
 * mapping explicit so a dashboard cannot mistake one beacon for a precomputed
 * rate.
 *
 * `entry_prompt_submit_rate` is derived downstream from two event counts:
 *
 *   count(hawkr_agent_entry_prompt_submit) / count(hawkr_agent_entry_view)
 *
 * In a visible tab, the terminal latency waits for the commit that removes the
 * rail and the following animation frame, so it observes rendered UI rather than
 * a zero-duration reducer transition. A background tab cannot provide that paint
 * signal; it emits the aggregate counts at commit without a latency value.
 */
export const HAWKR_AGENT_METRIC_MAPPING = Object.freeze({
	entry_prompt_submit_rate: {
		kind: "derived-rate",
		numeratorEvent: "hawkr_agent_entry_prompt_submit",
		denominatorEvent: "hawkr_agent_entry_view",
	},
	terminal_rail_clear_latency: {
		kind: "event-parameter",
		event: "hawkr_agent_terminal_rendered",
		parameter: "terminal_rail_clear_latency_ms",
		definition:
			"authoritative-current-terminal-observed-to-post-react-commit-animation-frame-ms",
		filters: {
			rail_was_visible: true,
			page_visible_at_commit: true,
			page_visible_at_measurement: true,
		},
	},
	first_step_latency: {
		kind: "event-parameter",
		event: "hawkr_agent_first_public_step_observed",
		parameter: "first_step_latency_ms",
		definition:
			"command-invocation-to-first-allowlisted-public-step-arrival-ms",
	},
	duplicate_step_suppressed_count: {
		kind: "event-parameter",
		event: "hawkr_agent_terminal_rendered",
		parameter: "duplicate_step_suppressed_count",
		definition: "semantic-merge-no-op-count-per-terminal-operation",
	},
	visible_step_count: {
		kind: "event-parameter",
		event: "hawkr_agent_terminal_rendered",
		parameter: "visible_step_count",
	},
} as const);

type SafeTelemetryParams = Readonly<Record<string, number | boolean>>;

export type HawkrAgentTelemetryDispatch = (
	eventName: string,
	params: SafeTelemetryParams,
) => void;

const dispatchToGa4: HawkrAgentTelemetryDispatch = (eventName, params) => {
	track(eventName, params);
};

function dispatchSafely(
	dispatch: HawkrAgentTelemetryDispatch,
	eventName: string,
	params: SafeTelemetryParams,
): void {
	try {
		dispatch(eventName, params);
	} catch {
		// Product interaction must remain fail-open when analytics is unavailable.
	}
}

/** Denominator observation for `entry_prompt_submit_rate`. */
export function trackHawkrAgentEntryView(
	dispatch: HawkrAgentTelemetryDispatch = dispatchToGa4,
): void {
	dispatchSafely(dispatch, "hawkr_agent_entry_view", { sample_count: 1 });
}

/** Numerator observation for `entry_prompt_submit_rate`; accepts no prompt. */
export function trackHawkrAgentEntryPromptSubmit(
	dispatch: HawkrAgentTelemetryDispatch = dispatchToGa4,
): void {
	dispatchSafely(dispatch, "hawkr_agent_entry_prompt_submit", {
		sample_count: 1,
	});
}

export interface FirstStepLatencySample {
	commandInvokedAt: number;
	firstStepObservedAt: number;
}

function safeLatency(startedAt: number, observedAt: number): number | null {
	if (
		!Number.isFinite(startedAt) ||
		!Number.isFinite(observedAt) ||
		startedAt < 0 ||
		observedAt < startedAt
	)
		return null;
	return observedAt - startedAt;
}

export function buildFirstStepLatencyParams(
	sample: FirstStepLatencySample,
): SafeTelemetryParams | null {
	const latency = safeLatency(
		sample.commandInvokedAt,
		sample.firstStepObservedAt,
	);
	return latency === null
		? null
		: { first_step_latency_ms: latency, sample_count: 1 };
}

export function trackHawkrAgentFirstStepLatency(
	sample: FirstStepLatencySample,
	dispatch: HawkrAgentTelemetryDispatch = dispatchToGa4,
): void {
	const params = buildFirstStepLatencyParams(sample);
	if (!params) return;
	dispatchSafely(dispatch, "hawkr_agent_first_public_step_observed", params);
}

export interface RailTelemetryState {
	pending: boolean;
	maxVisibleStepCount: number;
	lastTerminalOperationGeneration: number | null;
}

export interface RailCommitObservation {
	pending: boolean;
	visibleStepCount: number;
	terminalObservedAt: number | null;
	/** Browser-local epoch only; never included in analytics params. */
	terminalOperationGeneration: number | null;
	duplicateStepSuppressedCount: number;
}

export interface RailCommitResult {
	state: RailTelemetryState;
	terminal: {
		visibleStepCount: number;
		terminalObservedAt: number;
		duplicateStepSuppressedCount: number;
	} | null;
}

export const EMPTY_RAIL_TELEMETRY_STATE: RailTelemetryState = Object.freeze({
	pending: false,
	maxVisibleStepCount: 0,
	lastTerminalOperationGeneration: null,
});

function safeCount(value: number): number {
	return Number.isFinite(value) ? Math.max(0, Math.trunc(value)) : 0;
}

/**
 * Reduce committed UI state into one privacy-safe terminal sample. Only counts
 * and booleans cross this boundary; step code, label, payload, prompt and IDs do
 * not enter this module.
 */
export function observeRailCommit(
	previous: RailTelemetryState,
	observation: RailCommitObservation,
): RailCommitResult {
	const visibleStepCount = safeCount(observation.visibleStepCount);
	const terminalObservedAt =
		observation.terminalObservedAt !== null &&
		Number.isFinite(observation.terminalObservedAt) &&
		observation.terminalObservedAt >= 0
			? observation.terminalObservedAt
			: null;
	const terminalOperationGeneration =
		observation.terminalOperationGeneration !== null &&
		Number.isSafeInteger(observation.terminalOperationGeneration) &&
		observation.terminalOperationGeneration >= 0
			? observation.terminalOperationGeneration
			: null;

	if (observation.pending) {
		return {
			state: {
				pending: true,
				maxVisibleStepCount: previous.pending
					? Math.max(previous.maxVisibleStepCount, visibleStepCount)
					: visibleStepCount,
				lastTerminalOperationGeneration:
					previous.lastTerminalOperationGeneration,
			},
			terminal: null,
		};
	}

	const terminal =
		terminalObservedAt !== null &&
		terminalOperationGeneration !== null &&
		terminalOperationGeneration !== previous.lastTerminalOperationGeneration
			? {
					visibleStepCount: previous.pending
						? previous.maxVisibleStepCount
						: visibleStepCount,
					terminalObservedAt,
					duplicateStepSuppressedCount: safeCount(
						observation.duplicateStepSuppressedCount,
					),
				}
			: null;

	return {
		state: {
			pending: false,
			maxVisibleStepCount: 0,
			lastTerminalOperationGeneration:
				terminalOperationGeneration ?? previous.lastTerminalOperationGeneration,
		},
		terminal,
	};
}

export interface TerminalRenderedSample {
	visibleStepCount: number;
	duplicateStepSuppressedCount: number;
	terminalObservedAt: number;
	terminalRenderedAt: number;
	pageVisibleAtCommit: boolean;
	pageVisibleAtMeasurement: boolean;
}

export function buildTerminalRenderedParams(
	sample: TerminalRenderedSample,
): SafeTelemetryParams {
	const visibleStepCount = safeCount(sample.visibleStepCount);
	const railWasVisible = visibleStepCount > 0;
	const validLatency = safeLatency(
		sample.terminalObservedAt,
		sample.terminalRenderedAt,
	);

	return {
		visible_step_count: visibleStepCount,
		duplicate_step_suppressed_count: safeCount(
			sample.duplicateStepSuppressedCount,
		),
		rail_was_visible: railWasVisible,
		page_visible_at_commit: sample.pageVisibleAtCommit,
		page_visible_at_measurement: sample.pageVisibleAtMeasurement,
		...(railWasVisible &&
		sample.pageVisibleAtCommit &&
		sample.pageVisibleAtMeasurement &&
		validLatency !== null
			? { terminal_rail_clear_latency_ms: validLatency }
			: {}),
	};
}

export function trackHawkrAgentTerminalRendered(
	sample: TerminalRenderedSample,
	dispatch: HawkrAgentTelemetryDispatch = dispatchToGa4,
): void {
	dispatchSafely(
		dispatch,
		"hawkr_agent_terminal_rendered",
		buildTerminalRenderedParams(sample),
	);
}

type RequestFrame = (callback: () => void) => number;
type CancelFrame = (frame: number) => void;

interface ScheduleTerminalRailMeasurementOptions {
	visibleStepCount: number;
	duplicateStepSuppressedCount: number;
	terminalObservedAt: number;
	pageVisibleAtCommit: boolean;
	now?: () => number;
	isPageVisible?: () => boolean;
	requestFrame?: RequestFrame;
	cancelFrame?: CancelFrame;
	dispatch?: HawkrAgentTelemetryDispatch;
}

export function monotonicNow(): number {
	return typeof performance === "undefined" ? Date.now() : performance.now();
}

/**
 * Start after React's terminal commit and emit from the next paint opportunity.
 * The returned cleanup cancels the pending beacon on unmount or a superseding
 * committed state.
 */
export function scheduleTerminalRailMeasurement({
	visibleStepCount,
	duplicateStepSuppressedCount,
	terminalObservedAt,
	pageVisibleAtCommit,
	now = monotonicNow,
	isPageVisible = () =>
		typeof document !== "undefined" && document.visibilityState === "visible",
	requestFrame = (callback) => window.requestAnimationFrame(callback),
	cancelFrame = (frame) => window.cancelAnimationFrame(frame),
	dispatch = dispatchToGa4,
}: ScheduleTerminalRailMeasurementOptions): () => void {
	if (!pageVisibleAtCommit) {
		trackHawkrAgentTerminalRendered(
			{
				visibleStepCount,
				duplicateStepSuppressedCount,
				terminalObservedAt,
				terminalRenderedAt: now(),
				pageVisibleAtCommit: false,
				pageVisibleAtMeasurement: false,
			},
			dispatch,
		);
		return () => {};
	}
	let cancelled = false;
	const frame = requestFrame(() => {
		if (cancelled) return;
		trackHawkrAgentTerminalRendered(
			{
				visibleStepCount,
				duplicateStepSuppressedCount,
				terminalObservedAt,
				terminalRenderedAt: now(),
				pageVisibleAtCommit,
				pageVisibleAtMeasurement: isPageVisible(),
			},
			dispatch,
		);
	});
	return () => {
		cancelled = true;
		cancelFrame(frame);
	};
}
