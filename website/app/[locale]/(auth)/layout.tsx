import type { ReactNode } from "react";
import { noindexMetadata } from "@/lib/seo/metadata";

export const metadata = noindexMetadata();

export default function AuthLayout({ children }: { children: ReactNode }) {
	return children;
}
