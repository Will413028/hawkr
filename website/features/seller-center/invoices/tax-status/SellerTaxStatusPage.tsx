"use client";

/**
 * S02｜稅務身分申請與進度（58A §S02）。端點 17 是本頁唯一的資料源。
 *
 * 權限：`CapInvoiceTaxRemedy`，**在停權白名單內**。`useSellerTaxProfile()` 因此
 * **不得**加 `!isSuspended`（§B-2.0 T04）——停權賣家補齊稅務件正是解除停權的路徑，
 * 在前端把後端的放行抵銷掉，兩支查詢都回 200、沒有任何測試會叫。
 *
 * 🔴 **端點 18／19 皆已接上（2026-09-01）。**
 *
 * 本頁現況：
 *   ✅ 端點 17 唯讀進度（狀態卡 ＋ 補件備註 ＋ 已提交資料與文件）
 *   ✅ 端點 19 送出審核 ＋ 自行開票責任條款勾選（裁決 C：兩者是同一個動作）
 *   ✅ 端點 18 草稿存檔（身分表單 ＋ 稅籍證明 secure 上傳，2026-09-01 接上）
 *   ✅ 端點 20 申請延長（2026-09-01 接上）
 *
 * 🔴 端點 18 是整條發票鏈的第一步：T02 的守衛讀的是已落庫的 `evidence_object_keys`，
 * 而只有它會寫。接上之前，賣家從零走不到可送審的 DRAFT ⇒ 永遠不會被核定 ⇒ 永遠
 * 串不了 AMEGO ⇒ 訂單永遠開不出發票。
 *
 * ⚠️ 「本 repo 尚無 secure 上傳 helper」是**過期的前提**：`lib/media/secure-upload.ts`
 * 的 `uploadSecureDocument` 早就在，且已由 `CertificationsClient` 實地使用。
 */

import * as React from "react";
import { BackLink } from "@/components/shared/BackLink";
import Link from "@/components/shared/Link";
import { SellerPageShell } from "@/components/shared/SellerPageShell";
import Button from "@/components/ui/Button";
import {
	SELLER_TAX_ENTITY_TYPE_LABEL,
	SELLER_TAX_OBLIGATION_LABEL,
	SellerTaxReviewStatus,
} from "@/lib/codemeta/enums";
import { formatTaipeiDate, formatTaipeiDateTime } from "@/lib/date/taipei";
import {
	useSellerTaxProfile,
	useSubmitSellerTaxProfile,
} from "@/lib/hooks/useSellerInvoiceRemedy";
import { formatFileSize } from "@/lib/invoices/display";
import { sellerTaxDocumentHref } from "@/lib/invoices/seller-repo";
import {
	hasIntakeLimit,
	isSelfIssueAccepted,
	type SellerTaxDocument,
	type SellerTaxProfile,
} from "@/lib/invoices/seller-types";
import { SellerCardGridSkeleton } from "../../components/SellerSkeletons";
import { TaxStatusCard } from "../components/TaxStatusCard";
import { invoiceTaxStatusBanner } from "../invoiceTaxStatusBanner";
import { taxSubmitGate } from "../taxSubmitGate";
import { TaxExtensionSection } from "./TaxExtensionSection";
import { TaxIdentityForm } from "./TaxIdentityForm";
import { TaxSubmitSection } from "./TaxSubmitSection";

const OVERVIEW_HREF = "/seller-center/invoices";

export function SellerTaxStatusPage() {
	const { profile, notStarted, loading, error, refresh } =
		useSellerTaxProfile();

	// 兩條路徑會走到「尚未開始」：後端 404 ＋ 9320（還沒建過任何列），以及端點 17
	// 直接回 `reviewStatus = "NOT_STARTED"`（合成值）。兩者都不是錯誤態。
	const reviewStatus = notStarted
		? SellerTaxReviewStatus.NOT_STARTED
		: (profile?.reviewStatus ?? "");

	const cooldownUntilText = profile?.reapplyCooldownUntil
		? formatTaipeiDate(profile.reapplyCooldownUntil)
		: "";

	const banner = React.useMemo(
		() =>
			invoiceTaxStatusBanner({
				reviewStatus,
				selfIssueEffective: profile?.selfIssueAttestation.effective,
				cooldownUntilText,
			}),
		[reviewStatus, profile?.selfIssueAttestation.effective, cooldownUntilText],
	);

	const notes = React.useMemo(
		() => (profile?.admNotes ?? []).map((note) => note.content),
		[profile?.admNotes],
	);

	// 冷卻是否仍在進行中。在這裡算而不是在 `taxSubmitGate` 內：那一支是純函式，
	// 讓它自己讀 `Date.now()` 會讓判準隨測試的執行時刻飄。
	const cooldownActive = React.useMemo(() => {
		const until = profile?.reapplyCooldownUntil;
		if (!until) return false;
		const t = Date.parse(until);
		return Number.isFinite(t) && t > Date.now();
	}, [profile?.reapplyCooldownUntil]);

	const gate = taxSubmitGate({
		profile,
		notStarted,
		editable: banner.editable,
		cooldownActive,
	});

	// `expectedVersion` 只能來自端點 17 的 `version`（樂觀鎖）；不符後端回 9322。
	const {
		submit,
		submitting,
		error: submitError,
	} = useSubmitSellerTaxProfile(profile?.version ?? "");

	return (
		<SellerPageShell
			title="稅務身分"
			eyebrow="發票與稅籍"
			subtitle="這是你的發票／稅務身分，與「商家認證」是兩件不同的事。"
			actions={
				<Button size="lg" variant="bordered" onClick={() => void refresh()}>
					重新整理
				</Button>
			}
		>
			<BackLink fallbackHref={OVERVIEW_HREF}>返回發票與稅籍</BackLink>

			{loading ? (
				<SellerCardGridSkeleton
					variant="panel"
					count={2}
					className="flex flex-col gap-[var(--hkui-space-4)]"
				/>
			) : error ? (
				<section
					role="alert"
					className="seller-card flex flex-col items-center gap-[var(--hkui-space-3)] p-6 text-center"
				>
					<p className="m-0 text-sm text-[var(--hkui-foreground-soft)]">
						稅務身分資料載入失敗，請稍後再試。
					</p>
					<Button size="lg" variant="bordered" onClick={() => void refresh()}>
						重試
					</Button>
				</section>
			) : (
				<div className="flex flex-col gap-[var(--hkui-space-4)]">
					<TaxStatusCard
						banner={banner}
						deadlineAt={profile?.deadlineAt}
						selfIssueEffective={profile?.selfIssueAttestation.effective}
						notes={notes}
					/>

					{profile && hasIntakeLimit(profile.intakeLimit) ? (
						<section
							role={profile.intakeLimit.blocked ? "alert" : "status"}
							className="flex flex-col gap-1 rounded-hkui-lg border border-[var(--hkui-warning-200)] bg-warning-50 p-[var(--hkui-space-4)] text-warning-700"
						>
							<strong className="text-sm font-bold">
								{`稅務審核完成前，接單有上限：已接 ${profile.intakeLimit.orderCountUsed}／${profile.intakeLimit.orderCountLimit} 筆、${profile.intakeLimit.amountUsed}／${profile.intakeLimit.amountLimit} 元，先到者停止接受新訂單。`}
							</strong>
							{profile.intakeLimit.blocked ? (
								<p className="m-0 text-[0.8125rem]">
									已達上限，目前無法接受新訂單。完成稅務身分審核後自動恢復。
								</p>
							) : null}
						</section>
					) : null}

					{/*
					 * 送審（端點 19）＋ 自行開票責任條款勾選。
					 *
					 * 🔴 `gate.visible === false` ＝ 這個狀態根本沒有「送審」這件事
					 * （審核中／已核定／已撤銷）⇒ 整塊不渲染。渲染一顆必定回 9321 的鍵
					 * 比缺一顆鍵更糟。缺草稿／缺佐證則**要**渲染，因為那時賣家需要
					 * 知道下一步是什麼。
					 */}
					{/*
					 * 端點 18 的表單。**gate.visible 為真時就渲染**——那正是「這個狀態
					 * 還能編輯」的意思，而賣家要能填、要能補件。
					 *
					 * 🔴 這張表單是整條發票鏈的第一步：端點 19 的守衛讀的是已落庫的
					 * `evidence_object_keys`，而只有端點 18 會寫它。
					 */}
					{gate.visible ? <TaxIdentityForm profile={profile} /> : null}

					{gate.visible ? (
						<TaxSubmitSection
							blocker={gate.blocker}
							profile={profile}
							submitting={submitting}
							error={submitError}
							onSubmit={submit}
						/>
					) : null}

					{/*
					 * S04 的入口。
					 *
					 * 🔴 只在 `SUBMITTED` 渲染，而那正是送審表單**收起來**的時候
					 * （`gate.visible === false`）⇒ 沒有這一段，補簽條款、把自己推進
					 * `SELF_ISSUE_PENDING` 這條路在整個站上沒有任何可達路徑
					 * （送審表單裡的「閱讀完整條款」在這個狀態下不渲染）。
					 *
					 * ⚠️ 判準是 `reviewStatus === SUBMITTED`（T04 的 allowed-from），
					 * 不是 `selfIssueAttestation.effective === false`——後者在 `DRAFT`
					 * 也成立，會對還沒送審的賣家給一顆必定回 9321 的鍵。
					 */}
					{reviewStatus === SellerTaxReviewStatus.SUBMITTED ? (
						<section className="seller-card flex flex-col gap-2 p-5">
							<strong className="text-sm font-extrabold text-foreground">
								審核期間的開票責任
							</strong>
							<p className="m-0 text-[0.8125rem] leading-relaxed text-[var(--hkui-foreground-soft)]">
								{
									"審核完成前，發票由你自行開立與交付。閱讀完整責任條款並具結後，平台會把你的狀態推進到「已具結，等待平台審核」。"
								}
							</p>
							<div className="mt-1">
								<Button
									size="lg"
									variant="bordered"
									href="/seller-center/invoices/self-issue-terms"
								>
									自行開票責任條款
								</Button>
							</div>
						</section>
					) : null}

					{/* 端點 20。元件自己看 canRequestExtension，不可申請時回 null。 */}
					{profile ? <TaxExtensionSection profile={profile} /> : null}

					{profile ? <SubmittedSection profile={profile} /> : null}
				</div>
			)}
		</SellerPageShell>
	);
}

function SubmittedSection({ profile }: { profile: SellerTaxProfile }) {
	const identity = profile.taxIdentity;
	const attestation = profile.selfIssueAttestation;
	const hasIdentity =
		identity.taxId !== "" ||
		identity.legalName !== "" ||
		identity.entityType !== "";

	if (!hasIdentity && profile.documents.length === 0) return null;

	return (
		<>
			{hasIdentity ? (
				<section className="seller-card">
					<header className="seller-card__head">
						<h2>已提交的稅籍資料</h2>
					</header>
					<dl className="seller-order-detail__info">
						<Row
							label="申請類型"
							value={
								SELLER_TAX_ENTITY_TYPE_LABEL[identity.entityType] ??
								identity.entityType
							}
						/>
						{/*
						 * 🔴 `claimedStatus` 是**賣家自述**，與 `reviewStatus`（平台核定）
						 * 分開顯示。UI 不得把自述免用寫成「已核定免用統一發票」——那是
						 * ADM 的裁決（§S02 ⑤）。
						 */}
						<Row
							label="自述稅務身分"
							value={
								SELLER_TAX_OBLIGATION_LABEL[identity.claimedStatus] ??
								identity.claimedStatus
							}
						/>
						<Row label="統一編號" value={identity.taxId} />
						<Row label="法定名稱" value={identity.legalName} />
						<Row label="負責人" value={identity.responsibleName} />
						<Row
							label="地址"
							value={[
								identity.addressCity,
								identity.addressDistrict,
								identity.addressDetail,
							]
								.filter(Boolean)
								.join("")}
						/>
						<Row
							label="送出時間"
							value={formatTaipeiDateTime(profile.submittedAt)}
						/>
						<Row
							label="自行開票責任條款"
							value={
								isSelfIssueAccepted(attestation)
									? `已於 ${formatTaipeiDateTime(attestation.acceptedAt)} 同意（${attestation.termsVersion}）`
									: "尚未同意"
							}
						/>
					</dl>
				</section>
			) : null}

			{profile.documents.length > 0 ? (
				<section className="seller-card">
					<header className="seller-card__head">
						<h2>證明文件</h2>
					</header>
					<ul className="m-0 flex list-none flex-col gap-2 p-0">
						{profile.documents.map((doc) => (
							<DocumentRow key={doc.documentId} document={doc} />
						))}
					</ul>
				</section>
			) : null}
		</>
	);
}

function DocumentRow({ document }: { document: SellerTaxDocument }) {
	// 已上傳但後端還沒換發 objectKey 時 `fileAssetId` 為空 ⇒ **不渲染下載連結**，
	// 點下去必定 404（§S02 的「已上傳未存草稿」態）。
	const href = sellerTaxDocumentHref(document);
	const size = formatFileSize(document.fileSize);

	return (
		<li className="flex items-center justify-between gap-3 text-[0.8125rem]">
			<span className="flex min-w-0 flex-col gap-0.5">
				{/* 每列都有可見檔名文字，不只 icon（§S02 A11y）。 */}
				<span className="truncate text-[var(--hkui-foreground)]">
					{document.fileName || document.documentId}
				</span>
				<span className="text-[0.75rem] text-[var(--hkui-foreground-soft)]">
					{[size, formatTaipeiDateTime(document.uploadedAt)]
						.filter(Boolean)
						.join(" · ")}
				</span>
			</span>
			{href ? (
				<Link href={href} className="shrink-0 text-sm underline">
					下載
				</Link>
			) : (
				<span className="shrink-0 text-[0.75rem] text-[var(--hkui-foreground-soft)]">
					尚未可下載
				</span>
			)}
		</li>
	);
}

function Row({ label, value }: { label: string; value: string }) {
	if (!value) return null;
	return (
		<div>
			<dt>{label}</dt>
			<dd>{value}</dd>
		</div>
	);
}

export default SellerTaxStatusPage;
