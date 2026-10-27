"use client";

import type { CSSProperties } from "react";
import { useCallback, useEffect, useRef, useState } from "react";
import AppIcon from "@/components/shared/AppIcon";
import BrandLogo from "@/components/shared/BrandLogo";
import Link from "@/components/shared/Link";

type SellerWorldScene = {
	id: string;
	label: string;
	title: string;
	body?: string;
	poster?: string;
	objectPosition?: string;
	copyPlacement?: "top" | "center" | "bottom";
	steps?: readonly string[];
	disclaimer?: string;
	kind?: "end";
};

const SCENES: readonly SellerWorldScene[] = [
	{
		id: "birth",
		label: "人潮",
		title: "人潮很多。",
		poster: "/placeholders/journey-birth.svg",
		objectPosition: "center 48%",
		copyPlacement: "top",
	},
	{
		id: "harvest",
		label: "停留",
		title: "但很少人駐足停留。",
		poster: "/placeholders/journey-harvest.svg",
		objectPosition: "center 48%",
		copyPlacement: "top",
	},
	{
		id: "sorting",
		label: "一滑",
		title: "你的品牌，也被略過了嗎？",
		poster: "/placeholders/journey-sorting.svg",
		objectPosition: "center 50%",
		copyPlacement: "top",
	},
	{
		id: "processing",
		label: "產地",
		title: "停留，才會認識產地。",
		poster: "/placeholders/journey-processing.svg",
		objectPosition: "center 52%",
		copyPlacement: "bottom",
	},
	{
		id: "distribution",
		label: "看見",
		title: "看見，才會認識價值。",
		poster: "/placeholders/journey-distribution.svg",
		objectPosition: "center 50%",
		copyPlacement: "top",
	},
	{
		id: "retail",
		label: "理解",
		title: "高曝光，不一定會有好效果。",
		body: "了解產品的故事，商品就不會只是一張照片。",
		poster: "/placeholders/journey-retail.svg",
		objectPosition: "center 52%",
		copyPlacement: "center",
		steps: ["停留", "了解", "信任", "有機會成交"],
		disclaimer: "廣告情境示意",
	},
	{
		id: "table",
		label: "舞台",
		title: "我們想把舞台還給產品本身。",
		body: "讓好內容和好產品，走進日常生活。",
		poster: "/placeholders/journey-table.svg",
		objectPosition: "center 48%",
		copyPlacement: "bottom",
	},
	{
		id: "apply",
		label: "建立賣場",
		title: "讓產品有更好的舞台",
		body: "Hawkr・食品品牌自助開店",
		kind: "end",
	},
] as const;

const clamp = (value: number, min = 0, max = 1) =>
	Math.min(max, Math.max(min, value));

const smoothstep = (edge0: number, edge1: number, value: number) => {
	const x = clamp((value - edge0) / Math.max(edge1 - edge0, 0.0001));
	return x * x * (3 - 2 * x);
};

/**
 * 舞台是釘在 navbar 下方的（`.srw-sticky` 的 `top`），而且比視窗矮一個
 * navbar 淨空，所以進度不能再用「世界高度 − 視窗高度」硬算。這裡直接量：
 * `start` 是舞台開始被釘住的捲動位置，`span` 是它被釘住期間能走的距離。
 * 兩者都從實際幾何推出來，之後調 CSS 偏移也不會讓影片 scrub 失準。
 */
const readPinMetrics = (world: HTMLElement, sticky: HTMLElement) => {
	const rect = world.getBoundingClientRect();
	const pinTop = Number.parseFloat(window.getComputedStyle(sticky).top) || 0;
	const worldTop = window.scrollY + rect.top;
	const start = Math.max(worldTop - pinTop, 0);
	const end = worldTop + world.offsetHeight - sticky.offsetHeight - pinTop;

	return { rect, start, span: Math.max(end - start, 1) };
};

interface SellerRecruitmentScrollWorldProps {
	className?: string;
}

function SellerScrollHint() {
	return (
		<div className="srw-scroll-hint" aria-hidden="true">
			<div className="srw-scroll-hint-glyph">
				<AppIcon
					name="arrow-down"
					size={96}
					className="animate-pulse motion-reduce:animate-none"
				/>
			</div>
			<span className="srw-scroll-hint-label">往下滑，繼續看</span>
		</div>
	);
}

export default function SellerRecruitmentScrollWorld({
	className = "",
}: SellerRecruitmentScrollWorldProps) {
	const worldRef = useRef<HTMLElement>(null);
	const stickyRef = useRef<HTMLDivElement>(null);
	const sceneRefs = useRef<Array<HTMLElement | null>>([]);
	const copyRefs = useRef<Array<HTMLDivElement | null>>([]);
	const activeRef = useRef(0);
	const hintCheckedRef = useRef(false);
	const [activeIndex, setActiveIndex] = useState(0);
	const [showScrollHint, setShowScrollHint] = useState(false);

	const goToScene = useCallback((index: number) => {
		const world = worldRef.current;
		const sticky = stickyRef.current;
		if (!world || !sticky) return;

		const { start, span } = readPinMetrics(world, sticky);
		const target = start + ((index + 0.35) / SCENES.length) * span;
		const reduce = window.matchMedia(
			"(prefers-reduced-motion: reduce)",
		).matches;

		window.scrollTo({
			top: target,
			behavior: reduce ? "auto" : "smooth",
		});
	}, []);

	useEffect(() => {
		if (hintCheckedRef.current) return;
		hintCheckedRef.current = true;

		if (!window.matchMedia("(max-width: 640px)").matches) return;

		try {
			if (sessionStorage.getItem("seller-scroll-world-hint-seen")) return;
			sessionStorage.setItem("seller-scroll-world-hint-seen", "1");
		} catch {
			// If session storage is unavailable, show once for this page mount.
		}

		setShowScrollHint(true);
	}, []);

	useEffect(() => {
		if (!showScrollHint) return;

		const dismiss = () => setShowScrollHint(false);
		const timer = window.setTimeout(dismiss, 6000);

		window.addEventListener("pointerdown", dismiss, { passive: true });
		window.addEventListener("wheel", dismiss, { passive: true });
		document.addEventListener("keydown", dismiss);

		return () => {
			window.clearTimeout(timer);
			window.removeEventListener("pointerdown", dismiss);
			window.removeEventListener("wheel", dismiss);
			document.removeEventListener("keydown", dismiss);
		};
	}, [showScrollHint]);

	useEffect(() => {
		const world = worldRef.current;
		const sticky = stickyRef.current;
		if (!world || !sticky) return;

		let scrollTicking = false;

		const updateFromScroll = () => {
			const { start, span } = readPinMetrics(world, sticky);
			const progress = clamp((window.scrollY - start) / span);
			const position = progress * SCENES.length;
			const nextActive = Math.min(Math.floor(position), SCENES.length - 1);
			const fade = 0.09;

			world.style.setProperty("--srw-progress", progress.toFixed(5));
			world.dataset.activeScene = SCENES[nextActive].id;

			SCENES.forEach((sceneData, index) => {
				const local = clamp(position - index);
				const enter =
					index === 0 ? 1 : smoothstep(index - fade, index + fade, position);
				const exit =
					index === SCENES.length - 1
						? 1
						: 1 - smoothstep(index + 1 - fade, index + 1 + fade, position);
				const opacity = clamp(enter * exit);
				const scene = sceneRefs.current[index];

				if (scene) {
					scene.style.opacity = opacity.toFixed(4);
					scene.style.visibility = opacity > 0.002 ? "visible" : "hidden";
					scene.style.zIndex = String(index === nextActive ? 20 : 10 + index);
				}

				let copyOpacity = 0;
				if (index === 0) {
					copyOpacity = 1 - smoothstep(0.44, 0.84, local);
				} else if (index === SCENES.length - 1) {
					copyOpacity = smoothstep(0.14, 0.46, local);
				} else {
					copyOpacity =
						smoothstep(0.09, 0.32, local) * (1 - smoothstep(0.65, 0.92, local));
				}

				const copy = copyRefs.current[index];
				if (copy) {
					copy.style.opacity = copyOpacity.toFixed(4);
					// 這段位移是給浮在畫面上的文案卡用的。收尾那一幕是滿版卡面，
					// 位移只會讓舞台底色從它上下緣露出一條約 8px 的異色（深色模式
					// 更明顯），所以滿版的那一幕只淡入、不位移。
					if (sceneData.kind !== "end") {
						copy.style.transform = `translate3d(0, ${(0.5 - local) * 20}px, 0)`;
					}
					copy.style.pointerEvents = copyOpacity > 0.55 ? "auto" : "none";
				}
			});

			if (nextActive !== activeRef.current) {
				activeRef.current = nextActive;
				setActiveIndex(nextActive);
			}
			scrollTicking = false;
		};

		const onScroll = () => {
			if (scrollTicking) return;
			scrollTicking = true;
			requestAnimationFrame(updateFromScroll);
		};
		const onResize = () => updateFromScroll();
		updateFromScroll();
		window.addEventListener("scroll", onScroll, { passive: true });
		window.addEventListener("resize", onResize);
		window.addEventListener("orientationchange", onResize);

		return () => {
			window.removeEventListener("scroll", onScroll);
			window.removeEventListener("resize", onResize);
			window.removeEventListener("orientationchange", onResize);
		};
	}, []);

	return (
		<section
			className={`seller-recruitment-world ${className}`.trim()}
			ref={worldRef}
			aria-label="從人潮匆匆經過，到顧客停下來認識品牌的八幕旅程"
		>
			<div className="srw-sticky" ref={stickyRef}>
				<div className="srw-stage">
					{SCENES.map((scene, index) => {
						const isActive = activeIndex === index;
						const titleId = `srw-title-${scene.id}`;

						return (
							<article
								className={`srw-scene${isActive ? " is-active" : ""}`}
								key={scene.id}
								ref={(node) => {
									sceneRefs.current[index] = node;
								}}
								style={
									{
										"--srw-object-position":
											scene.objectPosition ?? "center 48%",
										opacity: index === 0 ? 1 : 0,
										visibility: index === 0 ? "visible" : "hidden",
									} as CSSProperties
								}
								aria-labelledby={titleId}
								aria-hidden={!isActive}
							>
								{scene.kind === "end" ? (
									<div
										className="srw-endcard"
										ref={(node) => {
											copyRefs.current[index] = node;
										}}
										style={{ opacity: 0 }}
									>
										<span className="srw-endcard-orb srw-endcard-orb--top" />
										<span className="srw-endcard-orb srw-endcard-orb--bottom" />
										<BrandLogo
											className="srw-endcard-logo"
											size="xl"
											variant="horizontal"
										/>
										<h2 id={titleId}>{scene.title}</h2>
										<p>{scene.body}</p>
										<Link
											className="srw-cta"
											href="/seller/onboarding"
											tabIndex={isActive ? undefined : -1}
										>
											開始建立賣場
										</Link>
									</div>
								) : (
									<>
										<div className="srw-media" aria-hidden="true">
											{/* biome-ignore lint/performance/noImgElement: the backdrop reuses the poster frame without optimizer drift. */}
											<img
												className="srw-media-backdrop"
												src={scene.poster}
												alt=""
												width={640}
												height={1136}
												fetchPriority={index === 0 ? "high" : "low"}
											/>
											<div className="srw-media-portrait">
												{/* biome-ignore lint/performance/noImgElement: direct local poster avoids optimizer drift. */}
												<img
													className="srw-poster"
													src={scene.poster}
													alt=""
													width={640}
													height={1136}
													fetchPriority={index === 0 ? "high" : "low"}
												/>
											</div>
										</div>

										<div
											className={`srw-copy${scene.steps ? " srw-copy--has-steps" : ""}`}
											data-placement={scene.copyPlacement ?? "center"}
											ref={(node) => {
												copyRefs.current[index] = node;
											}}
											style={{ opacity: index === 0 ? 1 : 0 }}
										>
											<p className="srw-label">{scene.label}</p>
											{scene.disclaimer && (
												<p className="srw-disclaimer">{scene.disclaimer}</p>
											)}
											<h2 className="srw-title" id={titleId}>
												{scene.title}
											</h2>
											{scene.body && <p className="srw-body">{scene.body}</p>}
											{scene.steps && (
												<ol className="srw-steps">
													{scene.steps.map((step) => (
														<li key={step}>{step}</li>
													))}
												</ol>
											)}
										</div>
									</>
								)}
							</article>
						);
					})}
				</div>

				<aside className="srw-scene-rail" aria-label="品牌旅程章節">
					{SCENES.map((scene, index) => (
						<button
							className={index === activeIndex ? "is-active" : ""}
							key={scene.id}
							type="button"
							onClick={() => goToScene(index)}
							aria-label={`前往「${scene.label}」章節`}
							aria-current={index === activeIndex ? "step" : undefined}
						>
							<span>{scene.label}</span>
							<i aria-hidden="true" />
						</button>
					))}
				</aside>

				{showScrollHint && <SellerScrollHint />}
			</div>
		</section>
	);
}
