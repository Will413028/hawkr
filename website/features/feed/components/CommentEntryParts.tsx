"use client";

import { useTranslations } from "next-intl";
import * as React from "react";
import Button from "@/components/ui/Button";

/**
 * 留言與討論區發言共用的就地編輯框與操作列（編輯、刪除、封鎖此人）：
 * 貼文留言（PostComments）與店家討論區（SellerMessageTab）是同一套互動。
 */

export function CommentEditor({
	draft,
	onDraftChange,
	onSubmit,
	onCancel,
	saving,
	saveLoadingLabel,
}: {
	draft: string;
	onDraftChange: (draft: string) => void;
	onSubmit: () => void;
	onCancel: () => void;
	saving: boolean;
	saveLoadingLabel: string;
}) {
	const textareaRef = React.useRef<HTMLTextAreaElement | null>(null);
	React.useEffect(() => {
		textareaRef.current?.focus();
	}, []);
	return (
		<div className="flex flex-col gap-[var(--hkui-space-2)] mt-1">
			<textarea
				ref={textareaRef}
				className="w-full border border-divider rounded-hkui-md py-2 px-3 bg-content1 min-h-10 resize-none focus:outline-none focus:border-hkui-primary [font-family:inherit] [font-weight:inherit] leading-5 text-[length:var(--hkui-text-small)]"
				value={draft}
				onChange={(e) => onDraftChange(e.target.value)}
				onKeyDown={(e) => {
					if (e.key === "Enter" && !e.shiftKey && !e.nativeEvent.isComposing) {
						e.preventDefault();
						onSubmit();
					} else if (e.key === "Escape") {
						e.preventDefault();
						onCancel();
					}
				}}
				rows={1}
				maxLength={500}
			/>
			<div className="flex gap-[var(--hkui-space-2)] self-end">
				<Button
					type="button"
					variant="bordered"
					color="default"
					size="sm"
					onClick={onCancel}
				>
					取消
				</Button>
				<Button
					type="button"
					size="sm"
					disabled={!draft.trim() || saving}
					isLoading={saving}
					loadingLabel={saveLoadingLabel}
					onClick={onSubmit}
				>
					儲存
				</Button>
			</div>
		</div>
	);
}

const ACTION_BUTTON =
	"bg-transparent border-0 text-[var(--hkui-foreground-soft)] text-[length:var(--hkui-text-tiny)] cursor-pointer py-[2px] px-[6px] rounded-[var(--hkui-radius-sm)] hover:bg-[var(--hkui-content2)]";

export function CommentEntryActions({
	canEdit,
	canDelete,
	canBlock,
	editDisabled = false,
	deleteDisabled = false,
	blockDisabled = false,
	deleting,
	deleteLoadingLabel,
	onEdit,
	onDelete,
	onBlock,
}: {
	canEdit: boolean;
	canDelete: boolean;
	canBlock: boolean;
	editDisabled?: boolean;
	deleteDisabled?: boolean;
	blockDisabled?: boolean;
	deleting: boolean;
	deleteLoadingLabel: string;
	onEdit: () => void;
	onDelete: () => void;
	onBlock: () => void;
}) {
	const tComments = useTranslations("seller.comments");
	return (
		<>
			{canEdit && (
				<button
					type="button"
					className={`${ACTION_BUTTON} hover:text-[var(--hkui-primary)]`}
					disabled={editDisabled}
					onClick={onEdit}
				>
					編輯
				</button>
			)}
			{canDelete && (
				<Button
					type="button"
					variant="light"
					color="danger"
					size="sm"
					className="h-auto min-w-0 px-[6px] py-[2px] text-[length:var(--hkui-text-tiny)] font-normal text-[var(--hkui-foreground-soft)]"
					disabled={deleteDisabled}
					isLoading={deleting}
					loadingLabel={deleteLoadingLabel}
					onClick={onDelete}
				>
					刪除
				</Button>
			)}
			{canBlock && (
				<button
					type="button"
					className={`${ACTION_BUTTON} hover:text-[var(--hkui-danger)]`}
					disabled={blockDisabled}
					onClick={onBlock}
				>
					{tComments("blockAction")}
				</button>
			)}
		</>
	);
}
