import type { ReactNode } from "react";
import { noindexMetadata } from "@/lib/seo/metadata";

export const metadata = noindexMetadata();

export default function CommerceLayout({ children }: { children: ReactNode }) {
	return children;
}
