"use client";

import { useTranslations } from "next-intl";
import Link from "@/components/shared/Link";
import { SellerPageShell } from "@/components/shared/SellerPageShell";
import Button from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { SellerCardGridSkeleton } from "@/features/seller-center/components/SellerSkeletons";
import { useSellerInquiry } from "@/lib/hooks/useSellerInquiries";
import {
	InquiryChips,
	InquiryMeta,
	InquiryReplyView,
	InquiryWithdrawnNote,
} from "./InquiryContents";
import InquiryReplyForm from "./InquiryReplyForm";
import { INQUIRIES_PATH, inquiryBody } from "./inquiryDisplay";

/**
 * 單筆詢問（`/seller/inquiries/[inquiryId]`）：平台的訊息與期限，能回覆時顯示回覆表單，已回覆與已撤回唯讀。
 * 能不能回覆讀伺服器的 `canReply`；逾期後仍可回覆。
 */
export default function SellerInquiryDetailPage({
	inquiryId,
}: {
	inquiryId: string;
}) {
	const t = useTranslations("seller.inquiries");
	const { inquiry, loading, error, notFound, refetch } =
		useSellerInquiry(inquiryId);

	const backLink = (
		<Link
			href={INQUIRIES_PATH}
			className="text-sm font-bold text-[var(--hkui-primary)]"
		>
			{t("backToList")}
		</Link>
	);

	if (!inquiry) {
		if (notFound) {
			return (
				<SellerPageShell title={t("detailTitle")} actions={backLink}>
					<Card className="flex flex-col items-center gap-3 p-6 text-center">
						<strong className="text-[0.9375rem] text-[var(--hkui-foreground)]">
							{t("notFound.title")}
						</strong>
						<p className="m-0 text-sm text-[var(--hkui-foreground-soft)]">
							{t("notFound.description")}
						</p>
					</Card>
				</SellerPageShell>
			);
		}
		if (error) {
			return (
				<SellerPageShell title={t("detailTitle")} actions={backLink}>
					<Card className="flex flex-col items-center gap-3 p-6 text-center">
						<strong className="text-[0.9375rem] text-[var(--hkui-foreground)]">
							{t("loadFailed.title")}
						</strong>
						<p className="m-0 text-sm text-[var(--hkui-foreground-soft)]">
							{error}
						</p>
						<Button
							variant="bordered"
							color="default"
							onClick={() => void refetch()}
						>
							{t("retry")}
						</Button>
					</Card>
				</SellerPageShell>
			);
		}
		return (
			<SellerPageShell title={t("detailTitle")} actions={backLink}>
				{loading ? <SellerCardGridSkeleton variant="panel" count={1} /> : null}
			</SellerPageShell>
		);
	}

	const body = inquiryBody(inquiry);
	return (
		<SellerPageShell title={t("detailTitle")} actions={backLink}>
			<Card className="flex flex-col gap-3 p-5">
				<div className="flex flex-wrap items-start justify-between gap-3">
					<InquiryMeta inquiry={inquiry} />
					<InquiryChips inquiry={inquiry} />
				</div>
				<strong className="text-sm text-[var(--hkui-foreground)]">
					{t("messageHeading")}
				</strong>
				<p className="m-0 whitespace-pre-wrap text-sm leading-[1.6] text-[var(--hkui-foreground-soft)]">
					{inquiry.message}
				</p>
				{body === "replied" && <InquiryReplyView inquiry={inquiry} />}
				<InquiryWithdrawnNote inquiry={inquiry} />
			</Card>
			{body === "form" && <InquiryReplyForm inquiry={inquiry} />}
		</SellerPageShell>
	);
}
