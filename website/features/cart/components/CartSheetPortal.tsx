"use client";

import * as React from "react";
import { createPortal } from "react-dom";
import "@/styles/components/sheet.css";

export function CartSheetPortal({ children }: { children: React.ReactNode }) {
	const [mounted, setMounted] = React.useState(false);

	React.useEffect(() => {
		setMounted(true);
	}, []);

	if (!mounted) return null;

	return createPortal(children, document.body);
}
