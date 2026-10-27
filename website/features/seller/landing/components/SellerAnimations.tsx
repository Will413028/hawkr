"use client";

import type Lenis from "lenis";
import type LenisSnap from "lenis/snap";
import { useEffect } from "react";

/**
 * One page-scoped motion controller for the public seller landing page.
 *
 * The server-rendered page remains complete without JavaScript. GSAP, Lenis,
 * and pointer physics are loaded only as progressive enhancement, while touch
 * and reduced-motion users keep the original native experience.
 */
export default function SellerAnimations() {
	useEffect(() => {
		const root = document.querySelector<HTMLElement>(".seller-onboarding");
		if (!root) return;

		const reduceMotion = window.matchMedia("(prefers-reduced-motion: reduce)");
		if (reduceMotion.matches) {
			root.dataset.sellerMotion = "reduced";
			return;
		}

		let cancelled = false;
		let disposeEnhancements: (() => void) | undefined;
		const handleReducedMotionChange = (event: MediaQueryListEvent) => {
			if (!event.matches) return;
			root.dataset.sellerMotion = "reduced";
			disposeEnhancements?.();
		};
		reduceMotion.addEventListener("change", handleReducedMotionChange);

		void (async () => {
			const [{ default: gsap }, { ScrollTrigger }] = await Promise.all([
				import("gsap"),
				import("gsap/ScrollTrigger"),
			]);
			if (cancelled || reduceMotion.matches) return;

			gsap.registerPlugin(ScrollTrigger);

			const disposers: Array<() => void> = [];
			let disposed = false;
			const precisePointer = window.matchMedia(
				"(min-width: 900px) and (hover: hover) and (pointer: fine)",
			);
			let lenis: Lenis | undefined;
			let lenisSnap: LenisSnap | undefined;
			let lenisTick: ((time: number) => void) | undefined;
			const syncScrollTrigger = () => ScrollTrigger.update();

			if (precisePointer.matches) {
				const [{ default: Lenis }, { default: Snap }] = await Promise.all([
					import("lenis"),
					import("lenis/snap"),
				]);
				if (cancelled || reduceMotion.matches) return;

				lenis = new Lenis({
					autoRaf: false,
					anchors: false,
					duration: 1,
					smoothWheel: true,
					syncTouch: false,
					wheelMultiplier: 0.88,
				});
				lenisTick = (time) => lenis?.raf(time * 1000);
				lenis.on("scroll", syncScrollTrigger);
				gsap.ticker.add(lenisTick);

				const snapSections = Array.from(
					root.querySelectorAll<HTMLElement>("[data-seller-snap]"),
				);
				lenisSnap = new Snap(lenis, {
					type: "proximity",
					distanceThreshold: "20%",
					debounce: 220,
					duration: 0.72,
					easing: (time) => 1 - (1 - time) ** 4,
				});
				lenisSnap.addElements(snapSections, {
					align: "end",
					ignoreTransform: true,
				});
			} else {
				document.documentElement.classList.add("seller-snap-native");
				disposers.push(() =>
					document.documentElement.classList.remove("seller-snap-native"),
				);
			}

			root.dataset.sellerMotion = "enhanced";

			const context = gsap.context(() => {
				const hero = root.querySelector<HTMLElement>(".seller-hero");
				if (hero) {
					const heroTimeline = gsap.timeline({
						defaults: { ease: "power3.out" },
					});
					heroTimeline
						.from(hero.querySelector(".eyebrow"), {
							autoAlpha: 0,
							y: 14,
							duration: 0.48,
						})
						.from(
							hero.querySelector("h1"),
							{
								autoAlpha: 0,
								y: 30,
								duration: 0.78,
							},
							0.08,
						)
						.from(
							hero.querySelector(".lede"),
							{
								autoAlpha: 0,
								y: 22,
								duration: 0.68,
							},
							0.28,
						)
						.from(
							hero.querySelectorAll(".hero-ctas .btn"),
							{
								autoAlpha: 0,
								y: 18,
								duration: 0.58,
								stagger: 0.08,
							},
							0.42,
						)
						.from(
							hero.querySelector(".spin-badge"),
							{
								autoAlpha: 0,
								scale: 0.72,
								rotation: -18,
								duration: 0.82,
								ease: "back.out(1.45)",
							},
							0.24,
						);
				}

				const fitSection = root.querySelector<HTMLElement>(".seller-fit");
				if (fitSection) {
					const fitHead = fitSection.querySelector(".seller-fit__head > div");
					const fitCards =
						fitSection.querySelectorAll<HTMLElement>(".seller-fit-card");

					gsap.from(fitHead, {
						autoAlpha: 0,
						y: 22,
						duration: 0.68,
						ease: "power3.out",
						scrollTrigger: {
							trigger: fitSection,
							start: "top 84%",
							once: true,
						},
					});
					gsap.from(fitCards, {
						autoAlpha: 0,
						duration: 0.62,
						stagger: 0.08,
						ease: "power2.out",
						scrollTrigger: {
							trigger: fitSection.querySelector(".seller-fit__grid"),
							start: "top 82%",
							once: true,
						},
					});
				}

				const process = root.querySelector<HTMLElement>(".seller-process");
				if (process) {
					const processSteps =
						process.querySelectorAll<HTMLElement>(".process-step");
					gsap.from(process.querySelector(".sect-head > div"), {
						autoAlpha: 0,
						y: 22,
						duration: 0.68,
						ease: "power3.out",
						scrollTrigger: {
							trigger: process,
							start: "top 82%",
							once: true,
						},
					});
					gsap.fromTo(
						processSteps,
						{ autoAlpha: 0, y: 22, "--draw": 0 },
						{
							autoAlpha: 1,
							y: 0,
							"--draw": 1,
							duration: 0.72,
							stagger: 0.12,
							ease: "power3.out",
							scrollTrigger: {
								trigger: process.querySelector(".process-steps"),
								start: "top 82%",
								once: true,
							},
						},
					);
				}

				const showcase = root.querySelector<HTMLElement>(".seller-showcase");
				if (showcase) {
					gsap.from(showcase.querySelector(".carousel-viewport"), {
						autoAlpha: 0,
						y: 28,
						scale: 0.985,
						duration: 0.82,
						ease: "power3.out",
						scrollTrigger: {
							trigger: showcase,
							start: "top 84%",
							once: true,
						},
					});
				}

				const faq = root.querySelector<HTMLElement>(".seller-faq");
				if (faq) {
					gsap.from(faq.querySelector(".sect-head > div"), {
						autoAlpha: 0,
						y: 22,
						duration: 0.68,
						ease: "power3.out",
						scrollTrigger: {
							trigger: faq,
							start: "top 84%",
							once: true,
						},
					});
					gsap.from(faq.querySelectorAll(".faq-item"), {
						autoAlpha: 0,
						y: 14,
						duration: 0.48,
						stagger: 0.055,
						ease: "power2.out",
						scrollTrigger: {
							trigger: faq.querySelector(".faq-list"),
							start: "top 84%",
							once: true,
						},
					});

					for (const item of faq.querySelectorAll<HTMLDetailsElement>(
						".faq-item",
					)) {
						const handleToggle = () => {
							if (!item.open) return;
							gsap.fromTo(
								item.querySelector(".faq-a"),
								{ autoAlpha: 0, y: -8 },
								{
									autoAlpha: 1,
									y: 0,
									duration: 0.3,
									ease: "power2.out",
									overwrite: true,
								},
							);
						};
						item.addEventListener("toggle", handleToggle);
						disposers.push(() =>
							item.removeEventListener("toggle", handleToggle),
						);
					}
				}

				const cta = root.querySelector<HTMLElement>("#seller");
				if (cta) {
					const chars = cta.querySelectorAll(".seller-h .char");
					const paragraph = cta.querySelector(".seller-p");
					const row = cta.querySelector(".seller-row");
					const ctaTimeline = gsap.timeline({
						scrollTrigger: {
							trigger: cta,
							start: "top 75%",
							once: true,
						},
						defaults: { ease: "power3.out" },
					});
					ctaTimeline
						.from(chars, {
							autoAlpha: 0,
							y: 28,
							duration: 0.68,
							stagger: { each: 0.035, from: "start" },
						})
						.from(paragraph, { autoAlpha: 0, y: 22, duration: 0.62 }, 0.32)
						.from(row, { autoAlpha: 0, y: 20, duration: 0.56 }, 0.48);
				}
			}, root);

			if (precisePointer.matches) {
				const hero = root.querySelector<HTMLElement>(".seller-hero");
				const badge = hero?.querySelector<HTMLElement>(".spin-badge");
				if (hero && badge) {
					const badgeX = gsap.quickTo(badge, "x", {
						duration: 0.7,
						ease: "power3.out",
					});
					const badgeY = gsap.quickTo(badge, "y", {
						duration: 0.7,
						ease: "power3.out",
					});
					const badgeRotate = gsap.quickTo(badge, "rotation", {
						duration: 0.8,
						ease: "power3.out",
					});
					const handleHeroPointer = (event: PointerEvent) => {
						const bounds = hero.getBoundingClientRect();
						const x = (event.clientX - bounds.left) / bounds.width;
						const y = (event.clientY - bounds.top) / bounds.height;
						hero.style.setProperty("--seller-pointer-x", `${x * 100}%`);
						hero.style.setProperty("--seller-pointer-y", `${y * 100}%`);
						badgeX((x - 0.5) * 18);
						badgeY((y - 0.5) * 14);
						badgeRotate(-6 + (x - 0.5) * 6);
					};
					const resetHeroPointer = () => {
						badgeX(0);
						badgeY(0);
						badgeRotate(-6);
					};
					hero.addEventListener("pointermove", handleHeroPointer);
					hero.addEventListener("pointerleave", resetHeroPointer);
					disposers.push(() => {
						hero.removeEventListener("pointermove", handleHeroPointer);
						hero.removeEventListener("pointerleave", resetHeroPointer);
						gsap.killTweensOf(badge);
						gsap.set(badge, { clearProps: "x,y,rotation" });
					});
				}

				for (const button of root.querySelectorAll<HTMLElement>(
					".hero-ctas .btn, .seller-row .btn",
				)) {
					const buttonX = gsap.quickTo(button, "x", {
						duration: 0.45,
						ease: "elastic.out(1, 0.45)",
					});
					const buttonY = gsap.quickTo(button, "y", {
						duration: 0.45,
						ease: "elastic.out(1, 0.45)",
					});
					const handleMove = (event: PointerEvent) => {
						const bounds = button.getBoundingClientRect();
						buttonX((event.clientX - bounds.left - bounds.width / 2) * 0.18);
						buttonY((event.clientY - bounds.top - bounds.height / 2) * 0.22);
					};
					const handleLeave = () => {
						buttonX(0);
						buttonY(0);
					};
					button.addEventListener("pointermove", handleMove);
					button.addEventListener("pointerleave", handleLeave);
					button.addEventListener("blur", handleLeave);
					disposers.push(() => {
						button.removeEventListener("pointermove", handleMove);
						button.removeEventListener("pointerleave", handleLeave);
						button.removeEventListener("blur", handleLeave);
						gsap.killTweensOf(button);
						gsap.set(button, { clearProps: "x,y" });
					});
				}
			}

			const refreshFrame = requestAnimationFrame(() => ScrollTrigger.refresh());
			void document.fonts.ready.then(() => {
				if (!cancelled && !disposed) ScrollTrigger.refresh();
			});

			disposeEnhancements = () => {
				if (disposed) return;
				disposed = true;
				cancelAnimationFrame(refreshFrame);
				context.revert();
				for (const dispose of disposers) dispose();
				lenisSnap?.destroy();
				if (lenis && lenisTick) {
					gsap.ticker.remove(lenisTick);
					lenis.off("scroll", syncScrollTrigger);
					lenis.destroy();
				}
				if (reduceMotion.matches) {
					root.dataset.sellerMotion = "reduced";
				} else {
					delete root.dataset.sellerMotion;
				}
			};
		})();

		return () => {
			cancelled = true;
			reduceMotion.removeEventListener("change", handleReducedMotionChange);
			disposeEnhancements?.();
		};
	}, []);

	return null;
}
