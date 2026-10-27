import { notFound } from "next/navigation";
import { HAWKR_AGENT_ENABLED } from "@/features/procurement-assistant/availability";
import {
	EXTERNAL_VENDOR_STEP_SLUGS,
	isExternalVendorStep,
} from "@/features/procurement-rfq-prototype/data";
import ExternalVendorPortal from "@/features/procurement-rfq-prototype/ExternalVendorPortal";

export const metadata = {
	title: "外部供應商交易入口",
	robots: { index: false, follow: false },
};

export function generateStaticParams() {
	if (!HAWKR_AGENT_ENABLED) return [];
	return EXTERNAL_VENDOR_STEP_SLUGS.filter((step) => step !== "invitation").map(
		(step) => ({ step }),
	);
}

export default async function Page({
	params,
}: {
	params: Promise<{ step: string }>;
}) {
	if (!HAWKR_AGENT_ENABLED) notFound();

	const { step } = await params;
	if (!isExternalVendorStep(step) || step === "invitation") notFound();

	return <ExternalVendorPortal step={step} />;
}
