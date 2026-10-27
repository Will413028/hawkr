"use client";

/**
 * Import 場景下的「單筆商品編輯」表單合成 — 沿用 ProductFormPage 的兩欄佈
 * 局、Card 分組、SectionHeader、底部固定操作列。差異：
 *
 *   - 沒有 「上架狀態 0/1/2」radio；改為 target_status 三選（ACTIVE / DRAFT /
 *     PENDING_REVIEW）的 select。
 *   - 沒有「進階設定」遮罩卡（import 場景不開放）。
 *   - 底部操作列多了「預覽列表」/「預覽此商品」按鈕，按下走新分頁開預覽路由。
 *   - errors / warnings 頂部總覽（紅 + 黃）+ DuplicateNameWarning。
 *
 * 元件組合：CategorySelector / DescriptionEditor / ProductImageUploader /
 * ImportItemSkuEditor / 認證欄位（共用 `ProductCertificationRow`，以 `mode="draft"`
 * 表示這一面的認證還沒有商品可掛、因此不得顯示任何審核狀態）全部 reuse 正式 form
 * 的視覺語彙。
 */

import * as React from "react";
import { ProductCertificationRow } from "@/components/shared/ProductCertificationRow";
import Button from "@/components/ui/Button";
import { Chip } from "@/components/ui/Chip";
import { Input } from "@/components/ui/Input";
import CategorySelector from "@/features/seller/components/CategorySelector";
import DescriptionEditor from "@/features/seller/components/DescriptionEditor";
import ProductImageUploader from "@/features/seller/components/ProductImageUploader";
import { SectionHeader } from "@/features/seller-center/products/components/SectionHeader";
import { PRODUCT_CERT_CATALOG } from "@/lib/products/certifications";
import { parseDescription } from "@/lib/products/seller-repo";
import type { useImportItemForm } from "../hooks/useImportItemForm";
import {
	fieldLabel,
	TARGET_STATUS_LABEL,
	UI_TEXT,
	validationErrorLabel,
} from "../i18n";
import {
	type ItemIssue,
	TARGET_STATUS,
	type TargetStatusValue,
} from "../types";
import { DuplicateNameWarning } from "./DuplicateNameWarning";
import ImportItemSkuEditor from "./ImportItemSkuEditor";

/**
 * 將同 (field, errorType) 的 issue 合併呈現「×N」。
 *
 * v3 後端雖然開始用 `ImportErrorMessageBuilder` 在內部 dedupe / aggregate，
 * 但 API 仍照原樣回所有 raw row；前端負責顯示層的視覺合併，避免畫面被
 * 同一條規則打成數十行。
 */
interface GroupedIssue {
	key: string;
	field: string;
	errorType: string;
	message: string;
	count: number;
}

function groupIssues(issues: ItemIssue[]): GroupedIssue[] {
	const map = new Map<string, GroupedIssue>();
	for (const it of issues) {
		const field = it.field ?? "";
		const key = `${field}|${it.errorType}`;
		const existed = map.get(key);
		if (existed) {
			existed.count += 1;
		} else {
			map.set(key, {
				key,
				field,
				errorType: it.errorType,
				message: it.message,
				count: 1,
			});
		}
	}
	return Array.from(map.values());
}

/** 與 ProductFormPage 完全相同的 Card 殼，確保整體節奏一致。 */
const CARD =
	"bg-hk-surface-card border border-hk-border-soft rounded-[24px] p-6 flex flex-col gap-4 scroll-mt-5 dark:bg-white/4 dark:border-[#f5dea5]/14";

const TARGET_STATUS_OPTIONS: TargetStatusValue[] = [
	TARGET_STATUS.ACTIVE,
	TARGET_STATUS.DRAFT,
];

interface Props {
	form: ReturnType<typeof useImportItemForm>;
	/** 「預覽列表」按鈕（新 tab 開）。 */
	previewListHref: string;
	/** 「預覽此商品」按鈕（新 tab 開）。 */
	previewItemHref: string;
	/** 「儲存」成功後路由。 */
	onSaved: () => void;
	/** 「儲存並下一筆」成功後路由（hook 不知道下一筆是哪個，由 page 提供）。 */
	onSavedAndNext: () => Promise<void> | void;
	/** 「取消」按鈕（含 dirty 確認）。 */
	onCancel: () => void;
	/** 「移除此商品」按鈕。 */
	onRemove: () => Promise<void> | void;
	/** 頂部資訊：CSV 行號 / product_key，供標題列顯示。 */
	csvLine?: string;
	productKey?: string;
}

export default function ImportItemEditForm({
	form,
	previewListHref,
	previewItemHref,
	onSaved,
	onSavedAndNext,
	onCancel,
	onRemove,
	csvLine,
	productKey,
}: Props) {
	const {
		draft,
		item,
		submitting,
		error,
		targetStatus,
		setTargetStatus,
		allowDuplicate,
		setAllowDuplicate,
		update,
		updateSku,
		addSku,
		removeSku,
		toggleCert,
		handleImagesChange,
		submit,
	} = form;

	const [hashtagInput, setHashtagInput] = React.useState("");
	const [imageUrlInput, setImageUrlInput] = React.useState("");
	const [imageUrlError, setImageUrlError] = React.useState<string | null>(null);
	const [pendingSaveAction, setPendingSaveAction] = React.useState<
		"save" | "save-and-next" | null
	>(null);
	const [removing, setRemoving] = React.useState(false);
	const actionBusy = submitting || removing;

	if (!item) {
		// 由 page 端負責顯示 loading；form 收到 null 時保險不渲染。
		return null;
	}

	const handleSave = async () => {
		setPendingSaveAction("save");
		try {
			const updated = await submit();
			if (updated) onSaved();
		} finally {
			setPendingSaveAction(null);
		}
	};

	const handleSaveAndNext = async () => {
		setPendingSaveAction("save-and-next");
		try {
			const updated = await submit();
			if (updated) await onSavedAndNext();
		} finally {
			setPendingSaveAction(null);
		}
	};

	const handleRemove = async () => {
		if (actionBusy) return;
		setRemoving(true);
		try {
			await onRemove();
		} finally {
			setRemoving(false);
		}
	};

	const addHashtag = () => {
		const t = hashtagInput.trim().replace(/^#/, "");
		if (!t) return;
		if ((draft.tags ?? []).length >= 10) return;
		if ((draft.tags ?? []).includes(t)) {
			setHashtagInput("");
			return;
		}
		update({ tags: [...(draft.tags ?? []), t] });
		setHashtagInput("");
	};

	const removeHashtag = (t: string) => {
		update({ tags: (draft.tags ?? []).filter((x) => x !== t) });
	};

	const addImageByUrl = () => {
		const raw = imageUrlInput.trim();
		if (!raw) {
			setImageUrlError("請貼上圖片網址");
			return;
		}
		if (!/^https:\/\//i.test(raw)) {
			setImageUrlError("僅支援 https 開頭的網址");
			return;
		}
		const existing = draft.images ?? [];
		if (existing.length >= 9) {
			setImageUrlError("圖片已達上限 9 張");
			return;
		}
		if (existing.some((img) => img.url === raw)) {
			setImageUrlError("此網址已加入");
			return;
		}
		handleImagesChange([...existing, { url: raw, stagedToken: undefined }]);
		setImageUrlInput("");
		setImageUrlError(null);
	};

	return (
		<div className="mx-auto w-full max-w-[1200px] px-5 pt-6 pb-[calc(120px+var(--hkui-space-6))]">
			{/* 標題列 */}
			<header className="mb-5 flex flex-col gap-[6px]">
				<div className="flex flex-wrap items-center gap-3">
					<h1 className="font-sans text-3xl font-bold tracking-tight text-hk-heading m-0">
						編輯匯入商品
					</h1>
					{productKey ? (
						<span className="font-mono text-xs text-hk-muted">
							{productKey}
							{csvLine ? ` · 第 ${csvLine} 行` : null}
						</span>
					) : null}
				</div>
				<p className="m-0 text-sm text-hk-muted leading-relaxed max-w-[640px]">
					{
						"與正式上架商品同步的編輯體驗：圖片可即時上傳、運費依包裝尺寸試算。認證人工審核功能建置期間僅能保留或移除既有資料；Commit 後才會真正上架。"
					}
				</p>
			</header>

			{/* errors / warnings 總覽（v3：同 (field, errorType) 合併顯示「×N」） */}
			{item.errors.length > 0 ? (
				<section
					className="mb-4 rounded-hk-md border border-[var(--hkui-danger)]/40 bg-[var(--hkui-danger-50)] p-3"
					role="alert"
				>
					<h3 className="m-0 text-sm font-semibold text-[var(--hkui-danger-700)]">
						需修正（{item.errors.length}）
					</h3>
					<ul className="mt-2 m-0 list-none space-y-1 text-xs text-[var(--hkui-danger-700)] p-0">
						{groupIssues(item.errors).map((g) => (
							<li key={g.key} className="flex flex-wrap gap-x-2">
								<span className="font-medium">
									{fieldLabel(g.field) || "—"}
								</span>
								<span>
									{validationErrorLabel(g.errorType, g.message)}
									{g.count > 1 ? (
										<span className="ml-1 opacity-70">×{g.count}</span>
									) : null}
								</span>
							</li>
						))}
					</ul>
				</section>
			) : null}
			{/* SKU 缺失 banner（issue「商品批次匯入仍然存在許多問題」#1/#2 真因）：
			    若 detail.skus 為空，後端 ValidateProductDraft 必定吐 SKUS_REQUIRED 短路
			    後續 SHIPPING / PRICE 等檢核，使用者在編輯頁看不到真正待修細節欄位。
			    歷史壞品 SKU 表已 0 筆（backend PR #1083 加 INVALID_ARGUMENT guard 預防後續發生），
			    此處顯示明確 banner，引導使用者從列表移除後重新匯入。 */}
			{(item.skus ?? []).length === 0 ? (
				<section
					className="mb-4 rounded-hk-md border border-[var(--hkui-danger)]/40 bg-[var(--hkui-danger-50)] p-3"
					role="alert"
				>
					<h3 className="m-0 text-sm font-semibold text-[var(--hkui-danger-700)]">
						此商品的規格資料缺失
					</h3>
					<p className="mt-2 m-0 text-xs text-[var(--hkui-danger-700)]">
						{
							"可能源自舊版的儲存錯誤。請從列表「移除」本筆後重新匯入；直接儲存會被系統拒絕（至少需要一組商品規格）。"
						}
					</p>
				</section>
			) : null}
			{item.warnings.length > 0 ? (
				<section
					className="mb-4 rounded-hk-md border border-[var(--hkui-warning)]/40 bg-[var(--hkui-warning-50)] p-3"
					role="alert"
				>
					<h3 className="m-0 text-sm font-semibold text-[var(--hkui-warning-700)]">
						警告（{item.warnings.length}）
					</h3>
					<ul className="mt-2 m-0 list-none space-y-1 text-xs text-[var(--hkui-warning-700)] p-0">
						{groupIssues(item.warnings).map((g) => (
							<li key={g.key} className="flex flex-wrap gap-x-2">
								<span className="font-medium">
									{fieldLabel(g.field) || "—"}
								</span>
								<span>
									{validationErrorLabel(g.errorType, g.message)}
									{g.count > 1 ? (
										<span className="ml-1 opacity-70">×{g.count}</span>
									) : null}
								</span>
							</li>
						))}
					</ul>
				</section>
			) : null}

			{/* DuplicateNameWarning — v3：欄位由 backend brief 帶入；
			    gateway 尚未上線新欄位時 fallback 走摘要文字（無連結 / 無 chip）。 */}
			{item.brief?.hasDuplicateExisting ? (
				<div className="mb-4">
					<DuplicateNameWarning
						existingProductId={item.brief.existingProductId ?? ""}
						existingProductName={item.brief.existingProductName ?? ""}
						existingStatusLabel={item.brief.existingProductStatusLabel ?? ""}
						allowDuplicate={allowDuplicate}
						onChange={setAllowDuplicate}
					/>
				</div>
			) : null}

			<form
				data-testid="item-edit-form"
				className="grid grid-cols-[minmax(0,1fr)_336px] gap-5 items-start max-[960px]:grid-cols-1"
				onSubmit={(e) => {
					e.preventDefault();
					void handleSave();
				}}
			>
				{/* ───────── 左主欄 ───────── */}
				<div className="flex flex-col gap-5 min-w-0">
					{/* 基本資訊 */}
					<section className={CARD} id="section-basic">
						<SectionHeader
							title="基本資訊"
							desc="商品名、分類、產地、標籤 — 買家最先看到的內容。"
						/>
						<Input
							id="import-product-name"
							data-testid="item-edit-name"
							variant="bordered"
							size="lg"
							label="商品名稱"
							isRequired
							value={draft.name}
							onChange={(e) => update({ name: e.target.value })}
							maxLength={80}
							placeholder="例：埔里高山高麗菜（限量）"
							description={`${draft.name.length} / 80 字`}
						/>
						<div
							data-testid="item-edit-category"
							className="flex flex-col gap-1.5 w-full"
						>
							<span
								className="text-sm font-medium text-foreground"
								data-slot="label"
							>
								商品分類
							</span>
							<CategorySelector
								value={draft.category_id}
								onChange={(id) => update({ category_id: id })}
							/>
							<small className="text-xs text-hk-muted" data-slot="hint">
								選到第三層才會送出。
							</small>
						</div>
						<Input
							id="import-product-origin"
							data-testid="item-edit-origin"
							variant="bordered"
							size="lg"
							label="產地"
							value={draft.origin ?? ""}
							onChange={(e) => update({ origin: e.target.value })}
							placeholder="例：南投縣埔里鎮"
							maxLength={40}
						/>
						{/* 標籤 */}
						<div className="flex flex-col gap-2">
							<span
								className="text-sm font-medium text-foreground"
								data-slot="label"
							>
								標籤（最多 10 個）
							</span>
							{(draft.tags ?? []).length > 0 ? (
								<div className="flex flex-wrap gap-2">
									{(draft.tags ?? []).map((t) => (
										<Chip
											key={t}
											variant="flat"
											color="primary"
											size="sm"
											onClose={() => removeHashtag(t)}
										>
											#{t}
										</Chip>
									))}
								</div>
							) : null}
							<div className="flex gap-2 items-end">
								<Input
									variant="bordered"
									size="sm"
									label="新增標籤"
									value={hashtagInput}
									onChange={(e) => setHashtagInput(e.target.value)}
									onKeyDown={(e) => {
										if (e.key === "Enter") {
											e.preventDefault();
											addHashtag();
										}
									}}
									placeholder="輸入後按 Enter"
								/>
								<Button
									type="button"
									variant="bordered"
									size="sm"
									onClick={addHashtag}
									disabled={(draft.tags ?? []).length >= 10}
								>
									新增
								</Button>
							</div>
						</div>
					</section>

					{/* 商品描述 — reuse 既有 EditorJS 組件 */}
					<section className={CARD} id="section-description">
						<SectionHeader
							title="商品描述"
							desc="支援多段文字 / 圖片區塊，第一段建議為核心賣點。圖片區塊可直接上傳，描述圖片依 backend issue #1014 完成度逐步落地。"
						/>
						<DescriptionEditor
							value={parseDescription(draft.description)}
							onChange={(next) => update({ description: next })}
						/>
					</section>

					{/* 商品圖片 — reuse ProductImageUploader */}
					<section className={CARD} id="section-images">
						<SectionHeader
							title="商品圖片"
							desc="第一張為主圖。建議比例 4:3，最少 800×600，最多 9 張。可拖曳 / 直接上傳，亦可保留 CSV 解析下載的既有圖。"
						/>
						<ProductImageUploader
							images={draft.images ?? []}
							onChange={handleImagesChange}
							max={9}
						/>
						{/* 貼 URL 新增 — 與 CSV 匯入流程一致，後端會下載並轉存 OSS */}
						<div className="flex flex-col gap-2 pt-3 border-t border-hk-border-soft/60 dark:border-[#f5dea5]/12">
							<span
								className="text-sm font-medium text-foreground"
								data-slot="label"
							>
								貼 URL 新增圖片
							</span>
							<small className="text-xs text-hk-muted">
								貼上 https 圖片網址後系統將自動下載並轉存到 OSS（與 CSV
								匯入流程一致）。
							</small>
							<div className="flex gap-2 items-end">
								<Input
									variant="bordered"
									size="sm"
									label="圖片網址"
									value={imageUrlInput}
									onChange={(e) => {
										setImageUrlInput(e.target.value);
										if (imageUrlError) setImageUrlError(null);
									}}
									onKeyDown={(e) => {
										if (e.key === "Enter") {
											e.preventDefault();
											addImageByUrl();
										}
									}}
									placeholder="https://example.com/photo.jpg"
								/>
								<Button
									type="button"
									variant="bordered"
									size="sm"
									onClick={addImageByUrl}
									disabled={(draft.images ?? []).length >= 9}
								>
									加入圖片
								</Button>
							</div>
							{imageUrlError ? (
								<small
									className="text-xs text-hk-danger dark:text-[#e6a3a3]"
									role="alert"
								>
									{imageUrlError}
								</small>
							) : null}
						</div>
					</section>

					{/* SKU */}
					<ImportItemSkuEditor
						skus={draft.skus}
						onUpdate={updateSku}
						onAdd={addSku}
						onRemove={removeSku}
					/>

					{error ? (
						<p
							className="m-0 py-3 px-4 bg-hk-danger/10 text-hk-danger rounded-md text-sm dark:bg-[rgba(218,110,110,0.16)] dark:text-[#e6a3a3]"
							role="alert"
						>
							{error}
						</p>
					) : null}
				</div>

				{/* ───────── 右側欄 ───────── */}
				<aside className="flex flex-col gap-5 min-w-0">
					{/* 上架狀態（target_status） */}
					<section className={CARD} id="section-target-status">
						<SectionHeader
							title="目標上架狀態"
							desc="Commit 時將以此狀態建立正式商品。可批次套用至同一任務下其他匯入商品。"
						/>
						<ul className="list-none m-0 p-0 flex flex-col gap-2">
							{TARGET_STATUS_OPTIONS.map((ts) => {
								const checked = targetStatus === ts;
								return (
									<li key={ts}>
										<label
											className={`flex items-center gap-3 py-[14px] px-4 bg-hk-surface-card border-[1.5px] border-hk-border-soft rounded-lg cursor-pointer transition-all duration-200 hover:border-hk-primary/40 ${
												checked
													? "border-hk-primary bg-[linear-gradient(135deg,rgb(var(--color-ink-600-rgb)/0.08),var(--theme-surface-card)_70%)] shadow-[0_0_0_3px_rgb(var(--color-ink-600-rgb)/0.12)] dark:border-hk-ink-200 dark:bg-[linear-gradient(135deg,rgb(var(--color-ink-300-rgb)/0.22),rgba(255,255,255,0.05)_70%)]"
													: "dark:border-[#f5dea5]/18 dark:hover:border-hk-ink-200/55"
											}`}
										>
											<input
												type="radio"
												name="import-target-status"
												checked={checked}
												className="w-[18px] h-[18px] accent-hkui-primary shrink-0"
												onChange={() => setTargetStatus(ts)}
											/>
											<span className="flex flex-col gap-0.5">
												<strong className="text-sm font-bold text-foreground">
													{TARGET_STATUS_LABEL[ts]}
												</strong>
												<span className="text-xs text-foreground-soft font-normal">
													{ts === TARGET_STATUS.ACTIVE
														? "Commit 時直接上架販售"
														: "Commit 時建立為正式商品，不上架"}
												</span>
											</span>
										</label>
									</li>
								);
							})}
						</ul>
					</section>

					{/* 認證 / 信任標章 — 與商品表單共用 ProductCertificationRow */}
					<section className={CARD} id="section-trust">
						<SectionHeader
							title="認證 / 信任標章"
							desc="CSV 帶進來的認證尚未送審；商品匯入後可在商品編輯頁送出申請並追蹤審核狀態。"
						/>
						<ul className="list-none m-0 p-0 flex flex-col gap-2">
							{PRODUCT_CERT_CATALOG.map((c) => (
								<li key={c.code}>
									<ProductCertificationRow
										item={c}
										mode="draft"
										entry={draft.certifications?.find((e) => e.code === c.code)}
										onToggle={() => toggleCert(c.code)}
										idPrefix="import-cert"
									/>
								</li>
							))}
						</ul>
					</section>

					{/* CSV 來源溫層摘要（純資訊，唯讀） */}
					<section className={CARD} id="section-csv-shipping-hint">
						<SectionHeader
							title="CSV 原始運送設定"
							desc="供對照原始 CSV 設定；實際送出值以下方 SKU 卡內「可配送方案」為準。"
						/>
						<div className="flex flex-col gap-1.5 text-xs text-hk-muted">
							{(item.skus ?? []).slice(0, 3).map((sku, i) => {
								// Backend 用 shippingMethods 陣列 `{ method, temperature }` 攜帶，
								// method/temperature 為 code value（如 "3001"/"3201"），這裡只
								// 列原始 code 對照（label 對應仍在下方 SKU 卡內顯示）。
								const carriers = (sku.shippingMethods ?? []).map(
									(m) => `${m.method}/${m.temperature}`,
								);
								return (
									<div
										// biome-ignore lint/suspicious/noArrayIndexKey: ordered csv rows
										key={`csv-sku-${i}`}
										className="flex flex-wrap gap-2"
									>
										<span className="font-mono">SKU #{i + 1}：</span>
										<span>{carriers.length ? carriers.join(" / ") : "—"}</span>
									</div>
								);
							})}
							{(item.skus ?? []).length > 3 ? (
								<p className="m-0 text-[0.6875rem] text-hk-muted/80">
									其餘 SKU 請於下方規格卡內逐筆檢視。
								</p>
							) : null}
						</div>
					</section>
				</aside>
			</form>

			{/* Sticky 底部操作列 */}
			<div
				className="seller-import-item__action-bar fixed left-0 right-0 bottom-0 bg-hk-surface-card border-t border-hk-border-soft shadow-hk-lift px-5 py-3 flex gap-4 justify-between items-center z-[var(--z-sticky)] dark:bg-white/4 dark:border-[#f5dea5]/14"
				role="toolbar"
			>
				<div className="seller-import-item__action-meta flex flex-col gap-[2px] min-w-0">
					<strong className="text-base font-semibold text-hk-heading whitespace-nowrap overflow-hidden text-ellipsis">
						{draft.name || "未命名商品"}
					</strong>
					<span className="text-xs text-hk-muted">
						{draft.skus?.length ?? 0} 規格 · {draft.certifications?.length ?? 0}{" "}
						認證
					</span>
				</div>
				<div className="seller-import-item__action-rail flex gap-2 flex-wrap justify-end">
					<Button
						variant="light"
						color="danger"
						size="sm"
						onClick={() => void handleRemove()}
						disabled={actionBusy}
						isLoading={removing}
						loadingLabel="移除匯入商品中…"
					>
						{UI_TEXT.actions.removeItem}
					</Button>
					<Button
						variant="bordered"
						color="default"
						size="sm"
						href={previewListHref}
						target="_blank"
						rel="noopener noreferrer"
						disabled={actionBusy}
					>
						預覽列表
					</Button>
					<Button
						variant="bordered"
						color="default"
						size="sm"
						href={previewItemHref}
						target="_blank"
						rel="noopener noreferrer"
						disabled={actionBusy}
					>
						預覽此商品
					</Button>
					<Button
						data-testid="item-edit-cancel"
						variant="light"
						color="default"
						size="sm"
						onClick={onCancel}
						disabled={actionBusy}
					>
						{UI_TEXT.actions.cancelEdit}
					</Button>
					<Button
						data-testid="item-edit-save-and-next"
						variant="bordered"
						color="primary"
						size="sm"
						onClick={() => void handleSaveAndNext()}
						disabled={actionBusy}
						isLoading={pendingSaveAction === "save-and-next"}
						loadingLabel="儲存並前往下一筆中…"
					>
						{UI_TEXT.actions.saveAndNext}
					</Button>
					<Button
						data-testid="item-edit-save"
						color="primary"
						size="sm"
						onClick={() => void handleSave()}
						disabled={actionBusy}
						isLoading={pendingSaveAction === "save"}
						loadingLabel="儲存商品中…"
					>
						{UI_TEXT.actions.save}
					</Button>
				</div>
			</div>
		</div>
	);
}
