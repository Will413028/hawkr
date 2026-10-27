"use client";

import { Code, ConnectError } from "@connectrpc/connect";
import type { ReactNode } from "react";
import { useCallback, useEffect, useRef, useState } from "react";
import BrandLogo from "@/components/shared/BrandLogo";
import Button from "@/components/ui/Button";
import { Input, Select, Textarea } from "@/components/ui/Input";
import { ApiClientError } from "@/lib/api/client";
import {
	ProcurementSellerResponseType,
	ProcurementTaxInclusionBasis,
} from "@/lib/codemeta/generated";
import {
	checkDraftAmountBalance,
	emptyVendorResponseDraft,
	isRfc3339,
	rfc3339ToTaipeiInput,
	taipeiInputToRfc3339,
	type VendorResponseDraft,
} from "@/lib/vendor-portal/draft";
import {
	resolveEntryStep,
	resolveInviteFailureStep,
	resolveSessionFailureStep,
	type VendorRfqFailure,
} from "@/lib/vendor-portal/flow";
import { formatTaipeiDateTime } from "@/lib/vendor-portal/format";
import { recipientQuery } from "@/lib/vendor-portal/invite-link";
import {
	exchangeVendorInvite,
	fetchVendorResponseDraft,
	fetchVendorRfq,
	fetchVendorSession,
	previewVendorInvite,
	saveVendorResponseDraft,
	submitVendorResponse,
	VENDOR_ASSURANCE_GOOGLE_VERIFIED,
	type VendorInvitePreview,
	type VendorRfqSummary,
	type VendorSessionState,
} from "@/lib/vendor-portal/repo";
import { VendorGoogleStepUp } from "./VendorGoogleStepUp";

type View =
	| { kind: "loading" }
	| { kind: "preview"; secret: string; preview: VendorInvitePreview }
	| {
			kind: "rfq";
			rfqRecipientId: string;
			rfq: VendorRfqSummary;
			session: VendorSessionState;
	  }
	| { kind: "failed"; reason: VendorRfqFailure };

const FAILURE_COPY: Record<VendorRfqFailure, { title: string; body: string }> =
	{
		link_invalid: {
			title: "連結無效",
			body: "這個連結可能已經過期或已經使用過。請確認信件中的連結是否完整，或與 Hawkr 聯繫。",
		},
		rate_limited: {
			title: "請稍後再試",
			body: "短時間內開啟太多次。請稍待幾分鐘後再開啟一次信件中的連結。",
		},
		error: {
			title: "暫時無法載入",
			body: "系統暫時無法處理，請稍後再試一次，或與 Hawkr 聯繫。",
		},
	};

/**
 * 稿 E-03 底部的七步驟軌。
 *
 * 🔴 第 3 步「正式提交」**只在真的提交之後**才點亮。存了草稿就點亮等於告訴業者他
 * 已經送出了——而草稿買家看不到，那個誤解會讓他一直等一個不會來的回應。
 */
function progressSteps(quoteStepState: string) {
	const submitted = quoteStepState === "已提交";
	return [
		{ label: "邀請驗證", state: "已完成" },
		{ label: "填寫報價", state: submitted ? "已完成" : quoteStepState },
		{ label: "正式提交", state: submitted ? "已完成" : "尚未開始" },
		{ label: "買家評估", state: "尚未開始" },
		{ label: "買家接受", state: "尚未開始" },
		{ label: "付款", state: "尚未開始" },
		{ label: "履約／結算", state: "尚未開始" },
	];
}

export function VendorRfqLanding() {
	const [view, setView] = useState<View>({ kind: "loading" });
	const mountedRef = useRef(true);
	// 進場只能跑一次。React StrictMode 在 dev 會把 effect 執行兩次，而這裡的
	// 第一個動作是打 invite preview——那支端點與 exchange 共用同一份
	// 額度（每個來源 5 分鐘最多 20 次），重複呼叫是在燒業者的額度。
	const startedRef = useRef(false);

	useEffect(() => {
		mountedRef.current = true;
		return () => {
			mountedRef.current = false;
		};
	}, []);

	// session 已知（剛兌換）就不再讀；否則（重新整理、書籤）先讀這張邀請的 session：驗證狀態以伺服器為準，
	// 不只存在 React 的記憶體。
	const loadBySession = useCallback(
		async (rfqRecipientId: string, known?: VendorSessionState) => {
			try {
				const session = known ?? (await fetchVendorSession(rfqRecipientId));
				const rfq = await fetchVendorRfq(rfqRecipientId);
				if (mountedRef.current) {
					setView({ kind: "rfq", rfqRecipientId, rfq, session });
				}
			} catch (error: unknown) {
				if (mountedRef.current) setView(resolveSessionFailureStep(error));
			}
		},
		[],
	);

	useEffect(() => {
		if (startedRef.current) return;
		startedRef.current = true;

		// secret 只存在於 fragment，server 永遠看不到它，所以這段一定在瀏覽器跑。
		const step = resolveEntryStep(window.location.hash, window.location.search);

		if (step.kind === "failed") {
			setView(step);
			return;
		}
		if (step.kind === "session") {
			void loadBySession(step.rfqRecipientId);
			return;
		}

		const { secret } = step;
		previewVendorInvite(secret)
			.then((preview) => {
				if (mountedRef.current) setView({ kind: "preview", secret, preview });
			})
			.catch((error: unknown) => {
				if (mountedRef.current) setView(resolveInviteFailureStep(error));
			});
	}, [loadBySession]);

	const handleExchange = useCallback(
		async (secret: string) => {
			setView({ kind: "loading" });
			try {
				const { rfqRecipientId, session } = await exchangeVendorInvite(secret);
				// 兌換完就把 secret 從網址清掉：它留在那裡會隨截圖、分享與瀏覽器
				// 歷史外流，而且已經沒有用處了（session cookie 接手）。網址改帶收件人 ID，
				// 重新整理時以它指名要用哪一張邀請的 session。
				window.history.replaceState(
					null,
					"",
					window.location.pathname + recipientQuery(rfqRecipientId),
				);
				await loadBySession(rfqRecipientId, session);
			} catch (error: unknown) {
				if (mountedRef.current) setView(resolveInviteFailureStep(error));
			}
		},
		[loadBySession],
	);

	return (
		<main className="mx-auto w-full max-w-[1200px] px-4 py-8">
			<PortalHeader />
			{view.kind === "loading" && <CenteredCard title="載入中…" />}
			{view.kind === "failed" && (
				<CenteredCard
					title={FAILURE_COPY[view.reason].title}
					body={FAILURE_COPY[view.reason].body}
				/>
			)}
			{view.kind === "preview" && (
				<InvitePreviewCard
					preview={view.preview}
					onOpen={() => void handleExchange(view.secret)}
				/>
			)}
			{view.kind === "rfq" && (
				<RfqWorkspace
					rfqRecipientId={view.rfqRecipientId}
					rfq={view.rfq}
					session={view.session}
				/>
			)}
		</main>
	);
}

function PortalHeader() {
	return (
		<header className="mb-6 flex flex-wrap items-center gap-3 border-b border-divider pb-4">
			<BrandLogo />
			<h1 className="m-0 font-display text-lg text-foreground md:text-xl">
				外部供應商報價入口
			</h1>
		</header>
	);
}

function CenteredCard({ title, body }: { title: string; body?: string }) {
	return (
		<div className="mx-auto flex max-w-[520px] flex-col items-center gap-3 rounded-3xl border border-divider bg-content1 p-8 text-center">
			<h2 className="m-0 font-display text-xl text-foreground">{title}</h2>
			{body && (
				<p className="m-0 max-w-[38ch] text-sm text-foreground-soft">{body}</p>
			)}
		</div>
	);
}

function InvitePreviewCard({
	preview,
	onOpen,
}: {
	preview: VendorInvitePreview;
	onOpen: () => void;
}) {
	return (
		<div className="mx-auto flex max-w-[520px] flex-col items-center gap-3 rounded-3xl border border-divider bg-content1 p-8 text-center">
			<h2 className="m-0 font-display text-xl text-foreground">
				{preview.brand} 採購詢價邀請
			</h2>
			<p className="m-0 text-sm text-foreground-soft">
				這封邀請寄給 {preview.maskedEmail}
			</p>
			<p className="m-0 text-sm text-foreground-soft">
				連結到期時間 {formatTaipeiDateTime(preview.expiresAt)}（台北時間）
			</p>
			<Button
				type="button"
				variant="solid"
				color="primary"
				size="md"
				className="mt-2"
				onClick={onOpen}
			>
				查看詢價內容
			</Button>
		</div>
	);
}

function RfqWorkspace({
	rfqRecipientId,
	rfq,
	session,
}: {
	rfqRecipientId: string;
	rfq: VendorRfqSummary;
	session: VendorSessionState;
}) {
	const controller = useResponseDraft(
		rfqRecipientId,
		rfq.publicationVersion,
		session,
	);
	return (
		<>
			<div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(0,1.4fr)_minmax(0,1fr)]">
				<BuyerRequirementCard rfq={rfq} />
				<QuoteDraftCard controller={controller} />
				<DealTruthCard />
			</div>
			<ProgressRail quoteStepState={controller.stepState} />
		</>
	);
}

function Field({ label, value }: { label: string; value: string }) {
	if (!value) return null;
	return (
		<div className="flex items-baseline justify-between gap-4 border-b border-divider py-2 last:border-b-0">
			<dt className="m-0 text-sm text-foreground-soft">{label}</dt>
			<dd className="m-0 text-sm font-medium text-foreground">{value}</dd>
		</div>
	);
}

function PriceRangeBlock({
	label,
	amounts,
	basis,
}: {
	label: string;
	amounts: string;
	basis: string;
}) {
	return (
		<div className="mt-4 rounded-2xl bg-content2 p-3">
			<p className="m-0 text-xs text-foreground-soft">{label}</p>
			<p className="mt-1 mb-0 font-medium text-foreground text-sm tabular-nums">
				{amounts}
			</p>
			{basis ? (
				<p className="mt-0.5 mb-0 text-foreground-soft text-xs">{basis}</p>
			) : null}
		</div>
	);
}

function basisLabel(quantity: string, unit: string): string {
	if (!quantity || !unit) return "";
	return `每 ${quantity} ${unit}`;
}

function rangeLabel(lower: string, upper: string): string {
	if (!lower || !upper) return "";
	return `${lower} ～ ${upper}`;
}

function BuyerRequirementCard({ rfq }: { rfq: VendorRfqSummary }) {
	const quantity = [rfq.quantity, rfq.unit].filter(Boolean).join(" ");
	const reference = rfq.referencePriceRange;
	const requested = rfq.buyerRequestedRange;
	const referenceRange = reference
		? rangeLabel(reference.lowerBoundAmount, reference.upperBoundAmount)
		: "";
	const requestedRange = requested
		? rangeLabel(requested.lowerBoundAmount, requested.upperBoundAmount)
		: "";
	return (
		<section className="rounded-3xl border border-divider bg-content1 p-6">
			<h2 className="mt-0 mb-3 font-display text-base text-foreground">
				買家需求
			</h2>
			<dl className="m-0">
				<Field label="品項" value={rfq.itemLabel || rfq.item} />
				<Field label="數量" value={quantity} />
				<Field label="包裝" value={rfq.packaging} />
				<Field label="配送地" value={rfq.deliveryCity} />
				<Field label="指定送達" value={rfq.deliveryDate} />
				<Field label="回覆期限" value={rfq.responseDeadline} />
			</dl>
			{rfq.buyerNote ? (
				<div className="mt-4 rounded-2xl bg-content2 p-3">
					<p className="m-0 text-xs text-foreground-soft">買家的補充說明</p>
					<p className="mt-1 mb-0 whitespace-pre-wrap text-sm text-foreground">
						{rfq.buyerNote}
					</p>
				</div>
			) : null}
			{reference && referenceRange ? (
				<PriceRangeBlock
					label="Pricebook 市場行情"
					amounts={referenceRange}
					basis={basisLabel(reference.basisQuantity, reference.basisUnit)}
				/>
			) : null}
			{requested && requestedRange ? (
				<PriceRangeBlock
					label="買家希望區間"
					amounts={[requested.currency, requestedRange]
						.filter(Boolean)
						.join(" ")}
					basis={basisLabel(requested.basisQuantity, requested.basisUnit)}
				/>
			) : null}
			{reference || requested ? (
				<p className="mt-4 mb-0 text-foreground-soft text-xs">
					這兩個區間只是報價脈絡，不是成交價格，也不是可接受的上下限。報價高於或低於區間都可以送出，由買家決定。
				</p>
			) : null}
		</section>
	);
}

/**
 * 業者看到的回價種類與含稅申報文案。
 *
 * ⚠️ 刻意**不用** CodeMeta 生成的 `PROCUREMENT_*_LABEL`：那組字是 ADM 營運視角的
 * 稱呼（「自行報價」「未稅」），寫給看得懂內部術語的人。這一頁的讀者是第一次接觸
 * Hawkr 的站外供應商，他要判斷的是「我該選哪一個」，所以文案從他的角度寫。
 * 值域本身仍然來自生成的常數——會變的是措辭，不是列舉值。
 */
const RESPONSE_TYPE_OPTIONS = [
	{
		value: ProcurementSellerResponseType.MANUAL_QUOTE,
		label: "我自己開條件報價",
	},
	{
		value: ProcurementSellerResponseType.ACCEPT_NON_PRICE_TERMS_WITH_QUOTE,
		label: "接受買家的非價格條款，另外報價",
	},
] as const;

const TAX_BASIS_OPTIONS = [
	{
		value: ProcurementTaxInclusionBasis.TAX_EXCLUDED,
		label: "總金額未含稅",
	},
	{
		value: ProcurementTaxInclusionBasis.TAX_INCLUDED,
		label: "總金額已含稅",
	},
] as const;

type DraftNoticeTone = "info" | "success" | "warning";

interface DraftNotice {
	tone: DraftNoticeTone;
	text: string;
	/** 版本衝突時才給——重新載入是唯一不會覆蓋別人內容的解法。 */
	offerReload?: boolean;
}

interface ResponseDraftController {
	draft: VendorResponseDraft;
	phase: "loading" | "idle" | "saving";
	notice: DraftNotice | null;
	stepState: string;
	patch: (partial: Partial<VendorResponseDraft>) => void;
	save: () => void;

	rfqRecipientId: string;
	/** 伺服器給的 Google nonce；"" ＝ session 不存在。 */
	googleNonce: string;
	/** "" ＝ 還沒做 Google 驗證（session 停在 INVITE_ONLY）。 */
	assurance: string;
	markVerified: (assurance: string) => void;
	submitPhase: "idle" | "submitting" | "submitted";
	submittedVersion: string;
	submit: () => void;
}

/**
 * 中間欄的草稿 ＋ 正式提交狀態機。
 *
 * 🔴 兩者是**不同的東西**，不要合併：存草稿可以一直覆寫，assurance 只需要
 * `INVITE_ONLY`（業者點邀請信進來就有）；正式提交建立的是 immutable 的報價版本，
 * 買家接受 exact version 即成交 ⇒ 需要 `GOOGLE_VERIFIED`，而且送出之後**不能改**，
 * 只能撤回重送。
 */
function useResponseDraft(
	rfqRecipientId: string,
	publicationVersion: string,
	initialSession: VendorSessionState,
): ResponseDraftController {
	const [draft, setDraft] = useState<VendorResponseDraft>(
		emptyVendorResponseDraft,
	);
	// CAS 標記。"" ＝「我這邊還沒有草稿」，也是第一次 PUT 要送出去的值。
	const [draftVersion, setDraftVersion] = useState("");
	const [phase, setPhase] = useState<"loading" | "idle" | "saving">("loading");
	const [notice, setNotice] = useState<DraftNotice | null>(null);
	// 驗證狀態與 nonce 的來源是伺服器（Exchange／GetSession）；這裡是畫面當下的副本。
	const [session, setSession] = useState(initialSession);
	const { assurance, googleNonce } = session;
	const [submitPhase, setSubmitPhase] = useState<
		"idle" | "submitting" | "submitted"
	>("idle");
	const [submittedVersion, setSubmittedVersion] = useState("");
	// 🔴 冪等鍵在「按下送出的那一刻」產生並**固定住**，重試沿用同一把。
	// 每次重試換一把鍵會產生第二個正式版本——而買家那邊會看到兩筆報價。
	const idempotencyKeyRef = useRef("");
	const mountedRef = useRef(true);

	useEffect(() => {
		mountedRef.current = true;
		return () => {
			mountedRef.current = false;
		};
	}, []);

	useEffect(() => {
		let active = true;
		// 開頁就把既有草稿讀回來：業者關掉分頁、隔天再點同一封信回來，看不到自己
		// 填過的東西等於這個功能沒有意義。沒有草稿時後端回 404，收斂成 null。
		fetchVendorResponseDraft(rfqRecipientId)
			.then((snapshot) => {
				if (!active) return;
				setPhase("idle");
				if (!snapshot) return;
				setDraft(snapshot.draft);
				setDraftVersion(snapshot.draftVersion);
				setNotice({
					tone: "info",
					text: `已帶回您上次儲存的草稿（${formatTaipeiDateTime(snapshot.updatedAt)} 台北時間）。`,
				});
			})
			.catch(() => {
				if (!active) return;
				setPhase("idle");
				setNotice({
					tone: "warning",
					text: "讀取先前的草稿時發生問題。您仍然可以在這裡填寫，但送出前請先重新載入確認沒有覆蓋掉舊內容。",
				});
			});
		return () => {
			active = false;
		};
	}, [rfqRecipientId]);

	const patch = useCallback((partial: Partial<VendorResponseDraft>) => {
		setDraft((current) => ({ ...current, ...partial }));
	}, []);

	const save = useCallback(() => {
		setPhase("saving");
		setNotice(null);
		saveVendorResponseDraft(rfqRecipientId, draftVersion, draft)
			.then((snapshot) => {
				if (!mountedRef.current) return;
				setDraft(snapshot.draft);
				setDraftVersion(snapshot.draftVersion);
				setPhase("idle");
				setNotice({
					tone: "success",
					text: `已儲存草稿，尚未正式提交（${formatTaipeiDateTime(snapshot.updatedAt)} 台北時間）。`,
				});
			})
			.catch((error: unknown) => {
				if (!mountedRef.current) return;
				setPhase("idle");
				const status = error instanceof ApiClientError ? error.status : 0;
				if (status === 409) {
					// 自動覆蓋是最糟的處理：另一個分頁存進去的可能才是業者要的那份。
					setNotice({
						tone: "warning",
						text: "這份草稿在別的地方被存過了（多半是您自己的另一個分頁）。這裡的內容沒有寫進去，也不會自動覆蓋。請重新載入取得最新的草稿再改。",
						offerReload: true,
					});
					return;
				}
				setNotice({
					tone: "warning",
					text: "草稿沒有存成功，請稍後再試一次。您填的內容還留在這個畫面上。",
				});
			});
	}, [rfqRecipientId, draftVersion, draft]);

	const markVerified = useCallback((next: string) => {
		setSession((prev) => ({ ...prev, assurance: next }));
	}, []);

	const submit = useCallback(() => {
		if (idempotencyKeyRef.current === "") {
			idempotencyKeyRef.current = crypto.randomUUID();
		}
		setSubmitPhase("submitting");
		setNotice(null);
		submitVendorResponse(
			rfqRecipientId,
			idempotencyKeyRef.current,
			{
				publicationVersion,
				// "" ＝「我還沒有正式回價」。撤回重送不在本畫面的範圍內。
				sellerResponseVersion: "",
			},
			draft,
		)
			.then((result) => {
				if (!mountedRef.current) return;
				setSubmitPhase("submitted");
				setSubmittedVersion(result.sellerResponseVersion);
			})
			.catch((error: unknown) => {
				if (!mountedRef.current) return;
				setSubmitPhase("idle");
				const status = error instanceof ApiClientError ? error.status : 0;
				if (status === 409) {
					// 這張 RFQ 在業者填的期間被買家改過。⛔ 不可自動用新版本重送——
					// 那等於替業者對一份他沒看過的需求報價。
					setNotice({
						tone: "warning",
						text: "這張需求在您填寫期間被買家更新了，報價沒有送出。請重新載入看過最新內容再送。",
						offerReload: true,
					});
					return;
				}
				if (status === 403) {
					// 閘道說這個 session 沒驗證：以伺服器現在的狀態為準重讀。讀到 unauthenticated ＝
					// session 已經不在，連 nonce 一起丟掉（畫面說明要重新開啟信件連結、不畫 Google 按鈕）；
					// 其他讀取失敗只退回未驗證。
					fetchVendorSession(rfqRecipientId)
						.then((fresh) => {
							if (mountedRef.current) setSession(fresh);
						})
						.catch((refetchError: unknown) => {
							if (!mountedRef.current) return;
							const gone =
								refetchError instanceof ConnectError &&
								refetchError.code === Code.Unauthenticated;
							setSession((prev) => ({
								assurance: "",
								googleNonce: gone ? "" : prev.googleNonce,
							}));
						});
					setNotice({
						tone: "warning",
						text: "驗證已失效，報價沒有送出。請重新完成 Google 帳號驗證再送一次。",
					});
					return;
				}
				setNotice({
					tone: "warning",
					text: "報價沒有送出，請稍後再試一次。您填的內容還留在這個畫面上。",
				});
			});
	}, [rfqRecipientId, publicationVersion, draft]);

	const stepState =
		submitPhase === "submitted"
			? "已提交"
			: phase === "loading"
				? "讀取中"
				: draftVersion
					? "已存草稿"
					: "進行中";

	return {
		draft,
		phase,
		notice,
		stepState,
		patch,
		save,
		rfqRecipientId,
		googleNonce,
		assurance,
		markVerified,
		submitPhase,
		submittedVersion,
		submit,
	};
}

const NOTICE_CLASS: Record<DraftNoticeTone, string> = {
	info: "bg-content2 text-foreground-soft",
	success: "bg-success-50 text-success-700",
	warning: "bg-warning-50 text-warning-700",
};

function DraftNoticeBox({ notice }: { notice: DraftNotice }) {
	return (
		<div
			role="status"
			aria-live="polite"
			className={`flex flex-col items-start gap-2 rounded-2xl p-3 text-xs ${NOTICE_CLASS[notice.tone]}`}
		>
			<span>{notice.text}</span>
			{notice.offerReload && (
				<Button
					type="button"
					variant="bordered"
					color="default"
					size="sm"
					onClick={() => window.location.reload()}
				>
					重新載入
				</Button>
			)}
		</div>
	);
}

const AMOUNT_HINT: Record<string, string> = {
	unbalanced:
		"貨款 ＋ 運費 ＋ 稅額目前不等於總金額。草稿可以照樣儲存，正式提交時才會擋。",
	balanced: "貨款 ＋ 運費 ＋ 稅額等於總金額。",
};

function QuoteDraftCard({
	controller,
}: {
	controller: ResponseDraftController;
}) {
	const { draft, phase, notice, patch, save } = controller;
	const amountHint = AMOUNT_HINT[checkDraftAmountBalance(draft)];
	const busy = phase !== "idle";

	return (
		<section className="rounded-3xl border border-divider bg-content1 p-6">
			<h2 className="mt-0 mb-3 font-display text-base text-foreground">
				填寫報價（草稿）
			</h2>
			<p className="mt-0 mb-4 rounded-2xl bg-content2 p-3 text-xs text-foreground-soft">
				{
					"這裡填的內容先存成草稿，買家看不到。要讓買家看到、可以成交，請在下方完成 Google 帳號驗證後正式提交。"
				}
			</p>

			<form
				className="flex flex-col gap-4"
				onSubmit={(event) => {
					event.preventDefault();
					save();
				}}
			>
				<Select
					label="回價方式"
					value={draft.responseType}
					disabled={busy}
					onChange={(event) => patch({ responseType: event.target.value })}
				>
					{RESPONSE_TYPE_OPTIONS.map((option) => (
						<option key={option.value} value={option.value}>
							{option.label}
						</option>
					))}
					{/* 後端存過一個這一版還不認得的值時，仍然把它列出來，不要靜默顯示成空白。 */}
					{!RESPONSE_TYPE_OPTIONS.some(
						(option) => option.value === draft.responseType,
					) && <option value={draft.responseType}>{draft.responseType}</option>}
				</Select>

				<DraftFieldGroup title="實際品相">
					<Input
						label="品相等級"
						placeholder="例：特級、A 級"
						value={draft.offeredQuality.grade}
						disabled={busy}
						onChange={(event) =>
							patch({
								offeredQuality: {
									...draft.offeredQuality,
									grade: event.target.value,
								},
							})
						}
					/>
					<Textarea
						label="驗貨備註"
						minRows={2}
						placeholder="例：抽驗 3 箱，允收糖度 11 度以上"
						value={draft.offeredQuality.inspectionNotes}
						disabled={busy}
						onChange={(event) =>
							patch({
								offeredQuality: {
									...draft.offeredQuality,
									inspectionNotes: event.target.value,
								},
							})
						}
					/>
				</DraftFieldGroup>

				<DraftFieldGroup title="計價與供貨">
					<DraftFieldRow>
						<Input
							label="計價基準數量"
							inputMode="decimal"
							placeholder="例：1"
							value={draft.unitBasis.quantity}
							disabled={busy}
							onChange={(event) =>
								patch({
									unitBasis: {
										...draft.unitBasis,
										quantity: event.target.value,
									},
								})
							}
						/>
						<Input
							label="計價基準單位"
							placeholder="例：KG"
							value={draft.unitBasis.unit}
							disabled={busy}
							onChange={(event) =>
								patch({
									unitBasis: { ...draft.unitBasis, unit: event.target.value },
								})
							}
						/>
					</DraftFieldRow>
					<DraftFieldRow>
						<Input
							label="供貨數量"
							inputMode="decimal"
							placeholder="例：300"
							value={draft.offeredQuantity}
							disabled={busy}
							onChange={(event) =>
								patch({ offeredQuantity: event.target.value })
							}
						/>
						<Input
							label="供貨數量單位"
							placeholder="例：KG"
							value={draft.offeredQuantityUnit}
							disabled={busy}
							onChange={(event) =>
								patch({ offeredQuantityUnit: event.target.value })
							}
						/>
					</DraftFieldRow>
					<Input
						label="換算依據"
						description="供貨單位與計價單位不同時，寫清楚怎麼換算。"
						placeholder="例：1 箱 = 10 KG"
						value={draft.conversionBasis}
						disabled={busy}
						onChange={(event) => patch({ conversionBasis: event.target.value })}
					/>
				</DraftFieldGroup>

				<DraftFieldGroup title="金額">
					<DraftFieldRow>
						<Input
							label="幣別"
							placeholder="TWD"
							value={draft.currency}
							disabled={busy}
							onChange={(event) => patch({ currency: event.target.value })}
						/>
						<Select
							label="含稅申報"
							value={draft.taxInclusionBasis}
							disabled={busy}
							onChange={(event) =>
								patch({ taxInclusionBasis: event.target.value })
							}
						>
							{TAX_BASIS_OPTIONS.map((option) => (
								<option key={option.value} value={option.value}>
									{option.label}
								</option>
							))}
							{!TAX_BASIS_OPTIONS.some(
								(option) => option.value === draft.taxInclusionBasis,
							) && (
								<option value={draft.taxInclusionBasis}>
									{draft.taxInclusionBasis}
								</option>
							)}
						</Select>
					</DraftFieldRow>
					<DraftFieldRow>
						<Input
							label="貨款金額"
							inputMode="decimal"
							placeholder="例：30000"
							value={draft.goodsAmount}
							disabled={busy}
							onChange={(event) => patch({ goodsAmount: event.target.value })}
						/>
						<Input
							label="運費金額"
							inputMode="decimal"
							placeholder="例：800"
							value={draft.shippingAmount}
							disabled={busy}
							onChange={(event) =>
								patch({ shippingAmount: event.target.value })
							}
						/>
					</DraftFieldRow>
					<DraftFieldRow>
						<Input
							label="稅額"
							inputMode="decimal"
							placeholder="例：1540"
							value={draft.taxAmount}
							disabled={busy}
							onChange={(event) => patch({ taxAmount: event.target.value })}
						/>
						<Input
							label="總金額"
							inputMode="decimal"
							placeholder="例：32340"
							value={draft.totalAmount}
							disabled={busy}
							onChange={(event) => patch({ totalAmount: event.target.value })}
						/>
					</DraftFieldRow>
					{amountHint && (
						<p className="m-0 text-xs text-foreground-soft">{amountHint}</p>
					)}
				</DraftFieldGroup>

				<DraftFieldGroup title="交期與效期">
					<Input
						label="備貨天數"
						inputMode="numeric"
						placeholder="例：3"
						value={draft.leadTimeDays}
						disabled={busy}
						onChange={(event) => patch({ leadTimeDays: event.target.value })}
					/>
					<Rfc3339Field
						label="承諾送達時間"
						description="台北時間。選好之後會存成含時區的標準格式，與平台賣家送出的一致。"
						value={draft.promisedDeliveryAt}
						disabled={busy}
						onChange={(next) => patch({ promisedDeliveryAt: next })}
					/>
					<Rfc3339Field
						label="報價有效期限"
						description="台北時間。正式提交時這一欄必填，而且必須晚於當下。"
						value={draft.validUntil}
						disabled={busy}
						onChange={(next) => patch({ validUntil: next })}
					/>
				</DraftFieldGroup>

				<DraftFieldGroup title="說明">
					<Textarea
						label="條款備註"
						minRows={2}
						placeholder="例：雨天順延一日"
						value={draft.termsNote}
						disabled={busy}
						onChange={(event) => patch({ termsNote: event.target.value })}
					/>
					<Textarea
						label="與買家需求的差異"
						minRows={2}
						description="包裝、規格、數量與買家需求不同的地方寫在這裡。"
						placeholder="例：改用塑膠籃出貨"
						value={draft.deviationDisclosure}
						disabled={busy}
						onChange={(event) =>
							patch({ deviationDisclosure: event.target.value })
						}
					/>
				</DraftFieldGroup>

				{notice && <DraftNoticeBox notice={notice} />}

				<div className="flex flex-col gap-2">
					<Button
						type="submit"
						variant="solid"
						color="primary"
						size="md"
						fullWidth
						isLoading={phase === "saving"}
						loadingLabel="儲存中"
						disabled={busy}
					>
						儲存草稿
					</Button>
					<SubmitBlock controller={controller} />
				</div>
			</form>
		</section>
	);
}

/**
 * 送出區：驗證 → 二次確認 → 正式提交。
 *
 * 🔴 **二次確認不是禮貌，是必要的**：送出建立的是 immutable 的報價版本，買家接受
 * exact version 即成交 ⇒ 按錯一下沒有「改一下再存」可以救，只能撤回重送。把它做成
 * 一個需要明確再按一次的動作，和旁邊的「儲存草稿」在手感上分開。
 */
function SubmitBlock({ controller }: { controller: ResponseDraftController }) {
	const {
		rfqRecipientId,
		googleNonce,
		assurance,
		markVerified,
		submitPhase,
		submittedVersion,
		submit,
	} = controller;
	const [confirming, setConfirming] = useState(false);

	if (submitPhase === "submitted") {
		return (
			<div className="rounded-2xl bg-success-soft p-3">
				<p className="m-0 text-xs text-foreground">
					{`報價已正式提交（第 ${submittedVersion} 版），買家現在看得到了。`}
				</p>
				<p className="m-0 mt-1 text-xs text-foreground-soft">
					{"這一版不能再修改。需要改價請聯絡 Hawkr 的窗口撤回後重送。"}
				</p>
			</div>
		);
	}

	if (assurance !== VENDOR_ASSURANCE_GOOGLE_VERIFIED) {
		return (
			<VendorGoogleStepUp
				rfqRecipientId={rfqRecipientId}
				googleNonce={googleNonce}
				onVerified={markVerified}
			/>
		);
	}

	if (!confirming) {
		return (
			<>
				<Button
					type="button"
					variant="bordered"
					color="primary"
					size="md"
					fullWidth
					onClick={() => setConfirming(true)}
				>
					正式提交報價
				</Button>
				<p className="m-0 text-xs text-foreground-soft">
					{"已完成 Google 帳號驗證。提交後買家就看得到這份報價。"}
				</p>
			</>
		);
	}

	return (
		<div className="flex flex-col gap-2 rounded-2xl border border-warning p-3">
			<p className="m-0 text-xs text-foreground">
				{"確定要送出嗎？送出後會建立一份買家看得到、您不能再修改的報價版本。"}
			</p>
			<div className="flex gap-2">
				<Button
					type="button"
					variant="solid"
					color="primary"
					size="md"
					fullWidth
					isLoading={submitPhase === "submitting"}
					loadingLabel="提交中"
					onClick={submit}
				>
					確定送出
				</Button>
				<Button
					type="button"
					variant="bordered"
					color="default"
					size="md"
					fullWidth
					disabled={submitPhase === "submitting"}
					onClick={() => setConfirming(false)}
				>
					再想一下
				</Button>
			</div>
		</div>
	);
}

function DraftFieldGroup({
	title,
	children,
}: {
	title: string;
	children: ReactNode;
}) {
	return (
		<fieldset className="m-0 flex flex-col gap-3 border-0 p-0">
			<legend className="mb-1 p-0 text-sm font-medium text-foreground">
				{title}
			</legend>
			{children}
		</fieldset>
	);
}

/** 桌機兩欄、手機一欄——這一頁業者多半是用手機開信點進來的。 */
function DraftFieldRow({ children }: { children: ReactNode }) {
	return <div className="grid gap-3 sm:grid-cols-2">{children}</div>;
}

/**
 * 產生 RFC3339 的日期時間欄。
 *
 * 用 `datetime-local` 選擇器而不是自由文字：正式提交收的格式**平台側早就定了**
 * （`hawkr_seller` 的 `isRfc3339()`），草稿的用途就是那次提交的預填，兩邊格式
 * 不同的話業者寫的「9/20 早上」永遠變不成合法提交——而正式提交尚未開放，這個錯要
 * 拖到很久以後才會被發現，中間存的草稿全都得重填。
 *
 * ⛔ 但草稿階段**不擋存**：格式不對只提示，按下儲存照樣要存得進去（與金額恆等式
 * 同一個原則）。既有草稿裡不是 RFC3339 的舊值也**不清空、不報錯**，原樣顯示在下面
 * 讓業者自己決定要不要用選擇器換掉。
 */
function Rfc3339Field({
	label,
	description,
	value,
	disabled,
	onChange,
}: {
	label: string;
	description: string;
	value: string;
	disabled: boolean;
	onChange: (next: string) => void;
}) {
	const legacy = value.trim() !== "" && !isRfc3339(value);
	return (
		<div className="flex w-full flex-col gap-1.5">
			<Input
				type="datetime-local"
				label={label}
				description={description}
				value={rfc3339ToTaipeiInput(value)}
				disabled={disabled}
				onChange={(event) => onChange(taipeiInputToRfc3339(event.target.value))}
			/>
			{legacy && (
				<p className="m-0 rounded-2xl bg-warning-50 p-2 text-xs text-warning-700">
					{`目前存著的是「${value}」，不是正式提交收的標準格式。草稿照樣存得起來，但正式提交會被擋——請用上面的選擇器重新指定一次。`}
				</p>
			)}
		</div>
	);
}

function DealTruthCard() {
	return (
		<section className="rounded-3xl border border-divider bg-content1 p-6">
			<h2 className="mt-0 mb-3 font-display text-base text-foreground">
				回價不是成交
			</h2>
			<p className="m-0 text-sm text-foreground-soft">
				供應商回價不是成交；買家接受指定版本後才會固定 acceptedDealPrice。
			</p>
		</section>
	);
}

function ProgressRail({ quoteStepState }: { quoteStepState: string }) {
	return (
		<ol className="mt-4 flex flex-wrap gap-3 rounded-3xl border border-divider bg-content1 p-6">
			{progressSteps(quoteStepState).map((step, index) => (
				<li
					key={step.label}
					// 測試要能問「第 N 格說什麼」。沒有這個定位點，斷言只能在整條軌上找
					// 文字，而第 3～7 格的狀態字完全相同——那種斷言擋不住第 3 格被改動。
					data-testid={`progress-step-${index + 1}`}
					className="flex min-w-[120px] flex-col gap-1 text-sm"
				>
					<span className="text-foreground">
						{index + 1} {step.label}
					</span>
					<span className="text-xs text-foreground-soft">{step.state}</span>
				</li>
			))}
		</ol>
	);
}
