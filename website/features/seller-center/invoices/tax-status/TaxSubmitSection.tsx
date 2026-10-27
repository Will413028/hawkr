"use client";

/**
 * S02 送審區塊（58A §S02「送出審核」；裁決 C，2026-08-31）。
 *
 * 🔴 **條款就在送審表單裡，不是另一個頁面的前置。**
 *
 * 原設計是「先去 S04 同意條款（端點 22）→ 再回來送審（端點 19）」，但端點 22 的
 * 來源狀態是 `SUBMITTED`，而 `SUBMITTED` 只有送審產得出來 ⇒ 新賣家（`DRAFT`）
 * 永遠走不進那個環。裁決 C 把兩者併成一個動作：勾選 ＋ 送出一次完成，後端在
 * 同一句 UPDATE 裡寫下狀態、60 天時鐘與條款簽署記錄。
 *
 * ⚠️ 因此本區塊**不連到 S04 當前置**。S04 保留給補簽（已送審但當初沒簽的舊資料），
 * 這裡只放一個「閱讀完整條款」的旁連結。
 *
 * 🔴 送不出去時**說原因**，不是給一顆 disabled 的鍵：賣家看不到原因會反覆點，
 * 而每一種 blocker 的下一步完全不同（補欄位／傳檔案／等冷卻／先建草稿）。
 */

import * as React from "react";
import Link from "@/components/shared/Link";
import Button from "@/components/ui/Button";
import { formatTaipeiDate } from "@/lib/date/taipei";
import { invoiceErrorCopy } from "@/lib/invoices/errorCopy";
import type { SellerTaxProfile } from "@/lib/invoices/seller-types";
import type { TaxSubmitBlocker } from "../taxSubmitGate";

/**
 * 條款全文的連結目標 —— S04 已於 2026-09-01 落地，依原註解的承諾改指它。
 *
 * S04 頁面列出五條要點（SSOT 是 `docs/legal/self-issue-attestation-terms.md`，
 * 由 `selfIssueTerms.unit.test.ts` 對拍），並提供「閱讀條款全文」連到服務條款
 * §二那份既有權威正文。
 *
 * ⚠️ 這裡不新開分頁：S04 是同一個賣家中心裡的頁面，開新分頁會讓賣家在兩個 tab
 * 之間對照送審狀態。
 */
const TERMS_HREF = "/seller-center/invoices/self-issue-terms";

interface Props {
	blocker: TaxSubmitBlocker | null;
	profile: SellerTaxProfile | null;
	/** 送出中；期間鎖住勾選框與按鈕。 */
	submitting: boolean;
	/** 上一次送出的失敗（含 `9325`／`9324`／`9321`／400）。 */
	error: unknown;
	onSubmit: (acceptedTerms: boolean) => void;
}

/** blocker → 賣家看得懂的下一步。文案的主詞是**動作**不是狀態。 */
function blockerCopy(
	blocker: Exclude<TaxSubmitBlocker, "not-editable">,
	cooldownUntilText: string,
): { title: string; description: string } {
	switch (blocker) {
		// ⚠️ 這兩句原本寫「線上填寫／上傳功能還在準備中」——**2026-09-01 端點 18 接上後
		// 那句話就變成假的**，而表單就渲染在這一段的正上方，兩者同時出現會讓賣家
		// 以為自己看到的表單不能用。改了前提就要回灌到引用它的每一處。
		case "not-started":
			return {
				title: "還沒有稅務身分資料",
				description:
					"請在上方填寫稅籍資料並上傳稅籍證明文件，存檔後就能送出審核。",
			};
		case "identity-incomplete":
			return {
				title: "稅籍資料還沒填完",
				description:
					"申請類型、法定名稱、負責人與地址都要填；自述「應開立統一發票」時還需要統一編號。",
			};
		case "evidence-missing":
			return {
				title: "還沒有稅籍證明文件",
				description:
					"請在上方的「稅籍證明文件」至少上傳一份，存檔後就能送出審核。",
			};
		case "cooldown":
			return {
				title: "還在重新申請的冷卻期間",
				description: cooldownUntilText
					? `可於 ${cooldownUntilText} 後重新送出審核。`
					: "冷卻結束後才能再次送出審核。",
			};
	}
}

export function TaxSubmitSection({
	blocker,
	profile,
	submitting,
	error,
	onSubmit,
}: Props) {
	const [accepted, setAccepted] = React.useState(false);

	const cooldownUntilText = profile?.reapplyCooldownUntil
		? formatTaipeiDate(profile.reapplyCooldownUntil)
		: "";

	// `not-editable` 由呼叫端的 `gate.visible` 擋掉、根本不會渲染本元件；在這裡
	// 也擋一次是為了讓型別窄化成 `blockerCopy` 的四值——那個 switch 因此可以沒有
	// default，值域增生時 TypeScript 當場報錯而不是靜默漏一種文案。
	if (blocker === "not-editable") return null;
	if (blocker !== null) {
		const copy = blockerCopy(blocker, cooldownUntilText);
		return (
			<section role="status" className="seller-card flex flex-col gap-2 p-6">
				<strong className="text-base font-bold">{copy.title}</strong>
				<p className="m-0 text-sm text-[var(--hkui-foreground-soft)]">
					{copy.description}
				</p>
			</section>
		);
	}

	// 條款版本一律顯示**當期**版本（`currentTermsVersion`），不是 `termsVersion`
	// ——後者是「已簽的那一版」，未簽時是空字串。
	const termsVersion = profile?.selfIssueAttestation.currentTermsVersion ?? "";
	const copy = invoiceErrorCopy(error);

	return (
		<section className="seller-card">
			<header className="seller-card__head">
				<h2>送出審核</h2>
			</header>
			<div className="flex flex-col gap-[var(--hkui-space-3)] p-6 pt-0">
				<p className="m-0 text-sm text-[var(--hkui-foreground-soft)]">
					送出後平台會開始審核，期間資料無法修改。審核完成前，這段期間的發票由你自行開立與交付。
				</p>

				{/* 五條要點是前端靜態文案（端點 21 只給版本與檔案 key）；
				    條款全文在 S04，這裡只放摘要與連結。 */}
				<ul className="m-0 flex list-disc flex-col gap-1 pl-5 text-[0.8125rem] text-[var(--hkui-foreground-soft)]">
					<li>
						審核完成前，你是這些訂單的發票銷售人，發票由你自行開立並交付買家。
					</li>
					<li>發票抬頭與統一編號使用你的稅籍資料，Hawkr 不會另行開立。</li>
					<li>審核期間平台會限制待審接單量，完成審核後自動恢復。</li>
					<li>逾期未完成審核會暫停接受新訂單，補齊後可申請重新審核。</li>
				</ul>

				<label className="seller-cert__checkbox">
					<input
						type="checkbox"
						checked={accepted}
						disabled={submitting}
						onChange={(e) => setAccepted(e.target.checked)}
					/>
					<div>
						<strong>我已閱讀並同意自行開票責任條款</strong>
						<small>
							{termsVersion ? `條款版本 ${termsVersion}。` : ""}
							<Link href={TERMS_HREF} className="underline">
								閱讀完整條款
							</Link>
						</small>
					</div>
				</label>

				{copy ? (
					<p role="alert" className="m-0 text-sm text-danger-700">
						{copy.title}
						{copy.description ? ` ${copy.description}` : ""}
						{/* 9324 的倒數時刻只在 data 裡，文案表拿不到 ⇒ 這裡補上。 */}
						{cooldownUntilText ? ` 可於 ${cooldownUntilText} 後再試。` : ""}
					</p>
				) : null}

				<div>
					<Button
						size="lg"
						// 🔴 未勾選就不能送。這是 UX 不是防線——後端 accepted_terms=false
						// 一律 9325 並且一筆都不寫。
						disabled={!accepted || submitting}
						isLoading={submitting}
						loadingLabel="送出中"
						onClick={() => onSubmit(accepted)}
					>
						送出審核
					</Button>
				</div>
			</div>
		</section>
	);
}

export default TaxSubmitSection;
