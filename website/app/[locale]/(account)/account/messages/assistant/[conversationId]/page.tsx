import { notFound } from "next/navigation";
import AssistantWorkspace from "@/features/procurement-assistant/AssistantWorkspace";
import { HAWKR_AGENT_ENABLED } from "@/features/procurement-assistant/availability";
import { readAgentCapabilities } from "@/features/procurement-assistant/capabilities.server";
import {
	DEMO_CONVERSATION_ID,
	isAssistantConversationId,
} from "@/features/procurement-assistant/routes";
import { redirect } from "@/lib/intl/navigation";
import { localeFromParam } from "@/lib/seo/metadata";

export const metadata = { title: { absolute: "Hawkr Agent｜訊息" } };

export default async function Page({
	params,
}: {
	params: Promise<{ locale: string; conversationId: string }>;
}) {
	const { locale, conversationId } = await params;
	if (!HAWKR_AGENT_ENABLED) {
		redirect({
			href: "/account/messages",
			locale: localeFromParam(locale),
		});
	}

	const capabilities = readAgentCapabilities();
	if (conversationId === DEMO_CONVERSATION_ID) {
		return (
			<AssistantWorkspace
				conversationId={conversationId}
				capabilities={capabilities}
			/>
		);
	}
	if (isAssistantConversationId(conversationId)) {
		return (
			<AssistantWorkspace
				initialConversationId={conversationId}
				capabilities={capabilities}
			/>
		);
	}
	notFound();
}
