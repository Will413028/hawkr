"use client";

import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import Button from "@/components/ui/Button";
import { useSellerTags } from "@/lib/hooks/useSellerTags";
import {
	addCustomerTags,
	type CustomerTag,
	fetchCustomerTags,
	removeCustomerTag,
} from "@/lib/seller/repo";

/** Quick-add presets — mirror the app's tag bottom-sheet suggestions, kept
 * neutral per the brand voice (no judgemental「奧客」labels in web UI). */
const QUICK_TAGS = ["新客", "回購", "高消費", "VIP", "待聯繫"];

interface Props {
	buyerId: string | number;
	/** Tag names already on the detail payload — rendered instantly while the
	 * authoritative `{id,name}` list loads (ids are needed to remove). */
	initialTags?: string[];
	/** Fired after a successful add / remove so a host (e.g. the customer list
	 * behind a drawer) can refresh its tag chips. */
	onChanged?: () => void;
}

type PendingTagAction =
	| { kind: "add"; source: "form" | "suggestion"; name: string }
	| { kind: "remove"; key: string }
	| null;

function tagActionKey(tag: CustomerTag): string {
	return tag.id ? `id-${tag.id}` : `name-${tag.name}`;
}

/**
 * Inline tag manager for one CRM customer. Reads the buyer's tags (with ids),
 * lets the seller attach free-text / suggested tags and remove existing ones.
 * Backed by `…/customers/{id}/tags` + `…/sellers/me/tags` — see
 * `lib/seller/repo.ts`. Mutations keep their initiating control mounted while
 * pending, then reconcile the authoritative tag list from the server.
 */
export default function CustomerTagEditor({
	buyerId,
	initialTags,
	onChanged,
}: Props) {
	// The buyer's applied tags are local editable state — seeded from
	// the detail's initialTags, then reconciled from the authoritative per-buyer
	// endpoint. Kept as a direct fetcher (not a shared query) so a background
	// refetch can't clobber an in-flight add/remove.
	const [tags, setTags] = React.useState<CustomerTag[]>(() =>
		(initialTags ?? []).map((name) => ({ id: "", name })),
	);
	// The seller's tag pool (suggestions) is display-only and shared with the
	// list filter chips → one cached query.
	const pool = useSellerTags();
	const [input, setInput] = React.useState("");
	const [pendingAction, setPendingAction] =
		React.useState<PendingTagAction>(null);
	const [error, setError] = React.useState<string | null>(null);
	const busy = pendingAction !== null;

	const reload = React.useCallback(async () => {
		const fresh = await fetchCustomerTags(buyerId);
		setTags(fresh);
	}, [buyerId]);

	React.useEffect(() => {
		let alive = true;
		fetchCustomerTags(buyerId)
			.then((mine) => {
				if (alive) setTags(mine);
			})
			.catch(() => {
				/* tags are non-critical — leave the initial display tags in place */
			});
		return () => {
			alive = false;
		};
	}, [buyerId]);

	const has = (name: string) =>
		tags.some((t) => t.name.toLowerCase() === name.toLowerCase());

	const add = async (name: string, source: "form" | "suggestion") => {
		const clean = name.trim();
		if (!clean || has(clean) || busy) return;
		setPendingAction({ kind: "add", source, name: clean });
		setError(null);
		setInput("");
		try {
			await addCustomerTags(buyerId, [clean]);
			await reload();
			onChanged?.();
		} catch (e) {
			setError(e instanceof Error ? e.message : "新增標籤失敗");
		} finally {
			setPendingAction(null);
		}
	};

	const remove = async (tag: CustomerTag) => {
		if (busy) return;
		setPendingAction({ kind: "remove", key: tagActionKey(tag) });
		setError(null);
		try {
			// id:"" is an initial display-only chip; no server removal is needed.
			if (tag.id) await removeCustomerTag(buyerId, tag.id);
			setTags((prev) => prev.filter((t) => t !== tag));
			onChanged?.();
		} catch (e) {
			setError(e instanceof Error ? e.message : "移除標籤失敗");
		} finally {
			setPendingAction(null);
		}
	};

	// Suggestions = quick presets + seller's own pool, minus what's applied.
	const suggestions = React.useMemo(() => {
		const seen = new Set(tags.map((t) => t.name.toLowerCase()));
		const out: string[] = [];
		for (const name of [...QUICK_TAGS, ...pool.map((t) => t.name)]) {
			const key = name.toLowerCase();
			if (name && !seen.has(key)) {
				seen.add(key);
				out.push(name);
			}
		}
		return out.slice(0, 12);
	}, [tags, pool]);

	return (
		<div className="crm-tag-editor">
			<div className="crm-tag-editor__head">
				<AppIcon name="tag" size={16} />
				<span>客戶標籤</span>
			</div>

			<div className="crm-tag-editor__chips">
				{tags.length === 0 && (
					<span className="crm-tag-editor__empty">尚未貼標籤</span>
				)}
				{tags.map((tag) => (
					<span
						key={tagActionKey(tag)}
						className="seller-customers__tag crm-tag-editor__chip"
					>
						{tag.name}
						<Button
							type="button"
							variant="light"
							color="default"
							size="sm"
							isIconOnly
							className="crm-tag-editor__remove h-4 w-4 p-0"
							aria-label={`移除標籤 ${tag.name}`}
							onClick={() => void remove(tag)}
							disabled={busy}
							isLoading={
								pendingAction?.kind === "remove" &&
								pendingAction.key === tagActionKey(tag)
							}
							loadingLabel="移除客戶標籤中…"
						>
							<AppIcon name="close" size={14} />
						</Button>
					</span>
				))}
			</div>

			<form
				className="crm-tag-editor__add"
				onSubmit={(e) => {
					e.preventDefault();
					void add(input, "form");
				}}
			>
				<input
					type="text"
					value={input}
					onChange={(e) => setInput(e.target.value)}
					placeholder="輸入標籤後按 Enter…"
					maxLength={16}
					aria-label="新增客戶標籤"
					disabled={busy}
				/>
				<Button
					type="submit"
					size="sm"
					className="crm-tag-editor__submit"
					disabled={busy || !input.trim()}
					isLoading={
						pendingAction?.kind === "add" && pendingAction.source === "form"
					}
					loadingLabel="新增客戶標籤中…"
				>
					新增
				</Button>
			</form>

			{suggestions.length > 0 && (
				<div className="crm-tag-editor__suggestions">
					{suggestions.map((name) => (
						<Button
							key={name}
							type="button"
							variant="bordered"
							color="default"
							size="sm"
							className="crm-tag-editor__suggestion h-auto px-2 py-0.5"
							onClick={() => void add(name, "suggestion")}
							disabled={busy}
							isLoading={
								pendingAction?.kind === "add" &&
								pendingAction.source === "suggestion" &&
								pendingAction.name === name
							}
							loadingLabel="新增客戶標籤中…"
						>
							＋ {name}
						</Button>
					))}
				</div>
			)}

			{error && (
				<p className="crm-tag-editor__error" role="alert">
					{error}
				</p>
			)}
		</div>
	);
}
