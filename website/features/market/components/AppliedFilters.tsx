"use client";

import { AnimatePresence, motion, useReducedMotion } from "framer-motion";
import Link from "@/components/shared/Link";
import { useObservedRouter } from "@/lib/navigation/use-observed-router";

/**
 * One removable applied-filter chip. Shape is fully serializable so the
 * server component can compute every href / label and hand the list across
 * the RSC boundary — this client only owns the animation.
 */
export interface AppliedChip {
	/** Stable identity across navigations so AnimatePresence can match enter /
	 * exit (e.g. `cat-鳳梨`, `hashtag-產銷履歷`, `temp`). */
	key: string;
	kind: "keyword" | "category" | "temp" | "price" | "hashtag" | "certification";
	/** Small grey prefix (關鍵字 / 分類 / 溫層 …). Omitted for hashtag chips. */
	label?: string;
	/** Main chip text. For `hashtag` it is the raw tag name (rendered with #). */
	value: string;
	/** L1 parent name for a category chip → renders "水果 › 鳳梨". */
	crumb?: string;
	/** Href that removes just this filter. */
	href: string;
	ariaLabel: string;
}

interface AppliedFiltersProps {
	chips: AppliedChip[];
	clearAllHref: string;
}

/**
 * Animated "已選篩選" summary. Static appearance is byte-for-byte the same as
 * the old server-rendered `<section>` (same class names / markup); the only
 * addition is Framer Motion so toggling a filter glides the sidebar instead
 * of snapping it — the height jump used to shove every control below it,
 * which read as jitter (see issue #860 discussion).
 *
 * Stays mounted at the same tree position across server navigations, so the
 * component's Framer state survives each filter toggle and the open/close +
 * chip add/remove animations actually fire.
 */
export default function AppliedFilters({
	chips,
	clearAllHref,
}: AppliedFiltersProps) {
	const reduceMotion = useReducedMotion();
	const router = useObservedRouter("market_applied_filters");
	const hasChips = chips.length > 0;

	// Easing matches the sidebar chevron / popover transitions already in
	// market-products-list.css for a consistent feel.
	const ease = [0.16, 1, 0.3, 1] as const;

	return (
		<AnimatePresence initial={false}>
			{hasChips ? (
				<motion.div
					key="applied"
					layout={reduceMotion ? false : "position"}
					// `marginBottom: -24` cancels the sidebar body's `gap: 24px`
					// while collapsed, so an empty sidebar shows no residual gap —
					// keeping the resting layout identical to before.
					initial={
						reduceMotion ? false : { height: 0, opacity: 0, marginBottom: -24 }
					}
					animate={{ height: "auto", opacity: 1, marginBottom: 0 }}
					exit={
						reduceMotion
							? { opacity: 0 }
							: { height: 0, opacity: 0, marginBottom: -24 }
					}
					transition={{ duration: reduceMotion ? 0 : 0.24, ease }}
					style={{ overflow: "hidden" }}
				>
					<section
						className="market-list-sidebar__applied"
						aria-label="已選的篩選"
					>
						<header className="market-list-sidebar__applied-head">
							<h2>已選篩選</h2>
							<button
								type="button"
								className="market-list-sidebar__applied-clear"
								onClick={() => {
									router.replace(clearAllHref, { scroll: false });
									router.refresh();
								}}
							>
								清除全部
							</button>
						</header>
						<ul className="market-list-sidebar__applied-list">
							<AnimatePresence initial={false} mode="popLayout">
								{chips.map((chip) => (
									<motion.li
										key={chip.key}
										layout={!reduceMotion}
										initial={reduceMotion ? false : { opacity: 0, scale: 0.85 }}
										animate={{ opacity: 1, scale: 1 }}
										exit={
											reduceMotion
												? { opacity: 0 }
												: { opacity: 0, scale: 0.85 }
										}
										transition={{ duration: reduceMotion ? 0 : 0.18, ease }}
									>
										<Link
											className="market-list-sidebar__chip"
											href={chip.href}
											scroll={false}
											aria-label={chip.ariaLabel}
										>
											{chip.label ? (
												<span className="market-list-sidebar__chip-label">
													{chip.label}
												</span>
											) : null}
											<span className="market-list-sidebar__chip-value">
												{chip.kind === "category" && chip.crumb ? (
													<>
														<span className="market-list-sidebar__chip-crumb">
															{chip.crumb}
														</span>
														<span
															className="market-list-sidebar__chip-sep"
															aria-hidden="true"
														>
															›
														</span>
														{chip.value}
													</>
												) : chip.kind === "hashtag" ? (
													`#${chip.value}`
												) : (
													chip.value
												)}
											</span>
											<span aria-hidden="true">×</span>
										</Link>
									</motion.li>
								))}
							</AnimatePresence>
						</ul>
					</section>
				</motion.div>
			) : null}
		</AnimatePresence>
	);
}
