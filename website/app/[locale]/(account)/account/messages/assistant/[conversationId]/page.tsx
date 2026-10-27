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
	// legacy 的 "new" segment 不會走到這裡：§V.10.12-4 的退場改在 next.config.ts
	// 的 redirects() 做（HTTP 層 307），因為 cacheComponents 下 PPR 會先把 shell
	// 串出去，server-component redirect() 只能降級成 client-side 導向、看得到閃爍。
	// 走到這裡的 "new" 會被下面的 isAssistantConversationId 判成非法 segment 而 404，
	// 那是正確的——它代表 config redirect 沒生效，不該安靜地回一個可用畫面。
	// 真實對話 id（TSID）→ 交給 workspace 從 transcript 端點還原；
	// 不屬於本人或不存在時後端回 404，workspace 落到 RESTORE_FAILED。
	// 非數字的垃圾 segment 在這裡就 404，不用打後端。
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
