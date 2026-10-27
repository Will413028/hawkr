export interface ConversationRangeInput {
	/** 這個分頁此刻**真的畫出來**幾列（已套用搜尋字串）。 */
	visible: number;
	/** 這個分頁載入了幾列（未套用搜尋字串）。恆 >= `visible`——兩個數字都由
	 * 同一個陣列推出來，`visible` 是它的子集。 */
	loaded: number;
	/**
	 * 這份清單有可能還沒抓完。人對人那邊是「pageSize 50 剛好拿滿」的推測，
	 * agent 那邊是後端明講的 `hasMore`；兩者都只證明「至少這麼多」。
	 */
	maybeTruncated: boolean;
}

/**
 * 側欄清單底部的計數（設計稿 01／03／04 的「顯示 1 - 6 / 6」）。
 *
 * 側欄沒有分頁器——載入的每一列都畫出來——所以範圍的起點恆為 1，終點就是看得
 * 到的列數。分母刻意是「這個分頁載入了幾列」而不是「總共幾列」：後者在 wire
 * 上沒有來源，猜一個數字填進去就是編造。清單可能還沒抓完時補一個 `+`，把句子
 * 從「就是這麼多」降級成「至少這麼多」，那才是這兩個訊號能撐得起的說法。
 *
 * 回 `null` ＝ 這一列不該出現：一列都看不到時空狀態已經在講話了，再加一句
 * 「顯示 1 - 0 / 6」既不是句子也和旁邊的文案打架。
 */
export function conversationRangeLabel({
	visible,
	loaded,
	maybeTruncated,
}: ConversationRangeInput): string | null {
	if (visible <= 0) return null;
	return `顯示 1 - ${visible} / ${loaded}${maybeTruncated ? "+" : ""}`;
}
