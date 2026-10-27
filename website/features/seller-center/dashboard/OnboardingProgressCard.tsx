"use client";

import type { KYCFlowStatus } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import Button from "@/components/ui/Button";
import { useUiText } from "@/lib/i18n/ui-text";
import type { ShippingOnboarding } from "@/lib/seller/shipping";
import type { MyStore } from "@/lib/seller/store";
import { useSellerDashboardUiStore } from "./dashboardUiStore";
import {
	buildOnboardingTasks,
	ONBOARDING_GROUP_LABEL,
	type OnboardingTask,
} from "./onboardingTasks";

function OnboardingStep({
	task,
	active,
	onSelect,
}: {
	task: OnboardingTask;
	active: boolean;
	onSelect: () => void;
}) {
	return (
		<li>
			<button
				type="button"
				className={`seller-onboarding-step${active ? " is-active" : ""}${
					task.completed ? " is-complete" : ""
				}${task.locked ? " is-locked" : ""}`}
				aria-current={active ? "step" : undefined}
				aria-label={`${task.title}，${task.statusLabel}${
					task.optional ? "，非必要" : ""
				}`}
				onClick={onSelect}
			>
				<span className="seller-onboarding-step__icon">
					<AppIcon name={task.icon} size={34} title={task.title} />
				</span>
				<span className="seller-onboarding-step__main">
					<span className="seller-onboarding-step__title">{task.title}</span>
					{task.optional && (
						<span className="seller-onboarding-step__optional">非必要</span>
					)}
				</span>
				<span
					className={`seller-onboarding-step__state ${
						task.completed ? "is-ok" : task.locked ? "is-muted" : ""
					}`}
					title={task.statusLabel}
					aria-hidden="true"
				>
					<AppIcon
						name={task.completed ? "check" : task.locked ? "lock" : "activity"}
						size={14}
					/>
				</span>
			</button>
		</li>
	);
}

export default function OnboardingProgressCard({
	self,
	flowStatus,
	shippingRows,
	productCount,
}: {
	self: MyStore | null;
	flowStatus: KYCFlowStatus | undefined;
	shippingRows: ShippingOnboarding[];
	productCount: number | null;
}) {
	const ut = useUiText();
	const tasks = React.useMemo(
		() =>
			buildOnboardingTasks({
				ut,
				my: self,
				flowStatus,
				shippingRows,
				productCount,
			}),
		[ut, self, flowStatus, shippingRows, productCount],
	);
	const requiredTasks = tasks.filter((task) => task.group === "required");
	const completedRequired = requiredTasks.filter(
		(task) => task.completed,
	).length;
	const totalRequired = requiredTasks.length;
	const progress = totalRequired
		? Math.round((completedRequired / totalRequired) * 100)
		: 0;
	const allRequiredComplete =
		totalRequired > 0 && completedRequired === totalRequired;
	const suggestedTask =
		requiredTasks.find((task) => !task.completed) ??
		tasks.find((task) => !task.completed);
	const activeTaskId = useSellerDashboardUiStore(
		(state) => state.onboardingActiveTaskId,
	);
	const setActiveTaskId = useSellerDashboardUiStore(
		(state) => state.setOnboardingActiveTaskId,
	);
	const collapsedOverride = useSellerDashboardUiStore(
		(state) => state.onboardingCollapsedOverride,
	);
	const toggleCollapsed = useSellerDashboardUiStore(
		(state) => state.toggleOnboardingCollapsed,
	);
	// 「公開前必須」全部滿足就預設收合；「建議完成」不影響進度。
	const defaultCollapsed = allRequiredComplete;
	const collapsed = collapsedOverride ?? defaultCollapsed;

	const activeTask =
		tasks.find((task) => task.id === activeTaskId) ?? suggestedTask ?? tasks[0];
	return (
		<section
			className={`seller-onboarding-progress seller-card${
				collapsed ? " is-collapsed" : ""
			}${allRequiredComplete ? " is-complete" : ""}`}
		>
			<div className="seller-onboarding-progress__header">
				<div className="seller-onboarding-progress__summary">
					<div>
						<span className="seller-onboarding-progress__eyebrow">
							開店進度
						</span>
						<span className="seller-onboarding-progress__heading-row">
							<strong>賣家開店任務</strong>
							{allRequiredComplete && (
								<span className="seller-onboarding-progress__complete-inline">
									<AppIcon name="shield" size={28} title="任務完成" />
									<span>任務完成</span>
								</span>
							)}
						</span>
					</div>
					{!allRequiredComplete && (
						<span className="seller-onboarding-progress__count">
							{completedRequired}/{totalRequired}
						</span>
					)}
				</div>
				{!allRequiredComplete && (
					<div className="seller-onboarding-progress__meter">
						<div
							className="seller-onboarding-progress__bar"
							role="progressbar"
							aria-label="開店進度"
							aria-valuemin={0}
							aria-valuemax={100}
							aria-valuenow={progress}
							style={{
								gridTemplateColumns: `repeat(${Math.max(totalRequired, 1)}, minmax(0, 1fr))`,
							}}
						>
							{requiredTasks.map((task, index) => (
								<span
									key={task.id}
									className={
										index < completedRequired ? "is-complete" : undefined
									}
								/>
							))}
						</div>
						<small>{progress}%</small>
					</div>
				)}
				<button
					type="button"
					className="seller-onboarding-progress__toggle"
					aria-expanded={!collapsed}
					onClick={() => toggleCollapsed(defaultCollapsed)}
				>
					{collapsed ? "展開任務" : "收合"}
					<span aria-hidden="true">⌄</span>
				</button>
			</div>

			{!collapsed && (
				<div className="seller-onboarding-progress__body">
					<div className="seller-onboarding-progress__aside">
						{(["required", "recommended"] as const).map((group) => {
							const groupTasks = tasks.filter((task) => task.group === group);
							if (groupTasks.length === 0) return null;
							return (
								<React.Fragment key={group}>
									<span className="seller-onboarding-progress__eyebrow">
										{ONBOARDING_GROUP_LABEL[group]}
									</span>
									<ul className="seller-onboarding-progress__steps">
										{groupTasks.map((task) => (
											<OnboardingStep
												key={task.id}
												task={task}
												active={task.id === activeTask.id}
												onSelect={() => setActiveTaskId(task.id)}
											/>
										))}
									</ul>
								</React.Fragment>
							);
						})}
					</div>

					<div className="seller-onboarding-progress__detail">
						<div className="seller-onboarding-progress__copy">
							<div className="seller-onboarding-progress__status-row">
								<span
									className={`seller-onboarding-progress__status ${
										activeTask.completed
											? "is-ok"
											: activeTask.locked
												? "is-muted"
												: ""
									}`}
								>
									{activeTask.statusLabel}
								</span>
								{activeTask.optional && (
									<span className="seller-onboarding-progress__optional">
										非必要
									</span>
								)}
							</div>
							<h2 className="seller-onboarding-progress__title">
								{activeTask.panelTitle}
							</h2>
							<p className="seller-onboarding-progress__description">
								{activeTask.panelDescription}
							</p>
							{activeTask.cta ? (
								<Button
									href={activeTask.href}
									size="lg"
									disabled={activeTask.locked}
									className="seller-onboarding-progress__cta"
								>
									{activeTask.cta}
								</Button>
							) : null}
						</div>
						<div
							className="seller-onboarding-progress__visual"
							aria-hidden="true"
						>
							<AppIcon key={activeTask.id} name={activeTask.icon} size={190} />
						</div>
					</div>
				</div>
			)}
		</section>
	);
}
