import type { AfterSaleStepState } from "@/lib/after-sales/status";
import { afterSaleSteps } from "@/lib/after-sales/status";

const NODE_CLASS: Record<AfterSaleStepState, string> = {
	done: "bg-hkui-primary text-white border-hkui-primary",
	current: "bg-warning-50 text-warning-700 border-warning-700",
	upcoming: "bg-content2 text-foreground-soft border-divider",
	failed: "bg-danger-50 text-danger-700 border-danger-700",
};

const LABEL_CLASS: Record<AfterSaleStepState, string> = {
	done: "text-foreground",
	current: "text-foreground font-semibold",
	upcoming: "text-foreground-soft",
	failed: "text-danger-700 font-semibold",
};

interface Props {
	status: string;
	/** `compact` = order-detail summary block; `expanded` = full detail page. */
	size?: "compact" | "expanded";
	className?: string;
}

/** 4-step after-sale progress: 售後申請 → 賣家審核 → 退貨/退款 → 完成. */
export function AfterSaleProgress({
	status,
	size = "expanded",
	className = "",
}: Props) {
	const steps = afterSaleSteps(status);
	const nodeSize =
		size === "compact" ? "w-6 h-6 text-[0.6875rem]" : "w-8 h-8 text-xs";
	const nodeDiameter = size === "compact" ? "1.5rem" : "2rem";
	const labelSize = size === "compact" ? "text-[0.6875rem]" : "text-xs";
	return (
		<ol className={`grid grid-cols-4 items-start ${className}`}>
			{steps.map((step, i) => {
				const isLast = i === steps.length - 1;
				const connectorDone = step.state === "done";
				return (
					<li
						key={step.key}
						className="relative flex min-w-0 flex-col items-center"
					>
						{!isLast ? (
							<span
								className={`absolute h-0.5 ${connectorDone ? "bg-hkui-primary" : "bg-divider"}`}
								style={{
									top: `calc(${nodeDiameter} / 2)`,
									left: `calc(50% + (${nodeDiameter} / 2))`,
									right: `calc(-50% + (${nodeDiameter} / 2))`,
									transform: "translateY(-50%)",
								}}
								aria-hidden="true"
							/>
						) : null}
						<span
							className={`relative z-10 inline-flex items-center justify-center rounded-full border font-bold shrink-0 ${nodeSize} ${NODE_CLASS[step.state]}`}
						>
							{step.state === "failed" ? "!" : i + 1}
						</span>
						<span
							className={`mt-1.5 text-center px-1 truncate max-w-full ${labelSize} ${LABEL_CLASS[step.state]}`}
						>
							{step.label}
						</span>
					</li>
				);
			})}
		</ol>
	);
}

export default AfterSaleProgress;
