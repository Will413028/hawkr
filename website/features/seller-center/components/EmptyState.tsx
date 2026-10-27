/**
 * Empty state — single centred message used inside seller-center cards
 * when a list is empty or loading. Replaces the inline `.seller-empty`
 * div that the dashboard / orders / inventory / customers pages all
 * spell out.
 */

import type * as React from "react";

export interface EmptyStateProps {
	children: React.ReactNode;
	className?: string;
}

export function EmptyState({
	children,
	className = "seller-empty",
}: EmptyStateProps) {
	return <div className={className}>{children}</div>;
}
