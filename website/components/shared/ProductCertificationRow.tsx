"use client";

/**
 * 認證 / 信任標章的一列 —— 三個認證輸入面共用（19 號設計書 §14.2）。
 *
 * 在這之前，商品表單與 CSV 匯入逐筆編輯各自帶著一份逐字相同的 `CertRow`，代營運工作區
 * 又另有一套 chip 列；「認證現在算什麼狀態」因此有三個各自演化的答案。本檔把判讀
 * （{@link resolveCertTypeView}）與徽章（{@link ProductCertificationStatusBadge}）收成
 * 一份，三個面全部由此取用。
 *
 * ⚠️ **代營運工作區只取徽章、不取這一列**：那一面是 `adm-*` 設計系統的 chip 欄位，換成
 * 目錄式的六列是版面變更；它以 `mode="readonly"` 呼叫共用判讀，於是「沒有登入態就不做
 * 即時驗證、不呼叫任何認證端點」這條約束由共用層保證，而不是靠各面自律。
 *
 * ⚠️ 本元件**自己不打任何端點**。送出與撤回由宿主注入（`onSubmit` / `onToggle`），因為
 * 只有宿主知道自己有沒有 `productId`、有沒有賣家登入態。
 */

import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import Button from "@/components/ui/Button";
import { Chip } from "@/components/ui/Chip";
import { Input } from "@/components/ui/Input";
import {
	type CertificationSurfaceMode,
	type CertStatusTone,
	type CertStatusView,
	resolveCertTypeView,
} from "@/lib/products/certification-status";
import type { ProductCertCatalogItem } from "@/lib/products/certifications";
import type {
	ProductCertificationApplication,
	ProductCertificationEntry,
} from "@/lib/products/seller-repo";

const TONE_TO_CHIP_COLOR: Record<
	CertStatusTone,
	"default" | "primary" | "success" | "warning" | "danger"
> = {
	default: "default",
	primary: "primary",
	success: "success",
	warning: "warning",
	danger: "danger",
};

/**
 * 審核狀態徽章。文字一律來自 CodeMeta 生成表（`PRODUCT_CERTIFICATION_STATUS_LABEL`），
 * 呼叫端不得自己寫碼值到中文的對照。
 */
export function ProductCertificationStatusBadge({
	view,
	size = "sm",
}: {
	view: CertStatusView | null;
	size?: "sm" | "md";
}) {
	if (!view) return null;
	return (
		<Chip variant="flat" color={TONE_TO_CHIP_COLOR[view.tone]} size={size}>
			{view.label}
		</Chip>
	);
}

export interface ProductCertificationRowProps {
	item: ProductCertCatalogItem;
	/**
	 * 這個面的能力與資料語意。
	 *
	 * `"review"` 需要商品已存在（有 `productId` 才打得到申請端點）；`"draft"` 是商品
	 * 建立頁與匯入逐筆編輯，那裡的值只是草稿宣稱，**不得顯示任何審核狀態**。
	 */
	mode: Exclude<CertificationSurfaceMode, "readonly">;
	/** 商品上掛著的認證。`review` 模式下它是 approved-only 投影。 */
	entry?: ProductCertificationEntry;
	/** 申請清單（`review` 模式才有）。 */
	applications?: readonly ProductCertificationApplication[];
	/** 取消勾選：`review` 撤回申請、`draft` 移除草稿條目。 */
	onToggle: () => void;
	/** 送出申請。只有 `review` 模式會呼叫。 */
	onSubmit?: (certNo: string) => void | Promise<void>;
	/** 可編輯草稿：空字串代表選取，`null` 代表移除；商品建立後由宿主送出。 */
	onDraftChange?: (certNo: string | null) => void;
	/** 送出／撤回進行中，期間鎖住這一列。 */
	busy?: boolean;
	/** 同一頁多個實例時分開 aria id。 */
	idPrefix: string;
	/** 這一列的錯誤訊息（宿主擁有錯誤狀態）。 */
	error?: string | null;
}

export function ProductCertificationRow({
	item,
	mode,
	entry,
	applications,
	onToggle,
	onSubmit,
	onDraftChange,
	busy = false,
	idPrefix,
	error,
}: ProductCertificationRowProps) {
	const view = resolveCertTypeView({
		code: item.code,
		mode,
		entry,
		applications,
	});

	// 尚未被宣稱的類型：勾選只是把輸入欄展開，按下「申請」才真的送出。
	// 兩者分開是刻意的——勾一下就送出一筆待審申請，賣家沒有反悔的空檔。
	const [expanded, setExpanded] = React.useState(false);
	const [typed, setTyped] = React.useState<string | null>(null);

	// 送出成功後宿主會重抓，`view.certNo` 因此改變；此時丟掉本地輸入值，
	// 讓欄位回到後端的權威值（否則畫面會停在使用者剛打的那一版）。
	// biome-ignore lint/correctness/useExhaustiveDependencies: 重置的觸發條件就是權威值改變
	React.useEffect(() => {
		setTyped(null);
	}, [view.certNo]);

	const review = mode === "review";
	const editableDraft = mode === "draft" && typeof onDraftChange === "function";
	const editable = review || editableDraft;
	const open = editableDraft ? view.checked : view.checked || expanded;
	// 草稿由宿主即時保存，不能用判讀層 trim 過的值蓋掉使用者正在輸入的空白。
	const certNo = editableDraft ? (entry?.certNo ?? "") : (typed ?? view.certNo);
	const trimmed = certNo.trim();
	const canSubmit =
		review &&
		typeof onSubmit === "function" &&
		!busy &&
		trimmed !== "" &&
		trimmed !== view.certNo.trim();

	const noteId = `${idPrefix}-note-${item.code}`;
	const statusId = `${idPrefix}-status-${item.code}`;
	const errorId = `${idPrefix}-error-${item.code}`;

	const handleToggle = () => {
		if (busy || (!editable && !view.checked)) return;
		if (editableDraft) {
			onDraftChange(view.checked ? null : "");
			return;
		}
		if (view.checked) {
			onToggle();
			return;
		}
		setExpanded((prev) => !prev);
	};

	const handleChange = (event: React.ChangeEvent<HTMLInputElement>) => {
		if (busy || !editable) return;
		if (editableDraft) {
			onDraftChange(event.target.value);
			return;
		}
		setTyped(event.target.value);
	};

	const handleSubmit = () => {
		if (!canSubmit || !onSubmit) return;
		void onSubmit(trimmed);
	};

	return (
		<div
			className={`flex flex-col gap-3 px-4 py-3 bg-hk-surface-card border-[1.5px] rounded-lg transition-[border-color,background-color,box-shadow] duration-200 motion-reduce:transition-none dark:bg-white/4 ${
				open
					? "border-hk-primary bg-[linear-gradient(135deg,var(--page-cert-tint),var(--theme-surface-card)_70%)] shadow-[0_0_0_3px_var(--page-cert-ring)] dark:border-[var(--page-cert-accent-on-dark)] dark:bg-[linear-gradient(135deg,var(--page-cert-tint-on-dark),var(--page-cert-tint-fade-on-dark)_70%)]"
					: "border-hk-border-soft hover:border-hk-primary/40 dark:border-[var(--page-cert-border-on-dark)]"
			}`}
		>
			{/* The real checkbox is `sr-only`, so keyboard focus has to be drawn on
			    the 40×40 box that stands in for it — otherwise tabbing through the
			    認證 list moves an invisible cursor. */}
			<label className="relative flex items-center gap-3 cursor-pointer has-[:disabled]:cursor-not-allowed has-[:disabled]:opacity-60">
				<input
					type="checkbox"
					checked={open}
					onChange={handleToggle}
					disabled={busy || (!editable && !view.checked)}
					// 徽章本身是視覺元素；把「未通過 + 原因」那一句掛成勾選框的說明，
					// 讀螢幕的人才拿得到它——否則畫面上有原因、朗讀出來只有類型名。
					aria-describedby={view.badge?.description ? statusId : undefined}
					className="peer sr-only"
				/>
				<span
					className={`relative grid place-items-center w-10 h-10 rounded-xl shrink-0 transition-colors duration-200 motion-reduce:transition-none peer-focus-visible:outline-2 peer-focus-visible:outline-offset-2 peer-focus-visible:outline-[var(--hkui-focus)] ${open ? "bg-hk-brand-primary/20 dark:bg-[var(--page-cert-accent-on-dark)]/28" : "bg-hk-surface-muted dark:bg-white/6"}`}
					aria-hidden="true"
				>
					{/* Decorative: the box is aria-hidden, so a name here is unreachable.
					    The label text next to it is what names the checkbox. */}
					<AppIcon name="shield" size={26} />
				</span>
				<div className="flex flex-col gap-[2px] flex-1 min-w-0">
					<strong
						className={`flex items-center gap-2 text-sm font-bold ${open ? "text-hk-primary dark:text-[var(--page-cert-title-on-dark)]" : "text-hk-heading"}`}
					>
						{item.label}
						<ProductCertificationStatusBadge view={view.badge} />
					</strong>
					<small className="text-xs text-hk-muted">{item.desc}</small>
				</div>
			</label>

			{view.badge?.description ? (
				<p
					id={statusId}
					className="m-0 pl-[52px] max-[420px]:pl-0 text-xs text-hk-muted"
				>
					{view.badge.description}
				</p>
			) : null}

			{view.publicCertNo ? (
				<p className="m-0 pl-[52px] max-[420px]:pl-0 text-xs text-hk-muted">
					{`買家目前看到的仍是舊證書編號 ${view.publicCertNo}，新號碼通過後才會替換。`}
				</p>
			) : null}

			{open && (
				<div className="flex flex-col gap-1.5 pl-[52px] max-[420px]:pl-0">
					<div className="flex items-end gap-2">
						<Input
							variant="bordered"
							size="sm"
							label="證書編號"
							value={certNo}
							readOnly={!editable}
							disabled={busy}
							aria-required={editableDraft || undefined}
							onChange={editable ? handleChange : undefined}
							isInvalid={Boolean(error)}
							aria-describedby={error ? `${noteId} ${errorId}` : noteId}
						/>
						{review ? (
							<Button
								type="button"
								variant="bordered"
								size="sm"
								disabled={!canSubmit}
								onClick={handleSubmit}
								aria-describedby={noteId}
							>
								申請
							</Button>
						) : null}
					</div>
					<small id={noteId} className="text-xs text-hk-muted">
						{review
							? "送出後由平台人工審核，通過才會顯示在商品頁；換號碼請直接改寫後再送一次。"
							: editableDraft
								? "商品建立成功後會自動送出認證申請；審核通過才會顯示標章。"
								: "商品建立後，可在商品編輯頁送出認證申請。"}
					</small>
					{error ? (
						<small
							id={errorId}
							aria-live="polite"
							className="text-xs text-hk-danger dark:text-[var(--page-cert-error-on-dark)]"
						>
							{error}
						</small>
					) : null}
				</div>
			)}
		</div>
	);
}

export default ProductCertificationRow;
