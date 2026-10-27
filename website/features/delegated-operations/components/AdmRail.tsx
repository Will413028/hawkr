"use client";

import AppIcon from "@/components/shared/AppIcon";
import BrandLogo from "@/components/shared/BrandLogo";
import { useDelegatedSession } from "../state/DelegatedSessionContext";

/**
 * 左側 ADM 窄欄（mockup §3.1、L1030）。
 *
 * ⚠️ **兩個入口都是純 `<a href>`**，跨 host 指向 ADM origin，**不得產生任何
 * session 狀態變更**（不 end、不 revoke、不 bump `revision`）。C8 要求連續兩次
 * `GET current` 位元等價；動到 `revision` 會讓操作者回到工作區時撞上「與剛建立
 * 的稽核上下文不相符」而**永久進不去**。
 *
 * 用原生 `<a>` 而不是站內 `Link`：目的地是另一個 origin，站內 Link 的預抓與
 * view transition 對它沒有意義。
 *
 * 「操作紀錄」指向 ADM 而不是工作區內的頁面，因為稽核查詢掛 `requireAdmin()`
 * （§6.1.11）—— www host 上的瀏覽器沒有 ADM session，在這裡開一個稽核頁只會
 * 拿到 9261。目的地是**本次工作階段的稽核主控台**（`auditUrl`）：這個入口存在
 * 的理由就是「讓操作者查得到自己剛留下的稽核」（§3871），指到商家頁上一個不
 * 存在的錨點等於它從來沒有生效過，而畫面上完全看不出來。
 */
export default function AdmRail() {
	const { summary, returnUrl, auditUrl } = useDelegatedSession();
	const actorName = summary?.actor.displayName ?? "";

	return (
		<nav className="adm-rail" aria-label="ADM 導覽">
			<div className="adm-rail__brand">
				<BrandLogo variant="mark" tone="light" size="xs" />
				<span>Hawkr</span>
				<span>ADM</span>
			</div>

			<a className="adm-rail__link" href={returnUrl}>
				<AppIcon name="arrow-left" aria-hidden="true" />
				<span>返回 ADM</span>
			</a>

			<a className="adm-rail__link" href={auditUrl}>
				<AppIcon name="clock" aria-hidden="true" />
				<span>操作紀錄</span>
			</a>

			<div className="adm-rail__spacer" />

			<div className="adm-rail__actor">
				<span className="adm-rail__actor-avatar" aria-hidden="true">
					{actorName.slice(0, 1)}
				</span>
				{/* displayName 常常是一組 email，而窄欄的內容寬度只有 72px。不截斷就
				    直接溢出被切掉，看起來像渲染壞了；截斷後把完整值放進 title，游標
				    停住仍看得到自己是誰。 */}
				<span className="adm-rail__actor-name" title={actorName}>
					{actorName}
				</span>
				<span className="adm-rail__actor-role">管理員</span>
			</div>
		</nav>
	);
}
