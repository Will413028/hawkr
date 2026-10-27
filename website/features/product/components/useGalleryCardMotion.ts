"use client";

import { animate, useMotionValue } from "framer-motion";
import {
	type KeyboardEvent,
	type PointerEvent,
	type RefObject,
	useCallback,
	useEffect,
	useRef,
	useState,
} from "react";
import { usePrefersReducedMotion } from "@/lib/a11y/reduced-motion";
import {
	galleryIndexAfterSwipe,
	PRODUCT_GALLERY_DRAG_START_PX,
	PRODUCT_GALLERY_SLIDE_GAP_PX,
} from "@/lib/product/gallery-swipe";

export function useGalleryCardMotion(
	count: number,
	frame: RefObject<HTMLElement | null>,
) {
	const [activeIndex, setActiveIndex] = useState(0);
	const [dragging, setDragging] = useState(false);
	const position = useMotionValue(0);
	const currentIndex = useRef(0);
	const reducedMotion = usePrefersReducedMotion();
	const gesture = useRef<{
		id: number;
		x: number;
		y: number;
		start: number;
		width: number;
		horizontal: boolean;
	} | null>(null);
	const settle = useCallback(
		(index: number) => {
			const target = Math.max(0, Math.min(count - 1, index));
			gesture.current = null;
			setDragging(false);
			position.stop();
			currentIndex.current = target;
			setActiveIndex(target);
			if (reducedMotion) position.set(target);
			else
				animate(position, target, { duration: 0.32, ease: [0.22, 1, 0.36, 1] });
		},
		[count, position, reducedMotion],
	);

	useEffect(() => {
		if (reducedMotion || currentIndex.current >= count) {
			const target = currentIndex.current >= count ? 0 : currentIndex.current;
			gesture.current = null;
			setDragging(false);
			position.stop();
			position.set(target);
			currentIndex.current = target;
			setActiveIndex(target);
		}
	}, [count, reducedMotion, position]);
	useEffect(() => () => position.stop(), [position]);

	const release = (event: PointerEvent<HTMLElement>) => {
		gesture.current = null;
		setDragging(false);
		if (event.currentTarget.hasPointerCapture?.(event.pointerId)) {
			event.currentTarget.releasePointerCapture(event.pointerId);
		}
	};
	const cancel = (event: PointerEvent<HTMLElement>) => {
		if (gesture.current?.id !== event.pointerId) return;
		release(event);
		settle(activeIndex);
	};
	const onPointerDown = (event: PointerEvent<HTMLElement>) => {
		if (
			count < 2 ||
			gesture.current ||
			event.isPrimary === false ||
			event.button !== 0
		)
			return;
		position.stop();
		gesture.current = {
			id: event.pointerId,
			x: event.clientX,
			y: event.clientY,
			start: position.get(),
			width: frame.current?.getBoundingClientRect().width || 1,
			horizontal: false,
		};
		event.currentTarget.setPointerCapture?.(event.pointerId);
	};
	const onPointerMove = (event: PointerEvent<HTMLElement>) => {
		const drag = gesture.current;
		if (!drag || drag.id !== event.pointerId) return;
		const dx = event.clientX - drag.x;
		const dy = event.clientY - drag.y;
		if (!drag.horizontal) {
			if (Math.max(Math.abs(dx), Math.abs(dy)) <= PRODUCT_GALLERY_DRAG_START_PX)
				return;
			if (Math.abs(dy) > Math.abs(dx)) {
				cancel(event);
				return;
			}
			drag.horizontal = true;
			setDragging(true);
		}
		if (reducedMotion) return;
		let next = drag.start - dx / (drag.width + PRODUCT_GALLERY_SLIDE_GAP_PX);
		if (next < 0) next *= 0.3;
		else if (next > count - 1) next = count - 1 + (next - count + 1) * 0.3;
		// MotionValue updates compositor transforms without rerendering every
		// image and thumbnail on every pointer event.
		position.set(next);
	};
	const onPointerUp = (event: PointerEvent<HTMLElement>) => {
		const drag = gesture.current;
		if (!drag || drag.id !== event.pointerId) return;
		const target = drag.horizontal
			? galleryIndexAfterSwipe(
					Math.round(drag.start),
					count,
					event.clientX - drag.x,
				)
			: activeIndex;
		release(event);
		settle(target);
	};
	const onKeyDown = (event: KeyboardEvent<HTMLElement>) => {
		const target =
			event.key === "ArrowRight"
				? currentIndex.current + 1
				: event.key === "ArrowLeft"
					? currentIndex.current - 1
					: event.key === "Home"
						? 0
						: event.key === "End"
							? count - 1
							: null;
		if (target === null || count < 2) return;
		event.preventDefault();
		settle(target);
	};
	return {
		activeIndex,
		dragging,
		position,
		reducedMotion,
		select: settle,
		handlers: {
			onPointerDown,
			onPointerMove,
			onPointerUp,
			onPointerCancel: cancel,
			onLostPointerCapture: cancel,
			onKeyDown,
		},
	};
}
