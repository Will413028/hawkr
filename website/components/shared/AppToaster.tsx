"use client";

import * as React from "react";
import ToastIcon from "@/components/shared/ToastIcon";
import { markToastViewportReady } from "@/lib/feedback/toast";
import { whenDocumentSettled } from "@/lib/hydration/document-settled";

type SonnerToaster = typeof import("sonner")["Toaster"];

interface AppToasterProps {
	id?: string;
	containerAriaLabel?: string;
}

const MOBILE_DOCK_QUERY = "not (min-width: 840px)";

const DESKTOP_TOP_OFFSET =
	"calc(var(--floating-nav-height, 84px) + var(--floating-nav-top-gap, 16px) + 12px + var(--hk-notif-toast-h, 0px))";

const MOBILE_DOCK_BOTTOM_OFFSET =
	"calc(var(--mobile-bottom-nav-height, 68px) + var(--mobile-bottom-nav-bottom, 10px) + 12px)";

/**
 * Shared Sonner presentation. A scoped `id` lets top-layer surfaces such as a
 * native modal dialog own their notifications without duplicating them in the
 * app-level toaster behind the dialog backdrop.
 */
export default function AppToaster({
	id,
	containerAriaLabel = "系統通知",
}: AppToasterProps) {
	const [Toaster, setToaster] = React.useState<SonnerToaster | null>(null);
	const [loadFailed, setLoadFailed] = React.useState(false);

	const [dockBottom, setDockBottom] = React.useState(false);

	React.useEffect(() => {
		if (id) return;
		const mql = window.matchMedia(MOBILE_DOCK_QUERY);
		const sync = () => setDockBottom(mql.matches);
		sync();
		mql.addEventListener("change", sync);
		return () => mql.removeEventListener("change", sync);
	}, [id]);

	React.useEffect(() => {
		let active = true;

		void whenDocumentSettled()
			.then(() => import("sonner"))
			.catch(() => import("sonner"))
			.then((module) => {
				if (active) setToaster(() => module.Toaster);
			})
			.catch(() => {
				if (active) setLoadFailed(true);
			});

		return () => {
			active = false;
		};
	}, []);

	React.useEffect(() => {
		if (!Toaster) return;
		const frame = window.requestAnimationFrame(markToastViewportReady);
		return () => window.cancelAnimationFrame(frame);
	}, [Toaster]);

	if (!Toaster && loadFailed && !id) {
		return (
			<output
				aria-live="polite"
				className="fixed inset-x-3 bottom-3 z-[var(--z-toast)] mx-auto block w-fit max-w-[calc(100vw-1.5rem)] rounded-hk-pill border border-hk-border-soft bg-hk-surface-card px-4 py-2 text-xs text-hk-body"
			>
				通知元件載入失敗，操作結果可能不會顯示，請重新整理頁面。
			</output>
		);
	}
	if (!Toaster) return null;

	return (
		<Toaster
			id={id}
			className="hk-sonner-toaster"
			position={dockBottom ? "bottom-center" : "top-center"}
			closeButton
			gap={8}
			visibleToasts={4}
			offset={
				dockBottom
					? { bottom: MOBILE_DOCK_BOTTOM_OFFSET }
					: {
							top: id ? 12 : DESKTOP_TOP_OFFSET,
						}
			}
			mobileOffset={
				dockBottom
					? { bottom: MOBILE_DOCK_BOTTOM_OFFSET, left: 12, right: 12 }
					: {
							top: 8,
							left: 12,
							right: 12,
						}
			}
			swipeDirections={
				dockBottom ? ["bottom", "left", "right"] : ["top", "left", "right"]
			}
			containerAriaLabel={containerAriaLabel}
			icons={{
				success: <ToastIcon kind="success" />,
				error: <ToastIcon kind="error" />,
				warning: <ToastIcon kind="warning" />,
				info: <ToastIcon kind="info" />,
				loading: <ToastIcon kind="loading" />,
			}}
			toastOptions={{
				closeButtonAriaLabel: "關閉通知",
				classNames: {
					toast: "hk-sonner-toast",
					content: "hk-sonner-toast__content",
					title: "hk-sonner-toast__title",
					description: "hk-sonner-toast__description",
					icon: "hk-sonner-toast__icon",
					closeButton: "hk-sonner-toast__close",
					actionButton: "hk-sonner-toast__action",
					cancelButton: "hk-sonner-toast__cancel",
				},
			}}
		/>
	);
}
