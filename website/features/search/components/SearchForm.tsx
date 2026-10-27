"use client";

import * as React from "react";
import Button from "@/components/ui/Button";
import AssistantEntryLink from "@/features/procurement-assistant/AssistantEntryLink";
import { HAWKR_AGENT_ENABLED } from "@/features/procurement-assistant/availability";
import { useLocalizedHref } from "@/lib/intl/use-router";
import { useObservedRouter } from "@/lib/navigation/use-observed-router";

interface SearchFormProps {
	query: string;
	sort: string;
}

export default function SearchForm({ query, sort }: SearchFormProps) {
	const router = useObservedRouter("search_form");
	const action = useLocalizedHref("/search");
	const [value, setValue] = React.useState(query);
	const [isPending, startTransition] = React.useTransition();
	const draftRevision = React.useRef(0);
	const submittedDrafts = React.useRef<
		Array<{ query: string; revision: number }>
	>([]);

	React.useEffect(() => {
		const submittedIndex = submittedDrafts.current.findLastIndex(
			(draft) => draft.query === query,
		);
		if (submittedIndex >= 0) {
			const submitted = submittedDrafts.current[submittedIndex];
			submittedDrafts.current.splice(0, submittedIndex + 1);
			// A URL commit acknowledges its submitted text, not newer keystrokes.
			if (submitted.revision !== draftRevision.current) return;
		} else {
			// Tags and browser history still replace the form's current query.
			submittedDrafts.current = [];
		}
		setValue(query);
	}, [query]);

	const handleSubmit = React.useCallback(
		(event: React.FormEvent<HTMLFormElement>) => {
			event.preventDefault();
			const next = new URLSearchParams();
			const normalizedQuery = value.trim();
			if (normalizedQuery !== query) {
				submittedDrafts.current.push({
					query: normalizedQuery,
					revision: draftRevision.current,
				});
			}
			if (normalizedQuery) next.set("q", normalizedQuery);
			if (sort !== "推薦") next.set("sort", sort);
			const search = next.toString();

			startTransition(() => {
				router.push(search ? `/search?${search}` : "/search", {
					scroll: false,
				});
			});
		},
		[query, router, sort, value],
	);

	return (
		<div className="grid max-w-[880px] gap-2.5 mb-3.5 min-[1280px]:max-w-none min-[1280px]:grid-cols-[minmax(0,760px)_minmax(320px,1fr)] min-[1280px]:items-start min-[1280px]:gap-4">
			<form
				className="search-box max-w-[880px] m-0!"
				action={action}
				onSubmit={handleSubmit}
				aria-busy={isPending}
			>
				{sort !== "推薦" ? (
					<input type="hidden" name="sort" value={sort} />
				) : null}
				<input
					name="q"
					value={value}
					onChange={(event) => {
						draftRevision.current += 1;
						setValue(event.currentTarget.value);
					}}
					inputMode="search"
					enterKeyHint="search"
					placeholder="搜尋芒果、蔬菜箱、火鍋..."
					aria-label="搜尋商品"
				/>
				<Button
					type="submit"
					disabled={isPending}
					isLoading={isPending}
					loadingLabel="搜尋中…"
				>
					搜尋
				</Button>
			</form>
			{/* class 是讓位規則的鉤子：0 結果時空態那顆才是主要 CTA，這顆隱藏
			 * （見 procurement-assistant.css 的 :has 規則）。 */}
			{HAWKR_AGENT_ENABLED ? (
				<AssistantEntryLink
					variant="page"
					query={value}
					source="search-page"
					className="search-page__agent-entry"
				/>
			) : null}
		</div>
	);
}
