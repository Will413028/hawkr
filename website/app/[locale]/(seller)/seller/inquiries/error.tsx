"use client";

import { useTranslations } from "next-intl";
import ErrorState from "@/components/shared/ErrorState";

/**
 * 案件詢問頁的 error boundary：不在賣家中心外框裡，錯誤停在這一段，重試或回詢問列表。
 */
export default function SellerInquiriesError({
	error,
	reset,
}: {
	error: Error & { digest?: string };
	reset: () => void;
}) {
	const t = useTranslations("seller.inquiries");
	return (
		<ErrorState
			error={error}
			reset={reset}
			title={t("error.title")}
			description={t("error.description")}
			homeHref="/seller/inquiries"
			homeLabel={t("error.home")}
		/>
	);
}
