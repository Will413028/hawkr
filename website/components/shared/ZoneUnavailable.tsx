"use client";

import ErrorState from "@/components/shared/ErrorState";
import type { RequiredAccess } from "@/lib/auth/redirect";

/**
 * 單體暫時答不出「這個帳號是誰」（B-199）：不是登出，所以不導去登入頁；顯示可重試的錯誤狀態。
 */
export default function ZoneUnavailable({
	required,
}: {
	required: RequiredAccess;
}) {
	return (
		<ErrorState
			error={new Error(`session unavailable: ${required}`)}
			reset={() => window.location.reload()}
		/>
	);
}
