"use client";

import { toast } from "@/lib/feedback/toast";
import type { FormActionResult } from "@/lib/forms/action-result";

type ActionToastCopy = {
	success?: string;
	error?: string;
};

export function toastActionResult<T>(
	result: FormActionResult<T>,
	copy: ActionToastCopy = {},
) {
	if (result.ok) {
		toast.success(result.message ?? copy.success ?? "操作完成");
		return;
	}

	toast.error(result.message ?? copy.error ?? "操作失敗，請稍後再試");
}
