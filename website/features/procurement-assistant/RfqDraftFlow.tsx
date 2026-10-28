"use client";

import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import { Button } from "@/components/ui";
import type {
	ProcurementCandidate,
	ProcurementSlots,
} from "@/lib/api/openapi-types";
import { formatTaipeiDateTime, formatTaipeiIsoDate } from "@/lib/date/taipei";
import { buyerQualityStateLabel } from "./buyer-quality-state";
import { countSelectedRecipients } from "./candidate-grouping";
import {
	BUYER_NOTE_MAX_LENGTH,
	hasConfirmablePriceBasis,
	priceBasisOf,
	type RfqCommandFailure,
	type RfqCommandResult,
	type RfqConfirmation,
	type RfqConfirmationCommand,
	type RfqPricePreview,
	type RfqPricePreviewCommand,
	validateBuyerDesiredRange,
} from "./rfq-price-contract";
import {
	publicationAudienceIdentityOf,
	type RfqPublicationApproval,
	type RfqPublicationAudience,
	type RfqPublicationExecuteCommand,
	type RfqPublicationPreviewCommand,
	type RfqPublicationResult,
} from "./rfq-publication-contract";

interface RfqDraftFlowProps {
	requestId: string | null;
	revisionId: string | null;
	candidateSetId: string;
	slots: ProcurementSlots;
	candidates: ProcurementCandidate[];
	onBackToCandidates: () => void;
	onReturnToConversation: () => void;
	onPreviewRfqPrice: (
		command: RfqPricePreviewCommand,
	) => Promise<RfqCommandResult<RfqPricePreview>>;
	onConfirmRfqSnapshot: (
		command: RfqConfirmationCommand,
	) => Promise<RfqCommandResult<RfqConfirmation>>;
	onRefreshSnapshot: () => Promise<boolean>;
	onPreviewRfqPublication: (
		command: RfqPublicationPreviewCommand,
	) => Promise<RfqCommandResult<RfqPublicationApproval>>;
	onExecuteRfqPublication: (
		command: RfqPublicationExecuteCommand,
	) => Promise<RfqCommandResult<RfqPublicationResult>>;
	onInteractionLockChange?: (locked: boolean) => void;
	onPublicationExecuted?: () => void;
	approvalEnabled: boolean;
}

type RemoteState<T> =
	| { status: "idle" }
	| { status: "loading" }
	| { status: "success"; data: T }
	| { status: "error"; error: RfqCommandFailure };

interface StoredPreviewCommand {
	fingerprint: string;
	command: RfqPricePreviewCommand;
}

interface StoredConfirmationCommand {
	fingerprint: string;
	command: RfqConfirmationCommand;
}

const STEP_LABELS = ["需求版本", "預計邀請", "價格確認", "快照確認"] as const;
const RECONFIRM_STEP = 3;

function revisionLabel(revisionId: string | null): string {
	if (!revisionId) return "尚未取得需求版本";
	return `需求版本 ${revisionId.slice(-6)}`;
}

function formatTwd(value: string): string {
	const [integer, fraction] = value.split(".");
	const grouped = new Intl.NumberFormat("zh-TW").format(BigInt(integer));
	return `NT$${grouped}${fraction ? `.${fraction}` : ""}`;
}

function createClientRequestId(): string {
	return crypto.randomUUID();
}

function shortIdentity(value: string): string {
	return value.length <= 12 ? value : `${value.slice(0, 6)}…${value.slice(-6)}`;
}

function QuoteTotalRange({ preview }: { preview: RfqPricePreview }) {
	const total = preview.quoteTotalRange;
	if (!total) return null;
	return (
		<div className="assistant-rfq-guidance__quote-total">
			<div>
				<small>詢價總額區間</small>
				<strong>
					{`${formatTwd(total.lowerBoundAmount)} — ${formatTwd(total.upperBoundAmount)}`}
				</strong>
				<small>
					{`${total.quantity} ${total.quantityUnit}・依本次單價範圍計算`}
				</small>
			</div>
		</div>
	);
}

export default function RfqDraftFlow({
	requestId,
	revisionId,
	candidateSetId,
	slots,
	candidates,
	onBackToCandidates,
	onReturnToConversation,
	onPreviewRfqPrice,
	onConfirmRfqSnapshot,
	onRefreshSnapshot,
	onPreviewRfqPublication,
	onExecuteRfqPublication,
	onInteractionLockChange,
	onPublicationExecuted,
	approvalEnabled,
}: RfqDraftFlowProps) {
	const [step, setStep] = React.useState(0);
	const [furthestStep, setFurthestStep] = React.useState(0);
	const [desiredPriceLower, setDesiredPriceLower] = React.useState("");
	const [desiredPriceUpper, setDesiredPriceUpper] = React.useState("");
	const [previewState, setPreviewState] = React.useState<
		RemoteState<RfqPricePreview>
	>({ status: "idle" });
	const [confirmationState, setConfirmationState] = React.useState<
		RemoteState<RfqConfirmation>
	>({ status: "idle" });
	const [snapshotReconfirmed, setSnapshotReconfirmed] = React.useState(false);
	const [buyerNote, setBuyerNote] = React.useState("");
	const [publicationReconfirmed, setPublicationReconfirmed] =
		React.useState(false);
	const [responseDeadline, setResponseDeadline] = React.useState("");
	const hasRecipients = candidates.length > 0;
	const recipientBreakdown = React.useMemo(() => {
		const platform = countSelectedRecipients(
			candidates.filter((candidate) => candidate.lane === "PLATFORM"),
		);
		const external = countSelectedRecipients(
			candidates.filter((candidate) => candidate.lane === "EXTERNAL"),
		);
		return [
			platform > 0 ? `平台賣家 ${platform} 家` : "",
			external > 0 ? `站外業者 ${external} 家` : "",
		]
			.filter(Boolean)
			.join(" · ");
	}, [candidates]);
	const [publicationState, setPublicationState] = React.useState<
		RemoteState<RfqPublicationApproval>
	>({ status: "idle" });
	const publicationInFlightRef = React.useRef(false);
	const publicationOperationRef = React.useRef(0);

	// 受眾身分：核准是針對「這一份名單」發出的，名單一變，它就不再是核准過的那一份。
	// ⛔ 有序比較：server 的 audienceHash 涵蓋收件人順序，先 sort 再比會把真實差異抹掉。
	// responseDeadline 有自己的 reset（handleResponseDeadlineChange），revisionId 由
	// CandidateCanvas 的 key remount 涵蓋，所以這裡只需要 candidateSet 與收件人清單。
	const publicationAudienceIdentity = React.useMemo(
		() =>
			publicationAudienceIdentityOf(
				candidateSetId,
				candidates.map((candidate) => candidate.candidateId),
			),
		[candidateSetId, candidates],
	);
	const previousAudienceIdentityRef = React.useRef(publicationAudienceIdentity);
	// 核准當下畫面上的那份受眾身分。effect 的清理是**非同步**的，所以這個 ref 才是
	// 「這份核准屬於誰」的唯一答案；execute 送出前用它再比一次。
	const publicationApprovalIdentityRef = React.useRef<string | null>(null);
	const invalidatePublicationApproval = React.useCallback(() => {
		// ⛔ 遞增 operation：還在路上的 preview 回應不得把作廢掉的核准寫回來。
		publicationOperationRef.current += 1;
		publicationApprovalIdentityRef.current = null;
		setPublicationReconfirmed(false);
		setPublicationState({ status: "idle" });
	}, []);
	const [executionState, setExecutionState] = React.useState<
		RemoteState<RfqPublicationResult>
	>({ status: "idle" });
	const executionInFlightRef = React.useRef(false);
	const executeCommandRef = React.useRef<{
		approvalFingerprint: string;
		clientRequestId: string;
	} | null>(null);
	const [refreshingSnapshot, setRefreshingSnapshot] = React.useState(false);
	const previewCommandRef = React.useRef<StoredPreviewCommand | null>(null);
	const confirmationCommandRef = React.useRef<StoredConfirmationCommand | null>(
		null,
	);
	const previewOperationRef = React.useRef(0);
	const confirmationOperationRef = React.useRef(0);
	const previewInFlightRef = React.useRef<number | null>(null);
	const confirmationInFlightRef = React.useRef<number | null>(null);
	const resultHeadingRef = React.useRef<HTMLHeadingElement>(null);

	const quantity = slots.quantity ? String(slots.quantity) : "";
	const unit = slots.unit ?? "";
	const deliveryCity = slots.deliveryCity ?? "";
	const deliveryDate = slots.deliveryDate ?? "";
	const subject = slots.item ? `${slots.item}採購詢價` : "採購詢價";
	const itemResolution = slots.itemResolution;
	const details = [
		slots.item ?? "",
		slots.packaging ? `包裝：${slots.packaging}` : "",
		slots.exactMatchRequired ? "必須完全符合指定品項" : "",
	]
		.filter(Boolean)
		.join("\n");
	const platformCount = candidates.filter(
		(candidate) => candidate.lane === "PLATFORM",
	).length;
	const externalCount = candidates.length - platformCount;
	const hasRequiredContext = Boolean(
		requestId && revisionId && quantity && unit && deliveryCity,
	);
	const desiredRangeError = validateBuyerDesiredRange(
		desiredPriceLower,
		desiredPriceUpper,
	);
	const todayTaipei = formatTaipeiIsoDate(Date.now());
	const responseDeadlineError =
		responseDeadline && todayTaipei && responseDeadline < todayTaipei
			? "回覆期限不能早於今天"
			: null;
	const desiredRangeBlocking = Boolean(desiredRangeError);
	const preview = previewState.status === "success" ? previewState.data : null;
	const confirmationLocked =
		confirmationState.status === "loading" ||
		confirmationState.status === "success";
	const confirmableBasis = preview
		? hasConfirmablePriceBasis(preview, unit)
		: false;
	const canConfirm = Boolean(
		approvalEnabled &&
			preview &&
			confirmableBasis &&
			snapshotReconfirmed &&
			!refreshingSnapshot &&
			confirmationState.status === "idle",
	);

	const clearDerivedPriceState = React.useCallback(() => {
		previewOperationRef.current += 1;
		confirmationOperationRef.current += 1;
		previewCommandRef.current = null;
		confirmationCommandRef.current = null;
		setPreviewState({ status: "idle" });
		setConfirmationState({ status: "idle" });
		setSnapshotReconfirmed(false);
		invalidatePublicationApproval();
	}, [invalidatePublicationApproval]);

	// 受眾變了就作廢既有核准。畫面上那條警示條（「任一需求、價格、候選或受眾變更，
	// 都會使此確認失效並要求重新試算」）承諾的就是這件事；在此之前只有需求與價格那
	// 一半有實作。⛔ 同時遞增 operation，還在路上的 preview 回應不得把舊核准寫回來。
	// 名單被清空也走同一條路徑：identity 變了 ⇒ 核准作廢 ⇒ execute CTA 不再 render。
	React.useEffect(() => {
		if (previousAudienceIdentityRef.current === publicationAudienceIdentity)
			return;
		previousAudienceIdentityRef.current = publicationAudienceIdentity;
		invalidatePublicationApproval();
	}, [invalidatePublicationApproval, publicationAudienceIdentity]);

	React.useEffect(() => {
		onInteractionLockChange?.(confirmationState.status === "loading");
		return () => onInteractionLockChange?.(false);
	}, [confirmationState.status, onInteractionLockChange]);

	const runPreview = React.useCallback(
		async (reuseExistingCommand: boolean) => {
			if (
				previewInFlightRef.current === previewOperationRef.current ||
				confirmationLocked ||
				!requestId ||
				!revisionId
			)
				return;
			if (!deliveryCity || !quantity || !unit) {
				setPreviewState({
					status: "error",
					error: {
						code: "STALE_CONTEXT",
						detail: "需求缺少數量、單位或配送城市，請回到對話補充。",
						retryable: false,
					},
				});
				return;
			}
			if (desiredRangeBlocking) {
				return;
			}

			const commandInput = {
				expectedRevisionId: revisionId,
				deliveryCity,
				buyerRequestedRange: {
					currency: "TWD" as const,
					priceComponent: "GOODS_ONLY" as const,
					lowerBoundAmount: desiredPriceLower.trim(),
					upperBoundAmount: desiredPriceUpper.trim(),
				},
			};
			const fingerprint = JSON.stringify(commandInput);
			const existing = previewCommandRef.current;
			const command =
				reuseExistingCommand && existing?.fingerprint === fingerprint
					? existing.command
					: { ...commandInput, clientRequestId: createClientRequestId() };
			previewCommandRef.current = { fingerprint, command };
			confirmationCommandRef.current = null;
			confirmationOperationRef.current += 1;
			setConfirmationState({ status: "idle" });
			setSnapshotReconfirmed(false);
			const operation = previewOperationRef.current + 1;
			previewOperationRef.current = operation;
			previewInFlightRef.current = operation;
			setPreviewState({ status: "loading" });
			try {
				const result = await onPreviewRfqPrice(command);
				if (previewOperationRef.current !== operation) return;
				if (
					result.ok &&
					(result.data.buyerRequestedRange.lowerBoundAmount !==
						command.buyerRequestedRange.lowerBoundAmount ||
						result.data.buyerRequestedRange.upperBoundAmount !==
							command.buyerRequestedRange.upperBoundAmount)
				) {
					setPreviewState({
						status: "error",
						error: {
							code: "INVALID_RESPONSE",
							detail: "取得的區間與本次設定不一致，請重試。",
							retryable: true,
						},
					});
					return;
				}
				setPreviewState(
					result.ok
						? { status: "success", data: result.data }
						: { status: "error", error: result.error },
				);
			} finally {
				if (previewInFlightRef.current === operation) {
					previewInFlightRef.current = null;
				}
			}
		},
		[
			deliveryCity,
			desiredPriceLower,
			desiredPriceUpper,
			desiredRangeBlocking,
			onPreviewRfqPrice,
			quantity,
			requestId,
			revisionId,
			confirmationLocked,
			unit,
		],
	);

	React.useEffect(() => {
		if (
			previewState.status !== "error" &&
			confirmationState.status !== "error" &&
			confirmationState.status !== "success"
		)
			return;
		resultHeadingRef.current?.focus();
	}, [confirmationState.status, previewState.status]);

	const runConfirmation = React.useCallback(
		async (reuseExistingCommand: boolean) => {
			if (
				confirmationInFlightRef.current === confirmationOperationRef.current ||
				!approvalEnabled ||
				!snapshotReconfirmed ||
				confirmationState.status === "success" ||
				!requestId ||
				!revisionId ||
				!preview ||
				!hasConfirmablePriceBasis(preview, unit)
			)
				return;
			const commandInput = {
				expectedRevisionId: revisionId,
				pricePreviewId: preview.pricePreviewId,
				pricePreviewVersion: preview.version,
				payloadHash: preview.payloadHash,
				buyerNote: buyerNote.trim(),
			};
			const fingerprint = JSON.stringify(commandInput);
			const existing = confirmationCommandRef.current;
			const command =
				reuseExistingCommand && existing?.fingerprint === fingerprint
					? existing.command
					: { ...commandInput, clientRequestId: createClientRequestId() };
			confirmationCommandRef.current = { fingerprint, command };
			const operation = confirmationOperationRef.current + 1;
			confirmationOperationRef.current = operation;
			confirmationInFlightRef.current = operation;
			setConfirmationState({ status: "loading" });
			try {
				const result = await onConfirmRfqSnapshot(command);
				if (confirmationOperationRef.current !== operation) return;
				setConfirmationState(
					result.ok
						? { status: "success", data: result.data }
						: { status: "error", error: result.error },
				);
				if (result.ok) {
					setStep(RECONFIRM_STEP);
					setFurthestStep((current) => Math.max(current, RECONFIRM_STEP));
				}
			} finally {
				if (confirmationInFlightRef.current === operation) {
					confirmationInFlightRef.current = null;
				}
			}
		},
		[
			approvalEnabled,
			buyerNote,
			confirmationState.status,
			onConfirmRfqSnapshot,
			preview,
			requestId,
			revisionId,
			snapshotReconfirmed,
			unit,
		],
	);

	const runPublicationPreview = React.useCallback(async () => {
		const snapshot =
			confirmationState.status === "success" ? confirmationState.data : null;
		if (
			publicationInFlightRef.current ||
			!hasRecipients ||
			!publicationReconfirmed ||
			responseDeadlineError ||
			!snapshot ||
			!revisionId ||
			publicationState.status === "success"
		)
			return;
		const audience: RfqPublicationAudience = {
			mode: "DIRECT_RECIPIENTS",
			candidateSetId,
			selectedCandidateIds: candidates.map(
				(candidate) => candidate.candidateId,
			),
		};
		const operation = publicationOperationRef.current + 1;
		publicationOperationRef.current = operation;
		publicationInFlightRef.current = true;
		setPublicationState({ status: "loading" });
		try {
			const result = await onPreviewRfqPublication({
				previewClientRequestId: createClientRequestId(),
				expectedRevisionId: revisionId,
				confirmedRfqSnapshotId: snapshot.confirmedRfqSnapshotId,
				audience,
				responseDeadline,
			});
			// 晚到的回應對應的是已經被換掉的那一份受眾，丟棄。
			if (publicationOperationRef.current !== operation) return;
			// 記下這份核准是對「哪一份名單」發的：用送出去的那個 audience，不是
			// 現在畫面上的值，兩者在這裡相同，但前者才是核准的真正對象。
			publicationApprovalIdentityRef.current = result.ok
				? publicationAudienceIdentityOf(
						audience.candidateSetId,
						audience.selectedCandidateIds,
					)
				: null;
			setPublicationState(
				result.ok
					? { status: "success", data: result.data }
					: { status: "error", error: result.error },
			);
		} finally {
			publicationInFlightRef.current = false;
		}
	}, [
		hasRecipients,
		candidateSetId,
		candidates,
		confirmationState,
		onPreviewRfqPublication,
		publicationReconfirmed,
		publicationState.status,
		responseDeadline,
		responseDeadlineError,
		revisionId,
	]);

	const runPublicationExecute = React.useCallback(async () => {
		const current =
			publicationState.status === "success" ? publicationState.data : null;
		if (
			executionInFlightRef.current ||
			!current ||
			!revisionId ||
			executionState.status === "success"
		)
			return;
		// ⛔ 送出前再同步比一次。受眾變更的作廢走 passive effect，而 effect 是排進
		// scheduler 後才跑的：名單已經 commit 上畫面、effect 還沒執行的那個視窗裡，
		// 舊的 execute CTA 仍然可按。只靠 effect 就會用**上一份名單的核准**送出。
		// 這裡擋在任何 side effect 之前；已經在飛的 command 由上面的 in-flight 檢查
		// 保護，保留它自己的 clientRequestId 與核准身分，不因畫面改動重送或作廢。
		if (
			publicationApprovalIdentityRef.current !== publicationAudienceIdentity
		) {
			invalidatePublicationApproval();
			return;
		}
		const approvalFingerprint = `${current.approvalId}:${current.approvalVersion}:${current.sellerFacingPayloadHash}`;
		const stored = executeCommandRef.current;
		const clientRequestId =
			stored?.approvalFingerprint === approvalFingerprint
				? stored.clientRequestId
				: createClientRequestId();
		executeCommandRef.current = { approvalFingerprint, clientRequestId };
		executionInFlightRef.current = true;
		setExecutionState({ status: "loading" });
		try {
			const outcome = await onExecuteRfqPublication({
				clientRequestId,
				expectedRevisionId: revisionId,
				approval: current,
			});
			setExecutionState(
				outcome.ok
					? { status: "success", data: outcome.data }
					: { status: "error", error: outcome.error },
			);
			if (outcome.ok) onPublicationExecuted?.();
		} finally {
			executionInFlightRef.current = false;
		}
	}, [
		executionState.status,
		invalidatePublicationApproval,
		onExecuteRfqPublication,
		onPublicationExecuted,
		publicationAudienceIdentity,
		publicationState,
		revisionId,
	]);

	const handleResponseDeadlineChange = (value: string) => {
		setResponseDeadline(value);
		setPublicationState({ status: "idle" });
	};

	const handleRefreshSnapshot = async () => {
		if (refreshingSnapshot) return;
		setRefreshingSnapshot(true);
		try {
			if (await onRefreshSnapshot()) clearDerivedPriceState();
		} finally {
			setRefreshingSnapshot(false);
		}
	};

	const goToStep = (nextStep: number) => {
		if (confirmationLocked && nextStep !== 2 && nextStep !== RECONFIRM_STEP)
			return;
		if (nextStep === RECONFIRM_STEP && confirmationState.status !== "success")
			return;
		setStep(nextStep);
		setFurthestStep((current) => Math.max(current, nextStep));
	};

	const activeError =
		confirmationState.status === "error"
			? confirmationState.error
			: previewState.status === "error"
				? previewState.error
				: null;
	const confirmed =
		confirmationState.status === "success" ? confirmationState.data : null;
	const previewBasis = preview ? priceBasisOf(preview) : null;
	const basisSuffix =
		previewBasis?.basisQuantity && previewBasis.basisUnit
			? `／${previewBasis.basisQuantity} ${previewBasis.basisUnit}`
			: "";

	return (
		<section
			className="assistant-rfq-draft"
			aria-labelledby="assistant-rfq-draft-title"
			data-request-id={requestId ?? undefined}
			data-candidate-set-id={candidateSetId}
		>
			<header className="assistant-rfq-draft__intro">
				<h2 id="assistant-rfq-draft-title">核對需求與價格</h2>
				<p>設定希望詢價區間；確認後仍需另外發布詢價。</p>
				<small>{revisionLabel(revisionId)}</small>
			</header>

			<nav className="assistant-rfq-draft__steps" aria-label="價格確認步驟">
				{STEP_LABELS.map((label, index) => (
					<button
						key={label}
						type="button"
						disabled={
							index > furthestStep ||
							(confirmationLocked && index !== 2 && index !== RECONFIRM_STEP)
						}
						aria-current={step === index ? "step" : undefined}
						data-complete={index < step || undefined}
						onClick={() => goToStep(index)}
					>
						<span>{index + 1}</span>
						<strong>{label}</strong>
					</button>
				))}
			</nav>

			<div className="assistant-rfq-draft__content">
				{step === 0 ? (
					<section className="assistant-rfq-draft__section">
						<header>
							<span className="assistant-rfq-draft__section-icon">
								<AppIcon name="document" size={19} aria-hidden="true" />
							</span>
							<div>
								<h3>目前需求版本</h3>
							</div>
						</header>
						<div className="assistant-rfq-draft__fields">
							<label>
								<span>詢價主旨</span>
								<input value={subject} readOnly />
							</label>
							<label>
								<span>需求內容</span>
								<textarea value={details} readOnly rows={3} />
							</label>
							<div className="assistant-rfq-draft__field-row">
								<label>
									<span>需求數量</span>
									<input value={quantity} readOnly />
								</label>
								<label>
									<span>單位</span>
									<input value={unit} readOnly />
								</label>
							</div>
							<div className="assistant-rfq-draft__field-row">
								<label>
									<span>配送城市</span>
									<input value={deliveryCity} readOnly />
								</label>
								<label>
									<span>交貨日期</span>
									<input value={deliveryDate} readOnly />
								</label>
							</div>
							{itemResolution ? (
								<label>
									<span>品項分類（你已確認）</span>
									<input value={itemResolution.displayValue} readOnly />
								</label>
							) : null}
							<p className="assistant-rfq-draft__readonly-help">
								要修改數量、單位、城市或交期，請回到對話產生新的需求版本；本畫面不會沿用舊試算，後續需重新確認。
							</p>
						</div>
					</section>
				) : null}

				{step === 1 ? (
					<div className="assistant-rfq-draft__recipient-view">
						<section className="assistant-rfq-draft__section">
							<header>
								<span className="assistant-rfq-draft__section-icon">
									<AppIcon name="people" size={19} aria-hidden="true" />
								</span>
								<div>
									<h3>預計邀請 {candidates.length} 筆候選</h3>
									<p>
										平台 {platformCount} 筆 · 外部 {externalCount} 筆
									</p>
								</div>
								<button type="button" onClick={onBackToCandidates}>
									調整名單
								</button>
							</header>
							<div className="assistant-rfq-draft__recipients">
								{candidates.map((candidate) => (
									<article key={candidate.candidateId}>
										<span className="assistant-rfq-draft__recipient-mark">
											<AppIcon
												name={candidate.lane === "PLATFORM" ? "bag" : "goods"}
												size={19}
												aria-hidden="true"
											/>
										</span>
										<div>
											<strong>{candidate.displayName}</strong>
											<span>
												{candidate.lane === "PLATFORM"
													? "平台候選"
													: "外部候選"}{" "}
												· 比對 {candidate.score} 分
											</span>
										</div>
									</article>
								))}
							</div>
						</section>
						<div className="assistant-rfq-draft__notice" data-tone="info">
							<AppIcon name="info" size={18} aria-hidden="true" />
							<div>
								<strong>這份名單尚未固定</strong>
								<p>
									本次確認只固定需求版本與價格範圍；正式發布時才會固定邀請對象。
								</p>
							</div>
						</div>
					</div>
				) : null}

				{step === 2 ? (
					<div className="assistant-rfq-draft__review">
						<section className="assistant-rfq-draft__requirement-summary">
							<dl>
								<div>
									<dt>品項</dt>
									<dd>{slots.item ?? "—"}</dd>
								</div>
								<div>
									<dt>數量</dt>
									<dd>{[quantity, unit].filter(Boolean).join(" ") || "—"}</dd>
								</div>
								<div>
									<dt>配送城市</dt>
									<dd>{deliveryCity || "—"}</dd>
								</div>
								{buyerQualityStateLabel(
									preview?.qualityKnowledgeState ?? "",
								) ? (
									<div>
										<dt>品相</dt>
										<dd>
											{buyerQualityStateLabel(
												preview?.qualityKnowledgeState ?? "",
											)}
										</dd>
									</div>
								) : null}
							</dl>
							<p>供應商回價時提供實際品相與驗收條件。</p>
						</section>

						<div className="assistant-rfq-draft__notice" data-tone="info">
							<AppIcon name="shield" size={19} aria-hidden="true" />
							<div>
								<strong>這一步只確認需求與價格範圍</strong>
								<p>不會建立邀請、不會通知供應商，也不代表成交價格。</p>
							</div>
						</div>

						<section
							className="assistant-rfq-guidance"
							aria-labelledby="assistant-rfq-guidance-title"
						>
							<header className="assistant-rfq-guidance__head">
								<span className="assistant-rfq-draft__section-icon">
									<AppIcon name="chart" size={19} aria-hidden="true" />
								</span>
								<div>
									<h3 id="assistant-rfq-guidance-title">希望詢價區間</h3>
									<p>
										請先取得價格試算，再核對本次希望詢價區間；尚未送出的輸入不會納入確認。
									</p>
								</div>
								<span
									className="assistant-rfq-guidance__state"
									data-mode={previewState.status}
								>
									{previewState.status === "loading"
										? "查詢中"
										: preview
											? "已取得"
											: "待查詢"}
								</span>
							</header>

							{previewState.status === "loading" ? (
								<div
									className="assistant-rfq-guidance__loading"
									role="status"
									aria-live="polite"
								>
									正在建立無參考價試算…
								</div>
							) : null}

							{activeError ? (
								<div className="assistant-rfq-guidance__error" role="alert">
									<h3 ref={resultHeadingRef} tabIndex={-1}>
										目前無法完成價格確認
									</h3>
									<p>{activeError.detail}</p>
									<div>
										{activeError.code === "STALE_REVISION" ||
										activeError.code === "STALE_CONTEXT" ? (
											<Button
												variant="bordered"
												isLoading={refreshingSnapshot}
												loadingLabel="讀取中…"
												onClick={() => void handleRefreshSnapshot()}
											>
												讀取最新需求
											</Button>
										) : null}
										{activeError.code === "RESOURCE_NOT_FOUND" ||
										activeError.code === "PRICE_PREVIEW_STALE" ? (
											<Button
												variant="bordered"
												onClick={() => void runPreview(false)}
											>
												重新試算
											</Button>
										) : null}
										{activeError.retryable ? (
											<Button
												variant="bordered"
												onClick={() =>
													void (confirmationState.status === "error"
														? runConfirmation(true)
														: runPreview(true))
												}
											>
												以原操作重試
											</Button>
										) : null}
										{activeError.code === "IDEMPOTENCY_KEY_CONFLICT" ? (
											<Button
												variant="bordered"
												onClick={() =>
													void (confirmationState.status === "error"
														? runConfirmation(false)
														: runPreview(false))
												}
											>
												建立新的操作
											</Button>
										) : null}
										{activeError.code === "PRICE_BASIS_UNAVAILABLE" ||
										activeError.code === "REQUEST_NOT_FOUND" ? (
											<Button
												variant="bordered"
												onClick={onReturnToConversation}
											>
												回到對話修改需求
											</Button>
										) : null}
									</div>
								</div>
							) : null}

							{preview ? (
								<div className="assistant-rfq-guidance__price">
									<div className="assistant-rfq-guidance__price-copy">
										<p>
											價格成分為商品本身，不含運費；真正價格由供應商回價、買家接受後形成。
										</p>
									</div>
								</div>
							) : null}

							{previewState.status !== "success" ? (
								<div className="assistant-rfq-guidance__desired">
									<div>
										<small>新台幣整數</small>
										<strong>自行設定希望詢價區間</strong>
										<p>取得完整的計價單位與數量後才可確認。</p>
									</div>
									<div className="assistant-rfq-guidance__desired-fields">
										<label>
											<span>希望下限</span>
											<span className="assistant-rfq-guidance__money-input">
												<span aria-hidden="true">NT$</span>
												<input
													value={desiredPriceLower}
													onChange={(event) => {
														setDesiredPriceLower(event.currentTarget.value);
														clearDerivedPriceState();
													}}
													disabled={confirmationLocked}
													inputMode="numeric"
													pattern="[1-9][0-9]*"
													aria-describedby="assistant-rfq-desired-range-error"
													aria-invalid={
														desiredRangeBlocking ? "true" : undefined
													}
												/>
											</span>
										</label>
										<label>
											<span>希望上限</span>
											<span className="assistant-rfq-guidance__money-input">
												<span aria-hidden="true">NT$</span>
												<input
													value={desiredPriceUpper}
													onChange={(event) => {
														setDesiredPriceUpper(event.currentTarget.value);
														clearDerivedPriceState();
													}}
													disabled={confirmationLocked}
													inputMode="numeric"
													pattern="[1-9][0-9]*"
													aria-describedby="assistant-rfq-desired-range-error"
													aria-invalid={
														desiredRangeBlocking ? "true" : undefined
													}
												/>
											</span>
										</label>
										<Button
											disabled={confirmationLocked || desiredRangeBlocking}
											onClick={() => void runPreview(false)}
										>
											取得無參考價試算
										</Button>
									</div>
									<p
										id="assistant-rfq-desired-range-error"
										className="assistant-rfq-guidance__range-help"
										data-error={
											desiredRangeBlocking ? desiredRangeError : undefined
										}
										aria-live="polite"
									>
										{desiredRangeBlocking ? desiredRangeError : null}
									</p>
								</div>
							) : null}

							{preview ? (
								<>
									{preview.buyerRequestedRange ? (
										<div className="assistant-rfq-guidance__server-range">
											<div>
												<small>你設定的希望詢價區間</small>
												<strong>
													{formatTwd(
														preview.buyerRequestedRange.lowerBoundAmount,
													)}{" "}
													—{" "}
													{formatTwd(
														preview.buyerRequestedRange.upperBoundAmount,
													)}
												</strong>
											</div>
											<div>
												<small>價格基準</small>
												<strong>
													{preview.buyerRequestedRange.basisQuantity &&
													preview.buyerRequestedRange.basisUnit
														? `${preview.buyerRequestedRange.basisQuantity} ${preview.buyerRequestedRange.basisUnit}`
														: "尚未取得"}
												</strong>
											</div>
											<Button
												variant="bordered"
												disabled={confirmationLocked}
												onClick={() => void runPreview(false)}
											>
												重新產生價格試算
											</Button>
										</div>
									) : null}
									<QuoteTotalRange preview={preview} />
								</>
							) : null}
						</section>

						<section
							className="assistant-rfq-snapshot"
							aria-labelledby="assistant-rfq-snapshot-title"
						>
							<header>
								<span className="assistant-rfq-draft__section-icon">
									<AppIcon name="lock" size={19} aria-hidden="true" />
								</span>
								<div>
									<h3
										id="assistant-rfq-snapshot-title"
										ref={confirmed ? resultHeadingRef : undefined}
										tabIndex={confirmed ? -1 : undefined}
									>
										確認本次需求與價格
									</h3>
									<p>確認後會固定這一版需求與希望區間。</p>
								</div>
								<span className="assistant-rfq-snapshot__state">
									{confirmed ? "已確認" : preview ? "可確認" : "等待價格"}
								</span>
							</header>
							<div className="assistant-rfq-snapshot__facts">
								<div>
									<small>需求版本</small>
									<strong>{revisionLabel(revisionId)}</strong>
								</div>
								<div>
									<small>邀請名單</small>
									<strong>尚待正式發布固定</strong>
								</div>
								<div>
									<small>確認編號</small>
									<strong>
										{confirmed
											? shortIdentity(confirmed.confirmedRfqSnapshotId)
											: "尚未產生"}
									</strong>
								</div>
							</div>
							{preview && !confirmableBasis ? (
								<div
									className="assistant-rfq-draft__notice"
									data-tone="warning"
								>
									<AppIcon name="warning" size={18} aria-hidden="true" />
									<div>
										<strong>價格基準尚不可確認</strong>
										<p>
											價格資料尚未完整，或與目前需求單位不一致。本次只能檢視，請重新試算或修改需求。
										</p>
									</div>
								</div>
							) : null}
							{confirmed ? (
								<div
									className="assistant-rfq-snapshot__confirmed"
									role="status"
								>
									<strong>需求版本與價格範圍已固定，等待發布。</strong>
								</div>
							) : (
								<>
									<div className="assistant-rfq-snapshot__note">
										<label htmlFor="assistant-rfq-buyer-note">
											<strong>給供應商的補充說明</strong>
											<small>
												選填。這段字會原文轉給供應商，最多{" "}
												{BUYER_NOTE_MAX_LENGTH} 字。
											</small>
										</label>
										<textarea
											id="assistant-rfq-buyer-note"
											rows={3}
											maxLength={BUYER_NOTE_MAX_LENGTH}
											value={buyerNote}
											disabled={
												!preview || !confirmableBasis || !approvalEnabled
											}
											placeholder="例如：中大果，不要有壓傷"
											onChange={(event) =>
												setBuyerNote(event.currentTarget.value)
											}
										/>
										<p
											className="assistant-rfq-snapshot__note-count"
											aria-live="polite"
										>
											{buyerNote.length} / {BUYER_NOTE_MAX_LENGTH}
										</p>
									</div>
									<label className="assistant-rfq-snapshot__confirmation">
										<input
											type="checkbox"
											checked={snapshotReconfirmed}
											disabled={
												!preview || !confirmableBasis || !approvalEnabled
											}
											onChange={(event) =>
												setSnapshotReconfirmed(event.currentTarget.checked)
											}
										/>
										<span>
											<strong>我已核對這份需求版本與價格範圍</strong>
											<small>
												{!approvalEnabled
													? "此環境尚未啟用價格確認。"
													: "這次確認不包含候選名單、截止時間與通知設定。"}
											</small>
										</span>
									</label>
								</>
							)}
						</section>
					</div>
				) : null}

				{step === RECONFIRM_STEP && confirmed && preview ? (
					<div className="assistant-rfq-reconfirm">
						<header className="assistant-rfq-reconfirm__intro">
							<h3
								id="assistant-rfq-reconfirm-title"
								ref={resultHeadingRef}
								tabIndex={-1}
							>
								重新確認 RFQ 快照
							</h3>
							<span className="assistant-rfq-reconfirm__state">待買家確認</span>
						</header>

						<div className="assistant-rfq-reconfirm__grid">
							<section
								className="assistant-rfq-reconfirm__block"
								aria-labelledby="assistant-rfq-reconfirm-requirement"
							>
								<h4 id="assistant-rfq-reconfirm-requirement">1. 需求條件</h4>
								<dl>
									<div>
										<dt>品項</dt>
										<dd>{slots.item ?? "—"}</dd>
									</div>
									<div>
										<dt>數量</dt>
										<dd>{[quantity, unit].filter(Boolean).join(" ") || "—"}</dd>
									</div>
									<div>
										<dt>配送</dt>
										<dd>{deliveryCity || "—"}</dd>
									</div>
									{slots.deliveryDate ? (
										<div>
											<dt>希望到貨</dt>
											<dd>{slots.deliveryDate}</dd>
										</div>
									) : null}
									{buyerQualityStateLabel(preview.qualityKnowledgeState) ? (
										<div>
											<dt>品相</dt>
											<dd>
												{buyerQualityStateLabel(preview.qualityKnowledgeState)}
											</dd>
										</div>
									) : null}
								</dl>
								<p>供應商回價時提供實際品相與驗收條件。</p>
							</section>

							<section
								className="assistant-rfq-reconfirm__block"
								aria-labelledby="assistant-rfq-reconfirm-basis"
							>
								<h4 id="assistant-rfq-reconfirm-basis">2. 價格基準</h4>
								<dl>
									<div>
										<dt>買家希望詢價</dt>
										<dd>
											{`${formatTwd(preview.buyerRequestedRange.lowerBoundAmount)}–${formatTwd(
												preview.buyerRequestedRange.upperBoundAmount,
											)}`}
											<small>{basisSuffix}</small>
										</dd>
									</div>
								</dl>
								<div
									className="assistant-rfq-draft__notice"
									data-tone="warning"
								>
									<AppIcon name="warning" size={18} aria-hidden="true" />
									<p>詢價基準，不是成交價格</p>
								</div>
							</section>

							<section
								className="assistant-rfq-reconfirm__block"
								aria-labelledby="assistant-rfq-reconfirm-audience"
							>
								<h4 id="assistant-rfq-reconfirm-audience">3. 發布受眾</h4>
								<dl>
									<div>
										<dt>發布對象</dt>
										<dd>{recipientBreakdown}</dd>
									</div>
								</dl>
								<p>
									{hasRecipients
										? "只有這份名單上的候選會收到這筆詢價"
										: "這筆詢價沒有可指定的對象，無法發布。"}
								</p>
							</section>

							<section
								className="assistant-rfq-reconfirm__block assistant-rfq-reconfirm__block--trace"
								aria-labelledby="assistant-rfq-reconfirm-trace"
							>
								<h4 id="assistant-rfq-reconfirm-trace">4. 快照追蹤</h4>
								<dl>
									<div>
										<dt>pricePreviewId</dt>
										<dd>{preview.pricePreviewId}</dd>
									</div>
									<div>
										<dt>snapshotHash</dt>
										<dd title={confirmed.payloadHash}>
											{shortIdentity(confirmed.payloadHash)}
										</dd>
									</div>
									{confirmed.expiresAt ? (
										<div>
											<dt>有效至</dt>
											<dd>
												{`${formatTaipeiDateTime(confirmed.expiresAt)}（Asia/Taipei）`}
											</dd>
										</div>
									) : null}
								</dl>
							</section>
						</div>

						<div className="assistant-rfq-reconfirm__deadline">
							<label htmlFor="assistant-rfq-response-deadline">
								<strong>回覆期限</strong>
								<small>
									選填。供應商會在邀請信與需求頁看到這個日期；不填就不顯示期限。
								</small>
							</label>
							<input
								id="assistant-rfq-response-deadline"
								type="date"
								value={responseDeadline}
								min={todayTaipei || undefined}
								disabled={
									publicationState.status === "loading" ||
									executionState.status === "loading" ||
									executionState.status === "success"
								}
								onChange={(event) =>
									handleResponseDeadlineChange(event.currentTarget.value)
								}
								aria-describedby="assistant-rfq-response-deadline-error"
								aria-invalid={responseDeadlineError ? "true" : undefined}
							/>
							<p
								id="assistant-rfq-response-deadline-error"
								data-error={responseDeadlineError ?? undefined}
								aria-live="polite"
							>
								{responseDeadlineError}
							</p>
						</div>

						<label className="assistant-rfq-reconfirm__confirmation">
							<input
								type="checkbox"
								checked={publicationReconfirmed}
								onChange={(event) =>
									setPublicationReconfirmed(event.currentTarget.checked)
								}
							/>
							<span>
								我已重新檢查需求、希望價格與發布受眾；確認後此快照不可變
							</span>
						</label>

						<div className="assistant-rfq-draft__notice" data-tone="warning">
							<AppIcon name="warning" size={18} aria-hidden="true" />
							<p>
								任一需求、價格、候選或受眾變更，都會使此確認失效並要求重新試算
							</p>
						</div>

						{publicationState.status === "success" ? (
							<div className="assistant-rfq-reconfirm__approval" role="status">
								<strong>
									{executionState.status === "success"
										? "發布核准已建立。"
										: "發布核准已建立，尚未送出邀請。"}
								</strong>
								<dl>
									<div>
										<dt>受眾政策</dt>
										<dd>{publicationState.data.audiencePolicyId}</dd>
									</div>
									<div>
										<dt>政策版本</dt>
										<dd>{publicationState.data.audiencePolicyVersion}</dd>
									</div>
									<div>
										<dt>受眾範圍</dt>
										<dd>{`${publicationState.data.recipientCount} 位候選`}</dd>
									</div>
									<div>
										<dt>核准有效至</dt>
										<dd>
											{`${formatTaipeiDateTime(publicationState.data.expiresAt)}（Asia/Taipei）`}
										</dd>
									</div>
								</dl>
							</div>
						) : null}

						{publicationState.status === "error" ? (
							<div
								className="assistant-rfq-draft__notice"
								data-tone="warning"
								role="alert"
							>
								<AppIcon name="warning" size={18} aria-hidden="true" />
								<div>
									<strong>無法取得發布核准</strong>
									<p>{publicationState.error.detail}</p>
								</div>
							</div>
						) : null}

						{executionState.status === "success" ? (
							<div className="assistant-rfq-reconfirm__published" role="status">
								<strong>
									{executionState.data.audienceMode === "WISH_POOL"
										? "已發布至許願池"
										: "已建立發布，邀請已排入處理"}
								</strong>
								<p>
									已建立發布，正在處理。這不代表賣家已看到、已報價或已成交。
								</p>
								<dl>
									<div>
										<dt>發布編號</dt>
										<dd>{shortIdentity(executionState.data.publicationId)}</dd>
									</div>
									<div>
										<dt>發布時間</dt>
										<dd>
											{`${formatTaipeiDateTime(executionState.data.publishedAt)}（Asia/Taipei）`}
										</dd>
									</div>
								</dl>
								{executionState.data.replayed ? (
									<p>這次是同一筆發布的重試結果，沒有建立第二筆。</p>
								) : null}
							</div>
						) : null}

						{executionState.status === "error" ? (
							<div
								className="assistant-rfq-draft__notice"
								data-tone="warning"
								role="alert"
							>
								<AppIcon name="warning" size={18} aria-hidden="true" />
								<div>
									<strong>無法送出邀請</strong>
									<p>{executionState.error.detail}</p>
								</div>
							</div>
						) : null}
					</div>
				) : null}
			</div>

			<footer className="assistant-rfq-draft__footer">
				<div>
					<strong>
						{step === 0
							? "先核對目前的需求"
							: step === 1
								? "名單只供正式發布時使用"
								: step === RECONFIRM_STEP
									? "這一步只取得發布核准，還不會送出"
									: confirmed
										? "價格已固定，接著確認要發給誰"
										: "先取得並確認價格試算"}
					</strong>
					<small>重新整理後目前無法還原價格確認，需要重新核對。</small>
				</div>
				<div className="assistant-rfq-draft__footer-actions">
					{step === 0 ? (
						<>
							<Button
								variant="bordered"
								color="default"
								onClick={onReturnToConversation}
							>
								回到對話修改
							</Button>
							<Button
								disabled={!hasRequiredContext}
								onClick={() => goToStep(1)}
								endContent={
									<AppIcon name="arrow-right" size={17} aria-hidden="true" />
								}
							>
								查看預計名單
							</Button>
						</>
					) : null}
					{step === 1 ? (
						<>
							<Button
								variant="bordered"
								color="default"
								onClick={() => goToStep(0)}
							>
								上一步
							</Button>
							<Button
								disabled={candidates.length === 0}
								onClick={() => goToStep(2)}
								endContent={
									<AppIcon name="preview" size={17} aria-hidden="true" />
								}
							>
								取得價格範圍
							</Button>
						</>
					) : null}
					{step === 2 ? (
						<>
							<Button
								variant="bordered"
								color="default"
								disabled={confirmationLocked}
								onClick={() => goToStep(1)}
							>
								上一步
							</Button>
							<Button
								disabled={!canConfirm && !confirmed}
								isLoading={confirmationState.status === "loading"}
								loadingLabel="確認中…"
								onClick={() =>
									confirmed
										? goToStep(RECONFIRM_STEP)
										: void runConfirmation(true)
								}
								endContent={
									<AppIcon name="lock" size={17} aria-hidden="true" />
								}
							>
								檢視不可變 RFQ 快照
							</Button>
						</>
					) : null}
					{step === RECONFIRM_STEP ? (
						<>
							<Button
								variant="bordered"
								color="default"
								onClick={() => goToStep(2)}
							>
								返回調整
							</Button>
							<Button
								variant={
									publicationState.status === "success" ? "bordered" : "solid"
								}
								color={
									publicationState.status === "success" ? "default" : "primary"
								}
								disabled={
									!hasRecipients ||
									!publicationReconfirmed ||
									Boolean(responseDeadlineError) ||
									publicationState.status === "success"
								}
								isLoading={publicationState.status === "loading"}
								loadingLabel="取得核准中…"
								onClick={() => void runPublicationPreview()}
								aria-describedby={
									publicationState.status === "success"
										? undefined
										: "assistant-rfq-publication-meaning"
								}
								endContent={
									<AppIcon name="lock" size={17} aria-hidden="true" />
								}
							>
								確認並準備發布
							</Button>
							{publicationState.status === "success" ? (
								<Button
									disabled={executionState.status === "success"}
									isLoading={executionState.status === "loading"}
									loadingLabel="發布中…"
									onClick={() => void runPublicationExecute()}
									aria-describedby="assistant-rfq-publication-meaning"
									endContent={
										<AppIcon name="send" size={17} aria-hidden="true" />
									}
								>
									確認並送出邀請
								</Button>
							) : null}
						</>
					) : null}
				</div>
				{step === RECONFIRM_STEP ? (
					<p
						id="assistant-rfq-publication-meaning"
						className="assistant-rfq-draft__footer-meaning"
					>
						這會發布一筆詢價需求，不代表供應商已報價、供貨或成交。
					</p>
				) : null}
			</footer>
		</section>
	);
}
