/**
 * `<input type="file">` 的 onChange：把選到的檔案取出來，同時讓同一個檔案能再選一次。
 *
 * ⚠️ **順序是這支函式存在的全部理由**：`input.files` 是一個 **live FileList**，而
 * `input.value = ""` 依規範會**就地清空它**。先清空再把那個參考傳出去，拿到的就是
 * 一個 `length === 0` 的清單 —— 呼叫端的 `if (files.length === 0) return;` 會照常
 * 早退，於是整條上傳鏈一次都不會跑，畫面上沒有錯誤、沒有網路請求、什麼都不會發生。
 * 這種靜默失效沒有任何型別或編譯期檢查擋得住。
 *
 * 清空 `value` 本身是必要的：不清，連續選同一個檔案不會再觸發 change 事件。
 *
 * 回傳 `File[]` 而不是 `FileList`：具體化之後的陣列與 input 再無關聯，呼叫端要在
 * 中途 `await` 什麼都不會影響它 —— 順序的正確性因此由型別保證，而不是靠「這個
 * handler 的第一個 await 出現在 `Array.from` 之後」這種隱性條件。
 */
export function takePickedFiles(input: HTMLInputElement): File[] {
	const picked = Array.from(input.files ?? []);
	input.value = "";
	return picked;
}
