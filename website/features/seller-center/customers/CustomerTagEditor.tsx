"use client";

import { errorText } from "@hawkr/api/connect/errors";
import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import Button from "@/components/ui/Button";
import { useCrmStoreId } from "@/lib/hooks/useCrmStoreId";
import { useSellerTags } from "@/lib/hooks/useSellerTags";
import { useDefaultErrorText } from "@/lib/i18n/ui-text";
import {
	addCustomerTags,
	type CustomerTag,
	removeCustomerTag,
} from "@/lib/seller/crm";

/** Quick-add presets — mirror the app's tag bottom-sheet suggestions, kept
 * neutral per the brand voice (no judgemental「奧客」labels in web UI). */
const QUICK_TAGS = ["新客", "回購", "高消費", "VIP", "待聯繫"];

interface Props {
	buyerId: string | number;
	/** The customer's tags (`Customer.tags`, with ids), from the list row or the detail. */
	initialTags?: CustomerTag[];
	/** Fired after a successful add / remove so a host (e.g. the customer list
	 * behind a drawer) can refresh its tag chips. */
	onChanged?: () => void;
}

type PendingTagAction =
	| { kind: "add"; source: "form" | "suggestion"; name: string }
	| { kind: "remove"; key: string }
	| null;

function tagActionKey(tag: CustomerTag): string {
	return `id-${tag.id}`;
}

/**
 * Inline tag manager for one CRM customer. Attaches free-text / suggested tags
 * (`AddCustomerTags`) and removes existing ones (`RemoveCustomerTag`); both return
 * the customer, whose tags become the new list. Mutations keep their initiating
 * control mounted while pending.
 */
export default function CustomerTagEditor({
	buyerId,
	initialTags,
	onChanged,
}: Props) {
	const { storeId } = useCrmStoreId();
	const defaults = useDefaultErrorText();
	// The buyer's applied tags: seeded from the row / detail, replaced by the customer each
	// mutation returns, and re-seeded when the host hands over a fresher list.
	const [tags, setTags] = React.useState<CustomerTag[]>(initialTags ?? []);
	React.useEffect(() => {
		if (initialTags) setTags(initialTags);
	}, [initialTags]);
	// The seller's tag pool (suggestions) is display-only and shared with the
	// list filter chips → one cached query.
	const { tags: pool, error: poolError } = useSellerTags();
	const [input, setInput] = React.useState("");
	const [pendingAction, setPendingAction] =
		React.useState<PendingTagAction>(null);
	const [error, setError] = React.useState<string | null>(null);
	const busy = pendingAction !== null;

	// 每次寫入回傳貼完（或移除完）之後的客戶：它的標籤成為新的列表。
	const applyCustomer = React.useCallback(
		(customer: { tags: CustomerTag[] }) => setTags(customer.tags),
		[],
	);

	const has = (name: string) =>
		tags.some((t) => t.name.toLowerCase() === name.toLowerCase());

	const add = async (name: string, source: "form" | "suggestion") => {
		const clean = name.trim();
		if (!clean || has(clean) || busy || !storeId) return;
		setPendingAction({ kind: "add", source, name: clean });
		setError(null);
		setInput("");
		try {
			const customer = await addCustomerTags(storeId, String(buyerId), [clean]);
			applyCustomer(customer);
			onChanged?.();
		} catch (e) {
			setError(errorText(e, defaults));
		} finally {
			setPendingAction(null);
		}
	};

	const remove = async (tag: CustomerTag) => {
		if (busy || !storeId) return;
		setPendingAction({ kind: "remove", key: tagActionKey(tag) });
		setError(null);
		try {
			const customer = await removeCustomerTag(
				storeId,
				String(buyerId),
				tag.id,
			);
			applyCustomer(customer);
			onChanged?.();
		} catch (e) {
			setError(errorText(e, defaults));
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

			{(error ?? poolError) && (
				<p className="crm-tag-editor__error" role="alert">
					{error ?? poolError}
				</p>
			)}
		</div>
	);
}
