"use client";

import AppIcon, { type AppIconName } from "@/components/shared/AppIcon";
import { afterSaleSteps } from "@/lib/after-sales/status";
import type { AfterSaleCase } from "@/lib/after-sales/types";
import { ORDER_CANCEL_REASON_LABEL } from "@/lib/codemeta/enums";
import { formatTaipeiMonthDayTime } from "@/lib/date/taipei";
import {
	ORDER_STATUS_TIMELINE_STEPS,
	orderTimelineCurrentIndex,
} from "./orderStatusTimelineModel";

interface OrderStatusTimelineProps {
	/** Order's current status (matches the keys in STATUS_LABEL). */
	status: string;
	/**
	 * Optional event log keyed by status. When provided, the matching step
	 * shows the timestamp underneath the label so buyers/sellers see when
	 * each milestone happened.
	 */
	events?: Partial<Record<string, number>>;
	cancelReason?: string;
	cancelByRole?: string;
	cancelRemark?: string;
	/** Latest after-sale case. When present, render the after-sale progress rail. */
	aftersale?: AfterSaleCase | null;
	/** Backend shipping method code or legacy label, e.g. 3001 / HOME_DELIVERY / 黑貓宅急便. */
	shippingMethod?: string | null;
	/** Compact mode is used inside seller after-sale rows. */
	variant?: "default" | "compact";
}

/**
 * Statuses that terminate the timeline outside the normal flow. Cancellation
 * still uses the rail so the stopped step can be shown as 已取消.
 */
const TERMINAL_VARIANTS: Record<
	string,
	{ label: string; tone: "danger" | "warning"; icon: string; detail?: string }
> = {
	refunded: { label: "已退款", tone: "danger", icon: "↺" },
	after_sale: { label: "退換貨處理中", tone: "warning", icon: "!" },
};

const terminalContainerClasses: Record<"danger" | "warning", string> = {
	danger:
		"bg-gradient-to-br from-[rgba(196,60,60,0.13)] to-[rgba(196,60,60,0.05)] border-[rgba(196,60,60,0.24)] text-[#6f2424] dark:from-[rgba(232,100,100,0.32)] dark:to-[rgba(196,60,60,0.14)] dark:border-[rgba(245,110,110,0.42)] dark:text-[#ffd7d7] dark:shadow-[0_6px_20px_-10px_rgba(196,60,60,0.6)]",
	warning:
		"bg-gradient-to-br from-[rgba(217,168,23,0.2)] to-[rgba(217,168,23,0.08)] border-[rgba(217,168,23,0.3)] text-[#6f5212] dark:from-[rgba(241,205,112,0.28)] dark:to-[rgba(241,205,112,0.1)] dark:border-[rgba(245,222,165,0.4)] dark:text-[#ffeec0] dark:shadow-[0_6px_20px_-10px_rgba(241,205,112,0.4)]",
};

const terminalIconClasses: Record<"danger" | "warning", string> = {
	danger:
		"bg-[#c43c3c] text-white shadow-[0_3px_8px_-3px_rgba(196,60,60,0.55)] dark:bg-[#e85a5a] dark:shadow-[0_0_0_1px_rgba(245,110,110,0.55),_0_3px_10px_-2px_rgba(232,90,90,0.55)]",
	warning:
		"bg-[#d9a817] text-white shadow-[0_3px_8px_-3px_rgba(217,168,23,0.55)] dark:bg-[#f1cd70] dark:text-[#2d2010] dark:shadow-[0_3px_10px_-2px_rgba(241,205,112,0.6)]",
};

type Tone = "done" | "current" | "idle";

const stepLi: Record<Tone, string> = {
	done: "text-hk-primary dark:text-hk-ink-200",
	current: "text-hk-body dark:text-[#f5e9c3]",
	idle: "text-hk-muted dark:text-hk-ink-100/50",
};

const stepDot: Record<Tone, string> = {
	done: "bg-hk-primary/14 text-hk-accent-on-tint ring-1 ring-hk-primary/25 dark:bg-hk-ink-500/22 dark:!text-hk-ink-200 dark:ring-hk-ink-200/28",
	current: "",
	idle: "bg-hk-primary/8 dark:bg-white/6 text-hk-muted dark:!text-hk-ink-100/48",
};

const stepLabel: Record<Tone, string> = {
	done: "",
	current: "font-bold",
	idle: "",
};

const stepText: Record<Tone, string> = {
	done: "text-hk-primary dark:!text-hk-ink-200",
	current: "text-hk-body dark:!text-[#f5e9c3]",
	idle: "text-hk-muted dark:!text-hk-ink-100/50",
};

const cancelledCurrentTextClass = "text-[#9f2f2f] dark:!text-[#ffd7d7]";

const cancelledCurrentDotClass =
	"bg-[#c43c3c] text-white shadow-[0_0_0_4px_rgba(196,60,60,0.16)] dark:bg-[#e85a5a] dark:text-[#081119] dark:shadow-[0_0_0_4px_rgba(232,90,90,0.22)]";

const cancelledConnectorClass = "bg-[#c43c3c]/55 dark:bg-[#e85a5a]/42";

type TimelineIconKey =
	| "waiting"
	| "paid"
	| "packing"
	| "shippingTruck"
	| "review"
	| "sellerReview"
	| "returning"
	| "refund"
	| "coupon"
	| "completed";

interface TimelineIconStep {
	state: Tone;
	icon: TimelineIconKey;
	mergedComplete?: boolean;
	symbol?: string;
}

const DOT_ICON_SIZE = 24;

const timelineIconName: Record<TimelineIconKey, AppIconName> = {
	waiting: "clock",
	paid: "coin",
	packing: "box",
	shippingTruck: "truck",
	review: "star",
	sellerReview: "search",
	returning: "refresh",
	refund: "wallet",
	coupon: "discount",
	completed: "party",
};

function orderStepIcon(
	stepKey: string,
	shippingMethod?: string | null,
): TimelineIconKey {
	switch (stepKey) {
		case "pending_payment":
			return "waiting";
		case "paid":
			return "paid";
		case "preparing":
			return "packing";
		case "shipping":
			return shippingMethodIcon(shippingMethod);
		case "completed":
			return "completed";
		default:
			return "waiting";
	}
}

function afterSaleStepIcon(
	stepKey: string,
	aftersale: Pick<
		AfterSaleCase,
		"refundAmount" | "returnShippingMethod" | "status"
	>,
): TimelineIconKey {
	switch (stepKey) {
		case "申請":
			return "review";
		case "審核":
			return "sellerReview";
		case "退貨退款":
			return afterSaleResolutionIcon(aftersale);
		case "完成":
			return "completed";
		default:
			return "review";
	}
}

function afterSaleResolutionIcon(
	aftersale: Pick<
		AfterSaleCase,
		"refundAmount" | "returnShippingMethod" | "status"
	>,
): TimelineIconKey {
	if (aftersale.returnShippingMethod) return "returning";
	const refunded = Number(aftersale.refundAmount);
	if (Number.isFinite(refunded) && refunded > 0) return "refund";
	if (Number.isFinite(refunded) && refunded <= 0 && aftersale.status === "50") {
		return "coupon";
	}
	return "refund";
}

function shippingMethodIcon(_method?: string | null): TimelineIconKey {
	return "shippingTruck";
}

function stepIconVisibilityClassFor(step: TimelineIconStep): string {
	if (step.mergedComplete) return "opacity-100";
	if (step.state === "done") return "opacity-75";
	if (step.state === "idle") return "opacity-70";
	return "opacity-100";
}

function timelinePaintFor(step: TimelineIconStep): string {
	if (step.state === "idle") return "var(--hkui-foreground-soft)";
	if (step.state === "done") return "var(--icon-accent,var(--hkui-primary))";
	return "var(--hkui-primary-foreground,#ffffff)";
}

function currentStepDotClass(stepKey: string): string {
	if (stepKey === "pending_payment") {
		return "bg-[#d8a817] text-hk-inverse shadow-[0_0_0_4px_rgba(216,168,23,0.16)] dark:bg-[#b89437] dark:text-[#081119] dark:shadow-[0_0_0_4px_rgba(184,148,55,0.2)]";
	}
	if (stepKey === "shipping") {
		return "bg-hk-ink-400 text-hk-inverse shadow-[0_0_0_4px_rgb(var(--color-ink-400-rgb)/0.16)] dark:bg-hk-ink-300 dark:text-[#081119] dark:shadow-[0_0_0_4px_rgb(var(--color-ink-300-rgb)/0.2)]";
	}
	return "bg-hk-primary text-hk-inverse shadow-[0_0_0_4px_rgb(var(--color-brand-primary-rgb)/0.16)] dark:bg-hk-ink-400 dark:text-[#081119] dark:shadow-[0_0_0_4px_rgb(var(--color-ink-400-rgb)/0.22)]";
}

function stepDotClass(tone: Tone, stepKey: string): string {
	return tone === "current" ? currentStepDotClass(stepKey) : stepDot[tone];
}

function stepConnectorClass(isDone: boolean): string {
	return isDone
		? "bg-hk-primary/55 dark:bg-hk-ink-400/42"
		: "bg-hk-border-soft dark:bg-white/10";
}

function TimelineDotIcon({
	step,
	size = DOT_ICON_SIZE,
}: {
	step: TimelineIconStep;
	size?: number;
}) {
	if (step.symbol) {
		return (
			<span
				className="grid size-full place-items-center text-[1.1em] font-bold leading-none"
				aria-hidden="true"
			>
				{step.symbol}
			</span>
		);
	}
	return (
		<AppIcon
			name={timelineIconName[step.icon]}
			size={size}
			style={{ color: timelinePaintFor(step) }}
			className={`${stepIconVisibilityClassFor(step)} scale-[1.18]`}
		/>
	);
}

/**
 * Horizontal step rail for the order detail page header. Pure
 * presentational. Caller passes the current status string and optionally
 * a timestamp lookup.
 */
export default function OrderStatusTimeline({
	status,
	events,
	cancelReason,
	cancelByRole,
	cancelRemark,
	aftersale,
	shippingMethod,
	variant = "default",
}: OrderStatusTimelineProps) {
	if (aftersale) {
		return <AfterSaleStatusTimeline aftersale={aftersale} variant={variant} />;
	}
	if (status === "cancelled") {
		return (
			<CancelledOrderTimeline
				events={events}
				cancelReason={cancelReason}
				cancelByRole={cancelByRole}
				cancelRemark={cancelRemark}
			/>
		);
	}

	const terminal = TERMINAL_VARIANTS[status];
	if (terminal) {
		return (
			<div
				className={`flex items-center gap-3.5 p-[16px_22px] rounded-hk-lg border shadow-[0_4px_14px_-8px_rgba(0,0,0,0.1)] ${terminalContainerClasses[terminal.tone]}`}
				role="status"
			>
				<span
					className={`w-[34px] h-[34px] rounded-full grid place-items-center text-[0.9375rem] font-extrabold shrink-0 leading-none ${terminalIconClasses[terminal.tone]}`}
					aria-hidden="true"
				>
					{terminal.icon}
				</span>
				<span className="flex min-w-0 flex-col gap-0.5">
					<span className="text-base font-bold tracking-[0.01em]">
						{terminal.label}
					</span>
					{terminal.detail ? (
						<span className="text-sm font-medium opacity-85">
							{terminal.detail}
						</span>
					) : null}
				</span>
			</div>
		);
	}

	const currentIdx = orderTimelineCurrentIndex({
		orderStatus: status,
		hasAfterSale: false,
	});

	return (
		<ol
			className="m-0 flex list-none items-start p-0"
			aria-label="訂單狀態進度"
		>
			{ORDER_STATUS_TIMELINE_STEPS.map((step, i) => {
				const isDone = i < currentIdx;
				const isCurrent = i === currentIdx;
				const stamp = events?.[step.key];
				const tone: Tone = isDone ? "done" : isCurrent ? "current" : "idle";
				const dotStep: TimelineIconStep = {
					state: tone,
					icon: orderStepIcon(step.key, shippingMethod),
				};
				return (
					<li
						key={step.key}
						className={`relative flex flex-1 flex-col items-center gap-1.5 text-[0.8125rem] ${stepLi[tone]}`}
						aria-current={isCurrent ? "step" : undefined}
					>
						<span
							className={`relative z-[1] grid size-[30px] place-items-center rounded-full text-[0.8125rem] font-bold ${stepDotClass(tone, step.key)}`}
							aria-hidden="true"
						>
							<TimelineDotIcon step={dotStep} size={20} />
						</span>
						<span
							className={`text-[0.8125rem] font-semibold ${stepLabel[tone]} ${stepText[tone]}`}
						>
							{step.label}
						</span>
						{stamp ? (
							<time className="text-[0.6875rem] !text-hk-muted dark:!text-hk-ink-100/50">
								{formatStamp(stamp)}
							</time>
						) : null}
						{i < ORDER_STATUS_TIMELINE_STEPS.length - 1 ? (
							<span
								className={`absolute top-[15px] left-[calc(50%+18px)] right-[calc(-50%+18px)] h-[2px] z-0 ${stepConnectorClass(isDone)}`}
								aria-hidden="true"
							/>
						) : null}
					</li>
				);
			})}
		</ol>
	);
}

function AfterSaleStatusTimeline({
	aftersale,
	variant,
}: {
	aftersale: AfterSaleCase;
	variant: "default" | "compact";
}) {
	const steps = afterSaleSteps(aftersale.status);
	const currentIdx = afterSaleTimelineCurrentIndex(steps);

	return (
		<ol
			className={[
				"m-0 flex list-none items-start p-0",
				variant === "compact" ? "text-[0.92em]" : "",
			]
				.filter(Boolean)
				.join(" ")}
			aria-label="售後狀態進度"
		>
			{steps.map((step, i) => {
				const isDone = i < currentIdx;
				const isCurrent = i === currentIdx;
				// 拒絕/取消時 afterSaleSteps 會把停住的那步標成 failed —— 比照
				// CancelledOrderTimeline 的紅色樣式呈現「流程止於此」。
				const isFailed = isCurrent && step.state === "failed";
				const tone: Tone = isDone ? "done" : isCurrent ? "current" : "idle";
				const dotStep: TimelineIconStep = {
					state: tone,
					icon: afterSaleStepIcon(step.key, aftersale),
					symbol: isFailed ? "!" : undefined,
				};
				const stamp = afterSaleStepStamp(aftersale, i, steps.length);

				return (
					<li
						key={step.key}
						className={`relative flex flex-1 flex-col items-center gap-1.5 text-[0.8125rem] ${isFailed ? cancelledCurrentTextClass : stepLi[tone]}`}
						aria-current={isCurrent ? "step" : undefined}
					>
						<span
							// stepDot.current 是空字串（實色圓底由 stepDotClass 依 step
							// 提供）—— 直接讀 stepDot 會讓進行中那步沒有圓底、而圖示又
							// 塗成白色 = 在淺色面板上隱形（售後詳情「狀態不見」bug）。
							className={`relative z-[1] grid size-[30px] place-items-center rounded-full text-[0.8125rem] font-bold ${isFailed ? cancelledCurrentDotClass : stepDotClass(tone, step.key)}`}
							aria-hidden="true"
						>
							<TimelineDotIcon step={dotStep} size={20} />
						</span>
						<span
							className={`text-center text-[0.8125rem] font-semibold ${stepLabel[tone]} ${isFailed ? cancelledCurrentTextClass : ""}`}
						>
							{step.label}
						</span>
						{stamp ? (
							<time className="text-[0.6875rem] text-hk-muted">{stamp}</time>
						) : null}
						{i < steps.length - 1 ? (
							<span
								className={`absolute top-[15px] left-[calc(50%+18px)] right-[calc(-50%+18px)] h-[2px] z-0 ${isDone ? "bg-hk-primary" : "bg-hk-border-soft"}`}
								aria-hidden="true"
							/>
						) : null}
					</li>
				);
			})}
		</ol>
	);
}

function CancelledOrderTimeline({
	events,
	cancelReason,
	cancelByRole,
	cancelRemark,
}: Pick<
	OrderStatusTimelineProps,
	"events" | "cancelReason" | "cancelByRole" | "cancelRemark"
>) {
	const currentIdx = cancelledTimelineCurrentIndex(events);
	const detail = formatCancelDetail(cancelReason, cancelRemark);
	const title = formatCancelTitle(cancelByRole, cancelReason);

	return (
		<div className="grid gap-2.5" role="status">
			<ol
				className="m-0 flex list-none items-start p-0"
				aria-label="訂單狀態進度"
			>
				{ORDER_STATUS_TIMELINE_STEPS.map((step, i) => {
					const isDone = i < currentIdx;
					const isCurrent = i === currentIdx;
					const stamp = events?.[step.key];
					const tone: Tone = isDone ? "done" : isCurrent ? "current" : "idle";
					const dotStep: TimelineIconStep = {
						state: tone,
						icon: orderStepIcon(step.key),
						symbol: isCurrent ? "×" : undefined,
					};
					const connectorClass =
						i === currentIdx - 1
							? cancelledConnectorClass
							: stepConnectorClass(isDone);
					return (
						<li
							key={step.key}
							className={`relative flex flex-1 flex-col items-center gap-1.5 text-[0.8125rem] ${isCurrent ? cancelledCurrentTextClass : stepLi[tone]}`}
							aria-current={isCurrent ? "step" : undefined}
						>
							<span
								className={`relative z-[1] grid size-[30px] place-items-center rounded-full text-[0.8125rem] font-bold ${isCurrent ? cancelledCurrentDotClass : stepDot[tone]}`}
								aria-hidden="true"
							>
								<TimelineDotIcon step={dotStep} size={20} />
							</span>
							<span
								className={`text-center text-[0.8125rem] font-semibold ${stepLabel[tone]} ${isCurrent ? cancelledCurrentTextClass : ""}`}
							>
								{isCurrent ? "已取消" : step.label}
							</span>
							{stamp ? (
								<time
									className={`text-[0.6875rem] ${isCurrent ? cancelledCurrentTextClass : "text-hk-muted"}`}
								>
									{formatStamp(stamp)}
								</time>
							) : null}
							{i < ORDER_STATUS_TIMELINE_STEPS.length - 1 ? (
								<span
									className={`absolute top-[15px] left-[calc(50%+18px)] right-[calc(-50%+18px)] h-[2px] z-0 ${connectorClass}`}
									aria-hidden="true"
								/>
							) : null}
						</li>
					);
				})}
			</ol>
			{detail ? (
				<p className="m-0 text-center text-xs font-medium text-hk-muted">
					{title}：{detail}
				</p>
			) : null}
		</div>
	);
}

function cancelledTimelineCurrentIndex(
	events?: Partial<Record<string, number>>,
): number {
	if (events?.completed) return 4;
	if (events?.shipping) return 3;
	if (events?.preparing || events?.paid) return 2;
	return 0;
}

function afterSaleTimelineCurrentIndex(
	steps: ReturnType<typeof afterSaleSteps>,
): number {
	const active = steps.findIndex(
		(step) => step.state === "current" || step.state === "failed",
	);
	if (active >= 0) return active;

	for (let i = steps.length - 1; i >= 0; i -= 1) {
		if (steps[i]?.state === "done") return i;
	}
	return 0;
}

function afterSaleStepStamp(
	aftersale: AfterSaleCase,
	index: number,
	stepCount: number,
): string | undefined {
	if (index === 0) return formatTimelineStamp(aftersale.createdAt);
	if (index === stepCount - 1) return formatTimelineStamp(aftersale.closedAt);
	return undefined;
}

function normalizeCancelRole(role?: string): string {
	return (role ?? "").trim().toUpperCase();
}

function formatCancelTitle(role?: string, reason?: string): string {
	const reasonCode = (reason ?? "").trim();
	if (reasonCode === "9103") return ORDER_CANCEL_REASON_LABEL[reasonCode];
	switch (normalizeCancelRole(role)) {
		case "BUYER":
		case "USER":
		case "CUSTOMER":
			return "買家取消訂單";
		case "SELLER":
		case "MERCHANT":
			return "賣家取消訂單";
		case "SYSTEM":
		case "PLATFORM":
			return "系統自動取消訂單";
		default:
			return "訂單已取消";
	}
}

function formatCancelDetail(
	reason?: string,
	remark?: string,
): string | undefined {
	const reasonCode = (reason ?? "").trim();
	const reasonLabel = reasonCode
		? (ORDER_CANCEL_REASON_LABEL[reasonCode] ?? reasonCode)
		: "";
	const cleanRemark = remark?.trim() ?? "";
	if (cleanRemark && reasonLabel && cleanRemark !== reasonLabel) {
		return `${reasonLabel}：${cleanRemark}`;
	}
	if (cleanRemark) return cleanRemark;
	if (reasonCode === "9103") return undefined;
	return reasonLabel || undefined;
}

function formatTimelineStamp(value?: number | string): string | undefined {
	if (!value) return undefined;
	if (typeof value === "number") return formatStamp(value);
	const d = new Date(value);
	if (Number.isNaN(d.getTime())) return undefined;
	return formatDate(d);
}

// 秒／毫秒判別與台北時區位移都在 `formatTaipeiMonthDayTime` 裡。
const formatStamp = formatTaipeiMonthDayTime;
const formatDate = formatTaipeiMonthDayTime;
