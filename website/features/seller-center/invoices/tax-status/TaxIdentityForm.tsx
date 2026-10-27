"use client";

/**
 * S02 的稅務身分表單（端點 18 存草稿）。
 *
 * 🔴 **這是整條發票鏈的第一步。** 只有端點 18 會寫 `evidence_object_keys`，
 * 而端點 19（送審）的守衛讀的正是那一欄 ⇒ 沒有這張表單，賣家從零走不到可送審的
 * `DRAFT`，也就永遠不會被核定、永遠串不了 AMEGO、訂單永遠開不出發票。
 *
 * 🔴 **稅籍證明走 secure 兩段式上傳**（`uploadSecureDocument`），bytes 不落公開 OSS。
 * 用公開通道的 `uploadImageFile` 會讓稅籍證明變成任何人拿到 key 就下得到的檔案
 * ——那是賣家的國稅局文件。
 *
 * ⚠️ `evidenceObjectKeys`（已落庫的既有佐證）與 `evidenceStagedTokens`（這次新上傳）
 * 是兩件事：回存時前者要原樣帶回，否則既有佐證會被清空。
 */

import * as React from "react";
import Button from "@/components/ui/Button";
import { Input, Select } from "@/components/ui/Input";
import {
	SELLER_TAX_ENTITY_TYPE_LABEL,
	SELLER_TAX_OBLIGATION_LABEL,
	SellerTaxEntityType,
	SellerTaxObligation,
} from "@/lib/codemeta/enums";
import { useSaveSellerTaxDraft } from "@/lib/hooks/useSellerInvoiceRemedy";
import { invoiceBizCode, invoiceErrorMessage } from "@/lib/invoices/buyer-repo";
import type {
	SellerTaxIdentityInput,
	SellerTaxProfile,
} from "@/lib/invoices/seller-types";
import { uploadSecureDocument } from "@/lib/media/secure-upload";

/** 上傳給後端辨識用的文件類型；與商家認證共用同一條 secure 通道。 */
const EVIDENCE_DOC_TYPE = "seller_tax_evidence";

export function TaxIdentityForm({
	profile,
}: {
	profile: SellerTaxProfile | null;
}) {
	const draft = useSaveSellerTaxDraft(profile?.version ?? "0");

	const [form, setForm] = React.useState<SellerTaxIdentityInput>(() =>
		initialForm(profile),
	);
	const [stagedTokens, setStagedTokens] = React.useState<string[]>([]);
	const [uploading, setUploading] = React.useState(false);
	const [uploadError, setUploadError] = React.useState("");

	const set = <K extends keyof SellerTaxIdentityInput>(
		key: K,
		value: SellerTaxIdentityInput[K],
	) => setForm((prev) => ({ ...prev, [key]: value }));

	const existingCount = profile?.documents.length ?? 0;
	// 至少要有一份佐證才送得出審核（後端 400）——已落庫的或這次新傳的都算。
	const hasEvidence = existingCount > 0 || stagedTokens.length > 0;

	const onPickFile = async (file: File | null) => {
		if (!file) return;
		setUploadError("");
		setUploading(true);
		try {
			const result = await uploadSecureDocument(file, EVIDENCE_DOC_TYPE);
			setStagedTokens((prev) => [...prev, result.stagedToken]);
		} catch (e) {
			setUploadError(e instanceof Error ? e.message : "上傳失敗，請再試一次。");
		} finally {
			setUploading(false);
		}
	};

	const onSave = () => {
		draft.save({
			taxIdentity: form,
			// 🔴 既有佐證原樣帶回——不帶會被清空。
			evidenceObjectKeys: (profile?.documents ?? [])
				.map((d) => d.fileKey)
				.filter(Boolean),
			evidenceStagedTokens: stagedTokens,
		});
	};

	// 存檔成功後把 staged token 清掉：它們已經落庫成 objectKey，留著會在下次存檔時
	// 被當成「又上傳了一份」重複送出。
	React.useEffect(() => {
		if (draft.succeeded) {
			setStagedTokens([]);
			draft.reset();
		}
	}, [draft.succeeded, draft.reset]);

	return (
		<section className="seller-card flex flex-col gap-3 p-[var(--hkui-space-4)]">
			<strong className="text-sm font-bold">稅務身分資料</strong>
			<p className="m-0 text-[0.8125rem] text-[var(--hkui-foreground-soft)]">
				{
					"填好基本資料並上傳稅籍證明後存檔，才能送出審核。存檔不會送出，你可以分次填。"
				}
			</p>

			<div className="grid grid-cols-1 gap-3 md:grid-cols-2">
				<Select
					label="營業型態"
					value={form.entityType}
					onChange={(e) => set("entityType", e.target.value)}
				>
					<option value="">請選擇</option>
					{Object.values(SellerTaxEntityType).map((v) => (
						<option key={v} value={v}>
							{SELLER_TAX_ENTITY_TYPE_LABEL[v] ?? v}
						</option>
					))}
				</Select>
				<Select
					label="開票義務（你的認知）"
					description="平台會依核定資料確認，這裡填你目前的認知即可。"
					value={form.claimedStatus}
					onChange={(e) => set("claimedStatus", e.target.value)}
				>
					<option value="">請選擇</option>
					{Object.values(SellerTaxObligation).map((v) => (
						<option key={v} value={v}>
							{SELLER_TAX_OBLIGATION_LABEL[v] ?? v}
						</option>
					))}
				</Select>
				<Input
					label="統一編號"
					description="免用統一發票的自然人賣家可留空。"
					inputMode="numeric"
					maxLength={8}
					value={form.taxId}
					onChange={(e) => set("taxId", e.target.value.replace(/\D/g, ""))}
				/>
				<Input
					label="法定名稱"
					value={form.legalName}
					onChange={(e) => set("legalName", e.target.value)}
				/>
				<Input
					label="負責人姓名"
					value={form.responsibleName}
					onChange={(e) => set("responsibleName", e.target.value)}
				/>
				{/*
				 * 🔴 **只有一個地址欄，不是三個。**
				 *
				 * 後端的 `joinAddress(city, district, detail)` 把三欄**串成單一
				 * `business_address`**，回程只填得出 `addressDetail`（`taxSubmitGate`
				 * 的註解也記著這件事）⇒ 拆成三格的話，賣家存完再回來會看到縣市與
				 * 鄉鎮市區永遠空白、而完整地址擠在第三格裡（2026-09-01 dev 實測截圖）。
				 *
				 * 送單一欄進 `addressDetail` 產生的 `business_address` 與拆三欄**完全相同**
				 * （空字串會被 join 濾掉），但它能原樣 round-trip。
				 */}
				<Input
					label="營業地址"
					description="請填完整地址（縣市 ＋ 鄉鎮市區 ＋ 街道門牌）。"
					wrapperClassName="md:col-span-2"
					value={form.addressDetail}
					onChange={(e) => set("addressDetail", e.target.value)}
				/>
			</div>

			<div className="flex flex-col gap-2 pt-2 border-t border-divider">
				<strong className="text-[0.8125rem] font-bold">稅籍證明文件</strong>
				<p className="m-0 text-[0.75rem] text-[var(--hkui-foreground-soft)]">
					{"支援圖片或 PDF。文件會加密保存，只有平台審核人員看得到。"}
				</p>
				<label className="flex items-center gap-2">
					<input
						type="file"
						accept="image/*,application/pdf"
						className="text-[0.75rem]"
						disabled={uploading}
						onChange={(e) => void onPickFile(e.target.files?.[0] ?? null)}
					/>
				</label>
				{uploading ? (
					<small role="status" className="text-[0.75rem] text-foreground/60">
						上傳中…
					</small>
				) : null}
				{uploadError ? (
					<small role="alert" className="text-[0.75rem] text-danger-700">
						{uploadError}
					</small>
				) : null}
				<small className="text-[0.75rem] text-foreground/60">
					{`已有 ${existingCount} 份已保存的文件；本次新增 ${stagedTokens.length} 份。`}
				</small>
			</div>

			<div className="flex flex-col gap-2">
				<Button
					size="lg"
					variant="solid"
					isLoading={draft.saving}
					loadingLabel="存檔中…"
					disabled={uploading}
					onClick={onSave}
				>
					儲存草稿
				</Button>
				{!hasEvidence ? (
					<small role="status" className="text-[0.75rem] text-foreground/60">
						{"還沒有稅籍證明文件。可以先存草稿，但送出審核前至少要有一份。"}
					</small>
				) : null}
				{draft.error ? (
					<small role="alert" className="text-[0.75rem] text-danger-700">
						{draftErrorText(draft.error)}
					</small>
				) : null}
			</div>
		</section>
	);
}

/** 端點 18 的樂觀鎖碼（`SELLER_TAX_PROFILE_VERSION_CONFLICT`）。 */
const TAX_PROFILE_VERSION_CONFLICT = 9322;

/**
 * 存草稿失敗時要顯示哪一句。
 *
 * 🔴 **原本對任何錯誤都印「資料可能已被更新，請重新整理後再試」**，而端點 18 的
 * 絕大多數失敗是欄位驗證（後端正確回 400 ＋「統一編號必須是 8 位數字」之類）。
 * 後果是賣家填錯統編，看到的卻是一句叫他重新整理的話——照做、再填同樣的錯，
 * 而畫面上沒有任何線索指向統編那一欄。
 *
 * ⚠️ 版本衝突（9322）**仍要用自有文案**：後端那句話講的是「稅務資格資料已被更新」，
 * 而賣家的下一步是重新整理，那是這一句唯一講得清楚的地方。判準是碼不是 HTTP status
 * ——400 與 409 在這裡都可能出現。
 */
function draftErrorText(err: unknown): string {
	if (invoiceBizCode(err) === TAX_PROFILE_VERSION_CONFLICT) {
		return "存檔失敗。資料可能已被更新，請重新整理後再試。";
	}
	const message = invoiceErrorMessage(err);
	return message || "存檔失敗，請確認欄位內容後再試一次。";
}

function initialForm(profile: SellerTaxProfile | null): SellerTaxIdentityInput {
	const id = profile?.taxIdentity;
	return {
		entityType: id?.entityType ?? "",
		taxId: id?.taxId ?? "",
		legalName: id?.legalName ?? "",
		responsibleName: id?.responsibleName ?? "",
		addressCity: id?.addressCity ?? "",
		addressDistrict: id?.addressDistrict ?? "",
		addressDetail: id?.addressDetail ?? "",
		claimedStatus: id?.claimedStatus ?? "",
	};
}
