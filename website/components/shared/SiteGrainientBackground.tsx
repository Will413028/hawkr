"use client";

import { useEffect, useRef } from "react";
import type { GrainientRenderer } from "@/lib/animation/grainient-renderer";
import {
	getSiteBackgroundState,
	hydrateSiteBackgroundState,
	setSiteBackgroundAvailable,
	subscribeSiteBackground,
} from "@/lib/animation/site-background-state";
import { whenDocumentSettled } from "@/lib/hydration/document-settled";

type SaveDataConnection = EventTarget & { saveData?: boolean };

/** One persistent decoration outside route content. CSS owns the static first paint. */
export default function SiteGrainientBackground() {
	const viewportRef = useRef<HTMLDivElement>(null);

	useEffect(() => {
		const viewport = viewportRef.current;
		if (!viewport) return;
		const reducedMotion = window.matchMedia("(prefers-reduced-motion: reduce)");
		const forcedColors = window.matchMedia("(forced-colors: active)");
		const connection = (
			navigator as Navigator & { connection?: SaveDataConnection }
		).connection;
		let disposed = false;
		let settled = false;
		let visible = false;
		let pageHidden = false;
		let importing = false;
		let failed = false;
		let idle: number | null = null;
		let timer: number | null = null;
		let renderer: GrainientRenderer | null = null;
		let canvas: HTMLCanvasElement | null = null;
		hydrateSiteBackgroundState();

		const allowed = () =>
			!reducedMotion.matches && !forcedColors.matches && !connection?.saveData;
		const onScreen = () => visible && !document.hidden && !pageHidden;
		const cancelScheduled = () => {
			if (idle !== null) window.cancelIdleCallback(idle);
			if (timer !== null) window.clearTimeout(timer);
			idle = null;
			timer = null;
		};
		const destroy = () => {
			renderer?.destroy();
			renderer = null;
			canvas?.remove();
			canvas = null;
			setSiteBackgroundAvailable(false);
		};
		const fail = () => {
			failed = true;
			destroy();
		};
		const initialize = async () => {
			idle = null;
			timer = null;
			if (disposed || !allowed() || !onScreen()) return;
			importing = true;
			try {
				const { createGrainientRenderer } = await import(
					"@/lib/animation/grainient-renderer"
				);
				// Imports cannot be aborted. Re-check every gate before allocating GPU resources.
				if (disposed || !allowed() || !onScreen()) return;
				const styles = getComputedStyle(viewport);
				const colors = [
					styles.getPropertyValue("--site-grainient-color-1").trim(),
					styles.getPropertyValue("--site-grainient-color-2").trim(),
					styles.getPropertyValue("--site-grainient-color-3").trim(),
					styles.getPropertyValue("--site-grainient-color-4").trim(),
				] as const;
				canvas = document.createElement("canvas");
				canvas.className = "site-grainient__canvas";
				canvas.setAttribute("aria-hidden", "true");
				viewport.appendChild(canvas);
				renderer = createGrainientRenderer(canvas, {
					colors,
					dark: document.documentElement.dataset.theme === "dark",
					onError: fail,
				});
				if (!renderer) {
					fail();
					return;
				}
				setSiteBackgroundAvailable(true);
				if (!getSiteBackgroundState().paused) renderer.resume();
			} catch {
				// Unsupported WebGL, shader/context failure or a failed chunk: keep CSS.
				fail();
			} finally {
				importing = false;
			}
		};
		const sync = () => {
			if (disposed) return;
			if (!allowed()) {
				cancelScheduled();
				destroy();
				return;
			}
			if (!onScreen()) {
				cancelScheduled();
				renderer?.pause();
				return;
			}
			if (renderer) {
				if (getSiteBackgroundState().paused) renderer.pause();
				else renderer.resume();
				return;
			}
			if (!settled || failed || importing || idle !== null || timer !== null)
				return;
			if (typeof window.requestIdleCallback === "function") {
				idle = window.requestIdleCallback(() => void initialize(), {
					timeout: 2000,
				});
			} else {
				timer = window.setTimeout(() => void initialize(), 250);
			}
		};
		const unsubscribe = subscribeSiteBackground(sync);
		const themeObserver = new MutationObserver(() => {
			renderer?.setDarkMode(document.documentElement.dataset.theme === "dark");
		});
		themeObserver.observe(document.documentElement, {
			attributes: true,
			attributeFilter: ["data-theme"],
		});
		const observer =
			typeof IntersectionObserver === "undefined"
				? null
				: new IntersectionObserver(([entry]) => {
						visible = entry.isIntersecting;
						sync();
					});
		observer?.observe(viewport);
		// Without visibility evidence keep the static fallback; never guess GPU eligibility.
		const hide = () => {
			pageHidden = true;
			sync();
		};
		const show = () => {
			pageHidden = false;
			sync();
		};
		reducedMotion.addEventListener("change", sync);
		forcedColors.addEventListener("change", sync);
		connection?.addEventListener("change", sync);
		document.addEventListener("visibilitychange", sync);
		window.addEventListener("pagehide", hide);
		window.addEventListener("pageshow", show);
		void whenDocumentSettled().then(() => {
			settled = true;
			sync();
		});

		return () => {
			disposed = true;
			unsubscribe();
			themeObserver.disconnect();
			cancelScheduled();
			observer?.disconnect();
			reducedMotion.removeEventListener("change", sync);
			forcedColors.removeEventListener("change", sync);
			connection?.removeEventListener("change", sync);
			document.removeEventListener("visibilitychange", sync);
			window.removeEventListener("pagehide", hide);
			window.removeEventListener("pageshow", show);
			destroy();
		};
	}, []);

	return (
		<div className="site-grainient" aria-hidden="true">
			<div ref={viewportRef} className="site-grainient__viewport" />
		</div>
	);
}
