"use client";

/**
 * N6｜稅務身分狀態卡（58A §S01 ②／§S02 ②）。
 *
 * 呈現決策全部來自 `../invoiceTaxStatusBanner.ts`；本元件只負責排版，不再自己
 * 判狀態——多一處判定就是多一份會漂移的狀態機。
 *
 * 🔴 狀態不可只靠顏色（§0.2.3）：每個 tone 都配一句 `<strong>` 標題文字，且
 * `role="status"`（`danger` 用 `role="alert"`）。
 */

import Button from "@/components/ui/Button";
import type { InvoiceTaxStatusBanner } from "../invoiceTaxStatusBanner";
import { DeadlineCountdown } from "./DeadlineCountdown";

interface Props {
	banner: InvoiceTaxStatusBanner;
	/** ISO 8601 UTC；`banner.showDeadline` 為 false 時忽略。 */
	deadlineAt?: string;
	/** 自行開票責任生效中的琥珀 pill。 */
	selfIssueEffective?: boolean;
	/** 補件備註（`admNotes[]`）；只有 `SUPPLEMENT_REQUIRED` 會有內容。 */
	notes?: readonly string[];
	className?: string;
}

// tone → 完整的 class 字串。兩件事都是刻意的：
//   1. 不用樣板拼 `border-` ＋ tone ＋ `-200`——Tailwind 掃的是字面值，拼出來的
//      類別不會被生成，畫面會變成沒有邊框的白卡。
//   2. 邊框色走 var() 形式而不是語意色 utility：`styles/globals.css` 的 @theme
//      只把每個語意色的 50 / 600 / 700 階對應成 --color-*，**200 那一階從未登記**
//      ⇒ 直接寫語意色的 200 階 utility 在 Tailwind v4 下不產生任何 CSS（既有的
//      `AfterSaleRefundBanner.tsx` 就有幾個這樣的死類別，`check-theme-tokens`
//      只掃 hk-/hkui- 前綴所以抓不到）。
//   ⚠️ 這段註解刻意不寫出完整的類別字面值：Tailwind v4 連註解都掃，寫成
//      class 形狀（尤其帶萬用字元）會真的產出一條 CSS 規則，帶 `*` 的那一條
//      還會讓整份 globals.css 解析失敗、開發伺服器起不來。
const TONE_CLASS: Record<InvoiceTaxStatusBanner["tone"], string> = {
	neutral: "border-divider bg-content2 text-[var(--hkui-foreground)]",
	info: "border-[var(--hkui-info-200)] bg-info-50 text-info-700",
	success: "border-[var(--hkui-success-200)] bg-success-50 text-success-700",
	warning: "border-[var(--hkui-warning-200)] bg-warning-50 text-warning-700",
	danger: "border-[var(--hkui-danger-200)] bg-danger-50 text-danger-700",
};

export function TaxStatusCard({
	banner,
	deadlineAt,
	selfIssueEffective,
	notes,
	className,
}: Props) {
	return (
		<section
			role={banner.tone === "danger" ? "alert" : "status"}
			className={[
				"flex flex-col gap-[var(--hkui-space-2)] rounded-hkui-lg border p-[var(--hkui-space-4)]",
				TONE_CLASS[banner.tone],
				className,
			]
				.filter(Boolean)
				.join(" ")}
		>
			<div className="flex flex-wrap items-center gap-[var(--hkui-space-2)]">
				<strong className="text-base font-bold">{banner.title}</strong>
				{selfIssueEffective ? (
					<span className="rounded-full bg-warning-50 px-2 py-[3px] text-[0.6875rem] font-black text-warning-700">
						自行開票責任生效中
					</span>
				) : null}
				{banner.showDeadline && deadlineAt ? (
					<DeadlineCountdown deadlineAt={deadlineAt} />
				) : null}
			</div>

			<p className="m-0 text-[0.8125rem] leading-relaxed text-[var(--hkui-foreground-soft)]">
				{banner.description}
			</p>

			{notes && notes.length > 0 ? (
				<ul className="m-0 flex list-disc flex-col gap-1 pl-5 text-[0.8125rem] leading-relaxed text-[var(--hkui-foreground-soft)]">
					{notes.map((note) => (
						<li key={note}>{note}</li>
					))}
				</ul>
			) : null}

			{banner.ctaLabel && banner.ctaHref ? (
				<div className="mt-1">
					<Button size="lg" variant="bordered" href={banner.ctaHref}>
						{banner.ctaLabel}
					</Button>
				</div>
			) : null}
		</section>
	);
}

export default TaxStatusCard;
