import AssistantWorkspace from "@/features/procurement-assistant/AssistantWorkspace";
import { HAWKR_AGENT_ENABLED } from "@/features/procurement-assistant/availability";
import { readAgentCapabilities } from "@/features/procurement-assistant/capabilities.server";
import { redirect } from "@/lib/intl/navigation";
import { localeFromParam } from "@/lib/seo/metadata";

export const metadata = { title: { absolute: "Hawkr Agent｜採購經理" } };

/**
 * Hawkr Agent 的**唯一**正式新對話入口（設計書 v6 §V.10.12-4）。
 *
 * capability flags 在這裡（Server Component）解析後往下傳，不由 client 讀
 * `NEXT_PUBLIC_*`——§V.13 規則 6 明說那是 build-time 值、不是即時 kill switch。
 */
export default async function Page({
	params,
	searchParams,
}: {
	params: Promise<{ locale: string }>;
	searchParams: Promise<{ requestId?: string }>;
}) {
	if (!HAWKR_AGENT_ENABLED) {
		redirect({
			href: "/account/messages",
			locale: localeFromParam((await params).locale),
		});
	}

	const { requestId } = await searchParams;
	return (
		<AssistantWorkspace
			initialRequestId={requestId ?? null}
			capabilities={readAgentCapabilities()}
		/>
	);
}
