/**
 * keyset 清單（ListAccounts、ListStores 等）的翻頁（AIP-158）：page token 不透明、只能往下一頁拿，所以記住走過的每一頁的 token，「上一頁」
 * 就是退回前一個。tokens[i] 是第 i+1 頁的 token，第一頁是空字串；目前頁碼是 tokens.length。token 綁定篩選條件，
 * 換條件時回到 firstPage()。
 */
export type PageTokens = readonly string[];

export function firstPage(): PageTokens {
	return [""];
}

export function currentPage(tokens: PageTokens): number {
	return tokens.length;
}

export function currentToken(tokens: PageTokens): string {
	return tokens[tokens.length - 1] ?? "";
}

/** 往下一頁；沒有下一頁（nextPageToken 是空字串）時不動。 */
export function nextPage(
	tokens: PageTokens,
	nextPageToken: string,
): PageTokens {
	return nextPageToken ? [...tokens, nextPageToken] : tokens;
}

/** 回上一頁；已經在第一頁時不動。 */
export function previousPage(tokens: PageTokens): PageTokens {
	return tokens.length > 1 ? tokens.slice(0, -1) : tokens;
}

export function pageCount(totalSize: number, pageSize: number): number {
	return Math.ceil(totalSize / pageSize);
}
