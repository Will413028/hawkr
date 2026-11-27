"use client";

import * as React from "react";
import Link from "@/components/shared/Link";
import { useAuth } from "@/lib/auth/context";
import { loginPath } from "@/lib/auth/redirect";
import { cn } from "@/lib/utils/cn";
import {
	ASSISTANT_ENTRY_LAYOUT,
	AssistantEntryContent,
} from "./AssistantEntryLayout";
import { HAWKR_AGENT_ENABLED } from "./availability";
import HawkrAgentMark from "./HawkrAgentMark";
import {
	type HawkrAgentHandoffSource,
	writeHawkrAgentHandoff,
} from "./handoff";
import { ASSISTANT_PROCUREMENT_PATH } from "./routes";

interface AssistantEntryLinkProps {
	className?: string;
	onNavigate?: () => void;
	query?: string;
	source?: HawkrAgentHandoffSource;
	variant?: "panel" | "page";
}

const AssistantEntryLink = React.forwardRef<
	HTMLAnchorElement,
	AssistantEntryLinkProps
>(function AssistantEntryLink(
	{
		className,
		onNavigate,
		query = "",
		source = "search-page",
		variant = "panel",
	},
	ref,
) {
	const { status } = useAuth();
	if (!HAWKR_AGENT_ENABLED) return null;

	const normalizedQuery = query.trim();

	const requiresBuyerLogin = status !== "unknown" && status !== "signedIn";
	const href = requiresBuyerLogin
		? loginPath({ role: "buyer", redirect: ASSISTANT_PROCUREMENT_PATH })
		: ASSISTANT_PROCUREMENT_PATH;

	return (
		<Link
			ref={ref}
			href={href}
			prefetch={false}
			data-hawkr-agent-entry
			className={cn(
				ASSISTANT_ENTRY_LAYOUT,
				"group no-underline",
				"transition-[background-color,border-color,box-shadow] duration-200 ease-out",
				"hover:border-hk-primary hover:bg-hk-primary-soft hover:shadow-hk-sm",
				"active:bg-hk-primary-soft active:shadow-none",
				"focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-hk-primary",
				"motion-reduce:transition-none",
				variant === "page" && "min-h-12 px-4 py-3",
				className,
			)}
			onClick={() => {
				if (normalizedQuery) {
					writeHawkrAgentHandoff(normalizedQuery, source);
				}
				onNavigate?.();
			}}
		>
			<AssistantEntryContent
				query={query}
				mark={<HawkrAgentMark className="assistant-mark assistant-mark--sm" />}
			/>
		</Link>
	);
});

export default AssistantEntryLink;
