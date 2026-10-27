"use client";

import {
	type CaseInquiry,
	CaseInquiryState,
	CaseInquiryWithdrawCause,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/case_inquiry_pb";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";
import { Field, Section } from "@/app/admin/governance/DetailBits";
import { documentSrc } from "@/app/admin/merchants/[id]/kyc-application";
import { Button, DeadlineText, StatusBadge } from "@/components/UI";
import { deadlineView } from "@/lib/deadline";
import { connectErrorText } from "@/lib/error-text";
import { LIST_REFRESH_MS, useNow } from "@/lib/use-now";
import { INQUIRY_LATE_REPLY_TEXT } from "../../inquiry-actions";
import {
	inquiryChannelText,
	inquiryKindText,
	inquiryStateText,
	inquiryWithdrawCauseText,
} from "../../inquiry-text";
import { actorTimeText, timeText } from "../../report-view";
import type { CaseInquiriesRead } from "../../use-case-inquiries";

/** I2：區塊頂端的固定說明（3cd D′7 的句子照 rb R4）。 */
export const INQUIRY_SECTION_NOTE =
	"這裡只記下詢問；賣家端的畫面上線前，系統不會通知賣家，請另外聯絡賣家。詢問只寫你轉述的內容，不要附上通報原文或通報人的資料。";

/** 詢問的卡片標頭：「{種類}・{狀態}」（I5）。 */
export function inquiryHeadingText(inquiry: CaseInquiry): string {
	return `${inquiryKindText(inquiry.kind)}・${inquiryStateText(inquiry.state)}`;
}

/** 撤回的一句（I12）：「已於 {時間} 撤回（{原因}）」。 */
export function inquiryWithdrawnText(inquiry: CaseInquiry): string {
	return `已於 ${timeText(inquiry.withdrawTime)} 撤回（${inquiryWithdrawCauseText(inquiry.withdrawCause)}）`;
}

function stateVariant(state: CaseInquiryState) {
	if (state === CaseInquiryState.OPEN) return "warning" as const;
	if (state === CaseInquiryState.REPLIED) return "success" as const;
	return "gray" as const;
}

function InquiryCard({
	inquiry,
	now,
	onReply,
	onWithdraw,
}: {
	inquiry: CaseInquiry;
	now: Date;
	onReply: (inquiry: CaseInquiry) => void;
	onWithdraw: (inquiry: CaseInquiry) => void;
}) {
	const replyView = deadlineView(inquiry.replyDeadline, now);
	const withdrawn = inquiry.state === CaseInquiryState.WITHDRAWN;
	const reply = inquiry.reply;
	return (
		<li
			style={{
				borderTop: "1px solid var(--admin-border)",
				padding: "16px 0",
			}}
			id={`inquiry-${inquiry.id}`}
			data-inquiry-id={inquiry.id}
		>
			<div
				style={{
					display: "flex",
					flexWrap: "wrap",
					alignItems: "center",
					gap: 12,
					marginBottom: 12,
				}}
			>
				<StatusBadge variant={stateVariant(inquiry.state)}>
					{inquiryHeadingText(inquiry)}
				</StatusBadge>
				{inquiry.canRecordReply && (
					<Button variant="outline" onClick={() => onReply(inquiry)}>
						記下回覆
					</Button>
				)}
				{inquiry.canWithdraw && (
					<Button variant="outline" onClick={() => onWithdraw(inquiry)}>
						撤回
					</Button>
				)}
			</div>
			<div className="info-grid">
				<Field label="建立">
					{actorTimeText(inquiry.createdByAdminId, inquiry.createTime)}
				</Field>
				<Field label="期限">
					<DeadlineText display={replyView} title={replyView.dueText} />
				</Field>
				<Field label="給賣家的說明" fullWidth>
					<span style={{ whiteSpace: "pre-wrap" }}>{inquiry.message}</span>
				</Field>
				{reply && (
					<>
						<Field label="管道">{inquiryChannelText(reply.channel)}</Field>
						<Field label="收到的時間">{timeText(reply.receiveTime)}</Field>
						<Field label="記下">
							{actorTimeText(reply.recordedByAdminId, reply.recordTime)}
						</Field>
						<Field label="回覆" fullWidth>
							<span style={{ whiteSpace: "pre-wrap" }}>{reply.body}</span>
						</Field>
						{reply.evidence.length > 0 && (
							<Field label="附件" fullWidth>
								<ul style={{ listStyle: "none", padding: 0, margin: 0 }}>
									{reply.evidence.map((item, i) => {
										const href = documentSrc(item.viewUri);
										return (
											<li key={item.position}>
												{href ? (
													<a
														href={href}
														target="_blank"
														rel="noopener noreferrer"
													>
														{`附件 ${i + 1}`}
													</a>
												) : (
													`附件 ${i + 1}`
												)}
											</li>
										);
									})}
								</ul>
							</Field>
						)}
					</>
				)}
			</div>
			{withdrawn && (
				<>
					<p>{inquiryWithdrawnText(inquiry)}</p>
					{inquiry.withdrawCause !== CaseInquiryWithdrawCause.UNSPECIFIED &&
						!inquiry.canRecordReply && (
							<p className="adm-operation-field-note">
								{INQUIRY_LATE_REPLY_TEXT}
							</p>
						)}
				</>
			)}
		</li>
	);
}

/**
 * 案件詳情的「詢問賣家」區塊（I1–I16）：舊到新，每筆一張卡。按鈕只看伺服器的 can_*；讀取失敗只影響這個區塊。
 * 附件是機敏檔，連結在新分頁開啟（每次讀取伺服器都記錄）。
 */
export function CaseInquiriesSection({
	read,
	onReply,
	onWithdraw,
}: {
	read: CaseInquiriesRead;
	onReply: (inquiry: CaseInquiry) => void;
	onWithdraw: (inquiry: CaseInquiry) => void;
}) {
	const now = useNow(LIST_REFRESH_MS);
	return (
		<Section id="case-inquiries-title" icon="forum" title="詢問賣家">
			<p className="adm-operation-field-note">{INQUIRY_SECTION_NOTE}</p>
			{read.loading ? (
				<p role="status">載入中...</p>
			) : read.failed ? (
				<div className="tab-empty-state" role="alert">
					<MaterialSymbol name="error" />
					<span>{connectErrorText(read.error, "詢問讀取失敗")}</span>
					<Button variant="outline" onClick={read.retry}>
						重試
					</Button>
				</div>
			) : read.inquiries.length === 0 ? (
				<p>還沒有詢問</p>
			) : (
				<ul style={{ listStyle: "none", padding: 0, margin: 0 }}>
					{read.inquiries.map((inquiry) => (
						<InquiryCard
							key={inquiry.id}
							inquiry={inquiry}
							now={now}
							onReply={onReply}
							onWithdraw={onWithdraw}
						/>
					))}
				</ul>
			)}
		</Section>
	);
}
