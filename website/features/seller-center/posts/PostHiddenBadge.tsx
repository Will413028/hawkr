/** 賣家自己頁面被平台下架貼文的狀態 pill。沿用 AfterSaleStatusBadge 的 danger tone
 *  設計 token（禁硬編碼色），含 dark-mode 覆寫。
 *
 *  文字是「平台下架」不是「已下架」：CodeMeta 的 `4403` 標籤就叫「已下架」，指的是
 *  **店家自己撤下**（`PostUnpublishedBadge`），與這裡的 `hidden_at` 平台強制下架是
 *  兩回事。同一份列表裡兩顆 pill 共用一個詞，代行者就分不出「平台罰的」還是「店家
 *  自己撤的」—— 而那正是他要回報給賣家的那件事。 */
export function PostHiddenBadge() {
	return (
		<span className="inline-flex items-center gap-1.5 whitespace-nowrap rounded-full bg-danger-50 px-2 py-0.5 text-[0.6875rem] font-bold text-danger-700 dark:bg-[#b45644]/26 dark:text-[#e6b8a6]">
			<span
				className="inline-block h-1.5 w-1.5 rounded-full bg-danger-700 dark:bg-[#d49682]"
				aria-hidden="true"
			/>
			平台下架
		</span>
	);
}
