"use client";

import * as React from "react";
import BrandLogo from "@/components/shared/BrandLogo";
import type { AgentCapabilities } from "./capabilities";
import {
	type AgentCapabilityChip,
	ENTRY_COMPOSER_PLACEHOLDER,
	ENTRY_HEADING,
	ENTRY_SUBHEADING,
	flaggedCapabilityChips,
	SLICE_A_CAPABILITY_CHIPS,
} from "./entry-content";
import HawkrAgentCapabilityIcon from "./HawkrAgentCapabilityIcon";

interface PrefillTarget {
	id: string;
	text: string;
}

/**
 * 帶入 composer 的規則（§V.11.0：「若 composer 已有使用者修改，不得靜默覆蓋」）。
 *
 * 做法是兩段式而不是彈 confirm dialog：草稿是空的、或內容就是上一次由我們帶入
 * 的範本時直接取代；否則第一次點只把該張卡切成「再按一次取代」，第二次點才真的
 * 覆蓋。使用者永遠不會因為誤觸而失去自己打的字，也不必面對一個 modal。
 */
function usePrefill(
	draft: string,
	onPrefill: (text: string) => void,
): {
	pendingId: string | null;
	apply: (target: PrefillTarget) => void;
} {
	const lastAppliedRef = React.useRef<string | null>(null);
	// 連草稿一起記下來。待確認狀態綁的是「使用者當時看到的那份草稿」——
	// 之後又打了字，那個確認就不再對應同一個決定，於是自動失效。用 effect
	// 去 reset 也做得到，但那會多一次 render，而且會把「什麼時候失效」的
	// 規則藏在依賴陣列裡。
	const [pending, setPending] = React.useState<{
		id: string;
		draftAtPrompt: string;
	} | null>(null);
	const pendingId = pending?.draftAtPrompt === draft ? pending.id : null;

	const apply = (target: PrefillTarget) => {
		const untouched = draft.trim() === "" || draft === lastAppliedRef.current;
		if (!untouched && pendingId !== target.id) {
			setPending({ id: target.id, draftAtPrompt: draft });
			return;
		}
		lastAppliedRef.current = target.text;
		setPending(null);
		onPrefill(target.text);
	};

	return { pendingId, apply };
}

interface AgentEntryPanelProps {
	capabilities: AgentCapabilities;
	/** 目前草稿——決定 chips／起手卡要不要走「不靜默覆蓋」的兩段式確認。 */
	draft: string;
	/** 把文字帶入 composer 並把焦點移過去。 */
	onPrefill: (text: string) => void;
	/**
	 * 唯一的那個 composer（§V.11.0「不生成第二套 composer」）。由
	 * `AssistantWorkspace` 渲染後放進這個插槽，首送後同一個元件改掛到 thread
	 * 底部——這裡刻意不自己畫一個輸入框。
	 */
	composer: React.ReactNode;
}

interface CapabilityButtonProps {
	chip: AgentCapabilityChip;
	pending: boolean;
	onApply: (chip: AgentCapabilityChip) => void;
}

export function CapabilityButton({
	chip,
	pending,
	onApply,
}: CapabilityButtonProps) {
	const descriptionId = chip.description
		? `hawkr-agent-capability-${chip.id}-description`
		: undefined;
	const accessibleName = pending
		? `再按一次，以「${chip.label}」取代目前輸入`
		: chip.label;

	return (
		<li>
			<button
				type="button"
				className="hawkr-agent-entry__chip"
				aria-label={accessibleName}
				aria-describedby={descriptionId}
				data-capability-id={chip.id}
				data-has-icon={chip.icon ? "true" : undefined}
				data-confirm={pending ? "replace" : undefined}
				onClick={() => onApply(chip)}
			>
				{chip.icon ? <HawkrAgentCapabilityIcon icon={chip.icon} /> : null}
				<span className="hawkr-agent-entry__chip-label">
					{pending ? "再按一次取代目前輸入" : chip.label}
				</span>
				{chip.description ? (
					<span
						id={descriptionId}
						className="hawkr-agent-entry__chip-description"
					>
						{chip.description}
					</span>
				) : null}
			</button>
		</li>
	);
}

/**
 * Hawkr Agent 新對話中央入口（設計書 v6 §V.11.0 Screen Contract 00，圖 00-A）。
 *
 * 版面由上而下固定為：置中 Hawkr 標誌 → 唯一 h1 → 一行說明 → 3 個 capability
 * chips → 大型中央 composer。
 *
 * 與 §V.11.0 的三處落差都是產品端決定，不是漏做：
 * 1. 「從範例開始」那排起手卡移除——三張卡與上面三個 chip 是同一組能力，等於
 *    同一件事給了兩個入口；範本已併到 chip 上。
 * 2. composer 下方不重複固定信任提示；真正可能詢價、通知、下單或付款的步驟
 *    才就地顯示版本化確認與 fail-closed CTA。這個產品決定已回寫 §V.11.0，
 *    不代表放寬 approval gate。
 * 3.「最近的任務」區塊移除——完整清單本來就在左側 Messages 導覽 sidebar 裡。
 *
 * 設計書的 anti-goals 直接對應到這支元件的**不做什麼**：沒有 thread header、
 * 沒有 transcript、沒有固定在 viewport 底部的小 composer、沒有空白的 Artifact
 * 右欄、沒有 integrations 牆、沒有假進度，chips 與起手卡也一律只帶字進輸入框，
 * 不觸發任何 side effect（不建案件、不發詢價、不通知商家）。
 */
export default function AgentEntryPanel({
	capabilities,
	draft,
	onPrefill,
	composer,
}: AgentEntryPanelProps) {
	const { pendingId, apply } = usePrefill(draft, onPrefill);

	const chips: readonly AgentCapabilityChip[] = [
		...SLICE_A_CAPABILITY_CHIPS,
		...flaggedCapabilityChips(capabilities),
	];

	return (
		<section
			className="hawkr-agent-entry"
			aria-labelledby="hawkr-agent-entry-heading"
			// entry 與 thread 是同一個 workspace 的兩個 view state，所以共用
			// data-assistant-mode——E2E 與樣式都靠它辨認「這是 live agent 表面」，
			// 換 view 不該讓那個判斷失效。
			data-assistant-mode="live"
			data-assistant-view="entry"
		>
			<div className="hawkr-agent-entry__hero">
				{/* 入口品牌記號，不是 run status。工作開始後由
				 * HawkrAgentSprite + canonical events 表達狀態。 */}
				<span
					className="hawkr-agent-entry__sprite hawkr-agent-sprite"
					aria-hidden="true"
				>
					<BrandLogo variant="mark" size="fill" />
				</span>
				<h1 id="hawkr-agent-entry-heading">{ENTRY_HEADING}</h1>
				<p className="hawkr-agent-entry__lede">{ENTRY_SUBHEADING}</p>
			</div>

			<ul className="hawkr-agent-entry__chips">
				{chips.map((chip) => (
					<CapabilityButton
						key={chip.id}
						chip={chip}
						pending={pendingId === chip.id}
						onApply={(target) =>
							apply({ id: target.id, text: target.template })
						}
					/>
				))}
			</ul>

			<div className="hawkr-agent-entry__composer">{composer}</div>
		</section>
	);
}

export { ENTRY_COMPOSER_PLACEHOLDER };
