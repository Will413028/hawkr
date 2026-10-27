"use client";

import EmptyChatIcon from "@/features/messages/components/EmptyChatIcon";
import { chatWorkspaceRole } from "@/lib/chat/workspace";
import { usePathname } from "@/lib/intl/navigation";

interface Props {
	/**
	 * Legacy back-href kept for route compat — the topbar that used it
	 * was removed (chat now sits directly under the navbar with no
	 * intermediate "回帳戶總覽" link). Safe to delete from callers when
	 * convenient.
	 */
	backHref?: string;
	/**
	 * Legacy — the sidebar lives in `layout.tsx` now and owns its own
	 * href base, so this prop is no longer read. Kept on the signature
	 * to avoid breaking seller-center's call site mid-refactor.
	 */
	threadHrefBase?: string;
}

/**
 * Right-pane "no thread selected" content for `/account/messages`.
 * The surrounding `<main className="chat-page">` + `<div className="chat-shell">`
 * + sidebar live in `layout.tsx` so they persist across navigation.
 */
export default function MessagesClient(_: Props) {
	const pathname = usePathname() || "";
	const isSeller = chatWorkspaceRole(pathname) === "seller";

	return (
		<section className="chat-empty-panel" aria-label="訊息內容">
			<div className="chat-empty-panel__inner">
				<div className="chat-empty-panel__icon" aria-hidden="true">
					<EmptyChatIcon />
				</div>
				<h2 className="chat-empty-panel__title">選擇一則對話</h2>
				<p className="chat-empty-panel__copy">
					左側是所有聊天列表。選一位
					{isSeller ? "客戶" : "賣家"}後，對話會在這裡展開。
				</p>
			</div>
		</section>
	);
}
