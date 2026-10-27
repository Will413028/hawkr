import type { SseConnectionStatus } from "@/lib/sse/client";

/**
 * 訂單頁的「即時同步已暫停」狀態列（backend#2040 問題三）。
 *
 * 根 layout 的通知串流（`NotificationUnreadProvider`）在兩種情況會停止供應更新，且
 * **兩種都不會報錯**：連線數達上限被 gateway 以 429 回絕、或斷線超過 30 秒的
 * 重連預算（`lib/sse/client.ts` 的 `unavailable`）；以及瀏覽器離線
 * （`offline`）。頁面本身仍然正常渲染既有資料，於是賣家看到的是一個「安靜地
 * 不再更新」的訂單列——會誤判成沒有新訂單，而不是察覺同步壞了。功能靜默消失
 * 沒有人會回報，這一行就是把它變成看得見的狀態。
 *
 * 只在這兩個「確定收不到更新」的狀態顯示；`connecting` / `idle` 是過渡狀態，
 * 掛上提示只會製造閃爍雜訊。沿用站內既有的 warning tone
 * （`AfterSaleStatusBadge` 同一組 token 與 dark-mode 覆寫），不新增元件語彙。
 */
const MESSAGES: Partial<Record<SseConnectionStatus, string>> = {
	unavailable:
		"即時更新已暫停，新訂單與狀態變更不會自動出現。請按上方「重新整理」取得最新資料。",
	offline: "網路連線中斷，即時更新已暫停。恢復連線後會自動繼續同步。",
};

export function RealtimeSyncNotice({
	status,
}: {
	status: SseConnectionStatus;
}) {
	const message = MESSAGES[status];
	if (!message) return null;
	return (
		<p
			role="status"
			className="m-0 flex items-center gap-2 rounded-hkui-md bg-warning-50 px-3.5 py-2.5 text-[0.8125rem] font-semibold text-warning-700 dark:bg-[#a9842c]/26 dark:text-[#d6c27e]"
		>
			<span
				className="inline-block h-1.5 w-1.5 shrink-0 rounded-full bg-warning-700 dark:bg-[#d6c27e]"
				aria-hidden="true"
			/>
			{message}
		</p>
	);
}
