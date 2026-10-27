"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { getGsiFailureStage, renderGoogleButton } from "@/lib/auth/gsi";
import { env } from "@/lib/config/env";
import { stepUpVendorPortalGoogle } from "@/lib/vendor-portal/repo";
import {
	stepUpBlockedMessage,
	stepUpFailureMessage,
} from "@/lib/vendor-portal/step-up-message";

export type StepUpPhase = "idle" | "verifying" | "done" | "error";

interface Props {
	/** 要驗證哪一張邀請的 session（頁面網址的收件人）。 */
	rfqRecipientId: string;
	/** 伺服器給的 nonce（ExchangeInvite／GetSession）；空字串＝session 不存在，不畫 Google 按鈕。 */
	googleNonce: string;
	/** 升級成功時回報新的 assurance，讓上層打開正式提交。 */
	onVerified: (assurance: string) => void;
}

/**
 * 站外業者的 Google step-up。
 *
 * 🔴 **為什麼存草稿不用、正式提交要**：草稿可以一直覆寫；正式提交產生的是 immutable
 * 的報價版本，買家接受 exact version 即成交 ⇒ 不可撤回的商務承諾不能只靠一條邀請信
 * 連結（拿到那封信的任何人都能代業者承諾價格）。
 *
 * ⚠️ 這裡只取 Google 回的 `idToken` 交給後端，**前端不讀也不送 email**——比對
 * 「這是不是被邀請的那個信箱」是後端的事，前端送的 email 是呼叫端說了算的值。
 *
 * ⚠️ `permission_denied` 的訊息刻意**不分成因**：Google token 無效、信箱未驗證、信箱不符、
 * session 已用別的信箱驗證，後端一律回同一個 `permission_denied`（同一段訊息），前端也不該自己
 * 猜出一個更「貼心」的訊息——那會變成逐項試探的介面。其他 code（unauthenticated、
 * resource_exhausted、unavailable）與成因無關，文字由 `stepUpFailureMessage` 依 code 決定。
 * 同一個 session 以相同信箱重送是冪等的成功，不算失敗。
 */
export function VendorGoogleStepUp({
	rfqRecipientId,
	googleNonce,
	onVerified,
}: Props) {
	const hostRef = useRef<HTMLDivElement>(null);
	const [phase, setPhase] = useState<StepUpPhase>("idle");
	const [message, setMessage] = useState("");
	const phaseRef = useRef(phase);
	phaseRef.current = phase;

	const handleIdToken = useCallback(
		(idToken: string) => {
			// 連點兩下 Google 按鈕不該送出兩次升級請求。
			if (phaseRef.current === "verifying") return;
			setPhase("verifying");
			setMessage("");
			stepUpVendorPortalGoogle(rfqRecipientId, idToken)
				.then(({ assurance }) => {
					setPhase("done");
					onVerified(assurance);
				})
				.catch((error: unknown) => {
					setPhase("error");
					setMessage(stepUpFailureMessage(error));
				});
		},
		[rfqRecipientId, onVerified],
	);

	useEffect(() => {
		const el = hostRef.current;
		if (!el) return;
		const controller = new AbortController();
		// nonce 換了（新的 session）要以新的 nonce 重新初始化，不留舊的按鈕。
		el.replaceChildren();

		// 沒有 nonce ＝ 伺服器沒有這張邀請的 session（顯示與 unauthenticated 相同的文字）。沒有 nonce 的 token 伺服器一定拒絕，所以不畫按鈕。
		const blocked = stepUpBlockedMessage(googleNonce);
		if (blocked) {
			setPhase("error");
			setMessage(blocked);
			return;
		}
		setPhase((p) => (p === "error" ? "idle" : p));
		setMessage("");

		// 空 client id ＝ 部署少設 NEXT_PUBLIC_GOOGLE_CLIENT_ID。講清楚是設定問題，
		// 不要讓業者以為是自己的帳號有問題而一直重試。
		if (!env.googleClientId) {
			setPhase("error");
			setMessage("驗證服務尚未設定完成，請聯絡 Hawkr 的窗口。");
			return;
		}

		renderGoogleButton(el, env.googleClientId, handleIdToken, {
			signal: controller.signal,
			nonce: googleNonce,
		}).catch((error: unknown) => {
			if (controller.signal.aborted) return;
			setPhase("error");
			setMessage(
				`無法載入 Google 驗證（${getGsiFailureStage(error) ?? "unknown"}）。請重新整理頁面再試一次。`,
			);
		});

		return () => controller.abort();
	}, [handleIdToken, googleNonce]);

	if (phase === "done") {
		return (
			<p className="m-0 text-xs text-foreground-soft">
				{"已完成 Google 帳號驗證，可以正式提交報價了。"}
			</p>
		);
	}

	return (
		<div className="flex flex-col gap-2">
			<p className="m-0 text-xs text-foreground-soft">
				{
					"正式提交會建立一份買家看得到、不可再修改的報價版本。送出前請先用收到邀請信的那個 Google 帳號完成驗證。"
				}
			</p>
			<div ref={hostRef} aria-busy={phase === "verifying"} />
			{phase === "verifying" && (
				<p className="m-0 text-xs text-foreground-soft">{"驗證中…"}</p>
			)}
			{phase === "error" && message !== "" && (
				<p className="m-0 text-xs text-danger" role="alert">
					{message}
				</p>
			)}
		</div>
	);
}
