"use client";

import { useEffect, useRef } from "react";
import AppIcon, { type AppIconName } from "@/components/shared/AppIcon";
import Link from "@/components/shared/Link";
import HomeEyebrow from "@/features/home/components/HomeEyebrow";

interface ProcessCardSpec {
	i: number;
	title: string;
	en: string;
	desc: string;
	num: string;
	icon: AppIconName;
	iconLabel: string;
	stamp: string;
}

const PROCESS_CARDS: ProcessCardSpec[] = [
	{
		i: 1,
		title: "挑選新鮮",
		en: "SELECT FRESH",
		desc: "瀏覽當季蔬果、海鮮與肉品，依溫層分類，輕鬆放入購物車。",
		num: "01.",
		icon: "leaf",
		iconLabel: "挑選新鮮食材",
		stamp: "常溫 · 冷藏",
	},
	{
		i: 2,
		title: "完成下單",
		en: "PLACE ORDER",
		desc: "選擇配送日期與時段，確認地址，安心結帳，全程加密保護。",
		num: "02.",
		icon: "buy",
		iconLabel: "完成下單",
		stamp: "統一金流",
	},
	{
		i: 3,
		title: "產地直採",
		en: "FARM PICKED",
		desc: "訂單抵達農場，小農當日清晨採收，依你指定的溫層分裝出貨。",
		num: "03.",
		icon: "discovery",
		iconLabel: "看清楚產地來源",
		stamp: "當日採收",
	},
	{
		i: 4,
		title: "鮮配到府",
		en: "HOME DELIVERED",
		desc: "全程冷鏈配送，最快隔日抵達。從產地直達你家，鎖住新鮮。",
		num: "04.",
		icon: "truck",
		iconLabel: "鮮配到府",
		stamp: "冷鏈直送",
	},
];

const ILLUS_BG: Record<number, string> = {
	1: "bg-hk-home-cream",
	2: "bg-hk-home-cream",
	3: "bg-hk-home-accent-100",
	4: "bg-hk-home-tint-peach",
};

const SEG_START = 0.06;
const SEG_END = 0.92;
const OVERLAP = 0.35;
const N = 4;
const SEG_LEN = (SEG_END - SEG_START) / N;
const OFFSCREEN_Y = 1000;
const HOLD_MIN = 380;
const HOLD_MAX = 720;
const HOLD_RATIO = 0.72;
const MOBILE_FLOW_WIDTH = 640;

const clamp01 = (t: number) => Math.max(0, Math.min(1, t));
const easeOut = (t: number) => {
	const x = clamp01(t);
	return 1 - (1 - x) ** 3;
};
const lerp = (a: number, b: number, t: number) => a + (b - a) * t;

interface FinalPos {
	x: number;
	y: number;
	r: number;
	s: number;
}

/**
 * Process story · scroll-driven fan-out.
 *
 * Mirrors the design's `home.js` IIFE: cards translate up from off-screen
 * and arrange themselves in a 4-card horizontal fan as the section
 * scrolls past. The stage-head shrinks + lifts so cards don't collide
 * with the heading. Layout points are mirrored 1:1 from the design.
 */
export default function HomeProcessCards() {
	const sectionRef = useRef<HTMLElement | null>(null);
	const cardsRef = useRef<Array<HTMLElement | null>>([]);
	const stageHeadRef = useRef<HTMLDivElement | null>(null);
	const barRef = useRef<HTMLDivElement | null>(null);
	const stepNRef = useRef<HTMLSpanElement | null>(null);
	const finalsRef = useRef<FinalPos[]>([]);

	useEffect(() => {
		const section = sectionRef.current;
		if (!section) return;

		const applyLayout = () => {
			const w = window.innerWidth;
			const h = window.innerHeight;
			if (w <= 760) {
				finalsRef.current = [
					{ x: 0, y: -210, r: -3, s: 0.92 },
					{ x: 24, y: -60, r: 2, s: 0.92 },
					{ x: -24, y: 90, r: -2, s: 0.92 },
					{ x: 0, y: 230, r: 3, s: 0.92 },
				];
			} else {
				const SPREAD_FAR = Math.min(520, w * 0.36);
				const SPREAD_MID = Math.min(175, w * 0.12);
				const SAFE_Y = Math.min(84, Math.max(42, h * 0.07));
				finalsRef.current = [
					{ x: -SPREAD_FAR, y: SAFE_Y, r: -6, s: 1.0 },
					{ x: -SPREAD_MID, y: SAFE_Y, r: -2, s: 1.0 },
					{ x: SPREAD_MID, y: SAFE_Y, r: 2, s: 1.0 },
					{ x: SPREAD_FAR, y: SAFE_Y, r: 6, s: 1.0 },
				];
			}
			update();
		};

		const progressFor = (i: number, raw: number) => {
			const a = SEG_START + i * SEG_LEN - (i > 0 ? OVERLAP * SEG_LEN : 0);
			const b = SEG_START + (i + 1) * SEG_LEN;
			return easeOut((raw - a) / (b - a));
		};

		const resetMobileFlow = () => {
			cardsRef.current.forEach((card) => {
				if (!card) return;
				card.style.zIndex = "";
				card.style.position = "";
				card.style.left = "";
				card.style.top = "";
				card.style.transform = "";
			});

			if (barRef.current) barRef.current.style.width = "100%";
			if (stepNRef.current) stepNRef.current.textContent = "04";
			if (stageHeadRef.current) {
				stageHeadRef.current.style.transform = "";
				stageHeadRef.current.style.opacity = "";
			}
		};

		const update = () => {
			if (window.innerWidth <= MOBILE_FLOW_WIDTH) {
				resetMobileFlow();
				return;
			}

			const rect = section.getBoundingClientRect();
			const holdScroll = Math.min(
				HOLD_MAX,
				Math.max(HOLD_MIN, window.innerHeight * HOLD_RATIO),
			);
			const playableScroll = Math.max(
				1,
				rect.height - window.innerHeight - holdScroll,
			);
			const raw = clamp01(-rect.top / playableScroll);
			let landed = 0;

			cardsRef.current.forEach((card, i) => {
				if (!card) return;
				const f = finalsRef.current[i];
				if (!f) return;
				const start: FinalPos = {
					x: f.x * 0.4,
					y: OFFSCREEN_Y,
					r: f.r * 0.5,
					s: 0.92,
				};
				const t = progressFor(i, raw);
				if (t >= 0.5) landed = Math.max(landed, i + 1);
				const x = lerp(start.x, f.x, t);
				const y = lerp(start.y, f.y, t);
				const r = lerp(start.r, f.r, t);
				const sc = lerp(start.s, f.s, t);
				card.style.zIndex = String(10 + i);
				card.style.position = "absolute";
				card.style.left = "50%";
				card.style.top = "50%";
				card.style.transform = `translate(-50%, -50%) translate(${x}px, ${y}px) rotate(${r}deg) scale(${sc})`;
			});

			if (barRef.current) {
				const w = clamp01((raw - SEG_START) / (SEG_END - SEG_START)) * 100;
				barRef.current.style.width = `${w.toFixed(1)}%`;
			}
			if (stepNRef.current) {
				stepNRef.current.textContent = String(Math.max(1, landed)).padStart(
					2,
					"0",
				);
			}
			if (stageHeadRef.current) {
				const headT = easeOut(clamp01(raw / 0.55));
				const scale = lerp(1, 0.68, headT);
				const lift = lerp(0, -24, headT);
				const alpha = lerp(1, 0.85, headT);
				// X-centering is owned by the flex wrapper around the head, so
				// the transform only carries the scroll animation (vertical
				// lift + scale).  Adding translate(-50%) here would shift the
				// head by half its width on top of the already-centred parent.
				stageHeadRef.current.style.transform = `translateY(${lift}px) scale(${scale})`;
				stageHeadRef.current.style.opacity = alpha.toFixed(2);
			}
		};

		let ticking = false;
		const onScroll = () => {
			if (!ticking) {
				ticking = true;
				requestAnimationFrame(() => {
					update();
					ticking = false;
				});
			}
		};

		const pageWrapper = document.querySelector(".page-wrapper");
		window.addEventListener("scroll", onScroll, { passive: true });
		pageWrapper?.addEventListener("scroll", onScroll, { passive: true });
		window.addEventListener("resize", applyLayout);
		applyLayout();

		return () => {
			window.removeEventListener("scroll", onScroll);
			pageWrapper?.removeEventListener("scroll", onScroll);
			window.removeEventListener("resize", applyLayout);
		};
	}, []);

	return (
		<section
			ref={sectionRef}
			className="relative bg-transparent h-[clamp(2450px,330vh,3200px)] mt-0 max-[640px]:h-auto max-[640px]:mt-[18px] max-[640px]:mx-3 max-[640px]:px-4 max-[640px]:pt-9 max-[640px]:pb-[118px]"
			id="story"
		>
			{/* Let the browser own the pinning. Switching this element between
			    absolute and fixed from the scroll handler produced two viewport-
			    sized layout shifts as the section entered and left the stage.
			    Sticky keeps the same visual hold while the containing section
			    supplies the scroll runway, without changing layout mode. */}
			<div className="sticky top-0 w-full h-screen overflow-hidden max-[640px]:relative max-[640px]:top-auto max-[640px]:h-auto max-[640px]:overflow-visible">
				{/* Flex centerer pinned to the stage's full width — keeps the
				 * head reliably horizontally centred regardless of what
				 * inline transform the scroll animation applies to it. */}
				<div className="absolute top-[6vh] left-0 right-0 flex justify-center z-[4] pointer-events-none max-[640px]:relative max-[640px]:top-auto">
					<div
						ref={stageHeadRef}
						className="stage-head text-center bg-hk-home-paper/[78%] backdrop-blur-[22px] backdrop-saturate-[115%] border border-white/60 rounded-[28px] p-[22px_36px_26px] shadow-[0_20px_40px_-20px_rgba(76,50,20,0.18),0_0_0_1px_rgba(255,255,255,0.4)_inset] [transform-origin:top_center] max-[640px]:w-full max-[640px]:[transform:none] max-[640px]:mb-[18px] max-[640px]:p-[18px_16px_20px] max-[640px]:rounded-[24px] max-[640px]:bg-hk-home-paper max-[640px]:opacity-100"
					>
						<HomeEyebrow>品牌故事 · OUR STORY</HomeEyebrow>
						{/* Section heading for an embeddable marketing block — must be an
						    h2, not h1, so host pages (e.g. /about) keep a single page h1. */}
						<h2 className="mt-[14px] font-[var(--font-body)] font-black text-[clamp(2.25rem,5.4vw,4.75rem)] leading-[1.05] tracking-[-0.01em] text-hk-home-ink max-[640px]:text-[clamp(2rem,9.6vw,2.625rem)] max-[640px]:leading-[1.04]">
							從產地出發，
							<br />
							<em className="not-italic text-hk-home-accent-700">
								走進你的日常餐桌
							</em>
						</h2>
						<div className="mt-[14px] font-[var(--font-body)] text-[0.8125rem] font-medium text-hk-home-ink/[55%] tracking-[0.04em] max-[640px]:text-[0.75rem] max-[640px]:leading-[1.5] max-[640px]:tracking-[0.02em]">
							↓ 一份食材從採收、整理、包裝、配送到開箱 —— 每一步都不只是物流
						</div>
					</div>
				</div>

				<div className="absolute inset-0 flex items-center justify-center z-[2] max-[640px]:relative max-[640px]:inset-auto max-[640px]:grid max-[640px]:grid-cols-1 max-[640px]:gap-4">
					{PROCESS_CARDS.map((card, idx) => (
						<article
							key={card.i}
							ref={(el) => {
								cardsRef.current[idx] = el;
							}}
							className="process-card absolute w-[clamp(260px,22vw,320px)] h-[clamp(400px,58vh,540px)] bg-hk-home-paper border border-[rgba(76,50,20,0.08)] rounded-[22px] shadow-[0_30px_50px_-20px_rgba(76,50,20,0.25),0_8px_16px_-8px_rgba(76,50,20,0.12)] p-[36px_32px_28px] flex flex-col [transform-origin:50%_60%] will-change-transform overflow-hidden z-[5] max-[640px]:relative max-[640px]:left-auto max-[640px]:top-auto max-[640px]:w-full max-[640px]:h-auto max-[640px]:min-h-0 max-[640px]:p-[26px_22px_20px] max-[640px]:[transform:none]"
							data-i={card.i}
						>
							<h2 className="headline font-[var(--font-body)] font-black text-[2.75rem] leading-[0.95] tracking-[-0.02em] text-hk-home-ink m-0 break-words max-[640px]:text-[clamp(1.875rem,8.8vw,2.375rem)] max-[640px]:leading-[1]">
								{card.title}
								<span className="block font-['Roboto',var(--font-body)] text-[0.875rem] font-black tracking-[0.18em] text-hk-home-ink/[45%] mt-[10px] leading-[1.2] max-[640px]:text-[0.75rem] max-[640px]:tracking-[0.16em]">
									{card.en}
								</span>
							</h2>
							<p className="font-[var(--font-body)] text-[0.8125rem] leading-[1.55] text-hk-home-ink/70 mt-[14px] max-w-[22ch] max-[640px]:max-w-none max-[640px]:text-[0.875rem] max-[640px]:leading-[1.65]">
								{card.desc}
							</p>
							<div className="num font-['Roboto',var(--font-body)] font-black text-[4.875rem] leading-[1] text-hk-home-ink mt-auto tracking-[-0.04em] max-[640px]:mt-[20px] max-[640px]:text-[3.25rem]">
								{card.num}
							</div>
							<div
								className={`illus mt-[18px] rounded-[14px] h-[130px] flex items-center justify-center text-[4.25rem] leading-[1] relative overflow-hidden ${ILLUS_BG[card.i] ?? ""} max-[640px]:h-[104px] max-[640px]:mt-[12px]`}
							>
								<AppIcon
									className="process-glyph pointer-events-none relative z-[1] block text-[var(--icon-accent)] drop-shadow-[0_10px_16px_rgb(var(--color-ink-700-rgb)/0.16)] max-[640px]:[transform:scale(0.86)]"
									name={card.icon}
									size={92}
									title={card.iconLabel}
								/>
								<span className="stamp absolute font-['Roboto',var(--font-body)] font-black text-[0.6875rem] tracking-[0.18em] text-hk-home-ink/[40%] bottom-[12px] right-[14px]">
									{card.stamp}
								</span>
							</div>
						</article>
					))}
				</div>

				<div className="progress-rail absolute left-[7vw] bottom-[9vh] z-[5] flex gap-[10px] items-center font-['Roboto',var(--font-body)] font-bold text-[0.6875rem] tracking-[0.22em] text-hk-home-ink/70 bg-hk-home-paper/[82%] backdrop-blur-[16px] backdrop-saturate-[115%] px-[18px] py-[10px] rounded-[100px] border border-white/60 shadow-[0_12px_22px_-14px_rgba(76,50,20,0.22)] max-[640px]:hidden">
					<span>STEP</span>
					<span ref={stepNRef}>01</span>
					<div className="w-[220px] h-[4px] bg-hk-home-ink/[12%] dark:bg-[rgba(245,233,195,0.16)] rounded-[100px] overflow-hidden">
						<div
							ref={barRef}
							className="h-full bg-hk-home-accent-500 rounded-[100px] w-0 transition-[width] duration-100 ease-linear"
						/>
					</div>
					<span>04</span>
				</div>

				<Link
					className="stage-cta absolute right-[7vw] bottom-[8vh] z-[6] flex items-center gap-[12px] no-underline bg-hk-brand-secondary text-[color:var(--color-brand-ink)] font-['Roboto',var(--font-body)] font-black text-[0.9375rem] tracking-[0.18em] px-[28px] py-[18px] rounded-[100px] shadow-[0_14px_30px_-10px_rgb(var(--color-brand-secondary-rgb)/0.56)] transition-[transform_0.3s_cubic-bezier(0.2,0.7,0.2,1),background_0.2s] hover:-translate-y-[2px] hover:bg-hk-home-gold-deep max-[640px]:relative max-[640px]:right-auto max-[640px]:bottom-auto max-[640px]:w-fit max-[640px]:mx-auto max-[640px]:mt-[24px] max-[640px]:[transform:none] max-[640px]:px-[22px] max-[640px]:py-[14px] max-[640px]:text-[0.8125rem] max-[640px]:hover:-translate-y-[2px]"
					href="/market/products"
				>
					開始選購{" "}
					<span className="pip w-[8px] h-[8px] rounded-[100px] bg-white" />
				</Link>
			</div>
		</section>
	);
}
