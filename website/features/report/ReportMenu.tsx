"use client";

import { useTranslations } from "next-intl";
import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import AuthGateWaitModal from "@/components/shared/AuthGateWaitModal";
import useDisclosureMenu from "@/components/shared/useDisclosureMenu";
import Button from "@/components/ui/Button";
import { useAuthGate } from "@/lib/auth/useAuthGate";
import { useLoginRequiredOverlay } from "@/lib/state/use-login-required-overlay";
import ReportDialog from "./ReportDialog";

export interface ReportMenuProps {
	storeId: string;
	product?: { id: string; name: string };
	className?: string;
}

/**
 * 「⋯」選單，只有一項「檢舉」。訪客點檢舉先走登入引導（共用 LoginRequiredModal），登入後才開表單。
 */
export default function ReportMenu({
	storeId,
	product,
	className,
}: ReportMenuProps) {
	const t = useTranslations("report");
	const [open, setOpen] = React.useState(false);
	const [formOpen, setFormOpen] = React.useState(false);
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
						className="flex w-full cursor-pointer items-center rounded-lg border-0 bg-transparent px-3 py-2 text-left text-sm text-hk-body hover:bg-[var(--hkui-content2)]"
						{...menu.menuItemProps}
						onClick={() => {
							setOpen(false);
							authGate(() => setFormOpen(true));
						}}
					>
						{t("menuItem")}
					</button>
				</div>
			) : null}
			<AuthGateWaitModal open={authWaiting} />
			<ReportDialog
				open={formOpen}
				onClose={() => setFormOpen(false)}
				storeId={storeId}
				product={product}
			/>
		</span>
	);
}
