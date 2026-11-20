"use client";

import { timestampDate } from "@bufbuild/protobuf/wkt";
import { useTranslations } from "next-intl";
import { Chip } from "@/components/ui/Chip";
import { formatTaipeiDateTime } from "@/lib/date/taipei";
import type { SellerInquiry } from "@/lib/seller/inquiries";
import {
	inquiryBody,
	inquiryChannelKey,
	inquiryDeadline,
	inquiryKindKey,
	inquiryStateColor,
	inquiryStateKey,
	replyIsOffSystem,
} from "./inquiryDisplay";

/** 狀態標籤與逾期標記。 */
export function InquiryChips({ inquiry }: { inquiry: SellerInquiry }) {
	const t = useTranslations("seller.inquiries");
	const stateKey = inquiryStateKey(inquiry.state);
	const deadline = inquiryDeadline(inquiry, new Date());
	return (
		<span className="inline-flex flex-wrap items-center gap-1.5">
			{stateKey && (
				<Chip variant="flat" color={inquiryStateColor(inquiry.state)} size="sm">
					{t(`state.${stateKey}`)}
				</Chip>
			)}
			{deadline.kind === "overdue" && (
				<Chip variant="flat" color="danger" size="sm">
					{t("deadline.overdueChip")}
				</Chip>
			)}
		</span>
	);
}

/** 種類、發出時間與期限的說明行。 */
export function InquiryMeta({ inquiry }: { inquiry: SellerInquiry }) {
	const t = useTranslations("seller.inquiries");
	const kindKey = inquiryKindKey(inquiry.kind);
	const deadline = inquiryDeadline(inquiry, new Date());
	const parts = [
		kindKey ? t(`kind.${kindKey}`) : null,
		inquiry.createTime
			? t("createdAt", {
					time: formatTaipeiDateTime(timestampDate(inquiry.createTime)),
				})
			: null,
		deadline.kind === "due"
			? t("deadline.due", { time: formatTaipeiDateTime(deadline.due) })
			: null,
		deadline.kind === "overdue"
			? t("deadline.overdue", { time: formatTaipeiDateTime(deadline.due) })
			: null,
	].filter((part): part is string => part !== null);
	return (
		<p className="m-0 text-xs text-[var(--hkui-foreground-muted)]">
			{parts.join(" · ")}
		</p>
	);
}

/** 已回覆的內容：回覆內文、管道、時間與附件數（附件是機敏檔，只顯示數量）。 */
export function InquiryReplyView({ inquiry }: { inquiry: SellerInquiry }) {
	const t = useTranslations("seller.inquiries");
	const reply = inquiry.reply;
	if (!reply) return null;
	const channelKey = inquiryChannelKey(reply.channel);
	const offSystem = replyIsOffSystem(inquiry);
	const channel = channelKey ? t(`channel.${channelKey}`) : null;
	const notes = [
		offSystem && channel ? t("reply.offSystem", { channel }) : null,
		!offSystem ? t("reply.viaSellerCenter") : null,
		reply.receiveTime
			? t("reply.receivedAt", {
					time: formatTaipeiDateTime(timestampDate(reply.receiveTime)),
				})
			: null,
		reply.attachmentCount > 0
			? t("reply.attachmentCount", { count: reply.attachmentCount })
			: null,
	].filter((note): note is string => note !== null);
	return (
		<div className="flex flex-col gap-2">
			<strong className="text-sm text-[var(--hkui-foreground)]">
				{t("reply.heading")}
			</strong>
			<p className="m-0 whitespace-pre-wrap text-sm leading-[1.6] text-[var(--hkui-foreground-soft)]">
				{reply.body}
			</p>
			{notes.map((note) => (
				<p
					key={note}
					className="m-0 text-xs text-[var(--hkui-foreground-muted)]"
				>
					{note}
				</p>
			))}
		</div>
	);
}

/** 撤回後唯讀的說明。 */
export function InquiryWithdrawnNote({ inquiry }: { inquiry: SellerInquiry }) {
	const t = useTranslations("seller.inquiries");
	if (inquiryBody(inquiry) !== "withdrawn") return null;
	return (
		<p className="m-0 text-sm text-[var(--hkui-foreground-soft)]">
			{inquiry.withdrawTime
				? t("withdrawnAt", {
						time: formatTaipeiDateTime(timestampDate(inquiry.withdrawTime)),
					})
				: t("withdrawn")}
		</p>
	);
}
