"use client";

/**
 * S03｜AMEGO 串接精靈（58A §S03）。
 *
 * **不做換頁 wizard**（兩端 repo 都查無 stepper 元件）：單頁 ＋ 錨點導覽 ＋ 分段
 * meter，全部落在既有語彙內。五步中只有第 2、3、4 步有互動。
 *
 * 資料源：端點 23（本頁主體）＋ 端點 17（步驟 1 的核定統編與法定名稱、以及
 * blocked gate 的判準）。
 *
 * 🔴 **blocked gate 只看端點 17 的 `reviewStatus`，不看 `9330`。**
 * 已核准但還沒開始串接的賣家一樣會拿到 `9330`——把它併進 gate 判準，會把正要來
 * 設定的人擋在門外出不去，而那正是這頁存在的理由。
 *
 * 🔴 **步驟 4／5 的分界是 `automationCutoverAt`，不是 `status`**（見 `amegoSetupState`）。
 *
 * 🔴 **讀業務碼不讀 HTTP status**：端點 25 驗證失敗是 HTTP 200 ＋ `9331`。
 * 只看 status 會把失敗畫成成功，而賣家會以為自己串好了、然後訂單一張發票都開不出來。
 */

import * as React from "react";
import { BackLink } from "@/components/shared/BackLink";
import Link from "@/components/shared/Link";
import { SellerPageShell } from "@/components/shared/SellerPageShell";
import Button from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import { Input } from "@/components/ui/Input";
import {
	InvoiceProviderAccountStatus,
	SellerTaxReviewStatus,
} from "@/lib/codemeta/enums";
import { formatTaipeiDateTime } from "@/lib/date/taipei";
import { useSellerTaxProfile } from "@/lib/hooks/useSellerInvoiceRemedy";
import {
	providerExpectedVersion,
	useProviderAccountActions,
	useSellerProviderAccount,
} from "@/lib/hooks/useSellerProviderAccount";
import { invoiceErrorMessage } from "@/lib/invoices/buyer-repo";
import { invoiceErrorCopy } from "@/lib/invoices/errorCopy";
import { AMEGO_CONSENT_VERSION } from "@/lib/invoices/seller-repo";
import { SellerCardGridSkeleton } from "../../components/SellerSkeletons";
import { amegoProgressPercent, amegoSetupState } from "./amegoSetupState";
import { MaskedSecretField } from "./MaskedSecretField";

const OVERVIEW_HREF = "/seller-center/invoices";
const TAX_STATUS_HREF = "/seller-center/invoices/tax-status";

const STEPS = [
	{ id: "amego-1", label: "確認統編" },
	{ id: "amego-2", label: "輸入 AppKey" },
	{ id: "amego-3", label: "測試連線" },
	{ id: "amego-4", label: "設定啟用時間" },
	{ id: "amego-5", label: "完成" },
] as const;

// RESUBMITTABLE_STATUSES 逐字對齊後端 ResubmitProviderCredential 的 allowed-from。
// 兩邊分家的後果是賣家看到一顆按下去必定 rows=0 的按鈕（backend #3089 第 15 項）。
const RESUBMITTABLE_STATUSES: ReadonlySet<string> = new Set([
	InvoiceProviderAccountStatus.NOT_CONFIGURED,
	InvoiceProviderAccountStatus.CREDENTIAL_SUBMITTED,
	InvoiceProviderAccountStatus.CREDENTIAL_VERIFYING,
	InvoiceProviderAccountStatus.INVALID_CREDENTIAL,
	InvoiceProviderAccountStatus.PROVIDER_REJECTED,
	InvoiceProviderAccountStatus.PROVIDER_UNAVAILABLE,
]);

export function SellerAmegoSetupPage() {
	const {
		profile,
		notStarted: taxNotStarted,
		loading: taxLoading,
	} = useSellerTaxProfile();
	const {
		account,
		notStarted,
		loading: accountLoading,
		error,
		refresh,
	} = useSellerProviderAccount();

	const expectedVersion = providerExpectedVersion(account);
	const actions = useProviderAccountActions(expectedVersion);

	const status = notStarted
		? InvoiceProviderAccountStatus.NOT_STARTED
		: (account?.status ?? InvoiceProviderAccountStatus.NOT_STARTED);
	const hasCutover = Boolean(account?.automationCutoverAt);
	const state = amegoSetupState(status, hasCutover);

	const [appKey, setAppKey] = React.useState("");
	const [editingKey, setEditingKey] = React.useState(false);
	const [cutoverLocal, setCutoverLocal] = React.useState("");
	const [cutoverError, setCutoverError] = React.useState("");
	const [confirmOpen, setConfirmOpen] = React.useState(false);
	const [detailsOpen, setDetailsOpen] = React.useState(false);

	// 驗證失敗（9331）時自動展開技術資訊，讓賣家不必自己找 request id。
	const verifyCopy = invoiceErrorCopy(actions.verifyError);
	React.useEffect(() => {
		if (actions.verifyError) setDetailsOpen(true);
	}, [actions.verifyError]);

	if (taxLoading || accountLoading) {
		return (
			<SellerPageShell title="AMEGO 串接" eyebrow="發票與稅籍">
				<SellerCardGridSkeleton variant="panel" count={4} />
			</SellerPageShell>
		);
	}

	// 🔴 gate 判準只看端點 17 —— 見檔頭。
	const approved =
		!taxNotStarted &&
		profile?.reviewStatus === SellerTaxReviewStatus.APPROVED_INVOICE_REQUIRED;
	if (!approved) {
		return (
			<SellerPageShell title="AMEGO 串接" eyebrow="發票與稅籍">
				<BackLink fallbackHref={OVERVIEW_HREF}>回發票與稅籍</BackLink>
				<div className="seller-card mt-4 p-6 text-center">
					<p className="text-sm font-extrabold text-foreground">
						請先完成稅務身分審核，才能串接 AMEGO。
					</p>
					<div className="mt-4">
						<Button href={TAX_STATUS_HREF} variant="solid" size="sm">
							前往稅務身分
						</Button>
					</div>
				</div>
			</SellerPageShell>
		);
	}

	if (error) {
		return (
			<SellerPageShell title="AMEGO 串接" eyebrow="發票與稅籍">
				<div className="seller-card mt-4 p-6 text-center">
					<p className="text-sm font-extrabold text-foreground">
						讀取串接資料時發生問題。
					</p>
					<div className="mt-4">
						<Button variant="bordered" size="sm" onClick={() => void refresh()}>
							重試
						</Button>
					</div>
				</div>
			</SellerPageShell>
		);
	}

	const approvedTaxId = profile?.taxIdentity.taxId ?? "";
	const keyEditing = editingKey || state.credentialEditable;
	const credentialCopy = invoiceErrorCopy(actions.credentialError);
	const activateCopy = invoiceErrorCopy(actions.activateError);

	const submitCredential = () => {
		if (!appKey.trim()) return;
		// 🔴 分流判準是 **status**，不是「有沒有 last4」（backend #3089 第 15 項）。
		//
		// 舊條件 `account?.credentialLast4 && !state.credentialEditable` 的註解寫
		// 「已 ACTIVE／DEGRADED」，但它實際涵蓋了 CREDENTIAL_VERIFYING／
		// CREDENTIAL_SUBMITTED／PROVIDER_UNAVAILABLE 三態（那些狀態也有 last4）
		// ⇒ 打錯 AppKey 的賣家被導向端點 27 輪替，而輪替的 allowed-from **只有 ACTIVE**
		// ⇒ **必然 409／9322「資料已被更新，請重新整理後再送出」**。
		// 版本其實沒衝突，重整一百次都一樣 —— 他沒有任何自助修復路徑。
		//
		// 值域逐字取自後端兩支 CAS 的 allowed-from（db/queries/invoice_provider_accounts.sql）：
		//   端點 27 StartProviderCredentialRotation ← status = 'ACTIVE'
		//   端點 24 ResubmitProviderCredential      ← NOT_CONFIGURED / CREDENTIAL_SUBMITTED /
		//                                             CREDENTIAL_VERIFYING / INVALID_CREDENTIAL /
		//                                             PROVIDER_REJECTED / PROVIDER_UNAVAILABLE
		if (account?.status === InvoiceProviderAccountStatus.ACTIVE) {
			actions.rotate(appKey.trim());
			return;
		}
		actions.saveCredential({
			sellerTaxId: approvedTaxId,
			appKey: appKey.trim(),
		});
	};

	// canEditKey ＝ 這個狀態下送出去真的有機會成功。
	//
	// ⚠️ DEGRADED／ROTATION_PENDING／SUSPENDED／REVOKED 兩支端點的 allowed-from
	// **都不含** ⇒ 讓表單可編輯只會給出一顆必然 rows=0 的按鈕（而 rows=0 回的是
	// 「資料已被更新」，讀起來像是暫時性問題）。這裡不畫它，比畫一顆按不動的誠實。
	const canEditKey =
		account?.status === InvoiceProviderAccountStatus.ACTIVE ||
		RESUBMITTABLE_STATUSES.has(
			account?.status ?? InvoiceProviderAccountStatus.NOT_CONFIGURED,
		);

	const submitActivation = () => {
		const iso = taipeiLocalToIso(cutoverLocal);
		if (!iso) {
			setCutoverError("請選擇啟用時間。");
			return;
		}
		if (new Date(iso).getTime() <= Date.now()) {
			// 前端就擋下不發請求：切點只能是未來（58A §S03 驗收 2）。
			setCutoverError(
				"啟用時間必須晚於現在。較早付款的訂單不會回頭改用新設定。",
			);
			return;
		}
		setCutoverError("");
		setConfirmOpen(false);
		actions.activate(iso);
	};

	return (
		<SellerPageShell
			title="AMEGO 串接"
			eyebrow="發票與稅籍"
			subtitle="串接光貿 AMEGO 之後，付款成功的訂單會自動開立電子發票。"
			toolbar={<AmegoProgressMeter step={state.step} />}
		>
			<BackLink fallbackHref={OVERVIEW_HREF}>回發票與稅籍</BackLink>

			<nav className="seller-cert__panel-anchors mt-4" aria-label="串接步驟">
				<ol className="flex gap-2 overflow-x-auto">
					{STEPS.map((s, i) => (
						<li key={s.id} className="shrink-0">
							<a
								href={`#${s.id}`}
								className="flex items-center gap-2 px-3 py-2 rounded-hkui-lg bg-content2 border border-divider text-[0.6875rem] font-extrabold text-foreground"
								aria-current={state.step === i + 1 ? "step" : undefined}
							>
								<span className="w-5 h-5 rounded-full bg-hk-accent text-white text-[0.625rem] grid place-items-center shrink-0">
									{i + 1}
								</span>
								{s.label}
							</a>
						</li>
					))}
				</ol>
			</nav>

			{account?.statusNotice ? (
				<p
					className="mt-4 p-[12px_14px] rounded-hkui-lg bg-content2 border border-divider text-[0.6875rem] leading-relaxed text-foreground/80"
					aria-live="polite"
				>
					{account.statusNotice}
				</p>
			) : null}

			{/* ③ 步驟 1：核定統編（唯讀） */}
			<section id="amego-1" className="seller-card mt-4 p-5">
				<h2 className="text-sm font-extrabold text-foreground">
					確認商家統一編號
				</h2>
				<dl className="mt-3 grid grid-cols-1 sm:grid-cols-2 gap-3 text-xs">
					<Field label="統一編號" value={approvedTaxId || "—"} mono />
					<Field
						label="法定名稱"
						value={profile?.taxIdentity.legalName || "—"}
					/>
				</dl>
				<p className="mt-3 text-[0.625rem] leading-relaxed text-foreground/55">
					這組統編來自平台核定的稅籍資料，無法在此修改。如需更正請回到
					<Link href={TAX_STATUS_HREF} className="text-hk-accent underline">
						「稅務身分」
					</Link>
					重新送審。
				</p>
			</section>

			{/* ④ 步驟 2：AppKey */}
			<section id="amego-2" className="seller-card mt-4 p-5">
				<h2 className="text-sm font-extrabold text-foreground mb-3">
					AMEGO AppKey
				</h2>
				<MaskedSecretField
					inputId="amego-appkey"
					label="AppKey"
					description="AppKey 只會在送出時傳輸一次，儲存後平台與你都無法再查看完整內容。請自行於光貿後台保管。"
					last4={account?.credentialLast4 ?? ""}
					updatedAtText={
						account?.credentialUpdatedAt
							? formatTaipeiDateTime(account.credentialUpdatedAt)
							: ""
					}
					editing={keyEditing}
					onStartEdit={() => {
						// 不可編輯的狀態不開表單（見 canEditKey 的註解）：
						// 給一顆按下去必定 rows=0 的按鈕，回的訊息還讀起來像暫時性問題。
						if (canEditKey) setEditingKey(true);
					}}
					onCancelEdit={() => {
						setEditingKey(false);
						setAppKey("");
					}}
					value={appKey}
					onChange={setAppKey}
					disabled={
						state.terminal || actions.savingCredential || actions.rotating
					}
					errorMessage={credentialCopy?.title ?? rotateErrorTitle(actions)}
				/>
				{credentialCopy?.description ? (
					<p className="mt-2 text-[0.6875rem] leading-relaxed text-danger-700">
						{credentialCopy.description}{" "}
						<Link href={TAX_STATUS_HREF} className="text-hk-accent underline">
							前往稅務身分
						</Link>
					</p>
				) : null}
				{keyEditing ? (
					<div className="mt-4 flex justify-end">
						<Button
							variant="solid"
							size="sm"
							isLoading={actions.savingCredential || actions.rotating}
							loadingLabel="儲存中…"
							disabled={!appKey.trim() || state.terminal}
							onClick={submitCredential}
						>
							儲存 AppKey
						</Button>
					</div>
				) : null}
				<p className="mt-3 text-[0.625rem] text-foreground/45">
					條款版本 {AMEGO_CONSENT_VERSION}
				</p>
			</section>

			{/* ⑤ 步驟 3：連線測試 */}
			<section id="amego-3" className="seller-card mt-4 p-5">
				<div className="flex items-center gap-3 flex-wrap">
					<h2 className="text-sm font-extrabold text-foreground flex-1">
						連線測試
					</h2>
					<StatusPill tone={state.pillTone} label={state.pillLabel} />
				</div>
				{account?.lastVerifiedAt ? (
					<p className="mt-2 text-[0.6875rem] text-foreground/60">
						上次測試：{formatTaipeiDateTime(account.lastVerifiedAt)}
					</p>
				) : null}
				{verifyCopy ? (
					<p
						className="mt-2 text-[0.6875rem] leading-relaxed text-danger-700"
						role="alert"
					>
						{/*
						 * 🔴 **後端的訊息優先於我的固定文案。**
						 * 同一個業務碼在不同環境的意思可以差很多：dev 實測 `9332` 回的是
						 * 「本環境未開放對外開立發票，無法驗證憑證」（環境閘），而我的固定文案
						 * 寫「AMEGO 服務暫時無法連線，系統稍後會自動重試」——那在 dev 是假的，
						 * 賣家會一直等一個永遠不會來的重試。API 設計規範也明訂失敗一律透傳
						 * 微服務 Message、Gateway 不得覆寫。⇒ 後端訊息當事實，我的文案只補「下一步」。
						 */}
						{invoiceErrorMessage(actions.verifyError) || verifyCopy.title}
						{verifyCopy.description ? ` ${verifyCopy.description}` : ""}
					</p>
				) : null}
				{account?.lastRequestId ? (
					<details className="mt-3" open={detailsOpen}>
						<summary className="cursor-pointer text-[0.6875rem] font-extrabold text-hk-accent list-none select-none">
							提供客服參考的技術資訊
						</summary>
						<p className="mt-2 text-[0.6875rem] font-mono break-all text-foreground/70">
							{account.lastRequestId}
						</p>
					</details>
				) : null}
			</section>

			{/* ⑥ 步驟 4：啟用時間 */}
			<section id="amego-4" className="seller-card mt-4 p-5">
				<h2 className="text-sm font-extrabold text-foreground mb-3">
					啟用時間
				</h2>
				<Input
					id="amego-cutover"
					type="datetime-local"
					label="自何時起的訂單使用這條連線"
					min={taipeiLocalNow()}
					value={cutoverLocal}
					isInvalid={Boolean(cutoverError)}
					errorMessage={cutoverError}
					disabled={!state.canActivate}
					onChange={(e) => {
						setCutoverLocal(e.target.value);
						setCutoverError("");
					}}
				/>
				<p className="mt-2 text-[0.625rem] leading-relaxed text-foreground/55">
					{cutoverLocal
						? `自 ${cutoverLocal.replace("T", " ")} 起付款的訂單，將由此連線處理。`
						: "選定時間後，自該時刻起付款的訂單將由此連線處理。"}
					<br />
					較早付款的訂單維持原開票方式。
				</p>
				{activateCopy ? (
					<p
						className="mt-2 text-[0.6875rem] leading-relaxed text-danger-700"
						role="alert"
					>
						{activateCopy.title}
						{activateCopy.description ? ` ${activateCopy.description}` : ""}
					</p>
				) : null}
			</section>

			{/* ⑦ 步驟 5：完成卡（只有真的設好切點才出現） */}
			{state.activated && account?.automationCutoverAt ? (
				<section id="amego-5" className="seller-card mt-4 p-5">
					<h2 className="text-sm font-extrabold text-foreground">
						AMEGO 串接已完成
					</h2>
					<p className="mt-2 text-[0.6875rem] leading-relaxed text-foreground/70">
						自 {formatTaipeiDateTime(account.automationCutoverAt)}{" "}
						起付款的訂單會自動開立發票。你可以在「發票與收據紀錄」查看每一筆的狀態。
					</p>
					<div className="mt-4">
						<Button href={OVERVIEW_HREF} variant="bordered" size="sm">
							回到發票與稅籍
						</Button>
					</div>
				</section>
			) : (
				<div id="amego-5" />
			)}

			{/* ⑧ 底部動作列 */}
			<div className="mt-5 flex justify-end gap-2 flex-wrap">
				<Button
					variant="bordered"
					size="sm"
					isLoading={actions.verifying}
					loadingLabel="測試連線中…"
					disabled={!state.canVerify || state.terminal}
					onClick={() => actions.verify()}
				>
					{account?.lastVerifiedAt ? "重新測試" : "測試連線"}
				</Button>
				<Button
					variant={state.activated ? "bordered" : "solid"}
					size="sm"
					isLoading={actions.activating}
					loadingLabel="啟用中…"
					disabled={!state.canActivate || !cutoverLocal}
					onClick={() => setConfirmOpen(true)}
				>
					{state.activated ? "更新啟用時間" : "確認啟用"}
				</Button>
			</div>

			<Dialog
				open={confirmOpen}
				onClose={() => setConfirmOpen(false)}
				size="sm"
				title="確認啟用時間"
				footer={
					<div className="flex justify-end gap-2">
						<Button
							variant="light"
							size="sm"
							onClick={() => setConfirmOpen(false)}
						>
							取消
						</Button>
						<Button variant="solid" size="sm" onClick={submitActivation}>
							確認啟用
						</Button>
					</div>
				}
			>
				{/* 逐字與 ADM A09 版本歷程頁一致，不得改寫。 */}
				<p className="text-xs leading-relaxed text-foreground">
					自 <strong>{cutoverLocal.replace("T", " ")}</strong>{" "}
					起付款的訂單，將由這條 AMEGO
					連線開立發票。較早付款的訂單維持原本的開票方式，
					<strong>不會回頭改用新設定</strong>。
				</p>
			</Dialog>
		</SellerPageShell>
	);
}

function rotateErrorTitle(actions: {
	rotateError: unknown;
}): string | undefined {
	const copy = invoiceErrorCopy(actions.rotateError);
	if (!copy) return undefined;
	// 輪替失敗時最重要的一句：線上的開票沒有壞掉。
	return `${copy.title} 原有憑證仍然有效，開票不受影響。`;
}

function Field({
	label,
	value,
	mono,
}: {
	label: string;
	value: string;
	mono?: boolean;
}) {
	return (
		<div className="flex flex-col gap-[2px]">
			<dt className="text-[0.625rem] font-extrabold text-hk-accent tracking-[0.05em]">
				{label}
			</dt>
			<dd
				className={`text-xs font-extrabold text-foreground ${mono ? "font-mono" : ""}`}
			>
				{value}
			</dd>
		</div>
	);
}

function StatusPill({ tone, label }: { tone: string; label: string }) {
	return (
		// 狀態同時用文字表達，不只靠顏色（A11y 基線）。
		<span
			className={`shipping-pill shipping-pill--${tone} text-[0.6875rem] font-extrabold`}
			aria-live="polite"
		>
			{label}
		</span>
	);
}

function AmegoProgressMeter({ step }: { step: number }) {
	const percent = amegoProgressPercent(step);
	return (
		<div className="flex items-center gap-2 min-w-[160px]">
			<div
				className="seller-onboarding-progress__meter flex-1 h-2 rounded-full bg-content3 overflow-hidden"
				role="progressbar"
				aria-valuemin={0}
				aria-valuemax={100}
				aria-valuenow={percent}
				aria-label="串接進度"
			>
				<div className="h-full bg-hk-accent" style={{ width: `${percent}%` }} />
			</div>
			<span className="text-[0.625rem] font-extrabold text-foreground/70 shrink-0">
				{percent}%
			</span>
		</div>
	);
}

/**
 * `datetime-local` 的值是**本機時區**的無時區字串。賣家與平台都在台北，所以這裡
 * 直接把它當台北時間轉成 ISO 8601 UTC 送出。
 *
 * ⚠️ 不用 `new Date(local)`：那會用瀏覽器時區解讀，賣家人在國外時會差好幾小時，
 * 而切點差幾小時就是「哪些訂單用新連線開票」差一批。
 */
function taipeiLocalToIso(local: string): string {
	if (!local) return "";
	const m = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})/.exec(local);
	if (!m) return "";
	const [, y, mo, d, h, mi] = m;
	// 台北恆為 UTC+8，無日光節約。
	const utcMs = Date.UTC(+y, +mo - 1, +d, +h - 8, +mi);
	return new Date(utcMs).toISOString().replace(/\.\d{3}Z$/, "Z");
}

/** `min` 屬性用的「現在」（台北時間，`datetime-local` 格式）。 */
function taipeiLocalNow(): string {
	const now = new Date(Date.now() + 8 * 60 * 60 * 1000);
	return now.toISOString().slice(0, 16);
}
