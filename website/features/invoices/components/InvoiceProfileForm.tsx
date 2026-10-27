"use client";

import * as React from "react";
import { Input, Select } from "@/components/ui";
import Button from "@/components/ui/Button";
import {
	INVOICE_CARRIER_FIELD_TEXT,
	INVOICE_CARRIER_FORMAT_TEXT,
	INVOICE_PROFILE_TEXT,
} from "@/features/invoices/i18n";
import type { InvoiceCarrierTypeValue } from "@/lib/codemeta/generated";
import { InvoiceCarrierType } from "@/lib/codemeta/generated";
import {
	CARRIER_VALUE_MAX_LENGTH,
	type CarrierShapeError,
	carrierValueIsMasked,
	INVOICE_CARRIER_TYPES,
	invoiceCarrierLabel,
	TAX_ID_MAX_LENGTH,
	validateBuyerName,
	validateCarrierValue,
	validateTaxId,
} from "@/lib/invoices/carrier";
import type { InvoiceProfile } from "@/lib/invoices/types";

/** 備註欄上限。58A §B08 定 20 字（後端 `profileLabelMaxRunes` 是 30，取較嚴者）。 */
const LABEL_MAX_LENGTH = 20;

export interface InvoiceProfileFormValues {
	carrierType: InvoiceCarrierTypeValue;
	value: string;
	buyerTaxId: string;
	buyerName: string;
	label: string;
	setDefault: boolean;
}

interface InvoiceProfileFormProps {
	/** `edit` 時類型唯讀（要換類型＝刪除後新增，端點 3 沒有 `carrierType`）。 */
	mode: "create" | "edit";
	/** `edit` 時的原資料。 */
	profile?: InvoiceProfile | null;
	submitting: boolean;
	/** 表單級錯誤（9303 上限／9306 互斥）。 */
	formError?: string | null;
	/** 欄位級錯誤（9305），顯示在對應欄位下方而不是頁首（§B-1.0.6）。 */
	fieldError?: string | null;
	submitLabel?: string;
	onCancel: () => void;
	onSubmit: (values: InvoiceProfileFormValues) => void;
}

const FORMAT_MESSAGE: Record<CarrierShapeError, string> = {
	mobileBarcode: INVOICE_CARRIER_FORMAT_TEXT.mobileBarcode,
	citizenCert: INVOICE_CARRIER_FORMAT_TEXT.citizenCert,
	donation: INVOICE_CARRIER_FORMAT_TEXT.donation,
	taxId: INVOICE_CARRIER_FORMAT_TEXT.taxId,
	buyerName: INVOICE_CARRIER_FORMAT_TEXT.buyerName,
};

function valueFieldText(carrierType: string) {
	switch (carrierType) {
		case InvoiceCarrierType.MOBILE_BARCODE:
			return INVOICE_CARRIER_FIELD_TEXT.mobileBarcode;
		case InvoiceCarrierType.CITIZEN_CERT:
			return INVOICE_CARRIER_FIELD_TEXT.citizenCert;
		case InvoiceCarrierType.DONATION:
			return INVOICE_CARRIER_FIELD_TEXT.donation;
		default:
			// 前端還不認得的型別（含 TAX_ID，它不用這一欄）。用碼表名稱當 label，
			// 讓畫面仍然可用而不是空白。
			return {
				label: invoiceCarrierLabel(carrierType),
				placeholder: "",
				description: "",
			};
	}
}

/**
 * 常用發票資料的新增／編輯表單（58A §B08、§B02 共用）。
 *
 * 三個設計決定：
 *   1. **編輯時類型唯讀，用 `<p>` 不用 disabled 的 `<Select>`**——disabled 的
 *      下拉看起來像壞掉。
 *   2. **遮罩型載具的原值不帶回欄位**：`MOBILE_BARCODE`／`CITIZEN_CERT` 在 DB
 *      是密文，Gateway 拿不到明碼 ⇒ placeholder 顯示原遮罩值、欄位留空，並用
 *      `description` 說明為什麼。`TAX_ID`／`DONATION` 的值後端本來就回明碼，
 *      帶回去才是對的（叫使用者重打一次自己的統編是沒有理由的摩擦）。
 *   3. **只做形狀檢查**（`lib/invoices/carrier.ts`），後端仍會驗；後端回來的
 *      9305 由 `fieldError` 進同一個欄位下方。
 */
export function InvoiceProfileForm({
	mode,
	profile = null,
	submitting,
	formError = null,
	fieldError = null,
	submitLabel = INVOICE_PROFILE_TEXT.save,
	onCancel,
	onSubmit,
}: InvoiceProfileFormProps) {
	const editing = mode === "edit" && profile != null;
	const masked = editing ? carrierValueIsMasked(profile.carrierType) : false;

	const [carrierType, setCarrierType] = React.useState<InvoiceCarrierTypeValue>(
		InvoiceCarrierType.MOBILE_BARCODE,
	);
	const [value, setValue] = React.useState(
		editing && !masked ? profile.displayValue : "",
	);
	const [buyerTaxId, setBuyerTaxId] = React.useState(
		editing ? profile.buyerTaxId : "",
	);
	const [buyerName, setBuyerName] = React.useState(
		editing ? profile.buyerName : "",
	);
	const [label, setLabel] = React.useState(editing ? profile.label : "");
	const [setDefault, setSetDefault] = React.useState(false);
	const [touched, setTouched] = React.useState<Record<string, boolean>>({});

	// 編輯模式的類型來自那筆資料（不可變更）；新增模式來自使用者選擇。
	const activeType = editing ? profile.carrierType : carrierType;
	const isTaxId = activeType === InvoiceCarrierType.TAX_ID;
	const fieldText = valueFieldText(activeType);

	const valueShape = isTaxId ? null : validateCarrierValue(activeType, value);
	const taxIdShape = isTaxId ? validateTaxId(buyerTaxId) : null;
	const nameShape = isTaxId ? validateBuyerName(buyerName) : null;

	const touch = (field: string) =>
		setTouched((current) => ({ ...current, [field]: true }));

	const valueMessage =
		touched.value && valueShape
			? FORMAT_MESSAGE[valueShape]
			: (fieldError ?? "");
	const taxIdMessage =
		touched.buyerTaxId && taxIdShape
			? FORMAT_MESSAGE[taxIdShape]
			: (fieldError ?? "");
	const nameMessage =
		touched.buyerName && nameShape ? FORMAT_MESSAGE[nameShape] : "";

	const canSubmit = !submitting && !valueShape && !taxIdShape && !nameShape;

	const submit = (event: React.FormEvent) => {
		event.preventDefault();
		if (!canSubmit) return;
		onSubmit({
			carrierType: activeType as InvoiceCarrierTypeValue,
			// TAX_ID 的 `value` 必須為空，後端靠「哪幾欄是空的」判形狀。
			value: isTaxId ? "" : value.trim(),
			buyerTaxId: isTaxId ? buyerTaxId.trim() : "",
			buyerName: isTaxId ? buyerName.trim() : "",
			label: label.trim(),
			setDefault,
		});
	};

	return (
		<form className="flex flex-col gap-5" onSubmit={submit}>
			{editing ? (
				<div className="flex flex-col gap-1">
					<span className="text-sm font-semibold text-[var(--hkui-foreground)]">
						{INVOICE_PROFILE_TEXT.typeField}
					</span>
					<p className="m-0 text-sm text-foreground-soft">
						{invoiceCarrierLabel(profile.carrierType)}
					</p>
				</div>
			) : (
				<Select
					variant="bordered"
					size="lg"
					label={INVOICE_PROFILE_TEXT.typeField}
					isRequired
					value={carrierType}
					disabled={submitting}
					onChange={(event) => {
						setCarrierType(event.target.value as InvoiceCarrierTypeValue);
						setValue("");
						setBuyerTaxId("");
						setBuyerName("");
						setTouched({});
					}}
				>
					{INVOICE_CARRIER_TYPES.map((option) => (
						<option key={option} value={option}>
							{invoiceCarrierLabel(option)}
						</option>
					))}
				</Select>
			)}

			{isTaxId ? (
				<>
					<Input
						variant="bordered"
						size="lg"
						label={INVOICE_CARRIER_FIELD_TEXT.taxId.label}
						isRequired
						inputMode="numeric"
						maxLength={TAX_ID_MAX_LENGTH}
						placeholder={INVOICE_CARRIER_FIELD_TEXT.taxId.placeholder}
						disabled={submitting}
						value={buyerTaxId}
						onChange={(event) => setBuyerTaxId(event.target.value)}
						onBlur={() => touch("buyerTaxId")}
						errorMessage={taxIdMessage}
						isInvalid={Boolean(taxIdMessage)}
					/>
					<Input
						variant="bordered"
						size="lg"
						label={INVOICE_CARRIER_FIELD_TEXT.buyerName.label}
						isRequired
						maxLength={60}
						placeholder={INVOICE_CARRIER_FIELD_TEXT.buyerName.placeholder}
						disabled={submitting}
						value={buyerName}
						onChange={(event) => setBuyerName(event.target.value)}
						onBlur={() => touch("buyerName")}
						errorMessage={nameMessage}
						isInvalid={Boolean(nameMessage)}
					/>
				</>
			) : (
				<Input
					variant="bordered"
					size="lg"
					label={fieldText.label}
					isRequired
					maxLength={CARRIER_VALUE_MAX_LENGTH[activeType]}
					placeholder={
						masked && profile ? profile.displayValue : fieldText.placeholder
					}
					description={
						masked
							? INVOICE_PROFILE_TEXT.editValueDescription
							: fieldText.description
					}
					disabled={submitting}
					value={value}
					onChange={(event) => setValue(event.target.value)}
					onBlur={() => touch("value")}
					errorMessage={valueMessage}
					isInvalid={Boolean(valueMessage)}
				/>
			)}

			<Input
				variant="bordered"
				size="lg"
				label={INVOICE_PROFILE_TEXT.labelField}
				maxLength={LABEL_MAX_LENGTH}
				placeholder={INVOICE_PROFILE_TEXT.labelPlaceholder}
				disabled={submitting}
				value={label}
				onChange={(event) => setLabel(event.target.value)}
			/>

			{/* 端點 3 沒有 setDefault（預設由 supersede 繼承），所以編輯模式不給這個
			    選項——給了就是宣稱一個送不出去的意圖。改預設走清單列的「設為預設」。 */}
			{editing ? null : (
				<label className="inline-flex items-start gap-2 text-sm text-foreground-muted cursor-pointer select-none">
					<input
						type="checkbox"
						className="w-[18px] h-[18px] mt-0.5 accent-hkui-primary cursor-pointer"
						checked={setDefault}
						disabled={submitting}
						onChange={(event) => setSetDefault(event.target.checked)}
					/>
					<span>{INVOICE_PROFILE_TEXT.setDefaultCheckbox}</span>
				</label>
			)}

			{formError ? (
				<p
					className="my-0 px-4 py-3 rounded-hkui-md bg-danger-50 text-danger-700 text-sm"
					role="alert"
				>
					{formError}
				</p>
			) : null}

			<div className="flex gap-2 justify-end">
				<Button
					type="button"
					variant="bordered"
					color="default"
					size="lg"
					disabled={submitting}
					onClick={onCancel}
				>
					{INVOICE_PROFILE_TEXT.cancel}
				</Button>
				<Button
					type="submit"
					size="lg"
					disabled={!canSubmit}
					isLoading={submitting}
					loadingLabel={INVOICE_PROFILE_TEXT.saving}
				>
					{submitLabel}
				</Button>
			</div>
		</form>
	);
}
