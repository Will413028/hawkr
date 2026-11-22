/**
 * 從聊天訊息封鎖對方：以訊息 ID 當參照，伺服器從訊息解出對話的另一方。
 * 買家封鎖對話的店家（`BuyerAppService.BlockAuthor`），店家封鎖對話的買家（`SellerAppService.BlockBuyer`）。
 * 本來就封鎖時也成功（`changed` 是 false），所以重複按不是錯誤；訊息已隱藏或抹除仍可當參照。
 */

import { buyerAppClient } from "@/lib/connect/buyer";
import { sellerAppClient } from "@/lib/connect/seller";
import type { ChatSide } from "./types";

export interface BlockClients {
	buyer: () => ReturnType<typeof buyerAppClient>;
	seller: () => ReturnType<typeof sellerAppClient>;
}

const DEFAULT_CLIENTS: BlockClients = {
	buyer: buyerAppClient,
	seller: sellerAppClient,
};

export async function blockPeerByMessage(
	side: ChatSide,
	messageId: string,
	clients: BlockClients = DEFAULT_CLIENTS,
): Promise<{ changed: boolean }> {
	const source = { case: "messageId" as const, value: messageId };
	const res =
		side === "store"
			? await clients.seller().blockBuyer({ source })
			: await clients.buyer().blockAuthor({ source });
	return { changed: res.changed };
}
