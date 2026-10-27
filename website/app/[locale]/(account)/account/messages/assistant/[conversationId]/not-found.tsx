import Link from "@/components/shared/Link";
import HawkrAgentMark from "@/features/procurement-assistant/HawkrAgentMark";
import { ASSISTANT_PROCUREMENT_PATH } from "@/features/procurement-assistant/routes";

export default function AssistantConversationNotFound() {
	return (
		<section
			className="thread-page assistant-route-state"
			aria-labelledby="assistant-not-found-title"
		>
			<HawkrAgentMark className="assistant-route-state__mark" hideBadge />
			<h1 id="assistant-not-found-title">找不到這個 Hawkr Agent 紀錄</h1>
			<p>這個對話網址無法載入。你可以開始新的對話，或回到訊息列表。</p>
			<div className="assistant-route-state__actions">
				{/* §V.10.12-4：唯一正式新對話入口。原本這裡還有一顆指向 legacy
				 * /assistant/new 的「開始新的找貨對話」，那條路現在會 redirect
				 * 回這裡，留著只是讓錯誤頁多一顆會繞路的按鈕。 */}
				<Link href={ASSISTANT_PROCUREMENT_PATH}>開始新的對話</Link>
				<Link href="/account/messages">返回訊息</Link>
			</div>
		</section>
	);
}
