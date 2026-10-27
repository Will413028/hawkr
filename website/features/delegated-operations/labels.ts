import type {
	DelegatedCapability,
	DelegatedSessionState,
} from "./state/session";

/**
 * 情境列的 capability chips 是**唯讀展示**（mockup §3.2）：mockup 用五組標籤表達
 * 封閉九值（`店面管理`／`商品編輯`／`商品上架 / 下架`／`動態內容`／`訂單唯讀`），
 * 而後端下發的是九個機器名。
 *
 * 一組的成員**全數核發**才畫該組的 chip；只核發一半時退回逐個 capability 的
 * 標籤，避免一顆 chip 對操作者宣稱他其實沒有的權限。認不得的 capability 直接
 * 略過（不畫、不報錯）——後端擴充第 10 個值時，工作區只會少一顆 chip。
 */
const CAPABILITY_GROUPS: { label: string; members: DelegatedCapability[] }[] = [
	{ label: "店面管理", members: ["storefront.read", "storefront.write"] },
	{ label: "商品編輯", members: ["products.read", "products.write"] },
	{ label: "商品上架 / 下架", members: ["products.publish"] },
	{
		label: "動態內容",
		members: ["posts.read", "posts.write", "posts.publish"],
	},
	{ label: "訂單唯讀", members: ["orders.read"] },
];

const CAPABILITY_LABELS: Record<string, string> = {
	"storefront.read": "查看店面內容",
	"storefront.write": "編輯店面內容",
	"products.read": "查看商品",
	"products.write": "編輯商品",
	"products.publish": "上架與下架商品",
	"posts.read": "查看貼文",
	"posts.write": "編輯貼文",
	"posts.publish": "發布與下架貼文",
	"orders.read": "查看訂單（唯讀）",
};

export function delegatedCapabilityChipLabels(
	capabilities: readonly DelegatedCapability[],
): string[] {
	const granted = new Set(capabilities);
	const chips: string[] = [];
	const covered = new Set<DelegatedCapability>();
	for (const group of CAPABILITY_GROUPS) {
		if (!group.members.every((member) => granted.has(member))) continue;
		chips.push(group.label);
		for (const member of group.members) covered.add(member);
	}
	for (const capability of capabilities) {
		if (covered.has(capability)) continue;
		const label = CAPABILITY_LABELS[capability];
		if (label) chips.push(label);
	}
	return chips;
}

export function delegatedSessionStateCopy(state: DelegatedSessionState): {
	label: string;
	title: string;
	description: string;
} {
	switch (state) {
		case "active":
			return {
				label: "ADM 代行中",
				title: "代行工作區",
				description: "工作階段有效，所有動作會記錄在目前的 ADM 身分下。",
			};
		case "expiring":
			return {
				label: "代行即將到期",
				title: "工作階段即將到期",
				description: "請先完成並儲存目前工作；到期後會立即停止所有代行操作。",
			};
		case "expired":
			return {
				label: "代行已到期",
				title: "代行工作階段已到期",
				description:
					"已停止所有代行操作。請返回 ADM 商家詳情重新確認資格，再建立新的工作階段。",
			};
		case "revoked":
			return {
				label: "代行已撤銷",
				title: "代行權限已被撤銷",
				description:
					"伺服器已撤銷此工作階段，所有代行操作都已停止。請返回 ADM 商家詳情確認原因。",
			};
		case "end_unknown":
			return {
				label: "結束結果待確認",
				title: "工作階段結束結果尚未確認",
				description:
					"為避免在狀態不明時繼續操作，已停止所有代行寫入。請重試結束，或返回 ADM。",
			};
		case "target_mismatch":
			return {
				label: "目標不相符",
				title: "工作階段與目前店家不相符",
				description:
					"為避免操作錯誤店家，已停止所有代行操作。請返回 ADM 重新選擇目標。",
			};
		default:
			return {
				label: "代行不可用",
				title: "目前無法確認代行工作階段",
				description:
					"工作階段資料不完整或服務暫時無法使用，已停止所有代行操作。請稍後重試。",
			};
	}
}
