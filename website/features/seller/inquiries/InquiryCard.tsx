"use client";

import { useTranslations } from "next-intl";
import Link from "@/components/shared/Link";
import { Card } from "@/components/ui/Card";
import type { SellerInquiry } from "@/lib/seller/inquiries";
import { InquiryChips, InquiryMeta } from "./InquiryContents";
import { inquiryHref } from "./inquiryDisplay";

/** 列表上的一張詢問卡：訊息摘要、種類與期限，整張連到詳情頁。 */
export default function InquiryCard({ inquiry }: { inquiry: SellerInquiry }) {
	const t = useTranslations("seller.inquiries");
	return (
		<Card
			as={Link}
			href={inquiryHref(inquiry)}
			isHoverable
			className="flex flex-col gap-2 p-5 no-underline"
		>
			<div className="flex flex-wrap items-start justify-between gap-3">
				<InquiryMeta inquiry={inquiry} />
				<InquiryChips inquiry={inquiry} />
			</div>
			<p className="m-0 line-clamp-3 whitespace-pre-wrap text-sm leading-[1.6] text-[var(--hkui-foreground)]">
				{inquiry.message}
			</p>
			<span className="text-xs font-bold text-[var(--hkui-primary)]">
				{inquiry.canReply ? t("viewAndReply") : t("viewDetail")}
			</span>
		</Card>
	);
}
