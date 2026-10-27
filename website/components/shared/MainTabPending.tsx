"use client";

import { useLayoutEffect, useRef, useState, useSyncExternalStore } from "react";
import RouteLoadingSkeleton from "@/components/shared/RouteLoadingSkeleton";
import { usePathname } from "@/lib/intl/navigation";
import {
	completeMainTabPending,
	getMainTabPendingServerSnapshot,
	getMainTabPendingSnapshot,
	subscribeMainTabPending,
} from "@/lib/navigation/main-tab-pending";
import "@/styles/components/main-tab-pending.css";

const labels = {
	home: "首頁",
	feed: "動態",
	"market-list": "市集",
	search: "搜尋",
};

export const MAIN_TAB_SKELETON_DELAY_MS = 100;

const scrollKeys = new Set([
	"ArrowUp",
	"ArrowDown",
	"PageUp",
	"PageDown",
	"Home",
	"End",
	" ",
]);

/** Locally available feedback, including before a cold route's Flight arrives.
 * Keep the server-owned route tree mounted and at its original height so its
 * history/scroll snapshot remains accurate until Next commits the destination.
 * Only the content becomes inert; the persistent navigation stays usable. */
export default function MainTabPending() {
	const pathname = usePathname() ?? "";
	const pending = useSyncExternalStore(
		subscribeMainTabPending,
		getMainTabPendingSnapshot,
		getMainTabPendingServerSnapshot,
	);
	const surfaceRef = useRef<HTMLElement>(null);
	const [revealed, setRevealed] = useState(false);
	const intent = pending?.sourcePathname === pathname ? pending : null;
	const visible = revealed ? intent : null;

	useLayoutEffect(() => {
		if (!intent) {
			setRevealed(false);
			return;
		}
		// The navigation selection/progress still responds synchronously via the
		// pending store. Only delay the skeleton: cached content can commit before
		// another surface paints, without guessing that a visited tab is ready.
		// Once visible, a newer destination updates the same surface immediately.
		if (revealed) return;
		const timer = setTimeout(
			() => setRevealed(true),
			MAIN_TAB_SKELETON_DELAY_MS,
		);
		return () => clearTimeout(timer);
	}, [intent, revealed]);

	useLayoutEffect(() => {
		if (pending) completeMainTabPending(pathname);
	}, [pathname, pending]);

	useLayoutEffect(() => {
		if (!visible) return;
		const content = document.querySelector<HTMLElement>(
			"main.page-content > .page-transition",
		);
		if (!content) return;
		const root = document.documentElement;
		const previousInert = content.inert;
		const previousAriaHidden = content.getAttribute("aria-hidden");
		content.inert = true;
		content.setAttribute("aria-hidden", "true");
		root.setAttribute("data-main-tab-pending", visible.variant);
		// A wheel over the placeholder must not move the hidden source page.
		const surface = surfaceRef.current;
		const preventScroll = (event: WheelEvent) => event.preventDefault();
		const preventKeyboardScroll = (event: KeyboardEvent) => {
			if (
				event.defaultPrevented ||
				event.altKey ||
				event.ctrlKey ||
				event.metaKey
			)
				return;
			if (!scrollKeys.has(event.key)) return;
			if (
				event.target instanceof Element &&
				event.target.closest(
					'input, textarea, select, button, [contenteditable]:not([contenteditable="false"]), [role="textbox"], [role="combobox"], [role="slider"], [role="spinbutton"], [role="listbox"], [role="menu"], [role="tree"]',
				)
			)
				return;
			event.preventDefault();
		};
		surface?.addEventListener("wheel", preventScroll, { passive: false });
		document.addEventListener("keydown", preventKeyboardScroll);
		return () => {
			content.inert = previousInert;
			if (previousAriaHidden === null) content.removeAttribute("aria-hidden");
			else content.setAttribute("aria-hidden", previousAriaHidden);
			root.removeAttribute("data-main-tab-pending");
			surface?.removeEventListener("wheel", preventScroll);
			document.removeEventListener("keydown", preventKeyboardScroll);
		};
	}, [visible]);

	if (!visible) return null;
	return (
		<section
			ref={surfaceRef}
			className="main-tab-pending"
			data-main-tab-loading={visible.variant}
			aria-label={`${labels[visible.variant]}內容`}
			aria-busy="true"
		>
			<RouteLoadingSkeleton page variant={visible.variant} />
		</section>
	);
}
