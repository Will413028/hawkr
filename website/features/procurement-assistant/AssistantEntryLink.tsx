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
	const { isLoggedIn, sessionSettled } = useAuth();
	if (!HAWKR_AGENT_ENABLED) return null;

	const normalizedQuery = query.trim();

	// Only treat the visitor as a guest after the cookie probe has produced an
	// authoritative answer. During hydration (or a transient session outage),
	// keep the intended deep link instead of falsely bouncing a signed-in buyer
	// to login.
	// 每個登入的帳號都是買家，所以只看有沒有登入。
	const requiresBuyerLogin = sessionSettled && !isLoggedIn;
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
				// 這是「內容列表 item」型的卡（記號＋標題＋說明＋chevron），
				// 所以 radius 走 16px（design-system §6「小卡片」＋§9.2「工作台
				// 型卡片 12–16px」）。訊息側欄已收斂成分頁，不再複製這張入口卡。
				ASSISTANT_ENTRY_LAYOUT,
				"group no-underline",
				// hover 走 §9.2 的「border + shadow 小幅變化」。底色也真的要變：
				// 原本 hover 指定同一個 bg token，等於只有邊框會動。
				"transition-[background-color,border-color,box-shadow] duration-200 ease-out",
				"hover:border-hk-primary hover:bg-hk-primary-soft hover:shadow-hk-sm",
				// 按下的回饋——觸控裝置沒有 hover，沒有 active 態就完全沒有回應感。
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
