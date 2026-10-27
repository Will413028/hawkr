"use client";

import type * as React from "react";
import { useDelegatedSession } from "../state/DelegatedSessionContext";
import DelegatedAreaUnauthorized from "./DelegatedAreaUnauthorized";

/**
 * 具名路由的 capability 閘門（§8.1.1 **層 1**＝ UI 不呈現，在 www 這一側的落點）。
 *
 * ⚠️ 層 1 是**表現層**的收斂，沒有任何 boot 等級或後端保證：真正擋得住越權的是
 * Gateway 那一層（拒絕碼 9262）。這個閘門的價值在於「畫面不替目標店家的狀態作證」，
 * 不在於安全 —— 把它讀成安全防線，日後就會有人以它為由省掉後端檢查。
 *
 * **為什麼閘門要在路由這一層、而不是在賣家中心元件裡**：`/adm/{storeId}/*` 的內容
 * 一律是賣家中心的既有元件（薄殼原則，`tests/delegated-workspace-faces.test.ts`
 * 守著），那些元件在賣家路徑上永遠有權限，於是「權限不足」這個分支在它們身上根本
 * 不存在 —— 它們只會照常渲染，把「查不到」畫成「沒有」。閘門放進元件等於要求每一支
 * 賣家中心元件都替代行想一遍，那件事不會發生，也不該發生。
 *
 * ⚠️ 閘門為 deny 時**不渲染 children**（不是 `disabled`、不是隱藏 CTA）：children
 * 一掛上去就會發查詢，而此刻每一支都必然被拒 —— 那批 `FAILED` 的稽核列會把「有人
 * 試圖跨店讀取」這個安全訊號淹掉。
 *
 * ⚠️ children 只在工作階段 summary 已到手時才會被渲染（`DelegatedWorkspaceFrame`
 * 在 `loading` 與不可用狀態下都不渲染 children），所以這裡的 `hasCapability` 不會在
 * 「還沒問到」的那一瞬間誤判成 deny。
 */
export default function DelegatedCapabilityGate({
	capability,
	reason,
	backSegment,
	backLabel,
	children,
}: {
	/** §6.1.1 端點總表的機器名，例：`posts.read`。 */
	capability: string;
	reason: string;
	/** 有出口的面才給（例：新增頁回列表）。區段接在 `/adm/{storeId}/` 之後。 */
	backSegment?: string;
	backLabel?: string;
	children: React.ReactNode;
}) {
	const { hasCapability, summary } = useDelegatedSession();
	const storeId = summary?.target.storeId ?? "";
	return (
		<DelegatedCapabilityGateView
			granted={hasCapability(capability)}
			reason={reason}
			// storeId 拿不到就不畫出口：一個組不出目的地的按鈕比沒有按鈕更糟。
			backHref={
				backSegment && storeId
					? `/adm/${encodeURIComponent(storeId)}/${backSegment}`
					: undefined
			}
			backLabel={backLabel}
		>
			{children}
		</DelegatedCapabilityGateView>
	);
}

/**
 * 閘門的純渲染半邊（context 之外可直接測）。
 *
 * 分出來的理由就是那條要守的不變式本身：**deny 時 children 必須完全不出現**。
 * 那件事只有真的渲染一次才驗得到，而 `useDelegatedSession` 需要一整個工作階段
 * provider（含 30 秒輪詢）才跑得起來。
 */
export function DelegatedCapabilityGateView({
	granted,
	reason,
	backHref,
	backLabel,
	children,
}: {
	granted: boolean;
	reason: string;
	backHref?: string;
	backLabel?: string;
	children: React.ReactNode;
}) {
	if (granted) return <>{children}</>;
	return (
		<DelegatedAreaUnauthorized
			reason={reason}
			backHref={backHref}
			backLabel={backLabel}
		/>
	);
}
