"use client";

import { Button, Chip } from "@/components/ui";
import type { InvoiceMethodSelection } from "@/features/invoices/checkout/InvoiceMethodSheet";
import { InvoiceModeBadge } from "@/features/invoices/components/InvoiceModeBadge";
import type { CheckoutInvoicePreview } from "@/lib/checkout/repo";

/**
 * B01｜結帳—發票與收據卡（58A §B01）。
 *
 * **結帳頁的載具欄位是法定必要、不是可選功能**（58 §3.1 L9：〈電子發票實施作業要點〉
 * 第 21 點第 2、3 項，平臺業者不得拒絕買受人以共通性載具索取雲端發票）
 * ⇒ 不得隱藏、延後或折疊掉這一區。整卡不折疊，只有金額明細用 `<details>`。
 *
 * 🔴 **`invoice` 為 null 時整個區塊不渲染**（呼叫端負責）：後端拿不到發票服務時
 * 不回這個 key，渲染一張空卡片會讓買家以為系統壞了。
 */
/** 把 B02 的選擇翻成這一行要顯示的字。undefined ⇒ 交還給 defaultProfile。 */
function describeSelection(s?: InvoiceMethodSelection): string | undefined {
	if (!s) return undefined;
	switch (s.kind) {
		case "profile":
			return s.displayValue;
		case "none":
			return "不使用載具（寄送至電子信箱）";
		case "taxId":
			return `公司用發票 · ${s.buyerTaxId}`;
		case "donation":
			return `捐贈發票 · 愛心碼 ${s.donationCode}`;
	}
}

export function CheckoutInvoiceSection({
	invoice,
	onChangeMethod,
	selection,
	sectionIndex,
}: {
	invoice: CheckoutInvoicePreview;
	/** 開 B02（發票方式選擇 Sheet）。尚未實作時傳 undefined ⇒ 不顯示該按鈕。 */
	onChangeMethod?: () => void;
	/**
	 * 買家這一單在 B02 選的方式。**undefined ＝ 他沒動過**，此時顯示後端算出的
	 * `defaultProfile`（預設常用資料）——兩者是不同的東西，不可互相冒充：
	 * 沒動過時 confirm 會整包省略 invoice 走 USE_BUYER_DEFAULT，
	 * 動過時才會帶。畫面必須跟 payload 講同一件事。
	 */
	selection?: InvoiceMethodSelection;
	sectionIndex: number;
}) {
	const { invoiceMode, amount, disclosure, defaultProfile, intakeLimit } =
		invoice;
	const chosen = describeSelection(selection);

	return (
		<section className="bg-content1 border border-divider rounded-[32px] p-[18px] shadow-hk-card">
			<div className="flex items-center gap-2 mb-[14px]">
				<span className="w-6 h-6 rounded-full bg-hk-accent text-white text-[0.6875rem] font-extrabold grid place-items-center shrink-0">
					{sectionIndex}
				</span>
				<h2 className="text-sm font-extrabold text-foreground">發票與收據</h2>
				<InvoiceModeBadge mode={invoiceMode} />
			</div>

			{/*
			 * 法定告知。四個模式都要有分支——缺一就會讓某類賣家的買家完全不知道
			 * 自己拿到的不是電子發票（58A §B01 逐字：缺一即落 invalid 而擋付款）。
			 *
			 * ⚠️ 這裡**沒有 checkbox**：DEC-20260831-210934 把告知改成只顯示不強制勾選
			 * （58 號 L1–L13 法規表查無「付款前必須告知」的法源；蝦皮／momo 實查也都
			 * 沒有結帳時的強制確認）。9314 已無 emitter。
			 */}
			{disclosure && disclosure.required ? (
				<div
					className="mb-3 p-[12px_14px] rounded-hkui-lg border border-warning-200 bg-warning-50"
					role="note"
				>
					<p className="text-xs font-extrabold text-warning-700">
						{disclosure.title}
					</p>
					<p className="mt-1 text-[0.6875rem] leading-relaxed text-warning-700/90">
						{disclosure.body}
					</p>
				</div>
			) : null}

			{/* 發票方式（目前顯示買家的預設常用資料；沒有就顯示缺省行為） */}
			<div className="flex items-center gap-3 p-[12px_14px] bg-content2 border border-divider rounded-hkui-lg">
				<div className="flex-1 min-w-0 flex flex-col gap-[2px]">
					<span className="text-[0.625rem] font-extrabold text-hk-accent tracking-[0.05em]">
						發票方式
					</span>
					<strong className="text-xs font-extrabold text-foreground block truncate">
						{chosen ??
							(defaultProfile
								? defaultProfile.displayValue
								: "由系統依你的預設資料開立")}
					</strong>
					{!selection && defaultProfile?.label ? (
						<span className="text-[0.625rem] text-foreground/60 truncate">
							{defaultProfile.label}
						</span>
					) : null}
				</div>
				{!selection && defaultProfile?.isDefault ? (
					<Chip variant="flat" color="default" size="sm">
						預設
					</Chip>
				) : null}
				{onChangeMethod ? (
					<Button variant="light" size="sm" onClick={onChangeMethod}>
						變更發票方式
					</Button>
				) : null}
			</div>

			{/*
			 * 金額明細（N4）。四列逐字對齊後端的 amount，**不拿右欄的實付總額充當發票金額**
			 * ——兩者在 Phase 1 相等是業務決策不是恆等式（58A §B01）。
			 */}
			{amount ? (
				<details className="mt-3 group">
					<summary className="cursor-pointer text-[0.6875rem] font-extrabold text-hk-accent list-none select-none">
						查看金額明細
					</summary>
					<dl className="mt-2 flex flex-col gap-1 text-[0.6875rem]">
						<AmountRow label="商品金額" value={amount.productAmount} />
						<AmountRow label="應稅運費" value={amount.shippingAmount} />
						<AmountRow
							label="優惠折扣"
							value={amount.discountAmount}
							negative
						/>
						<div className="border-t border-divider my-1" />
						<AmountRow label="發票金額" value={amount.invoiceAmount} emphasis />
					</dl>
				</details>
			) : null}

			<p className="mt-3 text-[0.625rem] leading-relaxed text-foreground/55">
				發票將於付款完成後開立，並寄送至你的電子信箱。
			</p>

			{/*
			 * 不可下單 banner。9326（接單上限）與 9327（和單）是**兩件獨立的事**，
			 * 不合併成一句——買家看到的處置不同（前者等賣家、後者拆單重下）。
			 * 目前只有 9326 有資料來源；9327 由 confirm 回應驅動，不在本區塊。
			 */}
			{intakeLimit?.blocked ? (
				<div
					className="mt-3 p-[12px_14px] rounded-hkui-lg border border-danger-200 bg-danger-50"
					role="alert"
				>
					<p className="text-xs font-extrabold text-danger-700">
						此賣家暫時無法接單
					</p>
					<p className="mt-1 text-[0.6875rem] leading-relaxed text-danger-700/90">
						賣家的稅務資格正在審核中，且已達審核期間的接單上限。請稍後再試或聯繫賣家。
					</p>
				</div>
			) : null}
		</section>
	);
}

/**
 * 金額列。
 *
 * `negative` 只影響**顯示**（前綴 −），不動值本身：後端給的 discountAmount 是正數，
 * 在前端翻成負號是呈現決定；把它變成負數存下去會讓任何後續相加靜默錯掉。
 */
function AmountRow({
	label,
	value,
	negative,
	emphasis,
}: {
	label: string;
	value: string;
	negative?: boolean;
	emphasis?: boolean;
}) {
	const shown = formatNtd(value);
	return (
		<div className="flex justify-between gap-3">
			<dt
				className={`min-w-0 ${emphasis ? "font-extrabold text-foreground" : "text-foreground/70"}`}
			>
				{label}
			</dt>
			<dd
				className={
					emphasis ? "font-extrabold text-foreground" : "text-foreground/80"
				}
			>
				{negative && shown !== "—" ? `−${shown}` : shown}
			</dd>
		</div>
	);
}

/**
 * 後端的金額是**元的整數字串**（API 設計規範：金額走字串避免 JS 精度問題）。
 * 解不出數字時回破折號而不是 `NaN` 或 `0`——後者會讓買家以為金額真的是 0。
 */
function formatNtd(raw: string): string {
	const n = Number(raw);
	if (!raw || !Number.isFinite(n)) return "—";
	return `NT$ ${n.toLocaleString("zh-TW")}`;
}
