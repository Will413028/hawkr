import { notFound } from "next/navigation";
import { HAWKR_AGENT_ENABLED } from "@/features/procurement-assistant/availability";
import { rfqPrototypeScreenPath } from "@/features/procurement-rfq-prototype/data";
import { redirect } from "@/lib/intl/navigation";
import { localeFromParam } from "@/lib/seo/metadata";

export default async function Page({
	params,
}: {
	params: Promise<{ locale: string }>;
}) {
	if (!HAWKR_AGENT_ENABLED) notFound();
	redirect({
		href: rfqPrototypeScreenPath("candidate-handoff"),
		locale: localeFromParam((await params).locale),
	});
}
