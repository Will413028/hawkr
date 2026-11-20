"use client";

import { sellerInquiryState } from "@hawkr/ui-text/keys";
import { useTranslations } from "next-intl";
import * as React from "react";
import { SellerPageShell } from "@/components/shared/SellerPageShell";
import { Card } from "@/components/ui/Card";
import { Tabs } from "@/components/ui/Tabs";
import { SellerEmptyState } from "@/features/seller-center/components/SellerEmptyState";
import { SellerCardGridSkeleton } from "@/features/seller-center/components/SellerSkeletons";
import { useSellerInquiries } from "@/lib/hooks/useSellerInquiries";
import { optionalText, useUiText } from "@/lib/i18n/ui-text";
import InquiryCard from "./InquiryCard";
import {
	INQUIRY_FILTERS,
	type InquiryFilter,
	inquiryFilterState,
} from "./inquiryDisplay";

/**
 * 平台詢問列表（`/seller/inquiries`）：依狀態篩選，每張卡連到詳情頁回覆。詢問以帳號為範圍，不在賣家中心外框裡：
 * 店家已關閉的帳號一樣讀得到、回得了。
 */
export default function SellerInquiriesPage() {
	const t = useTranslations("seller.inquiries");
	const ut = useUiText();
	const [filter, setFilter] = React.useState<InquiryFilter>("all");
	const { records, pendingReplyCount, loading, error } = useSellerInquiries(
		inquiryFilterState(filter),
	);

	return (
		<SellerPageShell
			title={t("title")}
			subtitle={
				pendingReplyCount > 0
					? t("pendingSummary", { count: pendingReplyCount })
					: t("subtitle")
			}
			toolbar={
				<Tabs
					aria-label={t("filterLabel")}
					variant="light"
					size="sm"
					selectedKey={filter}
					onSelectionChange={(key) => setFilter(key as InquiryFilter)}
					items={INQUIRY_FILTERS.map((key) => ({
						key,
						label:
							key === "all"
								? t("filter.all")
								: (optionalText(
										ut,
										sellerInquiryState(inquiryFilterState(key)),
									) ?? key),
					}))}
				/>
			}
		>
			{loading ? (
				<SellerCardGridSkeleton
					variant="panel"
					count={3}
					className="flex flex-col gap-[var(--hkui-space-4)]"
				/>
			) : error ? (
				<Card className="p-6 text-center text-sm text-[var(--hkui-foreground-soft)]">
					{error}
				</Card>
			) : records.length === 0 ? (
				<SellerEmptyState
					variant="todos"
					title={t("empty.title")}
					description={t("empty.description")}
				/>
			) : (
				<div className="flex flex-col gap-[var(--hkui-space-4)]">
					{records.map((inquiry) => (
						<InquiryCard key={inquiry.id} inquiry={inquiry} />
					))}
				</div>
			)}
		</SellerPageShell>
	);
}
