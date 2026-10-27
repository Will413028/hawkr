"use client";

import AppIcon from "@/components/shared/AppIcon";
import Link from "@/components/shared/Link";
import { Button } from "@/components/ui";

export default function AssistantConversationError({
	reset,
}: {
	error: Error & { digest?: string };
	reset: () => void;
}) {
	return (
		<section
			className="thread-page assistant-route-state"
			aria-labelledby="assistant-error-title"
		>
			<span className="assistant-route-state__mark" aria-hidden="true">
				<AppIcon name="warning" size={26} />
			</span>
			<h1 id="assistant-error-title">Hawkr Agent 暫時無法載入</h1>
			<p>你的操作沒有送出。可以重新載入；若仍失敗，先返回訊息再試一次。</p>
			<div className="assistant-route-state__actions">
				<Button onClick={reset}>重新載入</Button>
				<Link href="/account/messages">返回訊息</Link>
			</div>
		</section>
	);
}
