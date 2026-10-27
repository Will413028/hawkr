"use client";

import * as React from "react";
import { Chip, Sheet } from "@/components/ui";
import Button from "@/components/ui/Button";
import { MaskedValue } from "@/features/invoices/components/MaskedValue";
import { useInvoiceProfiles } from "@/lib/hooks/useInvoiceProfiles";
import { useLocalizedHref } from "@/lib/intl/use-router";
import { invoiceCarrierLabel } from "@/lib/invoices/carrier";

/**
 * B02｜發票方式選擇 Sheet（58A §B02）。
 *
 * 由 B01 的「變更發票方式」開啟；B06（付款後修改發票資料）之後也會復用。
 *
 * **兩個區塊的送出方式不同，這才是它們並存的理由**：
 *   A 常用發票資料 → 送 `{ profileId }`
 *   B 使用其他方式 → 送 `{ buyerTaxId, buyerName }` 或 `{ donationCode }`，
 *     而「不使用載具」**整包省略 invoice**
 *
 * 🔴 **手機條碼與自然人憑證不在這個 Sheet 裡手動輸入。**
 * 58 §5821 第 4 項推導過：`order_invoice_snapshots` 是零 PII 表，
 * `chk_ois_mode` 的四個值沒有一個代表「這一單自己填的載具」，
 * 落 `USE_BUYER_DEFAULT` 會把買家剛輸入的值靜默吞掉、落 `EXPLICIT_PREFERENCE`
 * ＋空 id 則因 `chk_ois_pref_pair` 是雙向等式而直接 500 ⇒ 只剩 fail-closed。
 * ⇒ 那兩種載具走「先到帳號設定存成常用資料 → 這裡選它」。本 Sheet 因此提供
 * 「管理常用發票資料」的入口，而不是一個送不出去的輸入框。
 */
export type InvoiceMethodSelection =
	| { kind: "profile"; profileId: string; displayValue: string }
	| { kind: "none" }
	| { kind: "taxId"; buyerTaxId: string; buyerName: string }
	| { kind: "donation"; donationCode: string };

export function InvoiceMethodSheet({
	open,
	onClose,
	onSelect,
	current,
}: {
	open: boolean;
	onClose: () => void;
	onSelect: (s: InvoiceMethodSelection) => void;
	/** 目前已選的方式，用來預選 radio。 */
	current?: InvoiceMethodSelection;
}) {
	const { profiles, loading, error } = useInvoiceProfiles();
	// Plain anchors on purpose: leaving the checkout sheet for the account page is a full page
	// load (tests/native-navigation-contract.test.ts).
	const invoiceProfilesHref = useLocalizedHref("/account/invoice-profiles");
	const [choice, setChoice] = React.useState<string>(() =>
		initialChoice(current),
	);
	const [taxId, setTaxId] = React.useState(
		current?.kind === "taxId" ? current.buyerTaxId : "",
	);
	const [title, setTitle] = React.useState(
		current?.kind === "taxId" ? current.buyerName : "",
	);
	const [donation, setDonation] = React.useState(
		current?.kind === "donation" ? current.donationCode : "",
	);
	const [fieldError, setFieldError] = React.useState<string | null>(null);

	// Sheet 每次開啟都回到「目前的選擇」，而不是上一次關閉時的暫存。
	// 不重置的話，買家按取消之後再打開會看到自己剛剛放棄的輸入。
	React.useEffect(() => {
		if (!open) return;
		setChoice(initialChoice(current));
		setTaxId(current?.kind === "taxId" ? current.buyerTaxId : "");
		setTitle(current?.kind === "taxId" ? current.buyerName : "");
		setDonation(current?.kind === "donation" ? current.donationCode : "");
		setFieldError(null);
	}, [open, current]);

	const submit = () => {
		const sel = buildSelection(choice, { taxId, title, donation });
		if (typeof sel === "string") {
			setFieldError(sel);
			return;
		}
		setFieldError(null);
		onSelect(sel);
		onClose();
	};

	return (
		<Sheet
			open={open}
			onClose={onClose}
			side="bottom"
			title="選擇發票方式"
			className="max-w-xl md:self-center"
			footer={
				<>
					<Button variant="bordered" onClick={onClose}>
						取消
					</Button>
					<Button color="primary" onClick={submit}>
						使用這個方式
					</Button>
				</>
			}
		>
			{fieldError ? (
				<p
					className="mb-3 p-[10px_12px] rounded-hkui-lg border border-danger-200 bg-danger-50 text-[0.6875rem] font-extrabold text-danger-700"
					role="alert"
				>
					{fieldError}
				</p>
			) : null}

			{/* 區塊 A：常用發票資料。**只在有 ≥1 筆時渲染**（58A §B02） */}
			{profiles.length > 0 ? (
				<section className="mb-4">
					<h3 className="text-[0.6875rem] font-extrabold text-foreground/70 mb-2">
						常用發票資料
					</h3>
					<div
						className="flex flex-col gap-2"
						role="radiogroup"
						aria-label="常用發票資料"
					>
						{profiles.map((p) => (
							<div key={p.profileId}>
								<label className="flex items-center gap-3 p-[12px_14px] bg-content2 border border-divider rounded-hkui-lg cursor-pointer">
									<input
										type="radio"
										name="invoice-method"
										className="shrink-0"
										checked={choice === `profile:${p.profileId}`}
										onChange={() => setChoice(`profile:${p.profileId}`)}
									/>
									<span className="flex-1 min-w-0 flex flex-col gap-[2px]">
										<span className="text-xs font-extrabold text-foreground truncate">
											<MaskedValue
												value={p.displayValue}
												label={p.displayValue}
											/>
										</span>
										<span className="text-[0.625rem] text-foreground/60 truncate">
											{invoiceCarrierLabel(p.carrierType)}
											{p.label ? ` · ${p.label}` : ""}
										</span>
									</span>
									{p.isDefault ? (
										<Chip variant="flat" color="default" size="sm">
											預設
										</Chip>
									) : null}
								</label>
							</div>
						))}
					</div>
					<a
						href={invoiceProfilesHref}
						className="mt-2 inline-block text-[0.6875rem] font-extrabold text-hk-accent"
					>
						管理常用發票資料 ›
					</a>
				</section>
			) : null}

			{loading ? (
				<p className="mb-3 text-[0.6875rem] text-foreground/60">
					載入常用發票資料…
				</p>
			) : null}
			{error ? (
				<p className="mb-3 text-[0.6875rem] text-warning-700" role="status">
					常用發票資料載入失敗，你仍可在下方直接指定這一次的開立方式。
				</p>
			) : null}

			{/* 區塊 B：使用其他方式 */}
			<section>
				<h3 className="text-[0.6875rem] font-extrabold text-foreground/70 mb-2">
					使用其他方式
				</h3>
				<div
					className="flex flex-col gap-2"
					role="radiogroup"
					aria-label="發票方式"
				>
					<OtherOption
						id="none"
						label="不使用載具"
						hint="發票會以電子郵件寄給你，不存入任何載具"
						checked={choice === "none"}
						onSelect={() => setChoice("none")}
					/>
					<OtherOption
						id="taxId"
						label="公司用發票（統一編號）"
						checked={choice === "taxId"}
						onSelect={() => setChoice("taxId")}
					>
						<div className="mt-2 pl-[30px] flex flex-col gap-3">
							<Field
								label="統一編號"
								value={taxId}
								onChange={setTaxId}
								maxLength={8}
								inputMode="numeric"
								placeholder="8 碼數字"
							/>
							<Field
								label="公司抬頭"
								value={title}
								onChange={setTitle}
								maxLength={60}
								placeholder="發票抬頭"
							/>
						</div>
					</OtherOption>
					<OtherOption
						id="donation"
						label="捐贈發票"
						checked={choice === "donation"}
						onSelect={() => setChoice("donation")}
					>
						<div className="mt-2 pl-[30px]">
							<Field
								label="捐贈碼"
								value={donation}
								onChange={setDonation}
								maxLength={7}
								inputMode="numeric"
								placeholder="3–7 碼數字"
							/>
						</div>
					</OtherOption>
				</div>

				{/*
				 * 手機條碼／自然人憑證的說明。**不是一個被停用的選項**——放一個點不下去的
				 * radio 會讓買家一直嘗試。直接說明去哪裡設定，那是唯一可行的路徑。
				 */}
				<p className="mt-3 text-[0.625rem] leading-relaxed text-foreground/55">
					要使用手機條碼或自然人憑證，請先到
					<a
						href={invoiceProfilesHref}
						className="text-hk-accent font-extrabold"
					>
						{" "}
						發票資料{" "}
					</a>
					儲存為常用發票資料，之後即可在上方選用。
				</p>
			</section>
		</Sheet>
	);
}

function initialChoice(current?: InvoiceMethodSelection): string {
	if (!current) return "none";
	switch (current.kind) {
		case "profile":
			return `profile:${current.profileId}`;
		case "taxId":
			return "taxId";
		case "donation":
			return "donation";
		default:
			return "none";
	}
}

/**
 * 把 radio 選擇折成送出值。回**字串代表驗證失敗**（那句話直接給買家看）。
 *
 * 前端只做形狀檢查，後端仍會驗；**絕不**因為「格式看起來對」就跳過後端錯誤的顯示。
 * 正則比 AMEGO 更緊是刻意的——財政部對這幾種是固定長度。
 */
function buildSelection(
	choice: string,
	v: { taxId: string; title: string; donation: string },
): InvoiceMethodSelection | string {
	if (choice.startsWith("profile:")) {
		return {
			kind: "profile",
			profileId: choice.slice("profile:".length),
			displayValue: "",
		};
	}
	if (choice === "taxId") {
		if (!/^\d{8}$/.test(v.taxId)) return "統一編號請填 8 碼數字。";
		// 抬頭上限取自 AMEGO BuyerName（60 字元）。用 Array.from 數字元而不是 .length，
		// 後者對中文以外的補充平面字元會少算。
		const len = Array.from(v.title.trim()).length;
		if (len < 1 || len > 60) return "公司抬頭請填 1–60 個字。";
		return { kind: "taxId", buyerTaxId: v.taxId, buyerName: v.title.trim() };
	}
	if (choice === "donation") {
		if (!/^\d{3,7}$/.test(v.donation)) return "捐贈碼請填 3–7 碼數字。";
		return { kind: "donation", donationCode: v.donation };
	}
	return { kind: "none" };
}

function OtherOption({
	id,
	label,
	hint,
	checked,
	onSelect,
	children,
}: {
	id: string;
	label: string;
	hint?: string;
	checked: boolean;
	onSelect: () => void;
	children?: React.ReactNode;
}) {
	return (
		<div className="p-[12px_14px] bg-content2 border border-divider rounded-hkui-lg">
			<label className="flex items-center gap-3 cursor-pointer">
				<input
					type="radio"
					name="invoice-method"
					className="shrink-0"
					checked={checked}
					onChange={onSelect}
				/>
				<span className="flex-1 min-w-0 flex flex-col gap-[2px]">
					<span className="text-xs font-extrabold text-foreground">
						{label}
					</span>
					{hint ? (
						<span className="text-[0.625rem] text-foreground/60">{hint}</span>
					) : null}
				</span>
			</label>
			{/* 就地展開：只有選中時才渲染欄位，避免未選中的必填欄干擾表單語意 */}
			{checked && children ? <div id={`${id}-fields`}>{children}</div> : null}
		</div>
	);
}

function Field({
	label,
	value,
	onChange,
	maxLength,
	inputMode,
	placeholder,
}: {
	label: string;
	value: string;
	onChange: (v: string) => void;
	maxLength: number;
	inputMode?: "numeric";
	placeholder?: string;
}) {
	return (
		<label className="flex flex-col gap-1">
			<span className="text-[0.625rem] font-extrabold text-hk-accent">
				{label}
			</span>
			<input
				type="text"
				className="w-full p-[10px_12px] text-xs bg-content1 border border-divider rounded-hkui-lg"
				value={value}
				maxLength={maxLength}
				inputMode={inputMode}
				placeholder={placeholder}
				onChange={(e) => onChange(e.target.value)}
			/>
		</label>
	);
}
