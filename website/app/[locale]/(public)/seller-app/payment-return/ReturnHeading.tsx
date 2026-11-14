"use client";

import { useSearchParams } from "next/navigation";

/** `?state=expired` 是連結過期；沒有或不認得的值當成完成。 */
export function returnStateOf(value: string | null): "done" | "expired" {
	return value === "expired" ? "expired" : "done";
}

export function ReturnHeading({
	done,
	expired,
}: {
	done: string;
	expired: string;
}) {
	const state = returnStateOf(useSearchParams()?.get("state") ?? null);
	return (
		<h1 className="text-xl font-bold">
			{state === "expired" ? expired : done}
		</h1>
	);
}
