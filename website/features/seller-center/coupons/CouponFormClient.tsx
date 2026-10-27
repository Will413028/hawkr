"use client";

import * as React from "react";
import AppIcon, { type AppIconName } from "@/components/shared/AppIcon";
import { BackLink } from "@/components/shared/BackLink";
import { DatePicker } from "@/components/ui";
import Button from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { useHydrationSafeClientRender } from "@/lib/hooks/useHydrationSafeClientRender";
import { useResettableError } from "@/lib/hooks/useResettableError";
import { useObservedRouter } from "@/lib/navigation/use-observed-router";
import {
	createSellerCoupon,
	fetchSellerCoupon,
	type SellerCoupon,
	type SellerCouponKind,
	type SellerCouponTarget,
	type SellerCustomer,
} from "@/lib/seller/repo";
import CouponBuyerPicker from "./CouponBuyerPicker";
import CouponRulesButton from "./CouponRulesButton";

interface Props {
	mode: "create" | "edit";
	couponId?: string;
	/** When set (create mode), the form opens pre-targeted at this one buyer —
	 * used by the customer-detail「發優惠券」CTA so a seller can issue a
	 * compensation / VIP coupon to a specific customer in one tap. */
	presetBuyer?: { id: string; name?: string };
}

// Coupon timestamps wire as Unix seconds; DatePicker speaks ISO YYYY-MM-DD.
// Read the calendar date with UTC getters to mirror isoToUnix (which parses
// date-only ISO as UTC midnight) and the toCouponDateTime sender (which anchors
// to ...00:00:00Z / ...23:59:59Z). Local getters would shove a stored
// 2026-05-31T23:59:59Z end date to 2026-06-01 in UTC+8.
function unixToIso(ts?: number): string {
	if (!ts) return "";
	const d = new Date(ts * 1000);
	if (Number.isNaN(d.getTime())) return "";
	return `${d.getUTCFullYear()}-${String(d.getUTCMonth() + 1).padStart(2, "0")}-${String(d.getUTCDate()).padStart(2, "0")}`;
}
function isoToUnix(iso: string): number | undefined {
	if (!iso) return undefined;
	const ms = new Date(iso).getTime();
	return Number.isFinite(ms) ? Math.floor(ms / 1000) : undefined;
}

interface KindOption {
	value: SellerCouponKind;
	title: string;
	desc: string;
}
const KIND_OPTIONS: KindOption[] = [
	{
		value: "amount",
		title: "現金折抵",
		desc: "結帳時直接折抵指定金額",
	},
	{
		value: "percent",
		title: "折扣百分比",
		desc: "按訂單金額打折，可設折抵上限",
	},
];

/** Icon paired to each coupon `type`; sized 32px to slot into the 44×44 icon
 * box. */
const KIND_ICON: Record<SellerCouponKind, AppIconName> = {
	amount: "coin",
	percent: "discount",
};

interface TargetOption {
	value: SellerCouponTarget;
	title: string;
	desc: string;
}
const TARGET_OPTIONS: TargetOption[] = [
	{
		value: "all",
		title: "全體買家",
		desc: "所有在 Hawkr 的買家都能領取",
	},
	{
		value: "members",
		title: "賣家會員",
		desc: "已綁定本店會員卡的買家才能領取",
	},
	{
		value: "specific",
		title: "指定買家 / 分群",
		desc: "依 VIP、CRM tag 或客戶清單挑選",
	},
];

/** Icon paired to each `targetType`; sized 32px to slot into the existing
 * `.coupon-form__type-amount` 40×40 box. */
const TARGET_ICON: Record<SellerCouponTarget, AppIconName> = {
	all: "notification",
	members: "people",
	specific: "user",
};

export default function CouponFormClient({
	mode,
	couponId,
	presetBuyer,
}: Props) {
	const router = useObservedRouter("seller_coupon_form");
	const hydrationSafeClientRender = useHydrationSafeClientRender();
	const [draft, setDraft] = React.useState<Partial<SellerCoupon>>(() => ({
		name: "",
		type: "amount",
		value: 50,
		min_spend: 500,
		usage_limit: 100,
		// A preset buyer (from the CRM「發優惠券」CTA) lands the form on the
		// 指定買家 path with that customer already selected.
		targetType: presetBuyer ? "specific" : "all",
		buyerAccountIds: presetBuyer ? [presetBuyer.id] : undefined,
	}));
	const [loading, setLoading] = React.useState(mode === "edit");
	const [submitting, setSubmitting] = React.useState(false);
	// mode + couponId 是優惠券表單的 session boundary:從新增切到編輯某張、
	// 或從一張編輯切到另一張時,前次錯誤訊息不該殘留到新的表單 session。
	const [error, setError] = useResettableError(`${mode}:${couponId ?? ""}`);
	// Cache buyer id → display name so chips can render names without
	// re-fetching the customer list every render. Populated by the picker
	// on confirm; on edit mode it starts empty and falls back to "買家 #id".
	const [buyerNames, setBuyerNames] = React.useState<Map<string, string>>(() =>
		presetBuyer?.name
			? new Map([[presetBuyer.id, presetBuyer.name]])
			: new Map(),
	);
	const [pickerOpen, setPickerOpen] = React.useState(false);

	React.useEffect(() => {
		if (mode !== "edit" || !couponId) return;
		void fetchSellerCoupon(couponId)
			.then((c) => {
				if (!c) {
					setError("找不到此優惠券，可能已被刪除或已失效。");
					return;
				}
				setDraft(c);
				// Seed the chip name cache from the targeted accounts so a
				// 指定買家 coupon shows real nicknames instead of "買家 #id".
				if (c.targetAccounts?.length) {
					setBuyerNames((prev) => {
						const next = new Map(prev);
						for (const a of c.targetAccounts ?? []) {
							if (a.buyerId)
								next.set(String(a.buyerId), a.nickname ?? `買家 #${a.buyerId}`);
						}
						return next;
					});
				}
			})
			.catch((e) => {
				setError(e instanceof Error ? e.message : "優惠券載入失敗");
			})
			.finally(() => setLoading(false));
	}, [mode, couponId]);

	// Coupons are immutable once created — the backend exposes no seller-side
	// update endpoint, only revoke + recreate. So "edit" mode is a read-only
	// 檢視 view: every control is populated but locked, and there is no save.
	const readOnly = mode === "edit";

	const update = (patch: Partial<SellerCoupon>) =>
		setDraft((prev) => ({ ...prev, ...patch }));

	const kind = (draft.type as SellerCouponKind) || "amount";
	const target = (draft.targetType as SellerCouponTarget) || "all";
	const isPercent = kind === "percent";
	const valueLabel = isPercent ? "折扣百分比" : "折抵金額";
	const valueSuffix = isPercent ? "%" : "NT$";
	const valueHint = isPercent
		? "輸入 1–99 的整數，例如 9 代表 91% 折"
		: "直接折抵的新台幣金額";

	// 前端先擋：對齊 backend 優惠券微服務 §5.1 SellerCreateTemplate 參數驗證，
	// 讓賣家在送出前就看到問題，而非等後端回 400。後端仍為最終把關（同一份規則）。
	const validateDraft = (): string | null => {
		if (!(draft.name ?? "").trim()) return "請輸入優惠券名稱。";

		const minSpend = Number(draft.min_spend ?? 0);
		if (!Number.isInteger(minSpend) || minSpend < 0)
			return "最低消費門檻需為 0 或正整數。";

		const value = Number(draft.value);
		if (isPercent) {
			if (!Number.isInteger(value) || value < 1 || value > 99)
				return "折扣百分比需為 1–99 的整數。";
			const cap = Number(draft.maxDiscountAmount);
			if (!Number.isInteger(cap) || cap <= 0)
				return "折扣百分比券必須設定大於 0 的單次折抵上限。";
		} else if (!Number.isInteger(value) || value <= 0) {
			return "折抵金額需為大於 0 的整數。";
		}

		if (!draft.starts_at) return "請選擇開始時間。";
		if (!draft.expires_at) return "請選擇到期時間。";
		if (draft.starts_at > draft.expires_at) return "開始時間不可晚於到期時間。";

		if (target === "specific" && (draft.buyerAccountIds?.length ?? 0) === 0)
			return "指定買家需至少選擇 1 位。";

		return null;
	};

	const handleSubmit = async (e: React.FormEvent) => {
		e.preventDefault();
		if (readOnly) return;
		const validationError = validateDraft();
		if (validationError) {
			setError(validationError);
			return;
		}
		setSubmitting(true);
		setError(null);
		try {
			// Only create reaches here — "edit" mode is a read-only 檢視 view
			// (no backend update endpoint) and bails out above.
			await createSellerCoupon({
				name: draft.name || "未命名優惠券",
				type: kind,
				value: Number(draft.value) || 0,
				maxDiscountAmount:
					kind === "percent" && draft.maxDiscountAmount
						? Number(draft.maxDiscountAmount)
						: undefined,
				min_spend: draft.min_spend ? Number(draft.min_spend) : undefined,
				usage_limit: draft.usage_limit ? Number(draft.usage_limit) : undefined,
				starts_at: draft.starts_at,
				expires_at: draft.expires_at,
				targetType: target,
				buyerAccountIds:
					target === "specific" ? draft.buyerAccountIds : undefined,
				status: "active",
			});
			router.push("/seller-center/coupons");
		} catch (e2) {
			setError(e2 instanceof Error ? e2.message : "儲存失敗");
			setSubmitting(false);
		}
	};

	if (!hydrationSafeClientRender || loading) {
		// 4-section skeleton that mirrors the real form's silhouette
		// (header + 基本資訊 / 折抵設定 / 發行對象 / 發行設定) so the
		// loading state visually fills the whole content column instead
		// of collapsing into a single small card at the top.
		return (
			<div className="flex flex-col gap-5 pb-12" aria-busy="true" role="status">
				<header className="flex flex-col gap-[6px]">
					<span className="block h-[12px] w-[25%] bg-[var(--hkui-default-200)] rounded-[var(--hkui-radius-sm)] animate-pulse motion-reduce:animate-none motion-reduce:opacity-70 dark:bg-[rgba(245,233,195,0.12)]" />
					<span
						className="block bg-[var(--hkui-default-200)] rounded-[var(--hkui-radius-sm)] animate-pulse motion-reduce:animate-none motion-reduce:opacity-70 dark:bg-[rgba(245,233,195,0.12)]"
						style={{ height: 28, width: "40%" }}
					/>
					<span className="block h-[12px] w-[90%] bg-[var(--hkui-default-200)] rounded-[var(--hkui-radius-sm)] animate-pulse motion-reduce:animate-none motion-reduce:opacity-70 dark:bg-[rgba(245,233,195,0.12)]" />
				</header>
				{[1, 2, 3, 4].map((i) => (
					<section
						key={i}
						className="bg-hk-surface-card border border-hk-border-soft rounded-[24px] p-6 flex flex-col gap-3 dark:bg-white/4 dark:border-[#f5dea5]/14"
					>
						<div className="flex flex-col gap-1">
							<span className="block h-[12px] w-[40%] bg-[var(--hkui-default-200)] rounded-[var(--hkui-radius-sm)] animate-pulse motion-reduce:animate-none motion-reduce:opacity-70 dark:bg-[rgba(245,233,195,0.12)]" />
							<span className="block h-[12px] w-[90%] bg-[var(--hkui-default-200)] rounded-[var(--hkui-radius-sm)] animate-pulse motion-reduce:animate-none motion-reduce:opacity-70 dark:bg-[rgba(245,233,195,0.12)]" />
						</div>
						<div className="h-12 rounded-md bg-hk-surface-muted animate-pulse dark:bg-[#f5e9c3]/12" />
						<div className="h-12 rounded-md bg-hk-surface-muted animate-pulse dark:bg-[#f5e9c3]/12" />
					</section>
				))}
				<span className="visually-hidden">表單載入中</span>
			</div>
		);
	}

	return (
		<div className="flex flex-col gap-5 pb-12">
			<header className="flex flex-col gap-[6px]">
				<BackLink
					fallbackHref="/seller-center/coupons"
					className="inline-flex items-center gap-1 w-fit text-sm text-hk-muted hover:text-hk-primary no-underline py-1 transition-colors duration-150"
					aria-label="返回"
				>
					<AppIcon name="arrow-left" size={14} />
					返回
				</BackLink>
				<h1 className="font-sans text-3xl font-bold tracking-tight text-hk-heading m-0">
					{readOnly ? "檢視優惠券" : "建立優惠券"}
				</h1>
				<p className="m-0 text-sm text-hk-muted leading-relaxed max-w-[640px]">
					{readOnly
						? "優惠券發行後內容即固定，無法修改。如需調整，請先收回這張優惠券再重新建立。"
						: "對齊 APP 端「建立優惠券」流程：選類型、設折抵與門檻、決定發行對象與有效期。結帳時系統會自動套用對買家最優惠的組合。"}
				</p>
			</header>

			<form className="flex flex-col gap-5" onSubmit={handleSubmit}>
				<section className="bg-hk-surface-card border border-hk-border-soft rounded-[24px] p-6 flex flex-col gap-4 dark:bg-white/4 dark:border-[#f5dea5]/14">
					<header className="flex flex-col gap-1">
						<h2 className="text-lg font-bold m-0 tracking-tight text-hk-heading">
							1 · 基本資訊
						</h2>
						<p className="m-0 text-sm text-hk-muted leading-relaxed">
							買家會在優惠券列表看到的名稱與折扣方式。
						</p>
					</header>

					<Input
						floatingLabel
						variant="bordered"
						size="lg"
						label="優惠券名稱"
						isRequired
						maxLength={40}
						disabled={readOnly}
						value={draft.name ?? ""}
						onChange={(e) => update({ name: e.target.value })}
						description={`${(draft.name ?? "").length} / 40 字 · 例：5月買蕃茄`}
					/>

					<fieldset className="flex flex-col gap-1.5 w-full border-0 p-0 m-0 min-w-0">
						<legend
							className="text-sm font-medium text-foreground"
							data-slot="label"
						>
							優惠類型
						</legend>
						<div
							className={`grid grid-cols-3 max-[720px]:grid-cols-2 max-[560px]:grid-cols-1 gap-3 mt-1.5 ${readOnly ? "pointer-events-none" : ""}`}
						>
							{KIND_OPTIONS.map((opt) => {
								const active = kind === opt.value;
								return (
									<label
										key={opt.value}
										className={`relative flex items-center gap-3 py-[14px] px-4 bg-hk-surface-card border-[1.5px] border-hk-border-soft rounded-lg text-left cursor-pointer overflow-hidden text-hk-body isolate transition-all duration-200 ease-in-out hover:border-hk-primary/40 hover:-translate-y-px hover:shadow-[0_6px_18px_-10px_rgb(var(--color-brand-primary-rgb)/0.24)] before:content-[''] before:absolute before:inset-[-1px] before:z-[-1] before:rounded-[inherit] before:bg-[radial-gradient(120%_90%_at_0%_0%,rgb(var(--color-ink-600-rgb)/0.16),transparent_60%)] before:opacity-0 before:transition-opacity before:duration-250 hover:before:opacity-60 dark:bg-white/4 dark:border-[#f5dea5]/18 dark:hover:border-hk-ink-200/55 dark:hover:shadow-[0_8px_22px_-10px_rgba(0,0,0,0.6)] dark:before:bg-[radial-gradient(120%_90%_at_0%_0%,rgb(var(--color-ink-200-rgb)/0.18),transparent_60%)] ${
											active
												? "border-hk-primary bg-[linear-gradient(135deg,rgb(var(--color-ink-600-rgb)/0.08),var(--theme-surface-card)_70%)] shadow-[0_0_0_3px_rgb(var(--color-ink-600-rgb)/0.12),0_8px_22px_-12px_rgb(var(--color-brand-primary-rgb)/0.28)] before:opacity-100 dark:bg-[linear-gradient(135deg,rgb(var(--color-ink-300-rgb)/0.22),rgba(255,255,255,0.05)_70%)] dark:border-hk-ink-200 dark:shadow-[0_0_0_3px_rgb(var(--color-ink-300-rgb)/0.22),0_10px_26px_-12px_rgba(0,0,0,0.6)]"
												: ""
										}`}
									>
										<input
											type="radio"
											name="coupon-type"
											value={opt.value}
											checked={active}
											disabled={readOnly}
											onChange={() => update({ type: opt.value })}
											className="peer sr-only"
										/>
										<span
											className={`relative grid place-items-center w-11 h-11 rounded-xl bg-hk-brand-primary/10 text-hk-accent-on-tint font-bold text-lg shrink-0 transition-all duration-250 peer-focus-visible:outline peer-focus-visible:outline-2 peer-focus-visible:outline-hk-primary peer-focus-visible:outline-offset-2 dark:bg-hk-ink-200/14 dark:text-hk-ink-100 dark:peer-focus-visible:outline-hk-ink-200 ${
												active
													? "bg-hk-brand-primary/20 shadow-[inset_0_0_0_1.5px_var(--color-brand-primary-strong),0_6px_16px_-8px_rgb(var(--color-ink-600-rgb)/0.4)] dark:bg-hk-ink-200/28 dark:shadow-[inset_0_0_0_1.5px_var(--color-ink-200),0_6px_16px_-8px_rgb(var(--color-ink-200-rgb)/0.4)]"
													: ""
											}`}
											aria-hidden="true"
										>
											<AppIcon name={KIND_ICON[opt.value]} size={32} />
										</span>
										<span className="flex flex-col gap-[3px] min-w-0">
											<strong
												className={`text-sm font-bold ${active ? "text-hk-primary dark:text-hk-ink-100" : "text-hk-heading"}`}
											>
												{opt.title}
											</strong>
											<small className="text-xs text-hk-muted leading-normal">
												{opt.desc}
											</small>
										</span>
										{active && (
											<span className="absolute top-2 right-2.5 w-4.5 h-4.5 grid place-items-center rounded-full bg-hk-primary text-white text-[0.6875rem] font-extrabold leading-none dark:bg-hk-ink-200 dark:text-hk-ink-900">
												<AppIcon name="check" size={12} fill />
											</span>
										)}
									</label>
								);
							})}
						</div>
					</fieldset>
				</section>

				<section className="bg-hk-surface-card border border-hk-border-soft rounded-[24px] p-6 flex flex-col gap-4 dark:bg-white/4 dark:border-[#f5dea5]/14">
					<header className="flex flex-col gap-1">
						<div className="flex items-center gap-2">
							<h2 className="text-lg font-bold m-0 tracking-tight text-hk-heading">
								2 · 折抵設定
							</h2>
							<CouponRulesButton />
						</div>
						<p className="m-0 text-sm text-hk-muted leading-relaxed">
							折抵金額與使用門檻，買家滿額時自動可用。
						</p>
					</header>

					<div className="grid grid-cols-2 gap-4 max-[720px]:grid-cols-1">
						<Input
							floatingLabel
							variant="bordered"
							size="lg"
							label={valueLabel}
							isRequired
							type="number"
							min={1}
							max={isPercent ? 99 : undefined}
							disabled={readOnly}
							value={String(draft.value ?? "")}
							onChange={(e) => update({ value: Number(e.target.value) })}
							endContent={
								<span className="text-sm font-semibold text-hk-muted tracking-normal whitespace-nowrap">
									{valueSuffix}
								</span>
							}
							description={valueHint}
						/>

						<Input
							floatingLabel
							variant="bordered"
							size="lg"
							label="最低消費門檻"
							type="number"
							min={0}
							disabled={readOnly}
							value={String(draft.min_spend ?? 0)}
							onChange={(e) => update({ min_spend: Number(e.target.value) })}
							endContent={
								<span className="text-sm font-semibold text-hk-muted tracking-normal whitespace-nowrap">
									NT$
								</span>
							}
							description="訂單金額達到此門檻才能使用，0 代表不限。"
						/>
					</div>

					{isPercent && (
						<Input
							floatingLabel
							variant="bordered"
							size="lg"
							label="單次折抵上限"
							isRequired
							type="number"
							min={1}
							disabled={readOnly}
							value={String(draft.maxDiscountAmount ?? "")}
							onChange={(e) =>
								update({ maxDiscountAmount: Number(e.target.value) })
							}
							endContent={
								<span className="text-sm font-semibold text-hk-muted tracking-normal whitespace-nowrap">
									NT$
								</span>
							}
							description="折扣券需設定折抵上限以避免大額訂單被打 100% 折。"
						/>
					)}
				</section>

				<section className="bg-hk-surface-card border border-hk-border-soft rounded-[24px] p-6 flex flex-col gap-4 dark:bg-white/4 dark:border-[#f5dea5]/14">
					<header className="flex flex-col gap-1">
						<h2 className="text-lg font-bold m-0 tracking-tight text-hk-heading">
							3 · 發行對象
						</h2>
						<p className="m-0 text-sm text-hk-muted leading-relaxed">
							決定誰能領這張優惠券。指定買家可用 VIP、CRM tag
							或客戶清單快速分群。
						</p>
					</header>

					<fieldset className="flex flex-col gap-1.5 w-full border-0 p-0 m-0 min-w-0">
						<legend className="visually-hidden">發行對象</legend>
						<div
							className={`grid grid-cols-3 max-[720px]:grid-cols-2 max-[560px]:grid-cols-1 gap-3 mt-1.5 ${readOnly ? "pointer-events-none" : ""}`}
						>
							{TARGET_OPTIONS.map((opt) => {
								const active = target === opt.value;
								return (
									<label
										key={opt.value}
										className={`relative flex items-center gap-3 py-[14px] px-4 bg-hk-surface-card border-[1.5px] border-hk-border-soft rounded-lg text-left cursor-pointer overflow-hidden text-hk-body isolate transition-all duration-200 ease-in-out hover:border-hk-primary/40 hover:-translate-y-px hover:shadow-[0_6px_18px_-10px_rgb(var(--color-brand-primary-rgb)/0.24)] before:content-[''] before:absolute before:inset-[-1px] before:z-[-1] before:rounded-[inherit] before:bg-[radial-gradient(120%_90%_at_0%_0%,rgb(var(--color-ink-600-rgb)/0.16),transparent_60%)] before:opacity-0 before:transition-opacity before:duration-250 hover:before:opacity-60 dark:bg-white/4 dark:border-[#f5dea5]/18 dark:hover:border-hk-ink-200/55 dark:hover:shadow-[0_8px_22px_-10px_rgba(0,0,0,0.6)] dark:before:bg-[radial-gradient(120%_90%_at_0%_0%,rgb(var(--color-ink-200-rgb)/0.18),transparent_60%)] ${
											active
												? "border-hk-primary bg-[linear-gradient(135deg,rgb(var(--color-ink-600-rgb)/0.08),var(--theme-surface-card)_70%)] shadow-[0_0_0_3px_rgb(var(--color-ink-600-rgb)/0.12),0_8px_22px_-12px_rgb(var(--color-brand-primary-rgb)/0.28)] before:opacity-100 dark:bg-[linear-gradient(135deg,rgb(var(--color-ink-300-rgb)/0.22),rgba(255,255,255,0.05)_70%)] dark:border-hk-ink-200 dark:shadow-[0_0_0_3px_rgb(var(--color-ink-300-rgb)/0.22),0_10px_26px_-12px_rgba(0,0,0,0.6)]"
												: ""
										}`}
									>
										<input
											type="radio"
											name="coupon-target"
											value={opt.value}
											checked={active}
											disabled={readOnly}
											onChange={() => update({ targetType: opt.value })}
											className="peer sr-only"
										/>
										<span
											className={`relative grid place-items-center w-11 h-11 rounded-xl bg-hk-brand-primary/12 text-hk-accent-on-tint font-bold text-lg shrink-0 transition-all duration-250 peer-focus-visible:outline peer-focus-visible:outline-2 peer-focus-visible:outline-hk-primary peer-focus-visible:outline-offset-2 dark:bg-hk-ink-200/16 dark:text-hk-ink-100 dark:peer-focus-visible:outline-hk-ink-200 ${
												active
													? "bg-hk-brand-primary/20 shadow-[inset_0_0_0_1.5px_var(--color-brand-primary-strong),0_6px_16px_-8px_rgb(var(--color-ink-600-rgb)/0.4)] dark:bg-hk-ink-200/28 dark:shadow-[inset_0_0_0_1.5px_var(--color-ink-200),0_6px_16px_-8px_rgb(var(--color-ink-200-rgb)/0.4)]"
													: ""
											}`}
											aria-hidden="true"
										>
											<AppIcon name={TARGET_ICON[opt.value]} size={32} />
										</span>
										<span className="flex flex-col gap-[3px] min-w-0">
											<strong
												className={`text-sm font-bold ${active ? "text-hk-primary dark:text-hk-ink-100" : "text-hk-heading"}`}
											>
												{opt.title}
											</strong>
											<small className="text-xs text-hk-muted leading-normal">
												{opt.desc}
											</small>
										</span>
										{active && (
											<span className="absolute top-2 right-2.5 w-4.5 h-4.5 grid place-items-center rounded-full bg-hk-primary text-white text-[0.6875rem] font-extrabold leading-none dark:bg-hk-ink-200 dark:text-hk-ink-900">
												<AppIcon name="check" size={12} fill />
											</span>
										)}
									</label>
								);
							})}
						</div>
					</fieldset>

					{target === "specific" && (
						<div className="flex flex-col gap-3 py-3 px-4 bg-hk-surface-muted border border-dashed border-hk-border-soft rounded-md dark:bg-white/4 dark:border-[#f5dea5]/20">
							{(draft.buyerAccountIds?.length ?? 0) === 0 ? (
								<p className="m-0 text-sm text-hk-muted">
									{readOnly
										? "此優惠券未指定買家。"
										: "尚未選擇任何買家。需要至少 1 位才能建立。"}
								</p>
							) : (
								<ul className="list-none m-0 p-0 flex flex-wrap gap-1.5">
									{draft.buyerAccountIds?.map((id) => (
										<li
											key={id}
											className="inline-flex items-center gap-1 py-1 pr-1.5 pl-2.5 bg-hk-primary/10 text-hk-accent-on-tint rounded-full text-xs font-semibold dark:bg-hk-ink-300/18 dark:text-hk-ink-200"
										>
											<span>{buyerNames.get(id) ?? `買家 #${id}`}</span>
											{!readOnly && (
												<button
													type="button"
													aria-label="移除位買家"
													className="border-0 bg-transparent text-inherit w-4.5 h-4.5 grid place-items-center rounded-full cursor-pointer leading-none text-[0.875rem] hover:bg-black/8 dark:hover:bg-white/8"
													onClick={() =>
														update({
															buyerAccountIds: (
																draft.buyerAccountIds ?? []
															).filter((x) => x !== id),
														})
													}
												>
													<AppIcon name="close" size={12} />
												</button>
											)}
										</li>
									))}
								</ul>
							)}
							{!readOnly && (
								<Button
									variant="bordered"
									size="sm"
									onClick={() => setPickerOpen(true)}
								>
									{(draft.buyerAccountIds?.length ?? 0) === 0
										? "選擇買家 / 分群"
										: "編輯選單"}
								</Button>
							)}
						</div>
					)}
				</section>

				<section className="bg-hk-surface-card border border-hk-border-soft rounded-[24px] p-6 flex flex-col gap-4 dark:bg-white/4 dark:border-[#f5dea5]/14">
					<header className="flex flex-col gap-1">
						<h2 className="text-lg font-bold m-0 tracking-tight text-hk-heading">
							4 · 發行設定
						</h2>
						<p className="m-0 text-sm text-hk-muted leading-relaxed">
							發行張數上限與有效期間。期間外的優惠券無法使用，APP 端會自動隱藏。
						</p>
					</header>

					<Input
						floatingLabel
						variant="bordered"
						size="lg"
						label="發行張數上限"
						type="number"
						min={1}
						disabled={readOnly}
						value={String(draft.usage_limit ?? "")}
						onChange={(e) => update({ usage_limit: Number(e.target.value) })}
						endContent={
							<span className="text-sm font-semibold text-hk-muted tracking-normal whitespace-nowrap">
								張
							</span>
						}
						description="留空表示不限張數。"
					/>

					<div className="grid grid-cols-2 gap-4 max-[720px]:grid-cols-1">
						<DatePicker
							label="開始時間"
							variant="bordered"
							size="lg"
							disabled={readOnly}
							value={unixToIso(draft.starts_at)}
							onChange={(v) => update({ starts_at: isoToUnix(v) })}
							isRequired
							description="優惠券開始生效的日期。"
						/>
						<DatePicker
							label="到期時間"
							variant="bordered"
							size="lg"
							disabled={readOnly}
							value={unixToIso(draft.expires_at)}
							onChange={(v) => update({ expires_at: isoToUnix(v) })}
							minDate={unixToIso(draft.starts_at) || undefined}
							isRequired
							description="可使用到當日 23:59。"
						/>
					</div>
				</section>

				{error && (
					<p
						className="m-0 py-3 px-4 bg-hk-danger/10 text-hk-danger rounded-md text-sm dark:bg-[rgba(218,110,110,0.16)] dark:text-[#e6a3a3]"
						role="alert"
					>
						{error}
					</p>
				)}

				<div className="flex justify-end gap-3 pt-2">
					<Button
						href="/seller-center/coupons"
						variant="bordered"
						color="default"
					>
						{readOnly ? "返回" : "取消"}
					</Button>
					{!readOnly && (
						<Button
							type="submit"
							disabled={submitting}
							isLoading={submitting}
							loadingLabel="建立並發行優惠券中…"
						>
							建立並發行
						</Button>
					)}
				</div>
			</form>

			<CouponBuyerPicker
				open={pickerOpen}
				initialSelected={draft.buyerAccountIds ?? []}
				onClose={() => setPickerOpen(false)}
				onConfirm={(rows: SellerCustomer[]) => {
					const ids = rows.map((r) => String(r.id));
					setBuyerNames((prev) => {
						const next = new Map(prev);
						for (const r of rows) next.set(String(r.id), r.name);
						return next;
					});
					update({ buyerAccountIds: ids });
					setPickerOpen(false);
				}}
			/>
		</div>
	);
}
