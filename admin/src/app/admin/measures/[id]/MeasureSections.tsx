"use client";

import { timestampDate } from "@bufbuild/protobuf/wkt";
import {
	type Measure,
	MeasureRevisionSystemCause,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/measure_pb";
import { ViolationStatus } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import Link from "next/link";
import { DeadlineLine, StatusBadge } from "@/components/UI";
import { formatDateTimeShort } from "@/lib/format";
import { policyCategoryText } from "@/lib/policy-category";
import { Field, Section } from "../../governance/DetailBits";
import {
	endTimeText,
	measureBasisText,
	measureStateBadge,
	revokeDetailText,
	revokeReasonLabel,
	violationSeverityText,
} from "../../merchants/[id]/measure";
import {
	evidenceLinks,
	revisionComparison,
	submittedVersionText,
} from "../measure-actions";
import {
	pendingRequestOf,
	requestKindLabel,
	requestStateText,
} from "../measure-inbox";

function timeText(value: Parameters<typeof timestampDate>[0] | undefined) {
	return value ? formatDateTimeShort(timestampDate(value)) : "-";
}

/** 佐證：經同源 `/api/bff` 讀取的連結（機敏檔，每次讀取都記錄，所以在新分頁開啟、不預先載入）。 */
function EvidenceList({
	evidence,
}: {
	evidence: Parameters<typeof evidenceLinks>[0];
}) {
	const links = evidenceLinks(evidence);
	if (links.length === 0) return <>-</>;
	return (
		<ul style={{ listStyle: "none", padding: 0, margin: 0 }}>
			{links.map((link) => (
				<li key={link.label}>
					{link.href ? (
						<a href={link.href} target="_blank" rel="noopener noreferrer">
							{link.label}
						</a>
					) : (
						link.label
					)}
				</li>
			))}
		</ul>
	);
}

/** 待處理的請求（沒有時不渲染）：種類、提出時間、理由、佐證，以及提出時的版本與目前版本（有差異時並列兩版的理由與期間）。 */
export function PendingRequestSection({
	measure,
	now,
}: {
	measure: Measure;
	now: Date;
}) {
	const request = pendingRequestOf(measure);
	if (!request) return null;
	const comparison = revisionComparison(measure, request.revision);
	return (
		<Section
			id="measure-pending-request-title"
			icon="quiz"
			title="待處理的請求"
		>
			<div className="info-grid">
				<Field label="請求">{requestKindLabel(request.kind)}</Field>
				<Field label="提出時間">{timeText(request.submitTime)}</Field>
				<Field label="理由" fullWidth>
					<span style={{ whiteSpace: "pre-wrap" }}>
						{request.reason || "-"}
					</span>
				</Field>
				<Field label="佐證" fullWidth>
					<EvidenceList evidence={request.evidence} />
				</Field>
			</div>
			<p>
				<DeadlineLine
					label="裁定期限"
					deadline={request.decideDeadline}
					now={now}
				/>
			</p>
			<p>{submittedVersionText(request.revision, measure.currentRevision)}</p>
			{comparison && (
				<div style={{ overflowX: "auto" }}>
					<table className="tab-table">
						<thead>
							<tr>
								<th></th>
								<th>第 {comparison.submitted.revision} 版</th>
								<th>第 {comparison.current.revision} 版</th>
							</tr>
						</thead>
						<tbody>
							<tr>
								<th scope="row">給賣家的理由</th>
								<td style={{ whiteSpace: "pre-wrap" }}>
									{comparison.submitted.statement || "-"}
								</td>
								<td style={{ whiteSpace: "pre-wrap" }}>
									{comparison.current.statement || "-"}
								</td>
							</tr>
							<tr>
								<th scope="row">結束時間</th>
								<td>
									{comparison.submitted.endTime
										? formatDateTimeShort(comparison.submitted.endTime)
										: "-"}
								</td>
								<td>
									{comparison.current.endTime
										? formatDateTimeShort(comparison.current.endTime)
										: "-"}
								</td>
							</tr>
						</tbody>
					</table>
				</div>
			)}
		</Section>
	);
}

/** 措施內容：依據、狀態、理由、建立與期間；已處理過的請求（申覆、解除請求）與它們的裁定。 */
export function MeasureContentSection({ measure }: { measure: Measure }) {
	const state = measureStateBadge(measure.state, measure.kind);
	const revoked = revokeDetailText(measure);
	const pastRequests = measure.requests.filter(
		(r) => r !== pendingRequestOf(measure),
	);
	return (
		<Section id="measure-content-title" icon="list_alt" title="措施內容">
			<div className="info-grid">
				<Field label="依據">{measureBasisText(measure)}</Field>
				<Field label="狀態">
					<StatusBadge variant={state.variant}>{state.label}</StatusBadge>
					{revoked && `（撤銷：${revoked}）`}
				</Field>
				<Field label="給賣家的理由" fullWidth>
					<span style={{ whiteSpace: "pre-wrap" }}>
						{measure.statement || "-"}
					</span>
				</Field>
				<Field label="建立時間">
					{timeText(measure.createTime)}
					{" · "}操作者 ID：{measure.createdByAdminId || "-"}
				</Field>
				{measure.period?.startTime && (
					<Field label="生效時間">{timeText(measure.period.startTime)}</Field>
				)}
				{endTimeText(measure) && (
					<Field label="結束時間">{endTimeText(measure)}</Field>
				)}
				{measure.reviewBy && (
					<Field label="複審期限">{timeText(measure.reviewBy)}</Field>
				)}
				{measure.appealDeadlineTime && (
					<Field label="申覆期限">{timeText(measure.appealDeadlineTime)}</Field>
				)}
				{measure.sourceMeasureId && (
					<Field label="來源措施">
						<Link href={`/admin/measures/${measure.sourceMeasureId}`}>
							{measure.sourceMeasureId}
						</Link>
					</Field>
				)}
				{pastRequests.length > 0 && (
					<Field label="請求" fullWidth>
						<ul style={{ listStyle: "none", padding: 0, margin: 0 }}>
							{pastRequests.map((request) => (
								<li key={request.id}>
									{requestStateText(request)}・{timeText(request.submitTime)}
									{request.decisionReason && `・${request.decisionReason}`}
								</li>
							))}
						</ul>
					</Field>
				)}
			</div>
		</Section>
	);
}

/** 依據的認定：每一筆的類別、嚴重度、建立時間與撤銷情形；不是這筆措施建立的認定連到建立它的措施（origin_measure_id）。 */
export function CitedFindingsSection({ measure }: { measure: Measure }) {
	if (measure.findings.length === 0) return null;
	return (
		<Section id="measure-findings-title" icon="gpp_bad" title="依據的認定">
			<div style={{ overflowX: "auto" }}>
				<table className="tab-table">
					<thead>
						<tr>
							<th>認定</th>
							<th>類別</th>
							<th>嚴重度</th>
							<th>建立時間</th>
							<th>狀態</th>
							<th>來源措施</th>
						</tr>
					</thead>
					<tbody>
						{measure.findings.map((finding) => (
							<tr key={finding.id}>
								<td>{finding.id}</td>
								<td>{policyCategoryText(finding.category)}</td>
								<td>{violationSeverityText(finding.severity)}</td>
								<td>{timeText(finding.createTime)}</td>
								<td>
									{finding.status === ViolationStatus.REVOKED ? (
										<StatusBadge variant="gray">
											{`已撤銷：${timeText(finding.revokeTime)}・${revokeReasonLabel(finding.revokeReason)}`}
										</StatusBadge>
									) : (
										"-"
									)}
								</td>
								<td>
									{finding.originMeasureId &&
									finding.originMeasureId !== measure.id ? (
										<Link href={`/admin/measures/${finding.originMeasureId}`}>
											{finding.originMeasureId}
										</Link>
									) : (
										"-"
									)}
								</td>
							</tr>
						))}
					</tbody>
				</table>
			</div>
		</Section>
	);
}

const SYSTEM_CAUSE_LABEL: Record<
	Exclude<MeasureRevisionSystemCause, MeasureRevisionSystemCause.UNSPECIFIED>,
	string
> = {
	[MeasureRevisionSystemCause.STORE_CLOSED]: "店家關閉，預告終止隨之結束",
};

/** 版本紀錄（舊到新）：每一版賣家看得到的理由與結束時間，以及誰、何時寫入（系統寫入的版本顯示原因）。 */
export function RevisionsSection({ measure }: { measure: Measure }) {
	if (measure.revisions.length === 0) return null;
	return (
		<Section id="measure-revisions-title" icon="history" title="版本紀錄">
			<div style={{ overflowX: "auto" }}>
				<table className="tab-table">
					<thead>
						<tr>
							<th>版本</th>
							<th>給賣家的理由</th>
							<th>結束時間</th>
							<th>操作者 ID</th>
							<th>建立時間</th>
						</tr>
					</thead>
					<tbody>
						{measure.revisions.map((revision) => (
							<tr key={revision.revision}>
								<td>第 {revision.revision} 版</td>
								<td style={{ whiteSpace: "pre-wrap" }}>
									{revision.statement || "-"}
								</td>
								<td>{timeText(revision.endTime)}</td>
								<td>
									{revision.createdByAdminId ||
										(
											SYSTEM_CAUSE_LABEL as Partial<
												Record<MeasureRevisionSystemCause, string>
											>
										)[revision.systemCause] ||
										"-"}
								</td>
								<td>{timeText(revision.createTime)}</td>
							</tr>
						))}
					</tbody>
				</table>
			</div>
		</Section>
	);
}
