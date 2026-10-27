import {
	arbitrationEventLabel,
	arbitrationOutcomeLabel,
} from "@/lib/after-sales/status";
import type { AfterSaleArbitration } from "@/lib/after-sales/types";
import { formatTaipeiDateTime } from "@/lib/date/taipei";

function formatDateTime(raw?: string): string {
	if (!raw) return "";
	// 解不出來就把原字串照登（沿用本卡片原本的行為）。
	return formatTaipeiDateTime(raw) || raw;
}

interface Props {
	arbitration: AfterSaleArbitration;
	className?: string;
}

/** Platform arbitration (申訴 / 仲裁) timeline + decision. Read-only; shared by
 * the buyer detail page and the seller case drawer. */
export function AfterSaleArbitrationCard({
	arbitration,
	className = "",
}: Props) {
	const { decision, timeline } = arbitration;
	return (
		<div className={className}>
			{decision ? (
				<div className="rounded-hkui-md border border-divider bg-content2 p-3 mb-3">
					<div className="flex items-center justify-between gap-2">
						<span className="text-sm font-semibold text-foreground">
							平台裁決：{arbitrationOutcomeLabel(decision.outcome)}
						</span>
						{decision.decidedAt ? (
							<span className="text-xs text-foreground-soft">
								{formatDateTime(decision.decidedAt)}
							</span>
						) : null}
					</div>
					{decision.refundAmount ? (
						<p className="m-0 mt-1 text-sm text-hk-primary font-semibold">
							裁決退款 NT$ {Number(decision.refundAmount).toLocaleString()}
						</p>
					) : null}
					{decision.reason ? (
						<p className="m-0 mt-1 text-sm text-hk-body whitespace-pre-wrap">
							{decision.reason}
						</p>
					) : null}
				</div>
			) : (
				<p className="text-sm text-foreground-soft mb-3">
					申訴審理中，平台將儘速裁決。
				</p>
			)}

			{timeline.length > 0 ? (
				<ol className="list-none m-0 p-0 flex flex-col gap-3">
					{timeline.map((e, i) => (
						<li
							key={`${e.eventType}-${e.timestamp || i}`}
							className="relative pl-4"
						>
							<span
								className="absolute left-0 top-1.5 w-2 h-2 rounded-full bg-hkui-primary"
								aria-hidden="true"
							/>
							<div className="flex items-center justify-between gap-2">
								<strong className="text-sm text-foreground">
									{e.label || arbitrationEventLabel(e.eventType)}
								</strong>
								{e.timestamp ? (
									<span className="text-xs text-foreground-soft">
										{formatDateTime(e.timestamp)}
									</span>
								) : null}
							</div>
							{e.description ? (
								<p className="m-0 mt-0.5 text-xs text-foreground-soft whitespace-pre-wrap">
									{e.description}
								</p>
							) : null}
						</li>
					))}
				</ol>
			) : null}
		</div>
	);
}

export default AfterSaleArbitrationCard;
