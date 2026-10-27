"use client";

import Image from "next/image";
import * as React from "react";
import AnchorLink from "@/components/shared/AnchorLink";
import AppIcon from "@/components/shared/AppIcon";
import Link from "@/components/shared/Link";
import { Button, Card, Input, Select } from "@/components/ui";
import { trackSellerLead } from "@/lib/analytics/events";
import { track } from "@/lib/analytics/gtag";
import "@/styles/features/seller-pricing.css";
import {
	type SellerLeadPlanId,
	type SellerLeadServiceInterest,
	submitSellerLeadRequest,
} from "./lead-submission";
import {
	PUBLIC_MANAGED_SERVICE_ADD_ON,
	PUBLIC_SELLER_PLANS,
	PUBLIC_SELLER_PRICING_FAQ_CATEGORIES,
	PUBLIC_SELLER_PRICING_FAQS,
	PUBLIC_TAX_NOTE,
	type PublicBillingCycle,
	type PublicPricingFaqCategoryId,
	type PublicSellerPlan,
	type PublicSellerPlanId,
} from "./public-data";
import { useSellerPricingEffects } from "./SellerPricingEffects";

type LeadState = {
	brandName: string;
	contactName: string;
	phone: string;
	email: string;
	productType: string;
};

const EMPTY_LEAD: LeadState = {
	brandName: "",
	contactName: "",
	phone: "",
	email: "",
	productType: "",
};

const PLAN_HIGHLIGHTS: Record<PublicSellerPlanId, string[]> = {
	starter: [
		"標準入駐指引與一次上線前缺漏檢查",
		"平台提供市集、交易、結算與對帳基礎",
	],
	pro: ["團隊自行上架並完整維護商品資料", "平台提供市集、交易、結算與對帳基礎"],
};

const twd = new Intl.NumberFormat("zh-TW");

function formatTwd(amount: number) {
	return `NT$${twd.format(amount)}`;
}

function formatRate(rate: number) {
	return `${Math.round(rate * 1000) / 10}%`;
}

function effectiveMonthlyFee(annualFee: number) {
	return Math.round(annualFee / 12);
}

function currentDevice() {
	if (typeof window === "undefined") return "unknown";
	if (window.innerWidth < 768) return "mobile";
	if (window.innerWidth < 1024) return "tablet";
	return "desktop";
}

function trackPricing(
	eventName: string,
	location: string,
	params: Record<string, unknown> = {},
) {
	track(eventName, {
		location,
		device: currentDevice(),
		...params,
	});
}

function PlanIcon({ plan }: { plan: PublicSellerPlan }) {
	return (
		<span className="sp-plan-icon" aria-hidden="true">
			<AppIcon name={plan.icon} size={25} />
		</span>
	);
}

function PricingCard({
	plan,
	billingCycle,
	selectedPlan,
	onSelectPlan,
}: {
	plan: PublicSellerPlan;
	billingCycle: PublicBillingCycle;
	selectedPlan: SellerLeadPlanId;
	onSelectPlan: (planId: PublicSellerPlanId) => void;
}) {
	const isPro = plan.id === "pro";
	const isAnnual = billingCycle === "annual";
	const fixedFee = isAnnual ? plan.annualFee : plan.monthlyFee;
	const highlights = [
		plan.zeroSalesNote,
		...PLAN_HIGHLIGHTS[plan.id],
		plan.excluded,
	];
	const monthlyBillingNote = isPro
		? `${formatTwd(plan.monthlyFee)}／月，按月保留調整方案的彈性。`
		: "固定方案費仍為 0 元，成交後才計算服務費。";
	const annualBillingNote =
		plan.annualSavings > 0
			? `比月繳 12 個月省 ${formatTwd(plan.annualSavings)}，平均每月 ${formatTwd(effectiveMonthlyFee(plan.annualFee))}`
			: "固定方案費仍為 0 元，成交後才計算服務費。";
	const inputId = `seller-plan-${plan.id}`;
	const isSelected = selectedPlan === plan.id;
	const [planNameLatin, ...planNameLocalParts] = plan.name.split(" ");
	const planNameLocal = planNameLocalParts.join(" ");
	const fixedFeeLabel = formatTwd(fixedFee);
	const transactionFeeLabel = formatRate(plan.transactionFeeRate);
	const inputRef = React.useRef<HTMLInputElement>(null);

	return (
		<>
			<input
				ref={inputRef}
				className="sp-plan-card__radio visually-hidden"
				id={inputId}
				type="radio"
				name="seller-plan"
				value={plan.id}
				checked={isSelected}
				onChange={() => {
					onSelectPlan(plan.id);
					trackPricing("plan_selected", "pricing_card", {
						plan: plan.id,
						billing_cycle: billingCycle,
					});
				}}
			/>
			<Card
				as="article"
				shadow="none"
				radius="lg"
				className="sp-plan-card"
				data-plan={plan.id}
				data-billing={billingCycle}
				data-selected={isSelected ? "true" : "false"}
				data-sp-spotlight="soft"
			>
				{/* biome-ignore lint/a11y/useKeyWithClickEvents: The associated native radio keeps its own keyboard interaction; this click interception only prevents pointer-triggered scrolling. */}
				<label
					className="sp-plan-card__hit-area"
					htmlFor={inputId}
					onClick={(event) => {
						event.preventDefault();
						inputRef.current?.focus({ preventScroll: true });
						if (!isSelected) inputRef.current?.click();
					}}
				>
					<span className="visually-hidden">選擇 {plan.name}</span>
				</label>
				<div className="sp-plan-card__head">
					<div className="sp-plan-card__identity">
						<PlanIcon plan={plan} />
						<div>
							<h3 className="sp-plan-card__title">
								<span className="sp-plan-card__title-latin">
									{planNameLatin}
								</span>
								{planNameLocal ? (
									<>
										{" "}
										<span className="sp-plan-card__title-local">
											{planNameLocal}
										</span>
									</>
								) : null}
							</h3>
							<p>{plan.subtitle}</p>
						</div>
					</div>
					<p className="sp-plan-card__contract-name">{plan.contractName}</p>
					<p className="sp-plan-card__fit">{plan.stage}</p>
				</div>

				<dl className="sp-plan-card__pricing">
					<div>
						<dt>{isAnnual ? "年繳方案費" : "方案月費"}</dt>
						<dd>
							<span className="sp-plan-card__price-value sp-plan-card__price-value--fixed">
								<span className="sp-plan-card__price-affix">NT$</span>
								<span className="sp-plan-card__price-number">
									{fixedFeeLabel.slice(3)}
								</span>
							</span>
							<small>{isAnnual ? "／年" : "／月"}</small>
						</dd>
					</div>
					<div>
						<dt>成交服務費</dt>
						<dd>
							<span className="sp-plan-card__price-value">
								<span className="sp-plan-card__price-number">
									{transactionFeeLabel.slice(0, -1)}
								</span>
								<span className="sp-plan-card__price-affix">%</span>
							</span>
							<small>／有效交易</small>
						</dd>
					</div>
				</dl>
				<div className="sp-plan-card__billing-note" data-cycle={billingCycle}>
					<p data-active={!isAnnual ? "true" : "false"} aria-hidden={isAnnual}>
						{monthlyBillingNote}
					</p>
					<p data-active={isAnnual ? "true" : "false"} aria-hidden={!isAnnual}>
						{annualBillingNote}
					</p>
				</div>

				<ul className="sp-plan-card__features">
					{highlights.map((highlight) => (
						<li key={highlight}>
							<AppIcon name="check" size={18} />
							<span>{highlight}</span>
						</li>
					))}
				</ul>

				<div className="sp-plan-card__select-hint" aria-hidden="true">
					<span className="sp-plan-card__select-indicator">
						<AppIcon name="check" size={20} />
					</span>
					<strong>{isSelected ? "想了解" : plan.cta}</strong>
					<span>{isSelected ? "可請團隊協助評估" : "點一下選擇此方案"}</span>
				</div>
			</Card>
		</>
	);
}

function BillingCycleToggle({
	value,
	onChange,
}: {
	value: PublicBillingCycle;
	onChange: (cycle: PublicBillingCycle) => void;
}) {
	return (
		<fieldset
			className="sp-billing-cycle"
			data-cycle={value}
			data-sp-reveal="up"
			data-sp-delay="1"
		>
			<legend className="visually-hidden">付款週期</legend>
			<input
				className="sp-billing-cycle__input visually-hidden"
				id="sp-billing-monthly"
				type="radio"
				name="pricing-billing-cycle"
				value="monthly"
				checked={value === "monthly"}
				onChange={() => onChange("monthly")}
			/>
			<label
				htmlFor="sp-billing-monthly"
				data-active={value === "monthly" ? "true" : "false"}
			>
				月繳
			</label>
			<input
				className="sp-billing-cycle__input visually-hidden"
				id="sp-billing-annual"
				type="radio"
				name="pricing-billing-cycle"
				value="annual"
				checked={value === "annual"}
				onChange={() => onChange("annual")}
			/>
			<label
				htmlFor="sp-billing-annual"
				data-active={value === "annual" ? "true" : "false"}
			>
				年繳
				<span className="sp-billing-cycle__saving">省 2 個月</span>
			</label>
		</fieldset>
	);
}

function continueToApplication(
	plan: PublicSellerPlanId,
	billingCycle: PublicBillingCycle,
) {
	const title = document.getElementById("sp-application-title");
	if (!title) return;
	const reduceMotion = window.matchMedia(
		"(prefers-reduced-motion: reduce)",
	).matches;
	title.focus({ preventScroll: true });
	title.scrollIntoView({
		behavior: reduceMotion ? "auto" : "smooth",
		block: "start",
	});
	trackPricing("seller_plan_help_click", "pricing_plan_continue", {
		plan,
		billing_cycle: billingCycle,
	});
}

function ManagedServiceOption({
	selectedService,
	onSelectService,
}: {
	selectedService: SellerLeadServiceInterest;
	onSelectService: (service: SellerLeadServiceInterest) => void;
}) {
	const addOn = PUBLIC_MANAGED_SERVICE_ADD_ON;
	const isSelected = selectedService === addOn.id;
	const inputRef = React.useRef<HTMLInputElement>(null);

	return (
		<fieldset
			className="sp-managed-service"
			id="managed-service"
			aria-describedby="sp-managed-service-boundary"
			data-selected={isSelected ? "true" : "false"}
			data-sp-reveal="up"
			data-sp-delay="2"
		>
			<legend className="visually-hidden">選擇是否加購代行包</legend>
			<div className="sp-managed-service__intro">
				<span className="sp-managed-service__icon" aria-hidden="true">
					<AppIcon name="work" size={22} />
				</span>
				<div>
					<span className="sp-managed-service__eyebrow">兩個方案皆可加購</span>
					<h3 id="sp-managed-service-title">{addOn.name}</h3>
					<p>{addOn.tagline}</p>
				</div>
			</div>

			<div className="sp-managed-service__price">
				<span>額外</span>
				<strong>+{formatRate(addOn.feeRate)}</strong>
				<span className="sp-managed-service__period">／每筆有效交易</span>
			</div>

			<div className="sp-managed-service__details">
				<ul aria-label="代行包包含">
					{addOn.includes.map((item) => (
						<li key={item}>
							<AppIcon name="check" size={18} />
							<span>{item}</span>
						</li>
					))}
				</ul>
				<div className="sp-managed-service__seller">
					<span className="sp-managed-service__seller-icon" aria-hidden="true">
						<AppIcon name="paper" size={20} />
					</span>
					<p>{addOn.sellerKeeps}</p>
				</div>
			</div>

			{/* biome-ignore lint/a11y/useKeyWithClickEvents: The nested native checkbox handles Space itself; this label click interception only prevents pointer-triggered scrolling. */}
			<label
				className="sp-managed-service__toggle"
				data-selected={isSelected ? "true" : "false"}
				onClick={(event) => {
					if (event.target === inputRef.current) return;
					event.preventDefault();
					inputRef.current?.focus({ preventScroll: true });
					inputRef.current?.click();
				}}
			>
				<input
					ref={inputRef}
					className="visually-hidden"
					type="checkbox"
					checked={isSelected}
					onChange={(event) => {
						const nextService = event.target.checked ? addOn.id : "none";
						onSelectService(nextService);
						trackPricing("managed_service_selected", "managed_service", {
							service_interest: nextService,
						});
					}}
				/>
				<span className="sp-managed-service__toggle-indicator">
					<AppIcon name="check" size={20} />
				</span>
				<span>
					<strong>{isSelected ? "已選代行包" : addOn.cta}</strong>
					<small>{isSelected ? "再次點選可取消" : "加購後同步到申請表"}</small>
				</span>
			</label>

			<p
				className="sp-managed-service__boundary"
				id="sp-managed-service-boundary"
			>
				{addOn.boundary}
			</p>
		</fieldset>
	);
}

function PlanSection({
	billingCycle,
	onBillingCycle,
	selectedPlan,
	onSelectPlan,
	selectedService,
	onSelectService,
}: {
	billingCycle: PublicBillingCycle;
	onBillingCycle: (cycle: PublicBillingCycle) => void;
	selectedPlan: SellerLeadPlanId;
	onSelectPlan: (planId: PublicSellerPlanId) => void;
	selectedService: SellerLeadServiceInterest;
	onSelectService: (service: SellerLeadServiceInterest) => void;
}) {
	return (
		<section
			className="sp-section sp-plans"
			id="plans"
			aria-labelledby="sp-plans-title"
		>
			<div className="sp-shell">
				<div
					className="sp-section-heading sp-section-heading--center"
					data-sp-reveal="up"
				>
					<h2 id="sp-plans-title">Hawkr 只有兩個方案。</h2>
					<p>
						Free 適合先驗證成交，Pro 適合已有穩定訂單的團隊；Pro 選年繳，可用 10
						個月費用使用 12 個月。
					</p>
				</div>
				<BillingCycleToggle
					value={billingCycle}
					onChange={(cycle) => {
						onBillingCycle(cycle);
						trackPricing("billing_cycle_selected", "pricing_toggle", {
							billing_cycle: cycle,
						});
					}}
				/>

				<fieldset className="sp-plan-chooser">
					<legend className="visually-hidden">選擇 Free 或 Pro 方案</legend>
					<div className="sp-plan-grid" data-sp-reveal="up" data-sp-delay="1">
						{PUBLIC_SELLER_PLANS.map((plan) => (
							<div
								id={`plan-${plan.id}`}
								key={plan.id}
								className="sp-plan-anchor"
							>
								<PricingCard
									plan={plan}
									billingCycle={billingCycle}
									selectedPlan={selectedPlan}
									onSelectPlan={onSelectPlan}
								/>
							</div>
						))}
					</div>
				</fieldset>

				<ManagedServiceOption
					selectedService={selectedService}
					onSelectService={onSelectService}
				/>

				<div className="sp-plan-action">
					<p aria-live="polite" aria-atomic="true">
						{selectedPlan === "undecided"
							? "先選一個想了解的方案，我們會把選擇同步到聯絡表。"
							: `想了解 ${PUBLIC_SELLER_PLANS.find((plan) => plan.id === selectedPlan)?.name ?? "方案"}，${billingCycle === "annual" ? "年繳" : "月繳"}。`}
					</p>
					<Button
						type="button"
						color="primary"
						size="lg"
						disabled={selectedPlan === "undecided"}
						className="sp-plan-action__button"
						onClick={() => {
							if (selectedPlan === "undecided") return;
							continueToApplication(selectedPlan, billingCycle);
						}}
						endContent={<AppIcon name="arrow-down" size={18} />}
					>
						請團隊協助評估
					</Button>
				</div>

				<p className="sp-plans__note">{PUBLIC_TAX_NOTE}</p>
			</div>
		</section>
	);
}

function PricingFaqSection() {
	const [activeCategory, setActiveCategory] =
		React.useState<PublicPricingFaqCategoryId | null>(null);
	const groupsRef = React.useRef<HTMLDivElement>(null);

	React.useEffect(() => {
		const syncCategoryFromHash = () => {
			const hashCategory = PUBLIC_SELLER_PRICING_FAQ_CATEGORIES.find(
				(category) =>
					window.location.hash === `#sp-faq-category-${category.id}`,
			);
			setActiveCategory(hashCategory?.id ?? "plans");
		};

		syncCategoryFromHash();
		window.addEventListener("hashchange", syncCategoryFromHash);

		const root = groupsRef.current;
		if (!root || typeof IntersectionObserver === "undefined") {
			return () =>
				window.removeEventListener("hashchange", syncCategoryFromHash);
		}

		const groups = Array.from(
			root.querySelectorAll<HTMLElement>("[data-faq-category]"),
		);

		const observer = new IntersectionObserver(
			(entries) => {
				const visibleEntry = entries
					.filter((entry) => entry.isIntersecting)
					.sort(
						(a, b) =>
							Math.abs(a.boundingClientRect.top - window.innerHeight * 0.22) -
							Math.abs(b.boundingClientRect.top - window.innerHeight * 0.22),
					)[0];
				const category = visibleEntry?.target.getAttribute(
					"data-faq-category",
				) as PublicPricingFaqCategoryId | null;
				if (category) setActiveCategory(category);
			},
			{
				rootMargin: "-16% 0px -62% 0px",
				threshold: [0, 0.1],
			},
		);

		for (const group of groups) observer.observe(group);
		return () => {
			observer.disconnect();
			window.removeEventListener("hashchange", syncCategoryFromHash);
		};
	}, []);

	return (
		<section
			className="sp-section sp-faq"
			id="faq"
			aria-labelledby="sp-faq-title"
		>
			<div className="sp-shell sp-faq__layout">
				<div className="sp-faq__sidebar" data-sp-reveal="left">
					<div className="sp-section-heading sp-faq__heading">
						<h2 id="sp-faq-title">常見問題</h2>
						<p>費用、退款與責任先說清楚，申請後再確認正式條款。</p>
						<Link className="sp-text-link" href="/seller-guide">
							查看完整賣家說明書
							<AppIcon name="arrow-right" size={17} />
						</Link>
					</div>

					<nav className="sp-faq__nav" aria-label="常見問題分類">
						<p className="sp-faq__nav-label">
							<span>快速前往</span>
							<span className="sp-faq__nav-meta">
								共 {PUBLIC_SELLER_PRICING_FAQS.length} 題
							</span>
						</p>
						<div className="sp-faq__nav-items">
							{PUBLIC_SELLER_PRICING_FAQ_CATEGORIES.map((category) => {
								const questionCount = PUBLIC_SELLER_PRICING_FAQS.filter(
									(faq) => faq.category === category.id,
								).length;
								const isActive = activeCategory === category.id;

								return (
									<a
										className="sp-faq__nav-link"
										href={`#sp-faq-category-${category.id}`}
										aria-controls={`sp-faq-category-${category.id}`}
										aria-current={isActive ? "location" : undefined}
										key={category.id}
										onClick={() => {
											setActiveCategory(category.id);
											trackPricing(
												"pricing_faq_category_select",
												"faq_navigation",
												{ category: category.id },
											);
										}}
									>
										<span className="sp-faq__nav-icon" aria-hidden="true">
											<AppIcon name={category.icon} size={20} />
										</span>
										<span className="sp-faq__nav-copy">
											<strong>{category.label}</strong>
											<small>{category.description}</small>
										</span>
										<span className="sp-faq__nav-count" aria-hidden="true">
											{questionCount}
										</span>
									</a>
								);
							})}
						</div>
					</nav>
				</div>

				<div
					className="sp-faq__groups"
					data-sp-reveal="right"
					data-sp-delay="1"
					ref={groupsRef}
				>
					{PUBLIC_SELLER_PRICING_FAQ_CATEGORIES.map((category) => {
						const questions = PUBLIC_SELLER_PRICING_FAQS.filter(
							(faq) => faq.category === category.id,
						);

						return (
							<section
								className="sp-faq-group"
								id={`sp-faq-category-${category.id}`}
								aria-labelledby={`sp-faq-category-title-${category.id}`}
								data-faq-category={category.id}
								key={category.id}
							>
								<header className="sp-faq-group__header">
									<span className="sp-faq-group__icon" aria-hidden="true">
										<AppIcon name={category.icon} size={21} />
									</span>
									<div>
										<h3 id={`sp-faq-category-title-${category.id}`}>
											{category.label}
										</h3>
										<p>{category.description}</p>
									</div>
									<span className="sp-faq-group__count">
										{questions.length} 題
									</span>
								</header>

								<div className="sp-faq__list">
									{questions.map((faq) => (
										<details
											className="sp-faq-item"
											name="seller-pricing-faq"
											key={faq.question}
											onToggle={(event) => {
												if (!event.currentTarget.open) return;
												trackPricing("pricing_faq_expand", "faq", {
													category: faq.category,
													question: faq.question,
												});
											}}
										>
											<summary>
												<h4>
													<span>{faq.question}</span>
													<span
														className="sp-faq-item__toggle"
														aria-hidden="true"
													>
														<AppIcon name="chevron-down" size={19} />
													</span>
												</h4>
											</summary>
											<div className="sp-faq-item__answer">
												<p>{faq.answer}</p>
											</div>
										</details>
									))}
								</div>
							</section>
						);
					})}
				</div>
			</div>
		</section>
	);
}

function SellerLeadForm({
	billingCycle,
	onBillingCycle,
	selectedPlan,
	onSelectedPlan,
	serviceInterest,
	onServiceInterest,
}: {
	billingCycle: PublicBillingCycle;
	onBillingCycle: (cycle: PublicBillingCycle) => void;
	selectedPlan: SellerLeadPlanId;
	onSelectedPlan: (planId: SellerLeadPlanId) => void;
	serviceInterest: SellerLeadServiceInterest;
	onServiceInterest: (service: SellerLeadServiceInterest) => void;
}) {
	const [lead, setLead] = React.useState<LeadState>(EMPTY_LEAD);
	const [status, setStatus] = React.useState<
		"idle" | "submitting" | "success" | "error"
	>("idle");
	const successPanelRef = React.useRef<HTMLDivElement>(null);
	const startedRef = React.useRef(false);
	const hasContact = lead.email.trim() !== "" || lead.phone.trim() !== "";
	const canSubmit =
		lead.brandName.trim() !== "" &&
		lead.contactName.trim() !== "" &&
		lead.productType.trim() !== "" &&
		hasContact;

	React.useEffect(() => {
		if (status === "success") successPanelRef.current?.focus();
	}, [status]);

	const updateLead = React.useCallback(
		(name: keyof LeadState, value: string) => {
			setLead((current) => ({ ...current, [name]: value }));
			if (!startedRef.current) {
				startedRef.current = true;
				trackPricing("seller_contact_request_started", "lead_form");
			}
			if (status === "error") setStatus("idle");
		},
		[status],
	);

	const handleSubmit = async (event: React.FormEvent<HTMLFormElement>) => {
		event.preventDefault();
		if (!canSubmit || status === "submitting") return;
		setStatus("submitting");
		const planAtSubmission = selectedPlan;
		const billingAtSubmission = billingCycle;
		const serviceAtSubmission = serviceInterest;

		try {
			await submitSellerLeadRequest(
				{
					...lead,
					plan: planAtSubmission,
					billingCycle: billingAtSubmission,
					serviceInterest: serviceAtSubmission,
				},
				{
					post: (submission) =>
						fetch("/api/public/seller-leads", {
							method: "POST",
							headers: {
								Accept: "application/json",
								"Content-Type": "application/json",
							},
							credentials: "same-origin",
							body: JSON.stringify(submission),
						}),
					onAccepted: (context) => {
						trackSellerLead(context);
						trackPricing("seller_contact_request_submit", "lead_form", {
							plan: context.plan,
							billing_cycle: context.billingCycle,
							service_interest: context.serviceInterest,
						});
					},
				},
			);
			setLead(EMPTY_LEAD);
			setStatus("success");
		} catch {
			setStatus("error");
		}
	};

	if (status === "success") {
		return (
			<div
				ref={successPanelRef}
				className="sp-lead-success"
				role="status"
				tabIndex={-1}
				aria-labelledby="sp-lead-success-title"
				aria-describedby="sp-lead-success-description"
			>
				<span className="sp-lead-success__icon" aria-hidden="true">
					<AppIcon name="check" size={30} fill />
				</span>
				<h3 id="sp-lead-success-title">我們已收到你的聯絡需求。</h3>
				<p id="sp-lead-success-description">
					團隊會依你留下的資料確認商品與開店需求，再說明適用方案及下一步。送出資料不代表審核通過或完成簽約。
				</p>
				<Button
					href="/seller/onboarding"
					variant="bordered"
					color="primary"
					size="lg"
					endContent={<AppIcon name="arrow-right" size={18} />}
				>
					繼續建立賣場
				</Button>
			</div>
		);
	}

	return (
		<form
			className="sp-lead-form"
			onSubmit={handleSubmit}
			aria-busy={status === "submitting" || undefined}
		>
			<div className="sp-lead-form__grid">
				<Input
					label="品牌／農場名稱"
					name="brandName"
					value={lead.brandName}
					onChange={(event) => updateLead("brandName", event.target.value)}
					isRequired
					variant="bordered"
					size="lg"
					maxLength={120}
					autoComplete="organization"
					startContent={<AppIcon name="work" size={19} />}
				/>
				<Input
					label="聯絡人"
					name="contactName"
					value={lead.contactName}
					onChange={(event) => updateLead("contactName", event.target.value)}
					isRequired
					variant="bordered"
					size="lg"
					maxLength={80}
					autoComplete="name"
					startContent={<AppIcon name="user" size={19} />}
				/>
				<Select
					label="商品品類"
					name="productType"
					value={lead.productType}
					onChange={(event) => updateLead("productType", event.target.value)}
					isRequired
					variant="bordered"
					size="lg"
					placeholder="請選擇"
					startContent={<AppIcon name="category" size={19} />}
				>
					<option value="蔬果與農產">蔬果與農產</option>
					<option value="肉品與水產">肉品與水產</option>
					<option value="加工食品">加工食品</option>
					<option value="飲品與甜點">飲品與甜點</option>
					<option value="禮盒與其他">禮盒與其他</option>
				</Select>
				<Select
					label="想了解的方案（選填）"
					name="plan"
					value={selectedPlan}
					onChange={(event) => {
						onSelectedPlan(event.target.value as SellerLeadPlanId);
						if (status === "error") setStatus("idle");
					}}
					variant="bordered"
					size="lg"
					startContent={<AppIcon name="paper" size={19} />}
				>
					<option value="undecided">還不確定，想先了解</option>
					{PUBLIC_SELLER_PLANS.map((plan) => (
						<option value={plan.id} key={plan.id}>
							{plan.name}
						</option>
					))}
				</Select>
				<Select
					label="付款週期"
					name="billingCycle"
					value={billingCycle}
					onChange={(event) => {
						onBillingCycle(event.target.value as PublicBillingCycle);
						if (status === "error") setStatus("idle");
					}}
					variant="bordered"
					size="lg"
					startContent={<AppIcon name="calendar" size={19} />}
				>
					<option value="monthly">月繳</option>
					<option value="annual">年繳，Pro 省 2 個月</option>
				</Select>
				<Select
					label="加值服務（選填）"
					name="serviceInterest"
					value={serviceInterest}
					onChange={(event) => {
						onServiceInterest(event.target.value as SellerLeadServiceInterest);
						if (status === "error") setStatus("idle");
					}}
					variant="bordered"
					size="lg"
					startContent={<AppIcon name="image" size={19} />}
				>
					<option value="none">暫不需要</option>
					<option value="managed">加值代行包，額外 6%</option>
				</Select>
			</div>

			<fieldset className="sp-lead-form__contact-methods">
				<legend>Email 或手機（至少填一項）</legend>
				<div className="sp-lead-form__grid">
					<Input
						label="Email"
						name="email"
						type="email"
						value={lead.email}
						onChange={(event) => updateLead("email", event.target.value)}
						variant="bordered"
						size="lg"
						maxLength={254}
						autoComplete="email"
						startContent={<AppIcon name="send" size={19} />}
					/>
					<Input
						label="手機"
						name="phone"
						type="tel"
						inputMode="tel"
						value={lead.phone}
						onChange={(event) => updateLead("phone", event.target.value)}
						variant="bordered"
						size="lg"
						minLength={8}
						maxLength={30}
						pattern="[0-9+\(\) #\-]{8,30}"
						title="請輸入至少 8 碼，可使用數字、加號、減號、括號或空格"
						autoComplete="tel"
						startContent={<AppIcon name="call" size={19} />}
					/>
				</div>
			</fieldset>

			{status === "error" ? (
				<p className="sp-lead-form__error" role="alert">
					<AppIcon name="danger" size={18} />
					<span>
						目前無法送出聯絡需求，請稍後再試，或改用
						<Link href="/contact">聯繫合作顧問</Link>。
					</span>
				</p>
			) : null}

			<div className="sp-lead-form__actions">
				<Button
					type="submit"
					color="primary"
					size="lg"
					fullWidth
					isLoading={status === "submitting"}
					loadingLabel="送出聯絡需求中…"
					disabled={!canSubmit}
					endContent={<AppIcon name="arrow-right" size={18} />}
				>
					送出聯絡需求
				</Button>
				<p className="sp-lead-form__privacy">
					送出即代表你同意我們依
					<Link href="/privacy">隱私權政策</Link>
					處理本次合作聯繫所需資料。
				</p>
			</div>
		</form>
	);
}

function ApplicationSection({
	billingCycle,
	onBillingCycle,
	selectedPlan,
	onSelectedPlan,
	serviceInterest,
	onServiceInterest,
}: {
	billingCycle: PublicBillingCycle;
	onBillingCycle: (cycle: PublicBillingCycle) => void;
	selectedPlan: SellerLeadPlanId;
	onSelectedPlan: (planId: SellerLeadPlanId) => void;
	serviceInterest: SellerLeadServiceInterest;
	onServiceInterest: (service: SellerLeadServiceInterest) => void;
}) {
	const selectedPlanName =
		PUBLIC_SELLER_PLANS.find((plan) => plan.id === selectedPlan)?.name ??
		"尚未選擇";
	const billingLabel = billingCycle === "annual" ? "年繳" : "月繳";
	const serviceLabel = serviceInterest === "managed" ? "代行包 +6%" : "未加購";

	return (
		<section
			className="sp-section sp-application"
			id="seller-application"
			aria-labelledby="sp-application-title"
		>
			<div className="sp-shell sp-application__layout">
				<div
					className="sp-section-heading sp-application__copy"
					data-sp-reveal="left"
				>
					<h2 id="sp-application-title" tabIndex={-1}>
						需要協助時，再留下聯絡方式。
					</h2>
					<p>
						自助開店不必先填這張表。若還不確定方案或想確認商品是否適合，再留下資料請團隊聯絡；送出不會扣款，也不等於審核通過或完成簽約。
					</p>
					<ul>
						<li>
							<AppIcon name="check" size={18} />
							Email 或手機擇一
						</li>
						<li>
							<AppIcon name="check" size={18} />
							Pro 可選月繳或年繳
						</li>
						<li>
							<AppIcon name="check" size={18} />
							聯絡需求不代表審核通過
						</li>
					</ul>
					<Link className="sp-text-link" href="/seller/onboarding">
						直接開始建立賣場
						<AppIcon name="arrow-right" size={18} />
					</Link>
				</div>

				<div
					className="sp-application__form-wrap"
					data-sp-reveal="right"
					data-sp-delay="1"
					data-sp-spotlight="soft"
				>
					<div className="sp-application__form-head">
						<span className="sp-application__form-icon" aria-hidden="true">
							<AppIcon name="paper-plus" size={24} />
						</span>
						<div>
							<h3>請團隊聯絡我</h3>
							<p>只填本次開店諮詢需要的資訊。</p>
						</div>
					</div>
					<div
						className="sp-selection-recap"
						role="status"
						aria-live="polite"
						aria-atomic="true"
						aria-label="目前選擇"
					>
						<div>
							<span>方案</span>
							<strong>{selectedPlanName}</strong>
						</div>
						<div>
							<span>付款</span>
							<strong>{billingLabel}</strong>
						</div>
						<div>
							<span>加值</span>
							<strong>{serviceLabel}</strong>
						</div>
					</div>
					<SellerLeadForm
						billingCycle={billingCycle}
						onBillingCycle={onBillingCycle}
						selectedPlan={selectedPlan}
						onSelectedPlan={onSelectedPlan}
						serviceInterest={serviceInterest}
						onServiceInterest={onServiceInterest}
					/>
				</div>
			</div>
		</section>
	);
}

export default function SellerPricingPage() {
	const rootRef = React.useRef<HTMLDivElement>(null);
	useSellerPricingEffects(rootRef);
	const [hydrated, setHydrated] = React.useState(false);
	const [billingCycle, setBillingCycle] =
		React.useState<PublicBillingCycle>("monthly");
	const [selectedPlan, setSelectedPlan] =
		React.useState<SellerLeadPlanId>("undecided");
	const [serviceInterest, setServiceInterest] =
		React.useState<SellerLeadServiceInterest>("none");

	React.useEffect(() => {
		setHydrated(true);
		trackPricing("pricing_page_view", "pricing_page");
	}, []);

	React.useEffect(() => {
		const viewed = new Set<string>();
		const cards = document.querySelectorAll<HTMLElement>(
			".sp-plan-card[data-plan]",
		);
		if (!("IntersectionObserver" in window)) {
			for (const card of cards) {
				const plan = card.dataset.plan;
				if (plan) trackPricing("pricing_plan_view", "pricing_card", { plan });
			}
			return;
		}
		const observer = new IntersectionObserver(
			(entries) => {
				for (const entry of entries) {
					if (!entry.isIntersecting) continue;
					const plan = (entry.target as HTMLElement).dataset.plan;
					if (!plan || viewed.has(plan)) continue;
					viewed.add(plan);
					trackPricing("pricing_plan_view", "pricing_card", { plan });
				}
			},
			{ threshold: 0.45 },
		);
		for (const card of cards) observer.observe(card);
		return () => observer.disconnect();
	}, []);

	return (
		<div
			ref={rootRef}
			className="seller-pricing-page"
			data-hydrated={hydrated ? "true" : "false"}
		>
			<section className="sp-hero" aria-labelledby="sp-hero-title">
				<div className="sp-shell sp-hero__grid">
					<div className="sp-hero__copy" data-sp-reveal="left">
						<h1 id="sp-hero-title">小農、鮮食與食品品牌的自助開店市集</h1>
						<p className="sp-hero__lede">
							先建立賣場，再依步驟完成必要審核；審核通過後，才開始販售。方案、費用與平台分工都先公開，遇到問題再找團隊協助。
						</p>
						<div className="sp-hero__actions">
							<Link
								className="sp-button sp-button--primary"
								data-hkui-button
								href="/seller/onboarding"
							>
								開始建立賣場
								<AppIcon name="arrow-right" size={18} />
							</Link>
							<AnchorLink
								className="sp-button sp-button--text"
								data-hkui-button
								href="#plans"
							>
								先看方案與費用
								<AppIcon name="arrow-down" size={18} />
							</AnchorLink>
						</div>
					</div>

					<div
						className="sp-hero__media"
						aria-hidden="true"
						data-sp-reveal="scale"
						data-sp-delay="1"
					>
						<div className="sp-hero__media-mask">
							<Image
								src="/placeholders/vision.svg"
								alt=""
								fill
								loading="eager"
								fetchPriority="high"
								unoptimized
								sizes="(max-width: 767px) 78vw, 40vw"
							/>
						</div>
					</div>
				</div>
			</section>

			<PlanSection
				billingCycle={billingCycle}
				onBillingCycle={setBillingCycle}
				selectedPlan={selectedPlan}
				onSelectPlan={setSelectedPlan}
				selectedService={serviceInterest}
				onSelectService={setServiceInterest}
			/>
			<PricingFaqSection />
			<ApplicationSection
				billingCycle={billingCycle}
				onBillingCycle={setBillingCycle}
				selectedPlan={selectedPlan}
				onSelectedPlan={setSelectedPlan}
				serviceInterest={serviceInterest}
				onServiceInterest={setServiceInterest}
			/>
		</div>
	);
}
