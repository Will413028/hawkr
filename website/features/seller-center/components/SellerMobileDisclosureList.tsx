"use client";

import "@/styles/components/seller-mobile-disclosure.css";

import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";

export interface SellerMobileDisclosureSummary {
	/** Accessible name for the whole expand/collapse control. */
	label: string;
	/** Compact 32px visual anchor: thumbnail, avatar, carrier mark, or icon. */
	leading: React.ReactNode;
	/** Primary record identifier. Kept to one line and ellipsized on narrow phones. */
	title: React.ReactNode;
	/** Optional compact datum such as price, stock, date, or unread count. */
	meta?: React.ReactNode;
	/** Optional semantic state badge. */
	status?: React.ReactNode;
}

interface SellerMobileDisclosureListProps<T> {
	items: readonly T[];
	getKey: (item: T) => string;
	getSummary: (item: T) => SellerMobileDisclosureSummary;
	renderDetails: (item: T) => React.ReactNode;
	ariaLabel: string;
	/** Mount details only while their row is expanded. Useful for media-heavy rows. */
	deferDetailsUntilExpanded?: boolean;
	/** Changing this value closes the open card after a filter/search/page change. */
	resetKey?: string;
	className?: string;
}

/**
 * Shared phone record-list contract for seller-center workspaces.
 *
 * The desktop representation remains owned by each feature. On phones every
 * record starts as one 58px card, and this component owns the single-expanded
 * invariant so individual pages cannot accidentally allow a wall of open rows.
 */
export function SellerMobileDisclosureList<T>({
	items,
	getKey,
	getSummary,
	renderDetails,
	ariaLabel,
	deferDetailsUntilExpanded = false,
	resetKey = "",
	className,
}: SellerMobileDisclosureListProps<T>) {
	const [expandedKey, setExpandedKey] = React.useState<string | null>(null);

	// biome-ignore lint/correctness/useExhaustiveDependencies: resetKey is an intentional external reset signal for search/filter/page changes.
	React.useEffect(() => {
		setExpandedKey(null);
	}, [resetKey]);

	React.useEffect(() => {
		setExpandedKey((current) =>
			current && !items.some((item) => getKey(item) === current)
				? null
				: current,
		);
	}, [getKey, items]);

	return (
		<ul
			className={["seller-mobile-disclosure-list", className]
				.filter(Boolean)
				.join(" ")}
			aria-label={ariaLabel}
		>
			{items.map((item) => {
				const itemKey = getKey(item);
				const expanded = expandedKey === itemKey;
				return (
					<SellerMobileDisclosureItem
						key={itemKey}
						expanded={expanded}
						onToggle={() =>
							setExpandedKey((current) =>
								current === itemKey ? null : itemKey,
							)
						}
						summary={getSummary(item)}
					>
						{!deferDetailsUntilExpanded || expanded
							? renderDetails(item)
							: null}
					</SellerMobileDisclosureItem>
				);
			})}
		</ul>
	);
}

function SellerMobileDisclosureItem({
	expanded,
	onToggle,
	summary,
	children,
}: {
	expanded: boolean;
	onToggle: () => void;
	summary: SellerMobileDisclosureSummary;
	children: React.ReactNode;
}) {
	const detailsId = React.useId();
	return (
		<li className="seller-mobile-disclosure-list__item">
			<article
				className="seller-mobile-disclosure"
				data-expanded={expanded ? "true" : "false"}
			>
				<button
					type="button"
					className="seller-mobile-disclosure__summary"
					aria-label={`${expanded ? "收合" : "展開"}${summary.label}`}
					aria-expanded={expanded}
					aria-controls={detailsId}
					onClick={onToggle}
				>
					<span
						className="seller-mobile-disclosure__leading"
						aria-hidden="true"
					>
						{summary.leading}
					</span>
					<strong className="seller-mobile-disclosure__title">
						{summary.title}
					</strong>
					<span className="seller-mobile-disclosure__meta">{summary.meta}</span>
					<span className="seller-mobile-disclosure__status">
						{summary.status}
					</span>
					<span
						className="seller-mobile-disclosure__chevron"
						aria-hidden="true"
					>
						<AppIcon name="chevron-down" size={18} />
					</span>
				</button>

				<div
					id={detailsId}
					className="seller-mobile-disclosure__details"
					hidden={!expanded}
				>
					{children}
				</div>
			</article>
		</li>
	);
}

/** Consistent fallback mark for records without an authoritative image. */
export function SellerMobileDisclosureIcon({
	name,
}: {
	name: React.ComponentProps<typeof AppIcon>["name"];
}) {
	return <AppIcon name={name} size={18} />;
}
