"use client";

/**
 * S07 的金額與稅額快照（58A §B-2.3 (8)）。
 *
 * 既有的商品區 `<dl>`（`orders/OrderDetailClient.tsx` 的金額明細）沒有稅別、也沒有
 * 折扣的 `createdBy`＋`fundedBy` 兩個標籤，所以另立一支；版型仍是那組 `<dl>`。
 *
 * 🔴 金額一律走 `formatInvoiceAmount`（字串進、字串出）：**不得 `Number()` 之後再
 * 算數**。發票金額是法定憑證上的數字，前端做任何加總都是在製造第二份真相——小計、
 * 稅額、總計三個值後端都給了，這裡只負責印。
 *
 * 🔴 `lines[]` 只有 `PRODUCT`／`SHIPPING`／`DISCOUNT` 三值（不含 `BUNDLE_COMPONENT`）；
 * `DISCOUNT` 行的 `netAmount` 是**負數**，用 danger 色。
 */

import {
	INVOICE_LINE_TYPE_LABEL,
	INVOICE_TAX_CATEGORY_LABEL,
	InvoiceLineType,
} from "@/lib/codemeta/enums";
import { formatInvoiceAmount } from "@/lib/invoices/display";
import type { SellerInvoiceDocumentDetail } from "@/lib/invoices/seller-types";

/**
 * 折扣的建立者與負擔者。
 *
 * 🔴 `fundedBy` **有三個值不是一個常數**：`UNDETERMINED` 是系統偵測到 ADMIN 券之後
 * fail-closed 轉人工的狀態，顯示「待平台確認」——空白或直接寫「賣家」都是在替平台
 * 對賣家宣告一個還沒決定的事實。
 */
const PARTY_LABEL: Record<string, string> = {
	SELLER: "賣家",
	PLATFORM: "平台",
	UNDETERMINED: "待平台確認",
};

function partyLabel(raw: string): string {
	return PARTY_LABEL[raw] ?? raw;
}

interface Props {
	detail: SellerInvoiceDocumentDetail;
}

/**
 * 品項行沒有任何穩定唯一鍵（同一張發票可以有兩行同名同型別，例如兩筆不同來源的
 * 折扣），而清單是唯讀且不重排 ⇒ 用「型別＋名稱＋同組出現序」當 key。不直接用陣列
 * index 是因為 lint 會擋，且真正的理由相同：index 在重排時會錯配 component state。
 */
function lineKeys(lines: SellerInvoiceDocumentDetail["lines"]): string[] {
	const seen = new Map<string, number>();
	return lines.map((line) => {
		const base = `${line.lineType}::${line.name}`;
		const nth = (seen.get(base) ?? 0) + 1;
		seen.set(base, nth);
		return `${base}::${nth}`;
	});
}

export function InvoiceSnapshotBreakdown({ detail }: Props) {
	const taxCategoryLabel = INVOICE_TAX_CATEGORY_LABEL[detail.taxCategory] ?? "";
	const keys = lineKeys(detail.lines);

	return (
		<div className="flex flex-col gap-[var(--hkui-space-3)]">
			{detail.lines.length > 0 ? (
				<ul className="m-0 flex list-none flex-col gap-2 p-0">
					{detail.lines.map((line, index) => {
						const isDiscount = line.lineType === InvoiceLineType.DISCOUNT;
						return (
							<li
								key={keys[index]}
								className="flex items-start justify-between gap-[var(--hkui-space-3)] text-[0.8125rem]"
							>
								<span className="flex min-w-0 flex-col gap-0.5">
									<span className="truncate text-[var(--hkui-foreground)]">
										{line.name ||
											INVOICE_LINE_TYPE_LABEL[line.lineType] ||
											line.lineType}
									</span>
									<span className="text-[0.75rem] text-[var(--hkui-foreground-soft)]">
										{INVOICE_LINE_TYPE_LABEL[line.lineType] ?? line.lineType}
										{line.quantity ? ` · ${line.quantity}` : ""}
									</span>
								</span>
								<span
									className={
										isDiscount
											? "shrink-0 tabular-nums text-danger-700"
											: "shrink-0 tabular-nums text-[var(--hkui-foreground)]"
									}
								>
									{formatInvoiceAmount(line.netAmount)}
								</span>
							</li>
						);
					})}
				</ul>
			) : null}

			<dl className="m-0 flex flex-col gap-2 border-t border-divider pt-[var(--hkui-space-3)] text-[0.8125rem]">
				<div className="flex items-center justify-between gap-3">
					<dt className="text-[var(--hkui-foreground-soft)]">銷售額</dt>
					<dd className="m-0 tabular-nums">
						{formatInvoiceAmount(detail.salesAmount)}
					</dd>
				</div>
				<div className="flex items-center justify-between gap-3">
					<dt className="text-[var(--hkui-foreground-soft)]">
						稅額
						{taxCategoryLabel ? (
							<span className="ml-1 text-[0.75rem]">
								（{taxCategoryLabel}）
							</span>
						) : null}
					</dt>
					<dd className="m-0 tabular-nums">
						{formatInvoiceAmount(detail.taxAmount)}
					</dd>
				</div>
				<div className="flex items-center justify-between gap-3 font-bold">
					<dt>合計</dt>
					<dd className="m-0 tabular-nums">
						{formatInvoiceAmount(detail.totalAmount)}
					</dd>
				</div>
			</dl>

			{detail.createdBy || detail.fundedBy ? (
				<p className="m-0 flex flex-wrap gap-x-3 gap-y-1 text-[0.75rem] text-[var(--hkui-foreground-soft)]">
					{detail.createdBy ? (
						<span>建立者：{partyLabel(detail.createdBy)}</span>
					) : null}
					{detail.fundedBy ? (
						<span>負擔者：{partyLabel(detail.fundedBy)}</span>
					) : null}
				</p>
			) : null}
		</div>
	);
}

export default InvoiceSnapshotBreakdown;
