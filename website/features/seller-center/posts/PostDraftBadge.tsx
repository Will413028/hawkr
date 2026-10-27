import {
	POST_PUBLISH_STATUS_LABEL,
	PostPublishStatus,
} from "@/lib/codemeta/enums";
import { formatTaipeiDateTime } from "@/lib/date/taipei";

/**
 * 「未發布」狀態 pill（`publish_status = 4401`）。
 *
 * 未發布的貼文在列表上原本沒有任何標記，而被平台下架的有「平台下架」pill —— 於是
 * 一則從未發布的貼文看起來就是一則普通的已發布貼文。代行者正是靠這個列表判斷這家
 * 店的經營狀態，他會把它當成已上線的內容回報給賣家。
 *
 * 用 neutral tone 而不是 danger：未發布不是錯誤狀態，它只是還沒發布。danger 留給
 * `PostHiddenBadge`（平台強制下架，那是治理動作）。
 *
 * 文字取 CodeMeta 的 label，不自行意譯 —— 同檔的 `PostUnpublishedBadge` 一直是這樣
 * 寫的，而這一顆曾經是裸字串「草稿」，於是 CodeMeta 改成「未發布」時只有它沒跟上。
 */
export function PostDraftBadge() {
	return (
		<span className="inline-flex items-center gap-1.5 whitespace-nowrap rounded-full bg-content2 px-2 py-0.5 text-[0.6875rem] font-bold text-foreground/70">
			<span
				className="inline-block h-1.5 w-1.5 rounded-full bg-foreground/45"
				aria-hidden="true"
			/>
			{POST_PUBLISH_STATUS_LABEL[PostPublishStatus.DRAFT]}
		</span>
	);
}

/**
 * 「已下架」狀態 pill —— **店家自己撤下的**（`publish_status = 4403`）。
 *
 * 與 `PostHiddenBadge`（平台強制下架，`hidden_at`）是兩件事，CodeMeta 明說；所以
 * tone 也刻意不同：這裡是 warning（店家的收縮型操作），那裡是 danger（治理動作）。
 * 兩顆不會同時出現在同一列（下架優先），但同一份列表裡兩種意義必須一眼分得出來
 * —— 「平台罰的」還是「店家自己撤的」正是代行者要回報給賣家的那件事。
 *
 * 文字取 CodeMeta 的 label，不自行意譯。
 */
export function PostUnpublishedBadge() {
	return (
		<span className="inline-flex items-center gap-1.5 whitespace-nowrap rounded-full bg-warning-50 px-2 py-0.5 text-[0.6875rem] font-bold text-warning-700 dark:bg-[#a9842c]/26 dark:text-[#d6c27e]">
			<span
				className="inline-block h-1.5 w-1.5 rounded-full bg-warning-700 dark:bg-[#c7ae62]"
				aria-hidden="true"
			/>
			{POST_PUBLISH_STATUS_LABEL[PostPublishStatus.UNPUBLISHED]}
		</span>
	);
}

/**
 * 這則貼文是不是未發布（4401）。
 *
 * `publishStatus` 缺值代表**這條資料路徑的上游沒給**，不是「這則貼文已發布」——
 * 所以缺值時回 false 讓呼叫端什麼都不畫，而不是推定成已發布。畫一個沒有根據的
 * 狀態，比不畫更糟。（賣家自己那條 `/app/sellers/me/posts` 現在也帶 `publishStatus`
 * 了，但缺值的處置不因此放寬 —— 那是述詞的防線，不是某一條路徑的暫時性補丁。）
 */
export function isPostDraft(p: { publishStatus?: string }): boolean {
	return p.publishStatus === PostPublishStatus.DRAFT;
}

/** 這則貼文是不是店家自己撤下的（曾發布後被下架）。缺值的處置理由同 `isPostDraft`。 */
export function isPostUnpublished(p: { publishStatus?: string }): boolean {
	return p.publishStatus === PostPublishStatus.UNPUBLISHED;
}

/**
 * 發布狀態 pill 的單一分派點。
 *
 * 三態各畫各的，`4402`（已發布）與缺值一律不畫 —— 已發布是常態不需要標記，缺值則是
 * 「上游沒給」而不是「已發布」。呼叫端只要問一次，就不會像原本那樣補了未發布卻漏掉
 * 已下架（4403 落在兩個判斷之間，與已發布貼文像素級相同）。
 *
 * 版面 class 由呼叫端給、且**沒有 pill 時連外框都不畫**：外框帶著 margin，空著渲染
 * 會讓已發布的那些列多出一段沒有內容的間距。
 */
export function PostPublishStatusBadge({
	publishStatus,
	scheduledPublishAt,
	className,
}: {
	publishStatus?: string;
	/** 代行者排定、尚未執行的發布時間；只在未發布的 pill 旁顯示。 */
	scheduledPublishAt?: string;
	className?: string;
}) {
	const scheduledText = scheduledPublishAt
		? formatTaipeiDateTime(scheduledPublishAt)
		: "";
	const pill = isPostDraft({ publishStatus }) ? (
		<>
			<PostDraftBadge />
			{scheduledText ? (
				<span className="text-xs text-foreground/60">
					排程於 {scheduledText} 發布
				</span>
			) : null}
		</>
	) : isPostUnpublished({ publishStatus }) ? (
		<PostUnpublishedBadge />
	) : null;
	if (!pill) return null;
	return <div className={className}>{pill}</div>;
}
