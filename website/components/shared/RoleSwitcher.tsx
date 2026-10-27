"use client";

import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import { useAuth } from "@/lib/auth/context";
import { usePathname } from "@/lib/intl/navigation";
import { isStandaloneFlowPath } from "@/lib/navigation/standalone-flow";

/**
 * Dev-only role inspector (bottom-right FAB). Not shipped to production —
 * `DevRoleSwitcherSlot` dynamic-imports it — but it is still a popup, so it
 * behaves like one: the trigger comes FIRST in the DOM (Tab used to land in
 * the panel before reaching the button that opens it), Escape and an outside
 * click close it, and focus returns to the FAB afterwards.
 */
export default function RoleSwitcher() {
	const pathname = usePathname() ?? "";
	const { activeRole, isLoggedIn, signOut } = useAuth();
	const [open, setOpen] = React.useState(false);
	const rootRef = React.useRef<HTMLDivElement>(null);
	const fabRef = React.useRef<HTMLButtonElement>(null);
	const panelId = React.useId();
	const titleId = React.useId();

	const close = React.useCallback((returnFocus: boolean) => {
		setOpen(false);
		if (returnFocus) fabRef.current?.focus();
	}, []);

	React.useEffect(() => {
		if (!open) return;
		const onKeyDown = (event: KeyboardEvent) => {
			if (event.key !== "Escape" || event.defaultPrevented) return;
			event.preventDefault();
			close(true);
		};
		const onPointerDown = (event: PointerEvent) => {
			const target = event.target;
			if (target instanceof Node && rootRef.current?.contains(target)) return;
			close(false);
		};
		document.addEventListener("keydown", onKeyDown);
		document.addEventListener("pointerdown", onPointerDown);
		return () => {
			document.removeEventListener("keydown", onKeyDown);
			document.removeEventListener("pointerdown", onPointerDown);
		};
	}, [open, close]);

	if (!pathname || isStandaloneFlowPath(pathname)) return null;

	return (
		<div className="role-switcher" data-open={open} ref={rootRef}>
			<button
				ref={fabRef}
				type="button"
				// The CSS box is 52px; the floor is declared here too so the
				// component guarantees its own target size.
				className="role-switcher__fab min-h-11 min-w-11"
				onClick={() => setOpen((v) => !v)}
				aria-label="切換登入角色"
				aria-expanded={open}
				aria-controls={panelId}
				aria-haspopup="dialog"
			>
				<span className="role-switcher__fab-icon" aria-hidden="true">
					{activeRole === "buyer" ? (
						<BuyerRoleIcon />
					) : activeRole === "seller" ? (
						<SellerRoleIcon />
					) : (
						<GuestRoleIcon />
					)}
				</span>
			</button>
			{open && (
				// `role="dialog"`, not `menu`: the panel is mostly read-only status
				// text with at most one action, and a menu whose children are not
				// menuitems is announced wrongly.
				<div
					id={panelId}
					className="role-switcher__panel"
					role="dialog"
					aria-labelledby={titleId}
				>
					<div className="role-switcher__title" id={titleId}>
						DEV · 目前角色
					</div>
					<div className="role-switcher__hint px-3 py-2">
						{isLoggedIn
							? `已登入 · 活動角色：${activeRole ?? "未設定"}`
							: "未登入 (訪客)"}
					</div>
					{/* Role is driven by the signed-in account + the stored
					    workspace. Use switchRole() from the navbar role switcher UI
					    instead of a dev override. */}
					{isLoggedIn && (
						<button
							type="button"
							className="role-switcher__option mt-1 border-t border-[var(--hkui-divider)] text-[var(--hkui-danger)]"
							onClick={() => {
								// 撤銷失敗時 signOut 會 reject（session 仍有效，畫面維持
								// 登入）；它已跳 toast，這裡只吸收 rejection。
								void signOut().catch(() => {});
								close(true);
							}}
						>
							<span className="role-switcher__icon" aria-hidden="true">
								<SignOutIcon />
							</span>
							<span>登出</span>
						</button>
					)}
				</div>
			)}
		</div>
	);
}

function BuyerRoleIcon() {
	return <AppIcon name="buy" size={24} />;
}

function SellerRoleIcon() {
	return <AppIcon name="leaf" size={24} />;
}

function GuestRoleIcon() {
	return <AppIcon name="user" size={24} />;
}

function SignOutIcon() {
	return <AppIcon name="logout" size={24} />;
}
