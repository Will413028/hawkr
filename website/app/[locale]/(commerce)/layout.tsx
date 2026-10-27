import type { ReactNode } from "react";
import { noindexMetadata } from "@/lib/seo/metadata";

// Transactional surfaces (cart / checkout / payment) must never be indexed.
// checkout has its own nested layout (RoleZoneGuard); this group layout only
// contributes the noindex metadata that all of them inherit.
export const metadata = noindexMetadata();

export default function CommerceLayout({ children }: { children: ReactNode }) {
	return children;
}
