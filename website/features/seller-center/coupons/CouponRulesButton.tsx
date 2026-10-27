"use client";

import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import { Dialog } from "@/components/ui/Dialog";

/**
 * 「？」說明按鈕 + 發放規則彈窗。放在「折抵設定」標題旁，點開後以 FAQ 折疊樣式
 * 向賣家說明優惠券發放規則。規則內容對齊 backend-payuni 優惠券微服務詳細設計書
 * （§2.7 賣家配額、§2.8 領券機制、§5.1 SellerCreateTemplate 參數驗證），與
 * CouponFormClient 的前端驗證為同一份規則，前端先擋、後端最終把關。
 */
interface Rule {
	q: string;
	a: React.ReactNode;
}

const RULES: Rule[] = [
	{
		q: "每店最多能有幾種優惠券？",
		a: (
			<p className="m-0">
				每間店鋪最多同時擁有 5
				種「有效中」的優惠券。已過期或已收回的會自動釋出名額；補償券不佔用配額。達到上限時將無法建立新券，可先收回不需要的券再建立。
			</p>
		),
	},
	{
		q: "折抵金額與門檻有什麼限制？",
		a: (
			<ul className="m-0 flex list-disc flex-col gap-1.5 pl-5">
				<li>所有金額皆為整數，不可填小數或負數。</li>
				<li>現金折抵：折抵金額需大於 0。</li>
				<li>
					折扣百分比：填
					1–99，且必須設定「單次折抵上限」，避免大額訂單被過度折抵。
				</li>
				<li>最低消費門檻：填 0 代表不限金額，不可為負數。</li>
			</ul>
		),
	},
	{
		q: "誰可以領取與使用？",
		a: (
			<ul className="m-0 flex list-disc flex-col gap-1.5 pl-5">
				<li>全體買家：需在本券建立前已註冊的買家才符合資格。</li>
				<li>賣家會員：已成為本店會員的買家。</li>
				<li>指定買家：至少選擇 1 位。</li>
				<li>每位買家每張券限領 1 次。</li>
			</ul>
		),
	},
	{
		q: "生效時間怎麼算？",
		a: (
			<p className="m-0">
				需設定開始與到期時間，開始時間不可晚於到期時間，可使用到到期當日
				23:59。期間外的優惠券買家端會自動隱藏、無法使用。
			</p>
		),
	},
	{
		q: "賣家券和平台券有什麼不同？",
		a: (
			<p className="m-0">
				賣家券即建即發、買家無需領取即可使用，且僅限在本店結帳時折抵。
			</p>
		),
	},
];

export default function CouponRulesButton() {
	const [open, setOpen] = React.useState(false);
	const [expanded, setExpanded] = React.useState<Set<number>>(
		() => new Set([0]),
	);

	const toggle = (idx: number) =>
		setExpanded((prev) => {
			const next = new Set(prev);
			if (next.has(idx)) next.delete(idx);
			else next.add(idx);
			return next;
		});

	return (
		<>
			<button
				type="button"
				aria-label="查看優惠券發放規則"
				onClick={() => setOpen(true)}
				className="inline-grid size-6 shrink-0 place-items-center rounded-full border border-hk-border-soft bg-hk-surface-card text-hk-muted transition-colors duration-150 hover:border-hk-primary hover:text-hk-primary focus-visible:outline-none focus-visible:[box-shadow:var(--hkui-focus-ring)] dark:border-[#f5dea5]/30 dark:bg-white/5"
			>
				<AppIcon name="info" size={14} />
			</button>

			<Dialog
				open={open}
				onClose={() => setOpen(false)}
				title="優惠券發放規則"
				description="建立前請確認以下規則，前端會先檢查，系統最終以後端為準。"
				size="lg"
			>
				<div className="flex flex-col gap-3">
					{RULES.map((rule, idx) => {
						const isOpen = expanded.has(idx);
						return (
							<div
								key={rule.q}
								className="glass-card overflow-hidden rounded-2xl"
							>
								<button
									type="button"
									aria-expanded={isOpen}
									onClick={() => toggle(idx)}
									className="flex w-full cursor-pointer items-center justify-between gap-4 border-0 bg-transparent px-5 py-4 text-left transition-colors duration-200 hover:bg-hk-surface-glass-hover"
								>
									<span className="text-base font-bold text-hk-heading">
										{rule.q}
									</span>
									<AppIcon
										name="chevron-down"
										className={`shrink-0 transition-transform duration-300 ease-in-out ${isOpen ? "rotate-180" : ""}`}
										size={28}
									/>
								</button>
								{isOpen && (
									<div className="px-5 pb-4 text-sm leading-relaxed text-hk-body">
										{rule.a}
									</div>
								)}
							</div>
						);
					})}
				</div>
			</Dialog>
		</>
	);
}
