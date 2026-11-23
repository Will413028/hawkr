"use client";

import { errorText } from "@hawkr/api/connect/errors";
import { useTranslations } from "next-intl";
import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import AuthGateWaitModal from "@/components/shared/AuthGateWaitModal";
import useDisclosureMenu from "@/components/shared/useDisclosureMenu";
import Button from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import { useAuthGate } from "@/lib/auth/useAuthGate";
import { useDefaultErrorText } from "@/lib/i18n/ui-text";
import { useLoginRequiredOverlay } from "@/lib/state/use-login-required-overlay";
import type { CommunityReportTarget } from "./api";
import CommunityReportDialog from "./CommunityReportDialog";

export interface BlockAction {
	/** 被封鎖的人（確認視窗標題與完成訊息用）。 */
	name: string;
	/** 確認視窗的說明（封鎖的後果，依買家方或店家方不同）。 */
	description: string;
	run: () => Promise<unknown>;
	onBlocked?: () => void;
}

export interface ReportActionMenuProps {
	target: CommunityReportTarget;
	/** 給了才有「封鎖」項（照 App：檢舉＋封鎖在同一個選單）；評價只檢舉，不給。 */
	block?: BlockAction;
	className?: string;
}

type BlockState =
	| { kind: "idle" }
	| { kind: "busy" }
	| { kind: "error"; message: string }
	| { kind: "done" };

/**
 * 「⋯」選單：檢舉，可加封鎖（照 buyer-app／seller-app 的檢舉與封鎖流程）。訪客點檢舉先走登入引導，
 * 登入後才開表單；封鎖要先在確認視窗確認，已經封鎖時也算成功。
 */
export default function ReportActionMenu({
	target,
	block,
	className,
}: ReportActionMenuProps) {
	const t = useTranslations("report");
	const tBlock = useTranslations("seller.comments");
	const defaults = useDefaultErrorText();
	const [open, setOpen] = React.useState(false);
	const [formOpen, setFormOpen] = React.useState(false);
	const [blockOpen, setBlockOpen] = React.useState(false);
	const [blockState, setBlockState] = React.useState<BlockState>({
		kind: "idle",
	});
	const triggerRef = React.useRef<HTMLButtonElement | null>(null);
	const panelRef = React.useRef<HTMLDivElement | null>(null);
	const loginPayload = React.useMemo(
		() => ({
			title: t("loginTitle"),
			description: t("loginDescription"),
			actionLabel: t("loginAction"),
		}),
		[t],
	);
	const openLoginRequired = useLoginRequiredOverlay(loginPayload);
	const { authGate, authWaiting } = useAuthGate(openLoginRequired);
	const menu = useDisclosureMenu({
		open,
		onClose: () => {
			setOpen(false);
			triggerRef.current?.focus();
		},
		onOpen: () => setOpen(true),
		triggerRef,
		panelRef,
	});

	const itemClass =
		"flex w-full cursor-pointer items-center rounded-lg border-0 bg-transparent px-3 py-2 text-left text-sm text-hk-body hover:bg-[var(--hkui-content2)]";

	const confirmBlock = async () => {
		if (!block || blockState.kind === "busy") return;
		setBlockState({ kind: "busy" });
		try {
			await block.run();
			setBlockState({ kind: "done" });
			block.onBlocked?.();
		} catch (e) {
			setBlockState({ kind: "error", message: errorText(e, defaults) });
		}
	};
	const closeBlock = () => {
		if (blockState.kind === "busy") return;
		setBlockOpen(false);
	};

	return (
		<span className={`relative inline-flex ${className ?? ""}`}>
			<Button
				ref={triggerRef as React.Ref<HTMLButtonElement & HTMLAnchorElement>}
				type="button"
				variant="light"
				color="default"
				size="sm"
				radius="full"
				isIconOnly
				aria-label={t("menuLabel")}
				onClick={() => setOpen((v) => !v)}
				{...menu.triggerProps}
			>
				<AppIcon name="more" size={18} />
			</Button>
			{open ? (
				<div
					ref={panelRef}
					className="absolute right-0 top-full z-20 mt-1 min-w-32 rounded-hk-xl border border-hk-border-soft bg-[var(--hkui-content1)] p-1 shadow-[var(--hkui-shadow-sm)]"
					{...menu.panelProps}
				>
					<button
						type="button"
						className={itemClass}
						{...menu.menuItemProps}
						onClick={() => {
							setOpen(false);
							authGate(() => setFormOpen(true));
						}}
					>
						{t("menuItem")}
					</button>
					{block ? (
						<button
							type="button"
							className={itemClass}
							{...menu.menuItemProps}
							onClick={() => {
								setOpen(false);
								setBlockState({ kind: "idle" });
								authGate(() => setBlockOpen(true));
							}}
						>
							{tBlock("blockAction")}
						</button>
					) : null}
				</div>
			) : null}
			<AuthGateWaitModal open={authWaiting} />
			<CommunityReportDialog
				open={formOpen}
				onClose={() => setFormOpen(false)}
				target={target}
			/>
			{block ? (
				<Dialog
					open={blockOpen}
					onClose={closeBlock}
					size="md"
					title={tBlock("blockTitle", { name: block.name })}
					footer={
						<div className="flex w-full justify-end gap-2">
							{blockState.kind === "done" ? (
								<Button onClick={() => setBlockOpen(false)}>
									{t("close")}
								</Button>
							) : (
								<>
									<Button
										variant="light"
										color="default"
										onClick={closeBlock}
										disabled={blockState.kind === "busy"}
									>
										{t("cancel")}
									</Button>
									<Button
										color="danger"
										onClick={() => void confirmBlock()}
										isLoading={blockState.kind === "busy"}
										loadingLabel={t("blocking")}
									>
										{t("blockConfirm")}
									</Button>
								</>
							)}
						</div>
					}
				>
					{blockState.kind === "done" ? (
						<p className="m-0 text-sm text-hk-heading" role="status">
							{tBlock("blocked", { name: block.name })}
						</p>
					) : (
						<div className="grid gap-3">
							<p className="m-0 text-sm text-hk-body">{block.description}</p>
							{blockState.kind === "error" ? (
								<p
									className="m-0 text-sm font-semibold text-danger-700"
									role="alert"
								>
									{blockState.message}
								</p>
							) : null}
						</div>
					)}
				</Dialog>
			) : null}
		</span>
	);
}
