import type { ReactNode } from "react";
import AppIcon from "@/components/shared/AppIcon";

export const ASSISTANT_ENTRY_LAYOUT =
	"flex min-h-11 items-center gap-3 rounded-hk-lg border border-hk-primary-border bg-hk-primary-subtle px-3 py-2.5 text-left text-hk-heading";

/** Keeps the known query text and line wrapping identical before/after import. */
export function AssistantEntryContent({
	query = "",
	mark,
}: {
	query?: string;
	mark: ReactNode;
}) {
	const normalized = query.trim();
	const preview =
		normalized.length > 34 ? `${normalized.slice(0, 34)}…` : normalized;
	return (
		<>
			{mark}
			<span className="min-w-0 flex-1">
				<strong className="block font-display text-sm font-black leading-snug">
					Hawkr Agent 幫你找
				</strong>
				<small className="mt-0.5 block text-xs leading-relaxed text-hk-muted line-clamp-2">
					{preview
						? `帶著「${preview}」進入聊天，確認後開始找貨。`
						: "大量採購、規格比較或找不到合適商品時，直接問 Hawkr Agent。"}
				</small>
			</span>
			<AppIcon
				name="chevron-right"
				size={18}
				className="shrink-0 text-hk-primary transition-transform group-hover:translate-x-0.5"
				aria-hidden="true"
			/>
		</>
	);
}

/** Inert entry shell while the search experience code loads; no auth or assets. */
export function AssistantEntrySkeleton({ query = "" }: { query?: string }) {
	return (
		<div
			className={ASSISTANT_ENTRY_LAYOUT}
			role="status"
			aria-busy="true"
			aria-label="Hawkr Agent 入口載入中"
		>
			<AssistantEntryContent
				query={query}
				mark={
					<span
						aria-hidden="true"
						className="size-9 shrink-0 animate-pulse rounded-full bg-default-100 motion-reduce:animate-none"
					/>
				}
			/>
		</div>
	);
}
