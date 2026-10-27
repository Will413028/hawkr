"use client";

import { timestampDate } from "@bufbuild/protobuf/wkt";
import { useQuery as useConnectQuery } from "@connectrpc/connect-query";
import { SellerGovernanceAdminService } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import type { Measure } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/measure_pb";
import { MeasureState } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";
import Link from "next/link";
import { Button } from "@/components/UI";
import { connectErrorText } from "@/lib/error-text";
import { formatDateTimeShort } from "@/lib/format";
import { unavailableLabels } from "@/lib/unavailability";
import {
	appealDeadlineText,
	endTimeText,
	measureBasisText,
	measureStateBadge,
	scheduledTerminationText,
} from "./measure";

interface ActiveMeasureCardProps {
	measure: Measure;
	endPending: boolean;
	onEnd: (measure: Measure) => void;
}

function timeText(value: Measure["createTime"]): string {
	return value ? formatDateTimeShort(timestampDate(value)) : "-";
}

/**
 * 停權中的紅色狀態卡，一個生效中的措施一張。風險依據：風險類型、給賣家的理由、建立時間與操作者 ID、結束時間與複審期限，
 * 以及結束入口（措施的 `can_end` 為真才有；不能結束時顯示 `end_unavailabilities` 的第一個原因，例如違規依據的措施要撤銷）。每一張都連到措施詳情頁（/admin/measures/[id]）。
 */
function ActiveMeasureCard({
	measure,
	endPending,
	onEnd,
}: ActiveMeasureCardProps) {
	const appealDeadline = appealDeadlineText(measure);
	const endTime = endTimeText(measure);
	// 不能結束時（例如依據違規認定的停權）顯示伺服器給的第一個原因，client 不自己寫句子。
	const endReason = measure.canEnd
		? undefined
		: unavailableLabels(measure.endUnavailabilities)[0];
	return (
		<div className="audit-notice rejected suspension-banner">
			<div className="audit-notice-icon">
				<MaterialSymbol name="block" fill />
			</div>
			<div className="audit-notice-content">
				<div className="audit-notice-title">
					{measureStateBadge(measure.state, measure.kind).label}
				</div>
				<div className="audit-notice-text">
					依據：{measureBasisText(measure)}
					<br />
					給賣家的理由：{measure.statement || "-"}
					<br />
					建立時間：{timeText(measure.createTime)}
					{" · "}操作者 ID：{measure.createdByAdminId || "-"}
					{measure.period?.startTime && (
						<>
							<br />
							生效時間：{timeText(measure.period.startTime)}
						</>
					)}
					{endTime && (
						<>
							<br />
							結束時間：{endTime}
						</>
					)}
					{measure.reviewBy && (
						<>
							{" · "}複審期限：{timeText(measure.reviewBy)}
						</>
					)}
					{appealDeadline && (
						<>
							<br />
							{appealDeadline}
						</>
					)}
					{endReason && (
						<>
							<br />
							{endReason}
						</>
					)}
				</div>
			</div>
			<Link className="btn-action gray" href={`/admin/measures/${measure.id}`}>
				查看
			</Link>
			{measure.canEnd && (
				<Button
					variant="outline"
					onClick={() => onEnd(measure)}
					disabled={endPending}
				>
					結束措施
				</Button>
			)}
		</div>
	);
}

/**
 * 預定終止（「預定終止：{日期時間}」）：帳號範圍、state=SCHEDULED 的終止措施（ListMeasures），顯示在停權狀態區上方。
 * 沒有預定終止時不渲染；讀取失敗要說出來（不能因為讀不到就當成沒有）並可重試。
 */
export function ScheduledTerminationNotice({
	accountId,
}: {
	accountId: string;
}) {
	const query = useConnectQuery(
		SellerGovernanceAdminService.method.listMeasures,
		{ accountId, state: MeasureState.SCHEDULED, pageSize: 100 },
	);
	if (query.isError) {
		return (
			<div className="audit-notice">
				<div className="audit-notice-content">
					<div className="audit-notice-text">
						{connectErrorText(query.error, "預定終止讀取失敗")}
					</div>
				</div>
				<Button variant="outline" onClick={() => void query.refetch()}>
					重試
				</Button>
			</div>
		);
	}
	const text = scheduledTerminationText(query.data?.measures ?? []);
	if (!text) return null;
	return (
		<div className="audit-notice">
			<div className="audit-notice-icon">
				<MaterialSymbol name="schedule" />
			</div>
			<div className="audit-notice-content">
				<div className="audit-notice-text">{text}</div>
			</div>
		</div>
	);
}

/**
 * 停權狀態區：GetStore.active_measures 的每一個措施各一張卡（帳號此刻全部生效中的，含跨 stint 繼承的）。
 * 頁面不自己決定有幾張卡；結束的開關只看店家閘與措施自己的 `can_end`。
 */
export function ActiveMeasureCards({
	active,
	endPending,
	onEnd,
}: {
	active: readonly Measure[];
	endPending: boolean;
	onEnd: (measure: Measure) => void;
}) {
	return (
		<>
			{active.map((measure) => (
				<ActiveMeasureCard
					key={measure.id}
					measure={measure}
					endPending={endPending}
					onEnd={onEnd}
				/>
			))}
		</>
	);
}

export default ActiveMeasureCard;
