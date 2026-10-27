"use client";

/**
 * Single-page product create/edit form, ported from the seller App's
 * multi-page wizard onto a single Web surface (web migration plan §6.2
 * Flow C). State, validation, AI patch merging, and submit live in
 * `useProductForm`; this file is the JSX composition.
 *
 * Layout: a Shopify-style two-column grid — the left main column holds the
 * content the seller crafts (基本資訊含產地 / 圖片 / 規格); the right rail keeps
 * secondary trust and advanced settings in compact cards.
 */

import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import { BackLink } from "@/components/shared/BackLink";
import Button from "@/components/ui/Button";
import AiAssistantPanel, {
	AI_LISTING_ASSISTANT_ENABLED,
} from "@/features/seller/components/AiAssistantPanel";
import ShelfStatusControl from "../components/ShelfStatusControl";
import { resolveStatusButtons } from "./adapters/productFormAdapter";
import ProductCreationCertifications from "./components/ProductCreationCertifications";
import ProductFormSections, {
	LOCKED_SURFACE_CLASS,
	LockedOverlay,
} from "./components/ProductFormSections";
import { useProductForm } from "./hooks/useProductForm";

function SparkleIcon() {
	return <AppIcon name="sparkle" size={16} />;
}

interface Props {
	mode: "create" | "edit";
	productId?: number | string;
}

/**
 * Mobile wizard steps. Desktop ignores these (the two-column form shows every
 * section at once); on ≤960px the `.product-form-wizard[data-wizard-step]`
 * CSS hides all `[data-step]` sections except the active one. The numbers map
 * to the mobile DOM stacking order so each step's sections stay contiguous.
 */
const WIZARD_STEPS = [
	{ step: 1, label: "基本" },
	{ step: 2, label: "規格" },
	{ step: 3, label: "認證" },
] as const;
const LAST_STEP = WIZARD_STEPS.length;

export default function ProductFormPage({ mode, productId }: Props) {
	const form = useProductForm({ mode, productId });
	const [aiOpen, setAiOpen] = React.useState(false);
	const [step, setStep] = React.useState(1);
	const [pendingSubmitAction, setPendingSubmitAction] = React.useState<
		Parameters<typeof form.submit>[0] | null
	>(null);

	// Jump to a step and reset scroll so the new step starts at the top
	// (mobile only — desktop never changes `step`).
	const goToStep = (next: number) => {
		setStep(Math.min(LAST_STEP, Math.max(1, next)));
		if (typeof window !== "undefined") window.scrollTo({ top: 0 });
	};

	if (form.createdProduct) return <ProductCreationCertifications form={form} />;

	if (form.loading) {
		// Skeleton silhouette mirroring the real form (header + stacked section
		// cards) so the loading state fills the column instead of collapsing
		// into a single line. Matches the coupon form's loading rhythm.
		return (
			<div
				className="mx-auto w-full max-w-[1200px] px-5 pt-6 pb-[calc(120px+var(--hkui-space-6))]"
				aria-busy="true"
				role="status"
			>
				<header className="mb-5 flex flex-col gap-[6px]">
					<span className="block h-[12px] w-[15%] bg-[var(--hkui-default-200)] rounded-[var(--hkui-radius-sm)] animate-pulse motion-reduce:animate-none motion-reduce:opacity-70 dark:bg-[rgba(245,233,195,0.12)]" />
					<span
						className="block bg-[var(--hkui-default-200)] rounded-[var(--hkui-radius-sm)] animate-pulse motion-reduce:animate-none motion-reduce:opacity-70 dark:bg-[rgba(245,233,195,0.12)]"
						style={{ height: 28, width: "30%" }}
					/>
					<span className="block h-[12px] w-[60%] bg-[var(--hkui-default-200)] rounded-[var(--hkui-radius-sm)] animate-pulse motion-reduce:animate-none motion-reduce:opacity-70 dark:bg-[rgba(245,233,195,0.12)]" />
				</header>
				<div className="flex flex-col gap-5">
					{[1, 2, 3].map((i) => (
						<section
							key={i}
							className="bg-hk-surface-card border border-hk-border-soft rounded-[24px] p-6 flex flex-col gap-3 dark:bg-white/4 dark:border-[#f5dea5]/14"
						>
							<div className="flex flex-col gap-1">
								<span className="block h-[12px] w-[35%] bg-[var(--hkui-default-200)] rounded-[var(--hkui-radius-sm)] animate-pulse motion-reduce:animate-none motion-reduce:opacity-70 dark:bg-[rgba(245,233,195,0.12)]" />
								<span className="block h-[12px] w-[80%] bg-[var(--hkui-default-200)] rounded-[var(--hkui-radius-sm)] animate-pulse motion-reduce:animate-none motion-reduce:opacity-70 dark:bg-[rgba(245,233,195,0.12)]" />
							</div>
							<div className="h-12 rounded-md bg-hk-surface-muted animate-pulse dark:bg-[#f5e9c3]/12" />
							<div className="h-12 rounded-md bg-hk-surface-muted animate-pulse dark:bg-[#f5e9c3]/12" />
						</section>
					))}
				</div>
				<span className="visually-hidden">表單載入中</span>
			</div>
		);
	}

	const { primary, secondary } = resolveStatusButtons(
		mode,
		Number(form.draft.status ?? 0),
	);
	// 上架中(=1)：內容不可改，但不再擋整頁；改以頂部可切換狀態 + 表單毛玻璃鎖住呈現。
	const locked = mode === "edit" && Number(form.draft.status ?? 0) === 1;
	// 編輯草稿／下架商品時，桌機可並排顯示「只儲存」與「儲存並上架」；手機底列
	// 空間不足，因此延續桌機的視覺優先順序：實心的上架操作直接顯示，另一個動作
	// 收入「其他儲存方式」。兩個 action 仍沿用 adapter 回傳值，不在 UI 猜狀態。
	const mobileProminentAction = secondary ?? primary;
	const mobileAlternateAction = secondary ? primary : null;
	const submitFromButton = async (
		action: Parameters<typeof form.submit>[0],
	) => {
		if (form.submitting) return;
		setPendingSubmitAction(action);
		try {
			const succeeded = await form.submit(action);
			// 成功後保留 loading，直到導頁卸載；驗證未過或失敗才恢復操作。
			if (!succeeded) setPendingSubmitAction(null);
		} catch (error) {
			setPendingSubmitAction(null);
			throw error;
		}
	};

	// 頂部狀態切換：下架就地重整轉可編輯；上架等同「儲存並上架」（存目前欄位後上架）。
	const handleShelfToggle = (target: "4003" | "4004") => {
		if (target === "4003") {
			void form.unshelf();
		} else {
			void form.submit("publish");
		}
	};

	return (
		<div className="product-form-page mx-auto w-full max-w-[1200px] px-5 pt-6 pb-[calc(120px+var(--hkui-space-6))]">
			<header className="mb-5 flex flex-col gap-[6px]">
				<BackLink
					fallbackHref="/seller-center/products"
					className="inline-flex items-center gap-1 w-fit text-sm text-hk-muted hover:text-hk-primary no-underline py-1 transition-colors duration-150"
					aria-label="返回"
				>
					<AppIcon name="arrow-left" size={14} />
					返回
				</BackLink>
				<div className="product-form__title-row flex items-center gap-3">
					<h1 className="product-form__title font-sans text-3xl font-bold tracking-tight text-hk-heading m-0">
						{mode === "edit" ? "編輯商品" : "新增商品"}
					</h1>
					{mode === "edit" ? (
						// 一律可點：上架中按一下即下架（解鎖表單），下架/草稿按一下即上架。
						<ShelfStatusControl
							status={form.draft.status}
							className="product-form__status-control"
							busy={form.submitting && pendingSubmitAction === null}
							onToggle={
								form.submitting && pendingSubmitAction !== null
									? undefined
									: handleShelfToggle
							}
						/>
					) : (
						<span className="product-form__status-control text-xs font-semibold px-2.5 py-1 rounded-full bg-hk-surface-muted text-hk-muted dark:bg-white/8">
							草稿
						</span>
					)}
					{AI_LISTING_ASSISTANT_ENABLED && (
						<button
							type="button"
							className="product-form__ai-btn"
							aria-label="開啟 AI 上架助手"
							onClick={() => setAiOpen(true)}
						>
							<SparkleIcon />
							<span className="product-form__ai-label">AI 助手</span>
						</button>
					)}
				</div>
				<p className="m-0 text-sm text-hk-muted leading-relaxed max-w-[640px]">
					左欄填寫商品內容與產地，右欄保留認證與進階設定。必填欄位未填會在儲存時提醒。
				</p>
			</header>

			{AI_LISTING_ASSISTANT_ENABLED && (
				<AiAssistantPanel
					open={aiOpen}
					onClose={() => setAiOpen(false)}
					onPatch={form.handleAiPatch}
				/>
			)}

			{/* Mobile-only step indicator. Hidden ≥961px where the full form shows. */}
			<ol className="min-[961px]:hidden flex items-center gap-1.5 mb-4 list-none p-0">
				{WIZARD_STEPS.map((s) => {
					const active = s.step === step;
					const done = s.step < step;
					return (
						<li key={s.step} className="flex-1">
							<button
								type="button"
								onClick={() => goToStep(s.step)}
								className={`w-full flex flex-col items-center gap-1 py-1 ${active ? "text-hk-primary" : done ? "text-hk-heading" : "text-hk-muted"}`}
							>
								<span
									className={`grid place-items-center w-6 h-6 rounded-full text-xs font-bold ${active ? "bg-hk-primary text-white" : done ? "bg-hk-primary/15 text-hk-accent-on-tint" : "bg-hk-surface-muted text-hk-muted dark:bg-white/8"}`}
								>
									{s.step}
								</span>
								<span className="text-[0.6875rem] font-medium leading-none">
									{s.label}
								</span>
							</button>
						</li>
					);
				})}
			</ol>

			<div className="relative">
				<form
					className={`product-form-wizard grid grid-cols-[minmax(0,1fr)_336px] gap-5 items-start max-[960px]:grid-cols-1${
						locked ? ` ${LOCKED_SURFACE_CLASS}` : ""
					}`}
					data-wizard-step={step}
					onSubmit={(e) => {
						e.preventDefault();
						if (locked) return;
						void form.submit(primary.action);
					}}
				>
					<ProductFormSections variant="page" form={form} />
				</form>
				{locked && <LockedOverlay />}
			</div>

			{/* Desktop toolbar — full action set, hidden on the mobile wizard. */}
			<div
				className="max-[960px]:hidden fixed left-0 right-0 bottom-0 bg-hk-surface-card border-t border-hk-border-soft shadow-hk-lift px-5 py-3 flex gap-4 justify-between items-center z-[var(--z-sticky)] dark:bg-white/4 dark:border-[#f5dea5]/14"
				role="toolbar"
			>
				<div className="flex flex-col gap-[2px] min-w-0">
					<strong className="text-base font-semibold text-hk-heading whitespace-nowrap overflow-hidden text-ellipsis">
						{form.draft.name || "未命名商品"}
					</strong>
					<span className="text-xs text-hk-muted">
						{form.draft.skus?.length ?? 0} 規格 ·{" "}
						{mode === "create"
							? form.certificationDrafts.length
							: (form.draft.certifications?.length ?? 0)}{" "}
						認證{mode === "create" ? "待送出" : ""}
					</span>
				</div>
				<div className="flex gap-2 flex-wrap justify-end">
					<Button
						href="/seller-center/products"
						variant="bordered"
						color="default"
						disabled={form.submitting}
					>
						{locked ? "返回" : "取消"}
					</Button>
					{/* 上架中內容鎖住，儲存鈕無意義 — 隱藏，改由頂部狀態切換下架解鎖。 */}
					{!locked && (
						<>
							<Button
								variant={primary.action === "publish" ? undefined : "bordered"}
								disabled={form.submitting}
								isLoading={
									form.submitting && pendingSubmitAction === primary.action
								}
								loadingLabel={`${primary.label}中…`}
								onClick={() => void submitFromButton(primary.action)}
							>
								{primary.label}
							</Button>
							{secondary && (
								<Button
									disabled={form.submitting}
									isLoading={
										form.submitting && pendingSubmitAction === secondary.action
									}
									loadingLabel={`${secondary.label}中…`}
									onClick={() => void submitFromButton(secondary.action)}
								>
									{secondary.label}
								</Button>
							)}
						</>
					)}
				</div>
			</div>

			{/* Mobile wizard step nav — prev/next, final step exposes submit. */}
			<div
				className="product-form__mobile-toolbar min-[961px]:hidden fixed left-0 right-0 bottom-0 z-[var(--z-sticky)] bg-hk-surface-card border-t border-hk-border-soft shadow-hk-lift px-4 py-3 flex items-center gap-3 dark:bg-white/4 dark:border-[#f5dea5]/14"
				role="toolbar"
				aria-label="商品表單步驟與儲存操作"
			>
				{step > 1 ? (
					<Button
						variant="bordered"
						color="default"
						disabled={form.submitting}
						onClick={() => goToStep(step - 1)}
					>
						上一步
					</Button>
				) : (
					<Button
						href="/seller-center/products"
						variant="bordered"
						color="default"
						disabled={form.submitting}
					>
						取消
					</Button>
				)}
				{step < LAST_STEP || locked ? (
					<span className="product-form__mobile-step-count flex-1 text-center text-xs text-hk-muted">
						步驟 {step} / {LAST_STEP}
					</span>
				) : null}
				{step < LAST_STEP ? (
					<Button disabled={form.submitting} onClick={() => goToStep(step + 1)}>
						下一步
					</Button>
				) : locked ? null : (
					<div className="product-form__mobile-submit-actions">
						<Button
							variant={
								mobileProminentAction.action === "publish"
									? undefined
									: "bordered"
							}
							disabled={form.submitting}
							isLoading={
								form.submitting &&
								pendingSubmitAction === mobileProminentAction.action
							}
							loadingLabel={`${mobileProminentAction.label}中…`}
							onClick={() =>
								void submitFromButton(mobileProminentAction.action)
							}
						>
							{mobileProminentAction.label}
						</Button>
						{mobileAlternateAction && (
							<details className="product-form__mobile-more">
								<summary
									aria-label={`其他儲存方式：${mobileAlternateAction.label}`}
								>
									<AppIcon name="more" size={20} />
									<span className="sr-only">其他儲存方式</span>
								</summary>
								<div className="product-form__mobile-more-menu">
									<Button
										variant={
											mobileAlternateAction.action === "publish"
												? undefined
												: "bordered"
										}
										fullWidth
										disabled={form.submitting}
										isLoading={
											form.submitting &&
											pendingSubmitAction === mobileAlternateAction.action
										}
										loadingLabel={`${mobileAlternateAction.label}中…`}
										onClick={() =>
											void submitFromButton(mobileAlternateAction.action)
										}
									>
										{mobileAlternateAction.label}
									</Button>
								</div>
							</details>
						)}
					</div>
				)}
			</div>
		</div>
	);
}
