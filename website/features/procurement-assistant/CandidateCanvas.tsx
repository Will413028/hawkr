"use client";

import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import { Button } from "@/components/ui";
import type {
	ProcurementCandidate,
	ProcurementSlots,
} from "@/lib/api/openapi-types";
import CandidateBrowser from "./CandidateBrowser";
import QuoteBatchSummary from "./QuoteBatchSummary";
import RfqDraftFlow from "./RfqDraftFlow";
import type {
	RfqCommandResult,
	RfqConfirmation,
	RfqConfirmationCommand,
	RfqPricePreview,
	RfqPricePreviewCommand,
} from "./rfq-price-contract";
import type {
	RfqPublicationApproval,
	RfqPublicationExecuteCommand,
	RfqPublicationPreviewCommand,
	RfqPublicationResult,
} from "./rfq-publication-contract";
import type { PublishedRfqPublication } from "./useProcurementSession";
import { useRfqRuntimeSnapshot } from "./useRfqRuntimeSnapshot";

type WorkspaceView = "candidates" | "rfq-draft" | "quote-invites";

const CANVAS_LABEL: Record<WorkspaceView, string> = {
	candidates: "候選資料工作台",
	"rfq-draft": "價格確認工作台",
	"quote-invites": "詢價狀態工作台",
};

interface CandidateCanvasProps {
	candidateSet:
		| React.ComponentProps<typeof CandidateBrowser>["candidateSet"]
		| null;
	publishedPublication: PublishedRfqPublication | null;
	candidatesStale: boolean;
	pendingSelectionCandidateIds: React.ComponentProps<
		typeof CandidateBrowser
	>["pendingSelectionCandidateIds"];
	onToggleSelection: (candidate: ProcurementCandidate) => void;
	onOpenCandidate: (candidate: ProcurementCandidate) => void;
	onRetrySearch: () => void;
	requestId: string | null;
	revisionId: string | null;
	conversationId: string | null;
	slots: ProcurementSlots;
	onReturnToConversation: () => void;
	onInteractionLockChange?: (locked: boolean) => void;
	hidden?: boolean;
	headingRef?: React.Ref<HTMLHeadingElement>;
	onPreviewRfqPrice: (
		command: RfqPricePreviewCommand,
	) => Promise<RfqCommandResult<RfqPricePreview>>;
	onPreviewRfqPublication: (
		command: RfqPublicationPreviewCommand,
	) => Promise<RfqCommandResult<RfqPublicationApproval>>;
	onExecuteRfqPublication: (
		command: RfqPublicationExecuteCommand,
	) => Promise<RfqCommandResult<RfqPublicationResult>>;
	onConfirmRfqSnapshot: (
		command: RfqConfirmationCommand,
	) => Promise<RfqCommandResult<RfqConfirmation>>;
	onRefreshSnapshot: () => Promise<boolean>;
	rfqPriceRangeEnabled: boolean;
	rfqApprovalEnabled: boolean;
}

export default function CandidateCanvas({
	candidateSet,
	publishedPublication,
	candidatesStale,
	pendingSelectionCandidateIds,
	onToggleSelection,
	onOpenCandidate,
	onRetrySearch,
	requestId,
	revisionId,
	conversationId,
	slots,
	onReturnToConversation,
	onInteractionLockChange,
	hidden = false,
	headingRef,
	onPreviewRfqPrice,
	onConfirmRfqSnapshot,
	onPreviewRfqPublication,
	onExecuteRfqPublication,
	onRefreshSnapshot,
	rfqPriceRangeEnabled,
	rfqApprovalEnabled,
}: CandidateCanvasProps) {
	const [workspaceView, setWorkspaceView] = React.useState<WorkspaceView>(
		candidateSet ? "candidates" : "quote-invites",
	);
	const [rfqInteractionLocked, setRfqInteractionLocked] = React.useState(false);
	React.useEffect(() => {
		onInteractionLockChange?.(rfqInteractionLocked);
	}, [onInteractionLockChange, rfqInteractionLocked]);
	React.useEffect(
		() => () => onInteractionLockChange?.(false),
		[onInteractionLockChange],
	);
	const selectedCandidates = candidateSet
		? [
				...candidateSet.candidates.platform,
				...candidateSet.candidates.external,
			].filter((candidate) => candidate.selected)
		: [];
	const candidateSelectionIdentity = JSON.stringify([
		candidateSet?.candidateSetId ?? null,
		selectedCandidates.map((candidate) => candidate.candidateId).sort(),
	]);
	const rfqDraftIdentity = JSON.stringify([
		requestId ?? null,
		revisionId ?? null,
	]);
	const previousCandidateSelectionIdentityRef = React.useRef(
		candidateSelectionIdentity,
	);

	React.useEffect(() => {
		if (!candidateSet) return;
		if (rfqPriceRangeEnabled || workspaceView === "candidates") return;
		setWorkspaceView("candidates");
	}, [candidateSet, rfqPriceRangeEnabled, workspaceView]);

	React.useEffect(() => {
		if (rfqInteractionLocked) return;
		if (
			previousCandidateSelectionIdentityRef.current ===
			candidateSelectionIdentity
		)
			return;
		previousCandidateSelectionIdentityRef.current = candidateSelectionIdentity;
		setWorkspaceView("candidates");
	}, [candidateSelectionIdentity, rfqInteractionLocked]);

	const [publicationExecuted, setPublicationExecuted] = React.useState(false);
	const published = publicationExecuted || publishedPublication !== null;

	React.useEffect(() => {
		if (!candidateSet) return;
		if (workspaceView !== "quote-invites" || published) return;
		setWorkspaceView("candidates");
	}, [candidateSet, published, workspaceView]);

	React.useEffect(() => {
		if (!publicationExecuted) return;
		setWorkspaceView("quote-invites");
	}, [publicationExecuted]);

	const showingQuoteInvites = workspaceView === "quote-invites";
	const runtime = useRfqRuntimeSnapshot(
		showingQuoteInvites ? conversationId : null,
		showingQuoteInvites ? requestId : null,
	);

	return (
		<aside
			id="hawkr-agent-candidate-canvas"
			className="hawkr-agent-canvas"
			hidden={hidden}
			aria-label={CANVAS_LABEL[workspaceView]}
		>
			{workspaceView !== "candidates" && candidateSet ? (
				<header className="hawkr-agent-canvas__head">
					<button
						type="button"
						className="hawkr-agent-canvas__back"
						disabled={rfqInteractionLocked}
						onClick={() => setWorkspaceView("candidates")}
						aria-label="返回候選資料"
					>
						<AppIcon name="arrow-left" size={17} aria-hidden="true" />
					</button>
					<h2 className="hawkr-agent-canvas__title">
						{workspaceView === "rfq-draft" ? "確認價格範圍" : "詢價已建立"}
					</h2>
					<span className="hawkr-agent-canvas__draft-state">
						{workspaceView === "rfq-draft" ? "價格流程" : "已建立"}
					</span>
				</header>
			) : null}

			<div className="hawkr-agent-canvas__body">
				<div hidden={workspaceView !== "candidates"}>
					{published ? (
						<div className="hawkr-agent-canvas__resume">
							<AppIcon name="document" size={20} aria-hidden="true" />
							<div>
								<strong>這份需求已建立詢價</strong>
								<p>收件人分布與目前的回價進度都在詢價狀態裡。</p>
							</div>
							<Button
								variant="bordered"
								color="default"
								onClick={() => setWorkspaceView("quote-invites")}
								endContent={
									<AppIcon name="arrow-right" size={18} aria-hidden="true" />
								}
							>
								查看詢價狀態
							</Button>
						</div>
					) : null}
					{candidateSet ? (
						<CandidateBrowser
							candidateSet={candidateSet}
							candidatesStale={candidatesStale}
							pendingSelectionCandidateIds={pendingSelectionCandidateIds}
							onToggleSelection={onToggleSelection}
							onOpenCandidate={onOpenCandidate}
							onRetrySearch={onRetrySearch}
							onStartRfqDraft={() => setWorkspaceView("rfq-draft")}
							rfqDraftEnabled={rfqPriceRangeEnabled}
							headingRef={headingRef}
						/>
					) : null}
				</div>
				{rfqPriceRangeEnabled && candidateSet ? (
					<div hidden={workspaceView !== "rfq-draft"}>
						<RfqDraftFlow
							key={rfqDraftIdentity}
							requestId={requestId}
							revisionId={revisionId}
							candidateSetId={candidateSet.candidateSetId}
							slots={slots}
							candidates={selectedCandidates}
							onBackToCandidates={() => setWorkspaceView("candidates")}
							onReturnToConversation={onReturnToConversation}
							onPreviewRfqPrice={onPreviewRfqPrice}
							onConfirmRfqSnapshot={onConfirmRfqSnapshot}
							onPreviewRfqPublication={onPreviewRfqPublication}
							onExecuteRfqPublication={onExecuteRfqPublication}
							onRefreshSnapshot={onRefreshSnapshot}
							onInteractionLockChange={setRfqInteractionLocked}
							onPublicationExecuted={() => setPublicationExecuted(true)}
							approvalEnabled={rfqApprovalEnabled}
						/>
					</div>
				) : null}
				<div hidden={workspaceView !== "quote-invites"}>
					<QuoteBatchSummary
						slots={slots}
						publication={runtime.publication}
						counts={runtime.counts}
						onViewSnapshot={
							rfqPriceRangeEnabled && candidateSet
								? () => setWorkspaceView("rfq-draft")
								: undefined
						}
					/>
				</div>
			</div>
		</aside>
	);
}
