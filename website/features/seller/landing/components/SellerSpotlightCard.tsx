"use client";

import { type PointerEvent, type ReactNode, useEffect, useRef } from "react";

interface SellerSpotlightCardProps {
	children: ReactNode;
	className?: string;
}

/**
 * Pointer-following highlight adapted from React Bits' SpotlightCard pattern.
 * The card remains fully visible without JavaScript; this layer only adds a
 * quiet desktop hover accent.
 *
 * Source: https://github.com/DavidHDev/react-bits
 * License: MIT + Commons Clause
 */
export default function SellerSpotlightCard({
	children,
	className,
}: SellerSpotlightCardProps) {
	const cardRef = useRef<HTMLElement>(null);
	const boundsRef = useRef<DOMRect | null>(null);
	const frameRef = useRef<number | null>(null);
	const pointerRef = useRef({ x: 0, y: 0 });

	useEffect(() => {
		const card = cardRef.current;
		const observer =
			card && typeof ResizeObserver !== "undefined"
				? new ResizeObserver(() => {
						boundsRef.current = null;
					})
				: null;
		if (card && observer) observer.observe(card);

		return () => {
			observer?.disconnect();
			if (frameRef.current !== null) cancelAnimationFrame(frameRef.current);
		};
	}, []);

	const captureBounds = (event: PointerEvent<HTMLElement>) => {
		if (event.pointerType === "touch") return;
		boundsRef.current = event.currentTarget.getBoundingClientRect();
	};

	const handlePointerMove = (event: PointerEvent<HTMLElement>) => {
		if (event.pointerType === "touch") return;

		const target = event.currentTarget;
		pointerRef.current = { x: event.clientX, y: event.clientY };
		if (frameRef.current !== null) return;

		frameRef.current = requestAnimationFrame(() => {
			frameRef.current = null;
			const bounds = boundsRef.current ?? target.getBoundingClientRect();
			const { x, y } = pointerRef.current;
			const relativeX = (x - bounds.left) / bounds.width;
			const relativeY = (y - bounds.top) / bounds.height;

			target.style.setProperty("--spotlight-x", `${x - bounds.left}px`);
			target.style.setProperty("--spotlight-y", `${y - bounds.top}px`);
			target.style.setProperty("--spotlight-opacity", "1");
			target.style.setProperty(
				"--seller-tilt-x",
				`${(0.5 - relativeY) * 5}deg`,
			);
			target.style.setProperty(
				"--seller-tilt-y",
				`${(relativeX - 0.5) * 7}deg`,
			);
			target.style.setProperty("--seller-card-lift", "-4px");
		});
	};

	const hideSpotlight = (event: PointerEvent<HTMLElement>) => {
		if (frameRef.current !== null) {
			cancelAnimationFrame(frameRef.current);
			frameRef.current = null;
		}
		boundsRef.current = null;
		event.currentTarget.style.setProperty("--spotlight-opacity", "0");
		event.currentTarget.style.setProperty("--seller-tilt-x", "0deg");
		event.currentTarget.style.setProperty("--seller-tilt-y", "0deg");
		event.currentTarget.style.setProperty("--seller-card-lift", "0px");
	};

	return (
		<article
			ref={cardRef}
			className={`seller-spotlight-card${className ? ` ${className}` : ""}`}
			onPointerEnter={captureBounds}
			onPointerMove={handlePointerMove}
			onPointerLeave={hideSpotlight}
			onPointerCancel={hideSpotlight}
		>
			{children}
		</article>
	);
}
