/**
 * 已掛載的 zustand store 的記憶體 reset 註冊表。
 *
 * 獨立成一支零 import 的模組，是為了避免賣家中心的 store bundle 透過
 * local-private-state.ts 拉進 cart / checkout 的程式碼——註冊方（store）與
 * 觸發方（syncLocalPrivateState）都只依賴這支。
 *
 * 註冊發生在 store 的 module load。沒被載入過的 store 本來就沒有記憶體副本
 * 需要清，而 storage 那層一律照 registry 靜態清除——兩層語意互補，不存在
 * 「store 沒載入就漏清」的洞。
 */

const resets = new Map<string, () => void>();

export function registerLocalPrivateReset(
	key: string,
	reset: () => void,
): void {
	resets.set(key, reset);
}

export function runLocalPrivateReset(key: string): void {
	resets.get(key)?.();
}
