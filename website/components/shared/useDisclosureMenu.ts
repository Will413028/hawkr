"use client";

import * as React from "react";

/**
 * Shared behaviour for the navbar account dropdowns (buyer BuyerMenu, seller
 * SellerAccountMenu + SellerMenu). All three used to carry byte-identical
 * copies of the outside-click / Escape effect, and none of them implemented the
 * keyboard contract that `role="menu"` promises: no roving tabindex, no
 * Arrow/Home/End, no focus move into the panel, and Escape never returned focus
 * to the trigger.
 *
 * What this hook owns
 * ────────────────────────────────────────────────────────────
 *  · outside pointer-down → close (skipped while `isLocked()` — a sign-out in
 *    flight keeps its own panel mounted so the user can see the pending state)
 *  · Escape → close AND return focus to the trigger
 *  · ArrowDown / ArrowUp / Home / End → roving focus across the panel's
 *    `[role="menuitem"]` children, with tabindex managed here so the whole menu
 *    is a single tab stop
 *  · opened from the keyboard → focus the first item; opened by pointer → leave
 *    focus on the trigger
 *  · `aria-controls` / panel `id` wiring
 *
 * tabindex is applied imperatively to the live DOM nodes rather than through
 * React props: the three call sites render heterogeneous item lists (links,
 * locked buttons, the logout Button) from several separate `map`s, so an
 * index-per-item prop contract would have to be threaded — and kept in sync —
 * through every branch. Callers must therefore NOT set `tabIndex` on menu
 * items themselves.
 */

const MENU_ITEM_SELECTOR = '[role="menuitem"]';

function menuItems(panel: HTMLElement | null): HTMLElement[] {
	if (!panel) return [];
	return Array.from(
		panel.querySelectorAll<HTMLElement>(MENU_ITEM_SELECTOR),
	).filter((element) => element.getClientRects().length > 0);
}

function rove(items: HTMLElement[], index: number) {
	for (let i = 0; i < items.length; i += 1) {
		items[i].tabIndex = i === index ? 0 : -1;
	}
	items[index]?.focus({ preventScroll: true });
}

export interface UseDisclosureMenuOptions {
	open: boolean;
	onClose: () => void;
	/** Called when a keyboard user asks for the menu from the closed trigger. */
	onOpen?: () => void;
	triggerRef: React.RefObject<HTMLElement | null>;
	panelRef: React.RefObject<HTMLElement | null>;
	/**
	 * While this returns true the menu refuses to close on Escape / outside
	 * click. Used for the in-flight sign-out, whose result the panel is showing.
	 */
	isLocked?: () => boolean;
}

export interface DisclosureMenu {
	panelId: string;
	triggerProps: {
		"aria-haspopup": "menu";
		"aria-expanded": boolean;
		"aria-controls": string | undefined;
		onPointerDown: () => void;
		onKeyDown: (event: React.KeyboardEvent) => void;
	};
	panelProps: {
		id: string;
		role: "menu";
		onKeyDown: (event: React.KeyboardEvent) => void;
		onFocusCapture: (event: React.FocusEvent) => void;
	};
	/**
	 * Spread onto every item. `tabIndex` is deliberately NOT part of this
	 * object: the hook writes it straight to the DOM, and a React-controlled
	 * tabIndex would be reset to its declared value on the next re-render.
	 */
	menuItemProps: { role: "menuitem" };
}

export function useDisclosureMenu({
	open,
	onClose,
	onOpen,
	triggerRef,
	panelRef,
	isLocked,
}: UseDisclosureMenuOptions): DisclosureMenu {
	const panelId = `${React.useId()}-menu`;
	// Pointer activation must not steal focus from the trigger; keyboard
	// activation must land on the first item. A pointerdown on the trigger
	// always precedes the click that flips `open`.
	const openedByPointer = React.useRef(false);

	const onCloseRef = React.useRef(onClose);
	onCloseRef.current = onClose;
	const isLockedRef = React.useRef(isLocked);
	isLockedRef.current = isLocked;

	React.useEffect(() => {
		if (!open) return;
		const locked = () => isLockedRef.current?.() ?? false;
		const onDocumentPointerDown = (event: MouseEvent) => {
			if (locked()) return;
			const target = event.target as Node | null;
			if (!target) return;
			if (triggerRef.current?.contains(target)) return;
			if (panelRef.current?.contains(target)) return;
			onCloseRef.current();
		};
		const onDocumentKeyDown = (event: KeyboardEvent) => {
			if (event.key !== "Escape" || locked()) return;
			onCloseRef.current();
			const trigger = triggerRef.current;
			if (trigger && document.activeElement !== trigger) {
				trigger.focus({ preventScroll: true });
			}
		};
		document.addEventListener("mousedown", onDocumentPointerDown);
		document.addEventListener("keydown", onDocumentKeyDown);
		return () => {
			document.removeEventListener("mousedown", onDocumentPointerDown);
			document.removeEventListener("keydown", onDocumentKeyDown);
		};
	}, [open, triggerRef, panelRef]);

	// Seed the roving tabindex whenever the panel mounts, and enter it when the
	// menu was summoned from the keyboard.
	React.useEffect(() => {
		if (!open) {
			openedByPointer.current = false;
			return;
		}
		const items = menuItems(panelRef.current);
		for (let i = 0; i < items.length; i += 1) {
			items[i].tabIndex = i === 0 ? 0 : -1;
		}
		if (!openedByPointer.current) items[0]?.focus({ preventScroll: true });
		openedByPointer.current = false;
	}, [open, panelRef]);

	const onPanelKeyDown = React.useCallback(
		(event: React.KeyboardEvent) => {
			const { key } = event;
			if (
				key !== "ArrowDown" &&
				key !== "ArrowUp" &&
				key !== "Home" &&
				key !== "End"
			) {
				return;
			}
			const items = menuItems(panelRef.current);
			if (items.length === 0) return;
			event.preventDefault();
			const current = items.indexOf(document.activeElement as HTMLElement);
			let next: number;
			if (key === "Home") next = 0;
			else if (key === "End") next = items.length - 1;
			else if (key === "ArrowDown")
				next = current < 0 ? 0 : (current + 1) % items.length;
			else next = current <= 0 ? items.length - 1 : current - 1;
			rove(items, next);
		},
		[panelRef],
	);

	const onPanelFocusCapture = React.useCallback(
		(event: React.FocusEvent) => {
			const item = (event.target as HTMLElement).closest?.(
				MENU_ITEM_SELECTOR,
			) as HTMLElement | null;
			if (!item) return;
			for (const element of menuItems(panelRef.current)) {
				element.tabIndex = element === item ? 0 : -1;
			}
		},
		[panelRef],
	);

	const onTriggerKeyDown = React.useCallback(
		(event: React.KeyboardEvent) => {
			if (event.key !== "ArrowDown" && event.key !== "ArrowUp") return;
			if (open) {
				event.preventDefault();
				const items = menuItems(panelRef.current);
				if (items.length === 0) return;
				rove(items, event.key === "ArrowDown" ? 0 : items.length - 1);
				return;
			}
			if (!onOpen) return;
			event.preventDefault();
			openedByPointer.current = false;
			onOpen();
		},
		[open, onOpen, panelRef],
	);

	const onTriggerPointerDown = React.useCallback(() => {
		openedByPointer.current = true;
	}, []);

	return {
		panelId,
		triggerProps: {
			"aria-haspopup": "menu",
			"aria-expanded": open,
			"aria-controls": open ? panelId : undefined,
			onPointerDown: onTriggerPointerDown,
			onKeyDown: onTriggerKeyDown,
		},
		panelProps: {
			id: panelId,
			role: "menu",
			onKeyDown: onPanelKeyDown,
			onFocusCapture: onPanelFocusCapture,
		},
		menuItemProps: { role: "menuitem" },
	};
}

export default useDisclosureMenu;
