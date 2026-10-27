"use client";

import AppIcon, { type AppIconName } from "@/components/shared/AppIcon";
import Button from "@/components/ui/Button";
import { cn } from "@/lib/utils/cn";

/**
 * 商品上下架狀態元件。把原本散落在 ProductsPage 的 `statusLabel()` / 狀態色票 /
 * kebab「上架·下架」按鈕收斂成單一可重用元件。
 *
 * 兩種顯示方式（`variant`）：
 *   - `"cta"`  — 帶文字的膠囊按鈕（表格/工具列主要操作）。
 *   - `"icon"` — 純圖示方形按鈕（緊湊清單、行尾操作）。
 *
 * 兩種行為：
 *   - 傳入 `onToggle` → 互動式開關（上架 ⇄ 下架），點擊回傳目標狀態碼。
 *   - 未傳 `onToggle` → 純顯示徽章（`<span>`，不可點）。
 *
 * 色票對齊設計規範（docs/design-system-guidelines.md §3.2）：
 *   - `cta` 用顯眼底色：上架中 = 春蕪綠（淡灰字）、下架中 = 琥珀（深墨字），80% 不透明。
 *   - `icon`／審核中／草稿用柔和底色：success / default / info / secondary 的淡階。
 *   圖示走 currentColor → icon 與字同色。
 */

export type ShelfState = "active" | "off" | "review" | "draft";

/**
 * 後端 `status` 是 CodeMeta 字串：4001 草稿 / 4002 審核中 / 4003 未啟用(下架) /
 * 4004 啟用(上架)。Schema 遷移期間仍可能收到舊版數字碼（0 草稿 / 1 上架 /
 * 2 下架 / 3 審核），一併容錯。
 */
export function resolveShelfState(status?: string | number): ShelfState {
	const k = status == null ? "" : String(status);
	switch (k) {
		case "4004":
		case "1":
			return "active";
		case "4003":
		case "2":
			return "off";
		case "4002":
		case "3":
			return "review";
		default:
			return "draft";
	}
}

/** ShelfState → 後端 CodeMeta 狀態碼。 */
const STATE_TO_CODE: Record<ShelfState, "4001" | "4002" | "4003" | "4004"> = {
	draft: "4001",
	review: "4002",
	off: "4003",
	active: "4004",
};

/**
 * 任意狀態表示（內部數字 0/1/2 或 CodeMeta 字串）正規化成後端要的 CodeMeta
 * 字串。送出表單前必呼叫：後端 `status` 是 string，直接送數字會 400。
 */
export function resolveStatusCode(
	status?: string | number,
): "4001" | "4002" | "4003" | "4004" {
	return STATE_TO_CODE[resolveShelfState(status)];
}

/** 互動開關時，由目前狀態推出「點一下」要切換到的目標狀態碼。 */
export function shelfToggleTarget(state: ShelfState): "4003" | "4004" | null {
	if (state === "active") return "4003"; // 上架中 → 下架
	if (state === "off" || state === "draft") return "4004"; // 下架/草稿 → 上架
	return null; // 審核中無法手動切換
}

type Tone = {
	/** 文字色（透過 currentColor 帶到圖示） */
	fg: string;
	/** 底色（淡） */
	bg: string;
	/** 互動 hover 底色 */
	hover: string;
};

const TONE: Record<ShelfState, Tone> = {
	active: {
		fg: "text-[var(--hkui-success-700)]",
		bg: "bg-[var(--hkui-success-50)]",
		hover: "hover:bg-[var(--hkui-success-100)]",
	},
	off: {
		fg: "text-[var(--hkui-foreground-muted)]",
		bg: "bg-[var(--hkui-default-100)]",
		hover: "hover:bg-[var(--hkui-default-200)]",
	},
	review: {
		fg: "text-[var(--hkui-info-700)]",
		bg: "bg-[var(--hkui-info-50)]",
		hover: "hover:bg-[var(--hkui-info-100)]",
	},
	draft: {
		fg: "text-[var(--hkui-secondary-700)]",
		bg: "bg-[var(--hkui-secondary-50)]",
		hover: "hover:bg-[var(--hkui-secondary-100)]",
	},
};

/**
 * `cta` 變體的顯眼配色。像紅綠燈：上架中＝春蕪綠（販售中）、下架中＝琥珀
 * （已暫停、需注意）。底色用 color-mix 套 80% 不透明度，不要全實心。文字依底色
 * 取最清楚的對比：綠底用淡灰、琥珀底用深墨（琥珀偏淺，白字看不清）。icon 經
 * 取 currentColor 跟文字同色。審核中／草稿沒有對應實心 token，退回 `TONE` 的柔和
 * 底色；`icon` 變體也走柔和色。
 */
type Solid = { fg: string; bg: string; hover: string };

const SOLID: Partial<Record<ShelfState, Solid>> = {
	active: {
		// 綠底配淡灰字，對比更明顯（白/淡灰在中綠上比深字更清楚、更像主要 CTA）。
		fg: "text-[var(--color-neutral-whisper)]",
		bg: "bg-[color-mix(in_srgb,var(--hkui-success)_80%,transparent)]",
		hover:
			"hover:bg-[color-mix(in_srgb,var(--hkui-success-600)_80%,transparent)]",
	},
	off: {
		// 琥珀底（warning）較淺，配深墨字最清楚。
		fg: "text-[var(--color-neutral-ink)]",
		bg: "bg-[color-mix(in_srgb,var(--hkui-warning)_80%,transparent)]",
		hover:
			"hover:bg-[color-mix(in_srgb,var(--hkui-warning-600)_80%,transparent)]",
	},
};

const LABEL: Record<ShelfState, string> = {
	active: "上架中",
	off: "下架中",
	review: "審核中",
	draft: "草稿",
};

/** 切換提示文案（tooltip / aria-label）。不可切換時退回目前狀態的顯示名。 */
function toggleHint(state: ShelfState, label: string): string {
	const target = shelfToggleTarget(state);
	if (target === "4004") return "點擊上架，開始販售";
	if (target === "4003") return "點擊下架，暫停販售";
	return label;
}

/** 各狀態的圖示：上架向上箭頭、下架向下箭頭，審核中時鐘，草稿文件。 */
const SHELF_GLYPH: Record<ShelfState, AppIconName> = {
	active: "arrow-up",
	off: "arrow-down",
	review: "clock",
	draft: "document",
};

export interface ShelfStatusControlProps {
	status?: string | number;
	/**
	 * 後端下發的狀態顯示名。給了就用它，沒給才用本地 `LABEL`。
	 *
	 * 代行工作區靠這一欄顯示 CodeMeta 的 Label（4004＝`啟用`／4003＝`未啟用`），
	 * 賣家中心沿用自己的操作語（`上架中`／`下架中`）—— 兩邊講的是同一個碼，但
	 * 讀者不同：賣家看的是自己的販售動作，代行紀錄要對得上稽核列的欄位值。
	 */
	statusLabel?: string;
	/** `"cta"` 文字膠囊（預設）｜`"icon"` 純圖示方形。 */
	variant?: "cta" | "icon";
	size?: "sm" | "md";
	/** 載入中（樂觀更新進行時）：使用共用品牌 loading 契約並鎖定。 */
	busy?: boolean;
	/**
	 * 提供後元件成為互動開關（上架 ⇄ 下架），點擊回傳目標狀態碼。
	 * 省略則為純顯示徽章。審核中一律不可點。
	 */
	onToggle?: (target: "4003" | "4004") => void;
	/**
	 * 設定後，當「點一下」會「上架」(4004) 時，開關停用並以此字串作為 tooltip /
	 * aria-label（如加工食品沒有核准的食品業者登錄）。只擋上架，下架一律允許。
	 */
	upshelfDisabledReason?: string;
	className?: string;
}

export default function ShelfStatusControl({
	status,
	statusLabel,
	variant = "cta",
	size = "sm",
	busy = false,
	onToggle,
	upshelfDisabledReason,
	className,
}: ShelfStatusControlProps) {
	const state = resolveShelfState(status);
	const label = statusLabel || LABEL[state];
	const tone = TONE[state];
	const target = shelfToggleTarget(state);
	const interactive = Boolean(onToggle) && target !== null;
	// 只擋「上架」方向；下架/草稿→上架時若被閘記，停用開關並改用閘記原因當提示。
	const upshelfBlocked = target === "4004" && Boolean(upshelfDisabledReason);

	// cta 變體的上架/下架用實心顯眼色；其餘退回柔和底色。
	const solid = variant === "cta" ? SOLID[state] : undefined;
	const fgClass = solid?.fg ?? tone.fg;
	const bgClass = solid?.bg ?? tone.bg;
	const hoverClass = solid?.hover ?? tone.hover;

	const dims =
		variant === "icon"
			? size === "md"
				? "size-9"
				: "size-8"
			: size === "md"
				? "h-9 gap-1.5 px-3 text-sm"
				: "h-8 gap-1.5 px-2.5 text-xs";

	const glyphSize = size === "md" ? 18 : 16;

	const base = cn(
		// 顏色/底色走 200ms 漸變；載入由共用 Button 契約接管。
		"inline-flex select-none items-center justify-center rounded-full font-semibold whitespace-nowrap align-middle",
		"[font-family:var(--hkui-font-sans)] transition-[background,color,box-shadow,transform,opacity] duration-200 ease-[var(--hkui-ease)]",
		"shelf-status-control",
		`shelf-status-control--${variant}`,
		`shelf-status-control--${state}`,
		interactive && "shelf-status-control--interactive",
		dims,
		fgClass,
		bgClass,
		interactive &&
			!upshelfBlocked &&
			cn(
				hoverClass,
				"cursor-pointer active:scale-[0.98]",
				"focus-visible:!outline-none focus-visible:![box-shadow:var(--hkui-focus-ring)]",
			),
		upshelfBlocked && "cursor-not-allowed opacity-55",
		className,
	);

	const glyph = (
		<span
			className="inline-flex shrink-0 items-center justify-center"
			style={{ width: glyphSize, height: glyphSize }}
			aria-hidden="true"
		>
			<AppIcon name={SHELF_GLYPH[state]} size={glyphSize} />
		</span>
	);

	// 純顯示徽章 ───────────────────────────────────────────────
	if (!interactive) {
		// icon 變體沒有可見文字，補 role="img" + aria-label；cta 文字已可見。
		const iconA11y =
			variant === "icon" ? ({ role: "img", "aria-label": label } as const) : {};
		return (
			<span className={base} {...iconA11y}>
				{glyph}
				{variant === "cta" && <span>{label}</span>}
			</span>
		);
	}

	// 互動開關 ─────────────────────────────────────────────────
	const hint = upshelfBlocked
		? (upshelfDisabledReason as string)
		: toggleHint(state, label);
	const pendingLabel = target === "4004" ? "上架商品中…" : "下架商品中…";
	return (
		<span className="shelf-status-control-tooltip" data-tooltip={hint}>
			<Button
				type="button"
				variant="light"
				color="default"
				size="sm"
				isIconOnly={variant === "icon"}
				className={base}
				disabled={busy || upshelfBlocked}
				isLoading={busy}
				loadingLabel={pendingLabel}
				aria-pressed={state === "active"}
				aria-label={busy ? pendingLabel : hint}
				onClick={() => target && onToggle?.(target)}
			>
				{glyph}
				{variant === "cta" && <span>{label}</span>}
			</Button>
		</span>
	);
}
