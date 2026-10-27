import "@/styles/components/hawkr-agent-mark.css";
import BrandLogo from "@/components/shared/BrandLogo";

interface HawkrAgentMarkProps {
	/** 有值＝這個記號自己承載語意（role="img"）；省略＝純裝飾。 */
	label?: string;
	/** 外圈容器的 class。預設 `.assistant-mark`（46px）。 */
	className?: string;
	/** 省略 AI 角標——只在角標會擠壓版面的極小尺寸使用。 */
	hideBadge?: boolean;
}

/**
 * Hawkr Agent 的識別記號——**全站唯一來源**（訊息側欄分頁與對話列、聊天室
 * 頭像、訊息頭像、step ticker、問題卡、任務卡、搜尋入口卡、路由錯誤頁）。
 *
 * 內容是 Hawkr 品牌標誌的靜態版。設計書 §V.10.10-12 把兩個元件的分工訂死，
 * 不要合併：這一顆是 sidebar／歷史對話／小型 identity 的**中性**記號，
 * `HawkrAgentSprite` 才是當前這一輪的狀態呈現。同一節明文禁止「讓目前 run 的
 * active／success 表情全域套到其他歷史 conversation」，所以這裡不吃 motion state。
 *
 * 尺寸一律由外圈容器的 class 決定（`.assistant-mark` 46px、
 * `.assistant-mark--sm` 36px、對話列 36px、訊息頭像 32px…），標誌填滿外圈。
 *
 * ⚠️ AI 角標一律用 `<small>`：CSS 的 `.assistant-mark small` 是角標樣式。
 */
export default function HawkrAgentMark({
	label,
	className = "assistant-mark",
	hideBadge = false,
}: HawkrAgentMarkProps) {
	return (
		<span
			className={className}
			{...(label
				? { role: "img" as const, "aria-label": label }
				: { "aria-hidden": true })}
		>
			<BrandLogo variant="mark" size="fill" />
			{hideBadge ? null : <small>AI</small>}
		</span>
	);
}
