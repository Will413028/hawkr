"use client";

import { useTranslations } from "next-intl";
import * as React from "react";
import ReportActionMenu from "@/features/report/ReportActionMenu";
import { blockPeerByMessage } from "@/lib/chat/block";
import type { ChatSide } from "@/lib/chat/types";

/**
 * 對方訊息旁的「⋯」：檢舉這則訊息（`CHAT_MESSAGE`），或封鎖對話的另一方（以訊息 ID 當參照）。
 * 買家封鎖的是店家，店家封鎖的是買家，確認說明不同。
 */
export default function ChatMessageActions({
	side,
	messageId,
	peerName,
	onBlocked,
}: {
	side: ChatSide;
	messageId: string;
	peerName: string;
	onBlocked?: () => void;
}) {
	const tReport = useTranslations("report");
	const tSeller = useTranslations("seller.comments");
	const block = React.useMemo(
		() => ({
			name: peerName,
			description:
				side === "store"
					? tSeller("blockDescription")
					: tReport("blockStoreDescription"),
			run: () => blockPeerByMessage(side, messageId),
			onBlocked,
		}),
		[side, messageId, peerName, onBlocked, tReport, tSeller],
	);
	return (
		<span className="absolute left-full top-1/2 ml-1 -translate-y-1/2">
			<ReportActionMenu
				target={{ kind: "chatMessage", id: messageId }}
				block={block}
			/>
		</span>
	);
}
