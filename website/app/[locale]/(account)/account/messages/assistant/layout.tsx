import "@/styles/features/messages.css";
import "@/styles/features/procurement-assistant.css";
import type { ReactNode } from "react";

/**
 * Keep the shared Messages primitives ahead of the Hawkr Agent extensions.
 * Several Agent selectors intentionally refine `.thread-page*` rules, so this
 * segment is the single route-level cascade owner for the full workspace.
 */
export default function AssistantMessagesLayout({
	children,
}: {
	children: ReactNode;
}) {
	return children;
}
