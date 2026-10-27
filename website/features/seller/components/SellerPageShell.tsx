"use client";

import type * as React from "react";
import { useScrollFade } from "@/components/ui/useScrollFade";

export default function SellerPageShell({
	children,
}: {
	children: React.ReactNode;
}) {
	useScrollFade();
	return <>{children}</>;
}
