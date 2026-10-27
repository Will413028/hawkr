import { notFound } from "next/navigation";
import { HAWKR_AGENT_ENABLED } from "@/features/procurement-assistant/availability";
import {
	isRfqPrototypeScreenSlug,
	RFQ_PROTOTYPE_SCREEN_SLUGS,
} from "@/features/procurement-rfq-prototype/data";
import RfqPrototype from "@/features/procurement-rfq-prototype/RfqPrototype";

export const metadata = {
	title: { absolute: "Hawkr Agent v5.0｜RFQ 交易流程前端預覽" },
	robots: { index: false, follow: false },
};

export function generateStaticParams() {
	if (!HAWKR_AGENT_ENABLED) return [];
	return RFQ_PROTOTYPE_SCREEN_SLUGS.map((screen) => ({ screen }));
}

export default async function Page({
	params,
}: {
	params: Promise<{ screen: string }>;
}) {
	if (!HAWKR_AGENT_ENABLED) notFound();

	const { screen } = await params;
	if (!isRfqPrototypeScreenSlug(screen)) notFound();

	return <RfqPrototype screen={screen} />;
}
