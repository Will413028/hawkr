import { useState } from "react";
import type { ActionResult } from "@/lib/connect-action";
import { keepConfirmed, matchFoundIds } from "./ProspectMatchDialog";

/** 寫入失敗的結果（`ActionResult` 的失敗分支，含 reason、metadata 與給操作者的文字）。 */
export type FailedAction = Extract<ActionResult, { ok: false }>;

export interface PendingMatch {
	ids: string[];
	confirmed: string[];
}

/**
 * 寫入撞到相符業者（`PROSPECT_MATCH_FOUND`）的流程：開 `ProspectMatchDialog` 讓操作者確認都不是同一家，帶全部 ID 重送；
 * 重送又撞到就以新的 ID 集合重開，已勾過而且仍在新集合裡的保留勾選。建立、修改、還原共用。
 *
 * `send` 帶著已確認的 ID 送出寫入；其他失敗交給 `onFailure`。確認框裡的失敗由 `confirm` 丟出、留在框內顯示，
 * 只有帶 reason 的失敗（例如重送撞到統編被其他業者持有）例外：關閉對話框並交給 `onFailure`，與第一次送出同一個出口。
 */
export function useProspectMatchRetry<T>({
	send,
	onFailure,
}: {
	send: (confirmedIds: string[]) => Promise<ActionResult<T>>;
	onFailure: (failure: FailedAction) => void;
}): {
	pending: PendingMatch | null;
	submit: () => Promise<ActionResult<T>>;
	confirm: (ids: string[]) => Promise<void>;
	cancel: () => void;
} {
	const [pending, setPending] = useState<PendingMatch | null>(null);

	const submit = async () => {
		const prior = pending?.confirmed ?? [];
		const result = await send(prior);
		if (result.ok) {
			setPending(null);
			return result;
		}
		const ids = matchFoundIds(result);
		if (ids) setPending({ ids, confirmed: keepConfirmed(prior, ids) });
		else onFailure(result);
		return result;
	};

	const confirm = async (ids: string[]) => {
		const result = await send(ids);
		if (result.ok) {
			setPending(null);
			return;
		}
		const nextIds = matchFoundIds(result);
		if (nextIds) {
			setPending({ ids: nextIds, confirmed: keepConfirmed(ids, nextIds) });
			return;
		}
		if (result.kind === "message" && result.reason) {
			setPending(null);
			onFailure(result);
			return;
		}
		throw new Error(result.text);
	};

	return { pending, submit, confirm, cancel: () => setPending(null) };
}
