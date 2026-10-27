"use client";

import { useEffect, useRef, useState } from "react";
import Link from "@/components/shared/Link";
import ContentIcon from "@/features/content/shared/ContentIcon";
import ExpandableCard from "@/features/content/shared/ExpandableCard";
import useReducedMotion from "@/features/content/shared/useReducedMotion";
import { faqCategories } from "@/lib/content/data";

type Gsap = typeof import("gsap")["gsap"];

export default function FaqPage() {
	const [audience, setAudience] = useState<"buyer" | "seller">("buyer");
	const [activeFaqCategory, setActiveFaqCategory] = useState("buyer-account");
	const reducedMotion = useReducedMotion();

	// Categories shown depend on the 買家/賣家 switch.
	const visibleCategories = faqCategories.filter(
		(cat) => cat.audience === audience,
	);

	// Switching audience resets the active category to that side's first one.
	const handleAudience = (next: "buyer" | "seller") => {
		if (next === audience) return;
		setAudience(next);
		const first = faqCategories.find((cat) => cat.audience === next);
		if (first) setActiveFaqCategory(first.id);
	};

	// Lazily load GSAP once (mirrors features/seller/landing SellerAnimations)
	// so the content route stays light until the page actually mounts.
	const gsapRef = useRef<Gsap | null>(null);
	const [gsapReady, setGsapReady] = useState(false);
	useEffect(() => {
		let mounted = true;
		import("gsap").then(({ gsap }) => {
			if (mounted) {
				gsapRef.current = gsap;
				setGsapReady(true);
			}
		});
		return () => {
			mounted = false;
		};
	}, []);

	// Cascade the visible category's cards in when the seller switches
	// tabs. We skip the very first run so SSR content never flashes hidden
	// on initial load — the questions stay readable with or without JS.
	const contentRef = useRef<HTMLDivElement>(null);
	const firstRunRef = useRef(true);
	useEffect(() => {
		if (firstRunRef.current) {
			firstRunRef.current = false;
			return;
		}
		const gsap = gsapRef.current;
		const root = contentRef.current;
		if (!gsapReady || !gsap || reducedMotion || !root) return;
		const cards = root.querySelectorAll<HTMLElement>(
			`[data-faq-cat="${activeFaqCategory}"] .faq-card`,
		);
		if (!cards.length) return;
		const ctx = gsap.context(() => {
			gsap.from(cards, {
				opacity: 0,
				y: 22,
				rotateX: -8,
				transformOrigin: "top center",
				duration: 0.5,
				ease: "power3.out",
				stagger: 0.07,
			});
		}, root);
		return () => ctx.revert();
	}, [activeFaqCategory, gsapReady, reducedMotion]);

	return (
		<section className="section relative overflow-hidden">
			<div className="content-container">
				{/* sr-only h1 keeps a single, meaningful zh-TW page heading for SEO;
					    the visible "FAQ" stays an h2 so the layout is untouched. */}
				<h1 className="sr-only">常見問題</h1>
				{/* Header */}
				<div className="flex flex-col items-center gap-3 mb-9 text-center">
					<span className="font-[var(--font-primary)] text-[length:var(--text-xl)] font-bold leading-7 text-[var(--color-primary)] uppercase">
						Return and Exchange Policy
					</span>
					<h2 className="font-[var(--font-chinese)] text-[length:var(--text-4xl)] font-bold leading-10 text-[var(--ink-body)] m-0">
						FAQ
					</h2>
					<div className="w-20 h-1 bg-[var(--color-primary)] rounded-full" />
					<p className="font-[var(--font-chinese)] text-[length:var(--text-lg)] font-normal leading-7 text-[var(--ink-body)] max-w-[786px] m-0">
						Hawkr 的常見問題與解答
					</p>
				</div>

				<div className="grid grid-cols-[308px_1fr] gap-[60px] items-start max-[1024px]:grid-cols-1 max-[1024px]:gap-6">
					{/* Left sidebar — sticky lives on the wrapper because
						    .glass-card sets position:relative and would override
						    a sticky on the <aside> itself (pushing it down by the
						    top offset even unscrolled). */}
					<div className="self-start sticky top-[88px] max-[1024px]:static">
						<aside className="p-6 rounded-[30px] flex flex-col gap-3 glass-card glass-card--solid max-[1024px]:flex-row max-[1024px]:flex-wrap max-[1024px]:rounded-[20px] max-[1024px]:p-4 max-[1024px]:gap-2">
							{/* Buyer / seller switch */}
							<div
								className="flex gap-1 p-1 rounded-[14px] border border-hk-border-soft max-[1024px]:w-full"
								role="tablist"
								aria-label="FAQ 對象"
							>
								{(["buyer", "seller"] as const).map((aud) => (
									<button
										key={`aud-${aud}`}
										type="button"
										role="tab"
										aria-selected={audience === aud}
										onClick={() => handleAudience(aud)}
										className={`flex-1 h-10 rounded-[10px] border-none cursor-pointer font-[var(--font-chinese)] text-[length:var(--text-base)] font-bold transition-colors duration-200 ${audience === aud ? "bg-hk-primary text-hk-inverse" : "bg-transparent text-hk-body hover:bg-hk-surface-glass-hover"}`}
									>
										{aud === "buyer" ? "買家" : "賣家"}
									</button>
								))}
							</div>
							<nav aria-label="FAQ categories">
								<div className="font-[var(--font-chinese)] text-[length:var(--text-lg)] font-bold text-hk-heading px-4 max-[1024px]:w-full">
									問題分類
								</div>
								<div className="w-full h-[1px] bg-[rgba(86,86,86,0.15)] max-[1024px]:w-full"></div>
								<div className="flex flex-col gap-2 max-[1024px]:flex-row max-[1024px]:flex-wrap max-[1024px]:gap-1">
									{visibleCategories.map((cat) => (
										<button
											key={`faqcat-${cat.id}`}
											className={`flex items-center gap-3 w-full h-12 px-4 border-none rounded-[14px] bg-transparent text-hk-body font-[var(--font-chinese)] text-[length:var(--text-lg)] font-normal cursor-pointer transition-all duration-200 text-left hover:bg-hk-surface-glass-hover max-[1024px]:w-auto max-[1024px]:h-auto max-[1024px]:p-[8px_14px] max-[1024px]:text-[0.9375rem] max-[768px]:text-[length:var(--text-sm)] max-[768px]:p-[6px_12px] ${activeFaqCategory === cat.id ? "bg-hk-surface-glass-active text-hk-primary font-bold" : ""}`}
											type="button"
											onClick={() => setActiveFaqCategory(cat.id)}
										>
											<span
												className={`flex items-center justify-center shrink-0 w-6 h-6 [&_svg]:stroke-hk-body ${activeFaqCategory === cat.id ? "[&_svg]:stroke-hk-primary" : ""}`}
											>
												<ContentIcon icon={cat.icon} />
											</span>
											<span className="flex-1">{cat.name}</span>
										</button>
									))}
								</div>
							</nav>
							<div className="bg-hk-surface-card border border-hk-border-soft rounded-[var(--radius-lg)] p-6 shadow-hk-card flex flex-col gap-2 max-[1024px]:w-full max-[1024px]:flex-row max-[1024px]:items-center max-[1024px]:gap-3 max-[480px]:flex-col">
								<p className="font-[var(--font-chinese)] text-[length:var(--text-base)] font-bold text-hk-heading m-0">
									找不到答案？
								</p>
								<p className="font-[var(--font-chinese)] text-[length:var(--text-sm)] font-normal text-hk-body m-0">
									我們的客服團隊隨時準備為你服務。
								</p>
								<Link
									href="/contact"
									className="hk-cta-primary w-full p-[7px_0] bg-hk-primary text-hk-inverse font-[var(--font-chinese)] text-[length:var(--text-sm)] font-bold border-none rounded-[10px] cursor-pointer text-center transition-[filter,background-color] duration-200 hover:brightness-110 max-[1024px]:w-auto max-[1024px]:p-[7px_24px] max-[1024px]:whitespace-nowrap max-[480px]:w-full"
								>
									聯繫客服
								</Link>
							</div>
						</aside>
					</div>

					{/* Right: FAQ content (show active category only) */}
					<div ref={contentRef} className="flex flex-col gap-9">
						{visibleCategories.map((cat) => {
							const isActive = activeFaqCategory === cat.id;
							return (
								<div
									key={`faqsec-${cat.id}`}
									data-faq-cat={cat.id}
									style={{ display: isActive ? undefined : "none" }}
								>
									<div className="flex items-center gap-3 mb-6">
										<div className="w-9 h-9 rounded-[10px] bg-[var(--color-primary)] flex items-center justify-center shrink-0">
											<ContentIcon icon={cat.icon} stroke="white" />
										</div>
										<h3 className="font-[var(--font-chinese)] text-[length:var(--text-2xl)] font-bold leading-8 text-[var(--ink-body)] m-0 max-[768px]:text-[length:var(--text-xl)]">
											{cat.name}
										</h3>
									</div>
									<div className="flex flex-col gap-4 [perspective:1400px]">
										{cat.items.map((item, itemIdx) => (
											<ExpandableCard
												key={`fqi-${cat.id}-${item.q}`}
												id={`faq-${cat.id}-${itemIdx}`}
												title={item.q}
												body={item.a}
												note={item.note}
												steps={item.steps}
												gsapRef={gsapRef}
												reducedMotion={reducedMotion}
											/>
										))}
									</div>
								</div>
							);
						})}
					</div>
				</div>
			</div>
		</section>
	);
}
