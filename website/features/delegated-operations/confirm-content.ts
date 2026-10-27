import { PRODUCT_STATUS_LABEL } from "@/lib/codemeta/enums";
import type { SellerMutationChange } from "@/lib/seller-data/scope";

/**
 * 代行確認抽屜的內容組法（設計書 §6.5、§4.1 的 `ADMIN_PRODUCT_PUBLISH`）。
 *
 * 抽屜是不可逆寫入前的最後一道人為關卡，設計書對它有兩條逐字要求：
 * ① 提示卡寫的是**目標店家的店名**（「以 ADM 身分替梨山盛花園上架商品」），不是一個
 *    代名詞 —— 代行事故的典型形狀是「按錯店」，而抽屜是最後一個攔得住它的地方
 * ② `ADMIN_PRODUCT_PUBLISH` 風險層級為高，確認要求是「diff＋二次確認＋通知」，
 *    diff 不因為只有一個欄位而免除
 *
 * 兩件事都只在「組字串」這一層，抽在這裡是為了讓它們各自有一支測得到的判準。
 */

/**
 * 目標店家在確認文案裡的稱呼。
 *
 * ⚠️ **拿不到店名時不得退回「本店」**：在一個「替別人操作」的畫面上，「本店」既沒有
 * 指出是哪一家，讀起來還像是管理員自己的店 —— 那正是這個欄位要防的誤讀。退到店家
 * 編號雖然不好讀，但它唯一指向一家店，操作者至少對得起情境列上的那一串數字；兩者都
 * 沒有時就明說「無法確認」，讓人停下來，而不是給一句讀得通的空話。
 *
 * 實務上 `storeName` 幾乎恆有值（`readDelegatedSessionSummary` 少了它整份 summary 就
 * 判為無效），所以退路的意義在於「summary 還沒回來就開了抽屜」這種時序，而不是常態。
 */
export function targetStoreLabel(
	storeName: string | null | undefined,
	storeId: string | null | undefined,
): string {
	const name = (storeName ?? "").trim();
	if (name) return name;
	const id = (storeId ?? "").trim();
	return id ? `店家 ${id}` : "無法確認的店家";
}

/**
 * 上／下架的變更列（設計書 §6.5「before／after」）。
 *
 * `/publish`／`/unpublish` 改變的恰好是一個欄位，所以這張表只有一列 —— 但**一列不是
 * 零列**：全 repo 風險最高的寫入，確認關卡上必須看得到「這一按會把什麼變成什麼」。
 *
 * ⚠️ 這裡**不列庫存**。mockup 把「改庫存」與「上架」畫成同一個確認動作，實際是兩支
 * 端點、兩個 capability、兩筆稽核列（B5）—— 把庫存畫進來等於承諾一次不會發生的寫入。
 *
 * `before` 優先用後端下發的 `statusLabel`（與稽核列的欄位值同一份字串），後端沒送才
 * 查 CodeMeta；`after` 只能查 CodeMeta，那一刻還沒有伺服器回應。
 */
export function shelfStatusChange(
	current: { status: string; statusLabel?: string },
	target: "4003" | "4004",
): SellerMutationChange {
	return {
		field: "發布狀態",
		before:
			current.statusLabel ||
			PRODUCT_STATUS_LABEL[current.status] ||
			current.status,
		after: PRODUCT_STATUS_LABEL[target] ?? target,
	};
}
