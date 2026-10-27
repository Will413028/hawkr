"use client";

import type { MutableRefObject } from "react";
import { useEffect, useRef, useState } from "react";
import CardDetails from "@/features/content/shared/CardDetails";

type Gsap = typeof import("gsap")["gsap"];

/**
 * A flippable description card, shared by the FAQ and the seller guide.
 *
 * The body is always in the DOM (so search engines and screen readers see it
 * once expanded) but starts collapsed: the outer wrapper has height 0 and the
 * inner page is folded up on its top hinge (rotateX). On open, GSAP grows the
 * wrapper to auto while the inner page swings down like a turning book page.
 * Reduced-motion users get a quick height fade with no 3D flip. If GSAP hasn't
 * loaded yet (or JS is slow), a plain style fallback still opens the card.
 */
export default function ExpandableCard({
	id,
	title,
	body,
	eyebrow,
	note,
	notes,
	steps,
	stepsLabel = "操作步驟",
	href,
	hrefLabel = "前往這個頁面",
	gsapRef,
	reducedMotion,
}: {
	id: string;
	title: string;
	body: string;
	/** Small label above the title — the guide uses it for the section number. */
	eyebrow?: string;
	note?: string;
	notes?: string[];
	steps?: string[];
	stepsLabel?: string;
	/** Optional deep link to the page this card describes. */
	href?: string;
	hrefLabel?: string;
	gsapRef: MutableRefObject<Gsap | null>;
	reducedMotion: boolean;
}) {
	const [open, setOpen] = useState(false);
	const outerRef = useRef<HTMLDivElement>(null);
	const innerRef = useRef<HTMLDivElement>(null);
	const tlRef = useRef<ReturnType<Gsap["timeline"]> | null>(null);

	const toggle = () => {
		const next = !open;
		setOpen(next);

		const outer = outerRef.current;
		const inner = innerRef.current;
		if (!outer || !inner) return;

		const gsap = gsapRef.current;
		if (!gsap) {
			// GSAP not ready — open/close instantly so content stays usable.
			outer.style.height = next ? "auto" : "0px";
			inner.style.opacity = next ? "1" : "0";
			inner.style.transform = next ? "none" : "rotateX(-92deg)";
			return;
		}

		tlRef.current?.kill();
		const tl = gsap.timeline();
		tlRef.current = tl;
		gsap.set(inner, { transformOrigin: "top center" });

		if (next) {
			tl.set(outer, { display: "block" });
			if (reducedMotion) {
				tl.to(
					outer,
					{ height: "auto", duration: 0.2, ease: "power1.out" },
					0,
				).to(inner, { opacity: 1, rotateX: 0, duration: 0.2 }, 0);
			} else {
				tl.to(
					outer,
					{ height: "auto", duration: 0.5, ease: "power3.out" },
					0,
				).fromTo(
					inner,
					{ rotateX: -92, opacity: 0 },
					{ rotateX: 0, opacity: 1, duration: 0.6, ease: "power3.out" },
					0.05,
				);
			}
		} else if (reducedMotion) {
			tl.to(inner, { opacity: 0, duration: 0.15 }, 0).to(
				outer,
				{ height: 0, duration: 0.2, ease: "power1.in" },
				0,
			);
		} else {
			tl.to(
				inner,
				{ rotateX: -92, opacity: 0, duration: 0.32, ease: "power2.in" },
				0,
			).to(outer, { height: 0, duration: 0.4, ease: "power2.inOut" }, 0.05);
		}
	};

	useEffect(
		() => () => {
			tlRef.current?.kill();
		},
		[],
	);

	const allNotes = notes ?? (note ? [note] : []);

	return (
		<div className="faq-card rounded-[20px] p-0 overflow-hidden glass-card glass-card--solid [perspective:1400px]">
			<button
				className="w-full flex items-center justify-between gap-4 p-[17px_25px] border-none bg-transparent cursor-pointer text-left font-[var(--font-chinese)] transition-colors duration-200 hover:bg-hk-surface-glass-hover max-[768px]:p-[14px_20px]"
				type="button"
				aria-expanded={open}
				aria-controls={id}
				onClick={toggle}
			>
				<span className="flex flex-col gap-1">
					{eyebrow && (
						<span className="font-[var(--font-primary)] text-[length:var(--text-sm)] font-bold leading-none text-hk-primary">
							{eyebrow}
						</span>
					)}
					<span className="font-[var(--font-chinese)] text-[length:var(--text-lg)] font-bold text-hk-heading max-[768px]:text-[length:var(--text-base)]">
						{title}
					</span>
				</span>
				<svg
					aria-hidden="true"
					className={`shrink-0 transition-transform duration-300 ease-in-out ${open ? "rotate-180" : ""}`}
					width="32"
					height="32"
					viewBox="0 0 32 32"
					fill="none"
				>
					<circle
						cx="16"
						cy="16"
						r="15"
						stroke="rgba(86,86,86,0.2)"
						strokeWidth="1.5"
					/>
					<polyline
						points="12 14 16 18 20 14"
						stroke="var(--ink-body)"
						strokeWidth="2"
						fill="none"
						strokeLinecap="round"
						strokeLinejoin="round"
					/>
				</svg>
			</button>
			<div
				id={id}
				ref={outerRef}
				className="h-0 overflow-hidden"
				aria-hidden={!open}
			>
				<div
					ref={innerRef}
					className="p-[0_25px_20px] [transform-style:preserve-3d] [backface-visibility:hidden] [will-change:transform,opacity] max-[768px]:p-[0_20px_16px]"
					style={{
						transformOrigin: "top center",
						transform: "rotateX(-92deg)",
						opacity: 0,
					}}
				>
					<CardDetails
						id={id}
						body={body}
						notes={allNotes}
						steps={steps}
						stepsLabel={stepsLabel}
						href={href}
						hrefLabel={hrefLabel}
					/>
				</div>
			</div>
		</div>
	);
}
