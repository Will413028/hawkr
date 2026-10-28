import type { ProcurementCandidate } from "@/lib/api/openapi-types";

/**
 * 候選按商家收斂（B-246 / backend #2377）。
 *
 * 產品要的是「找到 N 家商家」。在後端補上 `storeId` 之前，網頁拿到的是一串沒有歸屬的
 * 商品，只能把商品名當成商家名列出來——那正是 #2377 內文說的「標題寫『找到的商家』、
 * 卡片卻是『綜合生菜300克』」。
 *
 * ## 為什麼是「收斂成商家」而不是「商家底下掛商品」
 *
 * 候選的單位是**收件人**不是品項（#2377 是 #2450 `DIRECT_RECIPIENTS` 的前置）。而且
 * EXTERNAL lane 的候選來自站外名單、根本沒有平台商品，商品層的呈現對目前的主要 lane
 * 全是空的。買家在這個階段要決定的是「找哪幾家問」，不是「買哪一個 SKU」。
 *
 * 價格同理不在這裡：買家看到的是自己設定的希望詢價區間，不是某一個平台商品的
 * 標價——設計書 57 §1.1-2 只認四種價格事實，商品標價不在其中。
 *
 * ## 為什麼收斂是「資料驅動」而不是一個模式開關
 *
 * 這個檔案存在的第一原因是**不要做出「看起來有、點進去空」的東西**。`3319de8b` 曾經
 * 刻意刪掉一個 disabled 的「預覽」分頁，就是為了這件事。
 *
 * 所以這裡不提供任何開關：**候選實際帶得出商家身分才收斂，帶不出就退回原本的平表**。
 * 這個判準自動涵蓋三種情況，而且不需要 feature flag：
 *
 *   1. backend 尚未部署 #2377 —— 所有候選都沒有 storeId → 不收斂。
 *   2. 部署窗口內新舊 pod 並存 —— 同一次查詢可能混到有／沒有的候選 → 見 `ungrouped`。
 *   3. EXTERNAL lane —— 站外業者沒有平台賣家，`displayName` 本來就是商號名，維持原樣。
 */

/** 一家商家，以及它底下被合併掉的候選。 */
export interface CandidateSellerGroup {
	storeId: string;
	/** 缺店名時退回「商家 {storeId}」，不留空字串讓標題整個消失。 */
	sellerName: string;
	/** 缺 storeCode 時為 null：不得用 storeId 組連結，該路徑正在被收掉。 */
	storeCode: string | null;
	logoUrl: string | null;
	/**
	 * 這家商家底下的候選。UI 只顯示筆數，不列出來；勾選這一列＝勾選這裡的全部候選
	 * （詢價單的收件人是商家，一家只需要問一次）。
	 */
	candidates: ProcurementCandidate[];
	/**
	 * 合併後要顯示的查核結果，取群組內所有候選的**聯集**（依 key 去重）。
	 *
	 * 後端目前對同一次搜尋的每一筆 PLATFORM 候選算出的 matched/unverifiable 完全相同
	 * （`candidate_platform_lane.go` 只依 slots 計算、不依 item），所以聯集等於任一筆。
	 * 用聯集而不是「取第一筆」是為了讓後端日後改成逐候選判定時，這裡仍然誠實。
	 */
	matchedFacts: ProcurementCandidate["matchedFacts"];
	unverifiableFacts: ProcurementCandidate["unverifiableFacts"];
	/** 群組內最新的資料時間。 */
	factsAsOf: string;
	/** 群組內最高的比對分數（同上，目前每筆相同）。 */
	score: number;
}

export interface GroupedPlatformCandidates {
	/**
	 * 依「首次出現順序」排列的商家。刻意不重排：候選本身已由後端依 rank 排好，
	 * 重排會讓最相關的商家在視覺上被推到下面。
	 */
	groups: CandidateSellerGroup[];
	/**
	 * 帶不出商家身分的候選（部署窗口內的舊資料）。這些**不會被丟掉**，會接在商家列
	 * 後面以原本的候選列呈現——靜默丟棄候選是比不收斂更糟的失敗。
	 */
	ungrouped: ProcurementCandidate[];
}

/**
 * 判斷這批候選是否帶得出商家身分。
 *
 * 判準刻意寬鬆（**任一筆**帶得出 storeId 就算數）而不是「全部都要有」：部署窗口內
 * 一半新一半舊時，能收斂的先收斂、不能的照樣完整列出，比整批退回平表更貼近使用者
 * 要的東西。
 */
export function canGroupBySeller(
	candidates: readonly ProcurementCandidate[],
): boolean {
	return candidates.some((candidate) => Boolean(candidate.storeId));
}

/**
 * 把 PLATFORM lane 的候選收斂成商家。
 *
 * 同一家商家的多個商品必然落在同一組——那是後端 #2377 驗收條件 2 的前端對應面：
 * 後端保證同店候選的 storeId 相同，這裡就以它為唯一分組鍵，不做任何名稱比對之類的
 * 模糊歸併（店名可以重複，storeId 不會）。
 */
export function groupPlatformCandidates(
	candidates: readonly ProcurementCandidate[],
): GroupedPlatformCandidates {
	const byStore = new Map<string, CandidateSellerGroup>();
	const ungrouped: ProcurementCandidate[] = [];

	for (const candidate of candidates) {
		const storeId = candidate.storeId;
		if (!storeId) {
			ungrouped.push(candidate);
			continue;
		}

		const existing = byStore.get(storeId);
		if (!existing) {
			byStore.set(storeId, {
				storeId,
				sellerName: candidate.sellerName || `商家 ${storeId}`,
				storeCode: candidate.storeCode || null,
				logoUrl: candidate.logoUrl || null,
				candidates: [candidate],
				matchedFacts: [...candidate.matchedFacts],
				unverifiableFacts: [...candidate.unverifiableFacts],
				factsAsOf: candidate.factsAsOf,
				score: candidate.score,
			});
			continue;
		}

		existing.candidates.push(candidate);
		mergeByKey(existing.matchedFacts, candidate.matchedFacts);
		mergeByKey(existing.unverifiableFacts, candidate.unverifiableFacts);
		if (candidate.factsAsOf > existing.factsAsOf) {
			existing.factsAsOf = candidate.factsAsOf;
		}
		if (candidate.score > existing.score) existing.score = candidate.score;
	}

	return { groups: [...byStore.values()], ungrouped };
}

/** 依 `key` 去重地把 incoming 併進 target（就地修改，維持首次出現順序）。 */
function mergeByKey<T extends { key: string }>(
	target: T[],
	incoming: readonly T[],
): void {
	for (const item of incoming) {
		if (target.some((existing) => existing.key === item.key)) continue;
		target.push(item);
	}
}

/**
 * 一家商家的勾選狀態。
 *
 * 三態而不是布林：部分勾選必須表達得出來，否則使用者對「這家我選了嗎」得不到正確答案。
 * 對應到 `<input type="checkbox">` 的 `indeterminate`。
 */
export type SellerSelectionState = "none" | "partial" | "all";

export function sellerSelectionState(
	group: CandidateSellerGroup,
): SellerSelectionState {
	const selected = group.candidates.filter((c) => c.selected).length;
	if (selected === 0) return "none";
	return selected === group.candidates.length ? "all" : "partial";
}

/**
 * 勾選一家商家時，實際要對哪些候選發出切換。
 *
 * 詢價單的收件人是商家，所以「選這家」＝把它底下所有候選一起帶進去。回傳的是**需要
 * 改變狀態的那幾筆**，已經是目標狀態的不重送——每一筆都會打一次 toggle API，重送等於
 * 白白製造請求並讓 pending 狀態閃爍。
 */
export function candidatesToToggleForSeller(group: CandidateSellerGroup): {
	candidates: ProcurementCandidate[];
	nextSelected: boolean;
} {
	// 部分勾選時往「全選」走：使用者點一個非空的框，預期是補齊而不是清空。
	const nextSelected = sellerSelectionState(group) !== "all";
	return {
		candidates: group.candidates.filter((c) => c.selected !== nextSelected),
		nextSelected,
	};
}

/**
 * 已勾選的候選代表幾個**收件人**。
 *
 * 底部行動列講的是「已選 N 家」，那個 N 必須跟畫面上被勾起來的列數對得起來：
 * PLATFORM lane 的列已經收斂成商家（見上方 `groupPlatformCandidates`），同一家底下
 * 選了三個候選在畫面上只是一列，用候選筆數會報成 3。
 *
 * 帶不出 `storeId` 的候選各自算一家——EXTERNAL lane 的站外業者本來就是一家一列，
 * 部署窗口內混進來的舊候選也走 `ungrouped` 各自成列，兩者的畫面單位都是「一列一家」。
 *
 * 只餵已勾選的候選；這個函式不自己過濾 `selected`，因為呼叫端手上本來就有那份陣列。
 */
export function countSelectedRecipients(
	candidates: readonly ProcurementCandidate[],
): number {
	const stores = new Set<string>();
	let standalone = 0;
	for (const candidate of candidates) {
		if (candidate.storeId) stores.add(candidate.storeId);
		else standalone += 1;
	}
	return stores.size + standalone;
}
